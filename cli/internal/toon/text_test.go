package toon

import (
	"strings"
	"testing"
)

func TestMarshalObjectText_MultilineBecomesBlock(t *testing.T) {
	type doc struct {
		ID   string `json:"id"`
		Body string `json:"body"`
		Next string `json:"next"`
	}
	v := doc{
		ID:   "adr-20260724-slash-picker-local-only",
		Body: "# Heading\n\n| col | col |\n| --- | --- |\n| a   | b   |",
		Next: "tail",
	}

	out, err := MarshalObjectText(v)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `\n`) {
		t.Fatalf("text output must not contain literal backslash-n:\n%s", out)
	}
	if !strings.Contains(out, "id: adr-20260724-slash-picker-local-only\n") {
		t.Errorf("single-line values should stay key: value\ngot:\n%s", out)
	}
	if !strings.Contains(out, "body: |-\n") {
		t.Errorf("multi-line value should open a block scalar\ngot:\n%s", out)
	}
	if !strings.Contains(out, "\n  # Heading\n") {
		t.Errorf("block lines should be indented by two spaces\ngot:\n%s", out)
	}
	if !strings.Contains(out, "\n  | col | col |\n") {
		t.Errorf("markdown table rows should survive as real lines\ngot:\n%s", out)
	}
	if !strings.Contains(out, "\nnext: tail\n") {
		t.Errorf("field after the block must be unambiguously dedented\ngot:\n%s", out)
	}
}

func TestMarshalObjectText_BlankLinesCarryNoIndent(t *testing.T) {
	type doc struct {
		Body string `json:"body"`
	}
	out, err := MarshalObjectText(doc{Body: "a\n\nb"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "  a\n\n  b\n") {
		t.Errorf("blank line inside a block must not be padded with spaces\ngot:%q", out)
	}
}

func TestMarshalObjectText_ChompingIndicators(t *testing.T) {
	type doc struct {
		Body string `json:"body"`
	}
	tests := []struct {
		name string
		body string
		want string
	}{
		{"no trailing newline strips", "a\nb", "body: |-\n  a\n  b\n"},
		{"one trailing newline clips", "a\nb\n", "body: |\n  a\n  b\n"},
		{"extra trailing newlines kept", "a\nb\n\n", "body: |+\n  a\n  b\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := MarshalObjectText(doc{Body: tt.body})
			if err != nil {
				t.Fatal(err)
			}
			if out != tt.want {
				t.Errorf("want %q\ngot  %q", tt.want, out)
			}
		})
	}
}

func TestMarshalObjectText_SingleLineMatchesTOON(t *testing.T) {
	type row struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Count int    `json:"count"`
	}
	v := row{ID: "c3-1", Title: "API, Gateway", Count: 5}

	text, err := MarshalObjectText(v)
	if err != nil {
		t.Fatal(err)
	}
	toonOut, err := MarshalObject(v)
	if err != nil {
		t.Fatal(err)
	}
	if text != toonOut {
		t.Errorf("without multi-line values text mode must match TOON exactly\ntoon: %q\ntext: %q", toonOut, text)
	}
}

func TestMarshalObject_MultilineStillEscapedInTOON(t *testing.T) {
	type doc struct {
		Body string `json:"body"`
	}
	out, err := MarshalObject(doc{Body: "a\nb"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "body: \"a\\nb\"\n" {
		t.Fatalf("TOON must keep escaping multi-line strings (non-regression), got %q", out)
	}
}

func TestMarshalObjectText_NestedStructAndMapIndentBlocks(t *testing.T) {
	type inner struct {
		Body string `json:"body"`
	}
	type outer struct {
		Inner inner             `json:"inner"`
		Meta  map[string]string `json:"meta"`
	}
	out, err := MarshalObjectText(outer{
		Inner: inner{Body: "x\ny"},
		Meta:  map[string]string{"note": "p\nq"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "inner:\n  body: |-\n    x\n    y\n") {
		t.Errorf("nested struct block should indent relative to its key\ngot:\n%s", out)
	}
	if !strings.Contains(out, "meta:\n  note: |-\n    p\n    q\n") {
		t.Errorf("map value block should indent relative to its key\ngot:\n%s", out)
	}
}

func TestMarshalAnyText_SliceElementBlocks(t *testing.T) {
	out, err := MarshalAnyText([]string{"a\nb", "c"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, `\n`) {
		t.Fatalf("text list output must not contain literal backslash-n:\n%s", out)
	}
	if !strings.Contains(out, "  - |-\n    a\n    b\n") {
		t.Errorf("multi-line list element should become a block\ngot:\n%s", out)
	}
	if !strings.Contains(out, "  - c\n") {
		t.Errorf("single-line list element should stay inline\ngot:\n%s", out)
	}
}

func TestMarshalTableText_KeepsGrid(t *testing.T) {
	type item struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	items := []item{{ID: "c3-1", Title: "api"}}

	text, err := MarshalTableText("entities", items, []string{"id", "title"})
	if err != nil {
		t.Fatal(err)
	}
	toonOut, err := MarshalTable("entities", items, []string{"id", "title"})
	if err != nil {
		t.Fatal(err)
	}
	if text != toonOut {
		t.Errorf("table rows are positional; text mode must keep the TOON grid\ntoon: %q\ntext: %q", toonOut, text)
	}
}

func TestMarshalObjectText_CarriageReturnStaysEscaped(t *testing.T) {
	type doc struct {
		Body string `json:"body"`
	}
	out, err := MarshalObjectText(doc{Body: "a\r\nb"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `\r`) {
		t.Errorf("CRLF content is not block-safe and must stay quoted\ngot: %q", out)
	}
}
