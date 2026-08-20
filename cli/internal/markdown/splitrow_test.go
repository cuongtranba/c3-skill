package markdown

import (
	"strings"
	"testing"
)

func TestSplitRowCells_KeepsEscapeInsideItsCell(t *testing.T) {
	cases := map[string][]string{
		"| one | x \\| y | three |":   {"one", "x \\| y", "three"},
		"one | x \\| y \\| z":         {"one", "x \\| y \\| z"},
		"| a \\| b |":                 {"a \\| b"},
		"| `proactive` \\| `user` | n |": {"`proactive` \\| `user`", "n"},
	}
	for line, want := range cases {
		got := SplitRowCells(line)
		if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
			t.Errorf("SplitRowCells(%q)\n got: %q\nwant: %q", line, got, want)
		}
	}
}

// A row whose final cell ends in an escaped pipe has no trailing delimiter to
// strip; taking one anyway eats the backslash and re-breaks the escape.
func TestSplitRowCells_KeepsTrailingEscapedPipe(t *testing.T) {
	got := SplitRowCells(`a | b \|`)
	want := []string{"a", `b \|`}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Errorf("SplitRowCells: got %q, want %q", got, want)
	}
}
