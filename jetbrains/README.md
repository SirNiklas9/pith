# pith JetBrains Plugin

A native JetBrains plugin that wraps the pith CLI. All seven ops — read, search, explain, summary, edit, generate — work as proper IDE actions with real input dialogs and a tool window with clickable `file:line` output.

pith core doesn't change. The plugin is just a UI shell that calls the binary.

---

## Build

You need JDK 17+ and Gradle.

```
cd jetbrains
./gradlew buildPlugin
```

The `.zip` lands in `build/distributions/`. Install via **Settings ▸ Plugins ▸ ⚙ ▸ Install Plugin from Disk**.

Or run in a sandboxed IDE for development:

```
./gradlew runIde
```

---

## Configure

**Settings ▸ pith**

| Setting | Default | What it does |
|---|---|---|
| pith binary path | `pith` | Full path to pith.exe, or just `pith` if it's on your PATH |
| Default agent command | `claude --dangerously-skip-permissions -p` | The AI backend used for summary, explain, edit, generate |

You can swap the agent for any pith-compatible backend:
- `ollama run llama3` — local, free, no account
- `codex exec --full-auto` — OpenAI Codex

---

## Actions and shortcuts

All actions appear under **pith** in the editor right-click menu.

| Action | Shortcut | Input |
|---|---|---|
| Read File | `Ctrl+Alt+O` | none — reads current file |
| Read Package | `Ctrl+Alt+Shift+O` | none — reads current folder |
| Search | `Ctrl+Alt+F` | dialog (pre-filled with selected text) |
| Explain | `Ctrl+Alt+X` | uses selected text; dialog if nothing selected |
| Summary | `Ctrl+Alt+S` | none — summarizes current file |
| Edit Selection | `Ctrl+Alt+E` | dialog for instruction; applies to selected lines |
| Generate File | `Ctrl+Alt+G` | dialog for file path, then instruction |

Output appears in the **pith** tool window at the bottom of the IDE. `file:line:` references in read and search output are clickable.

For **Edit**, select the lines you want changed first, then hit the shortcut — a dialog asks for the instruction and the edit is applied directly to the file.

For **Explain**, select the declaration name (just the name, not the whole signature) and hit the shortcut.

---

## How it works

The plugin calls the pith binary as a subprocess for every action. It does not embed any AI itself — the AI backend is whatever you configured in Settings. pith core stays unchanged and editor-agnostic.
