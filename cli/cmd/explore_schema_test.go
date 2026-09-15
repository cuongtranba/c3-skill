package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// testAllowed is the built-in canvas vocabulary plus the M1 rel types the
// hand-built payloads below use.
func testAllowed() exploreAllowed {
	a, _ := exploreAllowedFor("")
	a.Kinds["depends_on"] = true
	a.Kinds["encloses"] = true
	a.Types["boundary"] = true
	a.Types["flow"] = true
	return a
}

func validExplorePayload() explorePayload {
	p := explorePayload{
		SchemaVersion: 2,
		Project:       "c3-design",
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Nodes: []exploreNode{
			{ID: "c3-0", Type: "system", Title: "sys", Level: "context", Lifecycle: "frozen", StatusKey: "unchecked"},
			{ID: "c3-1", Type: "container", Title: "cont", Parent: "c3-0", Level: "container", Lifecycle: "frozen", StatusKey: "unchecked"},
			{ID: "c3-101", Type: "component", Title: "comp", Parent: "c3-1", Level: "component", StatusKey: "changing",
				Lifecycle: "staged", Staged: true, StagedBy: []string{"adr-x"},
				Transition: &transition{From: "frozen", To: "changing", By: "adr-x"}},
			{ID: "c3-102", Type: "component", Title: "store", Parent: "c3-1", Level: "component", Lifecycle: "frozen", StatusKey: "stable",
				Eval: &exploreEval{Verdict: "holds"}, Boundaries: []string{"boundary-b"}},
			{ID: "boundary-b", Type: "boundary", Title: "b", Level: "component", Lifecycle: "frozen", StatusKey: "unchecked", BoundaryKind: "infra"},
			{ID: "flow-f", Type: "flow", Title: "f", Level: "component", Lifecycle: "frozen", StatusKey: "unchecked"},
		},
		Edges: []exploreEdge{
			{ID: "c3-0→c3-1#contains", From: "c3-0", To: "c3-1", Kind: "contains"},
			{ID: "c3-1→c3-101#contains", From: "c3-1", To: "c3-101", Kind: "contains"},
			{ID: "c3-1→c3-102#contains", From: "c3-1", To: "c3-102", Kind: "contains"},
			{ID: "c3-101→c3-102#depends_on", From: "c3-101", To: "c3-102", Kind: "depends_on"},
			{ID: "boundary-b→c3-102#encloses", From: "boundary-b", To: "c3-102", Kind: "encloses"},
			{ID: "c3-101→c3-102#flow_step#flow-f#1", From: "c3-101", To: "c3-102", Kind: "flow_step", Flow: "flow-f", Seq: 1},
		},
		Flows:      []exploreFlow{{ID: "flow-f", Title: "f", Steps: []exploreFlowStep{{Seq: 1, From: "c3-101", To: "c3-102"}}}},
		Boundaries: []exploreBoundary{{ID: "boundary-b", Kind: "infra", Members: []string{"c3-102"}}},
		Events: []exploreEvent{
			{ID: "adr-genesis", Date: "2026-01-01", Title: "genesis", Status: "done",
				Creates: []string{"c3-0", "c3-1", "c3-101", "c3-102", "boundary-b", "flow-f"}},
		},
	}
	layoutCity(&p)
	return p
}

func TestValidate_ValidPayloadPasses(t *testing.T) {
	if errs := validateExplorePayload(validExplorePayload(), testAllowed()); len(errs) != 0 {
		t.Fatalf("expected no issues, got: %v", errs)
	}
}

// TestValidate_ReportsEveryIssueInOnePass — the anti-goal: validation must not
// stop at the first problem. A payload with several distinct breakages surfaces a
// distinct issue for each, so nothing is missed before generation.
func TestValidate_ReportsEveryIssueInOnePass(t *testing.T) {
	p := validExplorePayload()
	p.Project = ""                  // 1: empty project
	p.Nodes[0].Type = "bogus"       // 2: invalid type
	p.Nodes[1].Archetype = "castle" // 3: unknown archetype
	p.Nodes[2].Transition = nil     // 4: staged missing transition
	dup := p.Nodes[0]
	dup.Type = "system"
	p.Nodes = append(p.Nodes, dup) // 5: duplicate id
	p.Edges = append(p.Edges,
		exploreEdge{ID: "c3-1→ghost#uses", From: "c3-1", To: "ghost", Kind: "uses"},       // 6: dangling 'to'
		exploreEdge{ID: "c3-0→c3-1#teleport", From: "c3-0", To: "c3-1", Kind: "teleport"}) // 7: invalid kind

	errs := validateExplorePayload(p, testAllowed())
	joined := strings.Join(errs, "\n")
	for _, want := range []string{
		"project: must not be empty",
		`invalid type "bogus"`,
		`unknown archetype "castle"`,
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

// TestValidate_AllowedSetsAreCanvasDriven — a project canvas adds a node type
// and an edge kind to the allowed sets; nothing is hardcoded.
func TestValidate_AllowedSetsAreCanvasDriven(t *testing.T) {
	_, c3Dir, _ := exploreCityFixture(t)
	allowed, err := exploreAllowedFor(c3Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"system", "container", "component", "ref", "rule", "adr", "boundary", "flow"} {
		if !allowed.Types[typ] {
			t.Errorf("type %q missing from canvas-driven allowed set", typ)
		}
	}
	for _, kind := range []string{"contains", "affects", "flow_step", "uses", "depends_on", "encloses", "flow_from", "flow_to"} {
		if !allowed.Kinds[kind] {
			t.Errorf("kind %q missing from canvas-driven allowed set", kind)
		}
	}
	builtin, err := exploreAllowedFor("")
	if err != nil {
		t.Fatal(err)
	}
	if builtin.Types["boundary"] || builtin.Kinds["depends_on"] {
		t.Error("built-in allowed set must not know project-only canvases")
	}
}

func TestValidate_DanglingEdgeCaught(t *testing.T) {
	p := validExplorePayload()
	p.Edges = append(p.Edges, exploreEdge{ID: "x", From: "c3-101", To: "missing-ref", Kind: "uses"})
	errs := validateExplorePayload(p, testAllowed())
	if !strings.Contains(strings.Join(errs, "\n"), `references missing node "missing-ref"`) {
		t.Fatalf("dangling edge not caught: %v", errs)
	}
}

func TestValidate_StagedConsistency(t *testing.T) {
	p := validExplorePayload()
	p.Nodes[2].Staged = false // lifecycle staged but flag false
	errs := validateExplorePayload(p, testAllowed())
	if !strings.Contains(strings.Join(errs, "\n"), "staged flag and lifecycle disagree") {
		t.Fatalf("staged inconsistency not caught: %v", errs)
	}
}

func TestValidate_ADRLifecycleMustBeADRState(t *testing.T) {
	p := validExplorePayload()
	p.Nodes = append(p.Nodes, exploreNode{ID: "adr-x", Type: "adr", Title: "d", Level: "container", Lifecycle: "frozen", StatusKey: "open"})
	layoutCity(&p)
	errs := validateExplorePayload(p, testAllowed())
	if !strings.Contains(strings.Join(errs, "\n"), "is not an ADR state") {
		t.Fatalf("adr lifecycle not validated: %v", errs)
	}
}

// TestValidate_V2Checks — each new validator branch fires on its own breakage.
func TestValidate_V2Checks(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(p *explorePayload)
		wants string
	}{
		{"schemaVersion", func(p *explorePayload) { p.SchemaVersion = 1 }, "schemaVersion: 1"},
		{"statusKey", func(p *explorePayload) { p.Nodes[0].StatusKey = "weird" }, `invalid statusKey "weird"`},
		{"statusKey staged", func(p *explorePayload) { p.Nodes[2].StatusKey = "stable" }, `staged node statusKey "stable"`},
		{"eval verdict", func(p *explorePayload) { p.Nodes[3].Eval = &exploreEval{Verdict: "maybe"} }, `invalid eval verdict "maybe"`},
		{"layout w", func(p *explorePayload) { p.Nodes[0].Layout.W = 0 }, "layout.w"},
		{"layout d", func(p *explorePayload) { p.Nodes[0].Layout.D = -1 }, "layout.d"},
		{"district exists", func(p *explorePayload) { p.Nodes[0].District = "nowhere" }, `district "nowhere" does not exist`},
		{"district kind", func(p *explorePayload) { p.Districts[0].Kind = "suburb" }, `invalid kind "suburb"`},
		{"boundary id type", func(p *explorePayload) { p.Nodes[4].Type = "component" }, "not a boundary node"},
		{"boundary member exists", func(p *explorePayload) { p.Boundaries[0].Members = []string{"ghost"} }, `member "ghost" does not exist`},
		{"node boundaries resolve", func(p *explorePayload) { p.Nodes[3].Boundaries = []string{"c3-1"} }, `boundaries[] entry "c3-1"`},
		{"flow exists", func(p *explorePayload) { p.Edges[5].Flow = "flow-ghost" }, `flow "flow-ghost"`},
		{"flow seq order", func(p *explorePayload) {
			p.Flows[0].Steps = append(p.Flows[0].Steps, exploreFlowStep{Seq: 1, From: "c3-101", To: "c3-102"})
		}, "seq does not strictly increase"},
		{"dock edge", func(p *explorePayload) { p.Nodes[2].Docks[0].EdgeID = "nope" }, `dock c3-101:`},
		{"dock face", func(p *explorePayload) { p.Nodes[2].Docks[0].Face = "up" }, `invalid face "up"`},
		{"route per edge", func(p *explorePayload) { p.Routes = p.Routes[1:] }, "has no route"},
		{"route waypoints", func(p *explorePayload) { p.Routes[0].Waypoints = p.Routes[0].Waypoints[:1] }, "fewer than 2 waypoints"},
		{"route start dock", func(p *explorePayload) { p.Routes[0].Waypoints[0][0] += 5 }, "does not start at"},
		{"route interior off road", func(p *explorePayload) { p.Routes[0].Waypoints[1][2] += 7 }, "off every street/avenue"},
		{"sharedWith resolves", func(p *explorePayload) { p.Routes[0].SharedWith = "ghost" }, `sharedWith "ghost"`},
		{"edge id unique", func(p *explorePayload) { p.Edges[3].ID = p.Edges[0].ID }, "duplicate edge id"},
		{"archetype for type", func(p *explorePayload) { p.Nodes[4].District = "hq" }, "zone/route node must not carry a district"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := validExplorePayload()
			c.mut(&p)
			errs := validateExplorePayload(p, testAllowed())
			if !strings.Contains(strings.Join(errs, "\n"), c.wants) {
				t.Fatalf("expected issue containing %q, got:\n%s", c.wants, strings.Join(errs, "\n"))
			}
		})
	}
}

// TestSchemaJSON_MatchesValidatorEnums — no-drift guard: every closed enum the
// validator enforces appears in the published JSON Schema, and the open
// type/kind strings are documented as canvas-driven.
func TestSchemaJSON_MatchesValidatorEnums(t *testing.T) {
	js := explorerSchemaJSON()
	for _, m := range []map[string]bool{schemaLevels, schemaLifecycles, schemaVerdicts, schemaStatusKeys, schemaArchetypes, schemaFaces, schemaDistrictKinds} {
		for v := range m {
			if !strings.Contains(js, `"`+v+`"`) {
				t.Errorf("schema JSON missing enum value %q", v)
			}
		}
	}
	if !strings.Contains(js, "json-schema.org") {
		t.Error("schema JSON missing $schema draft marker")
	}
	if !strings.Contains(js, "architecture-explorer.v2.json") {
		t.Error("schema $id must be the v2 contract")
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(js), &doc); err != nil {
		t.Fatalf("schema is not valid JSON: %v", err)
	}
	props := doc["properties"].(map[string]any)
	for _, key := range []string{"schemaVersion", "nodes", "edges", "flows", "boundaries", "districts", "roads", "routes", "events"} {
		if _, ok := props[key]; !ok {
			t.Errorf("schema lacks top-level property %q", key)
		}
	}
	nodeProps := props["nodes"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	typ := nodeProps["type"].(map[string]any)
	if _, closed := typ["enum"]; closed {
		t.Error("node type must be an open string, not an enum")
	}
	if typ["minLength"] != float64(1) || typ["description"] != "canvas-driven" {
		t.Errorf("node type schema = %v", typ)
	}
	for _, key := range []string{"category", "tech", "boundaries", "boundaryKind", "legacyBoundary", "code", "eval", "statusKey", "archetype", "district", "importance", "layout", "docks"} {
		if _, ok := nodeProps[key]; !ok {
			t.Errorf("node schema lacks %q", key)
		}
	}
	if _, has := nodeProps["ring"]; has {
		t.Error("ring was removed in v2")
	}
	edgeProps := props["edges"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	if _, closed := edgeProps["kind"].(map[string]any)["enum"]; closed {
		t.Error("edge kind must be an open string")
	}
	for _, key := range []string{"id", "label", "flow", "seq"} {
		if _, ok := edgeProps[key]; !ok {
			t.Errorf("edge schema lacks %q", key)
		}
	}
}

// TestBuildExplorePayload_PassesSchema — the live-store payload the command
// actually generates must satisfy the schema (the gate never trips on real data).
func TestBuildExplorePayload_PassesSchema(t *testing.T) {
	s, c3Dir, projectDir := exploreCityFixture(t)
	allowed, err := exploreAllowedFor(c3Dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, adr := range []bool{false, true} {
		p, err := buildExplorePayload(s, c3Dir, projectDir, adr)
		if err != nil {
			t.Fatalf("buildExplorePayload(includeADR=%v): %v", adr, err)
		}
		if errs := validateExplorePayload(p, allowed); len(errs) != 0 {
			t.Fatalf("live payload (includeADR=%v) failed schema: %v", adr, errs)
		}
	}
}

// TestValidate_TimelineReplayIntegrity — a fact no event creates, or one two
// events create, breaks the movie: the replayed final frame would not equal the
// live graph. Both must be caught.
func TestValidate_TimelineReplayIntegrity(t *testing.T) {
	p := validExplorePayload()
	p.Events[0].Creates = []string{"c3-0", "c3-1", "c3-102", "boundary-b", "flow-f"} // c3-101 unmapped
	errs := validateExplorePayload(p, testAllowed())
	if !strings.Contains(strings.Join(errs, "\n"), "fact c3-101: unmapped") {
		t.Fatalf("unmapped fact not caught: %v", errs)
	}

	p = validExplorePayload()
	p.Events = append(p.Events, exploreEvent{ID: "adr-later", Date: "2026-02-01", Title: "later", Status: "open",
		Creates: []string{"c3-101"}}) // second creator
	errs = validateExplorePayload(p, testAllowed())
	if !strings.Contains(strings.Join(errs, "\n"), "fact c3-101: created by 2 events") {
		t.Fatalf("duplicate creation not caught: %v", errs)
	}
}

func TestValidate_EventDateAndOrder(t *testing.T) {
	p := validExplorePayload()
	p.Events[0].Date = "yesterday"
	errs := validateExplorePayload(p, testAllowed())
	if !strings.Contains(strings.Join(errs, "\n"), `invalid date "yesterday"`) {
		t.Fatalf("bad date not caught: %v", errs)
	}

	p = validExplorePayload()
	p.Events[0].Creates = []string{"c3-0", "c3-1", "c3-102", "boundary-b", "flow-f"}
	p.Events = append([]exploreEvent{{ID: "adr-later", Date: "2026-06-01", Title: "later", Status: "done",
		Creates: []string{"c3-101"}}}, p.Events...)
	errs = validateExplorePayload(p, testAllowed())
	if !strings.Contains(strings.Join(errs, "\n"), "out of order") {
		t.Fatalf("out-of-order events not caught: %v", errs)
	}
}

func TestValidate_ModifyBeforeCreateCaught(t *testing.T) {
	p := validExplorePayload()
	p.Events[0].Creates = []string{"c3-0", "c3-1", "c3-102", "boundary-b", "flow-f"}
	p.Events[0].Modifies = []string{"c3-101"} // never created
	errs := validateExplorePayload(p, testAllowed())
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
	p, err := buildExplorePayload(s, c3Dir, "", false)
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
	if !strings.Contains(out, "architecture-explorer.v2") || !strings.Contains(out, "\"enum\"") {
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
			"Resync: (cd cli && go run . visualize --schema) > explorer-app/schema/explorer-payload.schema.json && npm run generate-types --prefix explorer-app")
	}
}
