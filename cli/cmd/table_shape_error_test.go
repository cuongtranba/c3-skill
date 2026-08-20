package cmd

import (
	"strings"
	"testing"
)

func dropLastCellFromFirstDataRow(t *testing.T, body, sectionName string) string {
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

func assertNamesOffendingRow(t *testing.T, issues []Issue, sectionName, rowFragment string, wantCols, gotCols string) {
	t.Helper()
	tableShapeRejection := "invalid required table: " + sectionName
	for _, issue := range issues {
		if !strings.Contains(issue.Message, tableShapeRejection) {
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
	t.Fatalf("no %q issue; got %+v", tableShapeRejection, issues)
}

func TestTableShapeError_StrictDocNamesOffendingRow(t *testing.T) {
	body := dropLastCellFromFirstDataRow(t, strictComponentBody("auth", "Own authentication for API requests."), "Governance")
	assertNamesOffendingRow(t, validateStrictComponentDoc(body, "error"), "Governance", "ref-jwt", "5", "4")
}

func TestTableShapeError_WriteValidationNamesOffendingRow(t *testing.T) {
	body := dropLastCellFromFirstDataRow(t, strictComponentBody("auth", "Own authentication for API requests."), "Governance")
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
