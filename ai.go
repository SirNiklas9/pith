package pith

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// NoBackendMsg explains the backends a user can choose. pith never picks a paid
// backend for you — you name one with [Backend], or the AI ops refuse.
const NoBackendMsg = "this needs an AI backend you choose:\n" +
	"  AGENT (edits files itself — best for `edit`):\n" +
	"    --agent \"claude --dangerously-skip-permissions -p\"   Claude Code\n" +
	"    --agent \"codex exec --full-auto\"                       Codex\n" +
	"  COMPLETION (returns text — splices/prints):\n" +
	"    --api ollama --model llama3        local (offline, free)\n" +
	"    --api openai --model gpt-4o-mini   (env OPENAI_API_KEY)\n" +
	"    --cmd \"ollama run llama3\"           local CLI"

// Backend is the user-chosen AI backend. Exactly one of API, Cmd, or Agent is
// normally set; an empty Backend ([Backend.None] is true) refuses every AI op
// so nothing is ever silently billed.
type Backend struct {
	Cmd   string // a CLI whose stdin is the prompt (e.g. "ollama run llama3")
	API   string // an OpenAI-compatible preset or base URL (e.g. "openai")
	Model string // the model name, for API backends
	Agent string // an agent CLI that edits files itself (e.g. "claude -p")
}

// None reports whether no backend was chosen (every AI op should refuse).
func (b Backend) None() bool { return b.Cmd == "" && b.API == "" && b.Agent == "" }

// IsAgent reports whether the agent backend is set — the one that edits files
// itself rather than returning text for pith to splice.
func (b Backend) IsAgent() bool { return b.Agent != "" }

// Run sends prompt to a completion-style backend and returns its text. The API
// backend wins if set, then a --cmd, then the agent treated as a command (its
// stdout is captured — agents answer questions fine). Use [Backend.RunAgent]
// when you instead want the agent to edit a file itself.
func (b Backend) Run(prompt string) (string, error) {
	if b.API != "" {
		base, keyEnv := resolveAPI(b.API)
		key := ""
		if keyEnv != "" {
			key = os.Getenv(keyEnv) // env var wins
			if key == "" {          // stored key is the set-and-forget fallback
				if cfg, err := LoadConfig(); err == nil {
					key = cfg.Key(keyEnv)
				}
			}
		}
		return runAPI(base, b.Model, key, prompt)
	}
	cmd := b.Cmd
	if cmd == "" {
		cmd = b.Agent // agents answer questions fine — capture their stdout
	}
	return runCommand(cmd, prompt)
}

// RunAgent hands task to the agent backend, which edits files itself, and
// returns the agent's stdout (its narration, if any).
func (b Backend) RunAgent(task string) (string, error) {
	return runCommand(b.Agent, task)
}

// RunAgentStreaming hands task to the agent backend and streams its output
// directly to os.Stdout/os.Stderr. Unlike RunAgent, this does not create
// internal pipes, so cmd.Wait returns as soon as the agent process exits —
// no hang waiting for orphaned child processes to release inherited handles.
func (b Backend) RunAgentStreaming(task string) error {
	parts := strings.Fields(b.Agent)
	if len(parts) == 0 {
		return fmt.Errorf("empty agent command")
	}
	viaArg := false
	for i, p := range parts {
		if p == "{}" {
			parts[i] = task
			viaArg = true
		}
	}
	c := exec.Command(parts[0], parts[1:]...)
	if !viaArg {
		c.Stdin = strings.NewReader(task)
	}
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

// Summarize gathers target and asks the backend for a 2–4 sentence gestalt of
// the deterministic digest (not the raw code — cheap and grounded). Returns an
// error if there is nothing to summarize or the backend has no choice set.
func Summarize(target, only string, b Backend) (string, error) {
	if b.None() {
		return "", fmt.Errorf("no AI backend chosen")
	}
	r, err := Gather(target, only)
	if err != nil {
		return "", err
	}
	if len(r.All) == 0 {
		return "", fmt.Errorf("nothing to summarize in %s", target)
	}
	out, err := b.Run(SummaryPrompt(r.IsDir, r.Pkg, r.All))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// SearchAI gathers the declarations under target (its subtree if recursive) and
// asks the backend which are most relevant to query, ranked, over the
// deterministic digest. Opt-in twice over: it reads only what you point it at,
// and it runs only because you supplied a backend.
func SearchAI(target, query string, recursive bool, b Backend) (string, error) {
	if b.None() {
		return "", fmt.Errorf("no AI backend chosen")
	}
	all, err := collect(target, recursive)
	if err != nil {
		return "", err
	}
	if len(all) == 0 {
		return "", fmt.Errorf("no Go declarations under %s", target)
	}
	out, err := b.Run(SearchPrompt(query, all))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// Explain finds a named declaration (or the one at a given line) in file and
// asks the backend for a plain-prose explanation of what it does and how it
// works — deeper than summary because it sends the actual source, not the digest.
// Pass name="" and line>0 to look up by line; pass name non-empty to look up by name.
func Explain(file, name string, line int, context string, b Backend) (string, error) {
	if b.None() {
		return "", fmt.Errorf("no AI backend chosen")
	}
	r, err := Gather(file, name)
	if err != nil {
		return "", err
	}
	var e *Entry
	if name != "" {
		if len(r.All) == 0 {
			return "", fmt.Errorf("no declaration %q in %s", name, file)
		}
		cp := r.All[0]
		e = &cp
	} else {
		for i := range r.All {
			if r.All[i].Line <= line {
				cp := r.All[i]
				e = &cp
			}
		}
		if e == nil {
			return "", fmt.Errorf("no declaration at or before line %d in %s", line, file)
		}
	}
	out, err := b.Run(ExplainPrompt(file, e.Name, e.Source, context))
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ExplainPrompt builds the prompt for explaining one declaration in depth.
// Unlike summary (which works from the digest), explain sends the full source.
func ExplainPrompt(file, name, source, context string) string {
	var b strings.Builder
	if lang := LangOf(file); lang != "" {
		fmt.Fprintf(&b, "Explain this %s declaration to a developer reading unfamiliar code.\n", lang)
	} else {
		b.WriteString("Explain this declaration to a developer reading unfamiliar code.\n")
	}
	b.WriteString("Write 3–6 sentences of plain prose: what it does, how it works, and when you would use it. Be specific to the actual code. No filler.\n\n")
	if context != "" {
		fmt.Fprintf(&b, "Surrounding code, for reference:\n%s\n\n", context)
	}
	fmt.Fprintf(&b, "%s:\n%s\n", name, source)
	return b.String()
}

// SearchPrompt asks a model to rank the digest by relevance to query. The model
// answers over the file:line digest, not the raw code, so it stays grounded and
// every hit it names is locatable.
func SearchPrompt(query string, all []Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "A developer is looking for: %s\n", query)
	b.WriteString("From the declarations below, list the ones that are actually relevant, most relevant first, each on its own line as `file:line — why it matches`. Use ONLY the list below; do not invent anything. If none are relevant, reply exactly: no matches.\n\n")
	for _, e := range all {
		fmt.Fprintf(&b, "%s:%d  %s %s — %s\n", e.File, e.Line, e.Kind, e.Name, orUndoc(e.What))
	}
	return b.String()
}

// SummaryPrompt builds the LLM prompt from the deterministic digest — the model
// summarizes the *digest*, not the raw code, so it's cheap and grounded.
func SummaryPrompt(isDir bool, pkg string, all []Entry) string {
	kind := "file"
	if isDir {
		kind = "package"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Summarize this Go %s for a developer orienting on unfamiliar code.\n", kind)
	b.WriteString("Using ONLY the declarations and doc-comments below, write 2-4 sentences: what it is for and how it is organized. Be concrete; do not invent details that aren't present.\n\n")
	if isDir && pkg != "" {
		fmt.Fprintf(&b, "package %s\n", pkg)
	}
	for _, e := range all {
		fmt.Fprintf(&b, "- %s %s — %s\n", e.Kind, e.Name, orUndoc(e.What))
	}
	return b.String()
}

// EditPrompt instructs a completion model to return ONLY the rewritten region.
// context, when non-empty, is surrounding code the model may consult but must
// not reproduce (see [BuildContext]).
func EditPrompt(file, region, instruction, context string) string {
	var b strings.Builder
	b.WriteString("You are a code-transformation function, NOT an assistant.\n")
	fmt.Fprintf(&b, "Apply the instruction to the %s region below and output ONLY the replacement code.\n", LangOf(file))
	b.WriteString("Hard rules: no explanation, no preamble, no commentary, no markdown fences — ")
	b.WriteString("output exactly what should replace the region and nothing else. ")
	b.WriteString("If the instruction means the code should be removed, output nothing at all.\n\n")
	if context != "" {
		fmt.Fprintf(&b, "Surrounding code, for reference only (do NOT reproduce it — output only the new region):\n%s\n\n", context)
	}
	fmt.Fprintf(&b, "Instruction: %s\n\nRegion:\n%s\n", instruction, region)
	return b.String()
}

// GeneratePrompt instructs a completion model to return ONLY a new file's
// contents. context, when non-empty, is existing code to match conventions
// against but not reproduce (see [BuildContext]).
func GeneratePrompt(file, instruction, context string) string {
	var b strings.Builder
	b.WriteString("You are a code-generation function, NOT an assistant.\n")
	if lang := LangOf(file); lang != "" {
		fmt.Fprintf(&b, "Write the complete contents of a new %s file named %s.\n", lang, filepath.Base(file))
	} else {
		fmt.Fprintf(&b, "Write the complete contents of a new file named %s.\n", filepath.Base(file))
	}
	b.WriteString("Hard rules: output ONLY the file's contents — no explanation, no preamble, no commentary, no markdown fences. ")
	b.WriteString("Output exactly what should be saved to the file and nothing else.\n\n")
	if context != "" {
		fmt.Fprintf(&b, "Existing project code, for reference (match its conventions; do NOT reproduce it):\n%s\n\n", context)
	}
	fmt.Fprintf(&b, "Instruction: %s\n", instruction)
	return b.String()
}

// AgentEditTask builds the instruction for an agent backend that edits the file
// itself. A blank region means an INSERTION at line a, not a transformation.
// context, when non-empty, is appended as background the agent may consult.
func AgentEditTask(file string, a, b int, region, instruction, context string) string {
	var task string
	if strings.TrimSpace(region) == "" {
		task = fmt.Sprintf("Edit the file %s in place. At line %d the content is blank — INSERT new code there, exactly at that location, per this instruction:\n\nInstruction: %s\n\nPlace the code at line %d; do not relocate it elsewhere and do not modify unrelated code. Save the file.",
			file, a, instruction, a)
	} else {
		task = fmt.Sprintf("Edit the file %s in place at lines %d-%d. The current code there is:\n\n%s\n\nApply exactly this change: %s\nModify only that region; leave unrelated code untouched. Save the file.",
			file, a, b, region, instruction)
	}
	return withContext(task, context)
}

// AgentGenerateTask builds the instruction for an agent backend that creates the
// new file itself rather than returning text to write.
func AgentGenerateTask(file, instruction, context string) string {
	task := fmt.Sprintf("Create a new file at %s per this instruction:\n\nInstruction: %s\n\nWrite only that one file; do not modify or create any other file. Save it.",
		file, instruction)
	return withContext(task, context)
}

// withContext appends reference context to an agent task, if any.
func withContext(task, context string) string {
	if context == "" {
		return task
	}
	return task + "\n\nFor reference, other code in scope:\n" + context
}

// LangOf names the language of file from its extension ("" if unknown).
func LangOf(file string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".go":
		return "Go"
	case ".py":
		return "Python"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".js", ".jsx":
		return "JavaScript"
	case ".rs":
		return "Rust"
	case ".c", ".h", ".cpp", ".cc":
		return "C/C++"
	case ".cs":
		return "C#"
	default:
		return ""
	}
}

// StripFences removes a leading ```lang and trailing ``` if a model added them.
func StripFences(s string) string {
	t := strings.TrimSpace(s)
	if !strings.HasPrefix(t, "```") {
		return s
	}
	ls := strings.Split(t, "\n")
	ls = ls[1:] // drop opening fence
	for len(ls) > 0 && strings.TrimSpace(ls[len(ls)-1]) == "```" {
		ls = ls[:len(ls)-1]
	}
	return strings.Join(ls, "\n")
}

// ParseRange parses "A:B" (1-based, inclusive) line numbers.
func ParseRange(s string) (int, int, error) {
	p := strings.SplitN(s, ":", 2)
	if len(p) != 2 {
		return 0, 0, fmt.Errorf("bad range %q (want A:B)", s)
	}
	a, e1 := strconv.Atoi(strings.TrimSpace(p[0]))
	b, e2 := strconv.Atoi(strings.TrimSpace(p[1]))
	if e1 != nil || e2 != nil {
		return 0, 0, fmt.Errorf("bad range %q (want A:B)", s)
	}
	return a, b, nil
}

// resolveAPI maps a preset (or a raw base URL) to a base URL + the env var that
// holds the API key. Empty keyEnv means no auth (local servers).
func resolveAPI(api string) (base, keyEnv string) {
	switch api {
	case "openai":
		return "https://api.openai.com/v1", "OPENAI_API_KEY"
	case "openrouter":
		return "https://openrouter.ai/api/v1", "OPENROUTER_API_KEY"
	case "ollama":
		return "http://localhost:11434/v1", ""
	default:
		return strings.TrimRight(api, "/"), "PITH_API_KEY"
	}
}

// runAPI calls an OpenAI-compatible /chat/completions endpoint and returns the
// message content.
func runAPI(base, model, key, prompt string) (string, error) {
	if model == "" {
		return "", fmt.Errorf("--api needs --model")
	}
	reqBody, _ := json.Marshal(map[string]any{
		"model":       model,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"temperature": 0,
	})
	req, err := http.NewRequest("POST", strings.TrimRight(base, "/")+"/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api %d: %s", resp.StatusCode, strings.TrimSpace(string(rb)))
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rb, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("api: no choices in response")
	}
	return out.Choices[0].Message.Content, nil
}

// runCommand pipes prompt to a backend command's stdin (or substitutes a {}
// placeholder argument) and returns its stdout.
func runCommand(cmdline, prompt string) (string, error) {
	parts := strings.Fields(cmdline)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty command")
	}
	viaArg := false
	for i, p := range parts {
		if p == "{}" {
			parts[i] = prompt
			viaArg = true
		}
	}
	c := exec.Command(parts[0], parts[1:]...)
	if !viaArg {
		c.Stdin = strings.NewReader(prompt)
	}
	var out, errb bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errb
	if err := c.Run(); err != nil {
		if s := strings.TrimSpace(errb.String()); s != "" {
			return "", fmt.Errorf("%v: %s", err, s)
		}
		return "", err
	}
	return out.String(), nil
}
