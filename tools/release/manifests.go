package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// manifest is one package-manager file generated from a template under
// packaging/.
type manifest struct {
	Template string // relative to the repository root
	Output   string // relative to the output directory
	Targets  []target
	JSON     bool // validate the result as JSON
}

var manifests = []manifest{
	{
		Template: "packaging/homebrew/devcade.rb.tmpl",
		Output:   "homebrew/devcade.rb",
		Targets:  []target{{"darwin", "amd64"}, {"darwin", "arm64"}, {"linux", "amd64"}, {"linux", "arm64"}},
	},
	{
		Template: "packaging/scoop/devcade.json.tmpl",
		Output:   "scoop/devcade.json",
		Targets:  []target{{"windows", "amd64"}, {"windows", "arm64"}},
		JSON:     true,
	},
}

// writeManifests renders every manifest from the parsed SHA256SUMS and writes
// it below out. It returns the written paths.
func writeManifests(root, out, version, baseURL string, sums map[string]string) ([]string, error) {
	var written []string
	for _, m := range manifests {
		tmpl, err := readText(filepath.Join(root, filepath.FromSlash(m.Template)))
		if err != nil {
			return nil, err
		}
		b, err := renderManifest(m, string(tmpl), version, baseURL, sums)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", m.Template, err)
		}
		p := filepath.Join(out, filepath.FromSlash(m.Output))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, b, 0o644); err != nil {
			return nil, err
		}
		written = append(written, p)
	}
	return written, nil
}

// renderManifest fills a template. {{url "os" "arch"}} expands to the
// archive's download URL and {{sha256 "os" "arch"}} to its checksum from
// SHA256SUMS; a missing checksum is an error, so a generated manifest never
// carries a placeholder hash.
func renderManifest(m manifest, text, version, baseURL string, sums map[string]string) ([]byte, error) {
	used := map[target]bool{}
	lookup := func(os, arch string) (target, error) {
		t := target{os, arch}
		for _, want := range m.Targets {
			if want == t {
				used[t] = true
				return t, nil
			}
		}
		return t, fmt.Errorf("target %s is not part of this manifest", t)
	}
	funcs := template.FuncMap{
		"url": func(os, arch string) (string, error) {
			t, err := lookup(os, arch)
			if err != nil {
				return "", err
			}
			return baseURL + "/" + archiveName(version, t), nil
		},
		"sha256": func(os, arch string) (string, error) {
			t, err := lookup(os, arch)
			if err != nil {
				return "", err
			}
			name := archiveName(version, t)
			sum, ok := sums[name]
			if !ok {
				return "", fmt.Errorf("%s has no entry for %s", sumsName, name)
			}
			return sum, nil
		},
	}
	tmpl, err := template.New(m.Template).Funcs(funcs).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	data := struct{ Version, BaseURL string }{version, baseURL}
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	for _, t := range m.Targets {
		if !used[t] {
			return nil, fmt.Errorf("template never references target %s", t)
		}
	}
	out := buf.Bytes()
	if bytes.Contains(out, []byte("{{")) || strings.Contains(strings.ToUpper(string(out)), "PLACEHOLDER") {
		return nil, fmt.Errorf("rendered output still contains template markers")
	}
	if m.JSON && !json.Valid(out) {
		return nil, fmt.Errorf("rendered output is not valid JSON")
	}
	return out, nil
}
