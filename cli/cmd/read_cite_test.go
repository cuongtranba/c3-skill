package cmd

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

// nodeHandleRE matches the node-anchored cite form that block patches and ADR
// Evidence cells require: <entity>#n<node>@v<version>:sha256:<nodeHash>.
var nodeHandleRE = regexp.MustCompile(`c3-101#n\d+@v\d+:sha256:[a-f0-9]{64}`)

// entityHandleRE matches the entity-root form that insert/frontmatter/retire
// patches anchor on. The `@` right after the id keeps it from matching a node handle.
var entityHandleRE = regexp.MustCompile(`c3-101@v\d+:sha256:[a-f0-9]{64}`)

func TestRunRead_CiteEmitsNodeHandles(t *testing.T) {
	s := createRichDBFixture(t)
	var buf bytes.Buffer

	if err := RunRead(ReadOptions{Store: s, ID: "c3-101", Cite: true}, &buf); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !entityHandleRE.MatchString(output) {
		t.Errorf("entity-root handle missing from bare --cite output:\n%s", output)
	}
	handles := nodeHandleRE.FindAllString(output, -1)
	if len(handles) == 0 {
		t.Fatalf("bare --cite emitted no node handles; an Evidence cell cannot be built from this output:\n%s", output)
	}
	if !strings.Contains(output, `"Handle authentication behavior for API requests."`) {
		t.Errorf("node handles should carry their source snippet:\n%s", output)
	}
}

func TestRunRead_CiteLabelsBothHandleKinds(t *testing.T) {
	s := createRichDBFixture(t)
	var buf bytes.Buffer

	if err := RunRead(ReadOptions{Store: s, ID: "c3-101", Cite: true}, &buf); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	// The entity handle keeps its existing "citation: <handle>" shape.
	if !strings.Contains(output, "citation: c3-101@v1:sha256:") {
		t.Errorf("entity citation line changed shape:\n%s", output)
	}
	if !strings.Contains(output, "node citations:") {
		t.Errorf("node handles should be listed under their own label:\n%s", output)
	}
	if !strings.Contains(output, "block patch") || !strings.Contains(output, "Evidence") {
		t.Errorf("output should say what the node handles anchor:\n%s", output)
	}
}

func TestRunRead_CiteJSONCarriesNodeCitations(t *testing.T) {
	s := createRichDBFixture(t)
	var buf bytes.Buffer

	if err := RunRead(ReadOptions{Store: s, ID: "c3-101", JSON: true, Cite: true}, &buf); err != nil {
		t.Fatal(err)
	}

	var result ReadResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	// Citation stays the entity-root handle so existing consumers keep working.
	if !entityHandleRE.MatchString(result.Citation) {
		t.Errorf("citation = %q, want entity-root handle", result.Citation)
	}
	if len(result.NodeCitations) == 0 {
		t.Fatalf("node_citations empty: %s", buf.String())
	}
	for _, c := range result.NodeCitations {
		if !nodeHandleRE.MatchString(c) {
			t.Errorf("node citation %q is not a node handle", c)
		}
	}
	// The JSON key must be present on the wire, not just on the struct.
	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	if _, ok := raw["node_citations"]; !ok {
		t.Errorf("node_citations key missing from JSON: %s", buf.String())
	}
}

func TestRunRead_CiteWithoutFlagEmitsNoCitations(t *testing.T) {
	s := createRichDBFixture(t)
	var buf bytes.Buffer

	if err := RunRead(ReadOptions{Store: s, ID: "c3-101", JSON: true}, &buf); err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	if _, ok := raw["node_citations"]; ok {
		t.Errorf("node_citations should be omitted without --cite: %s", buf.String())
	}
}

// TestRunRead_CiteCoversEverySection proves the bare listing spans the whole
// document: every handle --section --cite yields is present in it.
func TestRunRead_CiteCoversEverySection(t *testing.T) {
	s := createRichDBFixture(t)
	var all bytes.Buffer
	if err := RunRead(ReadOptions{Store: s, ID: "c3-101", JSON: true, Cite: true}, &all); err != nil {
		t.Fatal(err)
	}
	var doc ReadResult
	if err := json.Unmarshal(all.Bytes(), &doc); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	have := make(map[string]bool, len(doc.NodeCitations))
	for _, c := range doc.NodeCitations {
		have[c] = true
	}

	for _, section := range []string{"Goal", "Purpose", "Parent Fit", "Business Flow"} {
		var sec bytes.Buffer
		if err := RunRead(ReadOptions{Store: s, ID: "c3-101", Section: section, JSON: true, Cite: true}, &sec); err != nil {
			t.Fatalf("section %q: %v", section, err)
		}
		var sr ReadSectionResult
		if err := json.Unmarshal(sec.Bytes(), &sr); err != nil {
			t.Fatalf("section %q JSON parse: %v", section, err)
		}
		if len(sr.Citations) == 0 {
			t.Fatalf("section %q produced no citations to compare", section)
		}
		for _, c := range sr.Citations {
			if !have[c] {
				t.Errorf("section %q citation missing from whole-document listing: %s", section, c)
			}
		}
	}
}

// TestRunRead_SectionCiteShapeUnchanged guards the documented rebase workflow:
// --section --cite still emits section node handles only, with no entity root.
func TestRunRead_SectionCiteShapeUnchanged(t *testing.T) {
	s := createRichDBFixture(t)
	var buf bytes.Buffer

	if err := RunRead(ReadOptions{Store: s, ID: "c3-101", Section: "Goal", Cite: true}, &buf); err != nil {
		t.Fatal(err)
	}

	output := buf.String()
	if !strings.Contains(output, "citation: c3-101#n") {
		t.Errorf("section citation line changed shape:\n%s", output)
	}
	if entityHandleRE.MatchString(output) {
		t.Errorf("section --cite must not emit the entity-root handle:\n%s", output)
	}
	if strings.Contains(output, "node citations:") {
		t.Errorf("section --cite should keep its existing single-citation label:\n%s", output)
	}
}

func TestRunRead_SectionCiteJSONShapeUnchanged(t *testing.T) {
	s := createRichDBFixture(t)
	var buf bytes.Buffer

	if err := RunRead(ReadOptions{Store: s, ID: "c3-101", Section: "Goal", JSON: true, Cite: true}, &buf); err != nil {
		t.Fatal(err)
	}

	var raw map[string]any
	if err := json.Unmarshal(buf.Bytes(), &raw); err != nil {
		t.Fatalf("JSON parse: %v", err)
	}
	for _, key := range []string{"citation", "node_citations"} {
		if _, ok := raw[key]; ok {
			t.Errorf("section --cite JSON gained unexpected key %q: %s", key, buf.String())
		}
	}
	if _, ok := raw["citations"]; !ok {
		t.Errorf("section --cite JSON lost citations: %s", buf.String())
	}
}
