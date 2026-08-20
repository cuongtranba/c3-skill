package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/content"
	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// A retire is the one change that destroys its own evidence. These tests pin the
// resting state such a decision must reach: the change-unit's applied retire
// patch stands in for the After-cite the Affected Topology row can no longer
// carry, so the doc latches accepted -> done instead of stalling forever.

const retireTargetID = "c3-301"

type retireUnitFixture struct {
	store *store.Store
	c3Dir string
	adr   *store.Entity
	cite  string // the pre-retire Evidence handle for retireTargetID
}

// seedRetireUnit builds a change-unit whose single patch retires a component:
// a legacy container/component pair, an `accepted` ADR whose Affected Topology
// names the component with a live cite, and the retire patch on disk. Nothing is
// applied yet — each test drives `change apply` itself.
func seedRetireUnit(t *testing.T) retireUnitFixture {
	t.Helper()
	s := createRichDBFixture(t)

	legacy := []*store.Entity{
		{ID: "c3-3", Type: "container", Title: "legacy", Slug: "legacy", ParentID: "c3-0", Boundary: "app", Status: "active", Metadata: "{}"},
		{ID: retireTargetID, Type: "component", Title: "tui", Slug: "tui", Category: "feature", ParentID: "c3-3", Status: "active", Metadata: "{}"},
	}
	for _, e := range legacy {
		if err := s.InsertEntity(e); err != nil {
			t.Fatalf("seed %s: %v", e.ID, err)
		}
	}
	bodies := map[string]string{
		"c3-0": "# TestProject\n\n## Goal\n\nTest the system.\n\n## Containers\n\n| ID | Name | Boundary | Goal |\n|----|------|----------|------|\n| c3-1 | api | service | Serve API requests |\n| c3-2 | web | app | Web frontend |\n| c3-3 | legacy | app | Host the retiring surfaces |\n",
		"c3-3": "# legacy\n\n## Goal\n\nHost the retiring surfaces.\n\n" +
			"## Components\n\n| ID | Name | Category | Status | Goal Contribution |\n|----|------|----------|--------|-------------------|\n| c3-301 | tui | feature | active | Terminal UI |\n\n" +
			"## Responsibilities\n\nCarry the legacy surfaces that are on their way out of the model.\n",
		retireTargetID: strictComponentBody("tui", "Render the legacy terminal interface for the retiring surface."),
	}
	for id, body := range bodies {
		if err := content.WriteEntity(s, id, body); err != nil {
			t.Fatalf("seed body %s: %v", id, err)
		}
	}

	cite := testCitationForEntity(t, s, retireTargetID)

	adr := &store.Entity{ID: "adr-20260819-retire-tui", Type: "adr", Title: "Retire the legacy TUI", Slug: "retire-tui", Status: "open", Date: "20260819", Metadata: "{}"}
	if err := s.InsertEntity(adr); err != nil {
		t.Fatalf("seed adr: %v", err)
	}
	if err := content.WriteEntity(s, adr.ID, retireADRBody(cite)); err != nil {
		t.Fatalf("seed adr body: %v", err)
	}
	if err := s.SetEntityStatus(adr.ID, "accepted"); err != nil {
		t.Fatalf("accept adr: %v", err)
	}
	adr, err := s.GetEntity(adr.ID)
	if err != nil {
		t.Fatalf("reget adr: %v", err)
	}

	target, err := s.GetEntity(retireTargetID)
	if err != nil {
		t.Fatalf("reget %s: %v", retireTargetID, err)
	}
	c3Dir := filepath.Join(t.TempDir(), ".c3")
	unitDir := changeUnitDir(c3Dir, adr.ID)
	if err := os.MkdirAll(unitDir, 0o755); err != nil {
		t.Fatalf("mkdir unit: %v", err)
	}
	patch := fmt.Sprintf("---\ntarget: %s\nscope: retire\nbase: %s@v%d:sha256:%s\n---\n",
		retireTargetID, retireTargetID, target.Version, target.RootMerkle)
	if err := os.WriteFile(filepath.Join(unitDir, "01-retire-tui.patch.md"), []byte(patch), 0o644); err != nil {
		t.Fatalf("write patch: %v", err)
	}

	return retireUnitFixture{store: s, c3Dir: c3Dir, adr: adr, cite: cite}
}

// retireADRBody renders the ADR with the given Evidence cell for the retired
// component. Its ancestors take the documented N.A escape so the change-set is
// top-down complete.
func retireADRBody(evidence string) string {
	return "# Retire the legacy TUI\n\n" +
		"## Goal\n\nRemove the unmaintained legacy terminal interface from the model.\n\n" +
		"## Context\n\nThe TUI has no owner and no traffic; keeping it documented implies support we do not provide.\n\n" +
		"## Decision\n\nRetire c3-301 and let the legacy container carry no terminal surface.\n\n" +
		"## Affected Topology\n\n" +
		"| Entity | Type | Why affected | Evidence | Governance review |\n" +
		"| --- | --- | --- | --- | --- |\n" +
		"| c3-0 | system | N.A - ancestor named only to complete the top-down descent | N.A - ancestor row carries no delta | N.A - no review owed |\n" +
		"| c3-3 | container | N.A - ancestor named only to complete the top-down descent | N.A - ancestor row carries no delta | N.A - no review owed |\n" +
		"| c3-301 | component | Documents the legacy TUI surface, which this decision deletes | " + evidence + " | Retire |\n\n" +
		"## Verification\n\n" +
		"| Check | Result |\n" +
		"| --- | --- |\n" +
		"| c3x check --include-adr --only adr-20260819-retire-tui | clean |\n"
}

func (f retireUnitFixture) apply(t *testing.T) {
	t.Helper()
	var buf bytes.Buffer
	if err := RunChangeApply(ChangeApplyOptions{Store: f.store, C3Dir: f.c3Dir, UnitID: f.adr.ID}, &buf); err != nil {
		t.Fatalf("change apply must land a retire the unit's own ADR documents: %v\n%s", err, buf.String())
	}
	if _, err := f.store.GetEntity(retireTargetID); err == nil {
		t.Fatalf("apply did not retire %s", retireTargetID)
	}
}

func (f retireUnitFixture) rewriteADR(t *testing.T, evidence string) string {
	t.Helper()
	body := retireADRBody(evidence)
	if err := content.WriteEntity(f.store, f.adr.ID, body); err != nil {
		t.Fatalf("rewrite adr body: %v", err)
	}
	return body
}

// TestRetireUnit_ApplyAllowsTheDecidingDocToNameItsTarget — a change doc that
// declares `affects` on the fact it retires must not read as a dangling citer of
// its own decision, or the unit can never land.
func TestRetireUnit_ApplyAllowsTheDecidingDocToNameItsTarget(t *testing.T) {
	f := seedRetireUnit(t)
	if err := f.store.AddRelationship(&store.Relationship{FromID: f.adr.ID, ToID: retireTargetID, RelType: "affects"}); err != nil {
		t.Fatalf("seed affects edge: %v", err)
	}
	f.apply(t)
}

// TestRetireUnit_CheckClearsTheRetiredRow — after apply, neither the stale cite
// nor the N.A escape may warn: the applied retire patch is the row's proof.
func TestRetireUnit_CheckClearsTheRetiredRow(t *testing.T) {
	for _, tc := range []struct {
		name     string
		evidence func(f retireUnitFixture) string
	}{
		{"stale pre-retire cite", func(f retireUnitFixture) string { return f.cite }},
		{"N.A escape", func(retireUnitFixture) string { return "N.A - retired by this unit" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := seedRetireUnit(t)
			f.apply(t)
			body := f.rewriteADR(t, tc.evidence(f))

			for _, issue := range validateADRCoverage(f.store, f.c3Dir, f.adr.ID, body, "warning") {
				if strings.Contains(issue.Message, retireTargetID) {
					t.Errorf("retired row still warns: %s", issue.Message)
				}
			}
		})
	}
}

// TestRetireUnit_CheckFixLatchesToDone — the end-to-end resting state the issue
// asks for: `check --fix --include-adr` actualizes accepted -> done.
func TestRetireUnit_CheckFixLatchesToDone(t *testing.T) {
	for _, tc := range []struct {
		name     string
		evidence func(f retireUnitFixture) string
	}{
		{"stale pre-retire cite", func(f retireUnitFixture) string { return f.cite }},
		{"N.A escape", func(retireUnitFixture) string { return "N.A - retired by this unit" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := seedRetireUnit(t)
			f.apply(t)
			f.rewriteADR(t, tc.evidence(f))

			var buf bytes.Buffer
			if err := RunCheckV2(CheckOptions{
				Store: f.store, C3Dir: f.c3Dir, IncludeADR: true, Fix: true, Only: []string{f.adr.ID},
			}, &buf); err != nil {
				t.Fatalf("check --fix: %v\n%s", err, buf.String())
			}

			got, err := f.store.GetEntity(f.adr.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Status != "done" {
				t.Fatalf("retire ADR stuck at %q; check output:\n%s", got.Status, buf.String())
			}
		})
	}
}

// TestRetireUnit_UnappliedRetireStillOwesALiveCite — the discharge is the LANDED
// retire, not the mere intent to retire. While the fact is still there, the row
// obeys the ordinary evidence contract.
func TestRetireUnit_UnappliedRetireStillOwesALiveCite(t *testing.T) {
	f := seedRetireUnit(t)
	body := f.rewriteADR(t, "N.A - retired by this unit")

	issues := validateADRCoverage(f.store, f.c3Dir, f.adr.ID, body, "warning")
	for _, issue := range issues {
		if strings.Contains(issue.Message, "must cite current C3 evidence, not N.A") {
			return
		}
	}
	t.Fatalf("an unapplied retire must not discharge the row; issues=%+v", issues)
}

// TestRetireUnit_ApplyHintOmitsRetiredTargets — `apply` must not send the author
// to re-cite a fact it just deleted.
func TestRetireUnit_ApplyHintOmitsRetiredTargets(t *testing.T) {
	f := seedRetireUnit(t)
	var buf bytes.Buffer
	if err := RunChangeApply(ChangeApplyOptions{Store: f.store, C3Dir: f.c3Dir, UnitID: f.adr.ID}, &buf); err != nil {
		t.Fatalf("change apply: %v\n%s", err, buf.String())
	}
	out := buf.String()
	if strings.Contains(out, nodeCiteCommand(retireTargetID)) {
		t.Errorf("apply told the author to re-cite the retired %s:\n%s", retireTargetID, out)
	}
	if !strings.Contains(out, "check --include-adr --only "+f.adr.ID) {
		t.Errorf("apply dropped the close-the-unit hint:\n%s", out)
	}
}

// TestRetireUnit_ProseMentionDoesNotDischargeAnotherRow — the discharge belongs
// to the row that NAMES the retired fact, not to any row that mentions it.
func TestRetireUnit_ProseMentionDoesNotDischargeAnotherRow(t *testing.T) {
	f := seedRetireUnit(t)
	f.apply(t)

	body := strings.Replace(retireADRBody("N.A - retired by this unit"),
		"| c3-3 | container | N.A - ancestor named only to complete the top-down descent | N.A - ancestor row carries no delta | N.A - no review owed |",
		"| c3-3 | container | Loses the c3-301 surface it used to host | N.A - not re-cited | architecture review |", 1)
	if err := content.WriteEntity(f.store, f.adr.ID, body); err != nil {
		t.Fatalf("rewrite adr body: %v", err)
	}

	for _, issue := range validateADRCoverage(f.store, f.c3Dir, f.adr.ID, body, "warning") {
		if strings.Contains(issue.Message, "c3-3") && strings.Contains(issue.Message, "not N.A") {
			return
		}
	}
	t.Fatal("a surviving fact that merely mentions the retired id still owes a live cite")
}

// TestAutoDone_NAEvidenceIsNotAnUnresolvedCite — an "N.A - <reason>" cell is a
// declared non-cite, so it neither blocks the latch nor counts as proof.
func TestAutoDone_NAEvidenceIsNotAnUnresolvedCite(t *testing.T) {
	s := createRichDBFixture(t)
	entity, body := seedAcceptedPRD(t, s, testCitationForEntity(t, s, "c3-1"), "N.A - no story landed yet")

	flipped, unresolved := autoDoneLatch(s, "", entity, body, true)
	if !flipped {
		t.Fatalf("N.A cell must not read as an unresolved cite; unresolved=%+v", unresolved)
	}
}
