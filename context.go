package pith

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BuildContext returns extra code context for an AI op at the requested scope,
// or "" for "" / "none". Scopes widen from the declaration outward:
//
//	around   the current file's outline (names + purposes, no bodies)
//	file     the current file's full source
//	dir      the folder's outline
//	project  the whole project's outline, from the enclosing git root
//
// Outlines are the deterministic digest, so even a whole project stays small.
// Nothing is read unless a scope is asked for — context is opt-in, every time.
func BuildContext(file, level string) (string, error) {
	switch level {
	case "", "none":
		return "", nil
	case "around":
		return digestOf(file)
	case "file":
		b, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return string(b), nil
	case "dir":
		dir := filepath.Dir(file)
		if dir == "" {
			dir = "."
		}
		return digestOf(dir)
	case "project":
		es, err := collect(projectRoot(file), true)
		if err != nil {
			return "", err
		}
		return digestEntries(es), nil
	default:
		return "", fmt.Errorf("unknown --context %q (want around|file|dir|project)", level)
	}
}

// digestOf gathers target and renders its declarations as the file:line digest.
func digestOf(target string) (string, error) {
	r, err := Gather(target, "")
	if err != nil {
		return "", err
	}
	return digestEntries(r.All), nil
}

// digestEntries renders declarations as "file:line: sig — what" lines.
func digestEntries(es []Entry) string {
	var b strings.Builder
	for _, e := range es {
		fmt.Fprintf(&b, "%s:%d: %s — %s\n", e.File, e.Line, e.Sig, orUndoc(e.What))
	}
	return b.String()
}

// projectRoot returns the enclosing git root of file, or its directory if none.
func projectRoot(file string) string {
	dir := filepath.Dir(file)
	if dir == "" {
		dir = "."
	}
	for d := dir; ; {
		if fi, err := os.Stat(filepath.Join(d, ".git")); err == nil && fi.IsDir() {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return dir
		}
		d = parent
	}
}
