# pith in Rider (work machine, C#/.NET)

Since the tree-sitter rewrite, pith reads **C# (and any language)**, so on Rider
you get `read`, `search`, and `summary` — not just `edit`. They install as
JetBrains **External Tools** (same mechanism as GoLand) with one-key bindings
and clickable `file:line` output.

## 1. Get the binary onto the work machine

`pith.exe` is ~37 MB and not committed. Two options:

- **Build it** (needs Go): `go build -o pith.exe ./cmd/pith` from the pith repo.
- **Copy** an existing `pith.exe` over.

Put it at `C:\tools\pith\pith.exe` (the path baked into `pith.xml`), or drop it
anywhere and update the `COMMAND` path in each tool.

## 2. Install the tools

1. **Close Rider.**
2. Copy `rider\pith.xml` into your Rider config `tools` folder:
   `%APPDATA%\JetBrains\Rider<version>\tools\pith.xml`
   (find the exact folder via **Help ▸ Show Config Folder**; create `tools\` if absent).
3. Reopen Rider. The tools appear under **Tools ▸ External Tools ▸ pith** and in
   **Settings ▸ Tools ▸ External Tools**.

> Gotcha that silently breaks the drop-in: an XML comment may not contain `--`,
> and Rider only reads `tools/` at startup. `pith.xml` is comment-free for that
> reason — keep it that way, and install while Rider is closed.

## 3. Shortcuts

**Settings ▸ Keymap**, search "pith", right-click each ▸ **Add Keyboard Shortcut**:

| Tool                 | Suggested      | Works on C#? |
|----------------------|----------------|--------------|
| pith: read           | `Ctrl+Alt+O`   | ✅ read the open file |
| pith: read package   | `Ctrl+Alt+Shift+O` | ✅ the folder |
| pith: search         | `Ctrl+Alt+F`   | ✅ searches the **selected text** project-wide |
| pith: summary        | `Ctrl+Alt+S`   | ✅ (needs AI backend) |
| pith: edit: add doc  | `Ctrl+Alt+E`   | ✅ (needs AI backend) |

`read`/`search` are free, offline, deterministic. `summary`/`edit` need a
backend — edit the `--agent`/`--api`/`--cmd` string in `pith.xml` to taste.

For a one-off `edit`/`generate` instruction (External Tools can't prompt for
free text), use the Rider **terminal** (`Alt+F12`):
`pith edit Foo.cs --range 12:20 --prompt "..." --agent "..."`.

## Loading bar / popups / animation?

Honest answer:

- **You get free "it's working" feedback.** While an External Tool runs, Rider
  shows an **indeterminate progress indicator in the status bar** ("Running
  'pith: summary'…") that you can cancel. The Run tool window also opens and
  streams output. That covers the loading case for the AI ops.
- **No custom progress *bar* or completion *balloon*** comes from External
  Tools — they only stream stdout to the console.
- A real spinner / balloon notification / popup would require a **Rider plugin**
  (IntelliJ Platform SDK, Kotlin) — a separate project, and JetBrains-locked,
  which cuts against pith's no-vendor-lock grain. Not worth it just for chrome.
- If you want notification-style "popups", the **nvim** plugin already does them
  natively (`vim.notify`, async) with zero lock-in — that's the lightweight path.

So: status-bar progress yes (free), fancy animated bar/balloon only via a
dedicated plugin (not recommended).
