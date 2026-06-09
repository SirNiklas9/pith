# pith.nvim

A floating **purpose map** of the current file — every declaration's purpose, one line each, grouped by type. Jump to any declaration with `<CR>`.

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
      bin = "/path/to/pith/pith",  -- full path to the built binary
    })

    local p = require("pith")
    vim.keymap.set("n", "<leader>po", p.overview, { desc = "pith: overview" })
    vim.keymap.set("n", "<leader>ps", p.summary,  { desc = "pith: summary" })
    vim.keymap.set("v", "<leader>pe", p.edit,     { desc = "pith: edit selection" })
  end,
}
```

Requires Neovim 0.10+ (uses `vim.system` for async).

---

## Keys

| Key | What it does |
|---|---|
| `<leader>po` | Open the purpose map for the current file |
| `<CR>` (inside map) | Jump to that declaration |
| `q` / `<Esc>` | Close |
| `<leader>ps` | AI summary of the current file (needs a backend) |
| `<leader>pe` | AI edit of the visual selection (needs a backend) |

---

## Options

```lua
require("pith").setup({
  bin          = "pith",      -- path to pith binary (or just "pith" if on PATH)
  width        = 0.7,         -- float width (fraction of editor width)
  height       = 0.7,         -- float height (fraction of editor height)
  border       = "rounded",   -- float border style
  backend_args = {},          -- extra args passed to AI ops, e.g. {"--agent", "claude -p"}
  agent        = false,       -- set true if backend_args uses --agent
})
```

---

## AI ops

`summary` and `edit` need a backend. Set it in setup:

```lua
require("pith").setup({
  bin          = "/path/to/pith",
  backend_args = {"--agent", "claude --dangerously-skip-permissions -p"},
  agent        = true,
})
```

Or swap for a local model: `backend_args = {"--cmd", "ollama run llama3"}`.
