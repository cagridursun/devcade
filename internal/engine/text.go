package engine

import "fmt"

// Format translates a format string before formatting. Arguments are data:
// usernames and already translated strings must never be translated implicitly.
// Plain game canvases remain English, while localized UI canvases opt in.
func Format(c Canvas, format string, args ...any) string {
	if t, ok := c.(interface{ Translate(string) string }); ok {
		format = t.Translate(format)
	}
	return fmt.Sprintf(format, args...)
}
