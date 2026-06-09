// Package pith extracts the pith of Go code — each declaration's purpose,
// deterministically, with no AI and no network. It is the shared core behind
// the pith CLI, the editor integrations, and (later) the Pulp cell: parse a
// file or package into [Entry] values with [Gather], then render them to any
// [io.Writer]. The AI ops (summary, edit, generate) build on this digest and
// live in ai.go; they never run unless you hand them a backend.
package pith

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is one declaration's pith: where it is, what it is, what it's for.
type Entry struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Kind string `json:"kind"` // func | method | type
	Recv string `json:"recv,omitempty"`
	Name string `json:"name"`
	Sig  string `json:"sig"`
	What string `json:"what"` // first line of the doc comment; "" if undocumented
}

// Result is the parsed pith of a file or a package: the flat list of
// declarations plus the structure needed to render them grouped or by file.
type Result struct {
	Target string             // the file or dir that was read (for headers)
	Files  []string           // the .go files that contributed (one, if a file)
	ByFile map[string][]Entry // declarations per file
	All    []Entry            // every declaration, in file+source order
	Pkg    string             // the package name
	IsDir  bool               // true if Target was a directory
}

// Gather parses target (a Go file, or a directory of Go files) into a Result.
// If only is non-empty, just the declaration of that name is kept.
func Gather(target, only string) (Result, error) {
	fset := token.NewFileSet()
	r := Result{Target: target, ByFile: map[string][]Entry{}}

	info, statErr := os.Stat(target)
	r.IsDir = statErr == nil && info.IsDir()
	if r.IsDir {
		ents, e := os.ReadDir(target)
		if e != nil {
			return r, e
		}
		for _, en := range ents {
			n := en.Name()
			if en.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
				continue
			}
			r.Files = append(r.Files, filepath.Join(target, n))
		}
		sort.Strings(r.Files)
	} else {
		r.Files = []string{target}
	}

	for _, f := range r.Files {
		es, name, e := parseFile(fset, f, only)
		if e != nil {
			if !r.IsDir {
				return r, e
			}
			continue // skip a bad file inside a package, keep going
		}
		if r.Pkg == "" {
			r.Pkg = name
		}
		r.ByFile[f] = es
		r.All = append(r.All, es...)
	}
	return r, nil
}

// parseFile parses one Go file into declarations (optionally just `only`).
func parseFile(fset *token.FileSet, file, only string) (entries []Entry, pkg string, err error) {
	f, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		return nil, "", err
	}
	pkg = f.Name.Name
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			if only != "" && decl.Name.Name != only {
				continue
			}
			kind, recv, recvSig := "func", "", ""
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				kind = "method"
				recv = "(" + nodeStr(fset, decl.Recv.List[0].Type) + ")"
				recvSig = recv + " "
			}
			entries = append(entries, Entry{
				File: file,
				Line: fset.Position(decl.Pos()).Line,
				Kind: kind,
				Recv: recv,
				Name: decl.Name.Name,
				Sig:  recvSig + decl.Name.Name + sigStr(fset, decl.Type),
				What: firstLine(decl.Doc),
			})
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if only != "" && ts.Name.Name != only {
					continue
				}
				what := firstLine(ts.Doc)
				if what == "" {
					what = firstLine(decl.Doc)
				}
				entries = append(entries, Entry{
					File: file,
					Line: fset.Position(ts.Pos()).Line,
					Kind: "type",
					Name: ts.Name.Name,
					Sig:  "type " + ts.Name.Name,
					What: what,
				})
			}
		}
	}
	return entries, pkg, nil
}

// WriteText renders the human-facing purpose map to w: a package as
// file-sectioned groups, a single file as one header plus its groups.
func (r Result) WriteText(w io.Writer) {
	if r.IsDir {
		renderDir(w, r.Target, r.Files, r.ByFile, r.Pkg, len(r.All))
		return
	}
	fmt.Fprintf(w, "%s  (%d)\n\n", shortPath(r.Target), len(r.All))
	if len(r.All) == 0 {
		fmt.Fprintln(w, "  (no top-level declarations)")
		return
	}
	renderGroups(w, r.All)
}

// WriteGrep renders one line per declaration as "file:line: sig — what", the
// form editors and pipes (quickfix, ripgrep) consume.
func (r Result) WriteGrep(w io.Writer) {
	for _, e := range r.All {
		fmt.Fprintf(w, "%s:%d: %s — %s\n", e.File, e.Line, e.Sig, orUndoc(e.What))
	}
}

// WriteJSON renders the flat declaration list as indented JSON.
func (r Result) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r.All)
}

// renderDir prints a package header, then each file as a type-grouped section.
// File headers (▌ relpath) are how the editor jumps across files.
func renderDir(w io.Writer, dir string, files []string, byFile map[string][]Entry, pkg string, total int) {
	fmt.Fprintf(w, "package %s  (%d files, %d decls)\n", pkg, len(files), total)
	for _, f := range files {
		es := byFile[f]
		if len(es) == 0 {
			continue
		}
		rel, err := filepath.Rel(dir, f)
		if err != nil {
			rel = filepath.Base(f)
		}
		fmt.Fprintf(w, "\n▌ %s\n", filepath.ToSlash(rel))
		renderGroups(w, es)
	}
}

const funcsKey = "\x00funcs"

// renderGroups prints declarations grouped by type (each type with its
// methods), free functions last, blank line between groups.
func renderGroups(w io.Writer, entries []Entry) {
	gmap := map[string]*group{}
	var order []string
	get := func(key string, line int) *group {
		g := gmap[key]
		if g == nil {
			g = &group{key: key, first: line}
			gmap[key] = g
			order = append(order, key)
		}
		if line < g.first {
			g.first = line
		}
		return g
	}
	for _, e := range entries {
		switch e.Kind {
		case "type":
			ec := e
			get(e.Name, e.Line).typ = &ec
		case "method":
			g := get(recvBase(e.Recv), e.Line)
			g.rows = append(g.rows, e)
		default:
			g := get(funcsKey, e.Line)
			g.rows = append(g.rows, e)
		}
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := order[i], order[j]
		if a == funcsKey {
			return false
		}
		if b == funcsKey {
			return true
		}
		return gmap[a].first < gmap[b].first
	})

	for gi, key := range order {
		g := gmap[key]
		switch {
		case key == funcsKey:
			fmt.Fprintln(w, "functions")
		case g.typ != nil:
			fmt.Fprintf(w, "type %s — %s\n", g.typ.Name, orUndoc(g.typ.What))
		default:
			recv := key
			if len(g.rows) > 0 {
				recv = g.rows[0].Recv
			}
			fmt.Fprintln(w, recv)
		}
		width := 0
		for _, e := range g.rows {
			if n := len(e.Name); n > width {
				width = n
			}
		}
		if width > 28 {
			width = 28
		}
		sort.SliceStable(g.rows, func(i, j int) bool { return g.rows[i].Line < g.rows[j].Line })
		for _, e := range g.rows {
			fmt.Fprintf(w, "  %5d  %-*s   %s\n", e.Line, width, trunc(e.Name, width), trunc(orUndoc(e.What), 88))
		}
		if gi < len(order)-1 {
			fmt.Fprintln(w)
		}
	}
}

// group is a type and its methods (or the free-function bucket).
type group struct {
	key   string
	first int
	typ   *Entry
	rows  []Entry
}

// recvBase reduces a receiver like "(*state[T])" to its bare type name "state".
func recvBase(recv string) string {
	s := strings.Trim(recv, "()")
	s = strings.TrimPrefix(s, "*")
	if i := strings.IndexByte(s, '['); i >= 0 {
		s = s[:i]
	}
	return s
}

func orUndoc(s string) string {
	if s == "" {
		return "(undocumented)"
	}
	return s
}

// firstLine returns the first line of a doc comment, trimmed. "" if none.
func firstLine(cg *ast.CommentGroup) string {
	if cg == nil {
		return ""
	}
	t := strings.TrimSpace(cg.Text())
	if i := strings.IndexByte(t, '\n'); i >= 0 {
		t = t[:i]
	}
	return strings.TrimSpace(t)
}

// sigStr renders a function type as "(params) results" (drops the "func").
func sigStr(fset *token.FileSet, ft *ast.FuncType) string {
	return strings.TrimPrefix(nodeStr(fset, ft), "func")
}

// nodeStr printer-prints any AST node to a single whitespace-collapsed line.
func nodeStr(fset *token.FileSet, n ast.Node) string {
	var b strings.Builder
	_ = printer.Fprint(&b, fset, n)
	return strings.Join(strings.Fields(b.String()), " ")
}

// shortPath keeps the last two path segments (parent/file) for the header.
func shortPath(p string) string {
	p = filepath.ToSlash(p)
	parts := strings.Split(p, "/")
	if len(parts) >= 2 {
		return strings.Join(parts[len(parts)-2:], "/")
	}
	return p
}

// trunc shortens s to n runes with an ellipsis.
func trunc(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
