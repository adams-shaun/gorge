package host

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/protocol"
)

// load reads tables.json and every sidecar. A match still marked live was
// cut off by a crash or kill: it is rewritten as aborted (spec: restart
// aborts in-progress matches; resume is M5). Perpetual tables are left
// ready for StartAll to begin match k+1.
func (r *Registry) load() error {
	raw, err := os.ReadFile(filepath.Join(r.opts.Dir, "tables.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var tf tablesFile
	if err := json.Unmarshal(raw, &tf); err != nil {
		return fmt.Errorf("host: tables.json: %w", err)
	}
	for _, rec := range tf.Tables {
		if err := rec.Config.validate(r.opts.LoadDeck); err != nil {
			return err
		}
		t := newTable(rec.Config)
		t.k = rec.Match
		scs, err := readSidecars(r.opts.Dir, rec.Config.ID)
		if err != nil {
			return err
		}
		for _, sc := range scs {
			if sc.State == protocol.MatchLive {
				sc.State = protocol.MatchAborted
				if err := writeSidecar(r.opts.Dir, sc, r.opts.Sync); err != nil {
					return err
				}
				// Task M2c-1: a match the previous process was cut off in is
				// recorded here as aborted; an embedder that persists every
				// match needs to observe it as such, so OnMatchEnd fires with
				// the rewritten (aborted) MatchInfo, like any other terminal
				// transition. Sidecar-derived, not a live match — see
				// callOnMatchEnd.
				r.callOnMatchEnd(rec.Config.ID, sc.Match, sc.info())
			}
			t.archived = append(t.archived, sc)
			if sc.Match > t.k {
				t.k = sc.Match
			}
		}
		r.tables[rec.Config.ID] = t
	}
	return nil
}
