package cmd

import (
	"strings"
	"testing"
)

// Issue #116 item 2 — a table-shape rejection must name the offending row.
//
// markdown.ParseTable already knows the row number and the expected vs actual
// cell counts; every validator used to drop that error and report only
// "invalid required table: <Name>", whose hint listed section names — which is
// never the problem. Diagnosis was by hand-diffing the row against the schema.
// These tests pin the cause through each validator that surfaces the rejection.

// dropLastCell rewrites the first data row of the named table section so it
// carries one cell fewer than its header — the exact reported shape (a row with
// 4 cells where the canvas wants 5).
func dropLastCell(t *testing.T, body, sectionName string) string {
	t.Helper()
	lines := strings.Split(body, "\n")
	inSection := false
	seenSeparator := false
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "## "):
			inSection = strings.TrimSpace(strings.TrimPrefix(line, "## ")) == sectionName
			seenSeparator = false
		case inSection && strings.HasPrefix(line, "| ---"):
			seenSeparator = true
		case inSection && seenSeparator && strings.HasPrefix(line, "|"):
			cells := strings.Split(strings.Trim(strings.TrimSpace(line), "|"), "|")
			if len(cells) < 2 {
				t.Fatalf("row in %s has too few cells to shorten: %q", sectionName, line)
			}
			lines[i] = "|" + strings.Join(cells[:len(cells)-1], "|") + "|"
			return strings.Join(lines, "\n")
		}
	}
	t.Fatalf("no data row found in section %q", sectionName)
	return ""
}

// assertNamesOffendingRow finds the table-shape rejection for a section and
// requires it to carry the cause, not just the section name.
func assertNamesOffendingRow(t *testing.T, issues []Issue, sectionName, rowFragment string, wantCols, gotCols string) {
	t.Helper()
	marker := "invalid required table: " + sectionName
	for _, issue := range issues {
		if !strings.Contains(issue.Message, marker) {
			continue
		}
		text := issue.Message + " " + issue.Hint
		for _, want := range []string{"row", wantCols, gotCols, rowFragment} {
			if !strings.Contains(text, want) {
				t.Fatalf("table-shape rejection must name %q; got message=%q hint=%q", want, issue.Message, issue.Hint)
			}
		}
		return
	}
	t.Fatalf("no %q issue; got %+v", marker, issues)
}

func TestTableShapeError_StrictDocNamesOffendingRow(t *testing.T) {
	body := dropLastCell(t, strictComponentBody("auth", "Own authentication for API requests."), "Governance")
	assertNamesOffendingRow(t, validateStrictComponentDoc(body, "error"), "Governance", "ref-jwt", "5", "4")
}

func TestTableShapeError_WriteValidationNamesOffendingRow(t *testing.T) {
	body := dropLastCell(t, strictComponentBody("auth", "Own authentication for API requests."), "Governance")
	assertNamesOffendingRow(t, validateBodyContent(body, "component"), "Governance", "ref-jwt", "5", "4")
}

func TestTableShapeError_ADRCreationNamesOffendingRow(t *testing.T) {
	body := strings.Join([]string{
		"# Use Go",
		"",
		"## Affected Topology",
		"",
		"| Entity | Type | Why affected | Governance review | Evidence |",
		"| --- | --- | --- | --- | --- |",
		"| c3-101 | component | New parameter on the builder | Codemap updated |",
		"",
	}, "\n")
	assertNamesOffendingRow(t, validateADRCreationBody(body), "Affected Topology", "c3-101", "5", "4")
}
