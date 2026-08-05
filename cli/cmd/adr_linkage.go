package cmd

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

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
var citationHandleRE = regexp.MustCompile(`^([A-Za-z0-9_.:-]+)#n([0-9]+)@v([0-9]+):sha256:([a-f0-9]{64})(?:\s+"(.*)")?$`)

func validateADRCoverage(s *store.Store, body string, severity string) []Issue {
	return validateADRCoverageMode(s, body, severity, true)
}

func validateADRAuthoredCoverage(s *store.Store, body string, severity string) []Issue {
	return validateADRCoverageMode(s, body, severity, false)
}

func validateADRCoverageMode(s *store.Store, body string, severity string, includeMissing bool) []Issue {
	schemaCommand := adrSchemaHint()
	affected, issues := parseADRAffectedTopology(s, body, severity, schemaCommand)
	relatedRefs, refIssues := parseADRRelatedTable(s, body, "Compliance Refs", "Ref", "ref", severity, schemaCommand)
	issues = append(issues, refIssues...)
	relatedRules, ruleIssues := parseADRRelatedTable(s, body, "Compliance Rules", "Rule", "rule", severity, schemaCommand)
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
	issues = append(issues, topDownCompletenessIssues(s, body, severity)...)
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
func topDownCompletenessIssues(s *store.Store, body string, severity string) []Issue {
	affected, _ := parseADRAffectedTopology(s, body, severity, adrSchemaHint())
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

func parseADRAffectedTopology(s *store.Store, body string, severity string, schemaCommand string) ([]adrAffectedTarget, []Issue) {
	table, ok, issues := extractADRTable(body, "Affected Topology", severity, schemaCommand)
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

		// One pass per row: the cells fail independently, so collect every
		// finding instead of bailing at the first. Bailing made a blank Why hide
		// the row's Evidence defect, costing a submission round-trip per class.
		var entity *store.Entity
		rowResolved := true
		switch {
		case entityID == "" || targetType == "":
			rowResolved = false
			issues = append(issues, Issue{
				Severity: severity,
				Message:  "Affected Topology rows must include both Entity and Type, or use N.A - <reason>",
				Hint:     "fill the Entity and Type cells for each affected topology row",
			})
		default:
			resolved, err := s.GetEntity(entityID)
			if err != nil {
				rowResolved = false
				if bare := bareIDFromCell(s, entityID); bare != "" {
					issues = append(issues, freeFormIDCellIssue("Affected Topology", "Entity", "Why affected", entityID, bare, severity))
					break
				}
				issues = append(issues, Issue{
					Severity: severity,
					Message:  fmt.Sprintf("Affected Topology references unknown entity: %s", entityID),
					Hint:     "use an existing c3-* ID, or change the row to N.A - <reason>",
				})
				break
			}
			entity = resolved
			if resolved.Type != targetType {
				rowResolved = false
				issues = append(issues, Issue{
					Severity: severity,
					Message:  fmt.Sprintf("Affected Topology type mismatch: %s is %s, not %s", entityID, resolved.Type, targetType),
					Hint:     "align the Type column with the referenced entity kind",
				})
			}
		}

		if whyAffected == "" || isNARow(whyAffected) {
			rowResolved = false
			issues = append(issues, Issue{
				Severity: severity,
				Message:  fmt.Sprintf("Affected Topology row for %s must explain why it is affected", entityID),
				Hint:     "fill the Why affected column with the concrete reason, or mark the entire row N.A - <reason>",
			})
		}

		// Deliberate dependency, not an early bail: the version/hash/snippet
		// checks compare Evidence AGAINST the row target, so an Entity cell that
		// did not resolve leaves only the cell-shape checks runnable.
		if entity != nil {
			issues = append(issues, validateADREvidence(s, "Affected Topology", entityID, evidence, severity, false)...)
		} else {
			_, shapeIssues := validateADREvidenceShape("Affected Topology", entityID, evidence, severity, false)
			issues = append(issues, shapeIssues...)
		}

		if rowResolved {
			targets = append(targets, adrAffectedTarget{ID: entityID, Type: targetType})
		}
	}
	return targets, issues
}

// bareIDFromCell reports the first whitespace-separated token of an id cell when
// that token resolves to a known entity — i.e. the author wrote a DECORATED cell
// ("c3-3 shared (prompt.ts)") rather than naming something that does not exist.
// Empty when the cell is a single token or nothing in it resolves, so the caller
// keeps the plain unknown-target finding.
func bareIDFromCell(s *store.Store, cell string) string {
	fields := strings.Fields(cell)
	if len(fields) < 2 {
		return ""
	}
	if _, err := s.GetEntity(fields[0]); err != nil {
		return ""
	}
	return fields[0]
}

// freeFormIDCellIssue names the id it found instead of interpolating the whole
// cell as one. The old "unknown entity: c3-3 shared (prompt.ts)" sent authors
// hunting for a missing entity when the real fix was to trim the cell.
func freeFormIDCellIssue(sectionName, colName, proseCol, cell, bareID, severity string) Issue {
	return Issue{
		Severity: severity,
		Message:  fmt.Sprintf("%s %s cell must contain only the bare id, got free-form text: %s", sectionName, colName, cell),
		Hint:     fmt.Sprintf("use just %s in the %s cell and move the rest into %s", bareID, colName, proseCol),
	}
}

func parseADRRelatedTable(s *store.Store, body, sectionName, colName, targetType, severity string, schemaCommand string) (map[string]bool, []Issue) {
	table, ok, issues := extractADRTable(body, sectionName, severity, schemaCommand)
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

		// One pass per row, same as parseADRAffectedTopology: independent cells
		// yield independent findings.
		var entity *store.Entity
		creating := false
		rowResolved := true
		switch {
		case targetID == "":
			rowResolved = false
			issues = append(issues, Issue{
				Severity: severity,
				Message:  fmt.Sprintf("%s rows must include %s, or use N.A - <reason>", sectionName, colName),
				Hint:     fmt.Sprintf("fill the %s column for each %s row", colName, sectionName),
			})
		default:
			resolved, err := s.GetEntity(targetID)
			if err != nil {
				rowResolved = false
				bare := bareIDFromCell(s, targetID)
				switch {
				case bare != "":
					issues = append(issues, freeFormIDCellIssue(sectionName, colName, "Why required", targetID, bare, severity))
				case strings.Contains(action, "create"):
					// A create action legitimately names a target that does not
					// exist yet, so its Evidence may be N.A.
					creating = true
				default:
					issues = append(issues, Issue{
						Severity: severity,
						Message:  fmt.Sprintf("%s references unknown %s: %s", sectionName, targetType, targetID),
						Hint:     fmt.Sprintf("create %s first, or mark the Action as create-%s", targetID, targetType),
					})
				}
				break
			}
			entity = resolved
			if resolved.Type != targetType {
				rowResolved = false
				issues = append(issues, Issue{
					Severity: severity,
					Message:  fmt.Sprintf("%s type mismatch: %s is %s, not %s", sectionName, targetID, resolved.Type, targetType),
					Hint:     fmt.Sprintf("move %s to the correct ADR section", targetID),
				})
			}
		}

		if whyRequired == "" || isNARow(whyRequired) {
			rowResolved = false
			issues = append(issues, Issue{
				Severity: severity,
				Message:  fmt.Sprintf("%s row for %s must explain why compliance/review is required", sectionName, targetID),
				Hint:     "fill the Why required column with the compliance reason, or mark the entire row N.A - <reason>",
			})
		}

		// Deliberate dependency: only a resolved (or being-created) target lets
		// Evidence be checked against it; otherwise just the cell-shape checks.
		switch {
		case entity != nil:
			issues = append(issues, validateADREvidence(s, sectionName, targetID, evidence, severity, false)...)
		case creating:
			issues = append(issues, validateADREvidence(s, sectionName, targetID, evidence, severity, true)...)
		default:
			_, shapeIssues := validateADREvidenceShape(sectionName, targetID, evidence, severity, false)
			issues = append(issues, shapeIssues...)
		}

		if rowResolved {
			mentioned[targetID] = true
		}
	}
	return mentioned, issues
}

// validateADREvidenceShape runs the Evidence checks that need nothing but the
// cell text — present, not N.A, parseable as a cite handle — and returns the
// parsed handle when there is one. Split out so a row whose target does not
// resolve still gets its Evidence reported in the same pass; only the
// target-relative checks are genuinely blocked.
func validateADREvidenceShape(sectionName, targetID, raw string, severity string, allowNA bool) ([]string, []Issue) {
	if raw == "" {
		return nil, []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("%s row for %s must include Evidence citation", sectionName, targetID),
			Hint:     fmt.Sprintf("run %s and paste the matching handle, or use N.A - <reason> only when creating a new target", nodeCiteCommand(targetID)),
		}}
	}
	if strings.HasPrefix(raw, "N.A -") {
		if allowNA {
			return nil, nil
		}
		return nil, []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("%s row for %s must cite current C3 evidence, not N.A", sectionName, targetID),
			Hint:     fmt.Sprintf("run %s and paste the matching handle", nodeCiteCommand(targetID)),
		}}
	}
	m := citationHandleRE.FindStringSubmatch(raw)
	if m == nil {
		return nil, []Issue{{
			Severity: severity,
			Message:  fmt.Sprintf("%s row for %s has invalid Evidence citation", sectionName, targetID),
			Hint:     fmt.Sprintf(`expected <entity>#n<node>@v<version>:sha256:<nodeHash> "exact snippet" from %s`, nodeCiteCommand(targetID)),
		}}
	}
	return m, nil
}

func validateADREvidence(s *store.Store, sectionName, targetID, raw string, severity string, allowNA bool) []Issue {
	m, issues := validateADREvidenceShape(sectionName, targetID, raw, severity, allowNA)
	if m == nil {
		return issues
	}

	citedEntity := m[1]
	nodeID, _ := strconv.ParseInt(m[2], 10, 64)
	version, _ := strconv.Atoi(m[3])
	hash := m[4]
	snippet := m[5]

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

	outcome, node := evidenceNodeMatches(s, citedEntity, nodeID, hash, snippet)
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
	if other, err := s.GetNode(nodeID); err == nil && other.EntityID != citedEntity {
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

// evidenceNodeOutcome names WHICH half of a cite failed. The two causes are
// independent and want opposite remedies — a hash no node seals to is stale and
// wants a refresh, while a current hash with a non-matching snippet wants the
// snippet re-copied — so collapsing them into one bool made the reported cause
// affirmatively false half the time.
type evidenceNodeOutcome int

const (
	evidenceNodeOK evidenceNodeOutcome = iota
	evidenceNodeHashUnknown
	evidenceNodeSnippetMismatch
)

// evidenceNodeMatches resolves a cite against an entity's nodes, returning the
// hash-matching node when there is one so the caller can show its actual text.
func evidenceNodeMatches(s *store.Store, entityID string, nodeID int64, hash, snippet string) (evidenceNodeOutcome, *store.Node) {
	// The sha256 is the anchor; a snippet, when present, must also be contained.
	// Matching by hash across all of the entity's nodes makes a cite resilient to
	// node-id renumbering (same content, new integer id).
	var hashOnly *store.Node
	matches := func(node *store.Node) bool {
		if node.Hash != hash {
			return false
		}
		if snippet == "" || strings.Contains(node.Content, snippet) {
			return true
		}
		if hashOnly == nil {
			hashOnly = node
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
	if hashOnly != nil {
		return evidenceNodeSnippetMismatch, hashOnly
	}
	return evidenceNodeHashUnknown, nil
}

// nodeCiteCommand names the invocation that actually emits a per-NODE handle.
// Bare `c3x read <id> --cite` emits the ENTITY-ROOT handle, which the cite
// grammar rejects — hints that named it sent readers in a circle, re-fetching a
// value this validator had just refused.
func nodeCiteCommand(id string) string {
	return fmt.Sprintf("c3x read %s --section <name> --cite", id)
}

// citeExcerpt reduces a snippet or node body to one short single-line excerpt,
// so a cited-vs-actual comparison fits on an error line instead of dumping the
// whole node.
func citeExcerpt(text string) string {
	excerpt := strings.TrimSpace(text)
	if i := strings.IndexByte(excerpt, '\n'); i >= 0 {
		excerpt = strings.TrimSpace(excerpt[:i])
	}
	if len(excerpt) > 80 {
		excerpt = excerpt[:80] + "..."
	}
	return excerpt
}

func extractADRTable(body, sectionName, severity string, schemaCommand string) (*markdown.Table, bool, []Issue) {
	for _, section := range markdown.ParseSections(body) {
		if section.Name != sectionName {
			continue
		}
		table, err := markdown.ParseTable(strings.TrimSpace(section.Content))
		if err != nil {
			return nil, true, []Issue{{
				Severity: severity,
				Message:  fmt.Sprintf("invalid ADR table: %s", sectionName),
				Hint:     fmt.Sprintf("use the exact table columns from %s", schemaCommand),
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
			for col := range citeCols {
				raw := strings.TrimSpace(row[col])
				if raw == "" {
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
