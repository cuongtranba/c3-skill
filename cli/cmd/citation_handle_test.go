package cmd

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/lagz0ne/c3-design/cli/internal/store"
)

func emittedCitation(snippet string) string {
	entity := &store.Entity{ID: "c3-1", Version: 3}
	node := &store.Node{ID: 42, Hash: strings.Repeat("a", 64)}
	return nodeCitation(entity, node, snippet)
}

// A snippet is emitted with %q, so every quote/backslash arrives escaped. Parsing
// must unquote it back to the exact bytes the node carries — otherwise a cite of
// any line containing " or \ can never validate.
func TestParseCitationHandle_RoundTripsEscapedSnippets(t *testing.T) {
	snippets := map[string]string{
		"double quote": `the "exact" wording`,
		"backslash":    `path\to\thing`,
		"both":         `he said "C:\tmp" loudly`,
	}
	for name, snippet := range snippets {
		t.Run(name, func(t *testing.T) {
			handle, ok := parseCitationHandle(emittedCitation(snippet))
			if !ok {
				t.Fatalf("emitted handle must parse: %s", emittedCitation(snippet))
			}
			if handle.Snippet != snippet {
				t.Fatalf("snippet did not round-trip: got %q, want %q", handle.Snippet, snippet)
			}
			if handle.EntityID != "c3-1" || handle.NodeID != 42 || handle.Version != 3 {
				t.Fatalf("handle fields lost: %+v", handle)
			}
		})
	}
}

func TestParseCitationHandle_SnippetIsOptional(t *testing.T) {
	bare := "c3-1#n42@v3:sha256:" + strings.Repeat("a", 64)
	handle, ok := parseCitationHandle(bare)
	if !ok {
		t.Fatalf("a snippet-less handle must parse: %s", bare)
	}
	if handle.Snippet != "" {
		t.Fatalf("expected an empty snippet, got %q", handle.Snippet)
	}
}

func TestParseCitationHandle_RejectsUnparseableInput(t *testing.T) {
	if _, ok := parseCitationHandle("not a handle at all"); ok {
		t.Fatal("free-form text must not parse as a citation handle")
	}
}

func TestCitationSnippet_TruncatesOnRuneBoundaries(t *testing.T) {
	snippet := citationSnippet(strings.Repeat("é", 400))
	if !utf8.ValidString(snippet) {
		t.Fatalf("truncation split a multi-byte rune: %q", snippet)
	}
	if got := utf8.RuneCountInString(snippet); got != citationSnippetMaxRunes {
		t.Fatalf("expected %d runes, got %d", citationSnippetMaxRunes, got)
	}
}

func TestCiteExcerpt_TruncatesOnRuneBoundaries(t *testing.T) {
	excerpt := citeExcerpt(strings.Repeat("世", 400))
	if !utf8.ValidString(excerpt) {
		t.Fatalf("truncation split a multi-byte rune: %q", excerpt)
	}
	if !strings.HasSuffix(excerpt, "...") {
		t.Fatalf("expected a truncation marker, got %q", excerpt)
	}
	if got := utf8.RuneCountInString(strings.TrimSuffix(excerpt, "...")); got != citeExcerptMaxLen {
		t.Fatalf("expected %d runes before the marker, got %d", citeExcerptMaxLen, got)
	}
}
