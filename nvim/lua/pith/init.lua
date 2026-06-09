-- pith.nvim — purpose-map, AI summary, edit, explain, search, work-tracker.
--
-- Setup (lazy.nvim, local dir):
--   {
--     dir = "path/to/pith/nvim",
--     config = function()
--       require("pith").setup({
--         bin          = "/path/to/pith",          -- or just "pith" if on $PATH
--         backend_args = { "--agent", "claude --dangerously-skip-permissions -p" },
--         agent        = true,
--       })
--       local p = require("pith")
--       vim.keymap.set("n",          "<leader>po", p.overview,   { desc = "pith: overview" })
--       vim.keymap.set("n",          "<leader>ps", p.summary,    { desc = "pith: summary" })
--       vim.keymap.set("n",          "<leader>pf", p.search,     { desc = "pith: search" })
--       vim.keymap.set("v",          "<leader>pe", p.edit,       { desc = "pith: edit selection" })
--       vim.keymap.set("n",          "<leader>px", p.explain,    { desc = "pith: explain at cursor" })
--       vim.keymap.set("n",          "<leader>pg", p.generate,   { desc = "pith: generate file" })
--       vim.keymap.set("n",          "<leader>pw", p.work_list,  { desc = "pith: work list" })
--       vim.keymap.set("n",          "<leader>pa", p.work_add,   { desc = "pith: work add" })
--     end,
--   }

local M = {}

local config = {
  bin          = "pith",
  backend_args = {},    -- { "--agent", "claude --dangerously-skip-permissions -p" }
  agent        = false, -- true when backend edits files itself (edit reloads buffer)
  width        = 0.7,
  height       = 0.7,
  border       = "rounded",
}

function M.setup(opts)
  config = vim.tbl_deep_extend("force", config, opts or {})
end

-- ─── helpers ────────────────────────────────────────────────────────────────

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

-- synchronous run, returns lines-list or nil+err
local function run_sync(args)
  local cmd = vim.list_extend({ config.bin }, args)
  local out  = vim.fn.systemlist(cmd)
  if vim.v.shell_error ~= 0 then
    return nil, table.concat(out, "\n")
  end
  return out
end

-- asynchronous run (Neovim 0.10+), calls on_done(stdout|nil, err|nil)
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

-- ─── floating window ────────────────────────────────────────────────────────

-- Standard float: <CR> jumps to the line number at the start of the row.
-- Used for overview (lines start with a number), prose floats pass src_file=nil.
local function open_float(lines, src_file, wrap_text)
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].modifiable = false
  vim.bo[buf].buftype    = "nofile"
  vim.bo[buf].bufhidden  = "wipe"
  vim.bo[buf].filetype   = "pith"

  local cols, rows = vim.o.columns, vim.o.lines
  local w   = math.floor(cols * config.width)
  local h   = math.floor(rows * config.height)
  local win = vim.api.nvim_open_win(buf, true, {
    relative  = "editor",
    width     = w,
    height    = h,
    row       = math.floor((rows - h) / 2),
    col       = math.floor((cols - w) / 2),
    style     = "minimal",
    border    = config.border,
    title     = " pith ",
    title_pos = "center",
  })
  vim.wo[win].cursorline = true
  vim.wo[win].wrap       = wrap_text == true
  vim.wo[win].linebreak  = wrap_text == true

  local function close()
    if vim.api.nvim_win_is_valid(win) then
      vim.api.nvim_win_close(win, true)
    end
  end

  local function jump_to(file, lnum)
    close()
    vim.schedule(function()
      if vim.api.nvim_buf_get_name(0) ~= file then
        pcall(vim.cmd, "edit " .. vim.fn.fnameescape(file))
      end
      pcall(vim.api.nvim_win_set_cursor, 0, { lnum, 0 })
      vim.cmd("normal! zz")
    end)
  end

  local function jump()
    local line = vim.api.nvim_get_current_line()
    -- Overview format: lines start with a plain number (the source line)
    local n = line:match("^%s*(%d+)")
    if n and src_file then
      jump_to(src_file, tonumber(n))
      return
    end
    -- Grep format: path:line:... (search results)
    local f, l = line:match("^([^:]+):(%d+):")
    if f and l then
      jump_to(f, tonumber(l))
    end
  end

  local kopts = { buffer = buf, nowait = true, silent = true }
  vim.keymap.set("n", "q",     close, kopts)
  vim.keymap.set("n", "<Esc>", close, kopts)
  vim.keymap.set("n", "<CR>",  jump,  kopts)
end

-- ─── ops ────────────────────────────────────────────────────────────────────

-- overview: purpose-map of the current file (deterministic, no AI).
function M.overview()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then
    vim.notify("pith: current buffer has no file", vim.log.levels.WARN)
    return
  end
  local lines, err = run_sync({ "read", file })
  if not lines then
    vim.notify("pith: " .. (err ~= "" and err or "failed"), vim.log.levels.ERROR)
    return
  end
  open_float(lines, file, false)
end

-- search: find declarations by name pattern across the project (deterministic).
-- <CR> on a result line jumps to that file:line.
function M.search(query)
  if not query or query == "" then
    vim.ui.input({ prompt = "pith search: " }, function(q)
      if q and q ~= "" then M.search(q) end
    end)
    return
  end
  local cwd   = vim.fn.getcwd()
  local lines, err = run_sync({ "search", query, cwd, "-r" })
  if not lines then
    vim.notify("pith: no matches for \"" .. query .. "\"", vim.log.levels.INFO)
    return
  end
  open_float(lines, nil, false) -- grep format: jump parses file:line from the row
end

-- summary: AI gestalt of the current file, shown in a wrapped prose float.
function M.summary()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then vim.notify("pith: current buffer has no file", vim.log.levels.WARN); return end
  if not need_backend() then return end
  vim.notify("pith: summarizing…", vim.log.levels.INFO)
  local args = vim.list_extend({ "summary", file }, config.backend_args)
  run_async(args, function(out, err)
    if not out then vim.notify("pith: " .. err, vim.log.levels.ERROR); return end
    open_float(vim.split(vim.trim(out), "\n"), nil, true)
  end)
end

-- explain: AI deep explanation of the declaration at cursor line (or by name).
-- Call as M.explain() for cursor-line, M.explain("MyFunc") for a named decl.
function M.explain(name)
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then vim.notify("pith: current buffer has no file", vim.log.levels.WARN); return end
  if not need_backend() then return end
  local args
  if name and name ~= "" then
    args = { "explain", file, name }
  else
    local lnum = vim.api.nvim_win_get_cursor(0)[1]
    args = { "explain", file .. ":" .. lnum }
  end
  vim.list_extend(args, config.backend_args)
  vim.notify("pith: explaining…", vim.log.levels.INFO)
  run_async(args, function(out, err)
    if not out then vim.notify("pith: " .. err, vim.log.levels.ERROR); return end
    open_float(vim.split(vim.trim(out), "\n"), nil, true)
  end)
end

-- edit: AI edit of the visual selection (agent rewrites the file; u undoes it).
function M.edit()
  local file = vim.api.nvim_buf_get_name(0)
  if file == "" then vim.notify("pith: current buffer has no file", vim.log.levels.WARN); return end
  if not need_backend() then return end
  local p1, p2 = vim.fn.getpos("v")[2], vim.fn.getpos(".")[2]
  vim.api.nvim_feedkeys(vim.api.nvim_replace_termcodes("<Esc>", true, false, true), "n", false)
  local s, e = math.min(p1, p2), math.max(p1, p2)
  if s < 1 then vim.notify("pith: make a visual selection first", vim.log.levels.WARN); return end
  vim.ui.input({ prompt = "pith edit: " }, function(instruction)
    if not instruction or instruction == "" then return end
    vim.notify("pith: editing…", vim.log.levels.INFO)
    if config.agent and vim.bo.modified then
      vim.cmd("silent noautocmd write")
    end
    local args = { "edit", file, "--range", s .. ":" .. e, "--prompt", instruction }
    if not config.agent then
      table.insert(args, "--raw")
    end
    vim.list_extend(args, config.backend_args)
    run_async(args, function(out, err)
      if not out then
        vim.notify("pith: " .. (err or "failed"), vim.log.levels.ERROR)
        return
      end
      if config.agent then
        local ok, content = pcall(vim.fn.readfile, file)
        if ok then
          vim.api.nvim_buf_set_lines(0, 0, -1, false, content)
          vim.notify("pith: applied (u to undo)", vim.log.levels.INFO)
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

-- generate: create a new file from a description prompt.
function M.generate()
  if not need_backend() then return end
  vim.ui.input({ prompt = "pith generate — new file path: " }, function(path)
    if not path or path == "" then return end
    vim.ui.input({ prompt = "pith generate — what to generate: " }, function(prompt)
      if not prompt or prompt == "" then return end
      vim.notify("pith: generating " .. path .. "…", vim.log.levels.INFO)
      local args = { "generate", path, "--prompt", prompt, "--apply" }
      vim.list_extend(args, config.backend_args)
      run_async(args, function(_, err)
        if err then vim.notify("pith: " .. err, vim.log.levels.ERROR); return end
        vim.notify("pith: wrote " .. path, vim.log.levels.INFO)
        pcall(vim.cmd, "edit " .. vim.fn.fnameescape(path))
      end)
    end)
  end)
end

-- work_list: show the project work-tracker in a float.
function M.work_list()
  local lines, err = run_sync({ "work" })
  if not lines then
    vim.notify("pith: " .. (err or "no work items"), vim.log.levels.INFO)
    return
  end
  open_float(lines, nil, false)
end

-- work_add: add a work note anchored to the current file:line.
function M.work_add()
  local file = vim.api.nvim_buf_get_name(0)
  local lnum = vim.api.nvim_win_get_cursor(0)[1]
  vim.ui.input({ prompt = "pith work add: " }, function(note)
    if not note or note == "" then return end
    local args = { "work", "add", note }
    if file ~= "" then
      vim.list_extend(args, { "--at", file .. ":" .. lnum })
    end
    vim.fn.system(vim.list_extend({ config.bin }, args))
    if vim.v.shell_error == 0 then
      vim.notify("pith: work item added", vim.log.levels.INFO)
    else
      vim.notify("pith: failed to add work item", vim.log.levels.ERROR)
    end
  end)
end

return M
