# pith.nvim

Purpose map, AI summary, edit, search, explain, and work-tracker — all ops, in floating windows. Jump to any declaration with `<CR>`.

Works on any language (Go, Python, TypeScript, C#, Rust, C, C++, and 200 more).

---

## Install

No binary setup needed — the plugin finds pith automatically, in this order:

1. `bin` set explicitly in `setup()`
2. a binary built at the repo root (when the plugin runs from a pith checkout)
3. `bin/pith-<os>-<arch>` inside the plugin folder (drop one in by hand if you like)
4. a release binary the plugin **downloads itself** on first use (pinned to the plugin version, stored under `stdpath("data")/pith/`)
5. `pith` on PATH

`:lua print(require("pith").which())` shows what it resolved; `:lua require("pith").install()` forces the download (usable as a lazy.nvim `build` hook).

**Add to your lazy.nvim config**

```lua
{
  dir = "/path/to/pith/nvim",  -- or your plugin manager's GitHub spec
  config = function()
    require("pith").setup({
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
| `<leader>pm` | Map the project — one row per package, cached one-line AI purposes |
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
  bin          = nil,         -- explicit binary path; nil = resolve automatically
  width        = 0.7,         -- float width (fraction of editor width)
  height       = 0.7,         -- float height (fraction of editor height)
  border       = "rounded",   -- float border style
  backend_args = {},          -- extra args for AI ops, e.g. {"--agent", "claude -p"}
  agent        = false,       -- set true when using --agent (edits files directly)
  context      = nil,         -- nil = none, "ask" = pick per edit/generate, or a fixed
                              -- level: "uses:dir" (decls the selection references) /
                              -- "uses:dir:full" (their implementations) /
                              -- "around"|"file"|"dir"|"project"
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
