-- pith.nvim — purpose-map, AI summary, edit, explain, search, work-tracker.
--
-- The binary is found automatically: repo checkout → plugin bin/ → a release
-- binary downloaded on first use → pith on PATH. `bin` in setup() overrides.
--
-- Setup (lazy.nvim, local dir):
--   {
--     dir = "path/to/pith/nvim",
--     config = function()
--       require("pith").setup({
--         backend_args = { "--agent", "claude --dangerously-skip-permissions -p" },
--         agent        = true,
--       })
--       local p = require("pith")
--       vim.keymap.set("n",          "<leader>po", p.overview,   { desc = "pith: overview" })
--       vim.keymap.set("n",          "<leader>pm", p.map,        { desc = "pith: map project" })
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

-- Pinned pith release this plugin auto-downloads when no binary is found.
M.version = "0.4.0"

local config = {
  bin          = nil,   -- explicit binary path; nil = resolve automatically
  backend_args = {},    -- { "--agent", "claude --dangerously-skip-permissions -p" }
  agent        = false, -- true when backend edits files itself (edit reloads buffer)
  context      = nil,   -- nil = none, "ask" = pick per edit/generate, or a fixed
                        -- level: "around"|"file"|"dir"|"project"
  width        = 0.7,
  height       = 0.7,
  border       = "rounded",
}

function M.setup(opts)
  config = vim.tbl_deep_extend("force", config, opts or {})
end

-- ─── binary resolution ──────────────────────────────────────────────────────
-- Order: setup({bin=...}) → repo-checkout binary (plugin lives in pith/nvim)
-- → bin/ inside the plugin → previously downloaded release → pith on PATH.
-- Nothing found → kick off an async download of the pinned release.

local uv = vim.uv or vim.loop

local function platform()
  local u = uv.os_uname()
  local sys = u.sysname:lower()
  local os_ = sys:find("windows") and "windows" or (sys:find("darwin") and "darwin" or "linux")
  local m = (u.machine or ""):lower()
  local arch = (m:find("aarch") or m:find("arm")) and "arm64" or "amd64"
  return os_, arch
end

local function exists(p) return p and uv.fs_stat(p) ~= nil end

local function plugin_root()
  local src = debug.getinfo(1, "S").source:sub(2)
  return vim.fs.normalize(src):gsub("/lua/pith/init%.lua$", "")
end

local function download_target()
  local os_ = platform()
  local ext = os_ == "windows" and ".exe" or ""
  return vim.fn.stdpath("data") .. "/pith/" .. M.version .. "/pith" .. ext
end

local resolved
local function resolve_bin()
  if config.bin then return config.bin end
  if resolved then return resolved end
  local os_, arch = platform()
  local ext = os_ == "windows" and ".exe" or ""
  local root = plugin_root()
  local candidates = {
    root:gsub("/nvim$", "") .. "/pith" .. ext,                    -- repo checkout
    ("%s/bin/pith-%s-%s%s"):format(root, os_, arch, ext),         -- bundled drop-in
    download_target(),                                            -- downloaded release
  }
  for _, p in ipairs(candidates) do
    if exists(p) then
      resolved = p
      return resolved
    end
  end
  if vim.fn.executable("pith") == 1 then
    resolved = "pith"
    return resolved
  end
  return nil
end

local downloading = false
-- Downloads the pinned release binary into stdpath("data"). Safe to call
-- any time (also via lazy.nvim's build hook: ":lua require('pith').install()").
function M.install(on_done)
  if downloading then return end
  downloading = true
  local os_, arch = platform()
  local ext = os_ == "windows" and ".exe" or ""
  local asset = ("pith-%s-%s%s"):format(os_, arch, ext)
  local url = ("https://github.com/SirNiklas9/pith/releases/download/v%s/%s"):format(M.version, asset)
  local target = download_target()
  vim.fn.mkdir(vim.fn.fnamemodify(target, ":h"), "p")
  vim.notify("pith: downloading " .. asset .. " v" .. M.version .. "…")
  vim.system({ "curl", "-fsSL", "-o", target, url }, {}, function(res)
    vim.schedule(function()
      downloading = false
      if res.code ~= 0 then
        vim.notify(
          "pith: download failed — build it (go build -o pith ./cmd/pith), put it on PATH, or set bin in setup()",
          vim.log.levels.ERROR
        )
        return
      end
      if os_ ~= "windows" then uv.fs_chmod(target, 493) end -- 0755
      resolved = target
      vim.notify("pith: installed " .. target)
      if on_done then on_done() end
    end)
  end)
end

-- Which binary the plugin would run right now (nil = none yet).
function M.which() return resolve_bin() end

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
  local bin = resolve_bin()
  if not bin then
    M.install()
    return nil, "pith binary not found — downloading it now, retry in a moment"
  end
  local cmd = vim.list_extend({ bin }, args)
  local out  = vim.fn.systemlist(cmd)
  if vim.v.shell_error ~= 0 then
    return nil, table.concat(out, "\n")
  end
  return out
end

-- asynchronous run (Neovim 0.10+), calls on_done(stdout|nil, err|nil);
-- no binary yet → download it, then run.
local function run_async(args, on_done)
  local bin = resolve_bin()
  if not bin then
    M.install(function() run_async(args, on_done) end)
    return
  end
  local cmd = vim.list_extend({ bin }, args)
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

-- Resolves the --context args for an AI op per config.context, then calls
-- cb(ctx_args). "ask" shows a picker over levels; nil/none passes {}.
local function with_context(levels, cb)
  local c = config.context
  if not c or c == "none" then return cb({}) end
  if c ~= "ask" then return cb({ "--context", c }) end
  local items = vim.list_extend({ "none" }, levels)
  vim.ui.select(items, { prompt = "pith context:" }, function(choice)
    if not choice then return end -- dismissed = abort the op
    cb(choice == "none" and {} or { "--context", choice })
  end)
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
    with_context({ "around", "file", "dir", "project" }, function(ctx_args)
      vim.notify("pith: editing…", vim.log.levels.INFO)
      if config.agent and vim.bo.modified then
        vim.cmd("silent noautocmd write")
      end
      local args = { "edit", file, "--range", s .. ":" .. e, "--prompt", instruction }
      if not config.agent then
        table.insert(args, "--raw")
      end
      vim.list_extend(args, ctx_args)
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
  end)
end

-- generate: create a new file from a description prompt.
function M.generate()
  if not need_backend() then return end
  vim.ui.input({ prompt = "pith generate — new file path: " }, function(path)
    if not path or path == "" then return end
    vim.ui.input({ prompt = "pith generate — what to generate: " }, function(prompt)
      if not prompt or prompt == "" then return end
      with_context({ "dir", "project" }, function(ctx_args)
        vim.notify("pith: generating " .. path .. "…", vim.log.levels.INFO)
        local args = { "generate", path, "--prompt", prompt, "--apply" }
        vim.list_extend(args, ctx_args)
        vim.list_extend(args, config.backend_args)
        run_async(args, function(_, err)
          if err then vim.notify("pith: " .. err, vim.log.levels.ERROR); return end
          vim.notify("pith: wrote " .. path, vim.log.levels.INFO)
          pcall(vim.cmd, "edit " .. vim.fn.fnameescape(path))
        end)
      end)
    end)
  end)
end

-- map: whole-repo purpose map, one row per package. Purposes need a backend
-- and are content-hash cached in .pith-map.json — re-runs are free until the
-- code changes. Without a backend the map is deterministic structure only.
function M.map()
  local cwd  = vim.fn.getcwd()
  local args = { "map", cwd }
  vim.list_extend(args, config.backend_args)
  vim.notify("pith: mapping…", vim.log.levels.INFO)
  run_async(args, function(out, err)
    if not out then vim.notify("pith: " .. err, vim.log.levels.ERROR); return end
    open_float(vim.split(vim.trim(out), "\n"), nil, false)
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
    local bin = resolve_bin()
    if not bin then
      M.install()
      vim.notify("pith: binary not found — downloading, retry shortly", vim.log.levels.WARN)
      return
    end
    vim.fn.system(vim.list_extend({ bin }, args))
    if vim.v.shell_error == 0 then
      vim.notify("pith: work item added", vim.log.levels.INFO)
    else
      vim.notify("pith: failed to add work item", vim.log.levels.ERROR)
    end
  end)
end

return M
