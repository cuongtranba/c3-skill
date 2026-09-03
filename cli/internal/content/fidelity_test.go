package content

import (
	"strings"
	"testing"
)

const escapedPipeDoc = "## Turns\n\n" +
	"| Fact | Detail |\n" +
	"| --- | --- |\n" +
	"| compaction | `ActiveTurn.compactionTurn` (`proactive` \\| `user`) splits the two questions |\n"

// brokenPipeDoc is escapedPipeDoc after the stage-1 corruption: `\|` written
// back as `\ |`, which escapes a space rather than the pipe.
var brokenPipeDoc = strings.ReplaceAll(escapedPipeDoc, `\|`, `\ |`)

func TestVerifyTableFidelity_AcceptsEscapedPipe(t *testing.T) {
	if err := VerifyTableFidelity(escapedPipeDoc); err != nil {
		t.Fatalf("escaped pipe must round-trip, got: %v", err)
	}
}

func TestVerifyTableFidelity_RejectsRowThatWouldTruncate(t *testing.T) {
	err := VerifyTableFidelity(brokenPipeDoc)
	if err == nil {
		t.Fatal("a row with more cells than its header truncates on write; want refusal")
	}
	if !strings.Contains(err.Error(), "splits the two questions") {
		t.Errorf("error must quote the row at risk, got: %v", err)
	}
}

// A short row is padded to the header width, losing nothing.
func TestVerifyTableFidelity_AcceptsShortRow(t *testing.T) {
	doc := "| a | b | c |\n| --- | --- | --- |\n| one | two |\n"
	if err := VerifyTableFidelity(doc); err != nil {
		t.Fatalf("short row is padded, not truncated, got: %v", err)
	}
}

// Pipes inside a fenced block are literal text, not table columns.
func TestVerifyTableFidelity_IgnoresFencedCode(t *testing.T) {
	doc := "## Example\n\n```\n| a | b | c | d |\n| --- | --- |\n| way | too | many | cells |\n```\n"
	if err := VerifyTableFidelity(doc); err != nil {
		t.Fatalf("fenced code is not a table, got: %v", err)
	}
}

// A table inside a fence longer than 3 backticks must be ignored, even when
// that fence's content also contains 3-backtick lines — those shorter runs
// must not prematurely close the outer fence and expose the table.
func TestVerifyTableFidelity_IgnoresTableInsideLongFence(t *testing.T) {
	doc := "## Example\n\n````mermaid\n" +
		"```\n" +
		"| a | b | c | d |\n" +
		"| --- | --- |\n" +
		"| way | too | many | cells |\n" +
		"```\n" +
		"````\n"
	if err := VerifyTableFidelity(doc); err != nil {
		t.Fatalf("table inside long fence must be ignored, got: %v", err)
	}
}

func TestCheckUnclosedFence_DetectsUnclosedFence(t *testing.T) {
	body := "## Design\n\n```````mermaid\ngraph TD\n    A --> B\n\n## Code References\n\nrefs.\n"
	if err := CheckUnclosedFence(body); err == nil {
		t.Fatal("unclosed 7-backtick fence must be detected")
	}
}

func TestCheckUnclosedFence_AcceptsProperlyClosedFence(t *testing.T) {
	body := "## Design\n\n```mermaid\ngraph TD\n```\n\n## Other\n\ncontent.\n"
	if err := CheckUnclosedFence(body); err != nil {
		t.Fatalf("properly closed fence must be accepted, got: %v", err)
	}
}

func TestCheckUnclosedFence_AcceptsLongFenceClosedWithSameLength(t *testing.T) {
	body := "## Design\n\n```````mermaid\ngraph TD\n```````\n\n## Other\n\ncontent.\n"
	if err := CheckUnclosedFence(body); err != nil {
		t.Fatalf("7-backtick fence closed with 7 backticks must be accepted, got: %v", err)
	}
}

// The guard exists to stop the silent write. WriteEntity is the choke point every
// body write funnels through, so a doc that would lose a row must never commit.
func TestWriteEntity_RefusesRowShapeLoss(t *testing.T) {
	s := testStore(t)
	seedEntity(t, s, "test-1", "component")

	if err := WriteEntity(s, "test-1", brokenPipeDoc); err == nil {
		t.Fatal("WriteEntity must refuse a doc whose row would truncate")
	}

	nodes, err := s.NodesForEntity("test-1")
	if err != nil {
		t.Fatalf("NodesForEntity: %v", err)
	}
	if len(nodes) != 0 {
		t.Errorf("refused write must not commit nodes, got %d", len(nodes))
	}
}

func TestWriteEntity_AcceptsEscapedPipe(t *testing.T) {
	s := testStore(t)
	seedEntity(t, s, "test-1", "component")

	if err := WriteEntity(s, "test-1", escapedPipeDoc); err != nil {
		t.Fatalf("WriteEntity: %v", err)
	}

	body, err := ReadEntity(s, "test-1")
	if err != nil {
		t.Fatalf("ReadEntity: %v", err)
	}
	if !strings.Contains(body, "splits the two questions") {
		t.Errorf("row lost its tail on round-trip: %q", body)
	}
}
