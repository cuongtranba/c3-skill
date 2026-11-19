package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/store"
)

func TestRunSet_FrontmatterField(t *testing.T) {
	s := createRichDBFixture(t)
	var buf bytes.Buffer

	opts := SetOptions{Store: s, ID: "c3-101", Field: "goal", Value: "Handle JWT authentication"}
	err := RunSet(opts, &buf)
	if err != nil {
		t.Fatal(err)
	}

	entity, _ := s.GetEntity("c3-101")
	if entity.Goal != "Handle JWT authentication" {
		t.Errorf("goal = %q, want %q", entity.Goal, "Handle JWT authentication")
	}

	output := buf.String()
	if !strings.Contains(output, "Updated") {
		t.Errorf("should print Updated message, got: %s", output)
	}
}

func TestRunSet_EntityNotFound(t *testing.T) {
	s := createDBFixture(t)
	var buf bytes.Buffer

	opts := SetOptions{Store: s, ID: "c3-999", Field: "goal", Value: "test"}
	err := RunSet(opts, &buf)
	if err == nil {
		t.Fatal("expected error for nonexistent entity")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should mention 'not found': %v", err)
	}
}

func TestRunSet_UpdatesContainerGoal(t *testing.T) {
	s := createDBFixture(t)
	var buf bytes.Buffer

	opts := SetOptions{Store: s, ID: "c3-1", Field: "goal", Value: "Serve high-performance API requests"}
	err := RunSet(opts, &buf)
	if err != nil {
		t.Fatal(err)
	}

	entity, _ := s.GetEntity("c3-1")
	if entity.Goal != "Serve high-performance API requests" {
		t.Errorf("container goal = %q", entity.Goal)
	}
}

func TestRunSet_UpdatesRefGoal(t *testing.T) {
	s := createDBFixture(t)
	var buf bytes.Buffer

	opts := SetOptions{Store: s, ID: "ref-jwt", Field: "goal", Value: "Standardize JWT token format"}
	err := RunSet(opts, &buf)
	if err != nil {
		t.Fatal(err)
	}

	entity, _ := s.GetEntity("ref-jwt")
	if entity.Goal != "Standardize JWT token format" {
		t.Errorf("ref goal = %q", entity.Goal)
	}
}

func TestRunSet_UpdatesAdrStatus(t *testing.T) {
	s := createDBFixture(t)
	var buf bytes.Buffer

	opts := SetOptions{Store: s, ID: "adr-20260226-use-go", Field: "status", Value: "accepted"}
	err := RunSet(opts, &buf)
	if err != nil {
		t.Fatal(err)
	}

	entity, _ := s.GetEntity("adr-20260226-use-go")
	if entity.Status != "accepted" {
		t.Errorf("adr status = %q, want %q", entity.Status, "accepted")
	}
}

func TestRunSet_BlocksAdrProposedToImplemented(t *testing.T) {
	s := createDBFixture(t)
	var buf bytes.Buffer

	opts := SetOptions{Store: s, ID: "adr-20260226-use-go", Field: "status", Value: "implemented"}
	err := RunSet(opts, &buf)
	if err == nil {
		t.Fatal("expected error for proposed -> implemented transition")
	}
	if !strings.Contains(err.Error(), "accepted") {
		t.Errorf("error should hint at accepted intermediate state: %v", err)
	}

	entity, _ := s.GetEntity("adr-20260226-use-go")
	if entity.Status != "proposed" {
		t.Errorf("adr status should remain %q after blocked transition, got %q", "proposed", entity.Status)
	}
}

func TestRunSet_AllowsAdrAcceptedToImplemented(t *testing.T) {
	s := createDBFixture(t)
	var buf bytes.Buffer

	if err := RunSet(SetOptions{Store: s, ID: "adr-20260226-use-go", Field: "status", Value: "accepted"}, &buf); err != nil {
		t.Fatalf("proposed -> accepted: %v", err)
	}
	if err := RunSet(SetOptions{Store: s, ID: "adr-20260226-use-go", Field: "status", Value: "implemented"}, &buf); err != nil {
		t.Fatalf("accepted -> implemented: %v", err)
	}

	entity, _ := s.GetEntity("adr-20260226-use-go")
	if entity.Status != "implemented" {
		t.Errorf("adr status = %q after accepted -> implemented, want %q", entity.Status, "implemented")
	}
}

func TestRunSet_AllowsAdrProposedToProvisioned(t *testing.T) {
	s := createDBFixture(t)
	var buf bytes.Buffer

	opts := SetOptions{Store: s, ID: "adr-20260226-use-go", Field: "status", Value: "provisioned"}
	if err := RunSet(opts, &buf); err != nil {
		t.Fatalf("proposed -> provisioned should be allowed (design-only): %v", err)
	}

	entity, _ := s.GetEntity("adr-20260226-use-go")
	if entity.Status != "provisioned" {
		t.Errorf("adr status = %q, want %q", entity.Status, "provisioned")
	}
}

func TestRunSet_InvalidField(t *testing.T) {
	s := createDBFixture(t)
	var buf bytes.Buffer

	opts := SetOptions{Store: s, ID: "c3-101", Field: "nonexistent_field", Value: "test"}
	err := RunSet(opts, &buf)
	if err == nil {
		t.Error("expected error for unknown field name")
	}
}

func TestRunSet_AllFields(t *testing.T) {
	fields := map[string]string{
		"boundary": "service",
		"category": "feature",
		"title":    "New Title",
		"date":     "2026-03-20",
	}

	for field, value := range fields {
		t.Run(field, func(t *testing.T) {
			s := createRichDBFixture(t)
			var buf bytes.Buffer
			opts := SetOptions{Store: s, ID: "c3-101", Field: field, Value: value}
			err := RunSet(opts, &buf)
			if err != nil {
				t.Fatal(err)
			}
			entity, _ := s.GetEntity("c3-101")
			var got string
			switch field {
			case "boundary":
				got = entity.Boundary
			case "category":
				got = entity.Category
			case "title":
				got = entity.Title
			case "date":
				got = entity.Date
			}
			if got != value {
				t.Errorf("%s = %q, want %q", field, got, value)
			}
		})
	}
}

// Regression: set goal wrote only the cached goal field, which the next import
// re-derives from the ## Goal section, so the new goal vanished on a clean clone.
func TestRunSet_GoalSurvivesFreshImport(t *testing.T) {
	c3Dir := filepath.Join(t.TempDir(), ".c3")
	setupMinimalC3Dir(t, c3Dir)
	if err := RunRepair(RepairOptions{C3Dir: c3Dir}, &bytes.Buffer{}); err != nil {
		t.Fatalf("RunRepair on clean tree: %v", err)
	}
	s, err := store.Open(filepath.Join(c3Dir, "c3.db"))
	if err != nil {
		t.Fatal(err)
	}
	const goal = "Issue and verify signed session tokens for every API request."
	if err := RunSet(SetOptions{Store: s, C3Dir: c3Dir, ID: "c3-101", Field: "goal", Value: goal}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if err := AutoExportCanonical(s, c3Dir); err != nil {
		t.Fatal(err)
	}
	s.Close()

	var out bytes.Buffer
	if err := RunVerify(VerifyOptions{C3Dir: c3Dir, JSON: true}, &out); err != nil {
		t.Fatalf("verify against a fresh import must pass after set goal: %v\n%s", err, out.String())
	}
	fresh, cleanup, err := openFreshImportStore(c3Dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	e, err := fresh.GetEntity("c3-101")
	if err != nil {
		t.Fatal(err)
	}
	if e.Goal != goal {
		t.Errorf("goal after a fresh import = %q, want %q", e.Goal, goal)
	}
}
