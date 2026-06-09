# pith.nvim

Purpose map, AI summary, edit, search, explain, and work-tracker — all ops, in floating windows. Jump to any declaration with `<CR>`.

Works on any language (Go, Python, TypeScript, C#, Rust, C, C++, and 200 more).

---

## Install

**1. Build the binary**

```
cd /path/to/pith
go build -o pith ./cmd/pith        # Linux / macOS
go build -o pith.exe ./cmd/pith    # Windows
```

**2. Add to your lazy.nvim config**

```lua
{
  dir = "/path/to/pith/nvim",
  config = function()
    require("pith").setup({
      bin          = "/path/to/pith",
      backend_args = { "--agent", "claude --dangerously-skip-permissions -p" },
      agent        = true,
    })

    local p = require("pith")
    vim.keymap.set("n",  "<leader>po", p.overview,   { desc = "pith: overview" })
    vim.keymap.set("n",  "<leader>pf", p.search,     { desc = "pith: search" })
    vim.keymap.set("n",  "<leader>ps", p.summary,    { desc = "pith: summary" })
    vim.keymap.set("n",  "<leader>px", p.explain,    { desc = "pith: explain at cursor" })
    vim.keymap.set("v",  "<leader>pe", p.edit,       { desc = "pith: edit selection" })
    vim.keymap.set("n",  "<leader>pg", p.generate,   { desc = "pith: generate file" })
    vim.keymap.set("n",  "<leader>pw", p.work_list,  { desc = "pith: work list" })
    vim.keymap.set("n",  "<leader>pa", p.work_add,   { desc = "pith: work add at cursor" })
  end,
}
```

Requires Neovim 0.10+ (uses `vim.system` for async).

---

## Keys (default mapping)

| Key | What it does |
|---|---|
| `<leader>po` | Purpose map of the current file (deterministic, no AI) |
| `<CR>` (in float) | Jump to that declaration or search result |
| `q` / `<Esc>` | Close the float |
| `<leader>pf` | Search declarations by name across the project |
| `<leader>ps` | AI summary of the current file |
| `<leader>px` | AI explanation of the declaration at cursor line |
| `<leader>pe` | AI edit of the visual selection |
| `<leader>pg` | AI generate a new file from a description |
| `<leader>pw` | Show the work-tracker list |
| `<leader>pa` | Add a work note anchored to the current file:line |

---

## Options

```lua
require("pith").setup({
  bin          = "pith",      -- path to pith binary (or just "pith" if on PATH)
  width        = 0.7,         -- float width (fraction of editor width)
  height       = 0.7,         -- float height (fraction of editor height)
  border       = "rounded",   -- float border style
  backend_args = {},          -- extra args for AI ops, e.g. {"--agent", "claude -p"}
  agent        = false,       -- set true when using --agent (edits files directly)
})
```

---

## AI ops

`summary`, `explain`, `edit`, and `generate` need a backend. Pass one in setup:

```lua
-- agent mode (recommended): the LLM edits files itself
require("pith").setup({
  backend_args = { "--agent", "claude --dangerously-skip-permissions -p" },
  agent        = true,
})

-- or a local model via completion API
require("pith").setup({
  backend_args = { "--cmd", "ollama run llama3" },
  agent        = false,
})
```

AI ops refuse silently if `backend_args` is empty — nothing is ever billed without an explicit backend.
