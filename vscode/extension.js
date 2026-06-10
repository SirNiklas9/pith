// pith VS Code extension — a thin UI shell over the pith CLI.
// The binary ships inside the extension (bin/); settings can override it.
// pith core doesn't change; this only wires ops to VS Code UI.

const vscode = require("vscode");
const cp = require("child_process");
const fs = require("fs");
const path = require("path");

let extensionDir = "";
let output; // OutputChannel, lazily created

// ─── binary + backend resolution ────────────────────────────────────────────

function platform() {
  const goos = process.platform === "win32" ? "windows"
    : process.platform === "darwin" ? "darwin" : "linux";
  const goarch = process.arch === "arm64" ? "arm64" : "amd64";
  return { goos, goarch };
}

// Order: explicit setting → binary bundled in the extension → pith on PATH.
function resolveBinary() {
  const configured = vscode.workspace.getConfiguration("pith").get("binaryPath", "").trim();
  if (configured) return configured;
  const { goos, goarch } = platform();
  const ext = goos === "windows" ? ".exe" : "";
  const bundled = path.join(extensionDir, "bin", `pith-${goos}-${goarch}${ext}`);
  if (fs.existsSync(bundled)) {
    if (goos !== "windows") {
      try { fs.chmodSync(bundled, 0o755); } catch (_) { /* read-only install dir is fine */ }
    }
    return bundled;
  }
  return "pith";
}

function backendArgs() {
  const cfg = vscode.workspace.getConfiguration("pith");
  switch (cfg.get("backendMode", "config")) {
    case "agent": return ["--agent", cfg.get("agentCommand", "")];
    case "api":   return ["--api", cfg.get("apiTarget", ""), "--model", cfg.get("apiModel", "")];
    default:      return []; // pith's own stored config decides
  }
}

function agentMode() {
  return vscode.workspace.getConfiguration("pith").get("backendMode") === "agent";
}

// Context args for edit/generate per the pith.context setting; "ask" shows a
// per-invocation QuickPick over the given levels. Resolves to [] for none, or
// null if the user dismissed the picker (caller should abort).
async function contextArgs(levels) {
  let level = vscode.workspace.getConfiguration("pith").get("context", "none");
  if (level === "ask") {
    const items = levels.map(([value, detail]) => ({ label: value, detail }));
    const picked = await vscode.window.showQuickPick(items, { title: "pith: context to send" });
    if (!picked) return null;
    level = picked.label;
  }
  if (level === "none" || !levels.some(([v]) => v === level)) return [];
  return ["--context", level];
}

const EDIT_CONTEXTS = [
  ["none", "just the selection"],
  ["uses:dir", "what the selection references — outlines (this folder)"],
  ["uses:dir:full", "what the selection references — full implementations"],
  ["uses:dir:3:full", "follow the chain 3 hops — implementations of what those use too"],
  ["around", "this file's outline"],
  ["file", "this file's full source"],
  ["dir", "the folder's outline"],
  ["project", "the whole project's outline"],
];
const GENERATE_CONTEXTS = [
  ["none", "just the prompt"],
  ["dir", "the destination folder's outline"],
  ["project", "the whole project's outline"],
];

function workspaceRoot() {
  const folders = vscode.workspace.workspaceFolders;
  return folders && folders.length ? folders[0].uri.fsPath : undefined;
}

// Runs pith, resolves { code, stdout, stderr }. Everything streams into the
// output channel so agent narration is visible while it runs.
function runPith(args, cwd) {
  return new Promise((resolve) => {
    if (!output) output = vscode.window.createOutputChannel("pith");
    output.appendLine(`$ pith ${args.join(" ")}`);
    const child = cp.spawn(resolveBinary(), args, { cwd: cwd || workspaceRoot() });
    let stdout = "", stderr = "";
    child.stdout.on("data", (d) => { stdout += d; output.append(d.toString()); });
    child.stderr.on("data", (d) => { stderr += d; output.append(d.toString()); });
    child.on("error", (err) => resolve({ code: -1, stdout, stderr: String(err) }));
    child.on("close", (code) => resolve({ code, stdout, stderr }));
  });
}

function fail(res, what) {
  const msg = (res.stderr || res.stdout || "failed").trim().split("\n").pop();
  vscode.window.showErrorMessage(`pith ${what}: ${msg}`);
  if (output) output.show(true);
}

// ─── result presentation ────────────────────────────────────────────────────

// Grep-format lines (file:line: rest) → QuickPick that jumps on accept.
async function pickAndJump(lines, title) {
  const items = [];
  for (const line of lines) {
    const m = line.match(/^(.*?):(\d+):\s*(.*)$/);
    if (m) items.push({ label: m[3], description: `${path.basename(m[1])}:${m[2]}`, file: m[1], line: parseInt(m[2], 10) });
    else if (line.trim()) items.push({ label: line });
  }
  if (!items.length) { vscode.window.showInformationMessage(`pith: ${title} — nothing found`); return; }
  const picked = await vscode.window.showQuickPick(items, { title: `pith: ${title}`, matchOnDescription: true });
  if (!picked || !picked.file) return;
  const doc = await vscode.workspace.openTextDocument(picked.file);
  const editor = await vscode.window.showTextDocument(doc);
  const pos = new vscode.Position(picked.line - 1, 0);
  editor.selection = new vscode.Selection(pos, pos);
  editor.revealRange(new vscode.Range(pos, pos), vscode.TextEditorRevealType.InCenter);
}

// Prose results (summary/explain) → read-only markdown preview doc.
async function showProse(text) {
  const doc = await vscode.workspace.openTextDocument({ content: text.trim() + "\n", language: "markdown" });
  await vscode.window.showTextDocument(doc, { preview: true, viewColumn: vscode.ViewColumn.Beside });
}

function withProgress(title, task) {
  return vscode.window.withProgress(
    { location: vscode.ProgressLocation.Notification, title: `pith: ${title}` },
    task
  );
}

function currentFile() {
  const editor = vscode.window.activeTextEditor;
  if (!editor || editor.document.isUntitled) {
    vscode.window.showWarningMessage("pith: no file open");
    return undefined;
  }
  return editor;
}

// ─── commands ───────────────────────────────────────────────────────────────

async function cmdRead() {
  const editor = currentFile();
  if (!editor) return;
  const res = await runPith(["read", editor.document.fileName, "--grep"]);
  if (res.code !== 0) return fail(res, "read");
  await pickAndJump(res.stdout.split("\n"), "read");
}

async function cmdReadFolder() {
  const editor = currentFile();
  if (!editor) return;
  const res = await runPith(["read", path.dirname(editor.document.fileName), "--grep"]);
  if (res.code !== 0) return fail(res, "read");
  await pickAndJump(res.stdout.split("\n"), "read folder");
}

async function cmdMap() {
  const root = workspaceRoot();
  if (!root) { vscode.window.showWarningMessage("pith: open a folder first"); return; }
  // Purposes are content-hash cached (.pith-map.json), so this is cheap after
  // the first run. In "config" mode --ai opts in to pith's stored backend.
  const backend = backendArgs();
  const args = ["map", root, ...(backend.length ? backend : ["--ai"])];
  await withProgress("mapping…", async () => {
    const res = await runPith(args);
    if (res.code !== 0) return fail(res, "map");
    const doc = await vscode.workspace.openTextDocument({ content: res.stdout, language: "plaintext" });
    await vscode.window.showTextDocument(doc, { preview: true });
  });
}

async function cmdSearch() {
  const query = await vscode.window.showInputBox({ prompt: "pith search" });
  if (!query) return;
  const root = workspaceRoot();
  if (!root) { vscode.window.showWarningMessage("pith: open a folder first"); return; }
  const res = await runPith(["search", query, root, "-r"]);
  if (res.code !== 0) return fail(res, "search");
  await pickAndJump(res.stdout.split("\n"), `search "${query}"`);
}

async function cmdSummary() {
  const editor = currentFile();
  if (!editor) return;
  await withProgress("summarizing…", async () => {
    const res = await runPith(["summary", editor.document.fileName, ...backendArgs()]);
    if (res.code !== 0) return fail(res, "summary");
    await showProse(res.stdout);
  });
}

async function cmdExplain() {
  const editor = currentFile();
  if (!editor) return;
  const line = editor.selection.active.line + 1;
  await withProgress("explaining…", async () => {
    const res = await runPith(["explain", `${editor.document.fileName}:${line}`, ...backendArgs()]);
    if (res.code !== 0) return fail(res, "explain");
    await showProse(res.stdout);
  });
}

async function cmdEdit() {
  const editor = currentFile();
  if (!editor) return;
  const sel = editor.selection;
  if (sel.isEmpty) { vscode.window.showWarningMessage("pith: select the lines to edit first"); return; }
  const start = sel.start.line + 1;
  const end = sel.end.character === 0 && sel.end.line > sel.start.line ? sel.end.line : sel.end.line + 1;
  const prompt = await vscode.window.showInputBox({ prompt: `pith edit ${start}:${end}` });
  if (!prompt) return;
  const ctx = await contextArgs(EDIT_CONTEXTS);
  if (ctx === null) return;

  const file = editor.document.fileName;

  // Preview gate: a deterministic --dry-run (what the context resolved to +
  // the token estimate) with a Send/Cancel modal. "Always send directly"
  // flips the pith.preview setting off globally.
  if (vscode.workspace.getConfiguration("pith").get("preview", true)) {
    const dry = await runPith(["edit", file, "--range", `${start}:${end}`, "--prompt", prompt, ...ctx, "--dry-run"]);
    if (dry.code !== 0) return fail(dry, "preview");
    const choice = await vscode.window.showInformationMessage(
      "pith — preview before send", { modal: true, detail: dry.stdout },
      "Send", "Always send directly"
    );
    if (!choice) return;
    if (choice === "Always send directly") {
      await vscode.workspace.getConfiguration("pith").update("preview", false, vscode.ConfigurationTarget.Global);
    }
  }
  await withProgress("editing…", async () => {
    if (agentMode()) {
      // The agent edits files on disk itself (it may touch more than the
      // selection); VS Code only reloads clean docs, so save everything.
      await vscode.workspace.saveAll(false);
      const res = await runPith(["edit", file, "--range", `${start}:${end}`, "--prompt", prompt, ...ctx, ...backendArgs()]);
      if (res.code !== 0) return fail(res, "edit");
      vscode.window.showInformationMessage("pith: edit applied (agent wrote the file)");
      return;
    }
    // Completion backends: --raw prints just the new region, we splice it
    // into the buffer — no disk writes, native undo.
    const res = await runPith(["edit", file, "--range", `${start}:${end}`, "--prompt", prompt, "--raw", ...ctx, ...backendArgs()]);
    if (res.code !== 0) return fail(res, "edit");
    const range = new vscode.Range(start - 1, 0, end - 1, editor.document.lineAt(end - 1).text.length);
    const text = res.stdout.replace(/\r?\n$/, "");
    await editor.edit((b) => b.replace(range, text));
    vscode.window.showInformationMessage("pith: edit applied (Ctrl+Z to undo)");
  });
}

async function cmdGenerate() {
  const rel = await vscode.window.showInputBox({ prompt: "pith generate — new file path (relative to workspace)" });
  if (!rel) return;
  const prompt = await vscode.window.showInputBox({ prompt: "pith generate — what to generate" });
  if (!prompt) return;
  const ctx = await contextArgs(GENERATE_CONTEXTS);
  if (ctx === null) return;
  const root = workspaceRoot();
  const file = path.isAbsolute(rel) ? rel : path.join(root || "", rel);
  await withProgress(`generating ${rel}…`, async () => {
    const res = await runPith(["generate", file, "--prompt", prompt, "--apply", ...ctx, ...backendArgs()]);
    if (res.code !== 0) return fail(res, "generate");
    const doc = await vscode.workspace.openTextDocument(file);
    await vscode.window.showTextDocument(doc);
  });
}

async function cmdWorkList() {
  const res = await runPith(["work"]);
  if (res.code !== 0) return fail(res, "work");
  await pickAndJump(res.stdout.split("\n"), "work");
}

// Write-through key entry: handed once to `pith config set` (pith's own
// store — file-protected, masked, shared by every editor); VS Code keeps nothing.
async function cmdSetApiKey() {
  const target = vscode.workspace.getConfiguration("pith").get("apiTarget", "openrouter");
  const env = target === "openai" ? "OPENAI_API_KEY"
    : target === "openrouter" ? "OPENROUTER_API_KEY"
    : target === "ollama" ? null : "PITH_API_KEY";
  if (!env) { vscode.window.showInformationMessage(`pith: '${target}' is local — no API key needed`); return; }
  const key = await vscode.window.showInputBox({ prompt: `pith: API key for ${target} (saved to pith config, not VS Code)`, password: true });
  if (!key) return;
  const res = await runPith(["config", "set", env, key.trim()]);
  if (res.code !== 0) return fail(res, "config set");
  vscode.window.showInformationMessage(`pith: ${env} saved to pith config`);
}

async function cmdWorkAdd() {
  const note = await vscode.window.showInputBox({ prompt: "pith work add" });
  if (!note) return;
  const args = ["work", "add", note];
  const editor = vscode.window.activeTextEditor;
  if (editor && !editor.document.isUntitled) {
    args.push("--at", `${editor.document.fileName}:${editor.selection.active.line + 1}`);
  }
  const res = await runPith(args);
  if (res.code !== 0) return fail(res, "work add");
  vscode.window.showInformationMessage("pith: work item added");
}

// ─── activation ─────────────────────────────────────────────────────────────

function activate(context) {
  extensionDir = context.extensionPath;
  const commands = {
    "pith.read": cmdRead,
    "pith.readFolder": cmdReadFolder,
    "pith.map": cmdMap,
    "pith.search": cmdSearch,
    "pith.summary": cmdSummary,
    "pith.explain": cmdExplain,
    "pith.edit": cmdEdit,
    "pith.generate": cmdGenerate,
    "pith.workList": cmdWorkList,
    "pith.workAdd": cmdWorkAdd,
    "pith.setApiKey": cmdSetApiKey,
  };
  for (const [id, fn] of Object.entries(commands)) {
    context.subscriptions.push(vscode.commands.registerCommand(id, fn));
  }
}

function deactivate() {}

module.exports = { activate, deactivate };
