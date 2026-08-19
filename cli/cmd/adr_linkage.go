package cmd

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/lagz0ne/c3-design/cli/internal/changeset"
	"github.com/lagz0ne/c3-design/cli/internal/markdown"
	"github.com/lagz0ne/c3-design/cli/internal/schema"
	"github.com/lagz0ne/c3-design/cli/internal/store"
)

type adrAffectedTarget struct {
	ID   string
	Type string
}

type adrCoverage struct {
	refs  map[string][]string
	rules map[string][]string
}

// The trailing "snippet" is OPTIONAL: the sha256 IS the anchor, so the handle
// alone proves the cited content. Allowing snippet-less cites lets you cite a
// table-row block whose snippet would contain `|` and break this very table cell.
// The snippet group captures the surrounding quotes because the emitter writes it
// with %q, so it is a Go-quoted string that must be unquoted to recover the bytes.
var citationHandleRE = regexp.MustCompile(`^([A-Za-z0-9_.:-]+)#n([0-9]+)@v([0-9]+):sha256:([a-f0-9]{64})(?:\s+("(?:[^"\\]|\\.)*"))?$`)

type citationHandle struct {
	EntityID string
	NodeID   int64
	Version  int
	Hash     string
	Snippet  string
}

func parseCitationHandle(raw string) (citationHandle, bool) {
	m := citationHandleRE.FindStringSubmatch(raw)
	if m == nil {
		return citationHandle{}, false
	}
	snippet := ""
	if m[5] != "" {
		unquoted, err := strconv.Unquote(m[5])
		if err != nil {
			return citationHandle{}, false
		}
		snippet = unquoted
	}
	nodeID, _ := strconv.ParseInt(m[2], 10, 64)
	version, _ := strconv.Atoi(m[3])
	return citationHandle{
		EntityID: m[1],
		NodeID:   nodeID,
		Version:  version,
		Hash:     m[4],
		Snippet:  snippet,
	}, true
}

// validateADRCoverage discharges a change doc against the live graph, reading the
// unit's patch folder so a landed retire counts as its own proof.
func validateADRCoverage(s *store.Store, c3Dir, unitID, body, severity string) []Issue {
	return validateADRCoverageMode(s, body, severity, true, retiredByUnit(s, c3Dir, unitID))
}

func validateADRAuthoredCoverage(s *store.Store, body string, severity string) []Issue {
	return validateADRCoverageMode(s, body, severity, false, nil)
}

func validateADRCoverageMode(s *store.Store, body string, severity string, includeMissing bool, retired map[string]bool) []Issue {
	affected, issues := parseADRAffectedTopology(s, body, severity, retired)
	relatedRefs, refIssues := parseADRRelatedTable(s, body, "Compliance Refs", "Ref", "ref", severity)
	issues = append(issues, refIssues...)
	relatedRules, ruleIssues := parseADRRelatedTable(s, body, "Compliance Rules", "Rule", "rule", severity)
	issues = append(issues, ruleIssues...)

	if !includeMissing {
		return issues
	}
	// Touch-nothing -> invalid. A change doc with an empty / all-N.A affected set
	// changes nothing; there is nothing to discharge. Closes the old empty-
	// `expected` early return that let toothless docs pass.
	if len(affected) == 0 {
		issues = append(issues, Issue{
			Severity: severity,
			Message:  "change doc touches nothing: the affected topology is empty or entirely N.A",
			Hint:     "name at least one affected entity in the change-set, or this doc changes nothing",
		})
	}
	// Force top-down completeness. A named component/container owes its
	// higher-level rows (filled or N.A - reason).
	issues = append(issues, topDownCompletenessIssues(s, affected, body, severity)...)
	expected := expectedADRCoverage(s, affected)
	issues = append(issues, missingADRCoverageIssues(expected.refs, relatedRefs, "ref", severity)...)
	issues = append(issues, missingADRCoverageIssues(expected.rules, relatedRules, "rule", severity)...)
	return issues
}

// topDownCompletenessIssues enforces top-down completeness on the affected set:
// a named component (or container) owes a row for every higher-level ancestor
// (its container, its system) — present in the affected set, filled or N.A -
// reason. The descent system->container->component is enforced by walking
// ParentID up to 3 deep. The old UP-walk ancestor-presence check is subsumed by
// this.
func topDownCompletenessIssues(s *store.Store, affected []adrAffectedTarget, body string, severity string) []Issue {
	// All ids named in the change-set's affected set, including N.A-filtered ones
	// only matter as "present"; parse mentions directly so an N.A ancestor row
	// still counts as covered.
	mentioned := mentionedAffectedIDs(body)

	var issues []Issue
	seen := map[string]bool{}
	for _, target := range affected {
		entity, err := s.GetEntity(target.ID)
		if err != nil {
			continue
		}
		// Walk ParentID up to 3 deep; each ancestor must appear in the affected
		// set (delta or N.A - reason).
		current := entity
		for depth := 0; depth < 3; depth++ {
			if current.ParentID == "" {
				break
			}
			parent, err := s.GetEntity(current.ParentID)
			if err != nil {
				break
			}
			key := target.ID + "->" + parent.ID
			if !mentioned[parent.ID] && !seen[key] {
				seen[key] = true
				issues = append(issues, Issue{
					Severity: severity,
					Message:  fmt.Sprintf("top-down incomplete: change doc names %s but omits its higher-level %s %s", target.ID, parent.Type, parent.ID),
					Hint:     fmt.Sprintf("add a row for %s (the delta, or N.A - <reason>) so the change-set descends top-down", parent.ID),
				})
			}
			current = parent
		}
	}
	return issues
}

// changeDocTouchesNothing reports whether every STRICT change-set table row in a
// change doc is entirely N.A — i.e. the doc changes nothing. It reads the STRICT
// (non-FREE, required table) sections from the canvas definition and inspects
// only those. A doc with at least one non-N.A row in any STRICT table touches
// something.
func changeDocTouchesNothing(defs []schema.SectionDef, body string) bool {
	rules := deriveStrictRules(defs)
	sectionMap := map[string]markdown.Section{}
	for _, section := range markdown.ParseSections(body) {
		if section.Name != "" {
			sectionMap[section.Name] = section
		}
	}
	sawRow := false
	for sectionName := range rules.tableHeaders {
		section, ok := sectionMap[sectionName]
		if !ok {
			continue
		}
		table, err := markdown.ParseTable(strings.TrimSpace(section.Content))
		if err != nil {
			continue
		}
		for _, row := range table.Rows {
			sawRow = true
			naCells := 0
			for _, header := range table.Headers {
				// Canonical N.A. predicate: a cell counts as N.A. only when it
				// carries an explicit "N.A - <reason>". A BLANK cell is NOT N.A.
				// (it is undischarged), matching the strict-cell validator's
				// isNAReason. This keeps changeDocTouchesNothing from disagreeing
				// with the strict validator on the same cell.
				if isNAReason(strings.TrimSpace(row[header])) {
					naCells++
				}
			}
			if naCells != len(table.Headers) {
				// At least one non-N.A cell -> this row changes something.
				return false
			}
		}
	}
	// touches nothing only if there were STRICT rows and all were entirely N.A.
	return sawRow
}

// mentionedAffectedIDs collects every Entity id named in the Affected Topology
// table — including rows whose other cells are N.A — so an ancestor row counts
// as "covered" for top-down completeness regardless of whether it is a delta or
// an N.A - reason row.
func mentionedAffectedIDs(body string) map[string]bool {
	mentioned := map[string]bool{}
	for _, section := range markdown.ParseSections(body) {
		if section.Name != "Affected Topology" {
			continue
		}
		table, err := markdown.ParseTable(strings.TrimSpace(section.Content))
		if err != nil {
			return mentioned
		}
		for _, row := range table.Rows {
			id := strings.TrimSpace(row["Entity"])
			if id == "" || isNARow(id) {
				continue
			}
			mentioned[id] = true
		}
	}
	return mentioned
}

// retiredByUnit reports which facts a change unit retired and has already
// destroyed. A retire is the one change that removes its own evidence: once it
// lands there is no node left to cite, so nothing the doc could write in the
// Evidence cell would ever resolve. The unit's applied retire patch is the After
// proof instead — the same "target gone + retire scope ⇒ applied" reading
// changeset.PatchStateOf uses.
//
// A retire still STAGED (target present) is not a discharge: the fact is there,
// so the row owes an ordinary live cite until the switch is actually thrown.
func retiredByUnit(s *store.Store, c3Dir, unitID string) map[string]bool {
	if c3Dir == "" || unitID == "" {
		return nil
	}
	patches, err := changeset.ReadPatchDir(changeUnitDir(c3Dir, unitID))
	if err != nil {
		return nil
	}
	retired := map[string]bool{}
	for _, p := range patches {
		if p.Scope != changeset.ScopeRetire {
			continue
		}
		if _, err := s.GetEntity(p.Target); err != nil {
			retired[p.Target] = true
		}
	}
	return retired
}

// rowNamesRetiredFact reports whether a change-set row names a fact this unit
// retired — the row whose After proof is the retire patch itself. A cell counts
// only when it holds the bare id, which is how every id column is written, so a
// passing prose mention of the retired fact cannot discharge someone else's row.
func rowNamesRetiredFact(row map[string]string, retired map[string]bool) bool {
	for _, cell := range row {
		if retired[strings.TrimSpace(cell)] {
			return true
		}
	}
	return false
}

func parseADRAffectedTopology(s *store.Store, body string, severity string, retired map[string]bool) ([]adrAffectedTarget, []Issue) {
	table, ok, issues := extractADRTable(body, "Affected Topology", severity)
	if !ok {
		return nil, issues
	}
	if table == nil {
		return nil, issues
	}
	var targets []adrAffectedTarget
	for _, row := range table.Rows {
		entityID := strings.TrimSpace(row["Entity"])
		targetType := strings.TrimSpace(row["Type"])
		whyAffected := strings.TrimSpace(row["Why affected"])
		evidence := strings.TrimSpace(row["Evidence"])
		if isNARow(entityID) || isNARow(targetType) {
			continue
		}

		targetResolved := false
		rowRetired := false
		rowUsableAsTarget := true
		switch {
		case entityID == "" || targetType == "":
			rowUsableAsTarget = false
			issues = append(issues, Issue{
				Severity: severity,
				Message:  "Affected Topology rows must include both Entity and Type, or use N.A - <reason>",
				Hint:     "fill the Entity and Type cells for each affected topology row",
			})
		default:
			resolved, err := s.GetEntity(entityID)
			if err != nil {
				if retired[entityID] {
					// This unit retired it: absence IS the landed change, and the row
					// still names a real delta, so it stays a coverage target. Its
					// Evidence can only be the pre-retire handle or N.A — neither can
					// be refreshed against a fact that no longer exists.
					rowRetired = true
					break
				}
				rowUsableAsTarget = false
				if bareID := knownEntityIDPrefix(s, entityID); bareID != "" {
					issues = append(issues, freeFormIDCellIssue("Affected Topology", "Entity", "Why affected", entityID, bareID, severity))
					break
				}
				issues = append(issues, Issue{
					Severity: severity,
					Message:  fmt.Sprintf("Affected Topology references unknown entity: %s", entityID),
					Hint:     "use an existing c3-* ID, or change the row to N.A - <reason>",
				})
				break
			}
			targetResolved = true
			if resolved.Type != targetType {
				rowUsableAsTarget = false
				issues = append(issues, Issue{
					Severity: severity,
					Message:  fmt.Sprintf("Affected Topology type mismatch: %s is %s, not %s", entityID, resolved.Type, targetType),
					Hint:     "align the Type column with the referenced entity kind",
				})
			}
		}

		// An explicit "N.A - <reason>" Why is the escape hatch: the row still counts
		// for top-down completeness, but it names no delta, so it neither becomes a
		// coverage target (no subtree descent) nor owes a fresh cite. A BLANK Why is
		// not an escape hatch — it is simply undischarged.
		rowExcusedAsNA := isNAReason(whyAffected)
		switch {
		case rowExcusedAsNA:
			rowUsableAsTarget = false
		case whyAffected == "" || isNARow(whyAffected):
			rowUsableAsTarget = false
			issues = append(issues, Issue{
				Severity: severity,
				Message:  fmt.Sprintf("Affected Topology row for %s must explain why it is affected", entityID),
				Hint:     "fill the Why affected column with the concrete reason, or mark the entire row N.A - <reason>",
			})
		}

		allowNAEvidence := evidenceNARejected
		if rowExcusedAsNA || rowRetired {
			allowNAEvidence = evidenceNAAllowed
		}
		if targetResolved {
			issues = append(issues, validateADREvidence(s, "Affected Topology", entityID, evidence, severity, allowNAEvidence)...)
		} else {
			_, _, cellShapeIssues := validateADREvidenceCellShape("Affected Topology", entityID, evidence, severity, allowNAEvidence)
			issues = append(issues, cellShapeIssues...)
		}

		if rowUsableAsTarget {
			targets = append(targets, adrAffectedTarget{ID: entityID, Type: targetType})
		}
	}
	return targets, issues
}

func knownEntityIDPrefix(s *store.Store, cell string) string {
	tokens := strings.Fields(cell)
	cellIsDecorated := len(tokens) > 1
	if !cellIsDecorated {
		return ""
	}
	if _, err := s.GetEntity(tokens[0]); err != nil {
		return ""
	}
	return tokens[0]
}

func freeFormIDCellIssue(sectionName, colName, proseCol, cell, bareID, severity string) Issue {
	return Issue{
		Severity: severity,
		Message:  fmt.Sprintf("%s %s cell must contain only the bare id, got free-form text: %s", sectionName, colName, cell),
		Hint:     fmt.Sprintf("use just %s in the %s cell and move the rest into %s", bareID, colName, proseCol),
	}
}

func parseADRRelatedTable(s *store.Store, body, sectionName, colName, targetType, severity string) (map[string]bool, []Issue) {
	table, ok, issues := extractADRTable(body, sectionName, severity)
	if !ok {
		return nil, issues
	}
	if table == nil {
		return nil, issues
	}
	mentioned := make(map[string]bool, len(table.Rows))
	for _, row := range table.Rows {
		targetID := strings.TrimSpace(row[colName])
		whyRequired := strings.TrimSpace(row["Why required"])
		action := strings.ToLower(strings.TrimSpace(row["Action"]))
		evidence := strings.TrimSpace(row["Evidence"])
		if isNARow(targetID) {
			continue
		}

		targetResolved := false
		targetWillBeCreated := false
		rowUsableAsTarget := true
		switch {
		case targetID == "":
			rowUsableAsTarget = false
			issues = append(issues, Issue{
				Severity: severity,
				Message:  fmt.Sprintf("%s rows must include %s, or use N.A - <reason>", sectionName, colName),
				Hint:     fmt.Sprintf("fill the %s column for each %s row", colName, sectionName),
			})
		default:
			resolved, err := s.GetEntity(targetID)
			if err != nil {
				rowUsableAsTarget = false
				bareID := knownEntityIDPrefix(s, targetID)
				switch {
				case bareID != "":
					issues = append(issues, freeFormIDCellIssue(sectionName, colName, "Why required", targetID, bareID, severity))
				case strings.Contains(action, "create"):
					targetWillBeCreated = true
				default:
					issues = append(issues, Issue{
						Severity: severity,
						Message:  fmt.Sprintf("%s references unknown %s: %s", sectionName, targetType, targetID),
						Hint:     fmt.Sprintf("create %s first, or mark the Action as create-%s", targetID, targetType),
					})
				}
				break
			}
			targetResolved = true
			if resolved.Type != targetType {
				rowUsableAsTarget = false
				issues = append(issues, Issue{
					Severity: severity,
					Message:  fmt.Sprintf("%s type mismatch: %s is %s, not %s", sectionName, targetID, resolved.Type, targetType),
					Hint:     fmt.Sprintf("move %s to the correct ADR section", targetID),
				})
			}
		}

		if whyRequired == "" || isNARow(whyRequired) {
			rowUsableAsTarget = false
			issues = append(issues, Issue{
				Severity: severity,
				Message:  fmt.Sprintf("%s row for %s must explain why compliance/review is required", sectionName, targetID),
				Hint:     "fill the Why required column with the compliance reason, or mark the entire row N.A - <reason>",
			})
		}

		switch {
		case targetResolved:
			issues = append(issues, validateADREvidence(s, sectionName, targetID, evidence, severity, evidenceNARejected)...)
		case targetWillBeCreated:
			issues = append(issues, validateADREvidence(s, sectionName, targetID, evidence, severity, evidenceNAAllowed)...)
		default:
			_, _, cellShapeIssues := validateADREvidenceCellShape(sectionName, targetID, evidence, severity, evidenceNARejected)
			issues = append(issues, cellShapeIssues...)
		}

		if rowUsableAsTarget {
			mentioned[targetID] = true
		}
	}
	return mentioned, issues
}

const (
	evidenceNARejected = false
	evidenceNAAllowed  = true
)

func validateADREvidenceCellShape(sectionName, targetID, raw string, severity string, allowNA bool) (handle citationHandle, parsed bool, issues []Issue) {
	if raw == "" {
		return citationHandle{}, false, []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("%s row for %s must include Evidence citation", sectionName, targetID),
			Hint:     fmt.Sprintf("run %s and paste the matching handle, or use N.A - <reason> only when creating a new target", nodeCiteCommand(targetID)),
		}}
	}
	if strings.HasPrefix(raw, "N.A -") {
		if allowNA {
			return citationHandle{}, false, nil
		}
		return citationHandle{}, false, []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("%s row for %s must cite current C3 evidence, not N.A", sectionName, targetID),
			Hint:     fmt.Sprintf("run %s and paste the matching handle", nodeCiteCommand(targetID)),
		}}
	}
	cite, ok := parseCitationHandle(raw)
	if !ok {
		return citationHandle{}, false, []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("%s row for %s has invalid Evidence citation", sectionName, targetID),
			Hint:     fmt.Sprintf(`expected <entity>#n<node>@v<version>:sha256:<nodeHash> "exact snippet" from %s`, nodeCiteCommand(targetID)),
		}}
	}
	return cite, true, nil
}

func validateADREvidence(s *store.Store, sectionName, targetID, raw string, severity string, allowNA bool) []Issue {
	cite, parsed, issues := validateADREvidenceCellShape(sectionName, targetID, raw, severity, allowNA)
	if !parsed {
		return issues
	}

	citedEntity := cite.EntityID
	nodeID := cite.NodeID
	version := cite.Version
	hash := cite.Hash
	snippet := cite.Snippet

	if citedEntity != targetID {
		return []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("Evidence for %s row %s cites %s, want %s", sectionName, targetID, citedEntity, targetID),
			Hint:     "use evidence generated from the row target, not a nearby document",
		}}
	}

	entity, err := s.GetEntity(citedEntity)
	if err != nil {
		return []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("Evidence for %s row %s cites unknown entity %s", sectionName, targetID, citedEntity),
			Hint:     "create the target first, or use N.A - <reason> with a create action",
		}}
	}
	if entity.Version != version {
		return []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("Evidence for %s row %s cites version %d, current version is %d", sectionName, targetID, version, entity.Version),
			Hint:     fmt.Sprintf("refresh the handle with %s", nodeCiteCommand(targetID)),
		}}
	}

	outcome, node := resolveEvidenceNode(s, citedEntity, nodeID, hash, snippet)
	switch outcome {
	case evidenceNodeOK:
		return nil
	case evidenceNodeSnippetMismatch:
		return []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("Evidence for %s row %s has a snippet that does not match: the cited sha256 is current, but that node begins %q, not %q", sectionName, targetID, citeExcerpt(node.Content), citeExcerpt(snippet)),
			Hint:     fmt.Sprintf("re-copy the snippet from %s, or drop it: the sha256 is the anchor and the snippet is optional", nodeCiteCommand(targetID)),
		}}
	}
	// Only a node that genuinely carries the cited hash proves a cross-document
	// cite. Node ids renumber on `change apply`, so a stale id landing on another
	// entity is a coincidence — reporting it as a foreign citation hides the real
	// cause, the stale hash below.
	if other, err := s.GetNode(nodeID); err == nil && other.EntityID != citedEntity && other.Hash == hash {
		return []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("Evidence for %s row %s cites node %d from %s", sectionName, targetID, nodeID, other.EntityID),
			Hint:     fmt.Sprintf("refresh the handle with %s", nodeCiteCommand(targetID)),
		}}
	}
	return []Issue{{
		Severity: severity,
		Message:  fmt.Sprintf("Evidence for %s row %s has a stale cite (no node of %s seals to that hash)", sectionName, targetID, citedEntity),
		Hint:     fmt.Sprintf("refresh the handle with %s", nodeCiteCommand(targetID)),
	}}
}

type evidenceNodeOutcome int

const (
	evidenceNodeOK evidenceNodeOutcome = iota
	evidenceNodeHashUnknown
	evidenceNodeSnippetMismatch
)

func resolveEvidenceNode(s *store.Store, entityID string, nodeID int64, hash, snippet string) (evidenceNodeOutcome, *store.Node) {
	// The sha256 is the anchor; a snippet, when present, must also be contained.
	// Matching by hash across all of the entity's nodes makes a cite resilient to
	// node-id renumbering (same content, new integer id).
	var hashMatchWithWrongSnippet *store.Node
	matches := func(node *store.Node) bool {
		if node.Hash != hash {
			return false
		}
		if snippet == "" || strings.Contains(node.Content, snippet) {
			return true
		}
		if hashMatchWithWrongSnippet == nil {
			hashMatchWithWrongSnippet = node
		}
		return false
	}
	if node, err := s.GetNode(nodeID); err == nil && node.EntityID == entityID && matches(node) {
		return evidenceNodeOK, node
	}
	if nodes, err := s.NodesForEntity(entityID); err == nil {
		for _, node := range nodes {
			if matches(node) {
				return evidenceNodeOK, node
			}
		}
	}
	if hashMatchWithWrongSnippet != nil {
		return evidenceNodeSnippetMismatch, hashMatchWithWrongSnippet
	}
	return evidenceNodeHashUnknown, nil
}

// Bare `c3x read <id> --cite` emits the entity-root handle, which the cite grammar rejects.
func nodeCiteCommand(id string) string {
	return fmt.Sprintf("c3x read %s --section <name> --cite", id)
}

const citeExcerptMaxLen = 80

func citeExcerpt(text string) string {
	excerpt := strings.TrimSpace(text)
	if i := strings.IndexByte(excerpt, '\n'); i >= 0 {
		excerpt = strings.TrimSpace(excerpt[:i])
	}
	if truncated, cut := truncateRunes(excerpt, citeExcerptMaxLen); cut {
		excerpt = truncated + "..."
	}
	return excerpt
}

func extractADRTable(body, sectionName, severity string) (*markdown.Table, bool, []Issue) {
	for _, section := range markdown.ParseSections(body) {
		if section.Name != sectionName {
			continue
		}
		table, err := markdown.ParseTable(strings.TrimSpace(section.Content))
		if err != nil {
			return nil, true, []Issue{{
				Severity: severity,
				Message:  fmt.Sprintf("invalid ADR table: %s", sectionName),
				Hint:     fmt.Sprintf("use the exact table columns from %s", adrSchemaHint()),
			}}
		}
		return table, true, nil
	}
	return nil, false, nil
}

func expectedADRCoverage(s *store.Store, affected []adrAffectedTarget) adrCoverage {
	coverage := adrCoverage{
		refs:  map[string][]string{},
		rules: map[string][]string{},
	}
	for _, target := range affected {
		collectADRCoverageForEntity(s, coverage, target.ID)
	}
	return coverage
}

func collectADRCoverageForEntity(s *store.Store, coverage adrCoverage, entityID string) {
	entity, err := s.GetEntity(entityID)
	if err != nil {
		return
	}
	switch entity.Type {
	case "system":
		children, _ := s.Children(entityID)
		for _, child := range children {
			if child.Type == "container" {
				collectADRCoverageForEntity(s, coverage, child.ID)
			}
		}
	case "container":
		collectScopedRefs(s, coverage.refs, entityID, fmt.Sprintf("scoped to %s", entityID))
		children, _ := s.Children(entityID)
		for _, child := range children {
			if child.Type == "component" || child.Type == "container" {
				collectADRCoverageForEntity(s, coverage, child.ID)
			}
		}
	case "component":
		if entity.ParentID != "" {
			collectScopedRefs(s, coverage.refs, entity.ParentID, fmt.Sprintf("scoped to %s via %s", entity.ParentID, entity.ID))
		}
		rels, _ := s.RelationshipsFrom(entityID)
		for _, rel := range rels {
			if rel.RelType != "uses" {
				continue
			}
			switch citationType(s, rel.ToID) {
			case "ref":
				coverage.refs[rel.ToID] = appendUniqueString(coverage.refs[rel.ToID], fmt.Sprintf("cited by %s", entityID))
			case "rule":
				coverage.rules[rel.ToID] = appendUniqueString(coverage.rules[rel.ToID], fmt.Sprintf("cited by %s", entityID))
			}
		}
	}
}

// citationType classifies a citation target by its real entity type from the
// store, so linkage does not depend on the id-prefix naming convention. It
// falls back to the prefix only when the entity is absent (dangling citation),
// preserving prior behavior on malformed input.
func citationType(s *store.Store, id string) string {
	if e, err := s.GetEntity(id); err == nil {
		return e.Type
	}
	switch {
	case strings.HasPrefix(id, "ref-"):
		return "ref"
	case strings.HasPrefix(id, "rule-"):
		return "rule"
	case strings.HasPrefix(id, "adr-"):
		return "adr"
	default:
		return ""
	}
}

func collectScopedRefs(s *store.Store, target map[string][]string, entityID, reason string) {
	rels, _ := s.RelationshipsTo(entityID)
	for _, rel := range rels {
		if rel.RelType != "scope" || citationType(s, rel.FromID) != "ref" {
			continue
		}
		target[rel.FromID] = appendUniqueString(target[rel.FromID], reason)
	}
}

func missingADRCoverageIssues(expected map[string][]string, mentioned map[string]bool, targetType, severity string) []Issue {
	if len(expected) == 0 {
		return nil
	}
	var ids []string
	for id := range expected {
		if mentioned[id] {
			continue
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var issues []Issue
	for _, id := range ids {
		issues = append(issues, Issue{
			Severity: severity,
			Message:  fmt.Sprintf("ADR missing compliance %s %s (%s)", targetType, id, strings.Join(expected[id], "; ")),
			Hint:     fmt.Sprintf("add %s to the ADR's compliance %ss with why it must be reviewed/complied with, or document why it is N.A", id, targetType),
		})
	}
	return issues
}

func isNARow(value string) bool {
	value = strings.TrimSpace(value)
	return value == "" || strings.HasPrefix(value, "N.A -")
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

// isADRTerminal reports whether an ADR status is a terminal (historical) state.
// Terminal-state ADRs are exempt from check validation; their content is frozen.
func isADRTerminal(status string) bool {
	return status == "implemented" || status == "provisioned"
}

// isChangeDocTerminal reports whether a change doc is in a terminal (frozen,
// historical) state, keying on its DECLARED status. It generalizes
// isADRTerminal from ADR-only to any change doc: terminal for the canonical
// change-doc set {done, superseded} and for the migrated legacy ADR terminals
// {implemented, provisioned}. A terminal change doc is skipped by the default
// check; its content is frozen.
func isChangeDocTerminal(entity *store.Entity) bool {
	if entity == nil {
		return false
	}
	switch entity.Status {
	case "done", "superseded":
		return true
	default:
		return isADRTerminal(entity.Status)
	}
}

// autoDoneLatch is the one-way actualization latch (SETTLED — reverses "never
// auto-set done"). For an `accepted` change doc it gathers every per-row After
// cite from the STRICT change-set (the cite-typed columns of the non-FREE table
// sections) and resolves each with the freshness/intactness machinery
// (validateCitationColumnValue). The latch is two-phase, gated by `commit`:
//
//   - commit==false (plain `check`): READINESS ONLY. If at least one After cite is
//     present and ALL resolve fresh, it returns flipped=true WITHOUT touching the
//     status column — the caller reports readiness; it never mutates the DB or
//     rewrites sealed markdown on a plain read.
//   - commit==true (`check --fix`): it ACTUALIZES the flip accepted->done via the
//     sanctioned SetEntityStatus writer (status is edit-proof) and returns
//     flipped=true.
//
// Any stale/missing cite blocks the flip and is returned as unresolved. The latch
// is purely mechanical: it never fires on a non-`accepted` doc, never flips
// backward, and never judges whether the chosen After conditions were the right
// success criteria.
func autoDoneLatch(s *store.Store, c3Dir string, entity *store.Entity, body string, commit bool) (bool, []Issue) {
	if entity == nil || entity.Status != "accepted" {
		return false, nil
	}
	if !schema.IsChangeDocDir(c3Dir, entity.Type) {
		return false, nil
	}

	// Resolve the same canvas the rest of check uses (DefinitionForDir prefers a
	// project-local .c3/canvases override): otherwise, for a user with a custom
	// change-doc canvas, the latch would scan the BUILT-IN columns and resolve the
	// wrong cite set. c3Dir == "" falls back to the built-in definition.
	def, ok := schema.DefinitionForDir(c3Dir, entity.Type)
	if !ok {
		return false, nil
	}

	sectionMap := map[string]markdown.Section{}
	for _, sec := range markdown.ParseSections(body) {
		if sec.Name != "" {
			sectionMap[sec.Name] = sec
		}
	}

	opts := CheckOptions{Store: s}
	retired := retiredByUnit(s, c3Dir, entity.ID)
	var unresolved []Issue
	afterCites := 0

	for _, sd := range def.Sections {
		if sd.Free || sd.ContentType != "table" {
			continue
		}
		citeCols := map[string]bool{}
		for _, col := range sd.Columns {
			if col.Type == "cite" {
				citeCols[col.Name] = true
			}
		}
		if len(citeCols) == 0 {
			continue
		}
		sec, exists := sectionMap[sd.Name]
		if !exists {
			continue
		}
		table, err := markdown.ParseTable(strings.TrimSpace(sec.Content))
		if err != nil {
			continue
		}
		for _, row := range table.Rows {
			// A row naming a fact this unit retired is already discharged: the
			// applied retire patch is its After proof, and it counts as one, so a
			// unit whose whole decision was a retire can still latch.
			if rowNamesRetiredFact(row, retired) {
				afterCites++
				continue
			}
			for col := range citeCols {
				raw := strings.TrimSpace(row[col])
				// An "N.A - <reason>" cell is a declared non-cite, not a broken
				// handle: it proves nothing, so it neither counts nor blocks.
				if raw == "" || isNAReason(raw) {
					continue
				}
				afterCites++
				unresolved = append(unresolved, validateCitationColumnValue(raw, entity, opts)...)
			}
		}
	}

	if afterCites == 0 || len(unresolved) > 0 {
		return false, unresolved
	}

	// Ready to actualize. A plain `check` (commit==false) only REPORTS readiness:
	// it never flips, never mutates the DB, never rewrites sealed markdown.
	if !commit {
		return true, nil
	}

	if err := s.SetEntityStatus(entity.ID, "done"); err != nil {
		return false, []Issue{{
			Severity: "error",
			Entity:   entity.ID,
			Message:  fmt.Sprintf("auto-done latch: failed to actualize %s to done: %v", entity.ID, err),
		}}
	}
	entity.Status = "done"
	return true, nil
}

// statusTransitions is the canonical legal-jump table for the status command.
//
// Change-doc canonical set {open, accepted, done, superseded}:
//
//	open      -> accepted | superseded
//	accepted  -> done | superseded
//	done      -> superseded
//	superseded-> (terminal)
//
// Legacy ADR states are preserved so existing ADR transitions stay legal:
//
//	proposed    -> accepted | provisioned | superseded
//	accepted    -> implemented (in addition to done/superseded)
//	implemented -> superseded
//	provisioned -> superseded
//
// `*->superseded` is reachable only via the supersede command.
var statusTransitions = map[string][]string{
	"open":        {"accepted", "superseded"},
	"accepted":    {"done", "implemented", "superseded"},
	"done":        {"superseded"},
	"superseded":  {},
	"proposed":    {"accepted", "provisioned", "superseded"},
	"implemented": {"superseded"},
	"provisioned": {"superseded"},
}

// legalNextStates returns the states reachable from `from` in one legal jump.
// Unknown source states yield no legal next states.
func legalNextStates(from string) []string {
	return statusTransitions[from]
}

// statusTransitionLegal reports whether from->to is a legal one-step jump.
// A no-op (from == to) is always legal.
func statusTransitionLegal(from, to string) bool {
	if from == to {
		return true
	}
	return slices.Contains(legalNextStates(from), to)
}

// mapADRStatus folds a legacy ADR status onto the change-doc canonical set,
// reporting whether the fold is lossy (a distinction collapsed). The lossy
// signal is recorded for the migration sweep; this helper performs no
// coercion of stored state on its own.
//
//	proposed    -> open        (clean)
//	accepted    -> accepted    (clean)
//	implemented -> done        (clean)
//	provisioned -> done        (LOSSY: the design-only distinction is collapsed)
func mapADRStatus(status string) (mapped string, lossy bool) {
	switch status {
	case "proposed":
		return "open", false
	case "accepted":
		return "accepted", false
	case "implemented":
		return "done", false
	case "provisioned":
		return "done", true
	default:
		return status, false
	}
}
