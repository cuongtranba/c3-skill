package cmd

import (
	"strings"
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/store"
)

func swapCiteSnippet(t *testing.T, handle, snippet string) string {
	t.Helper()
	i := strings.Index(handle, ` "`)
	if i < 0 {
		t.Fatalf("handle has no snippet to swap: %s", handle)
	}
	return handle[:i] + ` "` + snippet + `"`
}

func adrTopoBodyWithEvidence(rows [][4]string) string {
	b := "# Sample ADR\n\n## Affected Topology\n\n" +
		"| Entity | Type | Why affected | Evidence | Governance review |\n" +
		"|---|---|---|---|---|\n"
	for _, r := range rows {
		b += "| " + r[0] + " | " + r[1] + " | " + r[2] + " | " + r[3] + " | N.A - test |\n"
	}
	return b
}

func adrRelatedBody(rows [][4]string) string {
	b := "# Sample ADR\n\n## Compliance Refs\n\n" +
		"| Ref | Why required | Evidence | Action |\n" +
		"|---|---|---|---|\n"
	for _, r := range rows {
		b += "| " + r[0] + " | " + r[1] + " | " + r[2] + " | " + r[3] + " |\n"
	}
	return b
}

func hintForIssueMentioning(t *testing.T, issues []Issue, needle string) string {
	t.Helper()
	for _, issue := range issues {
		if strings.Contains(issue.Message, needle) {
			return issue.Hint
		}
	}
	t.Fatalf("no issue mentioning %q, got %#v", needle, issues)
	return ""
}

func TestADREvidence_SnippetMismatchIsNotReportedAsStale(t *testing.T) {
	s := createRichDBFixture(t)
	currentAnchorWrongSnippet := swapCiteSnippet(t, testCitationForEntity(t, s, "c3-1"), "a snippet nobody wrote")

	issues := validateADREvidence(s, "Affected Topology", "c3-1", currentAnchorWrongSnippet, "warning", false)
	if len(issues) != 1 {
		t.Fatalf("expected exactly one issue, got %+v", issues)
	}
	if strings.Contains(issues[0].Message, "stale") {
		t.Fatalf("a fresh hash with a bad snippet must not be reported as stale: %q", issues[0].Message)
	}
	if !strings.Contains(issues[0].Message, "snippet") {
		t.Fatalf("expected the message to name the snippet, got %q", issues[0].Message)
	}
	if !strings.Contains(issues[0].Message, "a snippet nobody wrote") {
		t.Fatalf("expected the cited snippet to be echoed, got %q", issues[0].Message)
	}
	if !strings.Contains(issues[0].Message, "Serve API requests") {
		t.Fatalf("expected the node's actual leading text, got %q", issues[0].Message)
	}
	if strings.Contains(issues[0].Hint, "refresh the handle") {
		t.Fatalf("re-fetching returns the same handle; hint must not say refresh: %q", issues[0].Hint)
	}
	if !strings.Contains(issues[0].Hint, "snippet") {
		t.Fatalf("expected a re-copy/drop-the-snippet hint, got %q", issues[0].Hint)
	}
}

func TestADREvidence_UnknownHashKeepsStaleMessage(t *testing.T) {
	s := createRichDBFixture(t)
	stale := staleHashHandle(t, testCitationForEntity(t, s, "c3-1"))

	issues := validateADREvidence(s, "Affected Topology", "c3-1", stale, "warning", false)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "stale cite") {
		t.Fatalf("expected the stale-cite message when no node seals to the hash, got %+v", issues)
	}
	if !strings.Contains(issues[0].Hint, "refresh") {
		t.Fatalf("expected the refresh hint for a genuinely stale hash, got %q", issues[0].Hint)
	}
}

func TestCitationColumn_SnippetMismatchIsNotReportedAsStale(t *testing.T) {
	s := createRichDBFixture(t)
	currentAnchorWrongSnippet := swapCiteSnippet(t, testCitationForEntity(t, s, "c3-1"), "a snippet nobody wrote")

	issues := validateCitationColumnValue(currentAnchorWrongSnippet, mustEntity(t, s, "c3-1"), citeOpts(s))
	if len(issues) != 1 {
		t.Fatalf("expected exactly one issue, got %+v", issues)
	}
	if strings.Contains(issues[0].Message, "stale") {
		t.Fatalf("a fresh hash with a bad snippet must not be reported as stale: %q", issues[0].Message)
	}
	if !strings.Contains(issues[0].Message, "a snippet nobody wrote") ||
		!strings.Contains(issues[0].Message, "Serve API requests") {
		t.Fatalf("expected cited snippet vs actual node text, got %q", issues[0].Message)
	}
	if strings.Contains(issues[0].Hint, "refresh the handle") {
		t.Fatalf("hint must not say refresh for a snippet mismatch: %q", issues[0].Hint)
	}
}

func TestCitationColumn_SnippetMismatchExcerptsInsteadOfDumpingTheNode(t *testing.T) {
	s := createRichDBFixture(t)
	longSnippet := strings.Repeat("x", 400)
	handle := swapCiteSnippet(t, testCitationForEntity(t, s, "c3-1"), longSnippet)

	issues := validateCitationColumnValue(handle, mustEntity(t, s, "c3-1"), citeOpts(s))
	if len(issues) != 1 {
		t.Fatalf("expected exactly one issue, got %+v", issues)
	}
	if strings.Contains(issues[0].Message, longSnippet) {
		t.Fatalf("message must truncate the cited snippet, got %q", issues[0].Message)
	}
	if len(issues[0].Message) > 400 {
		t.Fatalf("message must stay to one readable line, got %d chars: %q", len(issues[0].Message), issues[0].Message)
	}
}

func TestADREvidence_EntityRootHandleHintNamesSection(t *testing.T) {
	s := createRichDBFixture(t)
	entity := mustEntity(t, s, "c3-1")
	entityRootHandle := entity.ID + "@v1:sha256:" + strings.Repeat("a", 64)

	issues := validateADREvidence(s, "Affected Topology", "c3-1", entityRootHandle, "warning", false)
	if len(issues) != 1 {
		t.Fatalf("expected the invalid-citation issue, got %+v", issues)
	}
	if !strings.Contains(issues[0].Hint, "--section") {
		t.Fatalf("hint must name --section <name> (bare --cite emits the rejected root form), got %q", issues[0].Hint)
	}
}

func TestCitationColumn_EntityRootHandleHintNamesSection(t *testing.T) {
	s := createRichDBFixture(t)
	entityRootHandle := "c3-1@v1:sha256:" + strings.Repeat("a", 64)

	issues := validateCitationColumnValue(entityRootHandle, mustEntity(t, s, "c3-1"), citeOpts(s))
	if len(issues) != 1 {
		t.Fatalf("expected the invalid-citation issue, got %+v", issues)
	}
	if !strings.Contains(issues[0].Hint, "--section") {
		t.Fatalf("hint must name --section <name>, got %q", issues[0].Hint)
	}
}

func TestAffectedTopology_ReportsWhyAndEvidenceTogether(t *testing.T) {
	s := createRichDBFixture(t)
	stale := staleHashHandle(t, testCitationForEntity(t, s, "c3-1"))
	body := adrTopoBodyWithEvidence([][4]string{{"c3-1", "container", "", stale}})

	_, issues := parseADRAffectedTopology(s, body, "warning", adrSchemaHint())
	if !hasIssue(issues, "must explain why it is affected") {
		t.Fatalf("expected the missing-why finding, got %#v", issues)
	}
	if !hasIssue(issues, "stale cite") {
		t.Fatalf("a blank Why must not hide the row's Evidence defect, got %#v", issues)
	}
}

func TestAffectedTopology_ReportsTypeMismatchAndEvidenceTogether(t *testing.T) {
	s := createRichDBFixture(t)
	stale := staleHashHandle(t, testCitationForEntity(t, s, "c3-1"))
	body := adrTopoBodyWithEvidence([][4]string{{"c3-1", "component", "boundary moves", stale}})

	_, issues := parseADRAffectedTopology(s, body, "warning", adrSchemaHint())
	if !hasIssue(issues, "type mismatch") {
		t.Fatalf("expected the type-mismatch finding, got %#v", issues)
	}
	if !hasIssue(issues, "stale cite") {
		t.Fatalf("a type mismatch must not hide the row's Evidence defect, got %#v", issues)
	}
}

func TestAffectedTopology_UnknownEntityStillReportsEvidenceCellShape(t *testing.T) {
	s := createRichDBFixture(t)
	body := adrTopoBodyWithEvidence([][4]string{{"c3-999", "component", "", "not a handle at all"}})

	_, issues := parseADRAffectedTopology(s, body, "warning", adrSchemaHint())
	if !hasIssue(issues, "unknown entity") {
		t.Fatalf("expected the unknown-entity finding, got %#v", issues)
	}
	if !hasIssue(issues, "must explain why it is affected") {
		t.Fatalf("expected the missing-why finding alongside, got %#v", issues)
	}
	if !hasIssue(issues, "invalid Evidence citation") {
		t.Fatalf("expected the Evidence shape finding alongside, got %#v", issues)
	}
}

func TestRelatedTable_ReportsWhyAndEvidenceTogether(t *testing.T) {
	s := createRichDBFixture(t)
	stale := staleHashHandle(t, testCitationForEntity(t, s, "ref-jwt"))
	body := adrRelatedBody([][4]string{{"ref-jwt", "", stale, "comply"}})

	_, issues := parseADRRelatedTable(s, body, "Compliance Refs", "Ref", "ref", "warning", adrSchemaHint())
	if !hasIssue(issues, "must explain why compliance/review is required") {
		t.Fatalf("expected the missing-why finding, got %#v", issues)
	}
	if !hasIssue(issues, "stale cite") {
		t.Fatalf("a blank Why must not hide the row's Evidence defect, got %#v", issues)
	}
}

func TestRelatedTable_ReportsTypeMismatchAndEvidenceTogether(t *testing.T) {
	s := createRichDBFixture(t)
	stale := staleHashHandle(t, testCitationForEntity(t, s, "c3-1"))
	body := adrRelatedBody([][4]string{{"c3-1", "container is not a ref", stale, "comply"}})

	_, issues := parseADRRelatedTable(s, body, "Compliance Refs", "Ref", "ref", "warning", adrSchemaHint())
	if !hasIssue(issues, "type mismatch") {
		t.Fatalf("expected the type-mismatch finding, got %#v", issues)
	}
	if !hasIssue(issues, "stale cite") {
		t.Fatalf("a type mismatch must not hide the row's Evidence defect, got %#v", issues)
	}
}

func TestAffectedTopology_BrokenRowDoesNotSuppressTheNextRowsFindings(t *testing.T) {
	s := createRichDBFixture(t)
	body := adrTopoBodyWithEvidence([][4]string{
		{"c3-999", "component", "", "not a handle at all"},
		{"c3-2", "container", "", "also not a handle"},
	})

	_, issues := parseADRAffectedTopology(s, body, "warning", adrSchemaHint())
	whys := 0
	for _, issue := range issues {
		if strings.Contains(issue.Message, "must explain why it is affected") {
			whys++
		}
	}
	if whys != 2 {
		t.Fatalf("expected one missing-why finding per row, got %d: %#v", whys, issues)
	}
}

func TestFreeFormCells_ReportTypedFindingsInsteadOfPanicking(t *testing.T) {
	s := createRichDBFixture(t)
	freeForm := "c3-3 shared (kanna-system-prompt.ts)"

	run := func(name string, fn func() []Issue) {
		t.Helper()
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("%s panicked on a free-form cell: %v", name, r)
			}
		}()
		if issues := fn(); len(issues) == 0 {
			t.Fatalf("%s: expected a typed validation error for a free-form cell, got none", name)
		}
	}

	run("Affected Topology", func() []Issue {
		body := adrTopoBodyWithEvidence([][4]string{{freeForm, "component", "shared prompt moves", "N.A - none"}})
		_, issues := parseADRAffectedTopology(s, body, "warning", adrSchemaHint())
		return issues
	})
	run("Compliance Refs", func() []Issue {
		body := adrRelatedBody([][4]string{{"ref-jwt (see kanna-system-prompt.ts)", "token policy applies", "N.A - none", "comply"}})
		_, issues := parseADRRelatedTable(s, body, "Compliance Refs", "Ref", "ref", "warning", adrSchemaHint())
		return issues
	})
}

func TestAffectedTopology_FreeFormEntityCellNamesTheBareID(t *testing.T) {
	s := createRichDBFixture(t)
	decoratedKnownEntityCell := "c3-1 shared (kanna-system-prompt.ts)"
	body := adrTopoBodyWithEvidence([][4]string{{decoratedKnownEntityCell, "container", "boundary moves", "N.A - none"}})

	_, issues := parseADRAffectedTopology(s, body, "warning", adrSchemaHint())
	if hasIssue(issues, "unknown entity") {
		t.Fatalf("c3-1 is known; the cell is decorated, not unknown: %#v", issues)
	}
	if !hasIssue(issues, "only the bare id") {
		t.Fatalf("expected a bare-id finding, got %#v", issues)
	}
	if hint := hintForIssueMentioning(t, issues, "only the bare id"); !strings.Contains(hint, "use just c3-1") {
		t.Fatalf("expected the hint to name the id that was found, got %q", hint)
	}
}

func TestAffectedTopology_UnrecognisedFreeFormStaysUnknownEntity(t *testing.T) {
	s := createRichDBFixture(t)
	decoratedUnknownEntityCell := "c3-3 shared (kanna-system-prompt.ts)"
	body := adrTopoBodyWithEvidence([][4]string{{decoratedUnknownEntityCell, "component", "shared prompt moves", "N.A - none"}})

	_, issues := parseADRAffectedTopology(s, body, "warning", adrSchemaHint())
	if !hasIssue(issues, "unknown entity") {
		t.Fatalf("no token resolves; expected the unknown-entity finding, got %#v", issues)
	}
}

func TestRelatedTable_FreeFormRefCellNamesTheBareID(t *testing.T) {
	s := createRichDBFixture(t)
	decoratedKnownRefCell := "ref-jwt (see kanna-system-prompt.ts)"
	body := adrRelatedBody([][4]string{{decoratedKnownRefCell, "token policy applies", "N.A - none", "comply"}})

	_, issues := parseADRRelatedTable(s, body, "Compliance Refs", "Ref", "ref", "warning", adrSchemaHint())
	if hasIssue(issues, "unknown ref") {
		t.Fatalf("ref-jwt is known; the cell is decorated, not unknown: %#v", issues)
	}
	if !hasIssue(issues, "only the bare id") {
		t.Fatalf("expected a bare-id finding, got %#v", issues)
	}
	if hint := hintForIssueMentioning(t, issues, "only the bare id"); !strings.Contains(hint, "use just ref-jwt") {
		t.Fatalf("expected the hint to name the id that was found, got %q", hint)
	}
}

func TestResolveEvidenceNode_SeparatesStaleHashFromSnippetMismatch(t *testing.T) {
	s := createRichDBFixture(t)
	handle := testCitationForEntity(t, s, "c3-1")
	nodes, err := s.NodesForEntity("c3-1")
	if err != nil {
		t.Fatalf("nodes for c3-1: %v", err)
	}
	var target *store.Node
	for _, n := range nodes {
		if strings.Contains(handle, n.Hash) {
			target = n
			break
		}
	}
	if target == nil {
		t.Fatalf("could not locate the cited node for %s", handle)
	}

	if outcome, _ := resolveEvidenceNode(s, "c3-1", target.ID, target.Hash, ""); outcome != evidenceNodeOK {
		t.Fatalf("a current hash must match, got %v", outcome)
	}
	outcome, node := resolveEvidenceNode(s, "c3-1", target.ID, target.Hash, "a snippet nobody wrote")
	if outcome != evidenceNodeSnippetMismatch {
		t.Fatalf("a current hash with a bad snippet must report a snippet mismatch, got %v", outcome)
	}
	if node == nil || node.Hash != target.Hash {
		t.Fatalf("expected the hash-matching node back so the message can show its text, got %+v", node)
	}
	if outcome, _ := resolveEvidenceNode(s, "c3-1", target.ID, strings.Repeat("0", 64), ""); outcome != evidenceNodeHashUnknown {
		t.Fatalf("a hash no node seals to must report hash-unknown, got %v", outcome)
	}
}
