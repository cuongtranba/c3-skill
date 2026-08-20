package cmd

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/lagz0ne/c3-design/cli/internal/changeset"
	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// The explorer frontend is a React+TS app (explorer-app/ at the repo root)
// built by Vite into a single dist/index.html; CI and scripts/build.sh copy it
// here before `go build`. The directory embed (with a committed .gitkeep)
// keeps `go build` working on a fresh clone — the bundle is checked at runtime.
//
//go:embed all:assets/explorer/dist
var explorerAssets embed.FS

// ExploreOptions holds parameters for the explore command.
type ExploreOptions struct {
	Store      *store.Store
	C3Dir      string
	IncludeADR bool
	OutFile    string // when empty, write to stdout
	Schema     bool   // print the payload JSON Schema and exit
}

// exploreNode is a single entity in the visual-layer payload.
type exploreNode struct {
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	Title      string      `json:"title"`
	Goal       string      `json:"goal,omitempty"`
	Parent     string      `json:"parent,omitempty"`
	Level      string      `json:"level"`
	Ring       string      `json:"ring"`
	Lifecycle  string      `json:"lifecycle"`
	Staged     bool        `json:"staged"`
	StagedBy   []string    `json:"stagedBy,omitempty"`
	Transition *transition `json:"transition,omitempty"`
}

// transition makes an AG-4 status change explicit and observable.
type transition struct {
	From string `json:"from"`
	To   string `json:"to"`
	By   string `json:"by"`
}

// exploreEdge is a wiring edge between two nodes.
type exploreEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"` // contains | uses | affects
}

type explorePayload struct {
	Project     string         `json:"project"`
	GeneratedAt string         `json:"generatedAt"`
	Nodes       []exploreNode  `json:"nodes"`
	Edges       []exploreEdge  `json:"edges"`
	Events      []exploreEvent `json:"events"`
}

// ringForType maps a C3 entity type to a radial ring key (outside → in).
func ringForType(t string) string {
	switch t {
	case "system":
		return "core"
	case "container":
		return "platform"
	case "component":
		return "service"
	case "ref":
		return "infra"
	case "rule":
		return "security"
	case "adr":
		return "governance"
	default:
		return "service"
	}
}

// levelForType maps a C3 entity type to the coarsest C4 level it belongs to.
func levelForType(t string) string {
	switch t {
	case "system":
		return "context"
	case "container":
		return "container"
	case "adr":
		return "container"
	default:
		return "component"
	}
}

// nonTerminalADR reports whether an ADR status means the change-unit is still in
// flight (its patch targets are staged, not yet applied-and-frozen).
func nonTerminalADR(status string) bool {
	switch status {
	case "open", "proposed", "accepted":
		return true
	default:
		return false
	}
}

// collectStaging reads the change-unit folders under .c3/changes and returns, for
// every fact targeted by a patch in a NON-TERMINAL change-unit, the set of ADR ids
// that stage it. A change-unit's status is its ADR entity's status.
func collectStaging(s *store.Store, c3Dir string) (map[string][]string, error) {
	staged := map[string][]string{}
	changesDir := filepath.Join(c3Dir, "changes")
	entries, err := os.ReadDir(changesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return staged, nil
		}
		return nil, fmt.Errorf("read changes dir: %w", err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		adrID := e.Name()
		adr, err := s.GetEntity(adrID)
		if err != nil || !nonTerminalADR(adr.Status) {
			continue
		}
		patches, err := changeset.ReadPatchDir(filepath.Join(changesDir, adrID))
		if err != nil {
			return nil, err
		}
		for _, p := range patches {
			if p.Target == "" {
				continue
			}
			if !contains(staged[p.Target], adrID) {
				staged[p.Target] = append(staged[p.Target], adrID)
			}
		}
	}
	return staged, nil
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// RunExplore emits a self-contained HTML architecture explorer sourced live from
// the C3 store. Node/edge coverage mirrors the store exactly (AG-1/AG-2); lifecycle
// status (freeze / ADR state / change-unit staging) is explicit per node (AG-4).
func RunExplore(opts ExploreOptions, w io.Writer) error {
	if opts.Schema {
		_, err := fmt.Fprintln(w, explorerSchemaJSON())
		return err
	}

	payload, err := buildExplorePayload(opts.Store, opts.C3Dir, opts.IncludeADR)
	if err != nil {
		return err
	}

	// Pipeline gate: validate the payload against the schema BEFORE generating the
	// three.js HTML. Fail-closed and complete — report every issue at once and
	// refuse to generate, so no missing/invalid datum reaches the rendered file.
	if issues := validateExplorePayload(payload); len(issues) > 0 {
		return fmt.Errorf("explore: payload failed schema validation (%d issue(s)) — refusing to generate the explorer:\n  - %s\nhint: fix the issues above; `c3x explore --schema` prints the payload contract",
			len(issues), strings.Join(issues, "\n  - "))
	}

	html, err := renderExplorerHTML(payload, w)
	if err != nil {
		return err
	}

	if opts.OutFile == "" {
		_, err := io.WriteString(w, html)
		return err
	}
	if err := os.WriteFile(opts.OutFile, []byte(html), 0o644); err != nil {
		return fmt.Errorf("explore: write %s: %w", opts.OutFile, err)
	}
	fmt.Fprintf(w, "Wrote architecture explorer to %s (%d nodes, %d edges)\n",
		opts.OutFile, len(payload.Nodes), len(payload.Edges))
	return nil
}

// buildExplorePayload assembles the node/edge/status payload from the live store.
// It is the AG-1/AG-2/AG-4 contract in one place: every non-hidden store entity
// becomes a node, every membership/uses (and, with ADRs, affects) relationship
// becomes an edge, and every node carries an explicit lifecycle.
func buildExplorePayload(st *store.Store, c3Dir string, includeADR bool) (explorePayload, error) {
	entities, err := st.AllEntities()
	if err != nil {
		return explorePayload{}, fmt.Errorf("explore: load entities: %w", err)
	}

	staged, err := collectStaging(st, c3Dir)
	if err != nil {
		return explorePayload{}, fmt.Errorf("explore: collect staging: %w", err)
	}

	present := map[string]bool{}
	project := "architecture"
	for _, e := range entities {
		if e.Type == "adr" && !includeADR {
			continue
		}
		present[e.ID] = true
		if e.Type == "system" {
			project = e.Title
		}
	}

	payload := explorePayload{
		Project:     project,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	for _, e := range entities {
		if e.Type == "adr" && !includeADR {
			continue
		}
		node := exploreNode{
			ID:     e.ID,
			Type:   e.Type,
			Title:  e.Title,
			Goal:   e.Goal,
			Parent: e.ParentID,
			Level:  levelForType(e.Type),
			Ring:   ringForType(e.Type),
		}
		switch e.Type {
		case "adr":
			node.Lifecycle = normalizeADRStatus(e.Status)
		default:
			if by := staged[e.ID]; len(by) > 0 {
				sort.Strings(by)
				node.Lifecycle = "staged"
				node.Staged = true
				node.StagedBy = by
				node.Transition = &transition{From: "frozen", To: "changing", By: by[0]}
			} else {
				node.Lifecycle = "frozen"
			}
		}
		payload.Nodes = append(payload.Nodes, node)
	}

	// Edges: membership (contains), dependency (uses), change-unit staging (affects).
	for _, e := range entities {
		if e.Type == "adr" && !includeADR {
			continue
		}
		if e.ParentID != "" && present[e.ParentID] {
			payload.Edges = append(payload.Edges, exploreEdge{From: e.ParentID, To: e.ID, Kind: "contains"})
		}
		rels, _ := st.RelationshipsFrom(e.ID)
		var uses []string
		for _, r := range rels {
			if r.RelType == "uses" && present[r.ToID] {
				uses = append(uses, r.ToID)
			}
		}
		sort.Strings(uses)
		for _, to := range uses {
			payload.Edges = append(payload.Edges, exploreEdge{From: e.ID, To: to, Kind: "uses"})
		}
	}

	// Timeline events: the event-store view. One event per ADR, replaying which
	// facts each change-unit created or touched, in date order.
	factIDs := map[string]bool{}
	for _, n := range payload.Nodes {
		if n.Type != "adr" {
			factIDs[n.ID] = true
		}
	}
	events, err := buildExploreEvents(st, c3Dir, factIDs)
	if err != nil {
		return explorePayload{}, err
	}
	payload.Events = events

	// Change-unit affects edges: ADR → each staged target (only when ADRs are nodes).
	if includeADR {
		targets := make([]string, 0, len(staged))
		for t := range staged {
			targets = append(targets, t)
		}
		sort.Strings(targets)
		for _, t := range targets {
			if !present[t] {
				continue
			}
			for _, adrID := range staged[t] {
				if present[adrID] {
					payload.Edges = append(payload.Edges, exploreEdge{From: adrID, To: t, Kind: "affects"})
				}
			}
		}
	}

	return payload, nil
}

func normalizeADRStatus(s string) string {
	if s == "proposed" {
		return "open"
	}
	return s
}

// loadExplorerShell returns the prebuilt single-file explorer bundle. In a
// repo dev checkout it rebuilds a stale explorer-app bundle first (the CLI is
// the one entry point); everywhere else it serves the embedded copy.
func loadExplorerShell(w io.Writer) (string, error) {
	if shell, ok := devExplorerShell(w); ok {
		return shell, nil
	}
	shell, err := explorerAssets.ReadFile("assets/explorer/dist/index.html")
	if err != nil {
		return "", fmt.Errorf("explore: explorer bundle missing — build it with: npm ci --prefix explorer-app && npm run build --prefix explorer-app && cp explorer-app/dist/index.html cli/cmd/assets/explorer/dist/ (%w)", err)
	}
	return string(shell), nil
}

// injectExplorerPayload writes the live payload (and live-mode flag) into the
// bundle's placeholder script.
func injectExplorerPayload(shell string, payload explorePayload, live bool) (string, error) {
	if !strings.Contains(shell, "/*__C3_DATA__*/") {
		return "", fmt.Errorf("explore: explorer bundle is malformed: /*__C3_DATA__*/ placeholder not found in dist/index.html\nhint: rebuild it with: npm run build --prefix explorer-app && cp explorer-app/dist/index.html cli/cmd/assets/explorer/dist/")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("explore: marshal payload: %w", err)
	}
	// Neutralize any "</script>" so the JSON cannot break out of its <script> tag.
	safeData := strings.ReplaceAll(string(data), "</", "<\\/")

	out := strings.Replace(shell, "/*__C3_DATA__*/", "window.C3_DATA = "+safeData+";", 1)
	liveJS := ""
	if live {
		liveJS = "window.C3_LIVE = true;"
		if !strings.Contains(out, "/*__C3_LIVE__*/") {
			return "", fmt.Errorf("explore: bundle predates live mode: /*__C3_LIVE__*/ placeholder not found\nhint: rebuild it with: npm run build --prefix explorer-app && cp explorer-app/dist/index.html cli/cmd/assets/explorer/dist/")
		}
	}
	return strings.Replace(out, "/*__C3_LIVE__*/", liveJS, 1), nil
}

func renderExplorerHTML(payload explorePayload, w io.Writer) (string, error) {
	shell, err := loadExplorerShell(w)
	if err != nil {
		return "", err
	}
	return injectExplorerPayload(shell, payload, false)
}
