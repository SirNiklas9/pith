# pith in GoLand (and any JetBrains IDE)

GoLand has no plugin API as light as nvim's `vim.system`, so pith plugs in two
ways. The deterministic op (`read`) and `summary` live as **External Tools**
with one-key bindings and clickable output. The free-text ops (`edit`,
`generate` with an ad-hoc instruction) live in the **integrated terminal**,
because JetBrains External Tools cannot prompt for free text — every tool's
arguments are fixed when you define it.

## 1. Install the tools

**Option A — drop the file in (fastest).** Close GoLand, copy `pith.xml` to your
GoLand config `tools` folder, reopen.

- Find the config folder: GoLand → **Help ▸ Show Config Folder** (it opens
  `…\JetBrains\GoLand<version>\` in Explorer).
- Put `pith.xml` in the `tools\` subfolder there (create `tools\` if missing).
- Reopen GoLand. The tools appear under **Settings ▸ Tools ▸ External Tools**
  in a group named **pith**.

**Option B — type them in.** Settings ▸ Tools ▸ External Tools ▸ **+**, and copy
the values from `pith.xml` (Program = the full `pith.exe` path, Arguments =
the `PARAMETERS` string, Working dir = `$ProjectFileDir$`).

Edit the `pith.exe` path and the `--agent "claude …"` backend in `pith.xml` to
match your machine before installing. Swap the backend for `--api`/`--cmd` if
you don't want the Claude Code agent.

## 2. Bind keys

Settings ▸ **Keymap**, search "pith", right-click each tool ▸ **Add Keyboard
Shortcut**. Suggested, to echo the nvim `<leader>p*` set:

| Tool                | Suggested shortcut | What it does                                |
|---------------------|--------------------|---------------------------------------------|
| pith: read          | `Ctrl+Alt+O`       | purpose overview of the current file        |
| pith: read package  | `Ctrl+Alt+Shift+O` | overview of the whole package               |
| pith: summary       | `Ctrl+Alt+S`       | AI gestalt of the current file              |

## 3. Use it

- **read** runs on the open file and prints `file:line: sig — purpose` to the
  Run console. The output filter makes every `file:line` **clickable** — click
  to jump to the declaration. This is the daily driver and needs no AI.
- **summary** streams a 2–4 sentence gestalt to the console (needs a backend).
- **edit (fixed prompt)** — select lines, run a baked-in tool like
  *edit: add godoc*. The agent edits the file on disk and `synchronizeAfterRun`
  reloads it. Review with Git (the agent has latitude). Make one tool per
  recurring transform; you can't type a one-off instruction here.

## 4. generate / edit with a one-off instruction → terminal

For an instruction you type once, use the GoLand terminal (**Alt+F12**):

```
# generate a new file from a prompt (creates it, GoLand auto-detects it):
pith generate internal/widget/widget.go --prompt "a Widget type with New and Render" --apply --agent "claude --dangerously-skip-permissions -p"

# or: make the empty file in the Project tree first, then fill it:
pith generate widget.go --prompt "..." --apply --agent "..."

# edit a range you can read off the gutter:
pith edit widget.go --range 10:24 --prompt "return an error instead of panicking" --agent "claude --dangerously-skip-permissions -p"
```

`generate` refuses a file that already has content (use `edit` for those), but
it will fill an **empty** file — so the "New File ▸ run generate" flow works.
Without `--apply` it prints the proposed file to the terminal for review.

## Why the split

`read` is deterministic, free, offline — it belongs on a hotkey. The AI ops are
where JetBrains' fixed-argument External Tools fall short for free text, and the
terminal is the honest home for "type an instruction, run once." If a future
GoLand build exposes an input/prompt macro, the `edit`/`generate` tools can move
onto hotkeys too — we found Rider's did not.
