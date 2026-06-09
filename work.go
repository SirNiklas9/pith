package pith

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// WorkItem is one tracked piece of work: a note, optionally anchored to a
// file:line you mean to come back to, with a stable id and a done flag.
type WorkItem struct {
	ID    int       `json:"id"`
	File  string    `json:"file,omitempty"`
	Line  int       `json:"line,omitempty"`
	Note  string    `json:"note"`
	Done  bool      `json:"done"`
	Added time.Time `json:"added"`
}

// WorkList is a persistent set of [WorkItem]s. It is the deterministic, no-AI
// "work" feature: mark spots you owe yourself, list them, close them out. Load
// it with [LoadWork], mutate, then [WorkList.Save].
type WorkList struct {
	NextID int        `json:"next_id"`
	Items  []WorkItem `json:"items"`

	path string // where this list lives; set by LoadWork, not serialized
}

// DefaultWorkPath is .pith-work.json at the enclosing git root (so the list is
// the same from any subdirectory of a repo), or in the working directory if no
// repo encloses it.
func DefaultWorkPath() string {
	dir, err := os.Getwd()
	if err != nil {
		return ".pith-work.json"
	}
	for d := dir; ; {
		if fi, err := os.Stat(filepath.Join(d, ".git")); err == nil && fi.IsDir() {
			return filepath.Join(d, ".pith-work.json")
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return filepath.Join(dir, ".pith-work.json")
}

// LoadWork reads the work list at path, or returns an empty one (ready to Save)
// if the file does not exist yet.
func LoadWork(path string) (*WorkList, error) {
	w := &WorkList{NextID: 1, path: path}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return w, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, w); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	w.path = path
	if w.NextID < 1 {
		w.NextID = 1
	}
	return w, nil
}

// Save writes the list back to its path as indented JSON.
func (w *WorkList) Save() error {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(w.path, append(data, '\n'), 0o644)
}

// Add appends a new item (file/line may be empty/0 for an unanchored note) and
// returns it with its assigned id.
func (w *WorkList) Add(file string, line int, note string) WorkItem {
	it := WorkItem{ID: w.NextID, File: file, Line: line, Note: note, Added: time.Now()}
	w.NextID++
	w.Items = append(w.Items, it)
	return it
}

// Done marks the item with the given id done; it reports whether one matched.
func (w *WorkList) Done(id int) bool {
	for i := range w.Items {
		if w.Items[i].ID == id {
			w.Items[i].Done = true
			return true
		}
	}
	return false
}

// Remove deletes the item with the given id; it reports whether one matched.
func (w *WorkList) Remove(id int) bool {
	for i := range w.Items {
		if w.Items[i].ID == id {
			w.Items = append(w.Items[:i], w.Items[i+1:]...)
			return true
		}
	}
	return false
}

// ClearDone drops every done item and returns how many were removed.
func (w *WorkList) ClearDone() int {
	kept := w.Items[:0]
	removed := 0
	for _, it := range w.Items {
		if it.Done {
			removed++
			continue
		}
		kept = append(kept, it)
	}
	w.Items = kept
	return removed
}

// Render writes the list to out: one item per line as "[ ] #id file:line: note"
// (an x marks done). Anchored items lead with file:line so editors and quickfix
// can jump to them. Done items are shown only when includeDone is set.
func (w *WorkList) Render(out io.Writer, includeDone bool) {
	shown := 0
	for _, it := range w.Items {
		if it.Done && !includeDone {
			continue
		}
		shown++
		mark := " "
		if it.Done {
			mark = "x"
		}
		loc := ""
		if it.File != "" {
			loc = fmt.Sprintf("%s:%d: ", it.File, it.Line)
		}
		fmt.Fprintf(out, "[%s] #%-3d %s%s\n", mark, it.ID, loc, it.Note)
	}
	if shown == 0 {
		fmt.Fprintln(out, "no open work")
	}
}

// WriteJSON writes the items as indented JSON (for editor integrations).
func (w *WorkList) WriteJSON(out io.Writer) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(w.Items)
}
