// Command pith — the pith of your code.
//
// Deterministic, no AI, no network:
//
//	pith read <file.go> [name]     one file's purpose map (grouped by type)
//	pith read <dir>                a whole package, file by file
//	  ... [--grep] [--json]        file:line / structured output (pipe, quickfix, editors)
//
// AI, opt-in (the deterministic read above never needs it; you name the backend
// or the op refuses — nothing is ever silently billed):
//
//	pith summary  <file|dir>      --cmd/--api/--agent
//	pith edit     <file> --range A:B --prompt "..."   --cmd/--api/--agent [--apply|--raw]
//	pith generate <newfile>      --prompt "..."        --cmd/--api/--agent [--apply|--raw]
//
// The engine lives in the importable package pith; this is just its CLI.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"pith"
)

func main() {
	args := os.Args[1:]
	var jsonOut, grepOut, vsOut, apply, raw, recursive, allFlag, aiFlag, dryRun bool
	var backend pith.Backend
	var rangeArg, promptArg, atArg, contextArg string
	var pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		val := func() string { // consume the next arg as this flag's value
			if i+1 < len(args) {
				i++
				return args[i]
			}
			return ""
		}
		switch {
		case a == "--json":
			jsonOut = true
		case a == "--grep":
			grepOut = true
		case a == "--vs":
			vsOut = true
		case a == "--apply":
			apply = true
		case a == "--raw":
			raw = true
		case a == "-r" || a == "--recursive":
			recursive = true
		case a == "--ai":
			aiFlag = true
		case a == "--dry-run":
			dryRun = true
		case a == "--all":
			allFlag = true
		case a == "--at":
			atArg = val()
		case strings.HasPrefix(a, "--at="):
			atArg = strings.TrimPrefix(a, "--at=")
		case a == "--cmd":
			backend.Cmd = val()
		case strings.HasPrefix(a, "--cmd="):
			backend.Cmd = strings.TrimPrefix(a, "--cmd=")
		case a == "--api":
			backend.API = val()
		case strings.HasPrefix(a, "--api="):
			backend.API = strings.TrimPrefix(a, "--api=")
		case a == "--agent":
			backend.Agent = val()
		case strings.HasPrefix(a, "--agent="):
			backend.Agent = strings.TrimPrefix(a, "--agent=")
		case a == "--model":
			backend.Model = val()
		case strings.HasPrefix(a, "--model="):
			backend.Model = strings.TrimPrefix(a, "--model=")
		case a == "--range":
			rangeArg = val()
		case strings.HasPrefix(a, "--range="):
			rangeArg = strings.TrimPrefix(a, "--range=")
		case a == "--prompt":
			promptArg = val()
		case strings.HasPrefix(a, "--prompt="):
			promptArg = strings.TrimPrefix(a, "--prompt=")
		case a == "--context":
			contextArg = val()
		case strings.HasPrefix(a, "--context="):
			contextArg = strings.TrimPrefix(a, "--context=")
		default:
			pos = append(pos, a)
		}
	}
	usage := "usage:\n" +
		"  pith read     <file|dir> [name] [--grep|--json]\n" +
		"  pith map      [dir] [--json]                          repo map, one row per package; --ai (stored backend) or --cmd/--api/--agent adds cached one-line purposes\n" +
		"  pith search   <query> [dir] [-r] [--json]            deterministic; add --cmd/--api/--agent for AI\n" +
		"  pith explain  <file> <name> --cmd \"<llm>\"            deep explanation of one declaration\n" +
		"  pith explain  <file:line>  --cmd \"<llm>\"\n" +
		"  pith summary  <file|dir> --cmd \"<llm>\"\n" +
		"  pith edit     <file> --range A:B --prompt \"...\" --cmd \"<llm>\" [--apply|--raw] [--context around|file|dir|project|uses[:dir|:project][:N|:all][:full|:fullN]]\n" +
		"  pith generate <newfile> --prompt \"...\" --cmd \"<llm>\" [--apply] [--context file|dir|project]\n" +
		"  pith work     [add \"<note>\" [--at file:line] | done <id> | rm <id> | clear | --all]\n" +
		"  pith config   [set <name> <value> | unset <name> | path]   set-and-forget backend + API keys\n" +
		"  pith price    [model]    fetch + cache current rates so --dry-run shows cost (offline after)"
	if len(pos) == 0 {
		die(usage)
	}
	if pos[0] == "work" {
		workCmd(pos[1:], atArg, allFlag, jsonOut)
		return
	}
	if pos[0] == "config" {
		configCmd(pos[1:])
		return
	}
	if pos[0] == "price" {
		model := backend.Model
		if len(pos) >= 2 {
			model = pos[1]
		}
		if model == "" {
			if cfg, err := pith.LoadConfig(); err == nil {
				model = cfg.Model
			}
		}
		if model == "" {
			die("pith price needs a model, e.g. pith price anthropic/claude-haiku-4.5")
		}
		p, err := pith.FetchPrice(model)
		if err != nil {
			die("pith:", err)
		}
		fmt.Printf("%s\n  input   $%.2f /M tokens\n  output  $%.2f /M tokens\nCached — dry-run previews now show cost, offline.\n",
			model, p.In*1e6, p.Out*1e6)
		return
	}
	if pos[0] == "map" {
		dir := "."
		if len(pos) >= 2 {
			dir = pos[1]
		}
		mapCmd(dir, backend, aiFlag, jsonOut)
		return
	}
	// Set-and-forget defaults: AI ops with no backend flags fall back to the
	// stored config. Search is deliberately excluded — it is deterministic by
	// default and goes AI only on EXPLICIT flags, so a stored default can never
	// silently turn a free op into a billed one.
	switch pos[0] {
	case "explain", "summary", "edit", "generate", "gen":
		if backend.None() {
			if cfg, err := pith.LoadConfig(); err == nil {
				backend.API, backend.Model = cfg.API, cfg.Model
				backend.Agent, backend.Cmd = cfg.Agent, cfg.Cmd
			}
		}
	}
	if len(pos) < 2 {
		die(usage)
	}
	cmd, target := pos[0], pos[1]
	only := ""
	if len(pos) >= 3 {
		only = pos[2]
	}

	switch cmd {
	case "read":
		r, err := pith.Gather(target, only)
		if err != nil {
			die("pith:", err)
		}
		switch {
		case jsonOut:
			_ = r.WriteJSON(os.Stdout)
		case vsOut:
			r.WriteVS(os.Stdout)
		case grepOut:
			r.WriteGrep(os.Stdout)
		default:
			r.WriteText(os.Stdout)
		}
	case "search":
		query := target  // pos[1] is the query for search
		dir := only      // pos[2] is the optional directory
		if dir == "" {
			dir = "."
		}
		if backend.None() {
			matches, err := pith.Search(dir, query, recursive)
			if err != nil {
				die("pith:", err)
			}
			if len(matches) == 0 {
				fmt.Fprintln(os.Stderr, "pith: no matches")
				return
			}
			res := pith.Result{All: matches}
			switch {
			case jsonOut:
				_ = res.WriteJSON(os.Stdout)
			case vsOut:
				res.WriteVS(os.Stdout)
			default:
				res.WriteGrep(os.Stdout)
			}
			return
		}
		out, err := pith.SearchAI(dir, query, recursive, backend)
		if err != nil {
			die("pith:", err)
		}
		fmt.Println(out)
	case "explain":
		if backend.None() {
			die(pith.NoBackendMsg)
		}
		file, name, line := target, only, 0
		if name == "" {
			// check if target is file:line
			f, l := parseAt(target)
			if l > 0 {
				file, line = f, l
			}
		}
		ctx, err := pith.BuildContext(file, contextArg)
		if err != nil {
			die("pith:", err)
		}
		out, err := pith.Explain(file, name, line, ctx, backend)
		if err != nil {
			die("pith:", err)
		}
		fmt.Println(out)
	case "summary":
		if backend.None() {
			die(pith.NoBackendMsg)
		}
		out, err := pith.Summarize(target, only, backend)
		if err != nil {
			die("pith:", err)
		}
		fmt.Println(out)
	case "edit":
		if backend.None() && !dryRun {
			die(pith.NoBackendMsg)
		}
		editCmd(target, rangeArg, promptArg, backend, apply, raw, contextArg, dryRun)
	case "generate", "gen":
		if backend.None() && !dryRun {
			die(pith.NoBackendMsg)
		}
		generateCmd(target, promptArg, backend, apply, raw, contextArg, dryRun)
	default:
		die("pith: unknown command", cmd)
	}
}

// editCmd sends a line range + an instruction to the backend and applies the
// result. Diff by default (review), --apply writes it, --raw prints just the
// new region (for an editor to replace the selection with).
func editCmd(file, rangeArg, prompt string, backend pith.Backend, apply, raw bool, ctxLevel string, dryRun bool) {
	if rangeArg == "" || prompt == "" {
		die("pith edit needs --range A:B and --prompt \"...\"")
	}
	a, b, err := pith.ParseRange(rangeArg)
	if err != nil {
		die("pith:", err)
	}
	src, err := os.ReadFile(file)
	if err != nil {
		die("pith:", err)
	}
	lines := strings.Split(string(src), "\n")
	if a < 1 || b > len(lines) || a > b {
		die(fmt.Sprintf("pith: range %d:%d out of bounds (file has %d lines)", a, b, len(lines)))
	}
	region := strings.Join(lines[a-1:b], "\n")
	context, err := pith.BuildContextRegion(file, ctxLevel, a, b)
	if err != nil {
		die("pith:", err)
	}

	// DRY RUN: assemble everything exactly as a real call would, then report
	// instead of sending — what the context resolved to, and what it weighs.
	// Deterministic, offline, keyless.
	if dryRun {
		full := pith.EditPrompt(file, region, prompt, context)
		if backend.IsAgent() {
			full = pith.AgentEditTask(file, a, b, region, prompt, context)
		}
		printDryRun(fmt.Sprintf("edit %s %d:%d", file, a, b), ctxLevel, file, a, b,
			[][2]any{{fmt.Sprintf("region (lines %d:%d)", a, b), len(region)}, {"context", len(context)}, {"instruction", len(prompt)}},
			full, raw, dryRunModel(backend))
		return
	}

	// AGENT backend: hand it the task and let the agent edit the file ITSELF —
	// its native mode. pith does not splice; the caller reloads the file after.
	if backend.IsAgent() {
		if err := backend.RunAgentStreaming(pith.AgentEditTask(file, a, b, region, prompt, context)); err != nil {
			die("pith: agent failed:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: agent ran on %s (lines %d:%d) — reload the file\n", file, a, b)
		return
	}

	// COMPLETION backend: get the rewritten region, splice it.
	out, err := backend.Run(pith.EditPrompt(file, region, prompt, context))
	if err != nil {
		die("pith: backend failed:", err)
	}
	newRegion := pith.StripFences(strings.TrimRight(out, "\n"))

	switch {
	case raw:
		fmt.Println(newRegion)
	case apply:
		merged := append([]string{}, lines[:a-1]...)
		merged = append(merged, strings.Split(newRegion, "\n")...)
		merged = append(merged, lines[b:]...)
		if err := os.WriteFile(file, []byte(strings.Join(merged, "\n")), 0o644); err != nil {
			die("pith:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: applied to %s (lines %d:%d)\n", file, a, b)
	default:
		renderDiff(file, a, lines[a-1:b], strings.Split(newRegion, "\n"))
	}
}

// generateCmd is the new-file op: a prompt becomes one file. Unlike edit, there
// is no region — generation reads NOTHING but your prompt. No sibling scan, no
// same-file context, no search: that is all opt-in and lives behind an explicit
// flag, never automatic. Refuses to overwrite a non-empty file (use edit for
// those); an empty file is fillable, so the editor "New File → fill" flow works.
func generateCmd(file, prompt string, backend pith.Backend, apply, raw bool, ctxLevel string, dryRun bool) {
	if prompt == "" {
		die("pith generate needs --prompt \"...\"")
	}
	if fi, err := os.Stat(file); err == nil && fi.Size() > 0 {
		die(fmt.Sprintf("pith: %s already exists and is not empty — use `pith edit` for existing files", file))
	}
	context, err := pith.BuildContext(file, ctxLevel)
	if err != nil {
		die("pith:", err)
	}

	if dryRun {
		full := pith.GeneratePrompt(file, prompt, context)
		if backend.IsAgent() {
			full = pith.AgentGenerateTask(file, prompt, context)
		}
		printDryRun("generate "+file, ctxLevel, file, 0, 0,
			[][2]any{{"context", len(context)}, {"instruction", len(prompt)}}, full, raw, dryRunModel(backend))
		return
	}

	// AGENT backend: let the agent create the file ITSELF; pith does not write.
	if backend.IsAgent() {
		if err := backend.RunAgentStreaming(pith.AgentGenerateTask(file, prompt, context)); err != nil {
			die("pith: agent failed:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: agent ran to create %s — check it in\n", file)
		return
	}

	// COMPLETION backend: get the file content, write it.
	out, err := backend.Run(pith.GeneratePrompt(file, prompt, context))
	if err != nil {
		die("pith: backend failed:", err)
	}
	content := pith.StripFences(strings.TrimRight(out, "\n"))

	switch {
	case raw:
		fmt.Println(content)
	case apply:
		if dir := dirOf(file); dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				die("pith:", err)
			}
		}
		if err := os.WriteFile(file, []byte(content+"\n"), 0o644); err != nil {
			die("pith:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: wrote %s (%d lines)\n", file, strings.Count(content, "\n")+1)
	default:
		fmt.Printf("--- %s (proposed — rerun with --apply to write, --raw for just the code)\n", file)
		fmt.Println(content)
	}
}

// mapCmd renders the repo map. Deterministic by default; purposes are AI and
// run only on explicit backend flags or --ai (which opts in to the stored
// config backend) — a bare `pith map` can never silently become a billed op.
// Purposes are cached by content hash in .pith-map.json, so re-runs are free
// until a package actually changes.
func mapCmd(dir string, backend pith.Backend, useAI, jsonOut bool) {
	if useAI && backend.None() {
		if cfg, err := pith.LoadConfig(); err == nil {
			backend.API, backend.Model = cfg.API, cfg.Model
			backend.Agent, backend.Cmd = cfg.Agent, cfg.Cmd
		}
		if backend.None() {
			die("pith: --ai needs a stored backend (see pith config)")
		}
	}
	pkgs, err := pith.Map(dir)
	if err != nil {
		die("pith:", err)
	}
	if len(pkgs) == 0 {
		die("pith: no recognized code under", dir)
	}
	if !backend.None() {
		if err := pith.MapAI(dir, pkgs, backend); err != nil {
			die("pith:", err)
		}
	}
	if jsonOut {
		_ = pith.WriteMapJSON(os.Stdout, pkgs)
	} else {
		pith.WriteMapText(os.Stdout, dir, pkgs)
	}
}

// workCmd is the deterministic, no-AI work tracker: a persistent list of notes
// you owe yourself, optionally anchored to a file:line. The list lives at the
// git root (see pith.DefaultWorkPath) so it is the same from any subdirectory.
func workCmd(args []string, at string, all, jsonOut bool) {
	path := pith.DefaultWorkPath()
	wl, err := pith.LoadWork(path)
	if err != nil {
		die("pith:", err)
	}
	sub, rest := "list", []string(nil)
	if len(args) > 0 {
		sub, rest = args[0], args[1:]
	}
	switch sub {
	case "list", "ls":
		if jsonOut {
			_ = wl.WriteJSON(os.Stdout)
		} else {
			wl.Render(os.Stdout, all)
		}
	case "add":
		note := strings.TrimSpace(strings.Join(rest, " "))
		if note == "" {
			die("pith work add needs a note, e.g. pith work add \"fix the off-by-one\" --at file.go:42")
		}
		file, line := parseAt(at)
		it := wl.Add(file, line, note)
		if err := wl.Save(); err != nil {
			die("pith:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: added work #%d\n", it.ID)
	case "done":
		id := mustID(rest, "done")
		if !wl.Done(id) {
			die(fmt.Sprintf("pith: no work #%d", id))
		}
		if err := wl.Save(); err != nil {
			die("pith:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: closed work #%d\n", id)
	case "rm":
		id := mustID(rest, "rm")
		if !wl.Remove(id) {
			die(fmt.Sprintf("pith: no work #%d", id))
		}
		if err := wl.Save(); err != nil {
			die("pith:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: removed work #%d\n", id)
	case "clear":
		n := wl.ClearDone()
		if err := wl.Save(); err != nil {
			die("pith:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: cleared %d done item(s)\n", n)
	default:
		die("pith work: unknown subcommand", sub)
	}
}

// configCmd is the set-and-forget store: default backend + API keys, handed to
// pith once and saved in the user-config dir (no env vars, no registry, no
// per-shell exports). `pith config` shows it (keys masked), set/unset edit it,
// path prints where it lives.
func configCmd(args []string) {
	cfg, err := pith.LoadConfig()
	if err != nil {
		die("pith:", err)
	}
	sub, rest := "show", []string(nil)
	if len(args) > 0 {
		sub, rest = args[0], args[1:]
	}
	switch sub {
	case "show", "list", "ls":
		var b strings.Builder
		cfg.Render(&b)
		fmt.Print(b.String())
	case "set":
		if len(rest) < 2 {
			die("pith config set needs <name> <value>, e.g.\n" +
				"  pith config set api openrouter\n" +
				"  pith config set model openai/gpt-4o-mini\n" +
				"  pith config set OPENROUTER_API_KEY sk-or-...")
		}
		cfg.Set(rest[0], strings.Join(rest[1:], " "))
		if err := cfg.Save(); err != nil {
			die("pith:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: %s set\n", rest[0])
	case "unset":
		if len(rest) < 1 {
			die("pith config unset needs <name>")
		}
		cfg.Unset(rest[0])
		if err := cfg.Save(); err != nil {
			die("pith:", err)
		}
		fmt.Fprintf(os.Stderr, "pith: %s unset\n", rest[0])
	case "path":
		p, err := pith.ConfigPath()
		if err != nil {
			die("pith:", err)
		}
		fmt.Println(p)
	default:
		die("pith config: unknown subcommand", sub)
	}
}

// mustID reads a positive integer work id from a subcommand's args, or dies.
func mustID(rest []string, sub string) int {
	if len(rest) == 0 {
		die(fmt.Sprintf("pith work %s needs an id (see `pith work`)", sub))
	}
	id, err := strconv.Atoi(rest[0])
	if err != nil {
		die(fmt.Sprintf("pith: %q is not a work id", rest[0]))
	}
	return id
}

// parseAt splits a "file:line" anchor. It splits on the LAST colon so Windows
// drive letters (C:\...) survive; a missing or non-numeric line yields line 0.
func parseAt(s string) (file string, line int) {
	if s == "" {
		return "", 0
	}
	if i := strings.LastIndex(s, ":"); i >= 0 {
		if n, err := strconv.Atoi(s[i+1:]); err == nil {
			return s[:i], n
		}
	}
	return s, 0
}

// printDryRun renders the deterministic preview of an AI op: the uses hop
// tree (when the context level is relational), a byte/~token budget per prompt
// part, and — with --raw — the exact prompt. Offline, keyless, free.
func printDryRun(op, ctxLevel, file string, a, b int, parts [][2]any, full string, raw bool, model string) {
	fmt.Printf("DRY RUN  pith %s  (nothing sent — offline, keyless)\n\n", op)

	if strings.HasPrefix(ctxLevel, "uses") {
		decls, _, err := pith.UsesClosure(file, ctxLevel, a, b)
		if err == nil {
			fmt.Printf("context %s — %d declaration(s) resolved\n", ctxLevel, len(decls))
			for _, d := range decls {
				fmt.Printf("  hop %d  %s:%d  %s  [%d B]\n", d.Hop, d.File, d.Line, d.Sig, len(d.Source))
			}
			fmt.Println()
		}
	} else if ctxLevel != "" && ctxLevel != "none" {
		fmt.Printf("context %s\n\n", ctxLevel)
	}

	fmt.Println("prompt budget                    bytes   ~tokens")
	known := 0
	for _, p := range parts {
		name, n := p[0].(string), p[1].(int)
		known += n
		fmt.Printf("  %-28s %7d   %7d\n", name, n, pith.EstimateTokens(n))
	}
	scaffold := len(full) - known
	fmt.Printf("  %-28s %7d   %7d\n", "scaffolding", scaffold, pith.EstimateTokens(scaffold))
	lo, hi := pith.EstimateTokensRange(len(full))
	fmt.Printf("  %-28s %7d   %d–%d\n", "TOTAL", len(full), lo, hi)
	fmt.Println("\n~tokens = bytes/4 per part; the total is a range covering all mainstream tokenizers")
	if model != "" {
		if p, ok := pith.CachedPrice(model); ok {
			fmt.Printf("est. input cost  %s–%s   (%s @ $%.2f/M in, $%.2f/M out — output billed on the reply)\n",
				pith.Dollars(float64(lo)*p.In), pith.Dollars(float64(hi)*p.In), model, p.In*1e6, p.Out*1e6)
		} else {
			fmt.Printf("pricing: not cached — run `pith price %s` once and previews show cost offline\n", model)
		}
	}
	if raw {
		fmt.Println("\n--- exact prompt ---")
		fmt.Println(full)
	} else {
		fmt.Println("(add --raw to print the exact prompt)")
	}
}

// dryRunModel names the model a dry run would bill against: an explicit
// --model wins (API mode), else the stored config default.
func dryRunModel(backend pith.Backend) string {
	if backend.Model != "" {
		return backend.Model
	}
	if cfg, err := pith.LoadConfig(); err == nil && cfg.Model != "" {
		return cfg.Model
	}
	return ""
}

// renderDiff prints the old region (-) vs the proposed new region (+).
func renderDiff(file string, start int, oldLines, newLines []string) {
	fmt.Printf("--- %s:%d (current)\n", file, start)
	for _, l := range oldLines {
		fmt.Printf("- %s\n", l)
	}
	fmt.Println("+++ (proposed — rerun with --apply to write, --raw for just the code)")
	for _, l := range newLines {
		fmt.Printf("+ %s\n", l)
	}
}

// dirOf returns the directory part of a path, or "" if it has none.
func dirOf(file string) string {
	if i := strings.LastIndexAny(file, `/\`); i >= 0 {
		return file[:i]
	}
	return ""
}

func die(a ...any) {
	fmt.Fprintln(os.Stderr, a...)
	os.Exit(1)
}
