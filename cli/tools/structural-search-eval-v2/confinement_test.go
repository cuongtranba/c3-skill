package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resolvedTempDir resolves the temp dir because a governed root may not be
// reached through a symlink, and macOS hands out /var/folders paths where /var
// is one. Tests that compare a path against one a subprocess reports back need
// the same resolution.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// noopExecutable names a real do-nothing binary for confinement specs that only
// need an executable to point at. Its path differs across platforms.
func noopExecutable(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{"/bin/true", "/usr/bin/true"} {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	t.Skip("no true(1) binary on this platform")
	return ""
}

func writeGovernedFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReadBoundedRegularFileInsideAcceptsFileBeneathRoot(t *testing.T) {
	root := resolvedTempDir(t)
	governed := filepath.Join(root, "nested", "record.json")
	writeGovernedFile(t, governed, "generic\n")

	data, err := readBoundedRegularFileInside(root, governed, 4<<20)
	if err != nil {
		t.Fatalf("a regular file beneath the root must be readable: %v", err)
	}
	if string(data) != "generic\n" {
		t.Fatalf("governed read returned %q", data)
	}
}

func TestReadBoundedRegularFileInsideRejectsSymlinkedLeaf(t *testing.T) {
	root := resolvedTempDir(t)
	outside := filepath.Join(resolvedTempDir(t), "outside.json")
	writeGovernedFile(t, outside, "escaped\n")
	link := filepath.Join(root, "inside.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}

	if _, err := readBoundedRegularFileInside(root, link, 4<<20); err == nil {
		t.Fatal("a symlinked leaf escaping the root was accepted")
	}
}

func TestReadBoundedRegularFileInsideRejectsSymlinkedDirectoryComponent(t *testing.T) {
	root := resolvedTempDir(t)
	outsideDir := resolvedTempDir(t)
	writeGovernedFile(t, filepath.Join(outsideDir, "record.json"), "escaped\n")
	if err := os.Symlink(outsideDir, filepath.Join(root, "nested")); err != nil {
		t.Fatal(err)
	}

	if _, err := readBoundedRegularFileInside(root, filepath.Join(root, "nested", "record.json"), 4<<20); err == nil {
		t.Fatal("a symlinked directory component escaping the root was accepted")
	}
}

func TestReadBoundedRegularFileInsideRejectsEscapingRelativePath(t *testing.T) {
	root := resolvedTempDir(t)
	sibling := filepath.Join(filepath.Dir(root), "sibling.json")
	writeGovernedFile(t, sibling, "escaped\n")
	t.Cleanup(func() { os.Remove(sibling) })

	if _, err := readBoundedRegularFileInside(root, sibling, 4<<20); err == nil {
		t.Fatal("a path outside the root was accepted")
	}
}

func TestReadBoundedRegularFileInsideRejectsRootSymlinkAlias(t *testing.T) {
	real := resolvedTempDir(t)
	governed := filepath.Join(real, "record.json")
	writeGovernedFile(t, governed, "generic\n")
	alias := filepath.Join(resolvedTempDir(t), "root-alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}

	if _, err := readBoundedRegularFileInside(alias, filepath.Join(alias, "record.json"), 4<<20); err == nil {
		t.Fatal("a root reached through a symlink alias was accepted")
	}
}

func TestReadBoundedRegularFileInsideRejectsOversizedFile(t *testing.T) {
	root := resolvedTempDir(t)
	governed := filepath.Join(root, "record.json")
	writeGovernedFile(t, governed, strings.Repeat("x", 128))

	if _, err := readBoundedRegularFileInside(root, governed, 16); err == nil {
		t.Fatal("a file exceeding the byte cap was accepted")
	}
}

func TestReadBoundedRegularFileInsideRejectsDirectoryAndMissingTarget(t *testing.T) {
	root := resolvedTempDir(t)
	if err := os.MkdirAll(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := readBoundedRegularFileInside(root, filepath.Join(root, "nested"), 4<<20); err == nil {
		t.Fatal("a directory was accepted as a governed file")
	}
	if _, err := readBoundedRegularFileInside(root, filepath.Join(root, "absent.json"), 4<<20); err == nil {
		t.Fatal("a missing governed file was accepted")
	}
}
