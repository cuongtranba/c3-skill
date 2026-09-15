package cmd

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/lagz0ne/c3-design/cli/internal/content"
	"github.com/lagz0ne/c3-design/cli/internal/schema"
)

// The closed vocabularies of the payload. These sets are the SINGLE SOURCE of
// truth: validateExplorePayload checks against them and explorerSchemaJSON
// renders the published JSON Schema from them, so the documented schema and
// the enforced schema can never drift. Node `type` and edge `kind` are OPEN
// (canvas-driven, see exploreAllowedFor) and are not listed here.
var (
	schemaLevels        = set("context", "container", "component")
	schemaLifecycles    = set("frozen", "open", "accepted", "done", "superseded", "staged")
	schemaADRStates     = set("open", "accepted", "done", "superseded")
	schemaVerdicts      = set("holds", "drift", "needs-judgement", "unchecked")
	schemaStatusKeys    = set("stable", "changing", "drift", "review", "unchecked", "open", "accepted", "done", "superseded", "governance")
	schemaArchetypes    = set("headquarters", "command", "comms", "bunker", "plant", "transmit", "gatehouse", "outpost", "record", "zone", "route")
	schemaFaces         = set("n", "s")
	schemaDistrictKinds = set("sector", "hq", "governance")
)

// schemaFixedKinds are the edge kinds the payload derives itself; every other
// kind comes from a canvas edge-column.
var schemaFixedKinds = []string{"contains", "affects", "flow_step"}

// unroutedKinds are edge kinds the city realises without a cable: membership is
// the district, a boundary is a ground marking around its members, and a flow's
// participation edges are the flow overlay. Every other kind gets exactly one route.
var unroutedKinds = map[string]bool{"contains": true, "encloses": true, "flow_from": true, "flow_to": true}

func edgeIsRouted(kind string) bool { return !unroutedKinds[kind] }

func set(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

// exploreAllowed is the canvas-driven vocabulary: node types are the canvas
// ids, edge kinds are the fixed kinds plus every canvas-owned relationship.
type exploreAllowed struct {
	Types map[string]bool
	Kinds map[string]bool
}

// exploreAllowedFor derives the allowed sets from the built-in canvases plus
// the project's .c3/canvases. An empty c3Dir yields the built-in vocabulary.
func exploreAllowedFor(c3Dir string) (exploreAllowed, error) {
	defs, err := schema.AllDefinitions(c3Dir)
	if err != nil {
		return exploreAllowed{}, err
	}
	allowed := exploreAllowed{Types: map[string]bool{}, Kinds: set(schemaFixedKinds...)}
	for _, def := range defs {
		allowed.Types[def.ID] = true
		for rel := range content.CanvasOwnedRelTypes(def) {
			allowed.Kinds[rel] = true
		}
	}
	return allowed, nil
}

// validateExplorePayload returns EVERY schema violation in one pass. An empty
// slice means the payload is safe to render. This is the pipeline gate:
//
//	c3 docs -> build payload -> layout -> validate (here) -> generate -> three.js html
//
// It is fail-closed and complete by design — the anti-goal is that no missing or
// invalid datum reaches the generated HTML, so it never stops at the first error.
func validateExplorePayload(p explorePayload, allowed exploreAllowed) []string {
	var errs []string
	add := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }

	if p.SchemaVersion != explorePayloadVersion {
		add("schemaVersion: %d (want %d)", p.SchemaVersion, explorePayloadVersion)
	}
	if strings.TrimSpace(p.Project) == "" {
		add("project: must not be empty")
	}
	if strings.TrimSpace(p.GeneratedAt) == "" {
		add("generatedAt: must not be empty")
	} else if _, err := time.Parse(time.RFC3339, p.GeneratedAt); err != nil {
		add("generatedAt: %q is not an RFC3339 timestamp", p.GeneratedAt)
	}
	if len(p.Nodes) == 0 {
		add("nodes: must not be empty")
	}

	districts := map[string]bool{}
	for i, d := range p.Districts {
		if strings.TrimSpace(d.ID) == "" {
			add("district[%d]: empty id", i)
			continue
		}
		if districts[d.ID] {
			add("duplicate district id: %s", d.ID)
		}
		districts[d.ID] = true
		if !schemaDistrictKinds[d.Kind] {
			add("district %s: invalid kind %q (allowed: %s)", d.ID, d.Kind, strings.Join(sortedKeys(schemaDistrictKinds), ", "))
		}
		if d.W <= 0 || d.D <= 0 {
			add("district %s: footprint must be positive (w=%v d=%v)", d.ID, d.W, d.D)
		}
	}

	idCount := map[string]int{}
	nodeType := map[string]string{}
	edgeIDs := map[string]bool{}
	for _, e := range p.Edges {
		if e.ID != "" {
			edgeIDs[e.ID] = true
		}
	}
	nodes := map[string]exploreNode{}
	for i, n := range p.Nodes {
		if strings.TrimSpace(n.ID) == "" {
			add("node[%d]: empty id", i)
			continue
		}
		idCount[n.ID]++
		nodeType[n.ID] = n.Type
		nodes[n.ID] = n
		if strings.TrimSpace(n.Title) == "" {
			add("node %s: empty title", n.ID)
		}
		if !allowed.Types[n.Type] {
			add("node %s: invalid type %q (allowed: %s)", n.ID, n.Type, strings.Join(sortedKeys(allowed.Types), ", "))
		}
		if !schemaLevels[n.Level] {
			add("node %s: invalid level %q (allowed: %s)", n.ID, n.Level, strings.Join(sortedKeys(schemaLevels), ", "))
		}
		if !schemaLifecycles[n.Lifecycle] {
			add("node %s: invalid lifecycle %q (allowed: %s)", n.ID, n.Lifecycle, strings.Join(sortedKeys(schemaLifecycles), ", "))
		}
		if n.Type == "adr" && !schemaADRStates[n.Lifecycle] {
			add("node %s: adr lifecycle %q is not an ADR state (allowed: %s)", n.ID, n.Lifecycle, strings.Join(sortedKeys(schemaADRStates), ", "))
		}
		if !schemaStatusKeys[n.StatusKey] {
			add("node %s: invalid statusKey %q (allowed: %s)", n.ID, n.StatusKey, strings.Join(sortedKeys(schemaStatusKeys), ", "))
		}
		if n.Eval != nil && !schemaVerdicts[n.Eval.Verdict] {
			add("node %s: invalid eval verdict %q (allowed: %s)", n.ID, n.Eval.Verdict, strings.Join(sortedKeys(schemaVerdicts), ", "))
		}
		if n.Code != nil && (n.Code.Files < 0 || n.Code.Loc < 0) {
			add("node %s: code counts must not be negative", n.ID)
		}
		if !schemaArchetypes[n.Archetype] {
			add("node %s: unknown archetype %q (allowed: %s)", n.ID, n.Archetype, strings.Join(sortedKeys(schemaArchetypes), ", "))
		}
		if n.Layout.W <= 0 {
			add("node %s: layout.w must be > 0 (got %v)", n.ID, n.Layout.W)
		}
		if n.Layout.D <= 0 {
			add("node %s: layout.d must be > 0 (got %v)", n.ID, n.Layout.D)
		}
		if n.Archetype == "zone" || n.Archetype == "route" {
			if n.District != "" {
				add("node %s: zone/route node must not carry a district (got %q)", n.ID, n.District)
			}
		} else if !districts[n.District] {
			add("node %s: district %q does not exist", n.ID, n.District)
		}
		for _, d := range n.Docks {
			if !schemaFaces[d.Face] {
				add("node %s: dock %s: invalid face %q (allowed: n, s)", n.ID, d.ID, d.Face)
			}
			if !edgeIDs[d.EdgeID] {
				add("node %s: dock %s: edgeId %q does not exist", n.ID, d.ID, d.EdgeID)
			}
		}

		staged := n.Staged || n.Lifecycle == "staged"
		if staged {
			if !n.Staged || n.Lifecycle != "staged" {
				add("node %s: staged flag and lifecycle disagree (staged=%v lifecycle=%q)", n.ID, n.Staged, n.Lifecycle)
			}
			if len(n.StagedBy) == 0 {
				add("node %s: staged node missing stagedBy", n.ID)
			}
			if n.Transition == nil {
				add("node %s: staged node missing transition", n.ID)
			} else if n.Transition.From == "" || n.Transition.To == "" || n.Transition.By == "" {
				add("node %s: staged transition incomplete (from=%q to=%q by=%q)", n.ID, n.Transition.From, n.Transition.To, n.Transition.By)
			}
			if n.StatusKey != "changing" {
				add("node %s: staged node statusKey %q must be \"changing\"", n.ID, n.StatusKey)
			}
		} else if n.Transition != nil {
			add("node %s: non-staged node must not carry a transition", n.ID)
		}
	}

	dups := make([]string, 0)
	for id, c := range idCount {
		if c > 1 {
			dups = append(dups, fmt.Sprintf("%s (x%d)", id, c))
		}
	}
	sort.Strings(dups)
	for _, d := range dups {
		add("duplicate node id: %s", d)
	}

	// Boundaries: every listed boundary is a boundary node, members exist, and
	// every node's boundaries[] resolves to boundary nodes.
	for _, b := range p.Boundaries {
		if nodeType[b.ID] != "boundary" {
			add("boundary %s: not a boundary node", b.ID)
		}
		if b.Parent != "" && idCount[b.Parent] == 0 {
			add("boundary %s: parent %q does not exist", b.ID, b.Parent)
		}
		for _, m := range b.Members {
			if idCount[m] == 0 {
				add("boundary %s: member %q does not exist", b.ID, m)
			}
		}
	}
	for _, n := range p.Nodes {
		for _, b := range n.Boundaries {
			if nodeType[b] != "boundary" {
				add("node %s: boundaries[] entry %q is not a boundary node", n.ID, b)
			}
		}
	}

	// Flows: ids unique, steps resolve, seq strictly increases.
	flows := map[string]bool{}
	for _, f := range p.Flows {
		if strings.TrimSpace(f.ID) == "" {
			add("flow: empty id")
			continue
		}
		if flows[f.ID] {
			add("duplicate flow id: %s", f.ID)
		}
		flows[f.ID] = true
		if nodeType[f.ID] != "flow" {
			add("flow %s: not a flow node", f.ID)
		}
		prev := math.MinInt32
		for i, st := range f.Steps {
			if st.Seq <= prev {
				add("flow %s: seq does not strictly increase at step[%d] (%d after %d)", f.ID, i, st.Seq, prev)
			}
			prev = st.Seq
			if idCount[st.From] == 0 || idCount[st.To] == 0 {
				add("flow %s: step %d references a missing node (from=%q to=%q)", f.ID, st.Seq, st.From, st.To)
			}
		}
	}

	// Timeline events: every fact must be created by exactly one event (replay
	// integrity — the final frame of the movie must equal the live graph), dates
	// must be real and ordered, and every reference must resolve.
	if len(p.Events) == 0 {
		add("events: must not be empty (the timeline needs at least a genesis event)")
	}
	createdBy := map[string][]string{}
	prevDate := ""
	for i, ev := range p.Events {
		if strings.TrimSpace(ev.ID) == "" {
			add("event[%d]: empty id", i)
			continue
		}
		if !eventDateRE.MatchString(ev.Date) {
			add("event %s: invalid date %q (want YYYY-MM-DD)", ev.ID, ev.Date)
		}
		if !schemaADRStates[ev.Status] {
			add("event %s: invalid status %q (allowed: %s)", ev.ID, ev.Status, strings.Join(sortedKeys(schemaADRStates), ", "))
		}
		if ev.Date < prevDate {
			add("event %s: out of order (date %s after %s)", ev.ID, ev.Date, prevDate)
		}
		if ev.Date > prevDate {
			prevDate = ev.Date
		}
		for _, id := range ev.Creates {
			if idCount[id] == 0 {
				add("event %s: creates missing node %q", ev.ID, id)
			}
			createdBy[id] = append(createdBy[id], ev.ID)
		}
		for _, id := range ev.Modifies {
			if idCount[id] == 0 {
				add("event %s: modifies missing node %q", ev.ID, id)
			}
			if len(createdBy[id]) == 0 {
				add("event %s: modifies %q before any event creates it", ev.ID, id)
			}
		}
	}
	if len(p.Events) > 0 {
		for _, n := range p.Nodes {
			if n.Type == "adr" || strings.TrimSpace(n.ID) == "" {
				continue
			}
			switch owners := createdBy[n.ID]; len(owners) {
			case 0:
				add("fact %s: unmapped — no timeline event creates it", n.ID)
			case 1:
				// exactly once — replay integrity holds
			default:
				add("fact %s: created by %d events (%s)", n.ID, len(owners), strings.Join(owners, ", "))
			}
		}
	}

	// Edges: ids unique, kinds canvas-driven, endpoints present, flows resolve.
	seenEdge := map[string]bool{}
	seenEdgeID := map[string]bool{}
	edges := map[string]exploreEdge{}
	for i, e := range p.Edges {
		if e.From == "" || e.To == "" {
			add("edge[%d]: empty endpoint (from=%q to=%q)", i, e.From, e.To)
			continue
		}
		if strings.TrimSpace(e.ID) == "" {
			add("edge %s->%s: empty id", e.From, e.To)
		} else if seenEdgeID[e.ID] {
			add("duplicate edge id %s", e.ID)
		}
		seenEdgeID[e.ID] = true
		edges[e.ID] = e
		if !allowed.Kinds[e.Kind] {
			add("edge %s->%s: invalid kind %q (allowed: %s)", e.From, e.To, e.Kind, strings.Join(sortedKeys(allowed.Kinds), ", "))
		}
		if idCount[e.From] == 0 {
			add("edge %s->%s: 'from' references missing node %q", e.From, e.To, e.From)
		}
		if idCount[e.To] == 0 {
			add("edge %s->%s: 'to' references missing node %q", e.From, e.To, e.To)
		}
		if e.Kind == "flow_step" {
			if !flows[e.Flow] {
				add("edge %s: flow %q does not exist", e.ID, e.Flow)
			}
		} else if e.Flow != "" || e.Seq != 0 {
			add("edge %s: only flow_step edges carry flow/seq", e.ID)
		}
		key := e.From + "\x00" + e.To + "\x00" + e.Kind + "\x00" + e.Flow + "\x00" + fmt.Sprint(e.Seq)
		if seenEdge[key] {
			add("edge %s->%s (%s): duplicate", e.From, e.To, e.Kind)
		}
		seenEdge[key] = true
	}

	// Routes: exactly one per routed edge, dock-to-dock along the roads.
	routeByEdge := map[string]int{}
	routeIDs := map[string]bool{}
	for _, r := range p.Routes {
		routeByEdge[r.EdgeID]++
		routeIDs[r.ID] = true
	}
	for _, e := range p.Edges {
		if !edgeIsRouted(e.Kind) {
			if routeByEdge[e.ID] > 0 {
				add("edge %s: %s edges have no route", e.ID, e.Kind)
			}
			continue
		}
		switch routeByEdge[e.ID] {
		case 0:
			add("edge %s: has no route", e.ID)
		case 1:
		default:
			add("edge %s: has %d routes, want exactly one", e.ID, routeByEdge[e.ID])
		}
	}
	for _, r := range p.Routes {
		if strings.TrimSpace(r.ID) == "" {
			add("route for edge %s: empty id", r.EdgeID)
		}
		e, ok := edges[r.EdgeID]
		if !ok {
			add("route %s: edge %q does not exist", r.ID, r.EdgeID)
			continue
		}
		if r.Kind != e.Kind {
			add("route %s: kind %q differs from edge kind %q", r.ID, r.Kind, e.Kind)
		}
		if r.SharedWith != "" && !routeIDs[r.SharedWith] {
			add("route %s: sharedWith %q does not resolve to a route", r.ID, r.SharedWith)
		}
		if len(r.Waypoints) < 2 {
			add("route %s: fewer than 2 waypoints", r.ID)
			continue
		}
		dockEdge := r.EdgeID
		if r.SharedWith != "" {
			dockEdge = r.SharedWith
		}
		if !dockMatches(nodes[e.From], dockEdge, r.Waypoints[0]) {
			add("route %s: does not start at the source dock of %s (got %v)", r.ID, e.From, r.Waypoints[0])
		}
		if !dockMatches(nodes[e.To], dockEdge, r.Waypoints[len(r.Waypoints)-1]) {
			add("route %s: does not end at the target dock of %s (got %v)", r.ID, e.To, r.Waypoints[len(r.Waypoints)-1])
		}
		tol := math.Abs(r.Lane) + 0.01
		for i := 1; i < len(r.Waypoints)-1; i++ {
			wp := r.Waypoints[i]
			if !onAnyRoad(p.Roads, wp[0], wp[2], tol) {
				add("route %s: waypoint %d %v is off every street/avenue (lane %v)", r.ID, i, wp, r.Lane)
			}
		}
	}

	return errs
}

// dockMatches reports whether wp sits on the node's dock for edgeID at the
// dock height (layout.y + 0.45).
func dockMatches(n exploreNode, edgeID string, wp [3]float64) bool {
	const eps = 1e-6
	for _, d := range n.Docks {
		if d.EdgeID == edgeID && math.Abs(d.X-wp[0]) < eps && math.Abs(d.Z-wp[2]) < eps && math.Abs(n.Layout.Y+cityDockRise-wp[1]) < eps {
			return true
		}
	}
	return false
}

// onAnyRoad reports whether the xz point lies on some street or avenue
// centreline within tol (perpendicular) and inside its span (± tol).
func onAnyRoad(roads exploreRoads, x, z, tol float64) bool {
	for _, s := range roads.Streets {
		if math.Abs(z-s.Z) <= tol && x >= s.X0-tol && x <= s.X1+tol {
			return true
		}
	}
	for _, a := range roads.Avenues {
		if math.Abs(x-a.X) <= tol && z >= a.Z0-tol && z <= a.Z1+tol {
			return true
		}
	}
	return false
}

// explorerSchemaJSON renders the payload's JSON Schema (draft-07) from the same
// closed sets the validator enforces. Node `type` and edge `kind` are open
// strings because their vocabulary comes from the project's canvases. Printed
// by `c3x visualize --schema`.
func explorerSchemaJSON() string {
	enum := func(m map[string]bool) []string { return sortedKeys(m) }
	str := map[string]any{"type": "string"}
	num := map[string]any{"type": "number"}
	open := map[string]any{"type": "string", "minLength": 1, "description": "canvas-driven"}
	strArray := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	layout := map[string]any{
		"type":     "object",
		"required": []string{"x", "y", "z", "w", "d"},
		"properties": map[string]any{
			"x": num, "y": num, "z": num,
			"w": map[string]any{"type": "number", "exclusiveMinimum": 0},
			"d": map[string]any{"type": "number", "exclusiveMinimum": 0},
		},
	}
	waypoint := map[string]any{"type": "array", "minItems": 3, "maxItems": 3, "items": num}
	schema := map[string]any{
		"$schema":              "http://json-schema.org/draft-07/schema#",
		"$id":                  "https://c3x/schemas/architecture-explorer.v2.json",
		"title":                "C3 Architecture Explorer payload",
		"description":          "The window.C3_DATA contract (payload v2: nodes, edges, flows, boundaries and the derived city layout) validated before the three.js HTML is generated.",
		"type":                 "object",
		"required":             []string{"schemaVersion", "project", "generatedAt", "nodes", "edges", "flows", "boundaries", "districts", "roads", "routes", "events"},
		"additionalProperties": false,
		"properties": map[string]any{
			"schemaVersion": map[string]any{"const": explorePayloadVersion},
			"project":       map[string]any{"type": "string", "minLength": 1},
			"generatedAt":   map[string]any{"type": "string", "format": "date-time"},
			"nodes": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "type", "title", "level", "lifecycle", "staged", "category", "tech", "boundaries", "boundaryKind", "legacyBoundary", "code", "eval", "statusKey", "archetype", "district", "importance", "layout", "docks"},
					"properties": map[string]any{
						"id":        map[string]any{"type": "string", "minLength": 1},
						"type":      open,
						"title":     map[string]any{"type": "string", "minLength": 1},
						"goal":      str,
						"parent":    str,
						"level":     map[string]any{"enum": enum(schemaLevels)},
						"lifecycle": map[string]any{"enum": enum(schemaLifecycles)},
						"staged":    map[string]any{"type": "boolean"},
						"stagedBy":  strArray,
						"transition": map[string]any{
							"type":     []string{"object", "null"},
							"required": []string{"from", "to", "by"},
							"properties": map[string]any{
								"from": str,
								"to":   str,
								"by":   str,
							},
						},
						"category":       map[string]any{"type": "string", "description": "component category; may be empty"},
						"tech":           map[string]any{"type": "string", "description": "top two technologies by matched file count, joined with ' · '; empty when unknown"},
						"boundaries":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "enclosing boundary ids, outermost first"},
						"boundaryKind":   map[string]any{"type": "string", "description": "boundary nodes only: the Perimeter Kind"},
						"legacyBoundary": map[string]any{"type": "string", "description": "container free-text boundary: field"},
						"code": map[string]any{
							"type":     []string{"object", "null"},
							"required": []string{"globs", "files", "loc"},
							"properties": map[string]any{
								"globs": strArray,
								"files": map[string]any{"type": "integer", "minimum": 0},
								"loc":   map[string]any{"type": "integer", "minimum": 0},
							},
						},
						"eval": map[string]any{
							"type":     []string{"object", "null"},
							"required": []string{"verdict"},
							"properties": map[string]any{
								"verdict": map[string]any{"enum": enum(schemaVerdicts)},
							},
						},
						"statusKey":  map[string]any{"enum": enum(schemaStatusKeys)},
						"archetype":  map[string]any{"enum": enum(schemaArchetypes)},
						"district":   map[string]any{"type": "string", "description": "district id; empty for zone/route archetypes"},
						"importance": map[string]any{"type": "number", "minimum": 0, "maximum": 1},
						"layout":     layout,
						"docks": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":     "object",
								"required": []string{"id", "face", "x", "z", "edgeId"},
								"properties": map[string]any{
									"id":     map[string]any{"type": "string", "minLength": 1},
									"face":   map[string]any{"enum": enum(schemaFaces)},
									"x":      num,
									"z":      num,
									"edgeId": map[string]any{"type": "string", "minLength": 1},
								},
							},
						},
					},
				},
			},
			"edges": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "from", "to", "kind"},
					"properties": map[string]any{
						"id":    map[string]any{"type": "string", "minLength": 1},
						"from":  map[string]any{"type": "string", "minLength": 1},
						"to":    map[string]any{"type": "string", "minLength": 1},
						"kind":  open,
						"label": str,
						"flow":  map[string]any{"type": "string", "description": "flow_step edges only"},
						"seq":   map[string]any{"type": "integer", "description": "flow_step edges only"},
					},
				},
			},
			"flows": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "title", "steps"},
					"properties": map[string]any{
						"id":    map[string]any{"type": "string", "minLength": 1},
						"title": str,
						"steps": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type":     "object",
								"required": []string{"seq", "from", "to", "action"},
								"properties": map[string]any{
									"seq":    map[string]any{"type": "integer"},
									"from":   map[string]any{"type": "string", "minLength": 1},
									"to":     map[string]any{"type": "string", "minLength": 1},
									"action": str,
								},
							},
						},
					},
				},
			},
			"boundaries": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "kind", "parent", "members"},
					"properties": map[string]any{
						"id":      map[string]any{"type": "string", "minLength": 1},
						"kind":    map[string]any{"type": "string", "description": "the Perimeter Kind enum value; empty for N.A"},
						"parent":  str,
						"members": strArray,
					},
				},
			},
			"districts": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "title", "kind", "x", "z", "w", "d", "y", "members"},
					"properties": map[string]any{
						"id":      map[string]any{"type": "string", "minLength": 1},
						"title":   str,
						"kind":    map[string]any{"enum": enum(schemaDistrictKinds)},
						"x":       num,
						"z":       num,
						"w":       map[string]any{"type": "number", "exclusiveMinimum": 0},
						"d":       map[string]any{"type": "number", "exclusiveMinimum": 0},
						"y":       num,
						"members": strArray,
					},
				},
			},
			"roads": map[string]any{
				"type":     "object",
				"required": []string{"streets", "avenues"},
				"properties": map[string]any{
					"streets": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":     "object",
							"required": []string{"id", "district", "z", "x0", "x1", "width", "major"},
							"properties": map[string]any{
								"id":       map[string]any{"type": "string", "minLength": 1},
								"district": str,
								"z":        num,
								"x0":       num,
								"x1":       num,
								"width":    map[string]any{"type": "number", "exclusiveMinimum": 0},
								"major":    map[string]any{"type": "boolean"},
							},
						},
					},
					"avenues": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":     "object",
							"required": []string{"id", "district", "x", "z0", "z1", "width"},
							"properties": map[string]any{
								"id":       map[string]any{"type": "string", "minLength": 1},
								"district": str,
								"x":        num,
								"z0":       num,
								"z1":       num,
								"width":    map[string]any{"type": "number", "exclusiveMinimum": 0},
							},
						},
					},
				},
			},
			"routes": map[string]any{
				"type":        "array",
				"description": "One per routed edge (every kind except contains, encloses, flow_from, flow_to): dock to dock along streets and avenues.",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "edgeId", "kind", "active", "segments", "lane", "sharedWith", "waypoints"},
					"properties": map[string]any{
						"id":         map[string]any{"type": "string", "minLength": 1},
						"edgeId":     map[string]any{"type": "string", "minLength": 1},
						"kind":       open,
						"active":     map[string]any{"type": "boolean"},
						"segments":   strArray,
						"lane":       num,
						"sharedWith": str,
						"waypoints":  map[string]any{"type": "array", "minItems": 2, "items": waypoint},
					},
				},
			},
			"events": map[string]any{
				"type":        "array",
				"minItems":    1,
				"description": "The timeline: one event per change-unit (ADR), date-ordered; every fact is created by exactly one event so replaying all events reproduces the live graph.",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "date", "title", "status"},
					"properties": map[string]any{
						"id":       map[string]any{"type": "string", "minLength": 1},
						"date":     map[string]any{"type": "string", "pattern": `^\d{4}-\d{2}-\d{2}$`},
						"title":    str,
						"status":   map[string]any{"enum": enum(schemaADRStates)},
						"creates":  strArray,
						"modifies": strArray,
					},
				},
			},
		},
	}
	out, _ := json.MarshalIndent(schema, "", "  ")
	return string(out)
}
