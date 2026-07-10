package cmd

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/lagz0ne/c3-design/cli/internal/changeset"
	"github.com/lagz0ne/c3-design/cli/internal/content"
	"github.com/lagz0ne/c3-design/cli/internal/markdown"
	"github.com/lagz0ne/c3-design/cli/internal/store"
)

// exploreEvent is one beat of the architecture timeline: an ADR (change-unit)
// with the facts it brought into existence or touched. Ordered by date, the
// events replay the model's history — the event-store view of the architecture.
type exploreEvent struct {
	ID       string   `json:"id"`
	Date     string   `json:"date"` // YYYY-MM-DD; genesis sorts first
	Title    string   `json:"title"`
	Status   string   `json:"status"`
	Creates  []string `json:"creates,omitempty"`  // facts born at this event
	Modifies []string `json:"modifies,omitempty"` // facts touched but born earlier
}

var adrIDDateRE = regexp.MustCompile(`^adr-(\d{4})(\d{2})(\d{2})-`)
var eventDateRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

// eventDate resolves an ADR's timeline date: the entity's date field when it
// parses, else the date embedded in the id, else the genesis sentinel.
func eventDate(adr *store.Entity) string {
	if eventDateRE.MatchString(strings.TrimSpace(adr.Date)) {
		return strings.TrimSpace(adr.Date)
	}
	if m := adrIDDateRE.FindStringSubmatch(adr.ID); m != nil {
		return m[1] + "-" + m[2] + "-" + m[3]
	}
	return "0000-00-00"
}

// affectedTopologyEntities parses the Entity column of an ADR body's Affected
// Topology table.
func affectedTopologyEntities(body string) []string {
	var out []string
	for _, sec := range markdown.ParseSections(body) {
		if sec.Name != "Affected Topology" {
			continue
		}
		for _, line := range strings.Split(sec.Content, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "|") {
				continue
			}
			cells := strings.Split(strings.Trim(line, "|"), "|")
			if len(cells) == 0 {
				continue
			}
			first := strings.TrimSpace(cells[0])
			if first == "" || first == "Entity" || strings.HasPrefix(first, "---") || strings.HasPrefix(first, ":-") {
				continue
			}
			out = append(out, first)
		}
	}
	return out
}

// buildExploreEvents assembles the timeline: one event per ADR, dated, carrying
// the facts it targets — the union of its change-unit patch targets and its
// Affected Topology rows, filtered to facts present in the payload. A fact is
// CREATED by the earliest event that targets it and MODIFIED by later ones;
// facts no event targets fall back to the genesis (earliest) event, so every
// fact is created exactly once and the final replay frame equals the live graph.
func buildExploreEvents(st *store.Store, c3Dir string, factIDs map[string]bool) ([]exploreEvent, error) {
	adrs, err := st.EntitiesByType("adr")
	if err != nil {
		return nil, fmt.Errorf("explore: load adrs: %w", err)
	}

	type rawEvent struct {
		adr     *store.Entity
		date    string
		targets []string
	}
	raws := make([]rawEvent, 0, len(adrs))
	for _, adr := range adrs {
		targetSet := map[string]bool{}

		patches, err := changeset.ReadPatchDir(filepath.Join(c3Dir, "changes", adr.ID))
		if err != nil {
			return nil, fmt.Errorf("explore: read change-unit %s: %w", adr.ID, err)
		}
		for _, p := range patches {
			if p.Target != "" && factIDs[p.Target] {
				targetSet[p.Target] = true
			}
		}

		if body, err := content.ReadEntity(st, adr.ID); err == nil {
			for _, id := range affectedTopologyEntities(body) {
				if factIDs[id] {
					targetSet[id] = true
				}
			}
		}

		targets := make([]string, 0, len(targetSet))
		for id := range targetSet {
			targets = append(targets, id)
		}
		sort.Strings(targets)
		raws = append(raws, rawEvent{adr: adr, date: eventDate(adr), targets: targets})
	}

	if len(raws) == 0 {
		// A model with no ADRs still replays: one synthetic genesis event owns
		// every fact.
		all := make([]string, 0, len(factIDs))
		for id := range factIDs {
			all = append(all, id)
		}
		sort.Strings(all)
		return []exploreEvent{{ID: "genesis", Date: "0000-00-00", Title: "Genesis", Status: "done", Creates: all}}, nil
	}

	sort.Slice(raws, func(i, j int) bool {
		if raws[i].date != raws[j].date {
			return raws[i].date < raws[j].date
		}
		return raws[i].adr.ID < raws[j].adr.ID
	})

	created := map[string]bool{}
	events := make([]exploreEvent, 0, len(raws))
	for _, r := range raws {
		ev := exploreEvent{
			ID:     r.adr.ID,
			Date:   r.date,
			Title:  r.adr.Title,
			Status: normalizeADRStatus(r.adr.Status),
		}
		for _, t := range r.targets {
			if created[t] {
				ev.Modifies = append(ev.Modifies, t)
			} else {
				created[t] = true
				ev.Creates = append(ev.Creates, t)
			}
		}
		events = append(events, ev)
	}

	// Genesis fallback: facts no event targets are born at the earliest event.
	var orphans []string
	for id := range factIDs {
		if !created[id] {
			orphans = append(orphans, id)
		}
	}
	sort.Strings(orphans)
	events[0].Creates = append(events[0].Creates, orphans...)
	sort.Strings(events[0].Creates)

	return events, nil
}
