package pith

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Search returns the declarations under target that match query, most relevant
// first. Matching is deterministic: every whitespace-separated term of query
// must appear (case-insensitively) in a declaration's name, signature, or doc.
// No AI, no network. It reads only what you point it at — descending into
// subdirectories is opt-in via recursive, never automatic.
func Search(target, query string, recursive bool) ([]Entry, error) {
	all, err := collect(target, recursive)
	if err != nil {
		return nil, err
	}
	return rankMatches(all, query), nil
}

// collect gathers every declaration under target. With recursive it walks the
// subtree (skipping vendored, hidden, and testdata dirs); otherwise it reads
// just the file or the single directory's package. Shared by [Search] and
// [SearchAI] so the opt-in reading boundary is defined in one place.
func collect(target string, recursive bool) ([]Entry, error) {
	if !recursive {
		r, err := Gather(target, "")
		if err != nil {
			return nil, err
		}
		return r.All, nil
	}
	var all []Entry
	err := filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != target && skipDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if skipFile(d.Name()) {
			return nil
		}
		if _, ok := langFor(d.Name()); !ok {
			return nil
		}
		r, e := Gather(path, "")
		if e != nil {
			return nil // skip an unparseable file, keep walking
		}
		all = append(all, r.All...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return all, nil
}

// skipDir reports whether a directory should be pruned from a recursive walk
// (shared by search -r and map): vendored deps, build output, hidden dirs.
func skipDir(name string) bool {
	switch name {
	case "vendor", "node_modules", "testdata",
		"build", "dist", "target", "bin", "obj", "out":
		return true
	}
	return strings.HasPrefix(name, ".") // .git, .idea, …
}

// rankMatches keeps the entries that contain every term of query and orders
// them by where the terms hit: name beats signature beats doc, with an exact
// name match strongest. Ties break by file then line so output is stable and
// locatable.
func rankMatches(entries []Entry, query string) []Entry {
	terms := strings.Fields(strings.ToLower(query))
	if len(terms) == 0 {
		return nil
	}
	type scored struct {
		e     Entry
		score int
	}
	var hits []scored
	for _, e := range entries {
		name := strings.ToLower(e.Name)
		sig := strings.ToLower(e.Sig)
		doc := strings.ToLower(e.What)
		matchedAll, score := true, 0
		for _, t := range terms {
			switch {
			case name == t:
				score += 8
			case strings.Contains(name, t):
				score += 3
			case strings.Contains(sig, t):
				score += 2
			case strings.Contains(doc, t):
				score++
			default:
				matchedAll = false
			}
			if !matchedAll {
				break
			}
		}
		if matchedAll {
			hits = append(hits, scored{e, score})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		if hits[i].e.File != hits[j].e.File {
			return hits[i].e.File < hits[j].e.File
		}
		return hits[i].e.Line < hits[j].e.Line
	})
	out := make([]Entry, len(hits))
	for i, h := range hits {
		out[i] = h.e
	}
	return out
}
