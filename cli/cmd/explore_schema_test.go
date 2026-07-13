package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lagz0ne/c3-design/cli/internal/store"
)

func validExplorePayload() explorePayload {
	return explorePayload{
		Project:     "c3-design",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Nodes: []exploreNode{
			{ID: "c3-0", Type: "system", Title: "sys", Level: "context", Ring: "core", Lifecycle: "frozen"},
			{ID: "c3-1", Type: "container", Title: "cont", Parent: "c3-0", Level: "container", Ring: "platform", Lifecycle: "frozen"},
			{ID: "c3-101", Type: "component", Title: "comp", Parent: "c3-1", Level: "component", Ring: "service",
				Lifecycle: "staged", Staged: true, StagedBy: []string{"adr-x"},
				Transition: &transition{From: "frozen", To: "changing", By: "adr-x"}},
		},
		Edges: []exploreEdge{
			{From: "c3-0", To: "c3-1", Kind: "contains"},
			{From: "c3-1", To: "c3-101", Kind: "contains"},
		},
		Events: []exploreEvent{
			{ID: "adr-genesis", Date: "2026-01-01", Title: "genesis", Status: "done",
				Creates: []string{"c3-0", "c3-1", "c3-101"}},
		},
	}
}

func TestValidate_ValidPayloadPasses(t *testing.T) {
	if errs := validateExplorePayload(validExplorePayload()); len(errs) != 0 {
		t.Fatalf("expected no issues, got: %v", errs)
	}
}

// TestValidate_ReportsEveryIssueInOnePass — the anti-goal: validation must not
// stop at the first problem. A payload with several distinct breakages surfaces a
// distinct issue for each, so nothing is missed before generation.
func TestValidate_ReportsEveryIssueInOnePass(t *testing.T) {
	p := validExplorePayload()
	p.Project = ""                         // 1: empty project
	p.Nodes[0].Type = "bogus"              // 2: invalid type
	p.Nodes[1].Ring = "nope"               // 3: invalid ring
	p.Nodes[2].Transition = nil            // 4: staged missing transition
	p.Nodes = append(p.Nodes,              // 5: duplicate id
		exploreNode{ID: "c3-0", Type: "system", Title: "dup", Level: "context", Ring: "core", Lifecycle: "frozen"})
	p.Edges = append(p.Edges,
		exploreEdge{From: "c3-1", To: "ghost", Kind: "uses"},   // 6: dangling 'to'
		exploreEdge{From: "c3-0", To: "c3-1", Kind: "teleport"}) // 7: invalid kind

	errs := validateExplorePayload(p)
	joined := strings.Join(errs, "\n")
	for _, want := range []string{
		"project: must not be empty",
		`invalid type "bogus"`,
		`invalid ring "nope"`,
		"staged node missing transition",
		"duplicate node id: c3-0",
		`references missing node "ghost"`,
		`invalid kind "teleport"`,
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing expected issue %q in:\n%s", want, joined)
		}
	}
	if len(errs) < 7 {
		t.Fatalf("expected at least 7 issues reported at once, got %d:\n%s", len(errs), joined)
	}
}

func TestValidate_DanglingEdgeCaught(t *testing.T) {
	p := validExplorePayload()
	p.Edges = append(p.Edges, exploreEdge{From: "c3-101", To: "missing-ref", Kind: "uses"})
	errs := validateExplorePayload(p)
	if !strings.Contains(strings.Join(errs, "\n"), `references missing node "missing-ref"`) {
		t.Fatalf("dangling edge not caught: %v", errs)
	}
}

func TestValidate_StagedConsistency(t *testing.T) {
	p := validExplorePayload()
	p.Nodes[2].Staged = false // lifecycle staged but flag false
	errs := validateExplorePayload(p)
	if !strings.Contains(strings.Join(errs, "\n"), "staged flag and lifecycle disagree") {
		t.Fatalf("staged inconsistency not caught: %v", errs)
	}
}

func TestValidate_ADRLifecycleMustBeADRState(t *testing.T) {
	p := validExplorePayload()
	p.Nodes = append(p.Nodes, exploreNode{ID: "adr-x", Type: "adr", Title: "d", Level: "container", Ring: "governance", Lifecycle: "frozen"})
	errs := validateExplorePayload(p)
	if !strings.Contains(strings.Join(errs, "\n"), "is not an ADR state") {
		t.Fatalf("adr lifecycle not validated: %v", errs)
	}
}

// TestSchemaJSON_MatchesValidatorEnums — no-drift guard: every enum value the
// validator enforces must appear in the published JSON Schema.
func TestSchemaJSON_MatchesValidatorEnums(t *testing.T) {
	js := explorerSchemaJSON()
	for _, m := range []map[string]bool{schemaNodeTypes, schemaLevels, schemaRings, schemaLifecycles, schemaEdgeKinds} {
		for v := range m {
			if !strings.Contains(js, `"`+v+`"`) {
				t.Errorf("schema JSON missing enum value %q", v)
			}
		}
	}
	if !strings.Contains(js, "json-schema.org") {
		t.Error("schema JSON missing $schema draft marker")
	}
}

// TestBuildExplorePayload_PassesSchema — the live-store payload the command
// actually generates must satisfy the schema (the gate never trips on real data).
func TestBuildExplorePayload_PassesSchema(t *testing.T) {
	c3Dir := exportFixtureToDisk(t)
	s := importDir(t, c3Dir)
	for _, adr := range []bool{false, true} {
		p, err := buildExplorePayload(s, c3Dir, adr)
		if err != nil {
			t.Fatalf("buildExplorePayload(includeADR=%v): %v", adr, err)
		}
		if errs := validateExplorePayload(p); len(errs) != 0 {
			t.Fatalf("live payload (includeADR=%v) failed schema: %v", adr, errs)
		}
	}
}

// TestValidate_TimelineReplayIntegrity — a fact no event creates, or one two
// events create, breaks the movie: the replayed final frame would not equal the
// live graph. Both must be caught.
func TestValidate_TimelineReplayIntegrity(t *testing.T) {
	p := validExplorePayload()
	p.Events[0].Creates = []string{"c3-0", "c3-1"} // c3-101 unmapped
	errs := validateExplorePayload(p)
	if !strings.Contains(strings.Join(errs, "\n"), "fact c3-101: unmapped") {
		t.Fatalf("unmapped fact not caught: %v", errs)
	}

	p = validExplorePayload()
	p.Events = append(p.Events, exploreEvent{ID: "adr-later", Date: "2026-02-01", Title: "later", Status: "open",
		Creates: []string{"c3-101"}}) // second creator
	errs = validateExplorePayload(p)
	if !strings.Contains(strings.Join(errs, "\n"), "fact c3-101: created by 2 events") {
		t.Fatalf("duplicate creation not caught: %v", errs)
	}
}

func TestValidate_EventDateAndOrder(t *testing.T) {
	p := validExplorePayload()
	p.Events[0].Date = "yesterday"
	errs := validateExplorePayload(p)
	if !strings.Contains(strings.Join(errs, "\n"), `invalid date "yesterday"`) {
		t.Fatalf("bad date not caught: %v", errs)
	}

	p = validExplorePayload()
	p.Events[0].Creates = []string{"c3-0", "c3-1"}
	p.Events = append([]exploreEvent{{ID: "adr-later", Date: "2026-06-01", Title: "later", Status: "done",
		Creates: []string{"c3-101"}}}, p.Events...)
	errs = validateExplorePayload(p)
	if !strings.Contains(strings.Join(errs, "\n"), "out of order") {
		t.Fatalf("out-of-order events not caught: %v", errs)
	}
}

func TestValidate_ModifyBeforeCreateCaught(t *testing.T) {
	p := validExplorePayload()
	p.Events[0].Creates = []string{"c3-0", "c3-1"}
	p.Events[0].Modifies = []string{"c3-101"} // never created
	errs := validateExplorePayload(p)
	joined := strings.Join(errs, "\n")
	if !strings.Contains(joined, `modifies "c3-101" before any event creates it`) {
		t.Fatalf("modify-before-create not caught: %v", errs)
	}
}

func TestEventDate_Resolution(t *testing.T) {
	if d := eventDate(&store.Entity{ID: "adr-20260710-x", Date: "2026-07-10"}); d != "2026-07-10" {
		t.Fatalf("date field should win, got %s", d)
	}
	if d := eventDate(&store.Entity{ID: "adr-20260623-y"}); d != "2026-06-23" {
		t.Fatalf("id-embedded date fallback failed, got %s", d)
	}
	if d := eventDate(&store.Entity{ID: "adr-x"}); d != "0000-00-00" {
		t.Fatalf("genesis sentinel failed, got %s", d)
	}
}

func TestAffectedTopologyEntities_ParsesEntityColumn(t *testing.T) {
	body := "## Goal\n\nx\n\n## Affected Topology\n\n| Entity | Type | Why affected | Evidence | Governance review |\n| --- | --- | --- | --- | --- |\n| c3-1 | container | y | e | g |\n| c3-114 | component | y | e | g |\n"
	got := affectedTopologyEntities(body)
	if len(got) != 2 || got[0] != "c3-1" || got[1] != "c3-114" {
		t.Fatalf("expected [c3-1 c3-114], got %v", got)
	}
}

// TestBuildExploreEvents_EveryFactCreatedOnce — replay integrity at the source:
// the builder's events must create every fact exactly once (union mapping +
// genesis fallback), so the movie's final frame equals the live graph.
func TestBuildExploreEvents_EveryFactCreatedOnce(t *testing.T) {
	c3Dir := exportFixtureToDisk(t)
	s := importDir(t, c3Dir)
	p, err := buildExplorePayload(s, c3Dir, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Events) == 0 {
		t.Fatal("expected at least one timeline event")
	}
	created := map[string]int{}
	for _, ev := range p.Events {
		for _, id := range ev.Creates {
			created[id]++
		}
	}
	for _, n := range p.Nodes {
		if n.Type == "adr" {
			continue
		}
		if created[n.ID] != 1 {
			t.Errorf("fact %s created %d times, want exactly 1", n.ID, created[n.ID])
		}
	}
}

func TestRunExplore_SchemaFlagPrintsSchema(t *testing.T) {
	var buf bytes.Buffer
	if err := RunExplore(ExploreOptions{Schema: true}, &buf); err != nil {
		t.Fatalf("RunExplore --schema: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "architecture-explorer") || !strings.Contains(out, "\"enum\"") {
		t.Fatalf("schema output looks wrong:\n%s", out)
	}
}

// TestExplorerSchemaSnapshot_NoDrift — the committed snapshot the frontend
// generates its TypeScript types from must match the schema the Go validator
// publishes. Drift means the explorer-app types no longer describe the payload.
func TestExplorerSchemaSnapshot_NoDrift(t *testing.T) {
	snap, err := os.ReadFile("../../explorer-app/schema/explorer-payload.schema.json")
	if err != nil {
		t.Fatalf("read schema snapshot: %v", err)
	}
	if strings.TrimSpace(string(snap)) != strings.TrimSpace(explorerSchemaJSON()) {
		t.Fatal("explorer-app/schema/explorer-payload.schema.json is out of date.\n" +
			"Resync: (cd cli && go run . explore --schema) > explorer-app/schema/explorer-payload.schema.json && npm run generate-types --prefix explorer-app")
	}
}
