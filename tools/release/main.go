// Command release builds the DevCade release-candidate artifacts: six
// CGO-free binaries packed into deterministic archives, a SHA256SUMS file
// computed from the archive bytes, and Homebrew/Scoop manifests generated
// from that SHA256SUMS file.
//
// Usage, from the repository root:
//
//	go run ./tools/release -version 1.0.0-rc.1 -out dist/release
//	go run ./tools/release -version 1.0.0-rc.1 -out dist/release -manifests-only
//
// It never publishes anything; see docs/releasing.md.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	modulePath = "github.com/cagridursun/devcade"
	// defaultBaseURL is the GitHub release download location for a tag
	// v<version>. {version} is replaced with the release version.
	defaultBaseURL = "https://github.com/cagridursun/devcade/releases/download/v{version}"
	// defaultEpoch is the modification time stored for every archive entry
	// when SOURCE_DATE_EPOCH is not set: 2026-01-01T00:00:00Z.
	defaultEpoch = 1767225600
	sumsName     = "SHA256SUMS"
	noticeName   = "LICENSE-NOTICE.txt"
	readmeName   = "README.md"
)

// target is one GOOS/GOARCH pair that is released.
type target struct {
	OS, Arch string
}

// targets is the fixed release matrix, in archive-name order.
var targets = []target{
	{"darwin", "amd64"},
	{"darwin", "arm64"},
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"windows", "amd64"},
	{"windows", "arm64"},
}

func (t target) String() string { return t.OS + "/" + t.Arch }

// binaryName is the executable's name inside the archive.
func (t target) binaryName() string {
	if t.OS == "windows" {
		return "devcade.exe"
	}
	return "devcade"
}

// archiveName returns devcade_<version>_<os>_<arch>.tar.gz, or .zip on
// Windows.
func archiveName(version string, t target) string {
	ext := ".tar.gz"
	if t.OS == "windows" {
		ext = ".zip"
	}
	return "devcade_" + version + "_" + t.OS + "_" + t.Arch + ext
}

// versionRE accepts semantic versions without a leading "v" and without
// build metadata ("+..."), which would be awkward in file names and URLs:
// 1.0.0, 1.0.0-rc.1, 0.2.0-beta.3.
var versionRE = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z-]+(\.[0-9A-Za-z-]+)*)?$`)

func validateVersion(v string) error {
	if v == "" {
		return errors.New("version is required (for example -version 1.0.0-rc.1)")
	}
	if strings.HasPrefix(v, "v") {
		return fmt.Errorf("version %q: omit the leading \"v\" (the tag is v<version>)", v)
	}
	if len(v) > 64 || !versionRE.MatchString(v) {
		return fmt.Errorf("version %q is not MAJOR.MINOR.PATCH[-PRERELEASE] (for example 1.0.0 or 1.0.0-rc.1)", v)
	}
	// Semantic versioning forbids leading zeros in numeric pre-release parts.
	if i := strings.IndexByte(v, '-'); i >= 0 {
		for _, p := range strings.Split(v[i+1:], ".") {
			if len(p) > 1 && p[0] == '0' && isDigits(p) {
				return fmt.Errorf("version %q: numeric pre-release identifier %q has a leading zero", v, p)
			}
		}
	}
	return nil
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// baseURLRE limits base URLs to https URLs without characters that would need
// escaping inside the generated Ruby or JSON strings.
var baseURLRE = regexp.MustCompile(`^https://[A-Za-z0-9._~:/?#\[\]@!$&'()*+,;=%-]+$`)

func resolveBaseURL(raw, version string) (string, error) {
	u := strings.TrimRight(strings.ReplaceAll(raw, "{version}", version), "/")
	if !baseURLRE.MatchString(u) || strings.ContainsAny(u, `"'\`) {
		return "", fmt.Errorf("base URL %q must be an https:// URL without quotes or spaces", u)
	}
	return u, nil
}

// sourceDateEpoch returns the fixed archive timestamp: SOURCE_DATE_EPOCH when
// set (see reproducible-builds.org), otherwise defaultEpoch.
func sourceDateEpoch() (time.Time, error) {
	s := os.Getenv("SOURCE_DATE_EPOCH")
	if s == "" {
		return time.Unix(defaultEpoch, 0).UTC(), nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	// Zip timestamps cannot go before 1980.
	if err != nil || n < 315532800 || n > 4102444800 {
		return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH=%q must be a Unix time between 1980 and 2100", s)
	}
	return time.Unix(n, 0).UTC(), nil
}

// insideDir reports whether path is dir itself or below it.
func insideDir(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

type config struct {
	LeaderboardURL string
	Version        string
	Root           string // repository root (absolute)
	Out            string // output directory (absolute)
	BaseURL        string // resolved, no trailing slash
	ManifestsOnly  bool
	Go             string // go command
	MTime          time.Time
	Log            io.Writer
}

func main() {
	if err := cli(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func cli(args []string, log io.Writer) error {
	fs := flag.NewFlagSet("release", flag.ContinueOnError)
	version := fs.String("version", "", "release version without a leading v, e.g. 1.0.0-rc.1 (required)")
	out := fs.String("out", filepath.Join("dist", "release"), "output directory; must be inside <root>/dist")
	root := fs.String("root", ".", "repository root")
	baseURL := fs.String("base-url", defaultBaseURL, "download URL prefix for the generated manifests; {version} is replaced")
	leaderboardURL := fs.String("leaderboard-url", "", "public HTTPS leaderboard endpoint embedded in all game binaries")
	manifestsOnly := fs.Bool("manifests-only", false, "skip building; regenerate manifests from <out>/SHA256SUMS")
	goCmd := fs.String("go", "go", "go command used for building")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %q", fs.Args())
	}
	if err := validateVersion(*version); err != nil {
		return err
	}
	cfg := config{Version: *version, ManifestsOnly: *manifestsOnly, Go: *goCmd, Log: log}
	if err := validateLeaderboardURL(*leaderboardURL); err != nil {
		return err
	}
	cfg.LeaderboardURL = strings.TrimRight(*leaderboardURL, "/")
	var err error
	if cfg.BaseURL, err = resolveBaseURL(*baseURL, *version); err != nil {
		return err
	}
	if cfg.MTime, err = sourceDateEpoch(); err != nil {
		return err
	}
	if cfg.Root, err = filepath.Abs(*root); err != nil {
		return err
	}
	if err := checkRoot(cfg.Root); err != nil {
		return err
	}
	if cfg.Out, err = filepath.Abs(*out); err != nil {
		return err
	}
	if !insideDir(filepath.Join(cfg.Root, "dist"), cfg.Out) {
		return fmt.Errorf("output directory %s must be below %s (which git ignores)", cfg.Out, filepath.Join(cfg.Root, "dist"))
	}
	return run(cfg)
}

// checkRoot makes sure root is the DevCade module.
func checkRoot(root string) error {
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return fmt.Errorf("-root %s: %w", root, err)
	}
	if !regexp.MustCompile(`(?m)^module\s+` + regexp.QuoteMeta(modulePath) + `\s*$`).Match(b) {
		return fmt.Errorf("-root %s is not the %s module", root, modulePath)
	}
	return nil
}

// run executes the pipeline. Everything it writes goes below cfg.Out.
func run(cfg config) error {
	if !cfg.ManifestsOnly {
		if err := buildAll(cfg); err != nil {
			return err
		}
	}
	sums, err := os.ReadFile(filepath.Join(cfg.Out, sumsName))
	if err != nil {
		return fmt.Errorf("read checksums: %w", err)
	}
	parsed, err := parseChecksums(sums)
	if err != nil {
		return err
	}
	written, err := writeManifests(cfg.Root, cfg.Out, cfg.Version, cfg.BaseURL, parsed)
	if err != nil {
		return err
	}
	for _, p := range written {
		fmt.Fprintln(cfg.Log, "wrote", p)
	}
	return nil
}

func buildAll(cfg config) error {
	readme, err := readText(filepath.Join(cfg.Root, readmeName))
	if err != nil {
		return err
	}
	notice, err := readText(filepath.Join(cfg.Root, "packaging", noticeName))
	if err != nil {
		return err
	}
	if gv, err := exec.Command(cfg.Go, "version").Output(); err == nil {
		fmt.Fprint(cfg.Log, "toolchain: ", string(gv))
	}
	// Start from an empty directory so stale archives never end up in
	// SHA256SUMS. cfg.Out is always below <root>/dist.
	if err := os.RemoveAll(cfg.Out); err != nil {
		return err
	}
	buildDir := filepath.Join(cfg.Out, ".build")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}
	var files []string
	for _, t := range targets {
		bin := filepath.Join(buildDir, t.OS+"_"+t.Arch, t.binaryName())
		fmt.Fprintf(cfg.Log, "build %s\n", t)
		if err := goBuild(cfg, t, bin); err != nil {
			return err
		}
		binData, err := os.ReadFile(bin)
		if err != nil {
			return err
		}
		name := archiveName(cfg.Version, t)
		data, err := packageArchive(t, binData, readme, notice, cfg.MTime)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(cfg.Out, name), data, 0o644); err != nil {
			return err
		}
		files = append(files, name)
	}
	if err := os.RemoveAll(buildDir); err != nil {
		return err
	}
	// The installer scripts ship next to the archives (and are covered by
	// SHA256SUMS) so users can verify them too.
	for _, name := range []string{"install.sh", "install.ps1"} {
		b, err := readText(filepath.Join(cfg.Root, "packaging", "install", name))
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(cfg.Out, name), b, 0o644); err != nil {
			return err
		}
		files = append(files, name)
	}
	sums, err := checksumFile(cfg.Out, files)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(cfg.Out, sumsName), sums, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(cfg.Log, "wrote %s\n%s", filepath.Join(cfg.Out, sumsName), sums)
	return nil
}

// buildEnv returns the environment for one cross-compile. Variables that
// change the generated code are pinned so the user's environment cannot leak
// into a release.
func buildEnv(base []string, t target) []string {
	pinned := map[string]string{
		"CGO_ENABLED":  "0",
		"GOOS":         t.OS,
		"GOARCH":       t.Arch,
		"GOFLAGS":      "-mod=readonly",
		"GOAMD64":      "v1",
		"GOARM64":      "v8.0",
		"GOEXPERIMENT": "",
	}
	env := make([]string, 0, len(base)+len(pinned))
	for _, kv := range base {
		k, _, _ := strings.Cut(kv, "=")
		if _, ok := pinned[strings.ToUpper(k)]; ok {
			continue
		}
		env = append(env, kv)
	}
	for _, k := range []string{"CGO_ENABLED", "GOOS", "GOARCH", "GOFLAGS", "GOAMD64", "GOARM64", "GOEXPERIMENT"} {
		env = append(env, k+"="+pinned[k])
	}
	return env
}

// buildArgs returns the go build arguments. -buildvcs=false keeps git state
// (commit, commit time, dirty flag) out of the binary, so its bytes depend
// only on the source files, the toolchain and these flags.
func buildArgs(version, out string) []string {
	return []string{
		"build",
		"-trimpath",
		"-buildvcs=false",
		"-ldflags=-s -w -X main.version=" + version,
		"-o", out,
		"./cmd/devcade",
	}
}

func goBuild(cfg config, t target, out string) error {
	cmd := exec.Command(cfg.Go, buildArgsWithLeaderboard(cfg.Version, out, cfg.LeaderboardURL)...)
	cmd.Dir = cfg.Root
	cmd.Env = buildEnv(os.Environ(), t)
	cmd.Stdout = cfg.Log
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build %s: %w", t, err)
	}
	return nil
}

func validateLeaderboardURL(raw string) error {
	if raw == "" {
		return nil
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(raw, " \t\r\n\"'\\") {
		return errors.New("leaderboard URL must be public HTTPS without credentials, query, fragment or whitespace")
	}
	return nil
}
func buildArgsWithLeaderboard(version, out, endpoint string) []string {
	args := buildArgs(version, out)
	if endpoint != "" {
		for i, arg := range args {
			if strings.HasPrefix(arg, "-ldflags=") {
				args[i] += " -X main.leaderboardURL=" + endpoint
			}
		}
	}
	return args
}

// readText reads a text file with LF line endings, whatever the checkout's
// line-ending settings are.
func readText(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return normalizeNewlines(b), nil
}

func normalizeNewlines(b []byte) []byte {
	return []byte(strings.ReplaceAll(string(b), "\r\n", "\n"))
}
