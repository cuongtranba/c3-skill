#!/bin/bash
# Build and install the C3 Navigator VS Code extension
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EXT_DIR="$SCRIPT_DIR/../vscode-c3-nav"

if [ ! -d "$EXT_DIR" ]; then
  echo "Error: vscode-c3-nav directory not found at $EXT_DIR"
  exit 1
fi

echo "Building C3 Navigator extension..."
cd "$EXT_DIR"
npm install --ignore-scripts
npm run package

VSIX=$(ls -1 "$EXT_DIR"/c3-nav.vsix 2>/dev/null | head -1)
if [ -z "$VSIX" ]; then
  echo "Error: .vsix file not found after build"
  exit 1
fi

echo "Installing extension..."
code --install-extension "$VSIX" --force

echo "C3 Navigator extension installed successfully."
echo "Reload VS Code to activate it."
