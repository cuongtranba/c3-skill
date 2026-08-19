package index

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lagz0ne/c3-design/cli/internal/codemap"
	"github.com/lagz0ne/c3-design/cli/internal/store"
)

const structuralFileName = "_index/structural.md"

// Write builds a structural index from s and codeBindings and writes it to
// <c3Dir>/_index/structural.md. codeBindings may be nil (no code section emitted).
func Write(c3Dir string, s *store.Store, codeBindings codemap.CodeMap) error {
	entities, err := s.AllEntities()
	if err != nil {
		return fmt.Errorf("index: list entities: %w", err)
	}

	body := render(entities, s, codeBindings)

	path := filepath.Join(c3Dir, filepath.FromSlash(structuralFileName))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("index: mkdir: %w", err)
	}
	return os.WriteFile(path, []byte(body), 0644)
}

func render(entities []*store.Entity, s *store.Store, codeBindings codemap.CodeMap) string {
	sort.Slice(entities, func(i, j int) bool { return entities[i].ID < entities[j].ID })

	var sb strings.Builder
	sb.WriteString("# C3 Structural Index\n")

	for _, e := range entities {
		if e.Type == "adr" || e.Status == "retired" {
			continue
		}
		writeEntityBlock(&sb, e, s, codeBindings)
	}

	writeFileMap(&sb, entities, codeBindings)

	content := sb.String()
	hash := contentHash(content)

	// Insert hash comment on line 2 (after header).
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) == 2 {
		return lines[0] + "\n" + fmt.Sprintf("<!-- hash: sha256:%s -->", hash) + "\n" + lines[1]
	}
	return content
}

func writeEntityBlock(sb *strings.Builder, e *store.Entity, s *store.Store, codeBindings codemap.CodeMap) {
	fmt.Fprintf(sb, "\n## %s — %s (%s)\n", e.ID, e.Title, e.Type)

	switch e.Type {
	case "component":
		if e.ParentID != "" {
			containerLine := "container: " + e.ParentID
			if parent, err := s.GetEntity(e.ParentID); err == nil && parent.ParentID != "" {
				containerLine += " | context: " + parent.ParentID
			}
			fmt.Fprintf(sb, "%s\n", containerLine)
		}
	case "container":
		if e.ParentID != "" {
			fmt.Fprintf(sb, "context: %s\n", e.ParentID)
		}
	}

	refs, _ := s.RefsFor(e.ID)
	if len(refs) > 0 {
		ids := make([]string, 0, len(refs))
		for _, r := range refs {
			ids = append(ids, r.ID)
		}
		sort.Strings(ids)
		fmt.Fprintf(sb, "refs: %s\n", strings.Join(ids, ", "))
	}

	revDeps := reverseDeps(e.ID, s)
	if len(revDeps) > 0 {
		sort.Strings(revDeps)
		fmt.Fprintf(sb, "reverse deps: %s\n", strings.Join(revDeps, ", "))
	}

	if e.Type == "ref" || e.Type == "rule" {
		citers, _ := s.CitedBy(e.ID)
		if len(citers) > 0 {
			ids := make([]string, 0, len(citers))
			for _, c := range citers {
				ids = append(ids, c.ID)
			}
			sort.Strings(ids)
			fmt.Fprintf(sb, "citers: %s\n", strings.Join(ids, ", "))
		}
		scopeIDs := scopeOf(e.ID, s)
		if len(scopeIDs) > 0 {
			sort.Strings(scopeIDs)
			fmt.Fprintf(sb, "scope: %s\n", strings.Join(scopeIDs, ", "))
		}
	}

	if globs, ok := codeBindings[e.ID]; ok && len(globs) > 0 {
		sorted := make([]string, len(globs))
		copy(sorted, globs)
		sort.Strings(sorted)
		fmt.Fprintf(sb, "files: %s\n", strings.Join(sorted, ", "))
	}
}

func writeFileMap(sb *strings.Builder, entities []*store.Entity, codeBindings codemap.CodeMap) {
	if len(codeBindings) == 0 {
		return
	}

	entitySet := make(map[string]*store.Entity, len(entities))
	for _, e := range entities {
		entitySet[e.ID] = e
	}

	var patterns []string
	for pat := range codeBindings {
		if strings.HasPrefix(pat, "_") {
			continue
		}
		patterns = append(patterns, pat)
	}
	sort.Strings(patterns)
	if len(patterns) == 0 {
		return
	}

	sb.WriteString("\n## File Map\n")
	for _, id := range patterns {
		if entitySet[id] == nil {
			continue
		}
		globs := codeBindings[id]
		sort.Strings(globs)
		for _, g := range globs {
			fmt.Fprintf(sb, "%s → %s\n", g, id)
		}
	}
}

func reverseDeps(id string, s *store.Store) []string {
	rels, err := s.RelationshipsTo(id)
	if err != nil {
		return nil
	}
	var ids []string
	for _, r := range rels {
		if r.RelType == "uses" || r.RelType == "refs" {
			ids = append(ids, r.FromID)
		}
	}
	return ids
}

func scopeOf(id string, s *store.Store) []string {
	rels, err := s.RelationshipsFrom(id)
	if err != nil {
		return nil
	}
	var ids []string
	for _, r := range rels {
		if r.RelType == "scope" {
			ids = append(ids, r.ToID)
		}
	}
	return ids
}

func contentHash(content string) string {
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", sum)
}
