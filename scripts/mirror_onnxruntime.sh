#!/usr/bin/env bash
# Rebuild the onnxruntime mirror that c3x downloads its semantic-search runtime from.
#
# Microsoft removed github.com/microsoft/onnxruntime (and its release downloads)
# on 2026-11-18. The same native libraries ship in Microsoft's NuGet package
# Microsoft.ML.OnnxRuntime. This script downloads that package, verifies it
# against the SHA-512 NuGet publishes for it, extracts one library per platform
# under the names c3x requests, and writes a .sha256 next to each.
#
# The sha256 values printed at the end must match semanticONNXRuntimeAssets in
# cli/internal/store/semantic_assets.go.
#
# Usage:
#   scripts/mirror_onnxruntime.sh [--version 1.26.0] [--out-dir dist/onnxruntime] [--upload]
#
# --upload creates the prerelease onnxruntime-v<version> on this repository (if
# missing) and uploads the assets with gh. A prerelease is invisible to c3x
# runtime resolution and to GitHub's "Latest".
set -euo pipefail

VERSION="1.26.0"
OUT_DIR="dist/onnxruntime"
UPLOAD=false
while [ $# -gt 0 ]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --out-dir) OUT_DIR="$2"; shift 2 ;;
    --upload) UPLOAD=true; shift ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
done

PACKAGE="microsoft.ml.onnxruntime"
NUPKG_URL="https://api.nuget.org/v3-flatcontainer/${PACKAGE}/${VERSION}/${PACKAGE}.${VERSION}.nupkg"
REGISTRATION_URL="https://api.nuget.org/v3/registration5-gz-semver2/${PACKAGE}/${VERSION}.json"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
mkdir -p "$OUT_DIR"

echo "Downloading ${NUPKG_URL}"
curl -fsSL -o "$WORK/package.nupkg" "$NUPKG_URL"

catalog_url="$(curl -fsSL --compressed "$REGISTRATION_URL" | python3 -I -c 'import json,sys; print(json.load(sys.stdin)["catalogEntry"])')"
published="$(curl -fsSL --compressed "$catalog_url" | python3 -I -c 'import json,sys; d=json.load(sys.stdin); assert d["packageHashAlgorithm"] == "SHA512"; print(d["packageHash"])')"
actual="$(python3 -I -c 'import base64,hashlib,sys; print(base64.b64encode(hashlib.sha512(open(sys.argv[1],"rb").read()).digest()).decode())' "$WORK/package.nupkg")"
if [ "$published" != "$actual" ]; then
  echo "error: ${PACKAGE} ${VERSION} SHA-512 does not match the hash NuGet publishes" >&2
  echo "hint: published ${published}, downloaded ${actual}" >&2
  exit 1
fi
echo "Verified NuGet SHA-512 ${actual}"

# member in the nupkg -> mirror asset name
mirror() {
  local member="$1" asset="$2"
  python3 -I -c 'import sys, zipfile; sys.stdout.buffer.write(zipfile.ZipFile(sys.argv[1]).read(sys.argv[2]))' \
    "$WORK/package.nupkg" "$member" > "$OUT_DIR/$asset"
  (cd "$OUT_DIR" && shasum -a 256 "$asset" > "$asset.sha256")
}
mirror "runtimes/linux-x64/native/libonnxruntime.so"     "onnxruntime-${VERSION}-linux-amd64.so"
mirror "runtimes/linux-arm64/native/libonnxruntime.so"   "onnxruntime-${VERSION}-linux-arm64.so"
mirror "runtimes/osx-arm64/native/libonnxruntime.dylib"  "onnxruntime-${VERSION}-darwin-arm64.dylib"
mirror "runtimes/win-x64/native/onnxruntime.dll"         "onnxruntime-${VERSION}-windows-amd64.dll"
mirror "runtimes/win-arm64/native/onnxruntime.dll"       "onnxruntime-${VERSION}-windows-arm64.dll"

echo
cat "$OUT_DIR"/*.sha256

if [ "$UPLOAD" = true ]; then
  TAG="onnxruntime-v${VERSION}"
  if ! gh release view "$TAG" >/dev/null 2>&1; then
    gh release create "$TAG" --prerelease --title "onnxruntime ${VERSION} (mirror)" --notes \
"Native onnxruntime ${VERSION} libraries for c3x semantic search, mirrored because github.com/microsoft/onnxruntime was removed.

Extracted from Microsoft's NuGet package ${PACKAGE} ${VERSION} (${NUPKG_URL}), verified against NuGet's published SHA-512 ${actual}.
Rebuild with scripts/mirror_onnxruntime.sh --version ${VERSION}. Not a c3x release."
  fi
  gh release upload "$TAG" "$OUT_DIR"/* --clobber
fi
