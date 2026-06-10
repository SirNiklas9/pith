# pith JetBrains Plugin

A native JetBrains plugin that wraps the pith CLI. All eight ops — read, search, explain, summary, edit, generate, work — work as proper IDE actions with real input dialogs and a tool window with clickable `file:line` output.

pith core doesn't change. The plugin is just a UI shell that calls the binary.

---

## Install

**Prebuilt**: download `pith-jetbrains-<version>.zip` from **[Releases](https://github.com/SirNiklas9/pith/releases)** and install via **Settings ▸ Plugins ▸ ⚙ ▸ Install Plugin from Disk**. Works in any JetBrains IDE on 2024.1+ — Rider, GoLand, IntelliJ, PyCharm, all of them.

**The pith binary is bundled inside the plugin** (all platforms in release builds) and extracted on first use — no PATH setup, nothing else to install. Settings can still point at your own binary if you prefer.

## Build from source

You need JDK 17+, Gradle, and Go (the build compiles the pith binary into the plugin).

```
cd jetbrains
./gradlew clean buildPlugin                          # bundles your platform's binary
./gradlew clean buildPlugin "-PbundlePlatforms=all"  # bundles all six (release builds)
```

The `.zip` lands in `build/distributions/`. Always `clean` — incremental builds can package stale class files.

Or run in a sandboxed IDE for development:

```
./gradlew runIde
```

---

## Configure

**Settings ▸ pith**

| Setting | Default | What it does |
|---|---|---|
| pith binary path | *(empty)* | Empty = the binary bundled with the plugin. Set a path to use your own build instead |
| Default agent command | `claude --dangerously-skip-permissions -p` | The AI backend used for explain, summary, edit, generate |

You can swap the agent for any pith-compatible backend:
- `ollama run llama3` — local, free, no account
- `codex exec --full-auto` — OpenAI Codex

---

## Actions and shortcuts

All actions appear under **pith** in the editor right-click menu.

| Action | Shortcut | What it does |
|---|---|---|
| Read File | `Ctrl+Alt+O` | Purpose map of the current file |
| Read Package | `Ctrl+Alt+Shift+O` | Purpose map of the current folder |
| Search | `Ctrl+Alt+F` | Find declarations by name/doc across the project |
| Explain | `Ctrl+Alt+X` | AI deep-dive on the declaration at cursor |
| Summary | `Ctrl+Alt+S` | AI prose overview of the current file |
| Edit Selection | `Ctrl+Alt+E` | AI edit of the selected lines |
| Generate File | `Ctrl+Alt+G` | AI generates a new file from a description |
| Work List | `Ctrl+Alt+W` | Show the project work-note list |

Output appears in the **pith** tool window at the bottom of the IDE. `file:line:` references in read and search output are clickable.

**Edit**: select the lines you want changed first, then hit the shortcut — a dialog asks for the instruction and the edit is applied directly to the file.

**Explain**: place your cursor on or inside a declaration and hit the shortcut — pith finds the nearest declaration at that line.

---

## How it works

The plugin calls the pith binary as a subprocess for every action. It does not embed any AI itself — the AI backend is whatever you configured in Settings. pith core stays unchanged and editor-agnostic.
