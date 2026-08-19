package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRepair_RegeneratesStructuralIndex(t *testing.T) {
	dir := t.TempDir()
	c3Dir := filepath.Join(dir, ".c3")
	setupMinimalC3Dir(t, c3Dir)

	indexDir := filepath.Join(c3Dir, "_index")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		t.Fatalf("mkdir _index: %v", err)
	}
	stale := "# C3 Structural Index\n<!-- hash: sha256:staledeadbeef -->\n\n## c3-retired-1 — Gone (component)\ncontainer: c3-1\n"
	if err := os.WriteFile(filepath.Join(indexDir, "structural.md"), []byte(stale), 0644); err != nil {
		t.Fatalf("write stale structural.md: %v", err)
	}

	var buf bytes.Buffer
	if err := RunRepair(RepairOptions{C3Dir: c3Dir}, &buf); err != nil {
		t.Fatalf("RunRepair: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(indexDir, "structural.md"))
	if err != nil {
		t.Fatalf("read structural.md after repair: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "c3-retired-1") {
		t.Errorf("structural.md still references retired entity after repair:\n%s", got)
	}
	if !strings.Contains(got, "C3 Structural Index") {
		t.Errorf("structural.md missing header after repair:\n%s", got)
	}
	if !strings.Contains(got, "c3-101") {
		t.Errorf("structural.md missing live entity c3-101 after repair:\n%s", got)
	}
}

func TestRunRepair_CreatesStructuralIndexWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	c3Dir := filepath.Join(dir, ".c3")
	setupMinimalC3Dir(t, c3Dir)

	var buf bytes.Buffer
	if err := RunRepair(RepairOptions{C3Dir: c3Dir}, &buf); err != nil {
		t.Fatalf("RunRepair: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(c3Dir, "_index", "structural.md"))
	if err != nil {
		t.Fatalf("structural.md not created by repair: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, "C3 Structural Index") {
		t.Errorf("structural.md missing header:\n%s", got)
	}
}

func TestRunCheckV2_WarnsAboutStaleCodeMapEntries(t *testing.T) {
	dir := t.TempDir()
	c3Dir := filepath.Join(dir, ".c3")
	if err := os.MkdirAll(c3Dir, 0755); err != nil {
		t.Fatalf("mkdir c3Dir: %v", err)
	}

	// code-map.yaml with one valid and one retired entity ID
	codeMap := "c3-101:\n  - src/auth/**\nc3-retired-1:\n  - ext/tui/**\n_exclude:\n  - dist/**\n"
	if err := os.WriteFile(filepath.Join(c3Dir, "code-map.yaml"), []byte(codeMap), 0644); err != nil {
		t.Fatalf("write code-map.yaml: %v", err)
	}

	s := createDBFixture(t)

	var buf bytes.Buffer
	// check does not fail on warnings, so we allow err or not
	_ = RunCheckV2(CheckOptions{Store: s, C3Dir: c3Dir, JSON: false}, &buf)

	output := buf.String()
	if !strings.Contains(output, "c3-retired-1") {
		t.Errorf("check should warn about stale code-map entry c3-retired-1, got:\n%s", output)
	}
	if strings.Contains(output, "c3-101") && strings.Contains(output, "stale") {
		t.Errorf("check should NOT warn about live code-map entry c3-101")
	}
}

// setupMinimalC3Dir creates a minimal C3 directory with a sealed system entity
// and one strict-compliant component, suitable for testing repair (Force=true reseals automatically).
func setupMinimalC3Dir(t *testing.T, c3Dir string) {
	t.Helper()
	for _, sub := range []string{
		c3Dir,
		filepath.Join(c3Dir, "c3-1-api"),
		filepath.Join(c3Dir, "refs"),
	} {
		if err := os.MkdirAll(sub, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", sub, err)
		}
	}

	writeFile(t, filepath.Join(c3Dir, "README.md"), `---
id: c3-0
title: TestProject
---

# TestProject

## Goal

Test the system.

## Containers

| ID | Name | Boundary | Goal |
|----|------|----------|------|
| c3-1 | api | service | Serve API requests |
`)

	writeFile(t, filepath.Join(c3Dir, "refs", "ref-jwt.md"), `---
id: ref-jwt
title: JWT Authentication
goal: Standardize auth tokens
scope: [c3-1]
---

# JWT Authentication

## Goal

Standardize auth tokens.

## Choice

Use RS256 signed JWTs.

## Why

Industry standard, asymmetric verification.
`)

	writeFile(t, filepath.Join(c3Dir, "c3-1-api", "README.md"), `---
id: c3-1
title: api
type: container
boundary: service
parent: c3-0
goal: Serve API requests
---

# api

## Goal

Serve API requests.

## Components

| ID | Name | Category | Status | Goal Contribution |
|----|------|----------|--------|-------------------|
| c3-101 | auth | foundation | active | Authentication |

## Responsibilities

Handle all API traffic.
`)

	writeFile(t, filepath.Join(c3Dir, "c3-1-api", "c3-101-auth.md"), `---
id: c3-101
title: auth
type: component
category: foundation
parent: c3-1
uses: [ref-jwt]
---

# auth

## Goal

Handle authentication behavior for API requests.

## Parent Fit

| Field | Value |
| --- | --- |
| Parent role | Serves the parent container by owning authentication review evidence. |
| Parent constraint | Must preserve the parent API boundary and avoid cross-container policy ownership. |
| Upstream foundation | Depends on c3-1 container responsibilities and ref-jwt governance. |
| Downstream business value | Enables users workflow to trust authenticated API requests. |

## Purpose

Own authentication behavior for API requests, including token acceptance, failure semantics, and review evidence. It does not own user profile storage or system-wide security policy.

## Foundational Flow

| Aspect | Detail | Reference |
| --- | --- | --- |
| Preconditions | API request reaches authentication boundary with credentials available for validation. | ref-jwt |
| Inputs | Credentials and token material provided by caller. | ref-jwt |
| State / data | Does not persist user records; preserves token validation invariants. | ref-jwt |
| Shared dependencies | Uses shared token governance as the only dependency. | ref-jwt |

## Business Flow

| Aspect | Detail | Reference |
| --- | --- | --- |
| User/business outcome | Authenticated API requests can proceed to downstream workflows. | ref-jwt |
| Primary path | Validate token material, expose accepted identity, reject invalid requests. | ref-jwt |
| Alternate paths | Missing credentials produce rejection without mutating state. | ref-jwt |
| Failure behavior | Invalid token stops request before downstream behavior runs. | ref-jwt |

## Governance

| Reference | Type | Governs | Precedence | Notes |
| --- | --- | --- | --- | --- |
| ref-jwt | ref | Token format and validation expectations. | scoped ref beats local prose | Applies because component cites JWT behavior. |

## Contract

| Surface | Direction | Contract | Boundary | Evidence |
| --- | --- | --- | --- | --- |
| credentials | IN | Accept credential material for validation only. | API request boundary | ref-jwt |
| identity result | OUT | Provide accepted identity or explicit rejection. | Downstream workflow | ref-jwt |

## Change Safety

| Risk | Trigger | Detection | Required Verification |
| --- | --- | --- | --- |
| Invalid acceptance | Token validation changes without ref alignment. | Review ref-jwt mapping and auth tests. | go test ./cmd |
| Downstream break | Output identity contract changes. | Lookup consumers and inspect workflow. | go test ./... |

## Derived Materials

| Material | Must derive from | Allowed variance | Evidence |
| --- | --- | --- | --- |
| Code | Contract and Change Safety sections. | Names may differ while behavior stays equivalent. | go test ./... |
| Tests | Change Safety and Contract sections. | Test helper shape may differ. | go test ./... |
`)
}
