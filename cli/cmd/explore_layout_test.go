package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"
)

func cityPayload(t *testing.T) explorePayload {
	t.Helper()
	s, c3Dir, projectDir := exploreCityFixture(t)
	p, err := buildExplorePayload(s, c3Dir, projectDir, true)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// TestLayoutCity_Deterministic — the same input yields byte-identical layout.
func TestLayoutCity_Deterministic(t *testing.T) {
	p := cityPayload(t)
	a, b := p, p
	layoutCity(&a)
	layoutCity(&b)
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if string(ja) != string(jb) {
		t.Fatal("layoutCity is not deterministic: two runs over the same payload differ")
	}
}

type box struct{ x0, x1, z0, z1 float64 }

func footprintBox(n exploreNode) box {
	return box{
		x0: n.Layout.X - n.Layout.W/2, x1: n.Layout.X + n.Layout.W/2,
		z0: n.Layout.Z - n.Layout.D/2, z1: n.Layout.Z + n.Layout.D/2,
	}
}

func (b box) overlaps(o box) bool {
	return b.x0 < o.x1 && o.x0 < b.x1 && b.z0 < o.z1 && o.z0 < b.z1
}

func (b box) contains(o box) bool {
	const eps = 1e-6
	return b.x0-eps <= o.x0 && o.x1 <= b.x1+eps && b.z0-eps <= o.z0 && o.z1 <= b.z1+eps
}

// TestLayoutCity_NoBuildingOverlap — buildings sharing a district
// never overlap, and every building sits inside its district platform.
func TestLayoutCity_NoBuildingOverlap(t *testing.T) {
	p := cityPayload(t)
	byDistrict := map[string][]exploreNode{}
	districts := map[string]exploreDistrict{}
	for _, d := range p.Districts {
		districts[d.ID] = d
	}
	for _, n := range p.Nodes {
		if n.District == "" {
			continue
		}
		byDistrict[n.District] = append(byDistrict[n.District], n)
		d := districts[n.District]
		platform := box{x0: d.X - d.W/2, x1: d.X + d.W/2, z0: d.Z - d.D/2, z1: d.Z + d.D/2}
		if !platform.contains(footprintBox(n)) {
			t.Errorf("node %s footprint %+v spills outside district %s %+v", n.ID, footprintBox(n), d.ID, platform)
		}
	}
	for id, nodes := range byDistrict {
		for i := range nodes {
			for j := i + 1; j < len(nodes); j++ {
				if footprintBox(nodes[i]).overlaps(footprintBox(nodes[j])) {
					t.Errorf("district %s: %s and %s overlap", id, nodes[i].ID, nodes[j].ID)
				}
			}
		}
	}
	// Districts themselves do not overlap either.
	for i := range p.Districts {
		for j := i + 1; j < len(p.Districts); j++ {
			a, b := p.Districts[i], p.Districts[j]
			ba := box{a.X - a.W/2, a.X + a.W/2, a.Z - a.D/2, a.Z + a.D/2}
			bb := box{b.X - b.W/2, b.X + b.W/2, b.Z - b.D/2, b.Z + b.D/2}
			if ba.overlaps(bb) {
				t.Errorf("districts %s and %s overlap", a.ID, b.ID)
			}
		}
	}
}

// TestLayoutCity_NestedZoneInsideParent — a nested boundary's zone bbox lies
// inside its parent boundary's zone bbox, and a zone covers its members.
func TestLayoutCity_NestedZoneInsideParent(t *testing.T) {
	p := cityPayload(t)
	outer := footprintBox(nodeByID(t, p, "boundary-service-edge"))
	inner := footprintBox(nodeByID(t, p, "boundary-auth-core"))
	if !outer.contains(inner) {
		t.Errorf("nested zone %+v is not inside parent zone %+v", inner, outer)
	}
	if !inner.contains(footprintBox(nodeByID(t, p, "c3-101"))) {
		t.Errorf("zone %+v does not cover its member c3-101", inner)
	}
	// The outer boundary encloses container c3-1, so its zone covers that
	// sector's buildings.
	for _, id := range []string{"c3-1", "c3-101", "c3-110"} {
		if !outer.contains(footprintBox(nodeByID(t, p, id))) {
			t.Errorf("zone of boundary-service-edge does not cover %s", id)
		}
	}
	if outer.contains(footprintBox(nodeByID(t, p, "c3-201"))) {
		t.Errorf("zone of boundary-service-edge must not cover the other sector's c3-201")
	}
}

// onRoad reports whether the xz point lies on a street or avenue centreline
// within a perpendicular tolerance (the lane offset plus slack).
func onRoad(p explorePayload, x, z, tol float64) bool {
	for _, s := range p.Roads.Streets {
		if math.Abs(z-s.Z) <= tol && x >= s.X0-tol && x <= s.X1+tol {
			return true
		}
	}
	for _, a := range p.Roads.Avenues {
		if math.Abs(x-a.X) <= tol && z >= a.Z0-tol && z <= a.Z1+tol {
			return true
		}
	}
	return false
}

// TestLayoutCity_RoutesRunFromDockToDockAlongRoads — every route starts at a
// dock of its source, ends at a dock of its target, and every interior point
// lies on a street or avenue centreline offset by the route's lane.
func TestLayoutCity_RoutesRunFromDockToDockAlongRoads(t *testing.T) {
	p := cityPayload(t)
	nodes := map[string]exploreNode{}
	for _, n := range p.Nodes {
		nodes[n.ID] = n
	}
	edges := map[string]exploreEdge{}
	for _, e := range p.Edges {
		edges[e.ID] = e
	}
	if len(p.Routes) == 0 {
		t.Fatal("expected routes")
	}
	dockAt := func(n exploreNode, edgeID string, wp [3]float64) bool {
		for _, d := range n.Docks {
			if d.EdgeID == edgeID && d.X == wp[0] && d.Z == wp[2] && math.Abs(n.Layout.Y+0.45-wp[1]) < 1e-9 {
				return true
			}
		}
		return false
	}
	for _, r := range p.Routes {
		e, ok := edges[r.EdgeID]
		if !ok {
			t.Errorf("route %s: edge %s missing", r.ID, r.EdgeID)
			continue
		}
		if len(r.Waypoints) < 2 {
			t.Errorf("route %s: %d waypoints", r.ID, len(r.Waypoints))
			continue
		}
		dockEdge := r.EdgeID
		if r.SharedWith != "" {
			dockEdge = r.SharedWith
		}
		if !dockAt(nodes[e.From], dockEdge, r.Waypoints[0]) {
			t.Errorf("route %s does not start at a dock of %s: %v docks=%+v", r.ID, e.From, r.Waypoints[0], nodes[e.From].Docks)
		}
		if !dockAt(nodes[e.To], dockEdge, r.Waypoints[len(r.Waypoints)-1]) {
			t.Errorf("route %s does not end at a dock of %s: %v", r.ID, e.To, r.Waypoints[len(r.Waypoints)-1])
		}
		for i := 1; i < len(r.Waypoints)-1; i++ {
			wp := r.Waypoints[i]
			if wp[1] != 0.16 {
				t.Errorf("route %s waypoint %d y=%v, want 0.16", r.ID, i, wp[1])
			}
			if !onRoad(p, wp[0], wp[2], math.Abs(r.Lane)+0.01) {
				t.Errorf("route %s waypoint %d %v is off every street/avenue (lane %v)", r.ID, i, wp, r.Lane)
			}
		}
		if len(r.Segments) == 0 && r.SharedWith == "" {
			t.Errorf("route %s uses no road segment", r.ID)
		}
	}
}

// TestLayoutCity_LanesDistinctOnSharedSegment — two routes travelling the same
// stretch of road never share a lane, and lanes stay within a few steps of the
// centreline (conflicts are per segment, not per whole road).
func TestLayoutCity_LanesDistinctOnSharedSegment(t *testing.T) {
	p := cityPayload(t)
	shared := 0
	for i := range p.Routes {
		for j := i + 1; j < len(p.Routes); j++ {
			a, b := p.Routes[i], p.Routes[j]
			if a.SharedWith != "" || b.SharedWith != "" {
				continue
			}
			if !travelSameStretch(p, a, b) {
				continue
			}
			shared++
			if a.Lane == b.Lane {
				t.Errorf("routes %s and %s share a stretch but both use lane %v", a.ID, b.ID, a.Lane)
			}
		}
	}
	if shared == 0 {
		t.Fatal("fixture produced no shared stretches; the lane rule is untested")
	}
	for _, r := range p.Routes {
		if math.Abs(r.Lane) > 4*0.7 {
			t.Errorf("route %s lane %v is wider than the busiest segment should need", r.ID, r.Lane)
		}
	}
}

// hops returns a route's on-road hops as (road id, lo, hi) intervals, undoing
// the lane offset so two routes on the same road compare on the centreline.
func hops(p explorePayload, r exploreRoute) map[string][][2]float64 {
	out := map[string][][2]float64{}
	wp := r.Waypoints
	for i := 2; i < len(wp)-1; i++ {
		a, b := wp[i-1], wp[i]
		switch {
		case a[2] == b[2]: // horizontal hop
			for _, s := range p.Roads.Streets {
				if math.Abs(a[2]-r.Lane-s.Z) < 1e-6 || math.Abs(a[2]-s.Z) < 1e-6 {
					out[s.ID] = append(out[s.ID], [2]float64{math.Min(a[0], b[0]) - r.Lane, math.Max(a[0], b[0]) - r.Lane})
				}
			}
		case a[0] == b[0]: // vertical hop
			for _, av := range p.Roads.Avenues {
				if math.Abs(a[0]-r.Lane-av.X) < 1e-6 || math.Abs(a[0]-av.X) < 1e-6 {
					out[av.ID] = append(out[av.ID], [2]float64{math.Min(a[2], b[2]) - r.Lane, math.Max(a[2], b[2]) - r.Lane})
				}
			}
		}
	}
	return out
}

// travelSameStretch reports whether two routes overlap by more than a lane's
// width on some road.
func travelSameStretch(p explorePayload, a, b exploreRoute) bool {
	ha, hb := hops(p, a), hops(p, b)
	for road, ia := range ha {
		for _, x := range ia {
			for _, y := range hb[road] {
				if math.Min(x[1], y[1])-math.Max(x[0], y[0]) > 2 {
					return true
				}
			}
		}
	}
	return false
}

// TestLayoutCity_FlowStepSharesDependsOnRoute — a flow_step edge whose endpoints
// also carry a depends_on edge reuses that route's waypoints and points at it.
func TestLayoutCity_FlowStepSharesDependsOnRoute(t *testing.T) {
	p := cityPayload(t)
	var dep, step *exploreRoute
	for i := range p.Routes {
		r := &p.Routes[i]
		switch {
		case r.Kind == "depends_on" && r.EdgeID == "c3-110→c3-101#depends_on":
			dep = r
		case r.Kind == "flow_step" && len(r.EdgeID) > 0 && r.EdgeID[:len("c3-110→c3-101#flow_step")] == "c3-110→c3-101#flow_step":
			step = r
		}
	}
	if dep == nil || step == nil {
		t.Fatalf("routes missing: dep=%v step=%v", dep, step)
	}
	if step.SharedWith != dep.ID {
		t.Errorf("flow_step sharedWith = %q, want %q", step.SharedWith, dep.ID)
	}
	if len(step.Waypoints) != len(dep.Waypoints) {
		t.Fatalf("shared route waypoints differ: %v vs %v", step.Waypoints, dep.Waypoints)
	}
	for i := range step.Waypoints {
		if step.Waypoints[i] != dep.Waypoints[i] {
			t.Errorf("shared waypoint %d differs: %v vs %v", i, step.Waypoints[i], dep.Waypoints[i])
		}
	}
	if !step.Active || !dep.Active {
		t.Errorf("flow_step routes and the depends_on they mirror are active: step=%v dep=%v", step.Active, dep.Active)
	}
	// c3-201→c3-101 is a depends_on with no flow step behind it and no staging.
	for _, r := range p.Routes {
		if r.EdgeID == "c3-201→c3-101#depends_on" && r.Active {
			t.Errorf("depends_on without a flow step or staging must be inactive")
		}
	}
}

// TestLayoutCity_Archetypes — the derivation table: bunker/transmit by title,
// comms for pure producers, exactly one command per sector, plant otherwise.
func TestLayoutCity_Archetypes(t *testing.T) {
	p := explorePayload{
		Project: "t", GeneratedAt: "2026-01-01T00:00:00Z",
		Nodes: []exploreNode{
			{ID: "c3-0", Type: "system", Title: "sys", Level: "context", Lifecycle: "frozen"},
			{ID: "c3-1", Type: "container", Title: "api", Parent: "c3-0", Level: "container", Lifecycle: "frozen"},
			{ID: "c3-101", Type: "component", Title: "session store", Goal: "persist sessions", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
			{ID: "c3-102", Type: "component", Title: "report renderer", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
			{ID: "c3-103", Type: "component", Title: "ingress", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
			{ID: "c3-104", Type: "component", Title: "core", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
			{ID: "c3-105", Type: "component", Title: "side", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
			{ID: "ref-x", Type: "ref", Title: "x", Level: "component", Lifecycle: "frozen"},
			{ID: "custom-1", Type: "widget", Title: "w", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
		},
		Edges: []exploreEdge{
			{ID: "c3-103→c3-104#depends_on", From: "c3-103", To: "c3-104", Kind: "depends_on"},
			{ID: "c3-104→c3-105#depends_on", From: "c3-104", To: "c3-105", Kind: "depends_on"},
			{ID: "c3-104→c3-101#depends_on", From: "c3-104", To: "c3-101", Kind: "depends_on"},
		},
	}
	layoutCity(&p)
	want := map[string]string{
		"c3-0": "headquarters", "c3-1": "gatehouse", "c3-101": "bunker", "c3-102": "transmit",
		"c3-103": "comms", "c3-104": "command", "c3-105": "plant", "ref-x": "outpost", "custom-1": "plant",
	}
	for _, n := range p.Nodes {
		if n.Archetype != want[n.ID] {
			t.Errorf("%s archetype = %q, want %q", n.ID, n.Archetype, want[n.ID])
		}
	}
	// Layering: c3-103 (row 0) north of c3-104 (row 1) north of c3-105/c3-101 (row 2).
	z := func(id string) float64 { return nodeByID(t, p, id).Layout.Z }
	if !(z("c3-103") < z("c3-104") && z("c3-104") < z("c3-105") && z("c3-104") < z("c3-101")) {
		t.Errorf("longest-path layering violated: 103=%v 104=%v 105=%v 101=%v", z("c3-103"), z("c3-104"), z("c3-105"), z("c3-101"))
	}
	if nodeByID(t, p, "custom-1").District != "c3-1" || nodeByID(t, p, "ref-x").District != "governance" {
		t.Errorf("district assignment wrong")
	}
	// Staged component wins the command slot.
	for i := range p.Nodes {
		if p.Nodes[i].ID == "c3-105" {
			p.Nodes[i].Staged = true
			p.Nodes[i].Lifecycle = "staged"
		}
	}
	layoutCity(&p)
	if nodeByID(t, p, "c3-105").Archetype != "command" || nodeByID(t, p, "c3-104").Archetype != "plant" {
		t.Errorf("staged component must take the command slot: 105=%s 104=%s", nodeByID(t, p, "c3-105").Archetype, nodeByID(t, p, "c3-104").Archetype)
	}
}

// TestLayoutCity_CycleDoesNotHang — a depends_on cycle is layered by ignoring
// the back edge, ties by id.
func TestLayoutCity_CycleDoesNotHang(t *testing.T) {
	p := explorePayload{
		Project: "t", GeneratedAt: "2026-01-01T00:00:00Z",
		Nodes: []exploreNode{
			{ID: "c3-1", Type: "container", Title: "api", Level: "container", Lifecycle: "frozen"},
			{ID: "c3-101", Type: "component", Title: "a", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
			{ID: "c3-102", Type: "component", Title: "b", Parent: "c3-1", Level: "component", Lifecycle: "frozen"},
		},
		Edges: []exploreEdge{
			{ID: "c3-101→c3-102#depends_on", From: "c3-101", To: "c3-102", Kind: "depends_on"},
			{ID: "c3-102→c3-101#depends_on", From: "c3-102", To: "c3-101", Kind: "depends_on"},
		},
	}
	layoutCity(&p)
	if nodeByID(t, p, "c3-101").Layout.Z >= nodeByID(t, p, "c3-102").Layout.Z {
		t.Errorf("cycle: c3-101 (smaller id) must be layered first")
	}
	if len(p.Routes) != 2 {
		t.Errorf("routes = %d", len(p.Routes))
	}
}

// TestArchetypeFor — the pure part of the derivation table.
func TestArchetypeFor(t *testing.T) {
	cases := []struct {
		typ, title, goal, want string
		resolved               bool
	}{
		{"system", "x", "", "headquarters", true},
		{"container", "x", "", "gatehouse", true},
		{"ref", "x", "", "outpost", true},
		{"rule", "x", "", "outpost", true},
		{"adr", "x", "", "record", true},
		{"boundary", "x", "", "zone", true},
		{"flow", "x", "", "route", true},
		{"component", "SQLite cache", "", "bunker", true},
		{"component", "x", "persist the repository", "bunker", true},
		{"component", "explorer", "emit html", "transmit", true},
		{"component", "plain", "plain", "", false},
		{"widget", "x", "", "plant", true},
	}
	for _, c := range cases {
		got, ok := archetypeFor(c.typ, c.title, c.goal)
		if got != c.want || ok != c.resolved {
			t.Errorf("archetypeFor(%q,%q,%q) = %q,%v want %q,%v", c.typ, c.title, c.goal, got, ok, c.want, c.resolved)
		}
	}
}

// TestRouteGraphShortest_FloatNoiseVertices — dock spreading produces x values
// such as -9.9 and -9.900000000000002. On the dense fixture those became two
// vertices joined by an arc whose weight vanished in the distance sum, the tie
// rule re-parented them onto each other, and the path reconstruction looped
// until memory ran out. Coincident points must collapse to one vertex and the
// search must never relax a settled vertex.
func TestRouteGraphShortest_FloatNoiseVertices(t *testing.T) {
	g := &routeGraph{index: map[[2]float64]int{}}
	a := g.vertex(-9.9, 102)
	b := g.vertex(-9.900000000000002, 102) // same point up to float noise
	if a != b {
		t.Fatalf("noise-split points must share a vertex: %d != %d", a, b)
	}
	start := g.vertex(-40, 102)
	end := g.vertex(30, 102)
	g.connect(start, a, "street")
	g.connect(a, end, "street")

	// Force the tie-rule hazard directly too: two distinct vertices with a
	// zero-weight arc between them (weights below double precision at this
	// magnitude) plus a target beyond them.
	h := &routeGraph{index: map[[2]float64]int{}}
	p := h.vertex(0, 0)
	q := h.vertex(0, gridQuantum) // one quantum away: distinct vertices
	far := h.vertex(0, 200)
	src := h.vertex(0, -1e12)
	h.connect(src, p, "s")
	h.connect(p, q, "s")
	h.connect(q, p, "s") // duplicate arc: the tie fires on the way back
	h.connect(q, far, "s")
	// dist[p] is ~1e12, whose ulp (~1.2e-4) exceeds the quantum: 1e12 + 1e-6 == 1e12, a zero-weight step.

	done := make(chan struct{})
	var path []int
	var ok bool
	go func() {
		g.shortest(start, end)
		path, _, ok = h.shortest(src, far)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("shortest did not terminate on a zero-weight tie")
	}
	if !ok || path[0] != src || path[len(path)-1] != far {
		t.Fatalf("path = %v ok=%v", path, ok)
	}
	seen := map[int]bool{}
	for _, v := range path {
		if seen[v] {
			t.Fatalf("path revisits vertex %d: %v", v, path)
		}
		seen[v] = true
	}
}

// TestLayoutCity_ManySectorsWrapIntoBands — twelve containers must not form a
// single east–west strip: sectors wrap into bands separated by a boulevard, every
// sector's avenues reach the major roads north and south of its band, and a
// cross-band dependency still gets a dock-to-dock route.
func TestLayoutCity_ManySectorsWrapIntoBands(t *testing.T) {
	p := explorePayload{Project: "t", GeneratedAt: "2026-01-01T00:00:00Z"}
	for i := 1; i <= 12; i++ {
		cid := fmt.Sprintf("c3-%d", i)
		p.Nodes = append(p.Nodes, exploreNode{ID: cid, Type: "container", Title: cid, Level: "container", Lifecycle: "frozen"})
		for k := 1; k <= 2; k++ {
			p.Nodes = append(p.Nodes, exploreNode{ID: fmt.Sprintf("%s0%d", cid, k), Type: "component", Title: "svc", Parent: cid, Level: "component", Lifecycle: "frozen"})
		}
	}
	p.Edges = []exploreEdge{{ID: "c3-101→c3-1201#depends_on", From: "c3-101", To: "c3-1201", Kind: "depends_on"}}
	layoutCity(&p)

	var w, d float64
	xs, zs := map[float64]bool{}, map[float64]bool{}
	for _, dist := range p.Districts {
		if dist.Kind != "sector" {
			continue
		}
		w = math.Max(w, math.Abs(dist.X)+dist.W/2)
		d = math.Max(d, math.Abs(dist.Z)+dist.D/2)
		xs[dist.X] = true
		zs[dist.Z] = true
	}
	if len(zs) < 3 {
		t.Fatalf("12 sectors span only %d band(s); want a grid, not a strip", len(zs))
	}
	if w > 3*d {
		t.Errorf("base is still a strip: half-width %.0f vs half-depth %.0f", w, d)
	}
	boulevards := 0
	for _, s := range p.Roads.Streets {
		if strings.HasPrefix(s.ID, "boulevard:") {
			boulevards++
			if !s.Major {
				t.Errorf("%s must be a major road", s.ID)
			}
		}
	}
	if boulevards != len(zs)-1 {
		t.Errorf("boulevards = %d, want one between each pair of the %d bands", boulevards, len(zs))
	}
	if len(p.Routes) != 1 || len(p.Routes[0].Waypoints) < 4 {
		t.Fatalf("cross-band route missing or degenerate: %+v", p.Routes)
	}
}
