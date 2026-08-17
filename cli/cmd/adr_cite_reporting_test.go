package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/content"
	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// anySection stands in wherever a validateColumn test exercises column typing
// alone, with no section-scoped ownership in play.
const anySection = "Any Section"

func TestADRLinkageOwnsCiteColumn(t *testing.T) {
	owned := []string{"Affected Topology", "Compliance Refs", "Compliance Rules"}
	for _, section := range owned {
		if !adrLinkageOwnsCiteColumn("adr", section, "Evidence") {
			t.Errorf("ADR linkage validates %s Evidence; the generic cite path must yield", section)
		}
	}
	if adrLinkageOwnsCiteColumn("adr", "Work Breakdown", "Evidence") {
		t.Error("ADR linkage does not validate Work Breakdown; the generic path must still run")
	}
	if adrLinkageOwnsCiteColumn("prd", "Affected Topology", "Evidence") {
		t.Error("only ADRs have the linkage path")
	}
	if adrLinkageOwnsCiteColumn("adr", "Affected Topology", "Governance review") {
		t.Error("only the Evidence column is owned")
	}
}

// citableNodeOf returns a node of entityID that a handle can legitimately cite.
func citableNodeOf(t *testing.T, s *store.Store, entityID string) *store.Node {
	t.Helper()
	nodes, err := s.NodesForEntity(entityID)
	if err != nil {
		t.Fatalf("nodes for %s: %v", entityID, err)
	}
	for _, n := range nodes {
		if isCitableNode(n) {
			return n
		}
	}
	t.Fatalf("no citable node for %s", entityID)
	return nil
}

func handleFor(t *testing.T, s *store.Store, citedEntity string, nodeID int64, hash string) string {
	t.Helper()
	return fmt.Sprintf("%s#n%d@v%d:sha256:%s", citedEntity, nodeID, mustEntity(t, s, citedEntity).Version, hash)
}

// Node ids are globally sequential and renumber on `change apply`, so a stale
// cite's node id routinely lands on a foreign document. That is a coincidence,
// not a cross-document citation: the real defect is the stale hash.
func TestADREvidence_RenumberedNodeIDDoesNotBlameForeignDocument(t *testing.T) {
	s := createRichDBFixture(t)
	foreign := citableNodeOf(t, s, "ref-jwt")
	renumbered := handleFor(t, s, "c3-1", foreign.ID, strings.Repeat("0", 64))

	issues := validateADREvidence(s, "Affected Topology", "c3-1", renumbered, "warning", false)
	if len(issues) != 1 {
		t.Fatalf("expected exactly one issue, got %+v", issues)
	}
	if strings.Contains(issues[0].Message, "cites node") {
		t.Fatalf("a renumbered node id must not be reported as a foreign citation: %q", issues[0].Message)
	}
	if !strings.Contains(issues[0].Message, "stale cite") {
		t.Fatalf("expected the stale-cite message, got %q", issues[0].Message)
	}
}

func TestCitationColumn_RenumberedNodeIDDoesNotBlameForeignDocument(t *testing.T) {
	s := createRichDBFixture(t)
	foreign := citableNodeOf(t, s, "ref-jwt")
	renumbered := handleFor(t, s, "c3-1", foreign.ID, strings.Repeat("0", 64))

	issues := validateCitationColumnValue(renumbered, mustEntity(t, s, "c3-1"), citeOpts(s))
	if len(issues) != 1 {
		t.Fatalf("expected exactly one issue, got %+v", issues)
	}
	if strings.Contains(issues[0].Message, "cites node") {
		t.Fatalf("a renumbered node id must not be reported as a foreign citation: %q", issues[0].Message)
	}
	if !strings.Contains(issues[0].Message, "stale node hash") {
		t.Fatalf("expected the stale-hash message, got %q", issues[0].Message)
	}
}

// A genuine cross-document cite — the named node really does carry the cited
// hash — must keep naming the document it came from.
func TestADREvidence_GenuineForeignCitationStillNamesTheDocument(t *testing.T) {
	s := createRichDBFixture(t)
	foreign := citableNodeOf(t, s, "ref-jwt")
	collided := handleFor(t, s, "c3-1", foreign.ID, foreign.Hash)

	issues := validateADREvidence(s, "Affected Topology", "c3-1", collided, "warning", false)
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "cites node") {
		t.Fatalf("expected the foreign-citation message, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "ref-jwt") {
		t.Fatalf("expected the owning document to be named, got %q", issues[0].Message)
	}
}

func TestCitationColumn_GenuineForeignCitationStillNamesTheDocument(t *testing.T) {
	s := createRichDBFixture(t)
	foreign := citableNodeOf(t, s, "ref-jwt")
	collided := handleFor(t, s, "c3-1", foreign.ID, foreign.Hash)

	issues := validateCitationColumnValue(collided, mustEntity(t, s, "c3-1"), citeOpts(s))
	if len(issues) != 1 || !strings.Contains(issues[0].Message, "cites node") {
		t.Fatalf("expected the foreign-citation message, got %+v", issues)
	}
	if !strings.Contains(issues[0].Message, "ref-jwt") {
		t.Fatalf("expected the owning document to be named, got %q", issues[0].Message)
	}
}

// The ADR Evidence column is validated by the generic cite path AND by the
// richer, section-scoped ADR linkage path. Only the ADR one may speak.
func TestRunCheck_ADREvidenceCiteDefectReportedOnce(t *testing.T) {
	s := createRichDBFixture(t)
	stale := staleHashHandle(t, testCitationForEntity(t, s, "c3-1"))
	body := "# Use Go for CLI\n\n" +
		"## Goal\n\nSwitch the CLI to Go.\n\n" +
		"## Context\n\nThe current CLI is slow to start.\n\n" +
		"## Decision\n\nRewrite the CLI in Go.\n\n" +
		"## Affected Topology\n\n" +
		"| Entity | Type | Why affected | Evidence | Governance review |\n|---|---|---|---|---|\n" +
		"| c3-1 | container | the API container is rewritten | " + stale + " | architecture review |\n\n" +
		"## Verification\n\n" +
		"| Check | Result |\n|-------|--------|\n" +
		"| go test ./... | passing |\n"
	if err := content.WriteEntity(s, "adr-20260226-use-go", body); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := RunCheckV2(CheckOptions{Store: s, JSON: true, IncludeADR: true, Only: []string{"adr-20260226-use-go"}}, &buf); err != nil {
		t.Fatal(err)
	}
	var result CheckResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}

	var staleIssues []string
	for _, issue := range result.Issues {
		if strings.Contains(issue.Message, "stale") {
			staleIssues = append(staleIssues, issue.Message)
		}
	}
	if len(staleIssues) != 1 {
		t.Fatalf("expected exactly one report of the stale Evidence cite, got %d: %#v", len(staleIssues), staleIssues)
	}
	if !strings.Contains(staleIssues[0], "Affected Topology") {
		t.Fatalf("expected the surviving report to be the section-scoped ADR one, got %q", staleIssues[0])
	}
}
