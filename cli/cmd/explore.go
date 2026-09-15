package cmd

import (
	"bytes"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lagz0ne/c3-design/cli/internal/changeset"
	"github.com/lagz0ne/c3-design/cli/internal/codemap"
	"github.com/lagz0ne/c3-design/cli/internal/content"
	"github.com/lagz0ne/c3-design/cli/internal/markdown"
	"github.com/lagz0ne/c3-design/cli/internal/schema"
	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// The explorer frontend is a React+TS app (explorer-app/ at the repo root)
// built by Vite into a single dist/index.html; CI and scripts/build.sh copy it
// here before `go build`. The directory embed (with a committed .gitkeep)
// keeps `go build` working on a fresh clone — the bundle is checked at runtime.
//
//go:embed all:assets/explorer/dist
var explorerAssets embed.FS

// explorePayloadVersion is the payload contract this build emits.
const explorePayloadVersion = 2

// ExploreOptions holds parameters for the visualize command.
type ExploreOptions struct {
	Store      *store.Store
	C3Dir      string
	ProjectDir string // repo root: eval code globs resolve against it
	IncludeADR bool
	OutFile    string // when empty, write to stdout
	Schema     bool   // print the payload JSON Schema and exit
	Export     string // when set, also write the three.js ObjectLoader scene here
}

// exploreNode is a single entity in the visual-layer payload.
type exploreNode struct {
	ID             string        `json:"id"`
	Type           string        `json:"type"`
	Title          string        `json:"title"`
	Goal           string        `json:"goal,omitempty"`
	Parent         string        `json:"parent,omitempty"`
	Level          string        `json:"level"`
	Lifecycle      string        `json:"lifecycle"`
	Staged         bool          `json:"staged"`
	StagedBy       []string      `json:"stagedBy,omitempty"`
	Transition     *transition   `json:"transition,omitempty"`
	Category       string        `json:"category"`
	Tech           string        `json:"tech"`
	Boundaries     []string      `json:"boundaries"`
	BoundaryKind   string        `json:"boundaryKind"`
	LegacyBoundary string        `json:"legacyBoundary"`
	Code           *exploreCode  `json:"code"`
	Eval           *exploreEval  `json:"eval"`
	StatusKey      string        `json:"statusKey"`
	Archetype      string        `json:"archetype"`
	District       string        `json:"district"`
	Importance     float64       `json:"importance"`
	Layout         exploreLayout `json:"layout"`
	Docks          []exploreDock `json:"docks"`
}

// exploreCode is the fact→code binding summary: the eval spec's globs and what
// they currently match.
type exploreCode struct {
	Globs []string `json:"globs"`
	Files int      `json:"files"`
	Loc   int      `json:"loc"`
}

// exploreEval is the latest conformance verdict for a node that has an eval spec.
type exploreEval struct {
	Verdict string `json:"verdict"` // holds | drift | needs-judgement | unchecked
}

// exploreLayout is a building's ground centre (x, z), platform height (y) and
// footprint (w east-west, d north-south).
type exploreLayout struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
	W float64 `json:"w"`
	D float64 `json:"d"`
}

// exploreDock is where one route leaves or enters a building.
type exploreDock struct {
	ID     string  `json:"id"`
	Face   string  `json:"face"` // n | s
	X      float64 `json:"x"`
	Z      float64 `json:"z"`
	EdgeID string  `json:"edgeId"`
}

// transition makes an AG-4 status change explicit and observable.
type transition struct {
	From string `json:"from"`
	To   string `json:"to"`
	By   string `json:"by"`
}

// exploreEdge is a wiring edge between two nodes.
type exploreEdge struct {
	ID    string `json:"id"`
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"` // contains | affects | flow_step | any canvas-owned rel type
	Label string `json:"label,omitempty"`
	Flow  string `json:"flow,omitempty"`
	Seq   int    `json:"seq,omitempty"`
}

type exploreFlowStep struct {
	Seq    int    `json:"seq"`
	From   string `json:"from"`
	To     string `json:"to"`
	Action string `json:"action"`
}

type exploreFlow struct {
	ID    string            `json:"id"`
	Title string            `json:"title"`
	Steps []exploreFlowStep `json:"steps"`
}

type exploreBoundary struct {
	ID      string   `json:"id"`
	Kind    string   `json:"kind"`
	Parent  string   `json:"parent"`
	Members []string `json:"members"`
}

type exploreDistrict struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Kind    string   `json:"kind"` // sector | hq | governance
	X       float64  `json:"x"`
	Z       float64  `json:"z"`
	W       float64  `json:"w"`
	D       float64  `json:"d"`
	Y       float64  `json:"y"`
	Members []string `json:"members"`
}

type exploreStreet struct {
	ID       string  `json:"id"`
	District string  `json:"district"`
	Z        float64 `json:"z"`
	X0       float64 `json:"x0"`
	X1       float64 `json:"x1"`
	Width    float64 `json:"width"`
	Major    bool    `json:"major"`
}

type exploreAvenue struct {
	ID       string  `json:"id"`
	District string  `json:"district"`
	X        float64 `json:"x"`
	Z0       float64 `json:"z0"`
	Z1       float64 `json:"z1"`
	Width    float64 `json:"width"`
}

type exploreRoads struct {
	Streets []exploreStreet `json:"streets"`
	Avenues []exploreAvenue `json:"avenues"`
}

type exploreRoute struct {
	ID         string       `json:"id"`
	EdgeID     string       `json:"edgeId"`
	Kind       string       `json:"kind"`
	Active     bool         `json:"active"`
	Segments   []string     `json:"segments"`
	Lane       float64      `json:"lane"`
	SharedWith string       `json:"sharedWith"`
	Waypoints  [][3]float64 `json:"waypoints"`
}

type explorePayload struct {
	SchemaVersion int               `json:"schemaVersion"`
	Project       string            `json:"project"`
	GeneratedAt   string            `json:"generatedAt"`
	Nodes         []exploreNode     `json:"nodes"`
	Edges         []exploreEdge     `json:"edges"`
	Flows         []exploreFlow     `json:"flows"`
	Boundaries    []exploreBoundary `json:"boundaries"`
	Districts     []exploreDistrict `json:"districts"`
	Roads         exploreRoads      `json:"roads"`
	Routes        []exploreRoute    `json:"routes"`
	Events        []exploreEvent    `json:"events"`
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
			return nil, fmt.Errorf("error: change-unit %s has unreadable patch material: %w\nhint: repair the patch frontmatter (c3x change status %s shows the same failure) or mark the unit done", adrID, err, adrID)
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

	payload, err := buildExplorePayload(opts.Store, opts.C3Dir, opts.ProjectDir, opts.IncludeADR)
	if err != nil {
		return err
	}
	allowed, err := exploreAllowedFor(opts.C3Dir)
	if err != nil {
		return fmt.Errorf("error: visualize: load canvases: %w\nhint: fix the canvas reported above (c3x canvas list)", err)
	}

	// Pipeline gate: validate the payload against the schema BEFORE generating the
	// three.js HTML. Fail-closed and complete — report every issue at once and
	// refuse to generate, so no missing/invalid datum reaches the rendered file.
	if issues := validateExplorePayload(payload, allowed); len(issues) > 0 {
		return fmt.Errorf("error: visualize: payload failed schema validation (%d issue(s)) — refusing to generate the explorer:\n  - %s\nhint: fix the issues above; `c3x visualize --schema` prints the payload contract",
			len(issues), strings.Join(issues, "\n  - "))
	}

	// The scene export needs no frontend bundle, so it is written first: a
	// missing bundle must never block the geometry hand-off.
	if opts.Export != "" {
		scene, err := buildSceneJSON(payload)
		if err != nil {
			return fmt.Errorf("error: visualize: build scene: %w\nhint: rerun with --schema to inspect the payload contract", err)
		}
		if err := os.WriteFile(opts.Export, scene, 0o644); err != nil {
			return fmt.Errorf("error: visualize: write %s: %w\nhint: check the --export path is writable", opts.Export, err)
		}
		fmt.Fprintf(w, "Wrote scene export to %s (%d nodes, %d routes)\n", opts.Export, len(payload.Nodes), len(payload.Routes))
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
		return fmt.Errorf("error: visualize: write %s: %w\nhint: check the --file path is writable", opts.OutFile, err)
	}
	fmt.Fprintf(w, "Wrote architecture explorer to %s (%d nodes, %d edges)\n",
		opts.OutFile, len(payload.Nodes), len(payload.Edges))
	return nil
}

// exploreModel is the store-derived material buildExplorePayload assembles the
// payload from: entities, their presence, and the canvas vocabulary.
type exploreModel struct {
	st         *store.Store
	c3Dir      string
	projectDir string
	includeADR bool
	entities   []*store.Entity
	byID       map[string]*store.Entity
	present    map[string]bool
	defs       map[string]schema.Canvas // canvas id → definition
	relKinds   map[string]bool          // every canvas-owned rel type
	staged     map[string][]string
	bindings   codemap.CodeMap
	hasSpec    map[string]bool
}

// buildExplorePayload assembles the node/edge/status payload from the live store
// and lays the city out. It is the AG-1/AG-2/AG-4 contract in one place: every
// non-hidden store entity becomes a node, every membership / canvas-owned /
// flow-step (and, with ADRs, affects) relationship becomes an edge, and every
// node carries an explicit lifecycle. projectDir is the repo root the eval code
// globs resolve against; empty means code/tech are not derived.
func buildExplorePayload(st *store.Store, c3Dir, projectDir string, includeADR bool) (explorePayload, error) {
	entities, err := st.AllEntities()
	if err != nil {
		return explorePayload{}, fmt.Errorf("explore: load entities: %w", err)
	}
	staged, err := collectStaging(st, c3Dir)
	if err != nil {
		return explorePayload{}, err
	}
	defs, err := schema.AllDefinitions(c3Dir)
	if err != nil {
		return explorePayload{}, fmt.Errorf("explore: load canvases: %w", err)
	}
	specs, err := LoadEvalSpecs(c3Dir)
	if err != nil {
		return explorePayload{}, fmt.Errorf("explore: load eval specs: %w", err)
	}

	m := &exploreModel{
		st: st, c3Dir: c3Dir, projectDir: projectDir, includeADR: includeADR,
		entities: entities, byID: map[string]*store.Entity{}, present: map[string]bool{},
		defs: map[string]schema.Canvas{}, relKinds: map[string]bool{}, staged: staged,
		bindings: EvalBindings(specs), hasSpec: map[string]bool{},
	}
	for _, sp := range specs {
		m.hasSpec[sp.Fact] = true
	}
	for _, def := range defs {
		m.defs[def.ID] = def
		for rel := range content.CanvasOwnedRelTypes(def) {
			m.relKinds[rel] = true
		}
	}

	project := "architecture"
	for _, e := range entities {
		m.byID[e.ID] = e
		if e.Type == "adr" && !includeADR {
			continue
		}
		m.present[e.ID] = true
		if e.Type == "system" {
			project = e.Title
		}
	}

	payload := explorePayload{
		SchemaVersion: explorePayloadVersion,
		Project:       project,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
		Edges:         []exploreEdge{},
		Flows:         []exploreFlow{},
		Boundaries:    []exploreBoundary{},
	}

	boundaries, err := m.collectBoundaries()
	if err != nil {
		return explorePayload{}, err
	}
	payload.Boundaries = boundaries
	closure := m.boundaryClosure(boundaries)

	for _, e := range entities {
		if !m.present[e.ID] {
			continue
		}
		node, err := m.node(e, closure[e.ID], boundaries)
		if err != nil {
			return explorePayload{}, err
		}
		payload.Nodes = append(payload.Nodes, node)
	}

	// Edges: membership (contains) and every canvas-owned relationship, per
	// entity in store order; then flow steps; then change-unit staging (affects).
	for _, e := range entities {
		if !m.present[e.ID] {
			continue
		}
		if e.ParentID != "" && m.present[e.ParentID] {
			payload.Edges = append(payload.Edges, exploreEdge{
				ID: edgeID(e.ParentID, e.ID, "contains"), From: e.ParentID, To: e.ID, Kind: "contains",
			})
		}
		relEdges, err := m.relationshipEdges(e)
		if err != nil {
			return explorePayload{}, err
		}
		payload.Edges = append(payload.Edges, relEdges...)
	}

	flows, flowEdges, err := m.collectFlows()
	if err != nil {
		return explorePayload{}, err
	}
	payload.Flows = flows
	payload.Edges = append(payload.Edges, flowEdges...)

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
			if !m.present[t] {
				continue
			}
			for _, adrID := range staged[t] {
				if m.present[adrID] {
					payload.Edges = append(payload.Edges, exploreEdge{
						ID: edgeID(adrID, t, "affects"), From: adrID, To: t, Kind: "affects",
					})
				}
			}
		}
	}

	layoutCity(&payload)
	return payload, nil
}

// edgeID is the stable edge identity: `from→to#kind`. Flow steps append the
// flow id and seq (see flowStepEdgeID) because one pair may carry several steps.
func edgeID(from, to, kind string) string {
	return from + "→" + to + "#" + kind
}

func flowStepEdgeID(from, to, flow string, seq int) string {
	return edgeID(from, to, "flow_step") + "#" + flow + "#" + strconv.Itoa(seq)
}

// node builds one payload node (layout fields are filled later by layoutCity).
func (m *exploreModel) node(e *store.Entity, boundaries []string, all []exploreBoundary) (exploreNode, error) {
	node := exploreNode{
		ID:             e.ID,
		Type:           e.Type,
		Title:          e.Title,
		Goal:           e.Goal,
		Parent:         e.ParentID,
		Level:          levelForType(e.Type),
		Category:       e.Category,
		LegacyBoundary: e.Boundary,
		Boundaries:     boundaries,
		Docks:          []exploreDock{},
	}
	if node.Boundaries == nil {
		node.Boundaries = []string{}
	}
	if e.Type == "boundary" {
		for _, b := range all {
			if b.ID == e.ID {
				node.BoundaryKind = b.Kind
			}
		}
	}
	switch e.Type {
	case "adr":
		node.Lifecycle = normalizeADRStatus(e.Status)
	default:
		if by := m.staged[e.ID]; len(by) > 0 {
			by = append([]string(nil), by...)
			sort.Strings(by)
			node.Lifecycle = "staged"
			node.Staged = true
			node.StagedBy = by
			node.Transition = &transition{From: "frozen", To: "changing", By: by[0]}
		} else {
			node.Lifecycle = "frozen"
		}
	}

	if globs := m.bindings[e.ID]; len(globs) > 0 && m.projectDir != "" {
		code, tech, err := summarizeCode(m.projectDir, globs)
		if err != nil {
			return exploreNode{}, fmt.Errorf("explore: code binding for %s: %w", e.ID, err)
		}
		node.Code = &code
		node.Tech = tech
	}
	if m.hasSpec[e.ID] {
		verdict, err := m.evalVerdict(e.ID)
		if err != nil {
			return exploreNode{}, err
		}
		node.Eval = &exploreEval{Verdict: verdict}
	}
	node.StatusKey = statusKeyFor(node)
	return node, nil
}

// evalVerdict reads the latest stored verdict; no record means "unchecked".
func (m *exploreModel) evalVerdict(id string) (string, error) {
	rec, err := m.st.EvalMatch(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "unchecked", nil
		}
		return "", fmt.Errorf("explore: eval match for %s: %w", id, err)
	}
	switch rec.Verdict {
	case "holds", "drift", "needs-judgement":
		return rec.Verdict, nil
	default:
		return "unchecked", nil
	}
}

// statusKeyFor derives the closed status vocabulary the renderer colours by.
func statusKeyFor(n exploreNode) string {
	switch {
	case n.Staged:
		return "changing"
	case n.Type == "adr":
		return n.Lifecycle
	case n.Type == "ref" || n.Type == "rule":
		return "governance"
	}
	if n.Eval != nil {
		switch n.Eval.Verdict {
		case "holds":
			return "stable"
		case "drift":
			return "drift"
		case "needs-judgement":
			return "review"
		}
	}
	return "unchecked"
}

// techByExtension maps file extensions to the technology label shown on a
// building's stencil.
var techByExtension = map[string]string{
	".go": "Go", ".ts": "TypeScript", ".tsx": "TypeScript", ".js": "JavaScript", ".jsx": "JavaScript",
	".py": "Python", ".rs": "Rust", ".md": "Markdown", ".sh": "Bash", ".yaml": "YAML", ".yml": "YAML", ".json": "JSON",
}

// maxLocFileSize is the size above which a matched file counts toward files
// but not loc.
const maxLocFileSize = 1 << 20

// summarizeCode resolves the eval code globs against the project and returns
// the code summary plus the tech label (top two technologies by file count,
// ties by name, joined with " · "; "" when no extension is recognised).
func summarizeCode(projectDir string, globs []string) (exploreCode, string, error) {
	fsys := os.DirFS(projectDir)
	seen := map[string]bool{}
	var files []string
	for _, glob := range globs {
		matches, err := codemap.GlobFiles(fsys, glob)
		if err != nil {
			return exploreCode{}, "", fmt.Errorf("glob %q: %w", glob, err)
		}
		for _, f := range matches {
			if seen[f] {
				continue
			}
			info, err := fs.Stat(fsys, f)
			if err != nil || info.IsDir() {
				continue
			}
			seen[f] = true
			files = append(files, f)
		}
	}
	sort.Strings(files)

	code := exploreCode{Globs: append([]string(nil), globs...), Files: len(files)}
	hist := map[string]int{}
	for _, f := range files {
		if tech, ok := techByExtension[strings.ToLower(path.Ext(f))]; ok {
			hist[tech]++
		}
		info, err := fs.Stat(fsys, f)
		if err != nil || info.Size() > maxLocFileSize {
			continue
		}
		data, err := fs.ReadFile(fsys, f)
		if err != nil || isBinary(data) {
			continue
		}
		code.Loc += bytes.Count(data, []byte{'\n'})
	}

	techs := make([]string, 0, len(hist))
	for t := range hist {
		techs = append(techs, t)
	}
	sort.Slice(techs, func(i, j int) bool {
		if hist[techs[i]] != hist[techs[j]] {
			return hist[techs[i]] > hist[techs[j]]
		}
		return techs[i] < techs[j]
	})
	if len(techs) > 2 {
		techs = techs[:2]
	}
	return code, strings.Join(techs, " · "), nil
}

// isBinary treats a NUL byte in the first 8 KiB as the binary signal, the same
// heuristic git uses.
func isBinary(data []byte) bool {
	if len(data) > 8192 {
		data = data[:8192]
	}
	return bytes.IndexByte(data, 0) >= 0
}

// relationshipEdges turns the entity's canvas-owned store relationships into
// edges, labelled from the body row that declares them when one exists.
func (m *exploreModel) relationshipEdges(e *store.Entity) ([]exploreEdge, error) {
	rels, err := m.st.RelationshipsFrom(e.ID)
	if err != nil {
		return nil, fmt.Errorf("explore: relationships of %s: %w", e.ID, err)
	}
	var kept []*store.Relationship
	for _, r := range rels {
		if m.relKinds[r.RelType] && m.present[r.ToID] {
			kept = append(kept, r)
		}
	}
	if len(kept) == 0 {
		return nil, nil
	}
	sort.Slice(kept, func(i, j int) bool {
		if kept[i].RelType != kept[j].RelType {
			return kept[i].RelType < kept[j].RelType
		}
		return kept[i].ToID < kept[j].ToID
	})
	labels := m.edgeLabels(e)
	edges := make([]exploreEdge, 0, len(kept))
	for _, r := range kept {
		edges = append(edges, exploreEdge{
			ID: edgeID(r.FromID, r.ToID, r.RelType), From: r.FromID, To: r.ToID, Kind: r.RelType,
			Label: labels[r.RelType+"\x00"+r.ToID],
		})
	}
	return edges, nil
}

// edgeLabels reads the entity body and, for every edge-column row, records the
// row's first `text`-typed cell as the label of the edges that row declares.
// The spec leaves the label source open; the first text column is the row's
// prose ("Why", "Action", "Notes") in every shipped canvas. Missing bodies or
// canvases simply yield no labels.
func (m *exploreModel) edgeLabels(e *store.Entity) map[string]string {
	labels := map[string]string{}
	def, ok := m.defs[schema.CanonicalDefinitionID(e.Type)]
	if !ok {
		return labels
	}
	body, err := content.ReadEntity(m.st, e.ID)
	if err != nil {
		return labels
	}
	for _, sec := range def.Sections {
		var edgeCols []schema.ColumnDef
		textCol := ""
		for _, col := range sec.Columns {
			if strings.TrimSpace(col.Edge) != "" {
				edgeCols = append(edgeCols, col)
			} else if textCol == "" && strings.TrimSpace(col.Type) == "text" {
				textCol = col.Name
			}
		}
		if len(edgeCols) == 0 || textCol == "" {
			continue
		}
		table, err := markdown.ExtractTableFromSection(body, sec.Name)
		if err != nil || table == nil {
			continue
		}
		for _, row := range table.Rows {
			label := strings.TrimSpace(row[textCol])
			if label == "" {
				continue
			}
			for _, col := range edgeCols {
				for _, tok := range referenceTokens(row[col.Name]) {
					key := col.Edge + "\x00" + tok
					if _, dup := labels[key]; !dup {
						labels[key] = label
					}
				}
			}
		}
	}
	return labels
}

// collectBoundaries reads every boundary-typed entity: kind from the first
// Perimeter row (an "N.A - <reason>" kind is reported as ""), members from its
// `encloses` relationships, parent from its parent id.
func (m *exploreModel) collectBoundaries() ([]exploreBoundary, error) {
	out := []exploreBoundary{}
	for _, e := range m.entities {
		if e.Type != "boundary" || !m.present[e.ID] {
			continue
		}
		b := exploreBoundary{ID: e.ID, Members: []string{}}
		if m.present[e.ParentID] {
			b.Parent = e.ParentID
		}
		if body, err := content.ReadEntity(m.st, e.ID); err == nil {
			if table, err := markdown.ExtractTableFromSection(body, "Perimeter"); err == nil && table != nil && len(table.Rows) > 0 {
				kind := strings.TrimSpace(table.Rows[0]["Kind"])
				if strings.HasPrefix(kind, "N.A") {
					kind = ""
				}
				b.Kind = kind
			}
		}
		rels, err := m.st.RelationshipsFrom(e.ID)
		if err != nil {
			return nil, fmt.Errorf("explore: relationships of %s: %w", e.ID, err)
		}
		for _, r := range rels {
			if r.RelType == "encloses" && m.present[r.ToID] {
				b.Members = append(b.Members, r.ToID)
			}
		}
		sort.Strings(b.Members)
		out = append(out, b)
	}
	return out, nil
}

// boundaryClosure computes, for every present entity, the boundaries enclosing
// it: those that enclose it directly, those enclosing any ancestor, and each
// such boundary's own boundary ancestors — ordered outermost first (fewest
// boundary ancestors, then id) and deduplicated.
func (m *exploreModel) boundaryClosure(boundaries []exploreBoundary) map[string][]string {
	byID := map[string]exploreBoundary{}
	direct := map[string][]string{} // member → boundaries enclosing it directly
	for _, b := range boundaries {
		byID[b.ID] = b
		for _, member := range b.Members {
			direct[member] = append(direct[member], b.ID)
		}
	}
	depth := map[string]int{}
	var depthOf func(id string, guard map[string]bool) int
	depthOf = func(id string, guard map[string]bool) int {
		if d, ok := depth[id]; ok {
			return d
		}
		if guard[id] {
			return 0
		}
		guard[id] = true
		b, ok := byID[id]
		d := 0
		if ok && b.Parent != "" {
			if _, parentIsBoundary := byID[b.Parent]; parentIsBoundary {
				d = depthOf(b.Parent, guard) + 1
			}
		}
		depth[id] = d
		return d
	}
	for id := range byID {
		depthOf(id, map[string]bool{})
	}

	closure := map[string][]string{}
	for _, e := range m.entities {
		if !m.present[e.ID] {
			continue
		}
		set := map[string]bool{}
		// Walk the parent chain (including self) collecting direct enclosures.
		// A boundary nested under another boundary is inside it, so a boundary
		// node's own boundary ancestors count as enclosing it too.
		seen := map[string]bool{}
		for cur := e.ID; cur != "" && !seen[cur]; {
			seen[cur] = true
			for _, b := range direct[cur] {
				set[b] = true
			}
			ent, ok := m.byID[cur]
			if !ok {
				break
			}
			if _, parentIsBoundary := byID[ent.ParentID]; parentIsBoundary && e.Type == "boundary" {
				set[ent.ParentID] = true
			}
			cur = ent.ParentID
		}
		// Each boundary's own boundary-typed parent chain.
		for id := range set {
			guard := map[string]bool{}
			for cur := id; cur != ""; {
				b, ok := byID[cur]
				if !ok || guard[cur] {
					break
				}
				guard[cur] = true
				if _, parentIsBoundary := byID[b.Parent]; parentIsBoundary {
					set[b.Parent] = true
				}
				cur = b.Parent
			}
		}
		ids := make([]string, 0, len(set))
		for id := range set {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool {
			if depth[ids[i]] != depth[ids[j]] {
				return depth[ids[i]] < depth[ids[j]]
			}
			return ids[i] < ids[j]
		})
		closure[e.ID] = ids
	}
	return closure
}

// collectFlows reads every flow-typed entity's Steps table. A row becomes a
// step (and a flow_step edge) when both From and To resolve to present nodes;
// rows naming an external actor ("N.A - …") are skipped. Seq is the parsed
// integer when the cell parses, else the 1-based row index; steps are then
// ordered by (seq, row) and any seq that does not strictly increase is bumped
// to predecessor+1 so the contract's monotonic-seq rule holds by construction.
func (m *exploreModel) collectFlows() ([]exploreFlow, []exploreEdge, error) {
	flows := []exploreFlow{}
	var edges []exploreEdge
	for _, e := range m.entities {
		if e.Type != "flow" || !m.present[e.ID] {
			continue
		}
		flow := exploreFlow{ID: e.ID, Title: e.Title, Steps: []exploreFlowStep{}}
		body, err := content.ReadEntity(m.st, e.ID)
		if err == nil {
			flow.Steps = m.flowSteps(body)
		}
		for _, st := range flow.Steps {
			edges = append(edges, exploreEdge{
				ID: flowStepEdgeID(st.From, st.To, e.ID, st.Seq), From: st.From, To: st.To, Kind: "flow_step",
				Flow: e.ID, Seq: st.Seq, Label: st.Action,
			})
		}
		flows = append(flows, flow)
	}
	return flows, edges, nil
}

func (m *exploreModel) flowSteps(body string) []exploreFlowStep {
	table, err := markdown.ExtractTableFromSection(body, "Steps")
	if err != nil || table == nil {
		return []exploreFlowStep{}
	}
	type raw struct {
		step exploreFlowStep
		row  int
	}
	var raws []raw
	for i, row := range table.Rows {
		from := m.resolveReference(row["From"])
		to := m.resolveReference(row["To"])
		if from == "" || to == "" {
			continue
		}
		seq, err := strconv.Atoi(strings.TrimSpace(row["Seq"]))
		if err != nil {
			seq = i + 1
		}
		raws = append(raws, raw{step: exploreFlowStep{Seq: seq, From: from, To: to, Action: strings.TrimSpace(row["Action"])}, row: i})
	}
	sort.Slice(raws, func(i, j int) bool {
		if raws[i].step.Seq != raws[j].step.Seq {
			return raws[i].step.Seq < raws[j].step.Seq
		}
		return raws[i].row < raws[j].row
	})
	steps := make([]exploreFlowStep, 0, len(raws))
	for i, r := range raws {
		if i > 0 && r.step.Seq <= steps[i-1].Seq {
			r.step.Seq = steps[i-1].Seq + 1
		}
		steps = append(steps, r.step)
	}
	return steps
}

// resolveReference returns the first token of a reference cell that names a
// present node, or "".
func (m *exploreModel) resolveReference(cell string) string {
	cell = strings.TrimSpace(cell)
	if cell == "" || strings.HasPrefix(cell, "N.A") {
		return ""
	}
	for _, tok := range referenceTokens(cell) {
		if m.present[tok] {
			if _, err := m.st.GetEntity(tok); err == nil {
				return tok
			}
		}
	}
	return ""
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
		return "", fmt.Errorf("error: visualize: explorer bundle missing (%w)\nhint: build it with: npm ci --prefix explorer-app && npm run build --prefix explorer-app && cp explorer-app/dist/index.html cli/cmd/assets/explorer/dist/", err)
	}
	return string(shell), nil
}

// injectExplorerPayload writes the live payload (and live-mode flag) into the
// bundle's placeholder script.
func injectExplorerPayload(shell string, payload explorePayload, live bool) (string, error) {
	if !strings.Contains(shell, "/*__C3_DATA__*/") {
		return "", fmt.Errorf("error: visualize: explorer bundle is malformed: /*__C3_DATA__*/ placeholder not found in dist/index.html\nhint: rebuild it with: npm run build --prefix explorer-app && cp explorer-app/dist/index.html cli/cmd/assets/explorer/dist/")
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("error: visualize: marshal payload: %w\nhint: rerun with --schema to inspect the payload contract", err)
	}
	// Neutralize any "</script>" so the JSON cannot break out of its <script> tag.
	safeData := strings.ReplaceAll(string(data), "</", "<\\/")

	out := strings.Replace(shell, "/*__C3_DATA__*/", "window.C3_DATA = "+safeData+";", 1)
	liveJS := ""
	if live {
		liveJS = "window.C3_LIVE = true;"
		if !strings.Contains(out, "/*__C3_LIVE__*/") {
			return "", fmt.Errorf("error: visualize: bundle predates live mode: /*__C3_LIVE__*/ placeholder not found\nhint: rebuild it with: npm run build --prefix explorer-app && cp explorer-app/dist/index.html cli/cmd/assets/explorer/dist/")
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
