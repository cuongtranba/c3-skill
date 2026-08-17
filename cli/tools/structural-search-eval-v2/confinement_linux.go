//go:build linux

package main

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

// openGovernedFileBeneathRoot re-resolves the path in the kernel. RESOLVE_BENEATH
// refuses a traversal that leaves the root and RESOLVE_NO_SYMLINKS refuses one
// that crosses any symlink, so a component swapped after the caller's Lstat walk
// cannot be followed between the walk and this open.
func openGovernedFileBeneathRoot(absoluteRoot, relative string) (*os.File, error) {
	rootFD, err := unix.Open(absoluteRoot, unix.O_PATH|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, errors.New("cannot pin governed root")
	}
	defer unix.Close(rootFD)
	fileFD, err := unix.Openat2(rootFD, relative, &unix.OpenHow{
		Flags:   unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW,
		Resolve: unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS,
	})
	if err != nil {
		return nil, errors.New("cannot pin governed file")
	}
	return os.NewFile(uintptr(fileFD), "governed-file"), nil
}
