# pith in VS Code

VS Code runs pith through **Tasks** — the built-in task runner that can prompt for free text, which means `edit`, `generate`, and `explain` all work properly here.

---

## Setup

**1. Copy `tasks.json` into your project**

```
.vscode/
  tasks.json     ← copy vscode/tasks.json here
```

Either copy it once into a specific project's `.vscode/` folder, or into a global tasks file if you want it everywhere.

**2. Add keybindings (optional)**

Copy the contents of `keybindings.json` into your VS Code keybindings:

- `Ctrl+Shift+P` → **Open Keyboard Shortcuts (JSON)**
- Paste the entries from `keybindings.json`

**3. Point at your binary**

In `tasks.json`, `"command": "pith"` assumes `pith` is on your PATH. If it's not, replace with the full path: `"C:\\path\\to\\pith.exe"`.

Also update the `--agent` strings to match your preferred AI tool.

---

## Keys

| Shortcut | What it does |
|---|---|
| `Ctrl+Alt+O` | Read the current file (clickable output in Problems panel) |
| `Ctrl+Alt+Shift+O` | Read the current folder |
| `Ctrl+Alt+F` | Search — prompts for a query |
| `Ctrl+Alt+S` | AI summary of the current file |
| `Ctrl+Alt+X` | AI explain — uses current line position |
| `Ctrl+Alt+E` | AI edit — prompts for line range and instruction |
| `Ctrl+Alt+W` | Work list |

Or run any task manually: `Ctrl+Shift+P` → **Tasks: Run Task** → pick from the list.

---

## How edit works here

VS Code tasks can prompt for free text (`promptString` inputs), so you type the line range and instruction inline:

1. `Ctrl+Alt+E`
2. VS Code asks: **Line range to edit?** → type `10:30`
3. VS Code asks: **Edit instruction?** → type your prompt
4. pith runs, the file is rewritten

For `generate`, the task prompts for a file path and what to generate.

For `explain`, use `Ctrl+Alt+X` (explain at current line) or run **pith: explain by name** from Tasks to name a specific declaration.

---

## Clickable output

`read` and `search` use the `problemMatcher` pattern so results appear in the **Problems panel** (`Ctrl+Shift+M`) as clickable links that jump to the declaration. The Terminal also shows the raw output.
