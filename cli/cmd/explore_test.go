package cmd

import (
	"bytes"
	"strings"
	"testing"
)

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

	payload, err := buildExplorePayload(s, c3Dir, false)
	if err != nil {
		t.Fatalf("buildExplorePayload: %v", err)
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
// parented entity yields a contains edge and a uses relationship yields a uses
// edge. Every edge endpoint must be a present node.
func TestBuildExplorePayload_DrawsMembershipAndDependencyEdges(t *testing.T) {
	c3Dir := exportFixtureToDisk(t)
	s := importDir(t, c3Dir)

	payload, err := buildExplorePayload(s, c3Dir, false)
	if err != nil {
		t.Fatal(err)
	}
	present := map[string]bool{}
	for _, n := range payload.Nodes {
		present[n.ID] = true
	}

	var contains, uses int
	for _, e := range payload.Edges {
		if !present[e.From] || !present[e.To] {
			t.Errorf("AG-2 breach: edge %s->%s references a node absent from the payload", e.From, e.To)
		}
		switch e.Kind {
		case "contains":
			contains++
		case "uses":
			uses++
		}
	}
	if contains == 0 {
		t.Error("expected at least one membership (contains) edge")
	}
	if uses == 0 {
		t.Error("expected at least one dependency (uses) edge")
	}
}

// TestBuildExplorePayload_IncludeADRAddsADRNodes — with --include-adr, ADR entities
// become nodes carrying their lifecycle state (open/accepted/done/superseded).
func TestBuildExplorePayload_IncludeADRAddsADRNodes(t *testing.T) {
	c3Dir := exportFixtureToDisk(t)
	s := importDir(t, c3Dir)

	withoutADR, err := buildExplorePayload(s, c3Dir, false)
	if err != nil {
		t.Fatal(err)
	}
	withADR, err := buildExplorePayload(s, c3Dir, true)
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
		}
	}
	if !sawADR {
		t.Error("expected at least one adr node with --include-adr")
	}
}

// TestRunExplore_EmitsSelfContainedHTML — the rendered output inlines the 3D
// engine and the live data, so the file needs no network at open time.
func TestRunExplore_EmitsSelfContainedHTML(t *testing.T) {
	c3Dir := exportFixtureToDisk(t)
	s := importDir(t, c3Dir)

	var buf bytes.Buffer
	if err := RunExplore(ExploreOptions{Store: s, C3Dir: c3Dir}, &buf); err != nil {
		t.Fatalf("RunExplore: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"window.C3_DATA", "window.C3_EXPLORER", "THREE", "<canvas id=\"c3-canvas\""} {
		if !strings.Contains(out, want) {
			t.Errorf("self-contained HTML missing %q", want)
		}
	}
	if strings.Contains(out, "</script>window.C3_DATA") {
		t.Error("data payload not embedded inside a script tag")
	}
}
