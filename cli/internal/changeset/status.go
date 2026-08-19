package changeset

import (
	"strings"

	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// PatchState is a patch's lifecycle, derived purely from seal state (hash
// comparison) — never stored, never read from git.
type PatchState string

const (
	StatePending PatchState = "pending" // anchor fresh, not yet applied
	StateApplied PatchState = "applied" // the live block already seals to the patch's result
	StateDrifted PatchState = "drifted" // the anchor moved to something unexpected → rebase
	StateNew     PatchState = "new"     // a create patch whose target does not exist yet
)

// PatchStateOf derives a patch's state by comparing the cited anchor against live
// seal state:
//   - whole-scope, no base (create): target absent → new, present → applied
//   - block anchor: base-hash matches live node → pending; result-hash matches → applied; otherwise → drifted
//   - entity anchor (retire / frontmatter): entity absent + retire scope → applied;
//     entity present with matching merkle → pending; otherwise → drifted
func PatchStateOf(s *store.Store, p Patch) PatchState {
	if p.Base == "" {
		// Only whole-scope patches use no-base as "create". Other scopes require an
		// anchor; treat a missing anchor as drifted so Apply still processes them
		// (CheckDrift on no-base returns nil, so they apply as before).
		if p.Scope != ScopeWhole {
			return StateDrifted
		}
		if _, err := s.GetEntity(p.Target); err == nil {
			return StateApplied
		}
		return StateNew
	}
	if _, nodeID, _, baseHash, ok := ParseCiteHandle(p.Base); ok {
		node, err := s.GetNode(nodeID)
		if err != nil || node.EntityID != p.Target {
			return StateDrifted
		}
		if node.Hash == baseHash {
			return StatePending
		}
		if p.Scope == ScopeBlock && node.Hash == store.ComputeNodeHash(p.Content, node.Type) {
			return StateApplied
		}
		return StateDrifted
	}
	if _, _, merkle, ok := ParseEntityHandle(p.Base); ok {
		e, err := s.GetEntity(p.Target)
		if err != nil {
			if p.Scope == ScopeRetire {
				return StateApplied
			}
			return StateDrifted
		}
		if e.RootMerkle == merkle {
			return StatePending
		}
		return StateDrifted
	}
	return StateDrifted
}

// normalizeHash strips an optional "sha256:" prefix so a declared result-hash and
// a stored node hash compare directly.
func normalizeHash(h string) string {
	return strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(h), "sha256:"))
}
