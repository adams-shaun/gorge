package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/matchrecord"
)

// recorder is -record's writer: one gorge-match-1 record per game, named
// by the game's place in the run and its seed, so a schedule that plays one
// seed twice still writes two records. A nil recorder writes nothing.
type recorder struct {
	dir      string
	producer matchrecord.Producer
	n        int
}

// openRecorder creates dir for a -record run; an empty dir is no recorder.
// cardsDir is the corpus the run plays, whose pin each record names.
func openRecorder(dir, cardsDir string) (*recorder, error) {
	if dir == "" {
		return nil, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("creating -record directory: %w", err)
	}
	return &recorder{dir: dir, producer: matchrecord.Producer{
		Tool: "mtgsim", GorgeCommit: matchrecord.GorgeCommit(), ForgeRef: matchrecord.ForgeRef(cardsDir),
	}}, nil
}

// write records the game cfg configured and e played. The record carries
// e's chance transcript, which an ordinary engine leaves empty, so
// matchrecord.VerifyChance verifies either kind.
func (r *recorder) write(seed uint64, cfg rules.Config, e *rules.Engine, policies []string) error {
	if r == nil {
		return nil
	}
	p := r.producer
	p.Policies = make([]string, len(cfg.Names))
	for i := range p.Policies {
		p.Policies[i] = "heuristic"
		if i < len(policies) && policies[i] != "" {
			p.Policies[i] = policies[i]
		}
	}
	rec, err := matchrecord.New(cfg, e.L, p)
	if err != nil {
		return fmt.Errorf("-record: %w", err)
	}
	rec.Chance = e.ChanceTranscript()
	path := filepath.Join(r.dir, fmt.Sprintf("%04d-seed%d%s", r.n, seed, matchrecord.Ext))
	r.n++
	if err := matchrecord.Write(path, rec); err != nil {
		return fmt.Errorf("-record: %w", err)
	}
	return nil
}
