package pith

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	ts "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// langFor resolves a file's tree-sitter language from its name, or false when
// no bundled grammar matches the extension/filename.
func langFor(file string) (*ts.Language, bool) {
	entry := grammars.DetectLanguage(filepath.Base(file))
	if entry == nil || entry.Language == nil {
		return nil, false
	}
	lang := entry.Language()
	return lang, lang != nil
}

// declsFromSource extracts a file's declarations with tree-sitter, uniformly
// across every grammar pith bundles. It walks the tree top-down: types and
// their methods are grouped (a method carries its enclosing type as a receiver),
// function bodies are not entered, and namespaces/wrappers stay transparent.
// Extraction is structural rather than tags-query based, because many grammars
// ship only an inferred tags query that omits type definitions.
func declsFromSource(file string, src []byte) (entries []Entry, pkg string, err error) {
	lang, ok := langFor(file)
	if !ok {
		return nil, "", fmt.Errorf("no bundled grammar for %s", filepath.Ext(file))
	}
	tree, err := ts.NewParser(lang).Parse(src)
	if err != nil {
		return nil, "", err
	}
	root := tree.RootNode()
	pkg = packageName(root, src, lang)

	var out []Entry
	emit := func(c *ts.Node, kind, recv string) {
		name := c.ChildByFieldName("name", lang)
		if name == nil {
			return // anonymous/wrapper node (function_type, struct_type, …)
		}
		e := Entry{
			File:   file,
			Line:   int(c.StartPoint().Row) + 1,
			Kind:   kind,
			Name:   name.Text(src),
			Sig:    firstLineTrim(c.Text(src)),
			What:   docAbove(c, src, lang),
			Source: c.Text(src),
		}
		if kind == "method" && recv != "" {
			e.Recv = "(" + recv + ")"
		}
		out = append(out, e)
	}

	var walk func(node *ts.Node, enclosing string)
	walk = func(node *ts.Node, enclosing string) {
		for i := 0; i < node.NamedChildCount(); i++ {
			c := node.NamedChild(i)
			t := c.Type(lang)
			switch {
			case isFuncNode(t):
				if c.ChildByFieldName("name", lang) == nil {
					walk(c, enclosing) // e.g. function_type signature — descend
					continue
				}
				recv := enclosing
				if r := c.ChildByFieldName("receiver", lang); r != nil {
					recv = goReceiverType(r.Text(src)) // Go: receiver param, not nesting
				}
				if recv != "" {
					emit(c, "method", recv)
				} else {
					emit(c, "func", "")
				}
				// deliberately do NOT descend into a function body
			case isTypeNode(t):
				emit(c, "type", "")
				next := enclosing
				if name := c.ChildByFieldName("name", lang); name != nil {
					next = name.Text(src)
				}
				walk(c, next) // nested methods take this type as their receiver
			case strings.Contains(t, "impl"): // Rust: impl Foo { fn … }
				recv := enclosing
				if ty := c.ChildByFieldName("type", lang); ty != nil {
					recv = baseTypeName(ty.Text(src))
				}
				walk(c, recv)
			default:
				walk(c, enclosing) // namespaces, exports, statements — transparent
			}
		}
	}
	walk(root, "")
	sort.SliceStable(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out, pkg, nil
}

// isFuncNode matches a function/method definition node across grammars. The
// name-field guard in emit filters out keyword-bearing non-definitions
// (function_type, call_expression, …).
func isFuncNode(t string) bool {
	return containsAny(t, "function", "method", "constructor", "subroutine", "procedure")
}

// isTypeNode matches a type/class/struct/enum/… definition node across grammars.
func isTypeNode(t string) bool {
	return containsAny(t, "class", "struct", "interface", "enum", "trait", "union",
		"record", "type_spec", "type_alias", "type_definition", "type_item",
		"object_declaration", "protocol", "message")
}

// firstLineTrim takes a declaration's first source line as its signature,
// dropping a trailing block-opener so "func F(x int) error {" and Python's
// "def f(x):" read as clean signatures.
func firstLineTrim(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	for {
		t := strings.TrimRight(strings.TrimRight(s, " \t"), "{:")
		if t == s {
			return strings.TrimSpace(t)
		}
		s = t
	}
}

// docAbove returns the first line of the contiguous comment block preceding a
// declaration — the language-agnostic stand-in for a doc comment. It climbs
// through single-child wrappers (e.g. Go's type_declaration around type_spec) so
// a comment written above the wrapper still attaches to the declaration inside.
func docAbove(node *ts.Node, src []byte, lang *ts.Language) string {
	for n := node; n != nil; {
		if c := precedingComment(n, src, lang); c != "" {
			return c
		}
		p := n.Parent()
		if p == nil {
			break
		}
		if fc := p.NamedChild(0); fc == nil || fc.StartByte() != n.StartByte() {
			break // n is not its parent's head child — don't borrow an outer comment
		}
		n = p
	}
	return ""
}

// precedingComment returns the first line of the contiguous comment block that
// directly precedes node among its siblings, or "".
func precedingComment(node *ts.Node, src []byte, lang *ts.Language) string {
	first := ""
	for p := node.PrevSibling(); p != nil; p = p.PrevSibling() {
		if !strings.Contains(p.Type(lang), "comment") {
			break
		}
		first = p.Text(src) // keep climbing; the topmost contiguous comment leads
	}
	return cleanComment(first)
}

// cleanComment strips common comment markers and returns the first line.
func cleanComment(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	for _, p := range []string{"/**", "/*", "///", "//", "#", "--", ";;", ";"} {
		if rest, found := strings.CutPrefix(s, p); found {
			s = rest
			break
		}
	}
	s = strings.TrimSuffix(s, "*/")
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "* ")
	return strings.TrimSpace(s)
}

// goReceiverType reduces a Go receiver like "(r *Repo[T])" to "Repo".
func goReceiverType(s string) string {
	s = strings.Trim(s, "()")
	s = strings.TrimSpace(s)
	if i := strings.LastIndexByte(s, ' '); i >= 0 {
		s = s[i+1:] // drop the receiver variable name
	}
	return baseTypeName(s)
}

// baseTypeName strips a leading pointer and any generic parameters.
func baseTypeName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "*")
	if i := strings.IndexByte(s, '['); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '<'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// packageName returns the name of a top-level package/namespace/module
// declaration if the source has one, else "".
func packageName(root *ts.Node, src []byte, lang *ts.Language) string {
	for i := 0; i < root.NamedChildCount(); i++ {
		n := root.NamedChild(i)
		if !containsAny(n.Type(lang), "package", "namespace", "module") {
			continue
		}
		if nm := n.ChildByFieldName("name", lang); nm != nil {
			return nm.Text(src)
		}
		for j := 0; j < n.NamedChildCount(); j++ { // Go: package_identifier child
			if c := n.NamedChild(j); containsAny(c.Type(lang), "identifier", "name") {
				return c.Text(src)
			}
		}
	}
	return ""
}

// skipFile reports whether a directory entry should be left out of a package
// read: hidden files, tests, and data/markup/lock files that are not code.
func skipFile(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	if containsAny(name, "_test.", ".test.", ".spec.") {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json", ".yaml", ".yml", ".toml", ".md", ".txt", ".csv",
		".lock", ".sum", ".mod", ".xml", ".html", ".htm", ".css", ".scss":
		return true
	}
	return false
}

// containsAny reports whether s contains any of subs.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
