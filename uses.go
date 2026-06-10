package pith

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ts "github.com/odvcencio/gotreesitter"
)

// Caps that keep relational context bounded no matter how identifier-dense a
// selection is: outlines stay one line each, full sources degrade to outline
// lines past the budget instead of flooding the prompt.
const (
	usesMaxDecls       = 40
	usesMaxSourceBytes = 24 * 1024
)

// BuildContextRegion returns context for an AI op on file lines a:b. The
// positional levels delegate to [BuildContext]; the relational "uses" levels
// send only the declarations the selected region actually references —
// the least context with the most meaning:
//
//	uses            outlines of referenced decls, resolved within this file
//	uses:dir        … resolved across the file's folder
//	uses:project    … resolved across the enclosing git root
//	uses:dir:3      … and what THOSE declarations reference, 3 hops out
//
// A numeric segment sets the hop depth (default 1): depth 2 parses each
// matched declaration's own body and resolves what it references too, breadth
// first, so the closest code always survives the caps. Append :full to send
// full source instead of outlines — real implementations in the project's own
// style, but only the relevant ones. Like every context level this is opt-in
// per invocation, and never resolves beyond the scope named.
func BuildContextRegion(file, level string, a, b int) (string, error) {
	if !strings.HasPrefix(level, "uses") {
		return BuildContext(file, level)
	}
	decls, fullDepth, err := UsesClosure(file, level, a, b)
	if err != nil || len(decls) == 0 {
		return "", err
	}

	// Detail falloff: hops within fullDepth get real source (until the byte
	// budget runs out), everything farther gets an outline line — degrading
	// beats silently dropping or silently flooding.
	var sb strings.Builder
	budget := usesMaxSourceBytes
	for _, d := range decls {
		if d.Hop <= fullDepth && budget >= len(d.Source) {
			fmt.Fprintf(&sb, "// %s:%d\n%s\n\n", d.File, d.Line, d.Source)
			budget -= len(d.Source)
		} else {
			fmt.Fprintf(&sb, "%s:%d: %s — %s\n", d.File, d.Line, d.Sig, orUndoc(d.What))
		}
	}
	return sb.String(), nil
}

// UsedDecl is one declaration a uses level pulled in, tagged with how many
// reference hops from the selection it sits (1 = directly referenced).
type UsedDecl struct {
	Entry
	Hop int
}

// UsesClosure resolves a uses level to its declarations without rendering —
// the shared engine behind [BuildContextRegion] and the dry-run report.
// fullDepth is the detail falloff boundary: hops ≤ fullDepth render as source.
func UsesClosure(file, level string, a, b int) (decls []UsedDecl, fullDepth int, err error) {
	scope, depth, fullDepth, err := parseUsesLevel(level)
	if err != nil {
		return nil, 0, err
	}

	idents, err := identifiersInRegion(file, a, b)
	if err != nil {
		return nil, fullDepth, err
	}
	if len(idents) == 0 {
		return nil, fullDepth, nil
	}

	pool, err := usesPool(file, scope)
	if err != nil {
		return nil, fullDepth, err
	}

	target := filepath.Clean(file)
	seen := map[string]bool{}     // file:line — never include a decl twice
	resolved := map[string]bool{} // names already chased — cycle/refan guard

	// Breadth-first closure: round 1 is the selection's own references; each
	// further round parses the previous round's declarations and follows what
	// they reference. BFS order means the caps always cut the farthest hops.
	frontier := idents
	for hop := 1; hop <= depth && len(frontier) > 0 && len(decls) < usesMaxDecls; hop++ {
		var round []UsedDecl
		for _, e := range pool {
			if !frontier[e.Name] {
				continue
			}
			// the selection itself is already in the prompt — don't echo it back
			if filepath.Clean(e.File) == target && e.Line >= a && e.Line <= b {
				continue
			}
			key := fmt.Sprintf("%s:%d", filepath.Clean(e.File), e.Line)
			if seen[key] {
				continue
			}
			seen[key] = true
			round = append(round, UsedDecl{Entry: e, Hop: hop})
		}
		for n := range frontier {
			resolved[n] = true
		}
		sort.SliceStable(round, func(i, j int) bool {
			if round[i].File != round[j].File {
				return round[i].File < round[j].File
			}
			return round[i].Line < round[j].Line
		})
		decls = append(decls, round...)

		if hop >= depth {
			break
		}
		next := map[string]bool{}
		for _, d := range round {
			endLine := d.Line + strings.Count(d.Source, "\n")
			ids, err := identifiersInRegion(d.File, d.Line, endLine)
			if err != nil {
				continue // an unparseable hop shouldn't sink the whole closure
			}
			for n := range ids {
				if !resolved[n] {
					next[n] = true
				}
			}
		}
		frontier = next
	}

	if len(decls) > usesMaxDecls {
		decls = decls[:usesMaxDecls]
	}
	return decls, fullDepth, nil
}

// EstimateTokens converts a byte count to an approximate LLM token count using
// the code-typical ~4 bytes/token ratio. Deterministic and vendor-agnostic by
// design; exact counts are tokenizer-specific and only the vendor can give them.
func EstimateTokens(bytes int) int {
	return (bytes + 3) / 4
}

// EstimateTokensRange brackets a byte count across mainstream tokenizers:
// on code they cluster between ~4.5 bytes/token (efficient, e.g. o200k) and
// ~3 bytes/token (less code-dense). The true count for any major model falls
// inside [low, high] — accuracy for every backend with no per-model config.
func EstimateTokensRange(bytes int) (low, high int) {
	return bytes * 2 / 9, (bytes + 2) / 3
}

// parseUsesLevel splits a uses level into scope, hop depth, and full depth.
// Depth: 1-9 or "all" (exhaust the chain — the scope is finite, so it
// terminates). fullDepth is the detail falloff: hops ≤ fullDepth send real
// source, farther hops send outlines — relevance decays with distance, so
// detail does too. ":full" = full everywhere, ":fullN" = full to hop N.
func parseUsesLevel(level string) (scope string, depth, fullDepth int, err error) {
	parts := strings.Split(level, ":")
	if parts[0] != "uses" {
		return "", 0, 0, fmt.Errorf("unknown --context %q", level)
	}
	scope, depth, fullDepth = "file", 1, 0
	for _, p := range parts[1:] {
		switch {
		case p == "dir" || p == "project":
			scope = p
		case p == "all":
			depth = usesMaxDecls // closure is capped there anyway
		case p == "full":
			fullDepth = usesMaxDecls
		case strings.HasPrefix(p, "full") && len(p) == 5 && p[4] >= '1' && p[4] <= '9':
			fullDepth = int(p[4] - '0')
		case len(p) == 1 && p[0] >= '1' && p[0] <= '9':
			depth = int(p[0] - '0')
		default:
			return "", 0, 0, fmt.Errorf("unknown --context %q (want uses[:dir|:project][:N|:all][:full|:fullN])", level)
		}
	}
	return scope, depth, fullDepth, nil
}

// usesPool gathers the declarations a uses level may resolve against.
func usesPool(file, scope string) ([]Entry, error) {
	switch scope {
	case "file":
		r, err := Gather(file, "")
		if err != nil {
			return nil, err
		}
		return r.All, nil
	case "dir":
		dir := filepath.Dir(file)
		if dir == "" {
			dir = "."
		}
		r, err := Gather(dir, "")
		if err != nil {
			return nil, err
		}
		return r.All, nil
	default: // project
		return collect(projectRoot(file), true)
	}
}

// identifiersInRegion parses file and returns the set of identifier names that
// appear in lines a:b (1-based, inclusive). Deterministic: a tree-sitter leaf
// walk, no AI, no heuristics beyond "is this node an identifier".
func identifiersInRegion(file string, a, b int) (map[string]bool, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	lang, ok := langFor(file)
	if !ok {
		return nil, fmt.Errorf("no bundled grammar for %s", filepath.Ext(file))
	}
	tree, err := ts.NewParser(lang).Parse(src)
	if err != nil {
		return nil, err
	}

	idents := map[string]bool{}
	var walk func(node *ts.Node)
	walk = func(node *ts.Node) {
		for i := 0; i < node.NamedChildCount(); i++ {
			c := node.NamedChild(i)
			row := int(c.StartPoint().Row) + 1
			endRow := int(c.EndPoint().Row) + 1
			if endRow < a || row > b {
				continue // subtree entirely outside the selection
			}
			if c.NamedChildCount() == 0 {
				if strings.Contains(c.Type(lang), "identifier") && row >= a && row <= b {
					idents[c.Text(src)] = true
				}
				continue
			}
			walk(c)
		}
	}
	walk(tree.RootNode())
	return idents, nil
}
