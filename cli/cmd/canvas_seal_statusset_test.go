package cmd

import (
	"strings"
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/frontmatter"
)

func canonicalSealForDoc(t *testing.T, md string) string {
	t.Helper()
	fm, body := frontmatter.ParseFrontmatter(md)
	if fm == nil {
		t.Fatal("nil frontmatter")
	}
	return computeCanonicalSeal(canonicalDocFromParsedDoc(frontmatter.ParsedDoc{Frontmatter: fm, Body: body}))
}

// TestCanonicalSeal_CoversCanvasStatusSet — regression guard for the 11.6.x seal
// regression where canonicalDocFromParsedDoc dropped a canvas's list-valued
// status:, so the re-rendered canonical (and its seal) silently omitted status.
// The effect: every status-set canvas (adr/prd/atomic-design-change) failed
// RunImport's hard seal check, deadlocking cache bootstrap. The status set must
// be part of the sealed canonical form: two canvases differing ONLY in their
// status set must produce different seals.
func TestCanonicalSeal_CoversCanvasStatusSet(t *testing.T) {
	base := "---\nid: adr\ntype: canvas\ndescription: Decision record\n%s---\n\ndomain: software\nsections: []\n"
	withOpen := canonicalSealForDoc(t, strings.Replace(base, "%s", "status:\n    - open\n    - accepted\n", 1))
	withDone := canonicalSealForDoc(t, strings.Replace(base, "%s", "status:\n    - done\n    - superseded\n", 1))

	if withOpen == withDone {
		t.Fatalf("canvas status set must be covered by the seal: differing status sets produced identical seal %s", withOpen)
	}
}

// TestRenderCanonicalDoc_PreservesCanvasStatusSet — the canonical render used for
// sealing must emit a canvas's status: set; dropping it is what caused the seal
// regression above.
func TestRenderCanonicalDoc_PreservesCanvasStatusSet(t *testing.T) {
	md := "---\nid: adr\ntype: canvas\ndescription: Decision record\nstatus:\n    - open\n    - accepted\n    - done\n    - superseded\n---\n\ndomain: software\nsections: []\n"
	fm, body := frontmatter.ParseFrontmatter(md)
	if fm == nil {
		t.Fatal("nil frontmatter")
	}
	rendered := renderCanonicalDoc(canonicalDocFromParsedDoc(frontmatter.ParsedDoc{Frontmatter: fm, Body: body}), false)
	if !strings.Contains(rendered, "status:") {
		t.Fatalf("canonical render dropped the canvas status set:\n%s", rendered)
	}
	for _, want := range []string{"open", "accepted", "done", "superseded"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("canonical render missing status value %q:\n%s", want, rendered)
		}
	}
}
