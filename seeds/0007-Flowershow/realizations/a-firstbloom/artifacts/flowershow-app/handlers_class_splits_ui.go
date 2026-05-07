package main

// PR: splits UI + mode toggle
// Server-side helpers for the class-splits UI: a fragment endpoint for
// refreshing only the splits subtree, plus template helper functions that
// expose split lookups to admin templates without having to thread the
// store through every render call.

import (
	"fmt"
	"log"
	"net/http"
	"sort"
)

// splitsRenderStore is set during app boot so the package-level template
// helpers (registered via init below) can resolve splits without needing a
// per-render handle on the *app instance.
var splitsRenderStore flowershowStore

// registerSplitsRenderStore lets main wire the runtime store after the app
// is constructed. Calling with nil is a no-op so tests can keep their own
// handles separate.
func registerSplitsRenderStore(store flowershowStore) {
	if store == nil {
		return
	}
	splitsRenderStore = store
}

func init() {
	templateFuncMap["splitsForClass"] = func(classID string) []*ClassSplit {
		if splitsRenderStore == nil || classID == "" {
			return nil
		}
		out := splitsRenderStore.classSplitsByClass(classID)
		// Defensive copy + sort so templates always see deterministic order.
		copied := make([]*ClassSplit, 0, len(out))
		for _, s := range out {
			if s != nil {
				copied = append(copied, s)
			}
		}
		sort.Slice(copied, func(i, j int) bool {
			if copied[i].SortOrder != copied[j].SortOrder {
				return copied[i].SortOrder < copied[j].SortOrder
			}
			return copied[i].SplitCode < copied[j].SplitCode
		})
		return copied
	}

	templateFuncMap["entriesInSplit"] = func(entries []*entryView, splitID string) []*entryView {
		out := make([]*entryView, 0, len(entries))
		for _, e := range entries {
			if e == nil || e.Entry == nil {
				continue
			}
			if e.Entry.SplitID == splitID {
				out = append(out, e)
			}
		}
		return out
	}

	templateFuncMap["entriesWithoutSplit"] = func(entries []*entryView) []*entryView {
		out := make([]*entryView, 0, len(entries))
		for _, e := range entries {
			if e == nil || e.Entry == nil {
				continue
			}
			if e.Entry.SplitID == "" {
				out = append(out, e)
			}
		}
		return out
	}

	templateFuncMap["splitDisplayLabel"] = func(s *ClassSplit) string {
		if s == nil {
			return ""
		}
		base := "Split " + s.SplitCode
		if s.Label != "" {
			return base + " · " + s.Label
		}
		return base
	}

	// dict assembles an ad-hoc map[string]any for template scoping.
	// This lets nested template invocations carry multiple named values
	// without inventing dedicated view structs for every fragment.
	if _, exists := templateFuncMap["dict"]; !exists {
		templateFuncMap["dict"] = func(values ...any) (map[string]any, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("dict requires an even number of arguments, got %d", len(values))
			}
			out := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings, got %T at position %d", values[i], i)
				}
				out[key] = values[i+1]
			}
			return out, nil
		}
	}
}

// handleAdminClassSplitsFragment returns the splits-aware grid for a single
// class as an HTML fragment. Callers can use this to refresh the class
// region after a mutation without reloading the entire intake panel.
func (a *app) handleAdminClassSplitsFragment(w http.ResponseWriter, r *http.Request) {
	classID := r.PathValue("classID")
	cls, ok := a.store.classByID(classID)
	if !ok {
		http.NotFound(w, r)
		return
	}
	showID := showIDForClass(a.store, classID)
	if showID == "" {
		http.NotFound(w, r)
		return
	}
	data, err := a.adminShowDetailData(showID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Find the matching boardClassView for the requested class.
	var target *boardClassView
	for _, div := range data.BoardDivisions {
		if div == nil {
			continue
		}
		for _, sec := range div.Sections {
			if sec == nil {
				continue
			}
			for _, candidate := range sec.Classes {
				if candidate != nil && candidate.Class != nil && candidate.Class.ID == cls.ID {
					target = candidate
					break
				}
			}
			if target != nil {
				break
			}
		}
		if target != nil {
			break
		}
	}
	if target == nil {
		http.NotFound(w, r)
		return
	}

	payload := map[string]any{
		"BasePath":   requestBasePath(r),
		"Show":       data.Show,
		"ClassEntry": target,
	}
	html, err := a.renderTemplateBlockForRequest(r, "show_admin.html", "admin_class_splits_fragment", payload)
	if err != nil {
		log.Printf("render admin class splits fragment %s: %v", classID, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}
