package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"
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
