package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// The explorer payload schema. These enum sets are the SINGLE SOURCE of truth:
// validateExplorePayload checks against them and explorerSchemaJSON renders the
// published JSON Schema from them, so the documented schema and the enforced
// schema can never drift.
var (
	schemaNodeTypes  = set("system", "container", "component", "ref", "rule", "pm-requirement", "user-story", "adr")
	schemaLevels     = set("context", "container", "component")
	schemaRings      = set("core", "platform", "service", "infra", "security", "governance")
	schemaLifecycles = set("frozen", "open", "accepted", "done", "superseded", "staged")
	schemaEdgeKinds  = set("contains", "uses", "affects")
	schemaADRStates  = set("open", "accepted", "done", "superseded")
)

func set(vals ...string) map[string]bool {
	m := make(map[string]bool, len(vals))
	for _, v := range vals {
		m[v] = true
	}
	return m
}

// validateExplorePayload returns EVERY schema violation in one pass. An empty
// slice means the payload is safe to render. This is the pipeline gate:
//
//	c3 docs -> build payload -> validate (here) -> generate -> three.js html
//
// It is fail-closed and complete by design — the anti-goal is that no missing or
// invalid datum reaches the generated HTML, so it never stops at the first error.
func validateExplorePayload(p explorePayload) []string {
	var errs []string
	add := func(format string, a ...any) { errs = append(errs, fmt.Sprintf(format, a...)) }

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

	idCount := map[string]int{}
	for i, n := range p.Nodes {
		if strings.TrimSpace(n.ID) == "" {
			add("node[%d]: empty id", i)
			continue
		}
		idCount[n.ID]++
		if strings.TrimSpace(n.Title) == "" {
			add("node %s: empty title", n.ID)
		}
		if !schemaNodeTypes[n.Type] {
			add("node %s: invalid type %q (allowed: %s)", n.ID, n.Type, strings.Join(sortedKeys(schemaNodeTypes), ", "))
		}
		if !schemaLevels[n.Level] {
			add("node %s: invalid level %q (allowed: %s)", n.ID, n.Level, strings.Join(sortedKeys(schemaLevels), ", "))
		}
		if !schemaRings[n.Ring] {
			add("node %s: invalid ring %q (allowed: %s)", n.ID, n.Ring, strings.Join(sortedKeys(schemaRings), ", "))
		}
		if !schemaLifecycles[n.Lifecycle] {
			add("node %s: invalid lifecycle %q (allowed: %s)", n.ID, n.Lifecycle, strings.Join(sortedKeys(schemaLifecycles), ", "))
		}
		if n.Type == "adr" && !schemaADRStates[n.Lifecycle] {
			add("node %s: adr lifecycle %q is not an ADR state (allowed: %s)", n.ID, n.Lifecycle, strings.Join(sortedKeys(schemaADRStates), ", "))
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

	seenEdge := map[string]bool{}
	for i, e := range p.Edges {
		if e.From == "" || e.To == "" {
			add("edge[%d]: empty endpoint (from=%q to=%q)", i, e.From, e.To)
			continue
		}
		if !schemaEdgeKinds[e.Kind] {
			add("edge %s->%s: invalid kind %q (allowed: %s)", e.From, e.To, e.Kind, strings.Join(sortedKeys(schemaEdgeKinds), ", "))
		}
		if idCount[e.From] == 0 {
			add("edge %s->%s: 'from' references missing node %q", e.From, e.To, e.From)
		}
		if idCount[e.To] == 0 {
			add("edge %s->%s: 'to' references missing node %q", e.From, e.To, e.To)
		}
		key := e.From + "\x00" + e.To + "\x00" + e.Kind
		if seenEdge[key] {
			add("edge %s->%s (%s): duplicate", e.From, e.To, e.Kind)
		}
		seenEdge[key] = true
	}

	return errs
}

// explorerSchemaJSON renders the payload's JSON Schema (draft-07) from the same
// enum sets the validator enforces. Printed by `c3 explore --schema`.
func explorerSchemaJSON() string {
	enum := func(m map[string]bool) []string { return sortedKeys(m) }
	schema := map[string]any{
		"$schema":     "http://json-schema.org/draft-07/schema#",
		"$id":         "https://c3x/schemas/architecture-explorer.v1.json",
		"title":       "C3 Architecture Explorer payload",
		"description": "The window.C3_DATA contract validated before the three.js HTML is generated.",
		"type":        "object",
		"required":    []string{"project", "generatedAt", "nodes", "edges"},
		"additionalProperties": false,
		"properties": map[string]any{
			"project":     map[string]any{"type": "string", "minLength": 1},
			"generatedAt": map[string]any{"type": "string", "format": "date-time"},
			"nodes": map[string]any{
				"type":     "array",
				"minItems": 1,
				"items": map[string]any{
					"type":     "object",
					"required": []string{"id", "type", "title", "level", "ring", "lifecycle"},
					"properties": map[string]any{
						"id":         map[string]any{"type": "string", "minLength": 1},
						"type":       map[string]any{"enum": enum(schemaNodeTypes)},
						"title":      map[string]any{"type": "string", "minLength": 1},
						"goal":       map[string]any{"type": "string"},
						"parent":     map[string]any{"type": "string"},
						"level":      map[string]any{"enum": enum(schemaLevels)},
						"ring":       map[string]any{"enum": enum(schemaRings)},
						"lifecycle":  map[string]any{"enum": enum(schemaLifecycles)},
						"staged":     map[string]any{"type": "boolean"},
						"stagedBy":   map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
						"transition": map[string]any{"type": []string{"object", "null"}, "required": []string{"from", "to", "by"}},
					},
				},
			},
			"edges": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":     "object",
					"required": []string{"from", "to", "kind"},
					"properties": map[string]any{
						"from": map[string]any{"type": "string", "minLength": 1},
						"to":   map[string]any{"type": "string", "minLength": 1},
						"kind": map[string]any{"enum": enum(schemaEdgeKinds)},
					},
				},
			},
		},
	}
	out, _ := json.MarshalIndent(schema, "", "  ")
	return string(out)
}
