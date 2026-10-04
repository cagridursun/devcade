package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var testMTime = time.Unix(defaultEpoch, 0).UTC()

const (
	fakeReadme = "# DevCade\n\nfixture readme\n"
	fakeNotice = "MIT License fixture\n"
)

func fakeBinary(t target) []byte {
	return []byte("fake devcade binary for " + t.String() + " " + modulePath + "\n")
}

func TestValidateVersion(t *testing.T) {
	good := []string{"1.0.0", "0.1.0", "1.0.0-rc.1", "10.20.30", "1.0.0-alpha", "1.0.0-alpha.beta.11", "1.0.0-x-y", "0.1.0-dev"}
	for _, v := range good {
		if err := validateVersion(v); err != nil {
			t.Errorf("validateVersion(%q) = %v, want nil", v, err)
		}
	}
	bad := []string{"", "v1.0.0", "1.0", "1", "1.0.0.0", "01.0.0", "1.02.0", "1.0.0-", "1.0.0-rc..1", "1.0.0-rc.01",
		"1.0.0+build.1", "1.0.0 rc", "1.0.0-rc/1", "1.0.0-ü", "../1.0.0", "1.0.0\n", strings.Repeat("1", 70) + ".0.0"}
	for _, v := range bad {
		if err := validateVersion(v); err == nil {
			t.Errorf("validateVersion(%q) = nil, want error", v)
		}
	}
}

func TestLeaderboardURLIsValidatedAndEmbeddedWithoutAmbientChanges(t *testing.T) {
	for _, u := range []string{"", "https://scores.example", "https://scores.example/api"} {
		if err := validateLeaderboardURL(u); err != nil {
			t.Fatal(err)
		}
	}
	for _, u := range []string{"http://scores.example", "https://user:secret@scores.example", "https://scores.example?token=secret", "https://scores.example#fragment", "https://scores.example/ bad", "https://scores.example/\"bad"} {
		if validateLeaderboardURL(u) == nil {
			t.Error("unsafe endpoint accepted")
		}
	}
	args := strings.Join(buildArgsWithLeaderboard("1.0.0-rc.1", "out", "https://scores.example"), " ")
	if !strings.Contains(args, "-X main.leaderboardURL=https://scores.example") {
		t.Fatal(args)
	}
	if strings.Contains(strings.Join(buildArgs("1.0.0", "out"), " "), "leaderboardURL") {
		t.Fatal("default build gained an endpoint")
	}
}

func TestTargetsAndArchiveNames(t *testing.T) {
	want := []string{
		"devcade_1.0.0-rc.1_darwin_amd64.tar.gz",
		"devcade_1.0.0-rc.1_darwin_arm64.tar.gz",
		"devcade_1.0.0-rc.1_linux_amd64.tar.gz",
		"devcade_1.0.0-rc.1_linux_arm64.tar.gz",
		"devcade_1.0.0-rc.1_windows_amd64.zip",
		"devcade_1.0.0-rc.1_windows_arm64.zip",
	}
	if len(targets) != len(want) {
		t.Fatalf("got %d targets, want %d", len(targets), len(want))
	}
	for i, tg := range targets {
		if got := archiveName("1.0.0-rc.1", tg); got != want[i] {
			t.Errorf("archiveName(%s) = %q, want %q", tg, got, want[i])
		}
	}
}

func TestBuildArgsAndEnv(t *testing.T) {
	args := strings.Join(buildArgs("1.2.3", "out/devcade"), " ")
	for _, want := range []string{"build", "-trimpath", "-buildvcs=false", "-ldflags=-s -w -X main.version=1.2.3", "./cmd/devcade"} {
		if !strings.Contains(args, want) {
			t.Errorf("build args %q lack %q", args, want)
		}
	}
	env := buildEnv([]string{"PATH=/bin", "CGO_ENABLED=1", "GOOS=plan9", "goarch=386", "GOFLAGS=-race", "GOAMD64=v3", "HOME=/h"}, target{"linux", "arm64"})
	got := map[string][]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		got[strings.ToUpper(k)] = append(got[strings.ToUpper(k)], v)
	}
	for k, v := range map[string]string{"CGO_ENABLED": "0", "GOOS": "linux", "GOARCH": "arm64", "GOFLAGS": "-mod=readonly", "GOAMD64": "v1", "GOARM64": "v8.0", "GOEXPERIMENT": "", "PATH": "/bin", "HOME": "/h"} {
		if len(got[k]) != 1 || got[k][0] != v {
			t.Errorf("env %s = %q, want exactly [%q]", k, got[k], v)
		}
	}
}

type archived struct {
	Name    string
	Mode    int64
	ModTime time.Time
	Uid     int
	Gid     int
	Data    string
}

func readTarGz(t *testing.T, b []byte) []archived {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if gz.Name != "" || !gz.ModTime.IsZero() {
		t.Errorf("gzip header name %q mtime %v, want empty", gz.Name, gz.ModTime)
	}
	tr := tar.NewReader(gz)
	var out []archived
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag != tar.TypeReg || h.Uname != "" || h.Gname != "" {
			t.Errorf("%s: type %c uname %q gname %q", h.Name, h.Typeflag, h.Uname, h.Gname)
		}
		data, _ := io.ReadAll(tr)
		out = append(out, archived{h.Name, h.Mode, h.ModTime.UTC(), h.Uid, h.Gid, string(data)})
	}
	return out
}

func readZip(t *testing.T, b []byte) []archived {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	var out []archived
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		out = append(out, archived{f.Name, int64(f.Mode().Perm()), f.Modified.UTC(), 0, 0, string(data)})
	}
	return out
}

func TestArchiveContents(t *testing.T) {
	for _, tg := range targets {
		t.Run(tg.OS+"_"+tg.Arch, func(t *testing.T) {
			b, err := packageArchive(tg, fakeBinary(tg), []byte(fakeReadme), []byte(fakeNotice), testMTime)
			if err != nil {
				t.Fatal(err)
			}
			var got []archived
			bin := "devcade"
			if tg.OS == "windows" {
				got = readZip(t, b)
				bin = "devcade.exe"
			} else {
				got = readTarGz(t, b)
			}
			want := []archived{
				{Name: noticeName, Mode: 0o644, Data: fakeNotice},
				{Name: readmeName, Mode: 0o644, Data: fakeReadme},
				{Name: bin, Mode: 0o755, Data: string(fakeBinary(tg))},
			}
			if len(got) != len(want) {
				t.Fatalf("entries %+v, want %d", got, len(want))
			}
			for i := range want {
				g, w := got[i], want[i]
				if g.Name != w.Name || g.Mode != w.Mode || g.Data != w.Data || g.Uid != 0 || g.Gid != 0 {
					t.Errorf("entry %d = %s mode %o uid %d gid %d, want %s mode %o", i, g.Name, g.Mode, g.Uid, g.Gid, w.Name, w.Mode)
				}
				if !g.ModTime.Equal(testMTime) {
					t.Errorf("%s mtime %v, want %v", g.Name, g.ModTime, testMTime)
				}
			}
		})
	}
}

func TestArchivesAreDeterministic(t *testing.T) {
	for _, tg := range targets {
		a, err := packageArchive(tg, fakeBinary(tg), []byte(fakeReadme), []byte(fakeNotice), testMTime)
		if err != nil {
			t.Fatal(err)
		}
		// Rebuild from fresh copies of the inputs after a delay: nothing
		// time- or memory-dependent may leak into the bytes.
		time.Sleep(10 * time.Millisecond)
		b, err := packageArchive(tg, append([]byte(nil), fakeBinary(tg)...), []byte(fakeReadme), []byte(fakeNotice), testMTime)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s: two builds differ", tg)
		}
		c, _ := packageArchive(tg, fakeBinary(tg), []byte(fakeReadme), []byte(fakeNotice), testMTime.Add(24*time.Hour))
		if bytes.Equal(a, c) {
			t.Errorf("%s: mtime is not recorded", tg)
		}
	}
}

func TestNormalizeNewlines(t *testing.T) {
	if got := string(normalizeNewlines([]byte("a\r\nb\r\n\nc"))); got != "a\nb\n\nc" {
		t.Errorf("got %q", got)
	}
}

func TestDistributionLicenseIncludesProjectAndDependencies(t *testing.T) {
	root := filepath.Join("..", "..")
	license, err := os.ReadFile(filepath.Join(root, "LICENSE"))
	if err != nil {
		t.Fatal(err)
	}
	notice, err := os.ReadFile(filepath.Join(root, "packaging", noticeName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(notice, normalizeNewlines(license)) {
		t.Fatal("binary distribution omits the project license")
	}
	for _, component := range []string{"github.com/gdamore/tcell/v2", "github.com/gdamore/encoding", "github.com/lucasb-eyer/go-colorful", "github.com/rivo/uniseg", "golang.org/x/sys", "golang.org/x/term", "golang.org/x/text", "Go runtime and standard library"} {
		if !bytes.Contains(notice, []byte(component)) {
			t.Errorf("binary distribution omits notices for %s", component)
		}
	}
}

var sumsLine = regexp.MustCompile(`^[0-9a-f]{64}  [A-Za-z0-9._+-]+$`)

func TestChecksumFile(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{"b.zip": "bee", "a.tar.gz": "ay", "install.sh": "#!/bin/sh\n"}
	var names []string
	for n, c := range files {
		if err := os.WriteFile(filepath.Join(dir, n), []byte(c), 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, n)
	}
	sums, err := checksumFile(dir, names)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasSuffix(sums, []byte("\n")) || bytes.Contains(sums, []byte("\r")) {
		t.Errorf("SHA256SUMS must use LF line endings and end with a newline: %q", sums)
	}
	lines := strings.Split(strings.TrimSuffix(string(sums), "\n"), "\n")
	wantOrder := []string{"a.tar.gz", "b.zip", "install.sh"}
	if len(lines) != len(wantOrder) {
		t.Fatalf("lines %q", lines)
	}
	for i, line := range lines {
		if !sumsLine.MatchString(line) {
			t.Errorf("line %q is not in sha256sum format", line)
		}
		sum := sha256.Sum256([]byte(files[wantOrder[i]]))
		if want := hex.EncodeToString(sum[:]) + "  " + wantOrder[i]; line != want {
			t.Errorf("line %d = %q, want %q", i, line, want)
		}
	}
	parsed, err := parseChecksums(sums)
	if err != nil || len(parsed) != 3 {
		t.Fatalf("parseChecksums = %v, %v", parsed, err)
	}
	if _, err := checksumFile(dir, []string{"a.tar.gz", "a.tar.gz"}); err == nil {
		t.Error("duplicate names accepted")
	}
	if _, err := checksumFile(dir, []string{"missing.zip"}); err == nil {
		t.Error("missing file accepted")
	}
}

func TestParseChecksumsRejectsMalformed(t *testing.T) {
	h := strings.Repeat("ab", 32)
	ok := h + "  x.zip\n" + h + " *y.tar.gz\n"
	if m, err := parseChecksums([]byte(ok)); err != nil || m["x.zip"] != h || m["y.tar.gz"] != h {
		t.Fatalf("parseChecksums(ok) = %v, %v", m, err)
	}
	for _, bad := range []string{
		"",
		h + " x.zip\n",                    // one space
		h[:63] + "  x.zip\n",              // short hash
		strings.ToUpper(h) + "  x.zip\n",  // upper case is not what we write
		h + "  x.zip\n" + h + "  x.zip\n", // duplicate
		h + "  ../x.zip\n",                // path
		h + "  x.zip\n\n",                 // blank line
		h + "  x.zip\r\n",                 // CRLF
		"0000000000000000000000000000000000000000000000000000000000000000z  x.zip\n",
	} {
		if _, err := parseChecksums([]byte(bad)); err == nil {
			t.Errorf("parseChecksums(%q) = nil error", bad)
		}
	}
}

// fakeSums returns SHA256SUMS-style data with distinct hashes per archive.
func fakeSums(version string) map[string]string {
	m := map[string]string{}
	for _, tg := range targets {
		sum := sha256.Sum256([]byte(tg.String()))
		m[archiveName(version, tg)] = hex.EncodeToString(sum[:])
	}
	return m
}

func TestManifestsFromTemplates(t *testing.T) {
	root := filepath.Join("..", "..")
	out := t.TempDir()
	const v = "1.0.0-rc.1"
	base := "https://github.com/cagridursun/devcade/releases/download/v" + v
	sums := fakeSums(v)
	paths, err := writeManifests(root, out, v, base, sums)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("wrote %v", paths)
	}

	rb, err := os.ReadFile(filepath.Join(out, "homebrew", "devcade.rb"))
	if err != nil {
		t.Fatal(err)
	}
	formula := string(rb)
	for _, want := range []string{"class Devcade < Formula", `version "` + v + `"`, "on_macos do", "on_linux do", "on_arm do", "on_intel do", `bin.install "devcade"`} {
		if !strings.Contains(formula, want) {
			t.Errorf("formula lacks %q", want)
		}
	}
	// Each URL is immediately followed by its own archive's checksum.
	for _, tg := range targets[:4] {
		name := archiveName(v, tg)
		want := "url \"" + base + "/" + name + "\"\n      sha256 \"" + sums[name] + "\""
		if !strings.Contains(formula, want) {
			t.Errorf("formula lacks %s with its checksum", name)
		}
	}
	if strings.Contains(formula, "windows") || strings.Contains(formula, "{{") {
		t.Error("formula references windows or template markers")
	}

	js, err := os.ReadFile(filepath.Join(out, "scoop", "devcade.json"))
	if err != nil {
		t.Fatal(err)
	}
	var scoop struct {
		Version      string
		Bin          string
		License      string
		Architecture map[string]struct{ URL, Hash string }
	}
	if err := json.Unmarshal(js, &scoop); err != nil {
		t.Fatalf("scoop manifest: %v\n%s", err, js)
	}
	if scoop.Version != v || scoop.Bin != "devcade.exe" || scoop.License != "MIT" || len(scoop.Architecture) != 2 {
		t.Errorf("scoop manifest = %+v", scoop)
	}
	for key, tg := range map[string]target{"64bit": {"windows", "amd64"}, "arm64": {"windows", "arm64"}} {
		a := scoop.Architecture[key]
		name := archiveName(v, tg)
		if a.URL != base+"/"+name || a.Hash != sums[name] {
			t.Errorf("scoop %s = %+v", key, a)
		}
	}

	// A missing checksum fails generation instead of leaving a placeholder.
	delete(sums, archiveName(v, target{"linux", "arm64"}))
	if _, err := writeManifests(root, t.TempDir(), v, base, sums); err == nil || !strings.Contains(err.Error(), "linux_arm64") {
		t.Errorf("missing checksum: err = %v", err)
	}
}

func TestRenderManifestRequiresAllTargets(t *testing.T) {
	m := manifest{Template: "t", Targets: []target{{"windows", "amd64"}, {"windows", "arm64"}}}
	sums := fakeSums("1.0.0")
	if _, err := renderManifest(m, `{{sha256 "windows" "amd64"}}`, "1.0.0", "https://x", sums); err == nil {
		t.Error("template that skips a target accepted")
	}
	if _, err := renderManifest(m, `{{sha256 "linux" "amd64"}} {{sha256 "windows" "amd64"}} {{sha256 "windows" "arm64"}}`, "1.0.0", "https://x", sums); err == nil {
		t.Error("foreign target accepted")
	}
	m.JSON = true
	if _, err := renderManifest(m, `{"a": "{{url "windows" "amd64"}}", "b": "{{sha256 "windows" "arm64"}}"`, "1.0.0", "https://x", sums); err == nil {
		t.Error("invalid JSON accepted")
	}
}

func TestResolveBaseURL(t *testing.T) {
	got, err := resolveBaseURL(defaultBaseURL, "1.0.0")
	if err != nil || got != "https://github.com/cagridursun/devcade/releases/download/v1.0.0" {
		t.Errorf("default = %q, %v", got, err)
	}
	if got, err := resolveBaseURL("https://example.test/dl/", "1.0.0"); err != nil || got != "https://example.test/dl" {
		t.Errorf("trailing slash = %q, %v", got, err)
	}
	for _, bad := range []string{"http://example.test", "file:///tmp", "https://a b", `https://a"b`, "https://a\\b", ""} {
		if _, err := resolveBaseURL(bad, "1.0.0"); err == nil {
			t.Errorf("resolveBaseURL(%q) accepted", bad)
		}
	}
}

func TestSourceDateEpoch(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "")
	if got, _ := sourceDateEpoch(); !got.Equal(testMTime) {
		t.Errorf("default = %v", got)
	}
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	if got, err := sourceDateEpoch(); err != nil || got.Unix() != 1700000000 {
		t.Errorf("= %v, %v", got, err)
	}
	for _, bad := range []string{"x", "-1", "100"} {
		t.Setenv("SOURCE_DATE_EPOCH", bad)
		if _, err := sourceDateEpoch(); err == nil {
			t.Errorf("SOURCE_DATE_EPOCH=%q accepted", bad)
		}
	}
}

func TestCLIValidation(t *testing.T) {
	root := filepath.Join("..", "..")
	var log bytes.Buffer
	cases := map[string][]string{
		"no version":       {"-root", root},
		"bad version":      {"-root", root, "-version", "v1.0.0"},
		"out outside dist": {"-root", root, "-version", "1.0.0", "-out", filepath.Join(root, "bin")},
		"out is dist":      {"-root", root, "-version", "1.0.0", "-out", filepath.Join(root, "dist")},
		"out escapes":      {"-root", root, "-version", "1.0.0", "-out", filepath.Join(root, "dist", "..", "cmd")},
		"bad base url":     {"-root", root, "-version", "1.0.0", "-base-url", "http://example.test"},
		"not the module":   {"-root", t.TempDir(), "-version", "1.0.0"},
		"extra args":       {"-root", root, "-version", "1.0.0", "extra"},
	}
	for name, args := range cases {
		if err := cli(args, &log); err == nil {
			t.Errorf("%s: cli(%q) = nil error", name, args)
		}
	}
}

func TestInsideDir(t *testing.T) {
	d := filepath.Join("r", "dist")
	for p, want := range map[string]bool{
		filepath.Join(d, "release"): true,
		filepath.Join(d, "a", "b"):  true,
		d:                           false,
		filepath.Join("r", "distx"): false,
		filepath.Join(d, "..", "x"): false,
		filepath.Join(d, "..dots"):  true,
	} {
		if got := insideDir(d, p); got != want {
			t.Errorf("insideDir(%q, %q) = %v, want %v", d, p, got, want)
		}
	}
}
