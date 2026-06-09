# pith.nvim

A floating **purpose map** of the current file — every declaration's *purpose*,
at a glance, grouped by type. The one thing your outline plugin (aerial / LSP
`documentSymbol`) doesn't give you: names you already get, *purposes inline* you
don't.

It is a thin wrapper over the [`pith`](../) CLI. It does **not** do
outline / search / navigation — use aerial, Telescope, and LSP for those. This
is purpose-overview only.

## Install (lazy.nvim, local dir)

```lua
{
  dir = "C:/Users/Nicholas/GolandProjects/pith/nvim",
  config = function()
    require("pith").setup({
      -- full path to the built binary (or just "pith" if it's on your PATH)
      bin = "C:/Users/Nicholas/GolandProjects/pith/pith.exe",
    })
    vim.keymap.set("n", "<leader>po", require("pith").overview,
      { desc = "pith: purpose overview" })
  end,
}
```

Build the binary first:

```
go build -C C:/Users/Nicholas/GolandProjects/pith -o pith.exe .
```

## Use

- `<leader>po` — open the purpose map of the current Go file.
- `<CR>` on a row — jump to that declaration.
- `q` / `<Esc>` — close.

(Today the CLI parses Go; other languages come later. Non-Go buffers will
report a parse error.)

## Options

| key      | default     | meaning                                       |
| -------- | ----------- | --------------------------------------------- |
| `bin`    | `"pith"`    | path to the pith executable                   |
| `width`  | `0.7`       | float width as a fraction of editor columns   |
| `height` | `0.7`       | float height as a fraction of editor lines    |
| `border` | `"rounded"` | float border style                            |
