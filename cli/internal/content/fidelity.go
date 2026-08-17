package content

import (
	"fmt"
	"strings"

	"github.com/lagz0ne/c3-design/cli/internal/markdown"
)

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
	inFence := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		// A separator row is what makes the line above it a header; it is the only
		// unambiguous marker of where a table starts.
		if inFence || i == 0 || !isSeparatorRow(trimmed) {
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
