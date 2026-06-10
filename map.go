package pith

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MapPackage is one directory's row in the repo map: where it is, how big it
// is, and (when an AI pass has run) what it is for in one line.
type MapPackage struct {
	Dir     string `json:"dir"` // relative to the map root; "." is the root itself
	Pkg     string `json:"pkg,omitempty"`
	Files   int    `json:"files"`
	Decls   int    `json:"decls"`
	Hash    string `json:"hash"` // content hash of the package digest, for the purpose cache
	Purpose string `json:"purpose,omitempty"`

	entries []Entry // kept for the AI prompt; not serialized
}

// Map walks root and returns one MapPackage per directory that contains
// recognized code — the top rung of the zoom ladder (map → read → explain).
// Deterministic, no AI, no network: just the tree, sizes, and content hashes.
// Purposes are filled separately by [MapAI], which is opt-in like every AI op.
func Map(root string) ([]MapPackage, error) {
	dirSet := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && skipDir(d.Name()) {
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
		dirSet[filepath.Dir(path)] = true
		return nil
	})
	if err != nil {
		return nil, err
	}

	dirs := make([]string, 0, len(dirSet))
	for d := range dirSet {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)

	var pkgs []MapPackage
	for _, dir := range dirs {
		r, e := Gather(dir, "")
		if e != nil {
			continue // an unreadable directory shouldn't sink the whole map
		}
		rel, e := filepath.Rel(root, dir)
		if e != nil {
			rel = dir
		}
		sum := sha256.Sum256([]byte(digestEntries(r.All)))
		pkgs = append(pkgs, MapPackage{
			Dir:     filepath.ToSlash(rel),
			Pkg:     r.Pkg,
			Files:   len(r.Files),
			Decls:   len(r.All),
			Hash:    hex.EncodeToString(sum[:6]),
			entries: r.All,
		})
	}
	return pkgs, nil
}

// MapAI fills each package's one-line Purpose, asking the backend only for
// packages whose content hash isn't already in the cache (.pith-map.json at
// root) — so a repo is described once and then stays free until code changes.
func MapAI(root string, pkgs []MapPackage, b Backend) error {
	if b.None() {
		return fmt.Errorf("no AI backend chosen")
	}
	cachePath := filepath.Join(root, ".pith-map.json")
	cache := loadMapCache(cachePath)
	changed := false
	for i := range pkgs {
		if c, ok := cache[pkgs[i].Dir]; ok && c.Hash == pkgs[i].Hash {
			pkgs[i].Purpose = c.Purpose
			continue
		}
		if len(pkgs[i].entries) == 0 {
			continue
		}
		fmt.Fprintf(os.Stderr, "pith: describing %s…\n", pkgs[i].Dir)
		out, err := b.Run(MapPrompt(pkgs[i].Pkg, pkgs[i].entries))
		if err != nil {
			return err
		}
		purpose := strings.TrimSpace(out)
		if j := strings.IndexByte(purpose, '\n'); j >= 0 {
			purpose = strings.TrimSpace(purpose[:j])
		}
		pkgs[i].Purpose = purpose
		cache[pkgs[i].Dir] = mapCacheEntry{Hash: pkgs[i].Hash, Purpose: purpose}
		changed = true
	}
	if changed {
		saveMapCache(cachePath, cache)
	}
	return nil
}

// MapPrompt asks for a single-line purpose of one package, grounded in its
// deterministic digest — the model never sees raw code, just the decl list.
func MapPrompt(pkg string, entries []Entry) string {
	var b strings.Builder
	b.WriteString("In ONE line of at most 12 words, state what this code package is for, based only on its declarations below. Output only that line — no preamble, no quotes.\n\n")
	if pkg != "" {
		fmt.Fprintf(&b, "package %s\n", pkg)
	}
	for _, e := range entries {
		fmt.Fprintf(&b, "- %s %s — %s\n", e.Kind, e.Name, orUndoc(e.What))
	}
	return b.String()
}

// WriteMapText renders the map as aligned columns: dir, package, sizes, and
// the one-line purpose when present.
func WriteMapText(w io.Writer, root string, pkgs []MapPackage) {
	total := 0
	dw, pw := 0, 0
	for _, p := range pkgs {
		total += p.Decls
		if len(p.Dir) > dw {
			dw = len(p.Dir)
		}
		if len(p.Pkg) > pw {
			pw = len(p.Pkg)
		}
	}
	if dw > 40 {
		dw = 40
	}
	fmt.Fprintf(w, "%s  (%d packages, %d decls)\n\n", shortPath(root), len(pkgs), total)
	for _, p := range pkgs {
		fmt.Fprintf(w, "%-*s  %-*s %4d decls %3d files", dw, trunc(p.Dir, dw), pw, p.Pkg, p.Decls, p.Files)
		if p.Purpose != "" {
			fmt.Fprintf(w, "   — %s", p.Purpose)
		}
		fmt.Fprintln(w)
	}
}

// WriteMapJSON renders the map rows as indented JSON.
func WriteMapJSON(w io.Writer, pkgs []MapPackage) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(pkgs)
}

// mapCacheEntry is one cached package purpose, valid while its hash matches.
type mapCacheEntry struct {
	Hash    string `json:"hash"`
	Purpose string `json:"purpose"`
}

func loadMapCache(path string) map[string]mapCacheEntry {
	cache := map[string]mapCacheEntry{}
	b, err := os.ReadFile(path)
	if err != nil {
		return cache
	}
	_ = json.Unmarshal(b, &cache) // a corrupt cache just means re-asking
	return cache
}

func saveMapCache(path string, cache map[string]mapCacheEntry) {
	b, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, b, 0o644) // cache is best-effort; never fail the op
}
