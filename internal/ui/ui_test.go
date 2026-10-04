package ui

import (
	"github.com/cagridursun/devcade/internal/engine"
	"github.com/cagridursun/devcade/internal/profile"
	"regexp"
	"strings"
	"testing"
)

func TestEveryTranslationHasMatchingFormattingAndSupportedGlyphs(t *testing.T) {
	spec := regexp.MustCompile(`%[-+0-9.]*[sdf]`)
	for key, row := range messages {
		for i, s := range row {
			if s == "" {
				t.Errorf("missing %s: %s", Languages[i+1], key)
			}
			if strings.Join(spec.FindAllString(key, -1), "|") != strings.Join(spec.FindAllString(s, -1), "|") {
				t.Errorf("format mismatch %s: %q", key, s)
			}
			for _, r := range s {
				if engine.Printable(r) != r {
					t.Errorf("unsupported glyph %q in %q", r, s)
				}
			}
			if strings.HasPrefix(key, " ") && len([]rune(s)) > 80 {
				t.Errorf("footer too wide (%d): %s", len([]rune(s)), s)
			}
		}
	}
}

func TestFormattingNeverTranslatesPlayerNames(t *testing.T) {
	c := Canvas{Language: "tr"}
	if got := engine.Format(c, "You: #%d  %s  %d", 1, "ready", 100); got != "Sen: #1  ready  100" {
		t.Fatal(got)
	}
	if !profile.ValidUsername("ready") {
		t.Fatal("fixture")
	}
}
func TestDefaultAndFallback(t *testing.T) {
	if Translate("en", "Settings") != "Settings" || Translate("unknown", "Settings") != "Settings" {
		t.Fatal("fallback")
	}
	if got := Translate("tr", "Choose your username"); got != "Kullanıcı adını seç" {
		t.Fatal(got)
	}
	lines := Wrap("a b c", 3)
	if len(lines) != 2 || lines[0] != "a b" || lines[1] != "c" {
		t.Fatal(lines)
	}
}
