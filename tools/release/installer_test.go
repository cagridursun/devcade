package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These tests run packaging/install/install.sh and install.ps1 against local
// fixture releases (no network): fake binaries packed with the real archive
// code plus a SHA256SUMS file from checksumFile.

const fixtureVersion = "9.9.9-fixture.1"

func fixtureBinary(tg target) []byte {
	return []byte("#!/bin/sh\n# " + modulePath + " fixture " + tg.String() + "\necho devcade " + fixtureVersion + "\n")
}

// writeFixtureRelease writes all six archives and SHA256SUMS into a new
// temporary directory and returns it.
func writeFixtureRelease(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	var names []string
	for _, tg := range targets {
		b, err := packageArchive(tg, fixtureBinary(tg), []byte(fakeReadme), []byte(fakeNotice), testMTime)
		if err != nil {
			t.Fatal(err)
		}
		name := archiveName(fixtureVersion, tg)
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	sums, err := checksumFile(dir, names)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, sumsName), sums, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// installCase describes one installer run.
type installCase struct {
	name    string
	os      string // fake uname -s (sh) ; unused by ps1
	arch    string // fake uname -m (sh) or -Arch (ps1, "" = detect)
	args    []string
	prepare func(t *testing.T, rel, installDir string) // may edit the fixture
	wantErr string                                     // "" means success
	want    target                                     // installed binary on success
	check   func(t *testing.T, installDir string)
}

func corruptSums(t *testing.T, rel, archive string) {
	b, err := os.ReadFile(filepath.Join(rel, sumsName))
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if strings.HasSuffix(line, "  "+archive) {
			line = strings.Repeat("0", 64) + "  " + archive
		}
		out = append(out, line)
	}
	if err := os.WriteFile(filepath.Join(rel, sumsName), []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func truncate(t *testing.T, path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b[:len(b)/2], 0o644); err != nil {
		t.Fatal(err)
	}
}

func dropSumsEntry(t *testing.T, rel, archive string) {
	b, _ := os.ReadFile(filepath.Join(rel, sumsName))
	var keep []string
	for _, line := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
		if !strings.HasSuffix(line, "  "+archive) {
			keep = append(keep, line)
		}
	}
	os.WriteFile(filepath.Join(rel, sumsName), []byte(strings.Join(keep, "\n")+"\n"), 0o644)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
}

func readOr(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return string(b)
}

func assertNotInstalled(binName string) func(t *testing.T, dir string) {
	return func(t *testing.T, dir string) {
		if _, err := os.Stat(filepath.Join(dir, binName)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s was installed (err %v)", binName, err)
		}
	}
}

func assertContent(binName, want string) func(t *testing.T, dir string) {
	return func(t *testing.T, dir string) {
		if got := readOr(filepath.Join(dir, binName)); got != want {
			t.Errorf("%s = %q, want %q", binName, got, want)
		}
	}
}

// assertOnly fails if dir holds anything besides the named files (for
// example a leftover staging file).
func assertOnly(t *testing.T, dir string, names ...string) {
	t.Helper()
	es, _ := os.ReadDir(dir)
	allowed := map[string]bool{}
	for _, n := range names {
		allowed[n] = true
	}
	for _, e := range es {
		if !allowed[e.Name()] {
			t.Errorf("unexpected file %s in %s", e.Name(), dir)
		}
	}
}

func shPath(p string) string { return filepath.ToSlash(p) }

func TestInstallShFixtures(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("skipping install.sh tests: no sh in PATH")
	}
	for _, tool := range []string{"tar", "gzip", "awk", "grep", "mktemp"} {
		if err := exec.Command(sh, "-c", "command -v "+tool).Run(); err != nil {
			t.Skipf("skipping install.sh tests: %s not available to sh", tool)
		}
	}
	if exec.Command(sh, "-c", "command -v sha256sum || command -v shasum || command -v openssl").Run() != nil {
		t.Skip("skipping install.sh tests: no SHA-256 tool available to sh")
	}
	hasCurl := exec.Command(sh, "-c", "command -v curl").Run() == nil
	script := repoFile(t, "packaging/install/install.sh")

	linuxAMD := target{"linux", "amd64"}
	linuxARM := target{"linux", "arm64"}
	darwinARM := target{"darwin", "arm64"}
	cases := []installCase{
		{name: "linux x86_64 installs", os: "Linux", arch: "x86_64", want: linuxAMD},
		{name: "linux aarch64 installs arm64", os: "Linux", arch: "aarch64", want: linuxARM},
		{name: "macOS arm64 installs", os: "Darwin", arch: "arm64", want: darwinARM},
		{name: "leading v in version is accepted", os: "Linux", arch: "x86_64", args: []string{"--version", "v" + fixtureVersion}, want: linuxAMD},
		{name: "explicit os and arch override detection", os: "FreeBSD", arch: "riscv64", args: []string{"--os", "linux", "--arch", "arm64"}, want: linuxARM},
		{name: "unsupported architecture", os: "Linux", arch: "riscv64", wantErr: "unsupported architecture: riscv64", check: assertNotInstalled("devcade")},
		{name: "unsupported architecture override", os: "Linux", arch: "x86_64", args: []string{"--arch", "386"}, wantErr: "unsupported architecture: 386"},
		{name: "unsupported operating system", os: "FreeBSD", arch: "x86_64", wantErr: "unsupported operating system: FreeBSD"},
		{name: "windows shell is pointed at install.ps1", os: "MINGW64_NT-10.0", arch: "x86_64", wantErr: "use install.ps1"},
		{name: "invalid version", os: "Linux", arch: "x86_64", args: []string{"--version", "1.0"}, wantErr: "invalid version"},
		{name: "plain http is refused", os: "Linux", arch: "x86_64", args: []string{"--base-url", "http://example.invalid/x"}, wantErr: "only https:// and file://"},
		{
			name: "checksum mismatch", os: "Linux", arch: "x86_64",
			prepare: func(t *testing.T, rel, _ string) { corruptSums(t, rel, archiveName(fixtureVersion, linuxAMD)) },
			wantErr: "checksum mismatch", check: assertNotInstalled("devcade"),
		},
		{
			name: "interrupted download (truncated archive)", os: "Linux", arch: "x86_64",
			prepare: func(t *testing.T, rel, _ string) {
				truncate(t, filepath.Join(rel, archiveName(fixtureVersion, linuxAMD)))
			},
			wantErr: "checksum mismatch", check: assertNotInstalled("devcade"),
		},
		{
			name: "missing archive", os: "Linux", arch: "x86_64",
			prepare: func(t *testing.T, rel, _ string) {
				os.Remove(filepath.Join(rel, archiveName(fixtureVersion, linuxAMD)))
			},
			wantErr: "download failed", check: assertNotInstalled("devcade"),
		},
		{
			name: "missing SHA256SUMS", os: "Linux", arch: "x86_64",
			prepare: func(t *testing.T, rel, _ string) { os.Remove(filepath.Join(rel, sumsName)) },
			wantErr: "download failed", check: assertNotInstalled("devcade"),
		},
		{
			name: "archive not listed in SHA256SUMS", os: "Linux", arch: "x86_64",
			prepare: func(t *testing.T, rel, _ string) { dropSumsEntry(t, rel, archiveName(fixtureVersion, linuxAMD)) },
			wantErr: "has 0 entries", check: assertNotInstalled("devcade"),
		},
		{
			name: "refuses to overwrite a foreign file", os: "Linux", arch: "x86_64",
			prepare: func(t *testing.T, _, dir string) {
				writeFile(t, filepath.Join(dir, "devcade"), "someone else's tool\n")
			},
			wantErr: "not a DevCade binary", check: assertContent("devcade", "someone else's tool\n"),
		},
		{
			name: "--force overwrites a foreign file", os: "Linux", arch: "x86_64", args: []string{"--force"},
			prepare: func(t *testing.T, _, dir string) {
				writeFile(t, filepath.Join(dir, "devcade"), "someone else's tool\n")
			},
			want: linuxAMD,
		},
		{
			name: "upgrades an existing devcade", os: "Linux", arch: "x86_64",
			prepare: func(t *testing.T, _, dir string) {
				writeFile(t, filepath.Join(dir, "devcade"), "old "+modulePath+" build\n")
			},
			want: linuxAMD,
		},
		{
			name: "refuses a directory in the way", os: "Linux", arch: "x86_64", args: []string{"--force"},
			prepare: func(t *testing.T, _, dir string) { os.MkdirAll(filepath.Join(dir, "devcade"), 0o755) },
			wantErr: "is a directory",
		},
	}
	if hasCurl {
		cases = append(cases,
			installCase{name: "file URL via curl", os: "Linux", arch: "x86_64", args: []string{"--base-url", "FILEURL"}, want: linuxAMD},
			installCase{
				name: "file URL via curl, missing archive", os: "Linux", arch: "x86_64", args: []string{"--base-url", "FILEURL"},
				prepare: func(t *testing.T, rel, _ string) {
					os.Remove(filepath.Join(rel, archiveName(fixtureVersion, linuxAMD)))
				},
				wantErr: "download failed", check: assertNotInstalled("devcade"),
			},
		)
	} else {
		t.Log("curl not available: file:// cases skipped")
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rel := writeFixtureRelease(t)
			installDir := filepath.Join(t.TempDir(), "bin")
			tmpDir := t.TempDir()
			fakeBin := t.TempDir()
			writeFile(t, filepath.Join(fakeBin, "uname"), "#!/bin/sh\ncase \"$1\" in\n-m) printf '%s\\n' \"$FAKE_UNAME_M\" ;;\n*) printf '%s\\n' \"$FAKE_UNAME_S\" ;;\nesac\n")
			if tc.prepare != nil {
				tc.prepare(t, rel, installDir)
			}
			base := shPath(rel)
			args := []string{shPath(script), "--version", fixtureVersion, "--base-url", base, "--install-dir", shPath(installDir)}
			for _, a := range tc.args {
				if a == "FILEURL" {
					a = "file://" + base
					if !strings.HasPrefix(base, "/") {
						a = "file:///" + base
					}
				}
				args = append(args, a)
			}
			cmd := exec.Command(sh, args...)
			cmd.Env = append(cleanEnv(),
				"PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"),
				"FAKE_UNAME_S="+tc.os, "FAKE_UNAME_M="+tc.arch,
				"TMPDIR="+shPath(tmpDir), "HOME="+shPath(t.TempDir()))
			out, err := cmd.CombinedOutput()
			checkRun(t, tc, out, err, installDir, "devcade")
			if es, _ := os.ReadDir(tmpDir); len(es) != 0 {
				t.Errorf("temporary files left behind in TMPDIR: %v", es)
			}
			if tc.wantErr == "" && runtime.GOOS != "windows" {
				fi, err := os.Stat(filepath.Join(installDir, "devcade"))
				if err != nil || fi.Mode().Perm() != 0o755 {
					t.Errorf("installed mode = %v, %v; want 0755", fi.Mode(), err)
				}
			}
		})
	}
}

func cleanEnv() []string {
	var env []string
	for _, kv := range os.Environ() {
		k, _, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(k) {
		case "PATH", "TMPDIR", "TMP", "TEMP", "HOME":
			continue
		}
		if strings.HasPrefix(strings.ToUpper(k), "DEVCADE_") {
			continue
		}
		env = append(env, kv)
	}
	return env
}

func checkRun(t *testing.T, tc installCase, out []byte, err error, installDir, binName string) {
	t.Helper()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("could not run installer: %v\n%s", err, out)
	}
	if tc.wantErr == "" {
		if err != nil {
			t.Fatalf("installer failed: %v\n%s", err, out)
		}
		assertContent(binName, string(fixtureBinary(tc.want)))(t, installDir)
		if binName == "devcade" {
			assertOnly(t, installDir, binName)
		} else {
			assertOnly(t, installDir, binName, readmeName, noticeName)
		}
	} else {
		if err == nil || exitErr.ExitCode() != 1 {
			t.Fatalf("installer exit = %v, want status 1\n%s", err, out)
		}
		if !bytes.Contains(out, []byte(tc.wantErr)) {
			t.Errorf("output lacks %q:\n%s", tc.wantErr, out)
		}
	}
	if tc.check != nil {
		tc.check(t, installDir)
	}
	if t.Failed() {
		t.Logf("installer output:\n%s", out)
	}
}

func TestInstallPs1Fixtures(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("skipping install.ps1 tests: Windows PowerShell is only exercised on Windows")
	}
	ps, err := exec.LookPath("powershell")
	if err != nil {
		t.Skip("skipping install.ps1 tests: powershell not in PATH")
	}
	script := repoFile(t, "packaging/install/install.ps1")
	winAMD := target{"windows", "amd64"}
	winARM := target{"windows", "arm64"}
	detected := winAMD
	if runtime.GOARCH == "arm64" {
		detected = winARM
	}
	exe := func(dir string) string { return filepath.Join(dir, "devcade.exe") }
	cases := []installCase{
		{name: "detected architecture installs", want: detected},
		{name: "arm64 installs", arch: "arm64", want: winARM},
		{name: "x64 alias installs amd64", arch: "x64", want: winAMD},
		{name: "unsupported architecture", arch: "x86", wantErr: "unsupported architecture: x86", check: assertNotInstalled("devcade.exe")},
		{name: "invalid version", args: []string{"-Version", "1.0"}, wantErr: "invalid version"},
		{name: "plain http is refused", args: []string{"-BaseUrl", "http://example.invalid/x"}, wantErr: "only https://"},
		{
			name: "checksum mismatch", arch: "amd64",
			prepare: func(t *testing.T, rel, _ string) { corruptSums(t, rel, archiveName(fixtureVersion, winAMD)) },
			wantErr: "checksum mismatch", check: assertNotInstalled("devcade.exe"),
		},
		{
			name: "interrupted download (truncated archive)", arch: "amd64",
			prepare: func(t *testing.T, rel, _ string) {
				truncate(t, filepath.Join(rel, archiveName(fixtureVersion, winAMD)))
			},
			wantErr: "checksum mismatch", check: assertNotInstalled("devcade.exe"),
		},
		{
			name: "missing archive", arch: "amd64",
			prepare: func(t *testing.T, rel, _ string) { os.Remove(filepath.Join(rel, archiveName(fixtureVersion, winAMD))) },
			wantErr: "download failed", check: assertNotInstalled("devcade.exe"),
		},
		{
			name: "archive not listed in SHA256SUMS", arch: "amd64",
			prepare: func(t *testing.T, rel, _ string) { dropSumsEntry(t, rel, archiveName(fixtureVersion, winAMD)) },
			wantErr: "has 0 entries", check: assertNotInstalled("devcade.exe"),
		},
		{
			name: "refuses to overwrite a foreign file", arch: "amd64",
			prepare: func(t *testing.T, _, dir string) { writeFile(t, exe(dir), "someone else's tool\n") },
			wantErr: "not a DevCade binary", check: assertContent("devcade.exe", "someone else's tool\n"),
		},
		{
			name: "-Force overwrites a foreign file", arch: "amd64", args: []string{"-Force"},
			prepare: func(t *testing.T, _, dir string) { writeFile(t, exe(dir), "someone else's tool\n") },
			want:    winAMD,
		},
		{
			name: "upgrades an existing devcade", arch: "amd64",
			prepare: func(t *testing.T, _, dir string) { writeFile(t, exe(dir), "old "+modulePath+" build\n") },
			want:    winAMD,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rel := writeFixtureRelease(t)
			installDir := filepath.Join(t.TempDir(), "devcade")
			tmpDir := t.TempDir()
			if tc.prepare != nil {
				tc.prepare(t, rel, installDir)
			}
			args := []string{"-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", script, "-InstallDir", installDir}
			// tc.args may override -Version and -BaseUrl (PowerShell rejects
			// a parameter given twice).
			overridden := strings.Join(tc.args, " ")
			if !strings.Contains(overridden, "-Version ") {
				args = append(args, "-Version", fixtureVersion)
			}
			if !strings.Contains(overridden, "-BaseUrl ") {
				args = append(args, "-BaseUrl", rel)
			}
			if tc.arch != "" {
				args = append(args, "-Arch", tc.arch)
			}
			args = append(args, tc.args...)
			cmd := exec.Command(ps, args...)
			cmd.Env = append(cleanEnv(), "PATH="+os.Getenv("PATH"), "TMP="+tmpDir, "TEMP="+tmpDir)
			out, err := cmd.CombinedOutput()
			checkRun(t, tc, out, err, installDir, "devcade.exe")
			if tc.wantErr == "" {
				if got := readOr(filepath.Join(installDir, noticeName)); got != fakeNotice {
					t.Errorf("%s = %q", noticeName, got)
				}
			}
			if es, _ := os.ReadDir(tmpDir); len(es) != 0 {
				t.Errorf("temporary files left behind in TEMP: %v", es)
			}
		})
	}
}
