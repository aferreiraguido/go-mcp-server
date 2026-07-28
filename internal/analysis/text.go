// text.go — textual search over sources (complement to AST).
//
// Counterpart of `text_utils.rs` (rust-mcp-server): whole-word matching with
// boundary checking to avoid false positives (`add` inside
// `address`). Textual tools (search_source, find_references,
// function_callers) consume these helpers; semantic ones arrive in iter 3.
package analysis

import (
	"sort"
	"strings"
)

// TextMatch is a textual match: file, line (1-based) and content.
type TextMatch struct {
	File string
	Line int
	Text string
}

// SearchSubstring searches for lines containing `query` (plain substring).
// caseSensitive=false → case-insensitive search. Order: (File, Line).
func (p *Project) SearchSubstring(query string, caseSensitive bool, maxResults int) []TextMatch {
	var out []TextMatch
	needle := query
	if !caseSensitive {
		needle = strings.ToLower(query)
	}
	for _, f := range p.Files {
		for i, line := range f.Lines {
			hay := line
			if !caseSensitive {
				hay = strings.ToLower(line)
			}
			if strings.Contains(hay, needle) {
				out = append(out, TextMatch{File: f.RelPath, Line: i + 1, Text: strings.TrimSpace(line)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	if maxResults > 0 && len(out) > maxResults {
		out = out[:maxResults]
	}
	return out
}

// FindWholeWordRefs searches for lines with a whole-word occurrence of `symbol`
// (boundaries: no alphanumeric or underscore on either side).
// If requireParen is true, only matches if followed by `(` (calls).
// Order: (File, Line).
func (p *Project) FindWholeWordRefs(symbol string, requireParen bool, maxResults int) []TextMatch {
	var out []TextMatch
	for _, f := range p.Files {
		for i, line := range f.Lines {
			if hasWholeWord(line, symbol, requireParen) {
				out = append(out, TextMatch{File: f.RelPath, Line: i + 1, Text: strings.TrimSpace(line)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	if maxResults > 0 && len(out) > maxResults {
		out = out[:maxResults]
	}
	return out
}

// hasWholeWord indicates whether `line` contains `word` with identifier boundaries.
func hasWholeWord(line, word string, requireParen bool) bool {
	if word == "" {
		return false
	}
	for start := 0; ; {
		idx := strings.Index(line[start:], word)
		if idx < 0 {
			return false
		}
		idx += start
		end := idx + len(word)

		leftOK := idx == 0 || !isIdentChar(line[idx-1])
		rightOK := end >= len(line) || !isIdentChar(line[end])
		parenOK := !requireParen || (end < len(line) && line[end] == '(')

		if leftOK && rightOK && parenOK {
			return true
		}
		start = end
		if start >= len(line) {
			return false
		}
	}
}

// isIdentChar: letter, digit or underscore (Go identifier characters).
func isIdentChar(c byte) bool {
	return c == '_' ||
		(c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9')
}
