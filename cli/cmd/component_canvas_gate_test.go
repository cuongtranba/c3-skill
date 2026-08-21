package cmd

import (
	"strings"
	"testing"

	"github.com/lagz0ne/c3-design/cli/internal/schema"
)

// A project that overrides the component canvas must be validated against ITS
// shape everywhere, not just in `check`. The change-apply gate reaches component
// bodies through validateBodyContentWithDefinition, so that path has to honour
// the resolved sections the same way check_enhanced does — otherwise every
// component in such a project is unpatchable while `check` reports it clean.
// Regression: cuongtranba/c3-skill#41.
func TestValidateBodyContentWithDefinition_ComponentUsesResolvedCanvas(t *testing.T) {
	projectSections := []schema.SectionDef{
		{Name: "Goal", ContentType: "text", Required: true},
		{Name: "Custom Project Section", ContentType: "text", Required: true},
	}

	body := "# widget\n\n## Goal\n\nServe the widget listing for the storefront.\n"

	issues := validateBodyContentWithDefinition(body, "component", projectSections)

	var messages []string
	for _, issue := range issues {
		messages = append(messages, issue.Message)
	}
	joined := strings.Join(messages, "\n")

	if !strings.Contains(joined, "missing required section: Custom Project Section") {
		t.Errorf("expected the project canvas's required section to be enforced, got:\n%s", joined)
	}

	for _, builtinOnly := range []string{"Parent Fit", "Purpose", "Governance", "Contract", "Derived Materials"} {
		if strings.Contains(joined, "missing required section: "+builtinOnly) {
			t.Errorf("gate demanded built-in section %q that the project canvas does not define, got:\n%s", builtinOnly, joined)
		}
	}
}

// The built-in canvas must still apply when a project has not overridden it,
// so the fix above cannot be a blanket removal of the strict component rules.
func TestValidateBodyContentWithDefinition_ComponentFallsBackToBuiltinCanvas(t *testing.T) {
	body := "# widget\n\n## Goal\n\nServe the widget listing for the storefront.\n"

	issues := validateBodyContentWithDefinition(body, "component", schema.ForType("component"))

	var messages []string
	for _, issue := range issues {
		messages = append(messages, issue.Message)
	}
	joined := strings.Join(messages, "\n")

	if !strings.Contains(joined, "missing required section: Parent Fit") {
		t.Errorf("expected the built-in component canvas to still be enforced, got:\n%s", joined)
	}
}
