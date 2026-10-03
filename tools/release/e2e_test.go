package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestEndToEndRelease cross-compiles all six targets with the real pipeline.
// It takes a while, so it only runs with DEVCADE_RELEASE_E2E=1:
//
//	DEVCADE_RELEASE_E2E=1 go test -count=1 -run EndToEnd -v ./tools/release
func TestEndToEndRelease(t *testing.T) {
	if os.Getenv("DEVCADE_RELEASE_E2E") != "1" {
		t.Skip("set DEVCADE_RELEASE_E2E=1 to cross-compile all six targets")
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	const v = "0.0.0-e2e.1"
	out := filepath.Join(root, "dist", fmt.Sprintf("e2e-test-%d", os.Getpid()))
	t.Cleanup(func() { os.RemoveAll(out) })
	var log bytes.Buffer
	if err := cli([]string{"-root", root, "-version", v, "-out", out}, &log); err != nil {
		t.Fatalf("release: %v\n%s", err, log.String())
	}

	sumsData, err := os.ReadFile(filepath.Join(out, sumsName))
	if err != nil {
		t.Fatal(err)
	}
	sums, err := parseChecksums(sumsData)
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != len(targets)+2 {
		t.Errorf("SHA256SUMS has %d entries, want %d", len(sums), len(targets)+2)
	}
	for name, want := range sums {
		b, err := os.ReadFile(filepath.Join(out, name))
		if err != nil {
			t.Fatal(err)
		}
		got := sha256.Sum256(b)
		if hex.EncodeToString(got[:]) != want {
			t.Errorf("%s: checksum mismatch", name)
		}
	}
	for _, tg := range targets {
		b, err := os.ReadFile(filepath.Join(out, archiveName(v, tg)))
		if err != nil {
			t.Fatal(err)
		}
		var es []archived
		if tg.OS == "windows" {
			es = readZip(t, b)
		} else {
			es = readTarGz(t, b)
		}
		if len(es) != 3 || es[2].Name != tg.binaryName() || es[2].Mode != 0o755 {
			t.Errorf("%s: entries %v", tg, names(es))
		}
		if !strings.Contains(es[2].Data, modulePath) {
			t.Errorf("%s: binary lacks the %s marker the installers look for", tg, modulePath)
		}
		if tg.OS == runtime.GOOS && tg.Arch == runtime.GOARCH {
			bin := filepath.Join(t.TempDir(), tg.binaryName())
			if err := os.WriteFile(bin, []byte(es[2].Data), 0o755); err != nil {
				t.Fatal(err)
			}
			got, err := exec.Command(bin, "--version").Output()
			if err != nil || strings.TrimSpace(string(got)) != "devcade "+v {
				t.Errorf("--version = %q, %v", got, err)
			}
			for _, arg := range []string{"--help", "list"} {
				if outp, err := exec.Command(bin, arg).CombinedOutput(); err != nil || len(outp) == 0 {
					t.Errorf("%s: %v\n%s", arg, err, outp)
				}
			}
		}
	}
	for _, p := range []string{"homebrew/devcade.rb", "scoop/devcade.json"} {
		b, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		for name, sum := range sums {
			if strings.Contains(string(b), name) && !strings.Contains(string(b), sum) {
				t.Errorf("%s references %s without its checksum", p, name)
			}
		}
	}
}

func names(es []archived) []string {
	var s []string
	for _, e := range es {
		s = append(s, fmt.Sprintf("%s %o", e.Name, e.Mode))
	}
	return s
}
