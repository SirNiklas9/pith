-- pith.nvim — the one thing your outline plugin doesn't give you:
-- every declaration's *purpose*, at a glance, grouped by type.
--
-- It shells out to the `pith` CLI (`pith read <file>`) and shows the
-- result in a floating window. <CR> jumps to a declaration, q/<Esc> closes.
-- It deliberately does NOT do outline/search/navigation — use aerial,
-- Telescope, and LSP for those. This is purpose-overview only.
--
-- Setup (lazy.nvim, local dir):
--   {
--     dir = "C:/Users/Nicholas/GolandProjects/pith/nvim",
--     config = function()
--       require("pith").setup({ bin = "C:/Users/Nicholas/GolandProjects/pith/pith.exe" })
--       vim.keymap.set("n", "<leader>po", require("pith").overview,
--         { desc = "pith: purpose overview" })
--     end,
--   }

local M = {}

local config = {
  bin = "pith", -- path to the pith executable (set to the full .exe path on Windows)
  backend_args = {}, -- args appended to pith for AI ops, e.g. { "--agent", "claude --dangerously-skip-permissions -p" }
  agent = false, -- true if the backend is an agent that edits files itself (edit reloads instead of splicing)
  width = 0.7, -- float width  (fraction of editor columns)
  height = 0.7, -- float height (fraction of editor lines)
  border = "rounded",
}

function M.setup(opts)
  config = vim.tbl_deep_extend("force", config, opts or {})
end

-- run `pith read <file>` and return its output lines, or nil + error text.
local function run(file)
  local out = vim.fn.systemlist({ config.bin, "read", file })
  if vim.v.shell_error ~= 0 then
    return nil, table.concat(out, "\n")
  end
  return out
end

-- open a read-only float with `lines`; <CR> jumps into `src_file` at the
-- line number that begins the row, q/<Esc> close.
local function open_float(lines, src_file, wrapText)
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].modifiable = false
  vim.bo[buf].buftype = "nofile"
  vim.bo[buf].bufhidden = "wipe"
  vim.bo[buf].filetype = "pith"

  local cols, rows = vim.o.columns, vim.o.lines
  local w = math.floor(cols * config.width)
  local h = math.floor(rows * config.height)
  local win = vim.api.nvim_open_win(buf, true, {
    relative = "editor",
    width = w,
    height = h,
    row = math.floor((rows - h) / 2),
    col = math.floor((cols - w) / 2),
    style = "minimal",
    border = config.border,
    title = " pith ",
    title_pos = "center",
  })
  vim.wo[win].cursorline = true
  vim.wo[win].wrap = wrapText == true -- prose (summary) wraps; the column map (overview) doesn't
  vim.wo[win].linebreak = wrapText == true

  local function close()
    if vim.api.nvim_win_is_valid(win) then
      vim.api.nvim_win_close(win, true)
    end
  end

  local function jump()
    local n = vim.api.nvim_get_current_line():match("^%s*(%d+)")
    if not n then -- header / blank row — nothing to jump to
      return
    end
    close()
    vim.schedule(function()
      if vim.api.nvim_buf_get_name(0) ~= src_file then
        pcall(vim.cmd, "edit " .. vim.fn.fnameescape(src_file))
      end
      pcall(vim.api.nvim_win_set_cursor, 0, { tonumber(n), 0 })
      vim.cmd("normal! zz")
    end)
  end

  local kopts = { buffer = buf, nowait = true, silent = true }
  vim.keymap.set("n", "q", close, kopts)
  vim.keymap.set("n", "<Esc>", close, kopts)
  vim.keymap.set("n", "<CR>", jump, kopts)
end

-- overview shows the purpose-map of the current file's declarations.
function M.overview()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then
    vim.notify("pith: current buffer has no file", vim.log.levels.WARN)
    return
  end
  local lines, err = run(file)
  if not lines then
    vim.notify("pith: " .. (err ~= "" and err or "failed to run"), vim.log.levels.ERROR)
    return
  end
  open_float(lines, file, false) -- column map: no wrap
end

-- run pith asynchronously (for the LLM-backed ops, which take a few seconds).
-- Requires Neovim 0.10+ (vim.system).
local function run_async(args, on_done)
  local cmd = vim.list_extend({ config.bin }, args)
  vim.system(cmd, { text = true }, function(res)
    vim.schedule(function()
      if res.code ~= 0 then
        on_done(nil, (res.stderr ~= "" and res.stderr) or res.stdout or "failed")
      else
        on_done(res.stdout or "", nil)
      end
    end)
  end)
end

local function need_backend()
  if not config.backend_args or #config.backend_args == 0 then
    vim.notify(
      "pith: set backend_args, e.g. { '--agent', 'claude --dangerously-skip-permissions -p' }",
      vim.log.levels.WARN
    )
    return false
  end
  return true
end

-- summary shows the AI gestalt of the current file in a float.
function M.summary()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then
    vim.notify("pith: current buffer has no file", vim.log.levels.WARN)
    return
  end
  if not need_backend() then
    return
  end
  vim.notify("pith: summarizing…", vim.log.levels.INFO)
  run_async(vim.list_extend({ "summary", file }, config.backend_args), function(out, err)
    if not out then
      vim.notify("pith: " .. err, vim.log.levels.ERROR)
      return
    end
    open_float(vim.split(vim.trim(out), "\n"), nil, true) -- prose: wrap
  end)
end

-- edit runs AI generation on the visual selection and replaces it with the
-- result (undo with u). Asks for the instruction via vim.ui.input.
function M.edit()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then
    vim.notify("pith: current buffer has no file", vim.log.levels.WARN)
    return
  end
  if not need_backend() then
    return
  end
  -- capture the LIVE visual selection (robust under lazy-load), then leave visual mode
  local p1, p2 = vim.fn.getpos("v")[2], vim.fn.getpos(".")[2]
  vim.api.nvim_feedkeys(vim.api.nvim_replace_termcodes("<Esc>", true, false, true), "n", false)
  local s, e = math.min(p1, p2), math.max(p1, p2)
  if s < 1 then
    vim.notify("pith: make a visual selection first", vim.log.levels.WARN)
    return
  end
  vim.ui.input({ prompt = "pith edit: " }, function(instruction)
    if not instruction or instruction == "" then
      return
    end
    vim.notify("pith: editing…", vim.log.levels.INFO)
    -- AGENT edits the file on DISK, so save first — otherwise it works from a
    -- stale version and unsaved lines (a function you just typed) get lost.
    if config.agent and vim.bo.modified then
      vim.cmd("silent noautocmd write")
    end
    local args = { "edit", file, "--range", s .. ":" .. e, "--prompt", instruction }
    if not config.agent then
      table.insert(args, "--raw") -- completion backend: return the region for us to splice
    end
    vim.list_extend(args, config.backend_args)
    run_async(args, function(out, err)
      if not out then
        vim.notify("pith: " .. (err or "failed"), vim.log.levels.ERROR)
        return
      end
      if config.agent then
        -- load the agent's result back as ONE undoable buffer change (a plain
        -- :e would wipe undo history; this keeps `u` working)
        local ok, content = pcall(vim.fn.readfile, file)
        if ok then
          vim.api.nvim_buf_set_lines(0, 0, -1, false, content)
          vim.notify("pith: agent edited the file (u to undo · review with git)", vim.log.levels.INFO)
        else
          vim.cmd("checktime")
          vim.notify("pith: agent edited the file (:e! to reload)", vim.log.levels.WARN)
        end
      else
        local t = vim.trim(out)
        vim.api.nvim_buf_set_lines(0, s - 1, e, false, (t == "") and {} or vim.split(t, "\n"))
        vim.notify("pith: applied (u to undo)", vim.log.levels.INFO)
      end
    end)
  end)
end

return M
