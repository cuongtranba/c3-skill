package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type sceneObject struct {
	UUID     string         `json:"uuid"`
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	UserData map[string]any `json:"userData"`
	Children []sceneObject  `json:"children"`
	Geometry string         `json:"geometry"`
	Material string         `json:"material"`
	Matrix   []float64      `json:"matrix"`
}

type sceneDoc struct {
	Metadata struct {
		Version   float64 `json:"version"`
		Type      string  `json:"type"`
		Generator string  `json:"generator"`
	} `json:"metadata"`
	Geometries []map[string]any `json:"geometries"`
	Materials  []map[string]any `json:"materials"`
	Object     sceneObject      `json:"object"`
}

func walkScene(o sceneObject, visit func(sceneObject)) {
	visit(o)
	for _, c := range o.Children {
		walkScene(c, visit)
	}
}

// TestBuildSceneJSON_Shape — the export is ObjectLoader JSON: metadata, one
// Group per district and per node, one Line per route, every c3 object tagged
// in userData.c3 with its node id exactly once, and deterministic uuids.
func TestBuildSceneJSON_Shape(t *testing.T) {
	p := cityPayload(t)
	data, err := buildSceneJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc sceneDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("scene is not valid JSON: %v", err)
	}
	if doc.Metadata.Version != 4.6 || doc.Metadata.Type != "Object" || doc.Metadata.Generator != "c3x visualize" {
		t.Errorf("metadata = %+v", doc.Metadata)
	}

	// Ids are namespaced by the c3 type: a container node and its sector
	// district share an id, so count nodes and districts separately.
	idCount := map[string]int{}
	districtCount := map[string]int{}
	groups, lines := 0, 0
	walkScene(doc.Object, func(o sceneObject) {
		if !uuidRE.MatchString(o.UUID) {
			t.Errorf("object %s has malformed uuid %q", o.Name, o.UUID)
		}
		c3, ok := o.UserData["c3"].(map[string]any)
		if !ok {
			return
		}
		id, _ := c3["id"].(string)
		typ, has := c3["type"].(string)
		if !has {
			t.Errorf("object %s userData.c3 lacks type", id)
		}
		switch typ {
		case "district":
			districtCount[id]++
		case "route":
		default:
			idCount[id]++
		}
		switch o.Type {
		case "Group":
			groups++
		case "Line":
			lines++
			if o.Geometry == "" || o.Material == "" {
				t.Errorf("line %s lacks geometry/material", id)
			}
		}
	})
	for _, n := range p.Nodes {
		if idCount[n.ID] != 1 {
			t.Errorf("node %s appears %d times in userData.c3.id, want exactly once", n.ID, idCount[n.ID])
		}
	}
	for _, d := range p.Districts {
		if districtCount[d.ID] != 1 {
			t.Errorf("district %s appears %d times, want once", d.ID, districtCount[d.ID])
		}
	}
	if groups != len(p.Nodes)+len(p.Districts) {
		t.Errorf("groups = %d, want nodes+districts = %d", groups, len(p.Nodes)+len(p.Districts))
	}
	if lines != len(p.Routes) {
		t.Errorf("lines = %d, want one per route (%d)", lines, len(p.Routes))
	}

	// Every referenced geometry/material resolves.
	geoms := map[string]bool{}
	for _, g := range doc.Geometries {
		geoms[g["uuid"].(string)] = true
	}
	mats := map[string]bool{}
	for _, m := range doc.Materials {
		mats[m["uuid"].(string)] = true
	}
	walkScene(doc.Object, func(o sceneObject) {
		if o.Geometry != "" && !geoms[o.Geometry] {
			t.Errorf("object %s references missing geometry %s", o.Name, o.Geometry)
		}
		if o.Material != "" && !mats[o.Material] {
			t.Errorf("object %s references missing material %s", o.Name, o.Material)
		}
	})

	again, err := buildSceneJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != string(data) {
		t.Error("scene export is not deterministic")
	}
}

// TestBuildSceneJSON_Golden — a stable fixture renders to the committed golden
// scene; a diff means the geometry contract moved. Regenerate with
// UPDATE_GOLDEN=1 go test ./cmd -run TestBuildSceneJSON_Golden.
func TestBuildSceneJSON_Golden(t *testing.T) {
	p := goldenPayload()
	layoutCity(&p)
	got, err := buildSceneJSON(p)
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "scene.golden.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (regenerate with UPDATE_GOLDEN=1)", err)
	}
	if string(want) != string(got) {
		t.Fatalf("scene export drifted from %s; regenerate with UPDATE_GOLDEN=1 if intended", golden)
	}
}

// goldenPayload is a small fixed payload (no timestamps, no store) so the
// golden file is stable across machines.
func goldenPayload() explorePayload {
	return explorePayload{
		SchemaVersion: 2, Project: "golden", GeneratedAt: "2026-01-01T00:00:00Z",
		Nodes: []exploreNode{
			{ID: "c3-0", Type: "system", Title: "golden", Level: "context", Lifecycle: "frozen"},
			{ID: "c3-1", Type: "container", Title: "api", Parent: "c3-0", Level: "container", Lifecycle: "frozen"},
			{ID: "c3-101", Type: "component", Title: "auth", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
			{ID: "c3-102", Type: "component", Title: "token store", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
			{ID: "ref-jwt", Type: "ref", Title: "JWT", Level: "component", Lifecycle: "frozen"},
			{ID: "boundary-edge", Type: "boundary", Title: "edge", Level: "component", Lifecycle: "frozen", BoundaryKind: "network"},
			{ID: "flow-login", Type: "flow", Title: "login", Level: "component", Lifecycle: "frozen"},
		},
		Edges: []exploreEdge{
			{ID: "c3-0→c3-1#contains", From: "c3-0", To: "c3-1", Kind: "contains"},
			{ID: "c3-1→c3-101#contains", From: "c3-1", To: "c3-101", Kind: "contains"},
			{ID: "c3-1→c3-102#contains", From: "c3-1", To: "c3-102", Kind: "contains"},
			{ID: "c3-101→c3-102#depends_on", From: "c3-101", To: "c3-102", Kind: "depends_on"},
			{ID: "c3-101→ref-jwt#uses", From: "c3-101", To: "ref-jwt", Kind: "uses"},
			{ID: "boundary-edge→c3-1#encloses", From: "boundary-edge", To: "c3-1", Kind: "encloses"},
			{ID: "c3-101→c3-102#flow_step#flow-login#1", From: "c3-101", To: "c3-102", Kind: "flow_step", Flow: "flow-login", Seq: 1, Label: "verify"},
		},
		Flows:      []exploreFlow{{ID: "flow-login", Title: "login", Steps: []exploreFlowStep{{Seq: 1, From: "c3-101", To: "c3-102", Action: "verify"}}}},
		Boundaries: []exploreBoundary{{ID: "boundary-edge", Kind: "network", Members: []string{"c3-1"}}},
		Events:     []exploreEvent{{ID: "genesis", Date: "0000-00-00", Title: "Genesis", Status: "done", Creates: []string{"c3-0", "c3-1", "c3-101", "c3-102", "ref-jwt", "boundary-edge", "flow-login"}}},
	}
}
