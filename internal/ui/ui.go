// Package ui contains localization and the three terminal palettes.
package ui

import (
	_ "embed"
	"encoding/json"
	"github.com/cagridursun/devcade/internal/engine"
	"strings"
)

//go:embed messages.json
var messagesJSON []byte
var messages = func() map[string][4]string {
	var m map[string][4]string
	if err := json.Unmarshal(messagesJSON, &m); err != nil {
		panic(err)
	}
	return m
}()
var Languages = []string{"en", "tr", "es", "nl", "fr"}
var LanguageNames = []string{"English", "Türkçe", "Español", "Nederlands", "Français"}
var Themes = []string{"mono", "midnight", "colorful"}
var ThemeNames = []string{"Black / white", "Midnight", "Colorful"}

func Translate(lang, s string) string {
	i := -1
	for n, l := range Languages[1:] {
		if lang == l {
			i = n
			break
		}
	}
	if i < 0 {
		return s
	}
	if row, ok := messages[s]; ok && row[i] != "" {
		return row[i]
	}
	return s
}

type Canvas struct {
	engine.Canvas
	Language string
}

func (c Canvas) Translate(s string) string { return Translate(c.Language, s) }
func (c Canvas) Text(x, y int, s string, color engine.Color) {
	c.Canvas.Text(x, y, c.Translate(s), color)
}
func Clip(s string, n int) string {
	r := []rune(s)
	if n <= 0 {
		return ""
	}
	if len(r) > n {
		return string(r[:n])
	}
	return s
}
func Wrap(s string, width int) []string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(s) {
		if len([]rune(line))+1+len([]rune(word)) > width && line != "" {
			lines = append(lines, line)
			line = ""
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	if line != "" {
		lines = append(lines, line)
	}
	return lines
}
