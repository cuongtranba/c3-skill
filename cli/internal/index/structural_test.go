package index

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/codemap"
	"github.com/lagz0ne/c3-design/cli/internal/store"
)

func TestWrite_ExcludesRetiredEntities(t *testing.T) {
	s := openMemStore(t)
	mustInsert(t, s, &store.Entity{ID: "c3-0", Type: "system", Title: "TestProject", Status: "active", Metadata: "{}"})
	mustInsert(t, s, &store.Entity{ID: "c3-1", Type: "container", Title: "api", Slug: "api", ParentID: "c3-0", Status: "active", Metadata: "{}"})
	mustInsert(t, s, &store.Entity{ID: "c3-101", Type: "component", Title: "auth", Slug: "auth", ParentID: "c3-1", Status: "active", Metadata: "{}"})
	mustInsert(t, s, &store.Entity{ID: "c3-301", Type: "component", Title: "gone", Slug: "gone", ParentID: "c3-1", Status: "retired", Metadata: "{}"})

	dir := t.TempDir()
	if err := Write(dir, s, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "_index", "structural.md"))
	if err != nil {
		t.Fatalf("read structural.md: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "c3-301") {
		t.Errorf("structural.md should not contain retired entity c3-301:\n%s", got)
	}
	if !strings.Contains(got, "c3-101") {
		t.Errorf("structural.md should contain active entity c3-101:\n%s", got)
	}
}

func TestWrite_ExcludesADREntities(t *testing.T) {
	s := openMemStore(t)
	mustInsert(t, s, &store.Entity{ID: "c3-0", Type: "system", Title: "TestProject", Status: "active", Metadata: "{}"})
	mustInsert(t, s, &store.Entity{ID: "adr-20260101-use-go", Type: "adr", Title: "Use Go", Status: "proposed", Metadata: "{}"})

	dir := t.TempDir()
	if err := Write(dir, s, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "_index", "structural.md"))
	if strings.Contains(string(data), "adr-20260101-use-go") {
		t.Errorf("structural.md should not contain ADR entities:\n%s", string(data))
	}
}

func TestWrite_IncludesCodeBindings(t *testing.T) {
	s := openMemStore(t)
	mustInsert(t, s, &store.Entity{ID: "c3-0", Type: "system", Title: "TestProject", Status: "active", Metadata: "{}"})
	mustInsert(t, s, &store.Entity{ID: "c3-101", Type: "component", Title: "auth", Status: "active", Metadata: "{}"})

	bindings := codemap.CodeMap{"c3-101": {"src/auth/**"}}
	dir := t.TempDir()
	if err := Write(dir, s, bindings); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "_index", "structural.md"))
	got := string(data)
	if !strings.Contains(got, "src/auth/**") {
		t.Errorf("structural.md should contain code glob:\n%s", got)
	}
	if !strings.Contains(got, "## File Map") {
		t.Errorf("structural.md should contain File Map section:\n%s", got)
	}
}

func TestWrite_HashEmbeddedInOutput(t *testing.T) {
	s := openMemStore(t)
	mustInsert(t, s, &store.Entity{ID: "c3-0", Type: "system", Title: "TestProject", Status: "active", Metadata: "{}"})

	dir := t.TempDir()
	if err := Write(dir, s, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "_index", "structural.md"))
	got := string(data)
	if !strings.Contains(got, "<!-- hash: sha256:") {
		t.Errorf("structural.md should contain embedded hash comment:\n%s", got)
	}
}

func TestWrite_CreatesIndexDirectory(t *testing.T) {
	s := openMemStore(t)
	mustInsert(t, s, &store.Entity{ID: "c3-0", Type: "system", Title: "TestProject", Status: "active", Metadata: "{}"})

	dir := t.TempDir()
	if err := Write(dir, s, nil); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "_index")); err != nil {
		t.Errorf("_index directory should be created: %v", err)
	}
}

func openMemStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustInsert(t *testing.T, s *store.Store, e *store.Entity) {
	t.Helper()
	if err := s.InsertEntity(e); err != nil {
		t.Fatalf("insert %s: %v", e.ID, err)
	}
}
