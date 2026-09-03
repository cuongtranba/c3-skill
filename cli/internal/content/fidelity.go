package content

import (
	"fmt"
	"strings"

	"github.com/lagz0ne/c3-design/cli/internal/markdown"
)

// fenceState tracks open code fences so callers can skip code content correctly.
// It returns the opener's backtick count (>0 = inside fence, 0 = outside).
// Both backtick and tilde fences are tracked; the character must match.
type fenceState struct {
	len  int  // backtick/tilde count of the opener; 0 = not in fence
	char byte // '`' or '~'
}

func (f *fenceState) update(trimmed string) {
	if f.len == 0 {
		if n := leadingRunLen(trimmed, '`'); n >= 3 {
			f.len, f.char = n, '`'
		} else if n := leadingRunLen(trimmed, '~'); n >= 3 {
			f.len, f.char = n, '~'
		}
		return
	}
	// Inside a fence: close only when the line is a run of the same character
	// with length ≥ the opener's and no info string.
	if n := leadingRunLen(trimmed, f.char); n >= f.len && strings.TrimSpace(trimmed[n:]) == "" {
		f.len, f.char = 0, 0
	}
}

func leadingRunLen(s string, ch byte) int {
	n := 0
	for n < len(s) && s[n] == ch {
		n++
	}
	return n
}

// CheckUnclosedFence returns an error when body contains a code fence that is
// never closed. An unclosed fence causes goldmark to swallow every subsequent
// line as code content, so headings after the opener become unreachable
// sections and check passes on a document that no standard CommonMark parser
// can render correctly.
func CheckUnclosedFence(body string) error {
	var f fenceState
	for _, line := range strings.Split(body, "\n") {
		f.update(strings.TrimSpace(line))
	}
	if f.len > 0 {
		return fmt.Errorf("unclosed code fence (opened with %d %c backticks): every line after the opener is treated as code content by standard CommonMark parsers; close the fence or normalise it with 'c3x repair'",
			f.len, f.char)
	}
	return nil
}

// VerifyTableFidelity reports the first table row that would lose text on write.
//
// Markdown cuts a row to its header's column count, so a row carrying more cells
// than its header is silently truncated — and because a doc is re-serialized by
// ANY write that loads it, the apply that destroys the text is usually not the one
// that broke it, and need not target the damaged doc at all. Nothing downstream
// notices: the row simply comes back shorter. This check is the only thing
// standing between a broken escape and committed prose disappearing.
//
// The usual cause is a `\|` rewritten as `\ |`, which escapes a space instead of
// the pipe and hands the table an extra column.
func VerifyTableFidelity(body string) error {
	lines := strings.Split(body, "\n")
	var f fenceState

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		f.update(trimmed)
		// A separator row is what makes the line above it a header; it is the only
		// unambiguous marker of where a table starts.
		if f.len > 0 || i == 0 || !isSeparatorRow(trimmed) {
			continue
		}
		width := len(markdown.SplitRowCells(lines[i-1]))
		for _, row := range lines[i+1:] {
			row = strings.TrimSpace(row)
			if !strings.Contains(row, "|") {
				break // the table ended
			}
			if got := len(markdown.SplitRowCells(row)); got > width {
				return fmt.Errorf(
					"table row would lose %d cell(s) on write — it has %d columns but the table has %d: %q\n"+
						"a literal pipe inside a cell must be written \\| (no space between the backslash and the pipe)",
					got-width, got, width, row)
			}
		}
	}
	return nil
}

// isSeparatorRow reports whether line is a table's `| --- | --- |` rule.
func isSeparatorRow(line string) bool {
	if !strings.Contains(line, "|") || !strings.Contains(line, "-") {
		return false
	}
	for _, cell := range markdown.SplitRowCells(line) {
		if strings.Trim(cell, "-: ") != "" {
			return false
		}
	}
	return true
}
