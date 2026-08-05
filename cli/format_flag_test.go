package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun_UnknownFormatValueErrors(t *testing.T) {
	var buf bytes.Buffer
	err := run([]string{"--c3-dir", t.TempDir(), "list", "--format", "yaml"}, &buf)
	if err == nil {
		t.Fatal("unknown --format value should fail before any command runs")
	}
	msg := err.Error()
	if !strings.Contains(msg, `unknown --format "yaml"`) {
		t.Errorf("error should name the bad value\ngot: %s", msg)
	}
	if !strings.Contains(msg, "hint: valid values are text, toon, json, mermaid") {
		t.Errorf("error should list valid values\ngot: %s", msg)
	}
	if buf.Len() > 0 {
		t.Errorf("nothing should be written on a bad --format, got: %q", buf.String())
	}
}

func TestRun_KnownFormatValuesAccepted(t *testing.T) {
	for _, v := range []string{"text", "toon", "json", "mermaid"} {
		var buf bytes.Buffer
		// No .c3/ here, so the command still fails — just not on --format.
		err := run([]string{"--c3-dir", t.TempDir(), "list", "--format", v}, &buf)
		if err != nil && strings.Contains(err.Error(), "--format") {
			t.Errorf("--format %s should be accepted, got: %v", v, err)
		}
	}
}
