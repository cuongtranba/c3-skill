package markdown

import (
	"fmt"
	"strings"
)

// Section represents a named section parsed from ## headers in markdown.
type Section struct {
	Name    string
	Content string
}

// Table represents a parsed markdown table.
type Table struct {
	Headers []string
	Rows    []map[string]string
}

// ParseSections splits a markdown body into sections by ## headers.
// Content before the first ## is captured as a preamble section with empty Name.
// Only ## (h2) headers create new sections; ### and deeper stay inside their parent.
// ## inside fenced code blocks are ignored.
func ParseSections(body string) []Section {
	lines := strings.Split(body, "\n")
	var sections []Section
	var currentName string
	var currentLines []string
	inFence := false
	started := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Track fenced code blocks
		if strings.HasPrefix(trimmed, "```") {
			inFence = !inFence
		}

		if !inFence && strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "### ") {
			// Save previous section
			if started {
				sections = append(sections, Section{
					Name:    currentName,
					Content: trimContent(currentLines),
				})
			}
			currentName = strings.TrimSpace(strings.TrimPrefix(trimmed, "## "))
			currentLines = nil
			started = true
		} else {
			if !started {
				// Preamble content before first ##
				currentLines = append(currentLines, line)
				started = true
				currentName = ""
			} else {
				currentLines = append(currentLines, line)
			}
		}
	}

	// Save last section
	if started {
		sections = append(sections, Section{
			Name:    currentName,
			Content: trimContent(currentLines),
		})
	}

	return sections
}

// trimContent joins lines and trims leading/trailing whitespace.
func trimContent(lines []string) string {
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// ParseTable parses a markdown table string into a Table struct.
func ParseTable(markdown string) (*Table, error) {
	lines := strings.Split(strings.TrimSpace(markdown), "\n")

	// Filter out comment rows and empty lines, find header/separator/data
	var filtered []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "<!--") {
			continue
		}
		filtered = append(filtered, line)
	}

	if len(filtered) < 2 {
		return nil, fmt.Errorf("not a valid markdown table: need at least header and separator rows")
	}

	// Parse header row
	headers := parseCells(filtered[0])
	if len(headers) == 0 {
		return nil, fmt.Errorf("not a valid markdown table: no headers found in header row: %s", strings.TrimSpace(filtered[0]))
	}

	// Verify separator row
	sepCells := parseCells(filtered[1])
	isSeparator := true
	for _, c := range sepCells {
		stripped := strings.Trim(c, "- :")
		if stripped != "" {
			isSeparator = false
			break
		}
	}
	if !isSeparator {
		return nil, fmt.Errorf("not a valid markdown table: second row is not a separator: %s", strings.TrimSpace(filtered[1]))
	}

	// Parse data rows
	var rows []map[string]string
	for rowIdx, line := range filtered[2:] {
		cells := parseCells(line)
		if len(cells) != len(headers) {
			return nil, fmt.Errorf("column count mismatch in data row %d: header has %d columns, row has %d: %s",
				rowIdx+1, len(headers), len(cells), strings.TrimSpace(line))
		}
		row := make(map[string]string, len(headers))
		for i, h := range headers {
			row[h] = cells[i]
		}
		rows = append(rows, row)
	}

	if rows == nil {
		rows = []map[string]string{}
	}

	return &Table{Headers: headers, Rows: rows}, nil
}

// SplitRowCells splits a markdown table row on UNESCAPED pipes, returning each
// cell's raw text with its `\|` escapes intact. It is the one place that knows
// where a table row's column boundaries are.
//
// The escape is the only thing separating a pipe INSIDE a cell from a delimiter,
// so a splitter that ignores it tears the cell apart. Rejoining the pieces then
// wrote `\|` back as `\ |`, which escapes a space rather than the pipe; the row
// now claimed more columns than its table had, and the next parse truncated it at
// the header's column count. A doc lost 533 bytes of a row no patch had targeted.
// Callers that want cell VALUES rather than storage text unescape on top — see
// parseCells.
func SplitRowCells(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|") // a leading pipe can never be escaped
	// A trailing `\|` is content, not the closing delimiter; stripping it would
	// eat the backslash and re-break the escape this function exists to keep.
	if strings.HasSuffix(line, "|") && !isEscapedPipeAt(line, len(line)-2) {
		line = line[:len(line)-1]
	}

	var cells []string
	var current strings.Builder
	for i := 0; i < len(line); i++ {
		switch {
		case isEscapedPipeAt(line, i):
			current.WriteString(`\|`)
			i++
		case line[i] == '|':
			cells = append(cells, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteByte(line[i])
		}
	}
	return append(cells, strings.TrimSpace(current.String()))
}

// parseCells splits a markdown table row into cell VALUES, with escaped pipes
// resolved to the literal pipe the author meant. escapeCell restores them on write.
func parseCells(line string) []string {
	cells := SplitRowCells(line)
	for i, c := range cells {
		cells[i] = strings.ReplaceAll(c, `\|`, "|")
	}
	return cells
}

// isEscapedPipeAt tolerates an out-of-range index so callers can probe a
// position that may not exist, such as the byte before a one-character row.
func isEscapedPipeAt(line string, i int) bool {
	return i >= 0 && i+1 < len(line) && line[i] == '\\' && line[i+1] == '|'
}

func escapeCell(value string) string {
	return strings.ReplaceAll(value, "|", `\|`)
}

func escapeCells(values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = escapeCell(v)
	}
	return out
}

// WriteTable converts a Table struct back to a markdown table string.
func WriteTable(t *Table) string {
	var sb strings.Builder

	// Header row
	sb.WriteString("| ")
	sb.WriteString(strings.Join(escapeCells(t.Headers), " | "))
	sb.WriteString(" |")
	sb.WriteString("\n")

	// Separator row
	sb.WriteString("|")
	for range t.Headers {
		sb.WriteString("------|")
	}
	sb.WriteString("\n")

	// Data rows
	for _, row := range t.Rows {
		sb.WriteString("| ")
		vals := make([]string, len(t.Headers))
		for i, h := range t.Headers {
			vals[i] = escapeCell(row[h])
		}
		sb.WriteString(strings.Join(vals, " | "))
		sb.WriteString(" |")
		sb.WriteString("\n")
	}

	return strings.TrimRight(sb.String(), "\n")
}

// ReplaceSection replaces the content of a named ## section in the markdown body.
func ReplaceSection(body string, name string, newContent string) (string, error) {
	sections := ParseSections(body)

	found := false
	for i := range sections {
		if sections[i].Name == name {
			sections[i].Content = newContent
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("section %q not found", name)
	}

	return reassemble(body, sections), nil
}

// reassemble reconstructs a markdown body from sections, preserving
// the leading whitespace style of the original.
func reassemble(originalBody string, sections []Section) string {
	var sb strings.Builder

	// Check if original body starts with a newline
	prefix := ""
	if strings.HasPrefix(originalBody, "\n") {
		prefix = "\n"
	}
	sb.WriteString(prefix)

	for i, s := range sections {
		if s.Name == "" {
			// Preamble
			if s.Content != "" {
				sb.WriteString(s.Content)
				sb.WriteString("\n")
			}
		} else {
			sb.WriteString("## ")
			sb.WriteString(s.Name)
			sb.WriteString("\n")
			if s.Content != "" {
				sb.WriteString("\n")
				sb.WriteString(s.Content)
				sb.WriteString("\n")
			} else {
				sb.WriteString("\n")
			}
		}
		// Add separator between sections (but not after the last)
		if i < len(sections)-1 && !(s.Name == "" && s.Content == "") {
			// Only add blank line if next section is named (not joining preamble with section)
			if sections[i+1].Name != "" || sections[i+1].Content != "" {
				// empty line not needed if we just wrote one
			}
		}
	}

	result := sb.String()
	// Ensure trailing newline
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result
}

// SetTableInSection replaces the table in a named section with a new table.
func SetTableInSection(body string, sectionName string, table *Table) (string, error) {
	newContent := WriteTable(table)
	return ReplaceSection(body, sectionName, newContent)
}

// AppendTableRow appends a row to the table in a named section.
func AppendTableRow(body string, sectionName string, row map[string]string) (string, error) {
	table, err := ExtractTableFromSection(body, sectionName)
	if err != nil {
		return "", err
	}
	if table == nil {
		return "", fmt.Errorf("no table found in section %q", sectionName)
	}

	table.Rows = append(table.Rows, row)
	return SetTableInSection(body, sectionName, table)
}

// ExtractTableFromSection extracts and parses the table from a named section.
func ExtractTableFromSection(body string, sectionName string) (*Table, error) {
	sections := ParseSections(body)

	var section *Section
	for i := range sections {
		if sections[i].Name == sectionName {
			section = &sections[i]
			break
		}
	}

	if section == nil {
		return nil, fmt.Errorf("section %q not found", sectionName)
	}

	// Find table lines in section content
	lines := strings.Split(section.Content, "\n")
	var tableLines []string
	inTable := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "|") || strings.HasPrefix(trimmed, "<!--") {
			inTable = true
			tableLines = append(tableLines, line)
		} else if inTable && trimmed == "" {
			// End of table
			break
		} else if inTable {
			break
		}
	}

	if len(tableLines) == 0 {
		return nil, nil
	}

	return ParseTable(strings.Join(tableLines, "\n"))
}
