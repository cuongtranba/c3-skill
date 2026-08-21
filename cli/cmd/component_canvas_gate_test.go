package cmd

import (
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/schema"
)

// customComponentSections returns a minimal project component canvas: only Goal
// is required. This diverges from the built-in component canvas (which requires
// Parent Fit, Purpose, Governance, Contract, Change Safety, Derived Materials).
func customComponentSections() []schema.SectionDef {
	return []schema.SectionDef{
		{Name: "Goal", ContentType: "text", Required: true},
		{Name: "Container Connection", ContentType: "text", Required: false},
		{Name: "Design", ContentType: "text", Required: false},
	}
}

// TestComponentCanvasGate_HonorsProjectCanvas verifies that the canvas gate in
// validateBodyContentWithDefinition uses the schemaSections it receives, not the
// built-in component canvas. A body valid under a custom project canvas (Goal
// only required) must be accepted even though it lacks the built-in required
// sections (Parent Fit, Purpose, etc.).
func TestComponentCanvasGate_HonorsProjectCanvas(t *testing.T) {
	body := "# Auth\n\n## Goal\n\nOwns token validation for API requests.\n\n## Container Connection\n\nServes the API gateway container.\n"

	issues := validateBodyContentWithDefinition(body, "component", customComponentSections())
	for _, issue := range issues {
		t.Errorf("unexpected issue: %s: %s", issue.Severity, issue.Message)
	}
}

// TestComponentCanvasGate_EnforcesProjectRequired verifies that a missing
// project-required section (Goal) is still rejected even when the body would
// satisfy the built-in canvas shape in all other respects.
func TestComponentCanvasGate_EnforcesProjectRequired(t *testing.T) {
	body := "# Auth\n\n## Container Connection\n\nServes the API gateway container.\n"

	issues := validateBodyContentWithDefinition(body, "component", customComponentSections())
	if !hasIssue(issues, "missing required section: Goal") {
		t.Fatalf("expected missing-Goal issue, got %#v", issues)
	}
}

// TestComponentCanvasGate_RejectsUnknownSections verifies that sections outside
// the project canvas are flagged as unknown, even when the body is otherwise valid.
func TestComponentCanvasGate_RejectsUnknownSections(t *testing.T) {
	body := "# Auth\n\n## Goal\n\nOwns token validation.\n\n## Parent Fit\n\nThis is a built-in section not in the project canvas.\n"

	issues := validateBodyContentWithDefinition(body, "component", customComponentSections())
	if !hasIssue(issues, "unknown section: Parent Fit") {
		t.Fatalf("expected unknown-section issue for Parent Fit, got %#v", issues)
	}
}
