package cmd

import (
	"strings"
	"testing"
)

// Issue #113 — `change apply` invalidates the change doc's own Evidence cites.
//
// That is by design: Affected Topology Evidence anchors the block the unit is
// about to rewrite, and the accepted→done latch requires those cites to resolve
// FRESH after the apply — which is what proves the change actually landed.
// Auto-refreshing them inside apply would make the latch prove nothing.
//
// What was missing is the signpost: apply said nothing, references/change.md
// stopped at `check`, and the author discovered the staleness as a wall of
// check failures with no documented recovery. Apply must point forward.

func TestRunChangeApply_PointsAtTheAfterCiteRefresh(t *testing.T) {
	s, c3Dir := openStoreC3(t)
	seedRef(t, s, "ref-jwt", "Standardize token verification so components do not reinvent it.", "Use RS256 signed JWTs verified against a shared public key.", "Old rationale about asymmetric signing here.")
	base := citeFor(t, s, "ref-jwt", "Old rationale")

	writePatch(t, c3Dir, "adr-1", "01-why.patch.md",
		"---\ntarget: ref-jwt\nscope: block\nbase: "+base+"\n---\nAsymmetric signing lets each service verify without holding the signing secret.\n")

	var buf strings.Builder
	if err := RunChangeApply(ChangeApplyOptions{Store: s, C3Dir: c3Dir, UnitID: "adr-1"}, &buf); err != nil {
		t.Fatalf("apply: %v\noutput: %s", err, buf.String())
	}
	out := buf.String()

	// It must name the patched target (that is where the refreshed handle comes
	// from), the command that yields a node handle, and the unit to re-check.
	for _, want := range []string{"ref-jwt", "--section", "--cite", "adr-1"} {
		if !strings.Contains(out, want) {
			t.Fatalf("apply output must point at the After-cite refresh with %q; got:\n%s", want, out)
		}
	}
}

// --dry-run performs no writes, so nothing has gone stale and there is nothing
// to refresh. Emitting the signpost there would be a false instruction.
func TestRunChangeApply_DryRunDoesNotPointAtRefresh(t *testing.T) {
	s, c3Dir := openStoreC3(t)
	seedRef(t, s, "ref-jwt", "Standardize token verification so components do not reinvent it.", "Use RS256 signed JWTs verified against a shared public key.", "Old rationale about asymmetric signing here.")
	base := citeFor(t, s, "ref-jwt", "Old rationale")

	writePatch(t, c3Dir, "adr-1", "01-why.patch.md",
		"---\ntarget: ref-jwt\nscope: block\nbase: "+base+"\n---\nAsymmetric signing lets each service verify without holding the signing secret.\n")

	var buf strings.Builder
	if err := RunChangeApply(ChangeApplyOptions{Store: s, C3Dir: c3Dir, UnitID: "adr-1", DryRun: true}, &buf); err != nil {
		t.Fatalf("dry-run apply: %v\noutput: %s", err, buf.String())
	}
	if strings.Contains(buf.String(), "--cite") {
		t.Fatalf("dry-run must not instruct a refresh — nothing was written:\n%s", buf.String())
	}
}
