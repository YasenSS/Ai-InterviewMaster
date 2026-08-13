package ai

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ClipRunes keeps the start of text and records the original length. Trailing
// truncation is avoided so prompt delimiters and leading facts stay intact.
func ClipRunes(value string, maxRunes int) string {
	value = strings.TrimSpace(value)
	if maxRunes < 1 {
		return value
	}
	count := utf8.RuneCountInString(value)
	if count <= maxRunes {
		return value
	}
	keep := maxRunes
	note := fmt.Sprintf("\n[内容因长度限制截断，共 %d 字符]", count)
	noteRunes := utf8.RuneCountInString(note)
	if keep > noteRunes+8 {
		keep -= noteRunes
	}
	return string([]rune(value)[:keep]) + note
}

// CountRunes returns the rune length of trimmed text.
func CountRunes(value string) int {
	return utf8.RuneCountInString(strings.TrimSpace(value))
}
