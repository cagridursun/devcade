package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// checksumFile returns SHA256SUMS content for the named files in dir, in the
// format sha256sum(1) writes and `sha256sum -c` reads: one
// "<64 lower-case hex digits><two spaces><file name>\n" line per file, sorted
// by name. The hashes are computed from the bytes on disk.
func checksumFile(dir string, names []string) ([]byte, error) {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	var buf bytes.Buffer
	for i, name := range sorted {
		if i > 0 && sorted[i-1] == name {
			return nil, fmt.Errorf("duplicate file %q", name)
		}
		if !sumNameRE.MatchString(name) {
			return nil, fmt.Errorf("file name %q is not allowed in %s", name, sumsName)
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(b)
		fmt.Fprintf(&buf, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	return buf.Bytes(), nil
}

var (
	sumNameRE = regexp.MustCompile(`^[A-Za-z0-9._+-]+$`)
	sumLineRE = regexp.MustCompile(`^([0-9a-f]{64}) [ *]([A-Za-z0-9._+-]+)$`)
)

// parseChecksums parses a SHA256SUMS file strictly: every line must be a
// valid entry, names must be unique, and at least one entry must exist.
func parseChecksums(b []byte) (map[string]string, error) {
	text := strings.TrimSuffix(string(b), "\n")
	if text == "" {
		return nil, fmt.Errorf("%s is empty", sumsName)
	}
	sums := map[string]string{}
	for i, line := range strings.Split(text, "\n") {
		m := sumLineRE.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("%s line %d: malformed entry %q", sumsName, i+1, line)
		}
		if _, dup := sums[m[2]]; dup {
			return nil, fmt.Errorf("%s line %d: duplicate entry for %s", sumsName, i+1, m[2])
		}
		sums[m[2]] = m[1]
	}
	return sums, nil
}
