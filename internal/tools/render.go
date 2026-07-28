// render.go — shared Markdown helpers (counterpart of `render.rs` in rust).
package tools

import (
	"fmt"
	"strings"
)

// table renders a Markdown table. headers and rows come pre-formatted
// (cells with backticks where applicable). Empty rows → header+separator only.
func table(headers []string, rows [][]string) string {
	var b strings.Builder
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	b.WriteString("|" + strings.Repeat("---|", len(headers)) + "\n")
	for _, row := range rows {
		b.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	return b.String()
}

// loc formats a stable location: `file.go:42` in backticks.
func loc(file string, line int) string {
	return fmt.Sprintf("`%s:%d`", file, line)
}

// truncate cuts s to n runes adding "…" if exceeded.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

// codeBlock wraps content in a code fence with optional language.
func codeBlock(lang, content string) string {
	return "```" + lang + "\n" + content + "\n```\n"
}

// yesNo renders a bool as yes/no (Exported columns).
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// dash renders "-" when s is empty (optional cells).
func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
