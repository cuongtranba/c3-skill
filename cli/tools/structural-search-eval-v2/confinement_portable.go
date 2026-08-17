//go:build !linux

package main

import (
	"errors"
	"os"
)

// openGovernedFileBeneathRoot re-resolves the path through os.Root, which refuses
// a traversal that leaves the root. openat2's RESOLVE_NO_SYMLINKS has no portable
// equivalent — os.Root permits a symlink that stays inside the root — so the
// caller's Lstat walk remains the symlink ban and its SameFile check is what
// refuses a component swapped between the walk and this open.
func openGovernedFileBeneathRoot(absoluteRoot, relative string) (*os.File, error) {
	root, err := os.OpenRoot(absoluteRoot)
	if err != nil {
		return nil, errors.New("cannot pin governed root")
	}
	defer root.Close()
	file, err := root.Open(relative)
	if err != nil {
		return nil, errors.New("cannot pin governed file")
	}
	return file, nil
}
