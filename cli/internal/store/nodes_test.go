package store

import (
	"database/sql"
	"testing"
)

func TestInsertNode_AndGet(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)

	n := &Node{
		EntityID: "auth-handler",
		Type:     "heading",
		Level:    2,
		Seq:      0,
		Content:  "Goal",
		Hash:     ComputeNodeHash("Goal", "heading"),
	}
	id, err := s.InsertNode(n)
	if err != nil {
		t.Fatalf("insert node: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	got, err := s.GetNode(id)
	if err != nil {
		t.Fatalf("get node: %v", err)
	}
	if got.EntityID != "auth-handler" || got.Type != "heading" || got.Content != "Goal" {
		t.Errorf("unexpected node: %+v", got)
	}
	if got.Hash == "" {
		t.Error("expected non-empty hash")
	}
}

func TestInsertNode_WithParent(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)

	parent := &Node{EntityID: "auth-handler", Type: "heading", Level: 2, Seq: 0, Content: "Dependencies", Hash: "abc"}
	parentID, err := s.InsertNode(parent)
	if err != nil {
		t.Fatalf("insert parent: %v", err)
	}

	child := &Node{
		EntityID: "auth-handler",
		ParentID: sql.NullInt64{Int64: parentID, Valid: true},
		Type:     "table_row",
		Seq:      0,
		Content:  "IN|auth-svc|c3-102",
		Hash:     "def",
	}
	childID, err := s.InsertNode(child)
	if err != nil {
		t.Fatalf("insert child: %v", err)
	}

	got, err := s.GetNode(childID)
	if err != nil {
		t.Fatalf("get child: %v", err)
	}
	if !got.ParentID.Valid || got.ParentID.Int64 != parentID {
		t.Errorf("parent_id = %v, want %d", got.ParentID, parentID)
	}
}

func TestNodesForEntity(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)

	nodes := []*Node{
		{EntityID: "auth-handler", Type: "heading", Level: 2, Seq: 0, Content: "Goal", Hash: "a"},
		{EntityID: "auth-handler", Type: "paragraph", Seq: 1, Content: "Authenticate requests", Hash: "b"},
		{EntityID: "auth-handler", Type: "heading", Level: 2, Seq: 2, Content: "Dependencies", Hash: "c"},
	}
	for _, n := range nodes {
		if _, err := s.InsertNode(n); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	got, err := s.NodesForEntity("auth-handler")
	if err != nil {
		t.Fatalf("nodes for entity: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d nodes, want 3", len(got))
	}
	if got[0].Content != "Goal" || got[1].Content != "Authenticate requests" || got[2].Content != "Dependencies" {
		t.Errorf("unexpected order: %v, %v, %v", got[0].Content, got[1].Content, got[2].Content)
	}
}

func TestDeleteNode_CascadesChildren(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)

	parent := &Node{EntityID: "auth-handler", Type: "list", Seq: 0, Hash: "a"}
	parentID, _ := s.InsertNode(parent)

	child := &Node{
		EntityID: "auth-handler",
		ParentID: sql.NullInt64{Int64: parentID, Valid: true},
		Type:     "list_item",
		Seq:      0,
		Content:  "item 1",
		Hash:     "b",
	}
	childID, _ := s.InsertNode(child)

	if err := s.DeleteNode(parentID); err != nil {
		t.Fatalf("delete parent: %v", err)
	}

	_, err := s.GetNode(childID)
	if err == nil {
		t.Error("expected child to be cascade-deleted")
	}
}

func TestReplaceEntityNodes(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)

	// Insert initial nodes.
	s.InsertNode(&Node{EntityID: "auth-handler", Type: "heading", Seq: 0, Content: "Old", Hash: "old"})

	// Replace with new set.
	newNodes := []*Node{
		{Type: "heading", Level: 2, Seq: 0, Content: "Goal", Hash: "a"},
		{Type: "paragraph", Seq: 1, Content: "New goal text", Hash: "b"},
	}
	if err := s.ReplaceEntityNodes("auth-handler", newNodes); err != nil {
		t.Fatalf("replace: %v", err)
	}

	got, err := s.NodesForEntity("auth-handler")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d nodes, want 2", len(got))
	}
	if got[0].Content != "Goal" {
		t.Errorf("got %q, want Goal", got[0].Content)
	}
}

func TestNodeChildren(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)

	parent := &Node{EntityID: "auth-handler", Type: "table", Seq: 0, Hash: "t"}
	parentID, _ := s.InsertNode(parent)

	for i, content := range []string{"row1", "row2", "row3"} {
		s.InsertNode(&Node{
			EntityID: "auth-handler",
			ParentID: sql.NullInt64{Int64: parentID, Valid: true},
			Type:     "table_row",
			Seq:      i,
			Content:  content,
			Hash:     ComputeNodeHash(content, "table_row"),
		})
	}

	children, err := s.NodeChildren(parentID)
	if err != nil {
		t.Fatalf("children: %v", err)
	}
	if len(children) != 3 {
		t.Fatalf("got %d children, want 3", len(children))
	}
}

func TestUpdateNode(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)

	n := &Node{EntityID: "auth-handler", Type: "paragraph", Seq: 0, Content: "old", Hash: "old"}
	id, _ := s.InsertNode(n)

	n.ID = id
	n.Content = "new content"
	n.Hash = ComputeNodeHash("new content", "paragraph")
	if err := s.UpdateNode(n); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := s.GetNode(id)
	if got.Content != "new content" {
		t.Errorf("got %q, want 'new content'", got.Content)
	}
}

// insertRow seeds one table row under an entity at the given seq.
func insertRow(t *testing.T, s *Store, entityID string, seq int, content string) int64 {
	t.Helper()
	id, err := s.InsertNode(&Node{
		EntityID: entityID,
		Type:     "table_row",
		Seq:      seq,
		Content:  content,
		Hash:     ComputeNodeHash(content, "table_row"),
	})
	if err != nil {
		t.Fatalf("seed row %q: %v", content, err)
	}
	return id
}

func rowContents(t *testing.T, s *Store, entityID string) []string {
	t.Helper()
	nodes, err := s.NodesForEntity(entityID)
	if err != nil {
		t.Fatalf("nodes for entity: %v", err)
	}
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Content)
	}
	return out
}

func TestInsertNodeAfter_LastRow(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)
	insertRow(t, s, "auth-handler", 0, "r0")
	last := insertRow(t, s, "auth-handler", 1, "r1")

	if _, err := s.InsertNodeAfter(last, &Node{Type: "table_row", Content: "new", Hash: ComputeNodeHash("new", "table_row")}); err != nil {
		t.Fatalf("insert after last row: %v", err)
	}

	got := rowContents(t, s, "auth-handler")
	want := []string{"r0", "r1", "new"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// The regression: shifting later siblings up by one collides with the UNIQUE
// index on (entity_id, parent_id, seq), because SQLite enforces it per ROW —
// row seq=1 becomes 2 while a row at seq=2 still exists. Appending after the
// last row never hits it, which is why this shipped.
func TestInsertNodeAfter_MiddleRowShiftsLaterSiblings(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)
	insertRow(t, s, "auth-handler", 0, "r0")
	anchor := insertRow(t, s, "auth-handler", 1, "r1")
	insertRow(t, s, "auth-handler", 2, "r2")
	insertRow(t, s, "auth-handler", 3, "r3")

	if _, err := s.InsertNodeAfter(anchor, &Node{Type: "table_row", Content: "new", Hash: ComputeNodeHash("new", "table_row")}); err != nil {
		t.Fatalf("insert after middle row: %v", err)
	}

	got := rowContents(t, s, "auth-handler")
	want := []string{"r0", "r1", "new", "r2", "r3"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// Inserting after the FIRST row shifts the most siblings — the worst case.
func TestInsertNodeAfter_FirstRowOfManySiblings(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)
	anchor := insertRow(t, s, "auth-handler", 0, "r0")
	for i := 1; i < 8; i++ {
		insertRow(t, s, "auth-handler", i, string(rune('a'+i-1)))
	}

	if _, err := s.InsertNodeAfter(anchor, &Node{Type: "table_row", Content: "new", Hash: ComputeNodeHash("new", "table_row")}); err != nil {
		t.Fatalf("insert after first row: %v", err)
	}

	got := rowContents(t, s, "auth-handler")
	if len(got) != 9 {
		t.Fatalf("expected 9 rows, got %d: %v", len(got), got)
	}
	if got[0] != "r0" || got[1] != "new" || got[2] != "a" || got[8] != "g" {
		t.Fatalf("wrong order: %v", got)
	}
}

// Two inserts into the same table in one unit — the shape a change-unit uses
// when it adds two contract rows.
func TestInsertNodeAfter_TwiceIntoSameTable(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)
	anchor := insertRow(t, s, "auth-handler", 0, "r0")
	insertRow(t, s, "auth-handler", 1, "r1")

	first, err := s.InsertNodeAfter(anchor, &Node{Type: "table_row", Content: "n1", Hash: ComputeNodeHash("n1", "table_row")})
	if err != nil {
		t.Fatalf("first insert: %v", err)
	}
	if _, err := s.InsertNodeAfter(first, &Node{Type: "table_row", Content: "n2", Hash: ComputeNodeHash("n2", "table_row")}); err != nil {
		t.Fatalf("second insert: %v", err)
	}

	got := rowContents(t, s, "auth-handler")
	want := []string{"r0", "n1", "n2", "r1"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// Siblings under a parent shift independently of another parent's children.
func TestInsertNodeAfter_ScopedToItsParent(t *testing.T) {
	s := createTestStore(t)
	seedFixture(t, s)
	parentA, err := s.InsertNode(&Node{EntityID: "auth-handler", Type: "heading", Seq: 0, Content: "A", Hash: ComputeNodeHash("A", "heading")})
	if err != nil {
		t.Fatalf("parent A: %v", err)
	}
	parentB, err := s.InsertNode(&Node{EntityID: "auth-handler", Type: "heading", Seq: 1, Content: "B", Hash: ComputeNodeHash("B", "heading")})
	if err != nil {
		t.Fatalf("parent B: %v", err)
	}
	child := func(parent int64, seq int, content string) int64 {
		t.Helper()
		id, err := s.InsertNode(&Node{
			EntityID: "auth-handler", ParentID: sql.NullInt64{Int64: parent, Valid: true},
			Type: "table_row", Seq: seq, Content: content, Hash: ComputeNodeHash(content, "table_row"),
		})
		if err != nil {
			t.Fatalf("child %q: %v", content, err)
		}
		return id
	}
	a0 := child(parentA, 0, "a0")
	child(parentA, 1, "a1")
	child(parentB, 0, "b0")
	child(parentB, 1, "b1")

	if _, err := s.InsertNodeAfter(a0, &Node{Type: "table_row", Content: "new", Hash: ComputeNodeHash("new", "table_row")}); err != nil {
		t.Fatalf("insert under parent A: %v", err)
	}

	bKids, err := s.NodeChildren(parentB)
	if err != nil {
		t.Fatalf("children of B: %v", err)
	}
	if len(bKids) != 2 || bKids[0].Content != "b0" || bKids[0].Seq != 0 || bKids[1].Seq != 1 {
		t.Fatalf("parent B's children were disturbed: %+v", bKids)
	}
}
