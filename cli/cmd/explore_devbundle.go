package cmd

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// devExplorerShell serves the explorer bundle straight from a repo dev
// checkout, rebuilding it with npm when the sources are newer than the dist.
// Detection walks up from the running binary (not the CWD, which is usually
// the user's project), so `c3x explore` run anywhere still picks up local
// frontend edits. Outside a dev checkout it reports !ok and the embedded
// bundle is used.
func devExplorerShell(w io.Writer) (string, bool) {
	appDir := findExplorerAppDir()
	if appDir == "" {
		return "", false
	}
	distPath := filepath.Join(appDir, "dist", "index.html")

	if explorerBundleStale(appDir, distPath) {
		if err := buildExplorerBundle(appDir, w); err != nil {
			fmt.Fprintf(w, "warning: explorer bundle rebuild failed (%v); serving the previous bundle\n", err)
		}
	}

	shell, err := os.ReadFile(distPath)
	if err != nil {
		return "", false
	}

	// Keep the go:embed source current so the next `go build` embeds this bundle.
	embedPath := filepath.Join(filepath.Dir(appDir), "cli", "cmd", "assets", "explorer", "dist", "index.html")
	_ = os.WriteFile(embedPath, shell, 0o644)

	return string(shell), true
}

func findExplorerAppDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	dir := filepath.Dir(exe)
	for i := 0; i < 8; i++ {
		candidate := filepath.Join(dir, "explorer-app")
		if fileExistsAt(filepath.Join(candidate, "package.json")) && fileExistsAt(filepath.Join(candidate, "vite.config.ts")) {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func fileExistsAt(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func explorerBundleStale(appDir, distPath string) bool {
	distInfo, err := os.Stat(distPath)
	if err != nil {
		return true
	}
	newest := newestExplorerSourceMtime(appDir)
	return newest.After(distInfo.ModTime())
}

func newestExplorerSourceMtime(appDir string) time.Time {
	var newest time.Time
	consider := func(path string) {
		if info, err := os.Stat(path); err == nil && info.ModTime().After(newest) {
			newest = info.ModTime()
		}
	}
	consider(filepath.Join(appDir, "index.html"))
	consider(filepath.Join(appDir, "vite.config.ts"))
	consider(filepath.Join(appDir, "package.json"))
	_ = filepath.WalkDir(filepath.Join(appDir, "src"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		consider(path)
		return nil
	})
	return newest
}

func buildExplorerBundle(appDir string, w io.Writer) error {
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm not found in PATH\nhint: install Node.js (>=18) or prebuild with: npm run build --prefix explorer-app")
	}
	if !fileExistsAt(filepath.Join(appDir, "node_modules", ".package-lock.json")) {
		fmt.Fprintln(w, "explorer: installing frontend dependencies (npm ci)…")
		ci := exec.Command(npm, "ci")
		ci.Dir = appDir
		ci.Stdout = io.Discard
		ci.Stderr = w
		if err := ci.Run(); err != nil {
			return fmt.Errorf("npm ci: %w", err)
		}
	}
	fmt.Fprintln(w, "explorer: frontend sources changed — rebuilding bundle (npm run build)…")
	build := exec.Command(npm, "run", "build")
	build.Dir = appDir
	build.Stdout = io.Discard
	build.Stderr = w
	if err := build.Run(); err != nil {
		return fmt.Errorf("npm run build: %w", err)
	}
	return nil
}
