# pith VS Code Extension

A real VS Code extension that wraps the pith CLI. The binary is **bundled inside the extension** — install the `.vsix` and everything works, no PATH setup, no config files to copy.

pith core doesn't change. The extension is just a UI shell that calls the binary.

---

## Install

**Prebuilt**: download `pith-vscode-<version>.vsix` from **[Releases](https://github.com/SirNiklas9/pith/releases)**, then in VS Code: `Ctrl+Shift+P` → **Extensions: Install from VSIX…**

**From source** (needs Node + Go):

```
cd vscode
go build -ldflags "-s -w" -o bin/pith-windows-amd64.exe ../cmd/pith   # your platform's name: pith-<goos>-<goarch>[.exe]
npx --yes @vscode/vsce package
```

---

## Keys

| Shortcut | Command |
|---|---|
| `Ctrl+Alt+O` | Read the current file (QuickPick — Enter jumps to the declaration) |
| `Ctrl+Alt+Shift+O` | Read the current folder |
| `Ctrl+Alt+M` | Map the project — one row per package with a cached one-line AI purpose |
| `Ctrl+Alt+F` | Search the workspace — prompts for a query, results jump on Enter |
| `Ctrl+Alt+S` | AI summary of the current file (opens beside the editor) |
| `Ctrl+Alt+X` | AI explain the declaration at the cursor |
| `Ctrl+Alt+E` | AI edit the selection — prompts for an instruction |
| `Ctrl+Alt+G` | Generate a new file from a prompt |
| `Ctrl+Alt+W` | Work list (anchored items jump on Enter) |

All commands are also in the palette under **pith:**, including **Work Add at Cursor**.

---

## Settings

`File ▸ Preferences ▸ Settings ▸ pith` — same knobs as the JetBrains plugin:

- **Binary Path** — leave empty to use the bundled binary (falls back to `pith` on PATH).
- **Backend Mode** — `config` (recommended: pith's own `pith config` store decides), `agent`, or `api`.
- **Agent Command / Api Target / Api Model** — used by the respective modes.
- **Context** — how much surrounding code edit/generate send as reference: `none` (default), `ask` to pick per invocation, or a fixed level. `uses:dir` / `uses:dir:full` send only the declarations your selection references (outlines or full implementations) — the least context with the most meaning; `around`/`file`/`dir`/`project` are the positional levels.

## How edit applies

- **API / config backend**: pith prints the new region (`--raw`), the extension splices it into the buffer. No disk writes, native undo.
- **Agent backend**: the agent edits the file on disk itself (it has latitude — review with git). Your buffer is saved first; VS Code reloads the file automatically.
