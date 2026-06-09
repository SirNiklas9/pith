# pith in Visual Studio (C#/.NET)

Visual Studio has **External Tools** (Tools ▸ External Tools…), but unlike JetBrains they live in the registry, so there's no file to drop in — you add them by hand once. Three things to know up front:

- Use the **`--vs`** output flag so `file(line):` is **double-click navigable** in the Output window (VS doesn't recognize `file:line:`).
- `$(CurText)` is the **selected text** — that's your highlight-select.
- External Tools are **output-only**: they can't replace a selection. So `read`, `search`, `summary`, `explain` work great here; `edit`/`generate` go through the terminal (bottom of this file).

## Add the tools

**Tools ▸ External Tools… ▸ Add**, and for each, check **Use Output window**:

| Title | Command | Arguments | Initial directory |
|---|---|---|---|
| pith: read | `C:\tools\pith\pith.exe` | `read "$(ItemPath)" --vs` | `$(ItemDir)` |
| pith: read folder | `C:\tools\pith\pith.exe` | `read "$(ItemDir)" --vs` | `$(ItemDir)` |
| pith: search selection | `C:\tools\pith\pith.exe` | `search "$(CurText)" "$(SolutionDir)" -r --vs` | `$(SolutionDir)` |
| pith: explain at line | `C:\tools\pith\pith.exe` | `explain "$(ItemPath):$(CurLine)" --agent "claude --dangerously-skip-permissions -p"` | `$(ItemDir)` |
| pith: summary | `C:\tools\pith\pith.exe` | `summary "$(ItemPath)" --agent "claude --dangerously-skip-permissions -p"` | `$(ItemDir)` |

(Point Command at wherever your `pith.exe` lives.)

Double-click a line in the Output window to jump to it.

## Configurable shortcuts

VS binds external tools by their **position** in the list. The first tool is `Tools.ExternalCommand1`, the second `Tools.ExternalCommand2`, and so on.

**Tools ▸ Options ▸ Environment ▸ Keyboard**, type `Tools.ExternalCommand` in "Show commands containing", pick the number matching your tool's order, set the shortcut, **Assign**. Suggested:

| Tool (list order) | Command | Suggested |
|---|---|---|
| 1 pith: read | `Tools.ExternalCommand1` | `Ctrl+Alt+O` |
| 2 pith: read folder | `Tools.ExternalCommand2` | `Ctrl+Alt+Shift+O` |
| 3 pith: search selection | `Tools.ExternalCommand3` | `Ctrl+Alt+F` |
| 4 pith: explain at line | `Tools.ExternalCommand4` | `Ctrl+Alt+X` |
| 5 pith: summary | `Tools.ExternalCommand5` | `Ctrl+Alt+S` |

Keep the tool order stable, or the slot numbers shift.

## Work notes

Work notes are best run from the integrated terminal — **View ▸ Terminal** (`Ctrl+`` `):

```
pith work                              # list all notes
pith work add "needs error handling"   # add a note
pith work add "revisit" --at Foo.cs:42 # pin to a line
pith work done 1                       # close note #1
```

## edit / generate (highlight-select that *changes* code)

VS External Tools can't pass a selection's line range or write back into the buffer, so use the integrated terminal:

```
pith edit Foo.cs --range 12:20 --prompt "return Result instead of throwing" --agent "claude --dangerously-skip-permissions -p"
pith edit Foo.cs --range 12:20 --prompt "..." --context file --agent "..."
pith generate Services/Cache.cs --prompt "an LRU cache" --apply --context dir --agent "..."
```

The agent edits the file on disk; VS notices and reloads it. `--context` (off by default) adds `file`/`dir`/`project` outline when you want the AI to see more — omit it and only your selection/prompt is sent.
