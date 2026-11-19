package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/lagz0ne/c3-design/cli/internal/content"
	"github.com/lagz0ne/c3-design/cli/internal/markdown"
	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// SetOptions holds parameters for the set command.
type SetOptions struct {
	Store  *store.Store
	C3Dir  string
	ID     string
	Field  string
	Value  string
	Append bool
	Remove bool
}

// RunSet updates a frontmatter field on an entity.
func RunSet(opts SetOptions, w io.Writer) error {
	entity, err := opts.Store.GetEntity(opts.ID)
	if err != nil {
		return fmt.Errorf("error: entity %q not found\nhint: run c3x search %q or c3x list --flat to find the current id", opts.ID, opts.ID)
	}
	return runSetField(entity, opts, w)
}

// ResolveSetArgs normalizes set arguments the same way the command runner does.
func ResolveSetArgs(opts Options) (id, field, value string) {
	if len(opts.Args) >= 1 {
		id = opts.Args[0]
	}
	if len(opts.Args) >= 2 {
		value = opts.Args[1]
	}
	field = opts.Field
	if field == "" && len(opts.Args) >= 2 {
		field = opts.Args[1]
		if len(opts.Args) >= 3 {
			value = opts.Args[2]
		}
	}
	return id, field, value
}

func runSetField(entity *store.Entity, opts SetOptions, w io.Writer) error {
	if entity.Type == "component" {
		if issues := validateCurrentEntityBody(opts.Store, entity); len(issues) > 0 {
			return formatValidationError(opts.ID, issues)
		}
	}

	switch opts.Field {
	case "goal":
		updated, err := setGoal(entity, opts)
		if err != nil {
			return err
		}
		entity = updated
	case "status":
		// The status command is the manual legal-jump path. Status is edit-proof:
		// it is NOT written by UpdateEntity, so it must move through the
		// dedicated SetEntityStatus writer.
		if !statusTransitionLegal(entity.Status, opts.Value) {
			next := legalNextStates(entity.Status)
			if len(next) == 0 {
				return fmt.Errorf("error: cannot transition %s from %q (terminal) to %q\nhint: this status is terminal and has no legal next state", opts.ID, entity.Status, opts.Value)
			}
			return fmt.Errorf("error: cannot transition %s from %q directly to %q\nhint: legal next state(s): %s; run: c3x set %s status %s",
				opts.ID, entity.Status, opts.Value, strings.Join(next, ", "), opts.ID, next[0])
		}
		if err := opts.Store.SetEntityStatus(opts.ID, opts.Value); err != nil {
			return fmt.Errorf("updating status: %w", err)
		}
		fmt.Fprintf(w, "Updated %s field %q\n", opts.ID, opts.Field)
		writeAgentHints(w, cascadeHintsForEntity(entity))
		return nil
	case "boundary":
		entity.Boundary = opts.Value
	case "category":
		entity.Category = opts.Value
	case "title":
		entity.Title = opts.Value
	case "date":
		entity.Date = opts.Value
	default:
		return fmt.Errorf("error: unknown field %q\nhint: set one of goal, status, boundary, category, title, or date", opts.Field)
	}

	if err := opts.Store.UpdateEntity(entity); err != nil {
		return fmt.Errorf("updating entity: %w", err)
	}

	fmt.Fprintf(w, "Updated %s field %q\n", opts.ID, opts.Field)
	writeAgentHints(w, cascadeHintsForEntity(entity))
	return nil
}

// setGoal sets an entity's goal. When the body has a ## Goal section, that
// section owns goal (import and block patches derive it from there), so the new
// goal is written into the section and goal is re-derived from it; writing only
// the goal field would be reverted by the next import. Without a Goal section the
// field is the only copy.
func setGoal(entity *store.Entity, opts SetOptions) (*store.Entity, error) {
	body, err := content.ReadEntity(opts.Store, opts.ID)
	if err != nil || !hasSection(body, "Goal") {
		entity.Goal = opts.Value
		return entity, nil
	}
	newBody, err := markdown.ReplaceSection(body, "Goal", opts.Value)
	if err != nil {
		return nil, fmt.Errorf("error: replacing the Goal section of %s: %w\nhint: edit it with c3x write %s --section Goal", opts.ID, err, opts.ID)
	}
	if err := content.WriteEntity(opts.Store, opts.ID, newBody); err != nil {
		return nil, fmt.Errorf("error: writing the Goal section of %s: %w\nhint: edit it with c3x write %s --section Goal", opts.ID, err, opts.ID)
	}
	return opts.Store.GetEntity(opts.ID)
}

func hasSection(body, name string) bool {
	for _, sec := range markdown.ParseSections(body) {
		if sec.Name == name {
			return true
		}
	}
	return false
}

func validateCurrentEntityBody(s *store.Store, entity *store.Entity) []Issue {
	body, err := content.ReadEntity(s, entity.ID)
	if err != nil {
		return nil
	}
	return validateBodyContent(body, entity.Type)
}
