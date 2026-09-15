package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Scene export constants.
const (
	sceneGunmetal      = 0x3a4150 // body material colour
	sceneMinPlatform   = 0.05     // height of a district platform whose y is 0
	sceneDefaultRoute  = 0x8a93a6 // route colour for kinds without a mapping
	sceneLoaderVersion = 4.6
)

// sceneKindColours maps an edge kind to its LineBasicMaterial colour.
var sceneKindColours = map[string]int{
	"depends_on": 0x4fb3ff,
	"flow_step":  0xffb347,
	"uses":       0x6fd88a,
	"affects":    0xff6fb3,
	"encloses":   0xb38cff,
	"flow_from":  0xffd166,
	"flow_to":    0xffd166,
}

// sceneUUID is sha256(id) formatted 8-4-4-4-12 so re-exports diff cleanly.
func sceneUUID(id string) string {
	sum := sha256.Sum256([]byte(id))
	h := hex.EncodeToString(sum[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// translation is a column-major 4×4 matrix placing an object at (x, y, z).
func translation(x, y, z float64) []float64 {
	return []float64{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, x, y, z, 1}
}

type sceneUserData struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Kind      string `json:"kind,omitempty"`
	Archetype string `json:"archetype,omitempty"`
	District  string `json:"district,omitempty"`
}

type sceneNode struct {
	UUID     string                   `json:"uuid"`
	Type     string                   `json:"type"`
	Name     string                   `json:"name,omitempty"`
	Layers   int                      `json:"layers"`
	Matrix   []float64                `json:"matrix"`
	Geometry string                   `json:"geometry,omitempty"`
	Material string                   `json:"material,omitempty"`
	UserData map[string]sceneUserData `json:"userData,omitempty"`
	Children []sceneNode              `json:"children,omitempty"`
}

type sceneBuilder struct {
	geometries []map[string]any
	materials  []map[string]any
	materialID map[string]string
}

func (b *sceneBuilder) box(id string, w, h, d float64) string {
	uuid := sceneUUID("geometry:" + id)
	b.geometries = append(b.geometries, map[string]any{
		"uuid": uuid, "type": "BoxGeometry",
		"width": w, "height": h, "depth": d,
		"widthSegments": 1, "heightSegments": 1, "depthSegments": 1,
	})
	return uuid
}

func (b *sceneBuilder) line(id string, waypoints [][3]float64) string {
	uuid := sceneUUID("geometry:" + id)
	positions := make([]float64, 0, len(waypoints)*3)
	for _, wp := range waypoints {
		positions = append(positions, wp[0], wp[1], wp[2])
	}
	b.geometries = append(b.geometries, map[string]any{
		"uuid": uuid, "type": "BufferGeometry",
		"data": map[string]any{
			"attributes": map[string]any{
				"position": map[string]any{"itemSize": 3, "type": "Float32Array", "array": positions, "normalized": false},
			},
		},
	})
	return uuid
}

func (b *sceneBuilder) material(name, typ string, colour int) string {
	if uuid, ok := b.materialID[name]; ok {
		return uuid
	}
	uuid := sceneUUID("material:" + name)
	m := map[string]any{"uuid": uuid, "type": typ, "name": name, "color": colour}
	if typ == "MeshStandardMaterial" {
		m["roughness"] = 0.85
		m["metalness"] = 0.35
	}
	b.materials = append(b.materials, m)
	b.materialID[name] = uuid
	return uuid
}

// buildSceneJSON emits the city as three.js ObjectLoader JSON: one Group per
// district (its platform box), one Group per node (a box of the footprint at
// the archetype height), one Line per route. Groups and Lines carry
// userData.c3 = {id, type, kind?, archetype?, district?}; the primitive mesh
// inside a Group carries none, so every c3 id appears exactly once. All uuids
// are sha256-derived from ids, so a re-export of the same payload is
// byte-identical.
func buildSceneJSON(p explorePayload) ([]byte, error) {
	b := &sceneBuilder{materialID: map[string]string{}}
	body := b.material("gunmetal", "MeshStandardMaterial", sceneGunmetal)
	platform := b.material("platform", "MeshStandardMaterial", 0x1c2130)

	scene := sceneNode{UUID: sceneUUID("scene"), Type: "Scene", Name: p.Project, Layers: 1, Matrix: translation(0, 0, 0)}

	for _, d := range p.Districts {
		h := d.Y
		if h <= 0 {
			h = sceneMinPlatform
		}
		group := sceneNode{
			UUID: sceneUUID("district:" + d.ID), Type: "Group", Name: d.Title, Layers: 1,
			Matrix:   translation(d.X, 0, d.Z),
			UserData: map[string]sceneUserData{"c3": {ID: d.ID, Type: "district", Kind: d.Kind}},
			Children: []sceneNode{{
				UUID: sceneUUID("district:" + d.ID + ":mesh"), Type: "Mesh", Name: d.ID + " platform", Layers: 1,
				Matrix:   translation(0, h/2, 0),
				Geometry: b.box("district:"+d.ID, d.W, h, d.D),
				Material: platform,
			}},
		}
		scene.Children = append(scene.Children, group)
	}

	for _, n := range p.Nodes {
		spec, ok := archetypeSpecs[n.Archetype]
		if !ok {
			return nil, fmt.Errorf("node %s: unknown archetype %q\nhint: the payload must pass validateExplorePayload before export; run c3x visualize --schema for the archetype set", n.ID, n.Archetype)
		}
		group := sceneNode{
			UUID: sceneUUID("node:" + n.ID), Type: "Group", Name: n.Title, Layers: 1,
			Matrix:   translation(n.Layout.X, n.Layout.Y, n.Layout.Z),
			UserData: map[string]sceneUserData{"c3": {ID: n.ID, Type: n.Type, Archetype: n.Archetype, District: n.District}},
			Children: []sceneNode{{
				UUID: sceneUUID("node:" + n.ID + ":mesh"), Type: "Mesh", Name: n.ID, Layers: 1,
				Matrix:   translation(0, spec.height/2, 0),
				Geometry: b.box("node:"+n.ID, n.Layout.W, spec.height, n.Layout.D),
				Material: body,
			}},
		}
		scene.Children = append(scene.Children, group)
	}

	for _, r := range p.Routes {
		colour, ok := sceneKindColours[r.Kind]
		if !ok {
			colour = sceneDefaultRoute
		}
		scene.Children = append(scene.Children, sceneNode{
			UUID: sceneUUID("route:" + r.ID), Type: "Line", Name: r.ID, Layers: 1,
			Matrix:   translation(0, 0, 0),
			Geometry: b.line("route:"+r.ID, r.Waypoints),
			Material: b.material("route:"+r.Kind, "LineBasicMaterial", colour),
			UserData: map[string]sceneUserData{"c3": {ID: r.ID, Type: "route", Kind: r.Kind}},
		})
	}

	doc := map[string]any{
		"metadata":   map[string]any{"version": sceneLoaderVersion, "type": "Object", "generator": "c3x visualize"},
		"geometries": b.geometries,
		"materials":  b.materials,
		"object":     scene,
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal scene: %w", err)
	}
	return append(out, '\n'), nil
}
