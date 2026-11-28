package host

import (
	"log"
	"os"

	"github.com/adams-shaun/gorge/server/protocol"
)

// prunable reports whether retention may delete sc: a finished or aborted
// all-bot match of a table configured without humans and without a draft
// event. A human seat in the stored record, a crashed (or live) match, an
// event table and a table configured with a human seat are all retained
// regardless of Options.KeepMatches.
func prunable(c TableConfig, sc sidecar) bool {
	if len(c.Humans) > 0 || c.Event != "" {
		return false
	}
	if sc.State != protocol.MatchFinished && sc.State != protocol.MatchAborted {
		return false
	}
	for _, s := range sc.info().Seats {
		if s.Human {
			return false
		}
	}
	return true
}

// prune deletes the oldest prunable matches beyond the newest
// Options.KeepMatches prunable ones, from the table's in-memory match index
// and from disk. It is a no-op without persistence or with a non-positive
// limit. Non-prunable entries (human, crashed, event) are always kept, and
// match numbers are never reused: t.k is untouched.
func (r *Registry) prune(t *table) {
	keep := r.opts.KeepMatches
	if r.opts.Dir == "" || keep <= 0 {
		return
	}
	t.mu.Lock()
	total := 0
	for _, sc := range t.archived {
		if prunable(t.cfg, sc) {
			total++
		}
	}
	drop := total - keep
	if drop <= 0 {
		t.mu.Unlock()
		return
	}
	kept := make([]sidecar, 0, len(t.archived)-drop)
	removed := make([]sidecar, 0, drop)
	for _, sc := range t.archived {
		if drop > 0 && prunable(t.cfg, sc) {
			removed = append(removed, sc)
			drop--
			continue
		}
		kept = append(kept, sc)
	}
	t.archived = kept
	if t.loaded != nil {
		for _, sc := range removed {
			if t.loaded.k == sc.Match {
				t.loaded = nil
				break
			}
		}
	}
	t.mu.Unlock()

	// The index is pruned even if a delete fails; a failure is reported once
	// per call (the walk always continues) so one unreadable file cannot
	// flood the log every archive.
	logged := false
	for _, sc := range removed {
		for _, ext := range [...]string{".json", ".events", ".intents"} {
			if err := os.Remove(matchPath(r.opts.Dir, t.cfg.ID, sc.Match, ext)); err != nil && !os.IsNotExist(err) && !logged {
				log.Printf("host: prune %s match %d: %v", t.cfg.ID, sc.Match, err)
				logged = true
			}
		}
	}
}
