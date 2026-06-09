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
	var jsonOut, grepOut, vsOut, apply, raw, recursive, allFlag bool
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
		"  pith search   <query> [dir] [-r] [--json]            deterministic; add --cmd/--api/--agent for AI\n" +
		"  pith explain  <file> <name> --cmd \"<llm>\"            deep explanation of one declaration\n" +
		"  pith explain  <file:line>  --cmd \"<llm>\"\n" +
		"  pith summary  <file|dir> --cmd \"<llm>\"\n" +
		"  pith edit     <file> --range A:B --prompt \"...\" --cmd \"<llm>\" [--apply|--raw] [--context around|file|dir|project]\n" +
		"  pith generate <newfile> --prompt \"...\" --cmd \"<llm>\" [--apply] [--context file|dir|project]\n" +
		"  pith work     [add \"<note>\" [--at file:line] | done <id> | rm <id> | clear | --all]"
	if len(pos) == 0 {
		die(usage)
	}
	if pos[0] == "work" {
		workCmd(pos[1:], atArg, allFlag, jsonOut)
		return
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
		if backend.None() {
			die(pith.NoBackendMsg)
		}
		editCmd(target, rangeArg, promptArg, backend, apply, raw, contextArg)
	case "generate", "gen":
		if backend.None() {
			die(pith.NoBackendMsg)
		}
		generateCmd(target, promptArg, backend, apply, raw, contextArg)
	default:
		die("pith: unknown command", cmd)
	}
}

// editCmd sends a line range + an instruction to the backend and applies the
// result. Diff by default (review), --apply writes it, --raw prints just the
// new region (for an editor to replace the selection with).
func editCmd(file, rangeArg, prompt string, backend pith.Backend, apply, raw bool, ctxLevel string) {
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
	context, err := pith.BuildContext(file, ctxLevel)
	if err != nil {
		die("pith:", err)
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
func generateCmd(file, prompt string, backend pith.Backend, apply, raw bool, ctxLevel string) {
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
