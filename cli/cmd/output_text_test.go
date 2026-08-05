package cmd

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// withFormatFlag sets the process-wide --format selector for one test and
// restores it afterwards, so format tests cannot leak into their neighbours.
func withFormatFlag(t *testing.T, v string) {
	t.Helper()
	prev := outputFormatFlag
	setOutputFormatFlag(v)
	t.Cleanup(func() { setOutputFormatFlag(prev) })
}

func TestResolveFormat_ExplicitTextWinsOverAgent(t *testing.T) {
	withFormatFlag(t, "text")
	if got := ResolveFormat(false, true); got != FormatText {
		t.Errorf("--format text must win over agent-mode TOON, got %d", got)
	}
}

func TestResolveFormat_ExplicitTextWinsOverJSON(t *testing.T) {
	withFormatFlag(t, "text")
	if got := ResolveFormat(true, false); got != FormatText {
		t.Errorf("--format text must win over --json, got %d", got)
	}
}

func TestResolveFormat_ExplicitJSONWinsOverAgent(t *testing.T) {
	withFormatFlag(t, "json")
	if got := ResolveFormat(false, true); got != FormatJSON {
		t.Errorf("--format json must win over agent-mode TOON, got %d", got)
	}
}

func TestResolveFormat_ExplicitTOONWinsOverJSONFlag(t *testing.T) {
	withFormatFlag(t, "toon")
	if got := ResolveFormat(true, false); got != FormatTOON {
		t.Errorf("--format toon must win over --json, got %d", got)
	}
}

func TestResolveFormat_MermaidLeavesGeneralPathAlone(t *testing.T) {
	// graph owns --format mermaid (cmd/graph.go); the general writers must fall
	// through to their normal defaults so graph's behaviour is untouched.
	withFormatFlag(t, "mermaid")
	if got := ResolveFormat(false, true); got != FormatTOON {
		t.Errorf("agent default must stay TOON under --format mermaid, got %d", got)
	}
	if got := ResolveFormat(true, false); got != FormatJSON {
		t.Errorf("--json must still win under --format mermaid, got %d", got)
	}
}

func TestResolveFormat_NoFlagUnchanged(t *testing.T) {
	withFormatFlag(t, "")
	if got := ResolveFormat(false, true); got != FormatTOON {
		t.Errorf("agent default must remain TOON, got %d", got)
	}
	if got := ResolveFormat(true, true); got != FormatTOON {
		t.Errorf("agent + --json must remain TOON, got %d", got)
	}
	if got := ResolveFormat(true, false); got != FormatJSON {
		t.Errorf("human + --json must remain JSON, got %d", got)
	}
	if got := ResolveFormat(false, false); got != FormatTOON {
		t.Errorf("default structured output must remain TOON, got %d", got)
	}
}

func TestValidateFormatFlag_Valid(t *testing.T) {
	for _, v := range []string{"", "text", "toon", "json", "mermaid"} {
		if err := ValidateFormatFlag(v); err != nil {
			t.Errorf("--format %q should be accepted: %v", v, err)
		}
	}
}

func TestValidateFormatFlag_UnknownHasActionableHint(t *testing.T) {
	err := ValidateFormatFlag("yaml")
	if err == nil {
		t.Fatal("unknown --format value must error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"yaml"`) {
		t.Errorf("error should quote the offending value\ngot: %s", msg)
	}
	if !strings.Contains(msg, "hint:") {
		t.Errorf("error should carry a hint line\ngot: %s", msg)
	}
	for _, v := range []string{"text", "toon", "json", "mermaid"} {
		if !strings.Contains(msg, v) {
			t.Errorf("hint should name valid value %q\ngot: %s", v, msg)
		}
	}
}

func TestParseArgs_PublishesFormatFlag(t *testing.T) {
	t.Cleanup(func() { setOutputFormatFlag("") })

	opts := ParseArgs([]string{"read", "c3-101", "--format", "text"})
	if opts.Format != "text" {
		t.Fatalf("Format = %q", opts.Format)
	}
	if got := ResolveFormat(false, true); got != FormatText {
		t.Errorf("ParseArgs should publish --format to the output layer, got %d", got)
	}

	// A later parse without --format must clear the selector again.
	ParseArgs([]string{"read", "c3-101"})
	if got := ResolveFormat(false, true); got != FormatTOON {
		t.Errorf("parse without --format should reset to the agent default, got %d", got)
	}
}

func TestWriteObjectOutput_TextKeepsRealNewlines(t *testing.T) {
	t.Setenv("C3X_MODE", "agent")
	withFormatFlag(t, "text")
	type doc struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}
	v := doc{ID: "adr-20260724-slash-picker-local-only", Body: "| a | b |\n| - | - |"}

	var buf bytes.Buffer
	if err := WriteObjectOutput(&buf, v, ResolveFormat(true, isAgentMode()), nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, `\n`) {
		t.Fatalf("--format text must not emit literal backslash-n:\n%s", out)
	}
	if !strings.Contains(out, "id: adr-20260724-slash-picker-local-only\n") {
		t.Errorf("ids must survive intact\ngot:\n%s", out)
	}
	if !strings.Contains(out, "\n  | a | b |\n  | - | - |\n") {
		t.Errorf("markdown table should render as real lines\ngot:\n%s", out)
	}
}

func TestWriteObjectOutput_AgentDefaultStillTOON(t *testing.T) {
	// Non-regression: agent mode without --format keeps the escaped TOON form.
	t.Setenv("C3X_MODE", "agent")
	withFormatFlag(t, "")
	type doc struct {
		Body string `json:"body"`
	}

	var buf bytes.Buffer
	if err := WriteObjectOutput(&buf, doc{Body: "a\nb"}, ResolveFormat(true, isAgentMode()), nil); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "body: \"a\\nb\"\n" {
		t.Fatalf("agent default must stay TOON, got %q", got)
	}
}

func TestWriteTableOutput_TextKeepsTOONGrid(t *testing.T) {
	t.Setenv("C3X_MODE", "agent")
	withFormatFlag(t, "text")
	type item struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	}
	items := []item{{ID: "c3-1", Title: "api"}}

	var buf bytes.Buffer
	if err := WriteTableOutput(&buf, "entities", items, []string{"id", "title"}, FormatText, nil); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "entities[1]{id,title}:\n") || !strings.Contains(out, "  c3-1,api\n") {
		t.Errorf("text mode must keep the positional table grid\ngot:\n%s", out)
	}
}

func TestWriteJSON_FormatTextWinsOverAgentTOON(t *testing.T) {
	t.Setenv("C3X_MODE", "agent")
	withFormatFlag(t, "text")
	type doc struct {
		ID   string `json:"id"`
		Body string `json:"body"`
	}

	var buf bytes.Buffer
	if err := writeJSON(&buf, doc{ID: "c3-101", Body: "line one\nline two"}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, `\n`) {
		t.Fatalf("writeJSON under --format text must not escape newlines:\n%s", out)
	}
	if !strings.Contains(out, "  line one\n  line two\n") {
		t.Errorf("body lines should be readable as-is\ngot:\n%s", out)
	}
}

func TestWriteJSON_FormatJSONWinsOverAgentTOON(t *testing.T) {
	t.Setenv("C3X_MODE", "agent")
	withFormatFlag(t, "json")
	type doc struct {
		ID string `json:"id"`
	}

	var buf bytes.Buffer
	if err := writeJSON(&buf, doc{ID: "c3-101"}); err != nil {
		t.Fatal(err)
	}
	var got doc
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("--format json should emit JSON even in agent mode: %v\ngot:\n%s", err, buf.String())
	}
}

func TestWriteJSON_AgentDefaultUnchanged(t *testing.T) {
	// Non-regression: the agent-mode default must not move.
	t.Setenv("C3X_MODE", "agent")
	withFormatFlag(t, "")
	type doc struct {
		Body string `json:"body"`
	}

	var buf bytes.Buffer
	if err := writeJSON(&buf, doc{Body: "a\nb"}); err != nil {
		t.Fatal(err)
	}
	if got := buf.String(); got != "body: \"a\\nb\"\n" {
		t.Fatalf("agent writeJSON default must stay TOON, got %q", got)
	}
}

func TestHelp_DocumentsFormatFlag(t *testing.T) {
	var buf bytes.Buffer
	ShowHelp("", &buf)
	out := buf.String()
	if !strings.Contains(out, "--format") {
		t.Fatalf("global help must document --format\ngot:\n%s", out)
	}
	for _, v := range []string{"text", "toon", "json"} {
		if !strings.Contains(out, v) {
			t.Errorf("global help should name --format value %q", v)
		}
	}
}
