package cmd

import (
	"strings"
	"testing"
)

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

	wantRefreshSignpost := []string{"ref-jwt", "--section", "--cite", "adr-1"}
	for _, want := range wantRefreshSignpost {
		if !strings.Contains(out, want) {
			t.Fatalf("apply output must point at the After-cite refresh with %q; got:\n%s", want, out)
		}
	}
}

func TestRunChangeApply_DryRunWritesNothingSoPointsAtNoRefresh(t *testing.T) {
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
