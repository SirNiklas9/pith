# pith

You open an unfamiliar file. Instead of reading every function to understand what it does, you run `pith read` and get a map — every declaration, one line each, with what it does pulled from its doc comment.

That's the core. The rest of pith (AI edits, search, work notes) builds on top of it.

---

## Install

### Prebuilt (no Go needed)

Grab the latest binary for your platform from **[Releases](https://github.com/SirNiklas9/pith/releases)** — Windows, Linux, macOS (Intel and Apple Silicon). Rename it `pith` (or `pith.exe`), put it on your PATH.

The JetBrains plugin zip is on the same page — install it via **Settings ▸ Plugins ▸ ⚙ ▸ Install Plugin from Disk** in any JetBrains IDE (Rider, GoLand, IntelliJ, PyCharm, 2024.1+).

### From source

You need [Go](https://go.dev/dl/) installed (1.22 or later). Then:

```
git clone https://github.com/SirNiklas9/pith
cd pith
go build -o pith ./cmd/pith
```

On Windows: `go build -o pith.exe ./cmd/pith`

Put the binary somewhere on your PATH, or point your editor at the full path.

---

## What it does

### Read a file

```
pith read main.go
```

Prints every declaration — functions, types, methods — one per line, with what it does:

```
main.go:12: func New(cfg Config) *Server — creates the server and registers routes
main.go:34: func (s *Server) Start() error — listens on the configured address
main.go:61: type Config struct — server configuration: address, timeout, log level
```

Works on any language: Go, Python, TypeScript, C#, Rust, C, C++, and 200 more.

```
pith read ./internal/auth/    # whole folder
pith read main.go NewServer   # one specific declaration
pith read main.go --grep      # output for piping or editor quickfix panels
```

### Search

```
pith search "error handler" .     # search this folder
pith search "error handler" . -r  # include subfolders too
```

Finds declarations whose name, signature, or doc comment matches your query. No index built first — runs against the same map `read` produces.

### Work notes

```
pith work add "this needs error handling"
pith work add "revisit this" --at parse.go:42
pith work
pith work done 1
```

A simple list of notes you leave yourself, optionally pinned to a file and line. Saved at the root of your git repo so it's in the same place from any subfolder. Not committed — it's personal.

### AI ops

These four ops use an AI tool you name. They do nothing without one — you're never billed silently.

```
pith explain main.go Server --cmd "ollama run llama3"
pith explain main.go:34 --cmd "ollama run llama3"
pith summary main.go --cmd "ollama run llama3"
pith edit parse.go --range 10:30 --prompt "add error handling" --cmd "ollama run llama3"
pith generate services/cache.go --prompt "an LRU cache" --apply --cmd "ollama run llama3"
```

**explain** points the AI at a single declaration's full source. It tells you what it does, why it exists, and what callers need to know — more depth than `read`, narrower than `summary`. Pass a name or a `file:line` position.

**summary** gives a prose overview of an entire file or folder.

**edit** shows a diff by default. Add `--apply` to write it. Add `--raw` for just the new code.

**generate** creates a new file. It won't overwrite one that already has content.

#### Picking an AI tool

| What you want | How to pass it |
|---|---|
| A local AI (no account needed) | `--cmd "ollama run llama3"` |
| Claude Code | `--agent "claude --dangerously-skip-permissions -p"` |
| OpenAI | `--api openai --model gpt-4o` (needs OPENAI_API_KEY) |
| Any OpenAI-compatible API | `--api https://your-url --model name` (needs PITH_API_KEY) |
| OpenRouter (access many models) | `--api openrouter --model ...` (needs OPENROUTER_API_KEY) |

Agent mode (`--agent`) lets the AI edit files directly in its own way. Completion mode (`--api`, `--cmd`) has pith splice the result in surgically.

#### Optional context

By default, only your selected lines and prompt are sent to the AI. If you want it to see more:

```
--context around    # the rest of the file's structure (not full source)
--context file      # the full source of the file
--context dir       # the structure of the folder
--context project   # the structure of the whole project
```

---

## Editor setup

pith is a plain binary — any editor can run it as an external tool. Pick yours:

- **[Neovim](nvim/README.md)** — floating windows for all ops, jump-to-declaration, visual edit
- **[JetBrains](jetbrains/README.md)** — native plugin for any JetBrains IDE; all ops, real input dialogs, clickable tool window
- **[Visual Studio](visualstudio/README.md)** — External Tools for read/search/summary/explain; edit/generate via the integrated terminal
- **[VS Code](vscode/README.md)** — tasks with free-text prompts, keyboard shortcuts

---

## Prior art

- `symbex` does what `pith read` does, for Python only
- `go doc` does it for Go, exported symbols only
- `universal-ctags` lists declaration locations but strips doc comments
- `aider` / `opencode` / `codex` are pure AI pipelines with no read op
- ThePrimeagen's `99` is the closest in spirit — deterministic work + AI ops — but it's a Neovim plugin
