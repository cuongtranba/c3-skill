---
description: Install the C3 Navigator VS Code extension for architecture doc navigation
---

Run the install script to build and install the C3 Navigator VS Code extension:

```bash
bash "$(dirname "$(find ~/.claude -path '*/c3-skill/scripts/install-vscode-ext.sh' 2>/dev/null | head -1)")/install-vscode-ext.sh"
```

After installation, reload VS Code. The extension activates in any workspace with a `.c3/code-map.yaml` file and provides:
- **CodeLens**: Clickable links above `c3-XXX` and `ref-XXX` IDs in `.c3/*.yaml` files
- **Ctrl+Click**: Navigate directly to the architecture document
- **Hover**: Preview document title, goal, and summary
