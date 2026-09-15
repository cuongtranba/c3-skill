package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// cityCanvasComponent is a project-local component canvas that adds the M1
// `Dependencies` section (edge depends_on) on top of the sections the rich
// fixture bodies already carry.
const cityCanvasComponent = `---
id: component
type: canvas
description: 'Component: an owned unit of behavior inside a container.'
---

domain: software
sections:
    - name: Goal
      content_type: text
      required: true
      purpose: What this component exists to do
    - name: Governance
      content_type: table
      required: true
      purpose: Refs and rules governing this component
      columns:
        - name: Reference
          type: reference
          edge: uses
          targets:
            - ref
            - rule
        - name: Type
          type: text
        - name: Governs
          type: text
        - name: Precedence
          type: text
        - name: Notes
          type: text
    - name: Dependencies
      content_type: table
      required: false
      purpose: Components and containers this one depends on
      columns:
        - name: Depends on
          type: reference
          edge: depends_on
          targets:
            - component
            - container
        - name: Why
          type: text
reject_if: []
workorder: ""
`

const cityCanvasBoundary = `---
id: boundary
type: canvas
description: 'Boundary: a perimeter that encloses containers and components.'
---

domain: software
sections:
    - name: Goal
      content_type: text
      required: true
      purpose: What this perimeter isolates
    - name: Perimeter
      content_type: table
      required: true
      purpose: The perimeter class
      columns:
        - name: Kind
          type: enum
          values:
            - security
            - network
            - infra
            - N.A - <reason>
        - name: Mediation
          type: text
    - name: Members
      content_type: table
      required: true
      purpose: Enclosed containers and components
      columns:
        - name: Member
          type: reference
          edge: encloses
          targets:
            - container
            - component
        - name: Role
          type: text
reject_if: []
workorder: ""
`

const cityCanvasFlow = `---
id: flow
type: canvas
description: 'Flow: an ordered path of interactions that realizes one outcome.'
---

domain: software
sections:
    - name: Goal
      content_type: text
      required: true
      purpose: The outcome this flow realizes
    - name: Steps
      content_type: table
      required: true
      purpose: Ordered hops
      columns:
        - name: Seq
          type: text
        - name: From
          type: reference
          edge: flow_from
          targets:
            - container
            - component
        - name: To
          type: reference
          edge: flow_to
          targets:
            - container
            - component
        - name: Action
          type: text
reject_if: []
workorder: ""
`

// exploreCityFixture exports the rich fixture to disk and layers the M1 model
// on top: a component canvas with Dependencies, a boundary canvas and two
// nested boundary facts, a flow canvas and one flow fact, an eval spec with a
// code binding, and a project dir holding the bound source. Returns the
// imported store, the c3Dir and the projectDir.
func exploreCityFixture(t *testing.T) (*store.Store, string, string) {
	t.Helper()
	c3Dir := exportFixtureToDisk(t)
	projectDir := filepath.Dir(c3Dir)

	canvases := filepath.Join(c3Dir, "canvases")
	if err := os.MkdirAll(canvases, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(canvases, "component.md"), cityCanvasComponent)
	writeFile(t, filepath.Join(canvases, "boundary.md"), cityCanvasBoundary)
	writeFile(t, filepath.Join(canvases, "flow.md"), cityCanvasFlow)

	appendDependencies := func(rel, target, why string) {
		path := filepath.Join(c3Dir, rel)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", rel, err)
		}
		body := string(data) + "\n## Dependencies\n\n| Depends on | Why |\n| --- | --- |\n| " + target + " | " + why + " |\n"
		writeFile(t, path, body)
	}
	appendDependencies("c3-1-api/c3-110-users.md", "c3-101", "needs auth")
	appendDependencies("c3-2-web/c3-201-renderer.md", "c3-101", "renders identity")

	docs := filepath.Join(c3Dir, "documents")
	for _, d := range []string{filepath.Join(docs, "boundary"), filepath.Join(docs, "flow")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, filepath.Join(docs, "boundary", "boundary-service-edge.md"), `---
id: boundary-service-edge
title: Service edge
type: boundary
---

# Service edge

## Goal

Isolate the API container behind the network edge.

## Perimeter

| Kind | Mediation |
| --- | --- |
| network | API gateway |

## Members

| Member | Role |
| --- | --- |
| c3-1 | api |
`)
	writeFile(t, filepath.Join(docs, "boundary", "boundary-auth-core.md"), `---
id: boundary-auth-core
title: Auth core
type: boundary
parent: boundary-service-edge
---

# Auth core

## Goal

Isolate credential handling inside the service edge.

## Perimeter

| Kind | Mediation |
| --- | --- |
| security | token validation |

## Members

| Member | Role |
| --- | --- |
| c3-101 | auth |
`)
	writeFile(t, filepath.Join(docs, "flow", "flow-login.md"), `---
id: flow-login
title: Login
type: flow
---

# Login

## Goal

A user signs in through the web renderer and is authenticated.

## Steps

| Seq | From | To | Action |
| --- | --- | --- | --- |
| 1 | c3-201 | c3-110 | submit credentials |
| 2 | c3-110 | c3-101 | verify token |
`)

	if err := os.MkdirAll(filepath.Join(c3Dir, "eval"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(c3Dir, "eval", "c3-101.yaml"), "fact: c3-101\ncode:\n  - src/auth/*.go\n  - src/auth/*.ts\n")
	if err := os.MkdirAll(filepath.Join(projectDir, "src", "auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(projectDir, "src", "auth", "login.go"), "package auth\n\nfunc Login() {}\n")
	writeFile(t, filepath.Join(projectDir, "src", "auth", "token.go"), "package auth\n\nfunc Token() {}\n")
	writeFile(t, filepath.Join(projectDir, "src", "auth", "client.ts"), "export const login = () => {};\n")

	s := importDir(t, c3Dir)
	return s, c3Dir, projectDir
}

func nodeByID(t *testing.T, p explorePayload, id string) exploreNode {
	t.Helper()
	for _, n := range p.Nodes {
		if n.ID == id {
			return n
		}
	}
	t.Fatalf("node %s not in payload", id)
	return exploreNode{}
}

func edgesOfKind(p explorePayload, kind string) []exploreEdge {
	var out []exploreEdge
	for _, e := range p.Edges {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

// TestBuildExplorePayload_CoversEveryFactAndStatus — AG-1/AG-4 contract: every
// non-ADR store entity becomes a node, and every node carries an explicit
// lifecycle. A fact with no node, or a node with an empty lifecycle, is a wall
// breach.
func TestBuildExplorePayload_CoversEveryFactAndStatus(t *testing.T) {
	c3Dir := exportFixtureToDisk(t)
	s := importDir(t, c3Dir)

	all, err := s.AllEntities()
	if err != nil {
		t.Fatal(err)
	}
	wantFact := map[string]bool{}
	for _, e := range all {
		if e.Type != "adr" {
			wantFact[e.ID] = true
		}
	}

	payload, err := buildExplorePayload(s, c3Dir, filepath.Dir(c3Dir), false)
	if err != nil {
		t.Fatalf("buildExplorePayload: %v", err)
	}
	if payload.SchemaVersion != 2 {
		t.Errorf("schemaVersion = %d, want 2", payload.SchemaVersion)
	}

	gotNode := map[string]bool{}
	for _, n := range payload.Nodes {
		gotNode[n.ID] = true
		if n.Lifecycle == "" {
			t.Errorf("AG-4 breach: node %s has no lifecycle status", n.ID)
		}
		if n.Type == "adr" {
			t.Errorf("adr %s leaked into payload without --include-adr", n.ID)
		}
	}
	for id := range wantFact {
		if !gotNode[id] {
			t.Errorf("AG-1 breach: fact %s has no node", id)
		}
	}
}

// TestBuildExplorePayload_DrawsMembershipAndDependencyEdges — AG-2 contract: a
// parented entity yields a contains edge and a canvas-owned uses relationship
// yields a uses edge. Every edge endpoint must be a present node.
func TestBuildExplorePayload_DrawsMembershipAndDependencyEdges(t *testing.T) {
	c3Dir := exportFixtureToDisk(t)
	s := importDir(t, c3Dir)

	payload, err := buildExplorePayload(s, c3Dir, filepath.Dir(c3Dir), false)
	if err != nil {
		t.Fatal(err)
	}
	present := map[string]bool{}
	for _, n := range payload.Nodes {
		present[n.ID] = true
	}

	ids := map[string]bool{}
	for _, e := range payload.Edges {
		if !present[e.From] || !present[e.To] {
			t.Errorf("AG-2 breach: edge %s->%s references a node absent from the payload", e.From, e.To)
		}
		if e.ID == "" {
			t.Errorf("edge %s->%s has no id", e.From, e.To)
		}
		if ids[e.ID] {
			t.Errorf("duplicate edge id %s", e.ID)
		}
		ids[e.ID] = true
	}
	if len(edgesOfKind(payload, "contains")) == 0 {
		t.Error("expected at least one membership (contains) edge")
	}
	if len(edgesOfKind(payload, "uses")) == 0 {
		t.Error("expected at least one dependency (uses) edge")
	}
}

// TestBuildExplorePayload_IncludeADRAddsADRNodes — with --include-adr, ADR entities
// become nodes carrying their lifecycle state (open/accepted/done/superseded).
func TestBuildExplorePayload_IncludeADRAddsADRNodes(t *testing.T) {
	c3Dir := exportFixtureToDisk(t)
	s := importDir(t, c3Dir)

	withoutADR, err := buildExplorePayload(s, c3Dir, filepath.Dir(c3Dir), false)
	if err != nil {
		t.Fatal(err)
	}
	withADR, err := buildExplorePayload(s, c3Dir, filepath.Dir(c3Dir), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(withADR.Nodes) <= len(withoutADR.Nodes) {
		t.Fatalf("expected --include-adr to add nodes: without=%d with=%d", len(withoutADR.Nodes), len(withADR.Nodes))
	}
	sawADR := false
	for _, n := range withADR.Nodes {
		if n.Type == "adr" {
			sawADR = true
			if n.Lifecycle == "" {
				t.Errorf("adr node %s has no lifecycle state", n.ID)
			}
			if n.Archetype != "record" || n.District != "governance" {
				t.Errorf("adr node %s: archetype=%q district=%q, want record/governance", n.ID, n.Archetype, n.District)
			}
			if n.StatusKey != n.Lifecycle {
				t.Errorf("adr node %s: statusKey %q must be its state %q", n.ID, n.StatusKey, n.Lifecycle)
			}
		}
	}
	if !sawADR {
		t.Error("expected at least one adr node with --include-adr")
	}
}

// TestBuildExplorePayload_CustomTypesBecomeNodes — boundary and flow facts are
// canvas-defined types; they are nodes like every other store entity.
func TestBuildExplorePayload_CustomTypesBecomeNodes(t *testing.T) {
	s, c3Dir, projectDir := exploreCityFixture(t)
	p, err := buildExplorePayload(s, c3Dir, projectDir, false)
	if err != nil {
		t.Fatal(err)
	}
	edge := nodeByID(t, p, "boundary-service-edge")
	if edge.Type != "boundary" || edge.Archetype != "zone" || edge.BoundaryKind != "network" || edge.District != "" {
		t.Errorf("boundary node wrong: %+v", edge)
	}
	core := nodeByID(t, p, "boundary-auth-core")
	if core.BoundaryKind != "security" || core.Parent != "boundary-service-edge" {
		t.Errorf("nested boundary wrong: %+v", core)
	}
	flow := nodeByID(t, p, "flow-login")
	if flow.Type != "flow" || flow.Archetype != "route" || flow.District != "" {
		t.Errorf("flow node wrong: %+v", flow)
	}
	if len(p.Boundaries) != 2 {
		t.Fatalf("boundaries = %d, want 2", len(p.Boundaries))
	}
	for _, b := range p.Boundaries {
		switch b.ID {
		case "boundary-service-edge":
			if b.Kind != "network" || b.Parent != "" || strings.Join(b.Members, ",") != "c3-1" {
				t.Errorf("boundary %+v", b)
			}
		case "boundary-auth-core":
			if b.Kind != "security" || b.Parent != "boundary-service-edge" || strings.Join(b.Members, ",") != "c3-101" {
				t.Errorf("boundary %+v", b)
			}
		}
	}
	if len(p.Flows) != 1 || p.Flows[0].ID != "flow-login" || len(p.Flows[0].Steps) != 2 {
		t.Fatalf("flows = %+v", p.Flows)
	}
	if st := p.Flows[0].Steps[1]; st.Seq != 2 || st.From != "c3-110" || st.To != "c3-101" || st.Action != "verify token" {
		t.Errorf("step 2 = %+v", st)
	}
}

// TestBuildExplorePayload_CanvasOwnedEdgesAndFlowSteps — every canvas-owned rel
// type becomes an edge, and flow Steps rows become flow_step edges carrying
// flow + seq + action label.
func TestBuildExplorePayload_CanvasOwnedEdgesAndFlowSteps(t *testing.T) {
	s, c3Dir, projectDir := exploreCityFixture(t)
	p, err := buildExplorePayload(s, c3Dir, projectDir, false)
	if err != nil {
		t.Fatal(err)
	}

	deps := edgesOfKind(p, "depends_on")
	if len(deps) != 2 {
		t.Fatalf("depends_on edges = %+v, want 2", deps)
	}
	var sawLabel bool
	for _, e := range deps {
		if e.To != "c3-101" {
			t.Errorf("depends_on target %s, want c3-101", e.To)
		}
		if e.ID != e.From+"→"+e.To+"#depends_on" {
			t.Errorf("depends_on id %q", e.ID)
		}
		if e.From == "c3-110" && e.Label == "needs auth" {
			sawLabel = true
		}
	}
	if !sawLabel {
		t.Errorf("depends_on label not derived from the row's text column: %+v", deps)
	}

	enc := edgesOfKind(p, "encloses")
	if len(enc) != 2 {
		t.Fatalf("encloses edges = %+v, want 2", enc)
	}

	steps := edgesOfKind(p, "flow_step")
	if len(steps) != 2 {
		t.Fatalf("flow_step edges = %+v, want 2", steps)
	}
	for _, e := range steps {
		if e.Flow != "flow-login" || e.Seq == 0 {
			t.Errorf("flow_step edge missing flow/seq: %+v", e)
		}
	}
	if steps[0].Seq != 1 || steps[0].From != "c3-201" || steps[0].To != "c3-110" || steps[0].Label != "submit credentials" {
		t.Errorf("step 1 edge = %+v", steps[0])
	}
	if steps[1].Seq != 2 || steps[1].From != "c3-110" || steps[1].To != "c3-101" {
		t.Errorf("step 2 edge = %+v", steps[1])
	}
	if len(edgesOfKind(p, "flow_from")) != 2 || len(edgesOfKind(p, "flow_to")) != 2 {
		t.Errorf("flow_from/flow_to edges must be emitted like any canvas-owned rel type")
	}
}

// TestBuildExplorePayload_BoundaryClosure — a component inherits the boundaries
// enclosing its parent container, ordered outermost first, plus its own.
func TestBuildExplorePayload_BoundaryClosure(t *testing.T) {
	s, c3Dir, projectDir := exploreCityFixture(t)
	p, err := buildExplorePayload(s, c3Dir, projectDir, false)
	if err != nil {
		t.Fatal(err)
	}
	auth := nodeByID(t, p, "c3-101")
	if got := strings.Join(auth.Boundaries, ","); got != "boundary-service-edge,boundary-auth-core" {
		t.Errorf("c3-101 boundaries = %q, want outer→inner closure", got)
	}
	users := nodeByID(t, p, "c3-110")
	if got := strings.Join(users.Boundaries, ","); got != "boundary-service-edge" {
		t.Errorf("c3-110 boundaries = %q, want the container's boundary inherited", got)
	}
	api := nodeByID(t, p, "c3-1")
	if got := strings.Join(api.Boundaries, ","); got != "boundary-service-edge" {
		t.Errorf("c3-1 boundaries = %q", got)
	}
	if api.LegacyBoundary != "service" {
		t.Errorf("c3-1 legacyBoundary = %q, want the frontmatter boundary field", api.LegacyBoundary)
	}
	web := nodeByID(t, p, "c3-201")
	if len(web.Boundaries) != 0 {
		t.Errorf("c3-201 boundaries = %v, want none", web.Boundaries)
	}
	core := nodeByID(t, p, "boundary-auth-core")
	if got := strings.Join(core.Boundaries, ","); got != "boundary-service-edge" {
		t.Errorf("nested boundary inherits its parent boundary, got %q", got)
	}
}

// TestBuildExplorePayload_CodeTechEval — an eval spec with a code: binding
// populates code (globs, files, loc), tech (extension histogram) and eval
// (verdict; unchecked when no match record exists). Nodes without a spec carry
// null code/eval and tech "".
func TestBuildExplorePayload_CodeTechEval(t *testing.T) {
	s, c3Dir, projectDir := exploreCityFixture(t)
	p, err := buildExplorePayload(s, c3Dir, projectDir, false)
	if err != nil {
		t.Fatal(err)
	}
	auth := nodeByID(t, p, "c3-101")
	if auth.Code == nil {
		t.Fatal("c3-101 code is nil despite an eval code: binding")
	}
	if strings.Join(auth.Code.Globs, ",") != "src/auth/*.go,src/auth/*.ts" || auth.Code.Files != 3 || auth.Code.Loc != 7 {
		t.Errorf("c3-101 code = %+v", *auth.Code)
	}
	if auth.Tech != "Go · TypeScript" {
		t.Errorf("c3-101 tech = %q", auth.Tech)
	}
	if auth.Eval == nil || auth.Eval.Verdict != "unchecked" {
		t.Errorf("c3-101 eval = %+v, want unchecked", auth.Eval)
	}
	if auth.StatusKey != "unchecked" {
		t.Errorf("c3-101 statusKey = %q", auth.StatusKey)
	}
	if auth.Category != "foundation" {
		t.Errorf("c3-101 category = %q", auth.Category)
	}

	users := nodeByID(t, p, "c3-110")
	if users.Code != nil || users.Eval != nil || users.Tech != "" {
		t.Errorf("c3-110 without a spec must carry null code/eval and empty tech: %+v", users)
	}
	if users.StatusKey != "unchecked" {
		t.Errorf("c3-110 statusKey = %q", users.StatusKey)
	}
	ref := nodeByID(t, p, "ref-jwt")
	if ref.StatusKey != "governance" || ref.Archetype != "outpost" {
		t.Errorf("ref-jwt statusKey=%q archetype=%q", ref.StatusKey, ref.Archetype)
	}

	// A stored verdict drives statusKey.
	if err := s.SaveEvalMatch(store.EvalMatchRecord{Fact: "c3-101", Verdict: "holds"}); err != nil {
		t.Fatal(err)
	}
	p, err = buildExplorePayload(s, c3Dir, projectDir, false)
	if err != nil {
		t.Fatal(err)
	}
	auth = nodeByID(t, p, "c3-101")
	if auth.Eval.Verdict != "holds" || auth.StatusKey != "stable" {
		t.Errorf("after holds verdict: eval=%+v statusKey=%q", auth.Eval, auth.StatusKey)
	}
	if err := s.SaveEvalMatch(store.EvalMatchRecord{Fact: "c3-101", Verdict: "needs-judgement"}); err != nil {
		t.Fatal(err)
	}
	p, _ = buildExplorePayload(s, c3Dir, projectDir, false)
	if nodeByID(t, p, "c3-101").StatusKey != "review" {
		t.Errorf("needs-judgement must map to review")
	}
}

// TestBuildExplorePayload_EveryNodeLaidOut — every node carries a layout,
// archetype and (unless zone/route) a district that exists.
func TestBuildExplorePayload_EveryNodeLaidOut(t *testing.T) {
	s, c3Dir, projectDir := exploreCityFixture(t)
	p, err := buildExplorePayload(s, c3Dir, projectDir, true)
	if err != nil {
		t.Fatal(err)
	}
	districts := map[string]bool{}
	for _, d := range p.Districts {
		districts[d.ID] = true
	}
	for _, n := range p.Nodes {
		if n.Layout.W <= 0 || n.Layout.D <= 0 {
			t.Errorf("node %s has no footprint: %+v", n.ID, n.Layout)
		}
		if n.Archetype == "" {
			t.Errorf("node %s has no archetype", n.ID)
		}
		if n.Archetype == "zone" || n.Archetype == "route" {
			if n.District != "" {
				t.Errorf("node %s (%s) must not have a district", n.ID, n.Archetype)
			}
			continue
		}
		if !districts[n.District] {
			t.Errorf("node %s district %q does not exist", n.ID, n.District)
		}
	}
	sys := nodeByID(t, p, "c3-0")
	if sys.Archetype != "headquarters" || sys.District != "hq" || sys.Importance != 1 {
		t.Errorf("system node = %+v", sys)
	}
	api := nodeByID(t, p, "c3-1")
	if api.Archetype != "gatehouse" || api.District != "c3-1" {
		t.Errorf("container node = %+v", api)
	}
	routed := 0
	for _, e := range p.Edges {
		if edgeIsRouted(e.Kind) {
			routed++
		}
	}
	if len(p.Routes) != routed {
		t.Errorf("routes = %d, want one per routed edge (%d)", len(p.Routes), routed)
	}
	// Zones and flow participation are realised by ground markings and the flow
	// overlay, never by cables: they must not get docks or routes.
	for _, r := range p.Routes {
		if !edgeIsRouted(r.Kind) {
			t.Errorf("route %s has unrouted kind %s", r.ID, r.Kind)
		}
	}
}

// TestBuildExplorePayload_StagedStatusKey — a fact staged by a non-terminal
// change-unit is `changing` regardless of its eval verdict.
func TestBuildExplorePayload_StagedStatusKey(t *testing.T) {
	s, c3Dir, projectDir := exploreCityFixture(t)
	unit := filepath.Join(c3Dir, "changes", "adr-20260226-use-go")
	if err := os.MkdirAll(unit, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(unit, "c3-110.patch.md"), "---\ntarget: c3-110\nscope: whole\ntype: component\n---\n\n# users\n\n## Goal\n\nNew goal.\n")
	p, err := buildExplorePayload(s, c3Dir, projectDir, false)
	if err != nil {
		t.Fatal(err)
	}
	users := nodeByID(t, p, "c3-110")
	if !users.Staged || users.StatusKey != "changing" || users.Lifecycle != "staged" {
		t.Errorf("staged node = %+v", users)
	}
}

// TestRunExplore_EmitsSelfContainedHTML — the rendered output inlines the
// prebuilt React bundle and the live data, so the file needs no network at
// open time. Skipped when the frontend bundle has not been built locally;
// CI builds the bundle before `go test`, so the gate always holds there.
func TestRunExplore_EmitsSelfContainedHTML(t *testing.T) {
	if _, err := explorerAssets.ReadFile("assets/explorer/dist/index.html"); err != nil {
		t.Skip("explorer bundle not built; run: npm run build --prefix explorer-app && cp explorer-app/dist/index.html cli/cmd/assets/explorer/dist/")
	}
	c3Dir := exportFixtureToDisk(t)
	s := importDir(t, c3Dir)

	var buf bytes.Buffer
	if err := RunExplore(ExploreOptions{Store: s, C3Dir: c3Dir, ProjectDir: filepath.Dir(c3Dir)}, &buf); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"window.C3_DATA", "C3_EXPLORER", "c3-canvas"} {
		if !strings.Contains(out, want) {
			t.Errorf("self-contained HTML missing %q", want)
		}
	}
	if strings.Contains(out, `src="http`) {
		t.Error("explorer HTML references an external script — not self-contained")
	}
	if strings.Contains(out, "</script>window.C3_DATA") {
		t.Error("data payload not embedded inside a script tag")
	}
}

// TestRunExplore_ExportWritesSceneWithoutBundle — --export writes the scene
// JSON before the HTML step, so a missing explorer bundle never blocks the
// export.
func TestRunExplore_ExportWritesScene(t *testing.T) {
	s, c3Dir, projectDir := exploreCityFixture(t)
	out := filepath.Join(t.TempDir(), "scene.json")
	var buf bytes.Buffer
	err := RunExplore(ExploreOptions{Store: s, C3Dir: c3Dir, ProjectDir: projectDir, Export: out, OutFile: filepath.Join(t.TempDir(), "x.html")}, &buf)
	data, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatalf("scene export not written (run err=%v): %v", err, readErr)
	}
	if !strings.Contains(string(data), `"generator": "c3x visualize"`) {
		t.Errorf("scene export lacks the ObjectLoader metadata:\n%.200s", data)
	}
	if !strings.Contains(buf.String(), out) {
		t.Errorf("export path not reported: %s", buf.String())
	}
}
