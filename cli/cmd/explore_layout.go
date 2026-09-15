package cmd

import (
	"container/heap"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
)

// City layout constants (world units; +X east, +Z south).
const (
	cityPad          = 10.0 // district padding around its buildings
	cityRowGap       = 14.0 // added to the tallest footprint to get the row pitch
	cityRowSpread    = 2.5  // centre-to-centre spacing as a multiple of the widest footprint in the row
	cityMinGap       = 40.0 // minimum gap between sectors
	cityHQOffset     = 60.0 // HQ centre north of the sectors' top edge
	cityGovOffset    = 50.0 // governance centre south of the sectors' bottom edge
	cityTrunkOffset  = 12.0 // main trunk / governance trench distance from the sectors
	cityOvershoot    = 3.0  // streets and avenues run this far past their district
	cityStreetWidth  = 3.0
	cityAvenueWidth  = 2.2
	cityTrunkWidth   = 4.2
	cityDockSpacing  = 1.8
	cityDockOffset   = 1.4  // dock distance from the building face
	cityDockRise     = 0.45 // dock height above the platform
	cityRoadY        = 0.16 // waypoint height on the ground
	cityLaneStep     = 0.7
	cityZonePad      = 4.0
	citySectorY      = 0.6
	cityHQY          = 1.1
	cityGovernanceY  = 0.0
	cityFlowNodeWest = 3.0 // a flow signpost sits this far west of its first dock
	cityEmptyZone    = 8.0 // footprint of a boundary with no placed members
)

// archetypeSpec is a building class: footprint, importance and export height.
type archetypeSpec struct {
	w, d, importance, height float64
}

var archetypeSpecs = map[string]archetypeSpec{
	"headquarters": {26, 20, 1.0, 20},
	"command":      {22, 17, 1.0, 19},
	"comms":        {9, 9, 0.8, 18},
	"bunker":       {18, 18, 0.75, 7},
	"plant":        {10, 10, 0.6, 9},
	"transmit":     {14, 10, 0.6, 8},
	"gatehouse":    {7, 7, 0.5, 6},
	"outpost":      {5, 5, 0.3, 3},
	"record":       {3, 3, 0.2, 1.5},
	"zone":         {0, 0, 0, 0.1}, // footprint = members' bbox, filled by the layout
	"route":        {2, 2, 0, 0.5},
}

var (
	bunkerRE   = regexp.MustCompile(`(?i)store|cache|sqlite|database|persist|repositor`)
	transmitRE = regexp.MustCompile(`(?i)explore|render|export|report|visual|emit|output`)
)

// archetypeFor applies the type- and text-driven rows of the archetype table.
// It returns ok=false for a component the sector-dependent rules (comms,
// command, plant) must still classify; layoutCity resolves those.
func archetypeFor(typ, title, goal string) (string, bool) {
	switch typ {
	case "system":
		return "headquarters", true
	case "container":
		return "gatehouse", true
	case "ref", "rule":
		return "outpost", true
	case "adr":
		return "record", true
	case "boundary":
		return "zone", true
	case "flow":
		return "route", true
	case "component":
		// The title names what a component IS; the goal often names what it talks
		// to ("…sourced from the store"), so the title is matched on its own first.
		for _, text := range []string{title, title + " " + goal} {
			if bunkerRE.MatchString(text) {
				return "bunker", true
			}
			if transmitRE.MatchString(text) {
				return "transmit", true
			}
		}
		return "", false
	default:
		return "plant", true
	}
}

/* ─── working state ──────────────────────────────────────────────── */

type cityBox struct{ x0, x1, z0, z1 float64 }

func (b cityBox) union(o cityBox) cityBox {
	return cityBox{math.Min(b.x0, o.x0), math.Max(b.x1, o.x1), math.Min(b.z0, o.z0), math.Max(b.z1, o.z1)}
}

func boxOf(l exploreLayout) cityBox {
	return cityBox{l.X - l.W/2, l.X + l.W/2, l.Z - l.D/2, l.Z + l.D/2}
}

// cityRoad is a horizontal (street/trunk/trench) or vertical (avenue) road.
type cityRoad struct {
	id         string
	horizontal bool
	pos        float64 // z for horizontal, x for vertical
	lo, hi     float64 // span along the road
}

type cityDock struct {
	nodeID string
	edgeID string
	face   string
	x, z   float64
	road   int // index into city.roads
}

type city struct {
	p        *explorePayload
	idx      map[string]int // node id → index in p.Nodes
	district map[string]*exploreDistrict
	roads    []cityRoad
	docks    map[string]*cityDock // edge id + "\x00" + node id → dock
	shared   map[string]string    // flow_step edge id → depends_on edge id it reuses

	sectorIDs                 []string
	hasSectors                bool
	sectorsLeft, sectorsRight float64
	sectorsTop, sectorsBottom float64
	sectorRows                map[string]int
	sectorPitch               map[string]float64
	trunkZ, trenchZ           float64
}

// layoutCity derives every geometric field of the payload — archetypes,
// districts, building positions, roads, docks and routes — as a pure,
// deterministic function of nodes, edges, flows and boundaries. It is safe to
// call repeatedly: every derived field is recomputed from scratch.
func layoutCity(p *explorePayload) {
	c := &city{
		p: p, idx: map[string]int{}, district: map[string]*exploreDistrict{},
		docks: map[string]*cityDock{}, shared: map[string]string{},
		sectorRows: map[string]int{}, sectorPitch: map[string]float64{},
	}
	for i := range p.Nodes {
		c.idx[p.Nodes[i].ID] = i
		p.Nodes[i].Docks = []exploreDock{}
		p.Nodes[i].Layout = exploreLayout{}
		p.Nodes[i].Archetype = ""
		p.Nodes[i].District = ""
		p.Nodes[i].Importance = 0
	}
	p.Districts = []exploreDistrict{}
	p.Roads = exploreRoads{Streets: []exploreStreet{}, Avenues: []exploreAvenue{}}
	p.Routes = []exploreRoute{}

	c.assignDistricts()
	c.placeSectors()
	c.placeHQ()
	c.placeGovernance()
	c.buildRoads()
	c.placeZones()
	c.findSharedRoutes()
	c.dockNodes(func(n exploreNode) bool { return n.Type != "flow" })
	c.placeFlowNodes()
	c.dockNodes(func(n exploreNode) bool { return n.Type == "flow" })
	c.routeEdges()
}

func (c *city) node(id string) *exploreNode {
	i, ok := c.idx[id]
	if !ok {
		return nil
	}
	return &c.p.Nodes[i]
}

/* ─── 1. districts + archetypes ───────────────────────────────────── */

// assignDistricts maps every node to its district and derives its archetype.
// Containers own a sector; the system lives in hq; refs, rules and records in
// governance; a component (or any custom fact) joins the sector of its nearest
// container ancestor, else governance. Zones and routes have no district.
func (c *city) assignDistricts() {
	p := c.p
	for i := range p.Nodes {
		n := &p.Nodes[i]
		arch, _ := archetypeFor(n.Type, n.Title, n.Goal)
		n.Archetype = arch
		switch n.Type {
		case "system":
			n.District = "hq"
		case "container":
			n.District = n.ID
			c.sectorIDs = append(c.sectorIDs, n.ID)
		case "ref", "rule", "adr":
			n.District = "governance"
		case "boundary", "flow":
			n.District = ""
		default:
			n.District = c.nearestContainer(n.ID)
		}
	}
	sort.Strings(c.sectorIDs)

	// Sector-dependent component rules: comms, then exactly one command, then plant.
	byDistrict := map[string][]int{}
	for i := range p.Nodes {
		n := &p.Nodes[i]
		if n.Type == "component" && n.Archetype == "" {
			byDistrict[n.District] = append(byDistrict[n.District], i)
		}
	}
	inSector := func(id, district string) bool {
		n := c.node(id)
		return n != nil && n.District == district
	}
	districts := make([]string, 0, len(byDistrict))
	for d := range byDistrict {
		districts = append(districts, d)
	}
	sort.Strings(districts)
	for _, d := range districts {
		members := byDistrict[d]
		sort.Slice(members, func(i, j int) bool { return p.Nodes[members[i]].ID < p.Nodes[members[j]].ID })
		inDeg, outDeg := map[string]int{}, map[string]int{}
		for _, e := range p.Edges {
			if e.Kind != "depends_on" {
				continue
			}
			outDeg[e.From]++
			if inSector(e.From, d) {
				inDeg[e.To]++
			}
		}
		var remaining []int
		for _, i := range members {
			n := &p.Nodes[i]
			if inDeg[n.ID] == 0 && outDeg[n.ID] >= 1 {
				n.Archetype = "comms"
				continue
			}
			remaining = append(remaining, i)
		}
		if len(remaining) > 0 {
			degree := func(i int) int { return inDeg[p.Nodes[i].ID] + outDeg[p.Nodes[i].ID] }
			command := -1
			for _, i := range remaining {
				if p.Nodes[i].Staged {
					command = i
					break
				}
			}
			if command < 0 {
				command = remaining[0]
				for _, i := range remaining[1:] {
					if degree(i) > degree(command) {
						command = i
					}
				}
			}
			for _, i := range remaining {
				if i == command {
					p.Nodes[i].Archetype = "command"
				} else {
					p.Nodes[i].Archetype = "plant"
				}
			}
		}
	}

	for i := range p.Nodes {
		n := &p.Nodes[i]
		if n.Archetype == "" {
			n.Archetype = "plant"
		}
		sp := archetypeSpecs[n.Archetype]
		n.Importance = sp.importance
		n.Layout.W, n.Layout.D = sp.w, sp.d
	}
}

// nearestContainer walks the parent chain to the closest container node id,
// or "governance" when there is none.
func (c *city) nearestContainer(id string) string {
	seen := map[string]bool{}
	n := c.node(id)
	for n != nil && n.Parent != "" && !seen[n.ID] {
		seen[n.ID] = true
		parent := c.node(n.Parent)
		if parent == nil {
			break
		}
		if parent.Type == "container" {
			return parent.ID
		}
		n = parent
	}
	return "governance"
}

/* ─── 2–3. sectors, hq, governance ────────────────────────────────── */

type sectorPlan struct {
	id      string
	members []string   // buildings (not the gatehouse), by id
	rows    [][]string // row → ids by id
	w, d    float64
	pitch   float64
}

// placeSectors lays every container's sector west→east at z = 0.
func (c *city) placeSectors() {
	p := c.p
	plans := make([]*sectorPlan, 0, len(c.sectorIDs))
	maxFootprintW := 0.0
	for _, sid := range c.sectorIDs {
		plan := &sectorPlan{id: sid}
		for i := range p.Nodes {
			n := &p.Nodes[i]
			if n.District == sid && n.ID != sid {
				plan.members = append(plan.members, n.ID)
			}
			if n.District == sid {
				maxFootprintW = math.Max(maxFootprintW, n.Layout.W)
			}
		}
		sort.Strings(plan.members)
		plan.rows = c.layerSector(plan.members)

		maxD := 0.0
		for _, id := range plan.members {
			maxD = math.Max(maxD, c.node(id).Layout.D)
		}
		plan.pitch = maxD + cityRowGap
		rowWidth := 0.0
		for _, row := range plan.rows {
			rowWidth = math.Max(rowWidth, c.rowWidth(row))
		}
		plan.w = rowWidth + 2*cityPad
		plan.d = float64(len(plan.rows))*plan.pitch + 2*cityPad
		plans = append(plans, plan)
	}
	if len(plans) == 0 {
		return
	}

	gap := math.Max(cityMinGap, 3*maxFootprintW)
	total := gap * float64(len(plans)-1)
	for _, plan := range plans {
		total += plan.w
	}
	left := -total / 2
	c.hasSectors = true
	c.sectorsLeft, c.sectorsRight = math.Inf(1), math.Inf(-1)
	c.sectorsTop, c.sectorsBottom = math.Inf(1), math.Inf(-1)
	for i, plan := range plans {
		if i > 0 {
			left += gap
		}
		cx := left + plan.w/2
		cz := 0.0
		d := exploreDistrict{
			ID: plan.id, Title: fmt.Sprintf("SECTOR %02d · %s", i+1, strings.ToUpper(c.node(plan.id).Title)),
			Kind: "sector", X: cx, Z: cz, W: plan.w, D: plan.d, Y: citySectorY, Members: []string{},
		}
		top := cz - plan.d/2
		gate := c.node(plan.id)
		gate.Layout.X, gate.Layout.Z, gate.Layout.Y = left+cityPad/2, top+cityPad/2, citySectorY
		for r, row := range plan.rows {
			maxW := c.rowMaxW(row)
			spacing := maxW * cityRowSpread
			width := c.rowWidth(row)
			z := top + cityPad + float64(r)*plan.pitch + plan.pitch/2
			for k, id := range row {
				n := c.node(id)
				n.Layout.X = cx - width/2 + maxW/2 + float64(k)*spacing
				n.Layout.Z = z
				n.Layout.Y = citySectorY
			}
		}
		c.sectorRows[plan.id] = len(plan.rows)
		c.sectorPitch[plan.id] = plan.pitch
		c.addDistrict(d)
		left += plan.w
		c.sectorsLeft = math.Min(c.sectorsLeft, cx-plan.w/2)
		c.sectorsRight = math.Max(c.sectorsRight, cx+plan.w/2)
		c.sectorsTop = math.Min(c.sectorsTop, top)
		c.sectorsBottom = math.Max(c.sectorsBottom, cz+plan.d/2)
	}
}

func (c *city) rowMaxW(row []string) float64 {
	maxW := 0.0
	for _, id := range row {
		maxW = math.Max(maxW, c.node(id).Layout.W)
	}
	return maxW
}

func (c *city) rowWidth(row []string) float64 {
	if len(row) == 0 {
		return 0
	}
	maxW := c.rowMaxW(row)
	return float64(len(row)-1)*maxW*cityRowSpread + maxW
}

// layerSector is longest-path layering over depends_on edges inside the sector
// (Kahn). When only cyclic nodes remain, the smallest id is released and its
// unprocessed incoming edges are ignored; ready nodes are always taken by id.
func (c *city) layerSector(members []string) [][]string {
	if len(members) == 0 {
		return nil
	}
	member := map[string]bool{}
	for _, id := range members {
		member[id] = true
	}
	out := map[string][]string{}
	inDeg := map[string]int{}
	for _, e := range c.p.Edges {
		if e.Kind != "depends_on" || !member[e.From] || !member[e.To] || e.From == e.To {
			continue
		}
		out[e.From] = append(out[e.From], e.To)
		inDeg[e.To]++
	}
	for id := range out {
		sort.Strings(out[id])
	}

	layer := map[string]int{}
	remaining := map[string]bool{}
	for _, id := range members {
		remaining[id] = true
	}
	var ready []string
	for _, id := range members {
		if inDeg[id] == 0 {
			ready = append(ready, id)
		}
	}
	for len(remaining) > 0 {
		if len(ready) == 0 {
			// Cycle: release the smallest remaining id, ignoring its back edges.
			rest := make([]string, 0, len(remaining))
			for id := range remaining {
				rest = append(rest, id)
			}
			sort.Strings(rest)
			ready = append(ready, rest[0])
		}
		sort.Strings(ready)
		v := ready[0]
		ready = ready[1:]
		if !remaining[v] {
			continue
		}
		delete(remaining, v)
		for _, w := range out[v] {
			if !remaining[w] {
				continue
			}
			if layer[v]+1 > layer[w] {
				layer[w] = layer[v] + 1
			}
			inDeg[w]--
			if inDeg[w] == 0 {
				ready = append(ready, w)
			}
		}
	}

	rows := 0
	for _, id := range members {
		if layer[id]+1 > rows {
			rows = layer[id] + 1
		}
	}
	result := make([][]string, rows)
	for _, id := range members {
		result[layer[id]] = append(result[layer[id]], id)
	}
	for _, row := range result {
		sort.Strings(row)
	}
	return result
}

// placeHQ centres the system building(s) north of the sectors.
func (c *city) placeHQ() {
	c.placeRow("hq", "HQ · "+strings.ToUpper(c.p.Project), "hq", cityHQY, c.sectorsMidX(), c.sectorsTopZ()-cityHQOffset)
}

// placeGovernance lays outposts and records in one east–west row south of the sectors.
func (c *city) placeGovernance() {
	c.placeRow("governance", "PERIMETER · GOVERNANCE", "governance", cityGovernanceY, c.sectorsMidX(), c.sectorsBottomZ()+cityGovOffset)
}

func (c *city) sectorsMidX() float64 {
	if !c.hasSectors {
		return 0
	}
	return (c.sectorsLeft + c.sectorsRight) / 2
}

func (c *city) sectorsTopZ() float64 {
	if !c.hasSectors {
		return 0
	}
	return c.sectorsTop
}

func (c *city) sectorsBottomZ() float64 {
	if !c.hasSectors {
		return 0
	}
	return c.sectorsBottom
}

// placeRow lays a district whose members form a single row spaced 3W apart,
// centred on (cx, cz). The district is created only when it has members.
func (c *city) placeRow(id, title, kind string, y, cx, cz float64) {
	p := c.p
	var members []string
	for i := range p.Nodes {
		if p.Nodes[i].District == id {
			members = append(members, p.Nodes[i].ID)
		}
	}
	if len(members) == 0 {
		return
	}
	sort.Strings(members)
	maxW, maxD := 0.0, 0.0
	for _, m := range members {
		maxW = math.Max(maxW, c.node(m).Layout.W)
		maxD = math.Max(maxD, c.node(m).Layout.D)
	}
	spacing := 3 * maxW
	width := float64(len(members)-1)*spacing + maxW
	d := exploreDistrict{ID: id, Title: title, Kind: kind, X: cx, Z: cz, W: width + 2*cityPad, D: maxD + 2*cityPad, Y: y, Members: []string{}}
	for k, m := range members {
		n := c.node(m)
		n.Layout.X = cx - width/2 + maxW/2 + float64(k)*spacing
		n.Layout.Z = cz
		n.Layout.Y = y
	}
	c.addDistrict(d)
}

func (c *city) addDistrict(d exploreDistrict) {
	for i := range c.p.Nodes {
		if c.p.Nodes[i].District == d.ID {
			d.Members = append(d.Members, c.p.Nodes[i].ID)
		}
	}
	sort.Strings(d.Members)
	c.p.Districts = append(c.p.Districts, d)
	c.district[d.ID] = &c.p.Districts[len(c.p.Districts)-1]
	// Re-point every pointer: append may have reallocated.
	for i := range c.p.Districts {
		c.district[c.p.Districts[i].ID] = &c.p.Districts[i]
	}
}

/* ─── 4. roads ────────────────────────────────────────────────────── */

func (c *city) addStreet(s exploreStreet) {
	c.p.Roads.Streets = append(c.p.Roads.Streets, s)
	c.roads = append(c.roads, cityRoad{id: s.ID, horizontal: true, pos: s.Z, lo: s.X0, hi: s.X1})
}

func (c *city) addAvenue(a exploreAvenue) {
	c.p.Roads.Avenues = append(c.p.Roads.Avenues, a)
	c.roads = append(c.roads, cityRoad{id: a.ID, horizontal: false, pos: a.X, lo: a.Z0, hi: a.Z1})
}

// buildRoads emits per-sector streets and avenues, the main trunk north of the
// sectors, the governance trench south of them, and the HQ's own avenues.
func (c *city) buildRoads() {
	if len(c.p.Districts) == 0 {
		return
	}
	all := cityBox{math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)}
	for _, d := range c.p.Districts {
		all = all.union(cityBox{d.X - d.W/2, d.X + d.W/2, d.Z - d.D/2, d.Z + d.D/2})
	}

	c.trunkZ = c.sectorsTopZ() - cityTrunkOffset
	c.trenchZ = c.sectorsBottomZ() + cityTrunkOffset

	for _, sid := range c.sectorIDs {
		d := c.district[sid]
		left, right := d.X-d.W/2, d.X+d.W/2
		top := d.Z - d.D/2
		rows := c.sectorRows[sid]
		pitch := c.sectorPitch[sid]
		for k := 0; k <= rows; k++ {
			c.addStreet(exploreStreet{
				ID: fmt.Sprintf("%s:street:%d", sid, k), District: sid,
				Z: top + cityPad + float64(k)*pitch, X0: left - cityOvershoot, X1: right + cityOvershoot,
				Width: cityStreetWidth,
			})
		}
		c.addAvenue(exploreAvenue{ID: sid + ":avenue:w", District: sid, X: left - cityOvershoot, Z0: c.trunkZ, Z1: c.trenchZ, Width: cityAvenueWidth})
		c.addAvenue(exploreAvenue{ID: sid + ":avenue:e", District: sid, X: right + cityOvershoot, Z0: c.trunkZ, Z1: c.trenchZ, Width: cityAvenueWidth})
	}

	c.addStreet(exploreStreet{ID: "main-trunk", Z: c.trunkZ, X0: all.x0 - cityOvershoot, X1: all.x1 + cityOvershoot, Width: cityTrunkWidth, Major: true})
	c.addStreet(exploreStreet{ID: "governance-trench", Z: c.trenchZ, X0: all.x0 - cityOvershoot, X1: all.x1 + cityOvershoot, Width: cityTrunkWidth, Major: true})

	if hq, ok := c.district["hq"]; ok {
		// Without sectors the trunk and trench are otherwise disconnected, so the
		// HQ avenues run through to the trench.
		z1 := c.trunkZ
		if !c.hasSectors {
			z1 = c.trenchZ
		}
		z0 := hq.Z - hq.D/2
		c.addAvenue(exploreAvenue{ID: "hq:avenue:w", District: "hq", X: hq.X - hq.W/2 - cityOvershoot, Z0: z0, Z1: z1, Width: cityAvenueWidth})
		c.addAvenue(exploreAvenue{ID: "hq:avenue:e", District: "hq", X: hq.X + hq.W/2 + cityOvershoot, Z0: z0, Z1: z1, Width: cityAvenueWidth})
	}
}

/* ─── 7a. zones ───────────────────────────────────────────────────── */

// placeZones sizes each boundary node to the bounding box of its members'
// footprints (a container counts as its whole sector) padded by 4. Children
// are placed before parents and a parent's box also covers its children's, so
// nested boundaries nest even when the member lists are not strict subsets.
// A boundary with no placed member gets an 8×8 marker at the origin.
func (c *city) placeZones() {
	byID := map[string]exploreBoundary{}
	for _, b := range c.p.Boundaries {
		byID[b.ID] = b
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
		d := 0
		if b, ok := byID[id]; ok {
			if _, parentIsBoundary := byID[b.Parent]; parentIsBoundary {
				d = depthOf(b.Parent, guard) + 1
			}
		}
		depth[id] = d
		return d
	}
	var zones []string
	for i := range c.p.Nodes {
		if c.p.Nodes[i].Type == "boundary" {
			zones = append(zones, c.p.Nodes[i].ID)
			depthOf(c.p.Nodes[i].ID, map[string]bool{})
		}
	}
	sort.Slice(zones, func(i, j int) bool {
		if depth[zones[i]] != depth[zones[j]] {
			return depth[zones[i]] > depth[zones[j]]
		}
		return zones[i] < zones[j]
	})
	placed := map[string]bool{}
	for _, zid := range zones {
		box := cityBox{math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)}
		y := 0.0
		any := false
		add := func(b cityBox, py float64) {
			box = box.union(b)
			y = math.Max(y, py)
			any = true
		}
		b := byID[zid]
		for _, m := range b.Members {
			n := c.node(m)
			if n == nil || n.Type == "boundary" || n.Type == "flow" {
				continue
			}
			if n.Type == "container" {
				if d, ok := c.district[n.ID]; ok {
					add(cityBox{d.X - d.W/2, d.X + d.W/2, d.Z - d.D/2, d.Z + d.D/2}, d.Y)
					continue
				}
			}
			if n.Layout.W > 0 {
				add(boxOf(n.Layout), n.Layout.Y)
			}
		}
		for _, other := range zones {
			if ob, ok := byID[other]; ok && ob.Parent == zid && placed[other] {
				add(boxOf(c.node(other).Layout), c.node(other).Layout.Y)
			}
		}
		n := c.node(zid)
		if !any {
			n.Layout = exploreLayout{X: 0, Y: 0, Z: 0, W: cityEmptyZone, D: cityEmptyZone}
		} else {
			n.Layout = exploreLayout{
				X: (box.x0 + box.x1) / 2, Z: (box.z0 + box.z1) / 2, Y: y,
				W: box.x1 - box.x0 + 2*cityZonePad, D: box.z1 - box.z0 + 2*cityZonePad,
			}
		}
		placed[zid] = true
	}
}

/* ─── 5. docks ────────────────────────────────────────────────────── */

// findSharedRoutes records every flow_step edge whose endpoints also carry a
// depends_on edge; those reuse the depends_on route and get no docks of their own.
func (c *city) findSharedRoutes() {
	dep := map[string]string{}
	for _, e := range c.p.Edges {
		if e.Kind == "depends_on" {
			dep[e.From+"\x00"+e.To] = e.ID
		}
	}
	for _, e := range c.p.Edges {
		if e.Kind != "flow_step" {
			continue
		}
		if id, ok := dep[e.From+"\x00"+e.To]; ok {
			c.shared[e.ID] = id
		}
	}
}

// ownRouteEdges returns the edges that get a route of their own (routed kinds,
// non-shared) in payload order.
func (c *city) ownRouteEdges() []exploreEdge {
	var out []exploreEdge
	for _, e := range c.p.Edges {
		if !edgeIsRouted(e.Kind) {
			continue
		}
		if _, shared := c.shared[e.ID]; shared {
			continue
		}
		if c.node(e.From) == nil || c.node(e.To) == nil {
			continue
		}
		out = append(out, e)
	}
	return out
}

// dockNodes computes the docks of every node the predicate admits. Each of the
// node's own-route edges gets a dock on the face toward the other endpoint
// (south when the other endpoint is not placed yet or level), projected onto
// the nearest horizontal road beyond that face; docks on a face are spread 1.8
// apart, centred, ordered by edge id.
func (c *city) dockNodes(admit func(exploreNode) bool) {
	type want struct {
		edgeID string
		face   string
		road   int
	}
	perNode := map[string][]want{}
	placed := func(n *exploreNode) bool { return n.Layout.W > 0 }
	for _, e := range c.ownRouteEdges() {
		for _, end := range []struct{ self, other string }{{e.From, e.To}, {e.To, e.From}} {
			self := c.node(end.self)
			if !admit(*self) || !placed(self) {
				continue
			}
			other := c.node(end.other)
			south := true
			if placed(other) && other.Layout.Z < self.Layout.Z {
				south = false
			}
			face, road := c.chooseFace(self, south)
			perNode[self.ID] = append(perNode[self.ID], want{edgeID: e.ID, face: face, road: road})
		}
	}
	ids := make([]string, 0, len(perNode))
	for id := range perNode {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		n := c.node(id)
		for _, face := range []string{"n", "s"} {
			var wants []want
			for _, w := range perNode[id] {
				if w.face == face {
					wants = append(wants, w)
				}
			}
			sort.Slice(wants, func(i, j int) bool { return wants[i].edgeID < wants[j].edgeID })
			for k, w := range wants {
				x := n.Layout.X + (float64(k)-float64(len(wants)-1)/2)*cityDockSpacing
				z := n.Layout.Z + n.Layout.D/2 + cityDockOffset
				if face == "n" {
					z = n.Layout.Z - n.Layout.D/2 - cityDockOffset
				}
				dock := exploreDock{ID: fmt.Sprintf("%s:%s:%d", id, face, k), Face: face, X: x, Z: z, EdgeID: w.edgeID}
				n.Docks = append(n.Docks, dock)
				c.docks[w.edgeID+"\x00"+id] = &cityDock{nodeID: id, edgeID: w.edgeID, face: face, x: x, z: z, road: w.road}
			}
		}
	}
}

// chooseFace picks the nearest horizontal road beyond the requested face that
// covers the node's x, falling back to the other face, then to the nearest
// horizontal road regardless of coverage. Returns the face and road index (-1
// when the city has no roads).
func (c *city) chooseFace(n *exploreNode, south bool) (string, int) {
	pick := func(south bool) int {
		best, bestDist := -1, math.Inf(1)
		for i, r := range c.roads {
			if !r.horizontal || n.Layout.X < r.lo || n.Layout.X > r.hi {
				continue
			}
			var dist float64
			if south {
				dist = r.pos - (n.Layout.Z + n.Layout.D/2)
			} else {
				dist = (n.Layout.Z - n.Layout.D/2) - r.pos
			}
			if dist < 0 {
				continue
			}
			if dist < bestDist || (dist == bestDist && c.roads[i].id < c.roads[best].id) {
				best, bestDist = i, dist
			}
		}
		return best
	}
	if i := pick(south); i >= 0 {
		return faceName(south), i
	}
	if i := pick(!south); i >= 0 {
		return faceName(!south), i
	}
	best, bestDist := -1, math.Inf(1)
	for i, r := range c.roads {
		if !r.horizontal {
			continue
		}
		if dist := math.Abs(r.pos - n.Layout.Z); dist < bestDist {
			best, bestDist = i, dist
		}
	}
	if best >= 0 {
		return faceName(c.roads[best].pos > n.Layout.Z), best
	}
	return faceName(south), -1
}

func faceName(south bool) string {
	if south {
		return "s"
	}
	return "n"
}

/* ─── 7b. flow nodes ──────────────────────────────────────────────── */

// placeFlowNodes puts each flow signpost 3 units west of its first step's
// source dock (the flow_step edge, or the depends_on route it shares). A flow
// with no placed step sits at the origin.
func (c *city) placeFlowNodes() {
	flows := map[string]exploreFlow{}
	for _, f := range c.p.Flows {
		flows[f.ID] = f
	}
	for i := range c.p.Nodes {
		n := &c.p.Nodes[i]
		if n.Type != "flow" {
			continue
		}
		sp := archetypeSpecs["route"]
		n.Layout = exploreLayout{W: sp.w, D: sp.d}
		f, ok := flows[n.ID]
		if !ok || len(f.Steps) == 0 {
			continue
		}
		first := f.Steps[0]
		for _, st := range f.Steps[1:] {
			if st.Seq < first.Seq {
				first = st
			}
		}
		edgeID := flowStepEdgeID(first.From, first.To, n.ID, first.Seq)
		if shared, ok := c.shared[edgeID]; ok {
			edgeID = shared
		}
		dock, ok := c.docks[edgeID+"\x00"+first.From]
		if !ok {
			continue
		}
		src := c.node(first.From)
		n.Layout.X, n.Layout.Z, n.Layout.Y = dock.x-cityFlowNodeWest, dock.z, src.Layout.Y
	}
}

/* ─── 6. routes ───────────────────────────────────────────────────── */

type routeGraph struct {
	index map[[2]float64]int
	pts   [][2]float64
	adj   [][]routeArc
}

type routeArc struct {
	to   int
	w    float64
	road string
}

func (g *routeGraph) vertex(x, z float64) int {
	key := [2]float64{x, z}
	if i, ok := g.index[key]; ok {
		return i
	}
	g.index[key] = len(g.pts)
	g.pts = append(g.pts, key)
	g.adj = append(g.adj, nil)
	return len(g.pts) - 1
}

func (g *routeGraph) connect(a, b int, road string) {
	w := math.Abs(g.pts[a][0]-g.pts[b][0]) + math.Abs(g.pts[a][1]-g.pts[b][1])
	g.adj[a] = append(g.adj[a], routeArc{to: b, w: w, road: road})
	g.adj[b] = append(g.adj[b], routeArc{to: a, w: w, road: road})
}

type pqItem struct {
	v    int
	dist float64
}
type pq []pqItem

func (q pq) Len() int { return len(q) }
func (q pq) Less(i, j int) bool {
	if q[i].dist != q[j].dist {
		return q[i].dist < q[j].dist
	}
	return q[i].v < q[j].v
}
func (q pq) Swap(i, j int)       { q[i], q[j] = q[j], q[i] }
func (q *pq) Push(x any)         { *q = append(*q, x.(pqItem)) }
func (q *pq) Pop() (x any)       { old := *q; x = old[len(old)-1]; *q = old[:len(old)-1]; return x }
func (g *routeGraph) count() int { return len(g.pts) }

// shortest runs Dijkstra and returns the vertex path and the road of each hop.
func (g *routeGraph) shortest(src, dst int) ([]int, []string, bool) {
	n := g.count()
	dist := make([]float64, n)
	prev := make([]int, n)
	prevRoad := make([]string, n)
	for i := range dist {
		dist[i] = math.Inf(1)
		prev[i] = -1
	}
	dist[src] = 0
	q := &pq{{v: src}}
	done := make([]bool, n)
	for q.Len() > 0 {
		it := heap.Pop(q).(pqItem)
		if done[it.v] {
			continue
		}
		done[it.v] = true
		if it.v == dst {
			break
		}
		arcs := append([]routeArc(nil), g.adj[it.v]...)
		sort.Slice(arcs, func(i, j int) bool {
			if arcs[i].to != arcs[j].to {
				return arcs[i].to < arcs[j].to
			}
			return arcs[i].road < arcs[j].road
		})
		for _, a := range arcs {
			nd := it.dist + a.w
			if nd < dist[a.to] || (nd == dist[a.to] && prev[a.to] > it.v) {
				dist[a.to] = nd
				prev[a.to] = it.v
				prevRoad[a.to] = a.road
				heap.Push(q, pqItem{v: a.to, dist: nd})
			}
		}
	}
	if math.IsInf(dist[dst], 1) {
		return nil, nil, false
	}
	var path []int
	var roads []string
	for v := dst; v != -1; v = prev[v] {
		path = append(path, v)
		if prev[v] != -1 {
			roads = append(roads, prevRoad[v])
		}
	}
	for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
		path[i], path[j] = path[j], path[i]
	}
	for i, j := 0, len(roads)-1; i < j; i, j = i+1, j-1 {
		roads[i], roads[j] = roads[j], roads[i]
	}
	return path, roads, true
}

// routeEdges builds the road graph (street×avenue and avenue×trunk/trench
// intersections plus every dock's projection), runs Dijkstra per own-route
// edge, assigns lanes by greedy colouring in route-id order, and writes one
// route per routed edge (shared flow steps copy their depends_on route).
func (c *city) routeEdges() {
	g := &routeGraph{index: map[[2]float64]int{}}
	for _, h := range c.roads {
		if !h.horizontal {
			continue
		}
		for _, v := range c.roads {
			if v.horizontal {
				continue
			}
			if v.pos >= h.lo && v.pos <= h.hi && h.pos >= v.lo && h.pos <= v.hi {
				g.vertex(v.pos, h.pos)
			}
		}
	}
	proj := map[string]int{} // dock key → projection vertex
	dockKeys := make([]string, 0, len(c.docks))
	for k := range c.docks {
		dockKeys = append(dockKeys, k)
	}
	sort.Strings(dockKeys)
	for _, k := range dockKeys {
		d := c.docks[k]
		if d.road < 0 {
			continue
		}
		proj[k] = g.vertex(d.x, c.roads[d.road].pos)
	}
	for _, r := range c.roads {
		var on []int
		for i, pt := range g.pts {
			if r.horizontal && pt[1] == r.pos && pt[0] >= r.lo && pt[0] <= r.hi {
				on = append(on, i)
			}
			if !r.horizontal && pt[0] == r.pos && pt[1] >= r.lo && pt[1] <= r.hi {
				on = append(on, i)
			}
		}
		sort.Slice(on, func(i, j int) bool {
			if r.horizontal {
				return g.pts[on[i]][0] < g.pts[on[j]][0]
			}
			return g.pts[on[i]][1] < g.pts[on[j]][1]
		})
		for i := 1; i < len(on); i++ {
			g.connect(on[i-1], on[i], r.id)
		}
	}

	steps := map[string]bool{}
	for _, f := range c.p.Flows {
		for _, st := range f.Steps {
			steps[st.From+"\x00"+st.To] = true
		}
	}

	own := map[string]*exploreRoute{}
	arcs := map[string]map[[2]int]bool{} // route id → graph arcs travelled
	var ownOrder []string
	for _, e := range c.ownRouteEdges() {
		r := exploreRoute{ID: e.ID, EdgeID: e.ID, Kind: e.Kind, Segments: []string{}, Waypoints: [][3]float64{}}
		src := c.docks[e.ID+"\x00"+e.From]
		dst := c.docks[e.ID+"\x00"+e.To]
		from, to := c.node(e.From), c.node(e.To)
		if src == nil || dst == nil {
			// An endpoint without a dock (no roads at all): a straight line.
			r.Waypoints = [][3]float64{{from.Layout.X, from.Layout.Y + cityDockRise, from.Layout.Z}, {to.Layout.X, to.Layout.Y + cityDockRise, to.Layout.Z}}
		} else {
			r.Waypoints, r.Segments, arcs[e.ID] = c.routeBetween(g, proj, src, dst, from.Layout.Y, to.Layout.Y)
		}
		r.Active = e.Kind == "flow_step" || (e.Kind == "depends_on" && steps[e.From+"\x00"+e.To]) || from.Staged
		own[e.ID] = &r
		ownOrder = append(ownOrder, e.ID)
	}

	// Lanes: greedy colouring in route-id order. Two routes conflict when they
	// travel the same segment — the stretch of road between two graph vertices —
	// not merely the same road, so a trunk carrying many short hops stays narrow.
	sorted := append([]string(nil), ownOrder...)
	sort.Strings(sorted)
	laneIdx := map[string]int{}
	maxIdx := 0
	for i, id := range sorted {
		used := map[int]bool{}
		for _, prior := range sorted[:i] {
			if sharesArc(arcs[id], arcs[prior]) {
				used[laneIdx[prior]] = true
			}
		}
		idx := 0
		for used[idx] {
			idx++
		}
		laneIdx[id] = idx
		if idx > maxIdx {
			maxIdx = idx
		}
	}
	// Lane pitch shrinks as the colour count grows so a hub trench reads as one
	// bundled cable trunk that stays inside its shoulders, instead of fanning
	// across neighbouring buildings; below cityLaneStep the lanes simply overlap.
	lanes := float64(maxIdx + 1)
	step := cityLaneStep
	if lanes > 1 {
		step = math.Min(cityLaneStep, (cityTrunkWidth-0.8)/(lanes-1))
	}
	for _, id := range ownOrder {
		r := own[id]
		r.Lane = (float64(laneIdx[id]) - (lanes-1)/2) * step
		applyLane(r)
	}

	for _, e := range c.p.Edges {
		if !edgeIsRouted(e.Kind) {
			continue
		}
		if shared, ok := c.shared[e.ID]; ok {
			base, ok := own[shared]
			if !ok {
				continue
			}
			c.p.Routes = append(c.p.Routes, exploreRoute{
				ID: e.ID, EdgeID: e.ID, Kind: e.Kind, Active: true,
				Segments: append([]string{}, base.Segments...), Lane: base.Lane, SharedWith: base.ID,
				Waypoints: append([][3]float64{}, base.Waypoints...),
			})
			continue
		}
		if r, ok := own[e.ID]; ok {
			c.p.Routes = append(c.p.Routes, *r)
		}
	}
}

// routeBetween returns the waypoints [dock, projection, …, projection, dock]
// with collinear interior vertices removed, the road ids travelled, and the
// graph arcs (vertex pairs) used for lane conflicts. When the projections
// coincide the touched road is still reported as the segment; when no path
// exists the two projections are joined directly.
func (c *city) routeBetween(g *routeGraph, proj map[string]int, src, dst *cityDock, ySrc, yDst float64) ([][3]float64, []string, map[[2]int]bool) {
	a, b := proj[src.edgeID+"\x00"+src.nodeID], proj[dst.edgeID+"\x00"+dst.nodeID]
	path, roads, ok := g.shortest(a, b)
	if !ok {
		path = []int{a, b}
		roads = nil
	}
	used := map[[2]int]bool{}
	for i := 1; i < len(path) && ok; i++ {
		u, v := path[i-1], path[i]
		if u > v {
			u, v = v, u
		}
		used[[2]int{u, v}] = true
	}
	var pts [][2]float64
	for _, v := range path {
		pts = append(pts, g.pts[v])
	}
	// Drop interior vertices collinear with both neighbours.
	kept := []int{0}
	for i := 1; i < len(pts)-1; i++ {
		p, q, r := pts[i-1], pts[i], pts[i+1]
		if (p[0] == q[0] && q[0] == r[0]) || (p[1] == q[1] && q[1] == r[1]) {
			continue
		}
		kept = append(kept, i)
	}
	if len(pts) > 1 {
		kept = append(kept, len(pts)-1)
	}
	wps := [][3]float64{{src.x, ySrc + cityDockRise, src.z}}
	for _, i := range kept {
		wps = append(wps, [3]float64{pts[i][0], cityRoadY, pts[i][1]})
	}
	wps = append(wps, [3]float64{dst.x, yDst + cityDockRise, dst.z})

	segments := []string{}
	for _, r := range roads {
		if len(segments) == 0 || segments[len(segments)-1] != r {
			segments = append(segments, r)
		}
	}
	if len(segments) == 0 && src.road >= 0 && ok {
		segments = append(segments, c.roads[src.road].id)
	}
	return wps, segments, used
}

func sharesArc(a, b map[[2]int]bool) bool {
	for arc := range a {
		if b[arc] {
			return true
		}
	}
	return false
}

// applyLane offsets every interior waypoint perpendicular to the road segments
// meeting there: z on horizontal segments, x on vertical ones. The dock drops
// (first and last hop) are not road segments and add no offset.
func applyLane(r *exploreRoute) {
	if r.Lane == 0 || len(r.Waypoints) < 3 {
		return
	}
	orig := append([][3]float64{}, r.Waypoints...)
	for i := 1; i < len(orig)-1; i++ {
		cur := orig[i]
		dx, dz := false, false
		if i > 1 {
			prev := orig[i-1]
			if prev[2] == cur[2] && prev[0] != cur[0] {
				dz = true
			}
			if prev[0] == cur[0] && prev[2] != cur[2] {
				dx = true
			}
		}
		if i < len(orig)-2 {
			next := orig[i+1]
			if next[2] == cur[2] && next[0] != cur[0] {
				dz = true
			}
			if next[0] == cur[0] && next[2] != cur[2] {
				dx = true
			}
		}
		if dz {
			r.Waypoints[i][2] += r.Lane
		}
		if dx {
			r.Waypoints[i][0] += r.Lane
		}
	}
}
