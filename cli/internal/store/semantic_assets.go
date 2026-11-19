package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	semanticHFRevision     = "1110a243fdf4706b3f48f1d95db1a4f5529b4d41"
	semanticHFRepo         = "sentence-transformers/all-MiniLM-L6-v2"
	semanticONNXRuntimeVer = "1.26.0"
	semanticHTTPTimeout    = 15 * time.Minute
)

const semanticModelAsset = "c3x-semantic-model-all-MiniLM-L6-v2-" + semanticHFRevision + ".onnx"
const semanticVocabAsset = "c3x-semantic-vocab-all-MiniLM-L6-v2-" + semanticHFRevision + ".txt"

const (
	semanticModelHFPath = "onnx/model.onnx"
	semanticVocabHFPath = "vocab.txt"
	semanticModelSHA256 = "6fd5d72fe4589f189f8ebc006442dbb529bb7ce38f8082112682524616046452"
	semanticVocabSHA256 = "07eced375cec144d27c900241f3e339478dec958f92fddbc551f295c992038a3"
)

type semanticModelSource struct {
	assetName string
	cacheName string
	hfPath    string
	url       string
	sha256    string
}

type semanticAssets struct {
	ModelPath      string
	VocabPath      string
	RuntimeLibPath string
	CacheDir       string
}

type runtimeAsset struct {
	URL      string
	LibName  string
	Platform string
	SHA256   string
}

// SemanticCacheDir returns the cache root used for downloaded semantic assets.
func SemanticCacheDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv("C3_SEMANTIC_CACHE_DIR")); dir != "" {
		return dir, nil
	}
	base := strings.TrimSpace(os.Getenv("XDG_CACHE_HOME"))
	if base == "" {
		var err error
		base, err = os.UserCacheDir()
		if err != nil {
			return "", fmt.Errorf("semantic cache dir: %w", err)
		}
	}
	version := strings.TrimSpace(os.Getenv("C3X_VERSION"))
	if version == "" {
		version = "dev"
	}
	return filepath.Join(base, "c3x", version), nil
}

func ensureSemanticAssets(ctx context.Context, allowDownload bool) (semanticAssets, error) {
	if os.Getenv("C3_SEMANTIC_OFFLINE") != "" {
		allowDownload = false
	}
	cacheDir, err := SemanticCacheDir()
	if err != nil {
		return semanticAssets{}, err
	}
	modelDir := semanticModelCacheDir(cacheDir)
	asset, err := currentRuntimeAsset()
	if err != nil {
		return semanticAssets{}, err
	}
	runtimeDir := semanticRuntimeCacheDir(cacheDir, asset)
	assets := semanticAssets{
		ModelPath:      filepath.Join(modelDir, "model.onnx"),
		VocabPath:      filepath.Join(modelDir, "vocab.txt"),
		RuntimeLibPath: filepath.Join(runtimeDir, asset.LibName),
		CacheDir:       cacheDir,
	}

	if err := ensureSemanticModelFiles(ctx, assets, allowDownload); err != nil {
		return semanticAssets{}, err
	}
	if fileExistsAndNonEmpty(assets.RuntimeLibPath) {
		return assets, nil
	}
	if ok, err := materializeEmbeddedSemanticRuntime(assets.RuntimeLibPath, asset.LibName); err != nil {
		return semanticAssets{}, err
	} else if ok {
		return assets, nil
	}
	if !allowDownload {
		return semanticAssets{}, ErrSemanticUnavailable
	}
	if err := downloadRuntimeLib(ctx, asset, runtimeDir, assets.RuntimeLibPath); err != nil {
		return semanticAssets{}, fmt.Errorf("%w: download onnxruntime: %w", ErrSemanticUnavailable, err)
	}
	return assets, nil
}

func ensureSemanticModelFiles(ctx context.Context, assets semanticAssets, allowDownload bool) error {
	if ok, err := materializeEmbeddedSemanticAssets(assets.ModelPath, assets.VocabPath); err != nil {
		return err
	} else if ok {
		return nil
	}
	sources := semanticModelSources()
	if err := ensureCanonicalSemanticFile(ctx, assets.ModelPath, sources[0], allowDownload); err != nil {
		return err
	}
	return ensureCanonicalSemanticFile(ctx, assets.VocabPath, sources[1], allowDownload)
}

// PrepareEmbeddedSemanticAssets writes verified canonical model and runtime
// files into the go:embed directory used by fat release builds.
func PrepareEmbeddedSemanticAssets(ctx context.Context, embedDir, targetOS, targetArch string) error {
	if err := PrepareEmbeddedSemanticModelAssets(ctx, embedDir); err != nil {
		return err
	}
	asset, err := runtimeAssetFor(targetOS, targetArch)
	if err != nil {
		return err
	}
	cacheDir, err := SemanticCacheDir()
	if err != nil {
		return err
	}
	return prepareEmbeddedSemanticRuntime(ctx, embedDir, asset, semanticModelCacheCandidateDirs(cacheDir))
}

// PrepareEmbeddedSemanticModelAssets writes the verified canonical model files
// into the go:embed directory used by fat release builds.
func PrepareEmbeddedSemanticModelAssets(ctx context.Context, embedDir string) error {
	cacheDir, err := SemanticCacheDir()
	if err != nil {
		return err
	}
	return prepareEmbeddedSemanticModelAssets(ctx, embedDir, semanticModelCacheCandidateDirs(cacheDir), semanticModelSources())
}

// PrepareReleaseSemanticModelAssets writes verified model release assets and
// matching .sha256 files for distribution packaging.
func PrepareReleaseSemanticModelAssets(ctx context.Context, outDir string) error {
	cacheDir, err := SemanticCacheDir()
	if err != nil {
		return err
	}
	return prepareReleaseSemanticModelAssets(ctx, outDir, semanticModelCacheCandidateDirs(cacheDir), semanticModelSources())
}

func semanticModelSources() []semanticModelSource {
	return []semanticModelSource{
		{
			assetName: semanticModelAsset,
			cacheName: "model.onnx",
			hfPath:    semanticModelHFPath,
			sha256:    semanticModelSHA256,
		},
		{
			assetName: semanticVocabAsset,
			cacheName: "vocab.txt",
			hfPath:    semanticVocabHFPath,
			sha256:    semanticVocabSHA256,
		},
	}
}

func semanticModelCacheDir(cacheDir string) string {
	return filepath.Join(cacheDir, "models", "all-MiniLM-L6-v2-"+semanticHFRevision)
}

func semanticRuntimeCacheDir(cacheDir string, asset runtimeAsset) string {
	return filepath.Join(cacheDir, "onnxruntime", semanticONNXRuntimeVer, asset.Platform)
}

func semanticModelSourceURL(source semanticModelSource) string {
	if source.url != "" {
		return source.url
	}
	return "https://huggingface.co/" + semanticHFRepo + "/resolve/" + semanticHFRevision + "/" + source.hfPath
}

func semanticModelCacheCandidateDirs(primary string) []string {
	dirs := []string{primary}
	if legacy, err := legacySemanticCacheDir(); err == nil && legacy != "" && legacy != primary {
		dirs = append(dirs, legacy)
	}
	return dirs
}

func legacySemanticCacheDir() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("legacy semantic cache dir: %w", err)
	}
	return filepath.Join(base, "c3", "semantic"), nil
}

func prepareEmbeddedSemanticModelAssets(ctx context.Context, embedDir string, cacheDirs []string, sources []semanticModelSource) error {
	if err := os.MkdirAll(embedDir, 0755); err != nil {
		return err
	}
	for _, source := range sources {
		cachePath, err := ensureCanonicalSemanticCachedFile(ctx, source, cacheDirs)
		if err != nil {
			return err
		}
		target := filepath.Join(embedDir, source.cacheName)
		if err := copyVerifiedSemanticFile(cachePath, target, source.sha256); err != nil {
			return fmt.Errorf("stage embedded semantic asset %s: %w", source.assetName, err)
		}
	}
	return nil
}

func prepareReleaseSemanticModelAssets(ctx context.Context, outDir string, cacheDirs []string, sources []semanticModelSource) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	for _, source := range sources {
		cachePath, err := ensureCanonicalSemanticCachedFile(ctx, source, cacheDirs)
		if err != nil {
			return err
		}
		target := filepath.Join(outDir, source.assetName)
		if err := copyVerifiedSemanticFile(cachePath, target, source.sha256); err != nil {
			return fmt.Errorf("write release semantic asset %s: %w", source.assetName, err)
		}
		sumPath := target + ".sha256"
		if err := os.WriteFile(sumPath, []byte(source.sha256+"  "+source.assetName+"\n"), 0644); err != nil {
			return fmt.Errorf("write checksum %s: %w", filepath.Base(sumPath), err)
		}
	}
	return nil
}

func prepareEmbeddedSemanticRuntime(ctx context.Context, embedDir string, asset runtimeAsset, cacheDirs []string) error {
	cachePath, err := ensureSemanticRuntimeCachedFile(ctx, asset, cacheDirs)
	if err != nil {
		return err
	}
	target := filepath.Join(embedDir, "runtime", asset.LibName)
	if err := copySemanticFile(cachePath, target); err != nil {
		return fmt.Errorf("stage embedded runtime asset %s: %w", asset.LibName, err)
	}
	return nil
}

func ensureSemanticRuntimeCachedFile(ctx context.Context, asset runtimeAsset, cacheDirs []string) (string, error) {
	if len(cacheDirs) == 0 {
		return "", errors.New("semantic cache candidates: empty")
	}
	primary := filepath.Join(semanticRuntimeCacheDir(cacheDirs[0], asset), asset.LibName)
	for _, cacheDir := range cacheDirs {
		candidate := filepath.Join(semanticRuntimeCacheDir(cacheDir, asset), asset.LibName)
		if !fileExistsAndNonEmpty(candidate) {
			continue
		}
		if candidate != primary {
			if err := copySemanticFile(candidate, primary); err != nil {
				return "", fmt.Errorf("copy runtime asset from legacy cache: %w", err)
			}
		}
		return primary, nil
	}
	if err := downloadRuntimeLib(ctx, asset, filepath.Dir(primary), primary); err != nil {
		return "", err
	}
	return primary, nil
}

func ensureCanonicalSemanticCachedFile(ctx context.Context, source semanticModelSource, cacheDirs []string) (string, error) {
	if len(cacheDirs) == 0 {
		return "", errors.New("semantic cache candidates: empty")
	}
	primary := filepath.Join(semanticModelCacheDir(cacheDirs[0]), source.cacheName)
	for _, cacheDir := range cacheDirs {
		candidate := filepath.Join(semanticModelCacheDir(cacheDir), source.cacheName)
		if err := verifyFileSHA256(candidate, source.sha256); err != nil {
			continue
		}
		if candidate != primary {
			if err := copyVerifiedSemanticFile(candidate, primary, source.sha256); err != nil {
				return "", fmt.Errorf("copy semantic asset from legacy cache: %w", err)
			}
		}
		return primary, nil
	}
	if err := ensureCanonicalSemanticFile(ctx, primary, source, true); err != nil {
		return "", err
	}
	return primary, nil
}

func ensureCanonicalSemanticFile(ctx context.Context, path string, source semanticModelSource, allowDownload bool) error {
	if err := verifyFileSHA256(path, source.sha256); err == nil {
		return nil
	}
	if !allowDownload {
		return ErrSemanticUnavailable
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := downloadURL(ctx, semanticModelSourceURL(source), tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: download semantic asset %s: %w\nhint: connect once to Hugging Face or use the fat C3 build for offline semantic search", ErrSemanticUnavailable, source.assetName, err)
	}
	if err := verifyFileSHA256(tmp, source.sha256); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: verify semantic asset %s: %w", ErrSemanticUnavailable, source.assetName, err)
	}
	return os.Rename(tmp, path)
}

func copyVerifiedSemanticFile(source, target, wantHex string) error {
	if err := verifyFileSHA256(source, wantHex); err != nil {
		return fmt.Errorf("verify source %s: %w", source, err)
	}
	if err := copySemanticFile(source, target); err != nil {
		return err
	}
	if err := verifyFileSHA256(target, wantHex); err != nil {
		return fmt.Errorf("verify copied %s: %w", target, err)
	}
	return nil
}

func copySemanticFile(source, target string) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		return err
	}
	tmp := target + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, target)
}

func currentRuntimeAsset() (runtimeAsset, error) {
	return runtimeAssetFor(runtime.GOOS, runtime.GOARCH)
}

// semanticRuntimeBaseURL hosts the pinned onnxruntime libraries. Microsoft removed
// github.com/microsoft/onnxruntime and its release downloads, so the raw libraries
// are mirrored from Microsoft's NuGet package; scripts/mirror_onnxruntime.sh
// republishes them and prints the SHA256 values pinned below.
const semanticRuntimeBaseURL = "https://github.com/cuongtranba/c3-skill/releases/download/onnxruntime-v" + semanticONNXRuntimeVer + "/"

// semanticRuntimeLibs maps GOOS/GOARCH to the mirrored library. darwin/amd64 is
// absent on purpose: Microsoft's NuGet package ships no osx-x64 build.
var semanticRuntimeLibs = map[string]struct {
	assetName string
	libName   string
	sha256    string
}{
	"linux/amd64": {
		"onnxruntime-" + semanticONNXRuntimeVer + "-linux-amd64.so",
		"libonnxruntime.so." + semanticONNXRuntimeVer,
		"5bd5bedf736fc501692435d0ec4f6e8b2bdf48cd30af8e6d00d61b3ddc9a7ab8",
	},
	"linux/arm64": {
		"onnxruntime-" + semanticONNXRuntimeVer + "-linux-arm64.so",
		"libonnxruntime.so." + semanticONNXRuntimeVer,
		"115ecb838e703d390262b8b4d07d5248e6693c67658d4c98c48f94905ab27af4",
	},
	"darwin/arm64": {
		"onnxruntime-" + semanticONNXRuntimeVer + "-darwin-arm64.dylib",
		"libonnxruntime." + semanticONNXRuntimeVer + ".dylib",
		"cb0462c3fd35ad722e8772313030a33c182f3d4c6b33f4e5e1fcb2ce3199b86c",
	},
	"windows/amd64": {
		"onnxruntime-" + semanticONNXRuntimeVer + "-windows-amd64.dll",
		"onnxruntime.dll",
		"b2ba7ca16e0e4fe71ad5148744ab885a2f5809e52a0c3de4d9ba3853a03977f9",
	},
	"windows/arm64": {
		"onnxruntime-" + semanticONNXRuntimeVer + "-windows-arm64.dll",
		"onnxruntime.dll",
		"01bba4af5089b1951df8b6e6e960e77313103e872519d57304c0ddc3b002a33b",
	},
}

func runtimeAssetFor(goos, goarch string) (runtimeAsset, error) {
	lib, ok := semanticRuntimeLibs[goos+"/"+goarch]
	if !ok {
		return runtimeAsset{}, fmt.Errorf("%w: unsupported platform %s/%s", ErrSemanticUnavailable, goos, goarch)
	}
	return runtimeAsset{
		URL:      semanticRuntimeBaseURL + lib.assetName,
		LibName:  lib.libName,
		Platform: goos + "-" + goarch,
		SHA256:   lib.sha256,
	}, nil
}

// downloadRuntimeLib fetches the library into a temp file beside target, checks
// it against the pinned SHA256, and only then renames it into place, so a bad
// download never leaves a loadable file at target.
func downloadRuntimeLib(ctx context.Context, asset runtimeAsset, dir, target string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "onnxruntime-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	defer os.Remove(tmpPath)

	if err := downloadURL(ctx, asset.URL, tmpPath); err != nil {
		return err
	}
	if err := verifyFileSHA256(tmpPath, asset.SHA256); err != nil {
		return fmt.Errorf("verify onnxruntime %s: checksum mismatch: %w\nhint: the download was corrupted or tampered with; retry, or use the fat C3 build", asset.LibName, err)
	}
	if err := os.Chmod(tmpPath, 0755); err != nil {
		return err
	}
	return os.Rename(tmpPath, target)
}

func downloadURL(ctx context.Context, url, target string) error {
	body, err := downloadBytes(ctx, url)
	if err != nil {
		return err
	}
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	if _, err := out.Write(body); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if !fileExistsAndNonEmpty(target) {
		return fmt.Errorf("download %s: empty file", url)
	}
	return nil
}

func downloadBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: semanticHTTPTimeout}
	res, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("download %s: status %s", url, res.Status)
	}
	return io.ReadAll(res.Body)
}

func verifyFileSHA256(path, wantHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, wantHex) {
		return errors.New("sha256 mismatch")
	}
	return nil
}

func fileExistsAndNonEmpty(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Size() > 0
}
