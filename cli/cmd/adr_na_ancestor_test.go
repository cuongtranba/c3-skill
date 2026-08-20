package cmd

import (
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// seedRefScopedToWeb adds a ref reachable only by descending the system root, so
// the tests can tell an N.A ancestor row (no descent) from a real one (descent).
func seedRefScopedToWeb(t *testing.T, s *store.Store) {
	t.Helper()
	ref := &store.Entity{ID: "ref-web-styling", Type: "ref", Title: "Web Styling", Slug: "web-styling", Goal: "One styling system", Status: "active", Metadata: "{}"}
	if err := s.InsertEntity(ref); err != nil {
		t.Fatalf("seed ref-web-styling: %v", err)
	}
	if err := s.AddRelationship(&store.Relationship{FromID: "ref-web-styling", ToID: "c3-2", RelType: "scope"}); err != nil {
		t.Fatalf("scope ref-web-styling: %v", err)
	}
}

func adrBodyWithAncestorRows(t *testing.T, s *store.Store, systemWhy, systemEvidence string) string {
	t.Helper()
	return "# Sample ADR\n\n## Affected Topology\n\n" +
		"| Entity | Type | Why affected | Evidence | Governance review |\n|---|---|---|---|---|\n" +
		"| c3-0 | system | " + systemWhy + " | " + systemEvidence + " | N.A - test |\n" +
		"| c3-1 | container | N.A - container boundary unchanged | N.A - container boundary unchanged | N.A - test |\n" +
		"| c3-101 | component | auth behavior changes | " + testCitationForEntity(t, s, "c3-101") + " | auth review |\n\n" +
		"## Compliance Refs\n\n" +
		"| Ref | Why required | Evidence | Action |\n|---|---|---|---|\n" +
		"| ref-jwt | auth tokens are governed here | " + testCitationForEntity(t, s, "ref-jwt") + " | comply |\n" +
		"| ref-error-handling | the container's error shape is governed here | " + testCitationForEntity(t, s, "ref-error-handling") + " | comply |\n"
}

// An `N.A - <reason>` ancestor row is an escape hatch: it satisfies top-down
// completeness without becoming a coverage target, so it neither demands a fresh
// cite nor drags its whole subtree into the expected compliance set.
func TestAffectedTopology_NAAncestorRowIsAnEscapeHatch(t *testing.T) {
	s := createRichDBFixture(t)
	seedRefScopedToWeb(t, s)
	body := adrBodyWithAncestorRows(t, s, "N.A - system boundary unchanged", "N.A - system boundary unchanged")

	issues := validateADRCoverage(s, "", "", body, "warning")
	if len(issues) != 0 {
		t.Fatalf("an N.A ancestor row must cost nothing, got %#v", issues)
	}
}

// The ceremony cliff is unchanged for a row with a REAL reason: it becomes a
// target and owes its entire subtree's compliance rows.
func TestAffectedTopology_RealReasonAncestorStillDescends(t *testing.T) {
	s := createRichDBFixture(t)
	seedRefScopedToWeb(t, s)
	body := adrBodyWithAncestorRows(t, s, "the system boundary moves", testCitationForEntity(t, s, "c3-0"))

	issues := validateADRCoverage(s, "", "", body, "warning")
	if !hasIssue(issues, "ref-web-styling") {
		t.Fatalf("a real reason on the system row must still owe its descendants' compliance refs, got %#v", issues)
	}
}

// A BLANK Why cell is not an escape hatch — it is undischarged and must still be
// reported.
func TestAffectedTopology_BlankWhyIsNotAnEscapeHatch(t *testing.T) {
	s := createRichDBFixture(t)
	body := adrTopoBodyWithEvidence([][4]string{{"c3-1", "container", "", testCitationForEntity(t, s, "c3-1")}})

	_, issues := parseADRAffectedTopology(s, body, "warning", nil)
	if !hasIssue(issues, "must explain why it is affected") {
		t.Fatalf("a blank Why cell must still be reported, got %#v", issues)
	}
}
