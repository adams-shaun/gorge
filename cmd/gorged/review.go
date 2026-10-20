package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/matchrecord"
	"github.com/adams-shaun/gorge/testkit/testutil/feedback"
)

// reviewLoad is `gorged review`'s loader. A directory is a feedback snapshot
// (match.json + log.json, the server's own feedback capture) read through
// feedback.Load; anything else is a match record file (<id>.match.json.gz,
// server/matchrecord) loaded against the same corpus. Both resolve card
// names and token scripts against the checkout's .cards. It lives here, in
// the binary, rather than in server/gorged, because server/gorged never
// imports testkit (docs/agents/invariants.md).
func reviewLoad(path string) (host.ReviewMatch, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return host.ReviewMatch{}, err
	}
	if fi.IsDir() {
		l, cfg, meta, err := feedback.Load(path)
		if err != nil {
			return host.ReviewMatch{}, err
		}
		return host.ReviewMatch{Config: cfg, Log: l, Label: meta.ID}, nil
	}
	reg, err := reviewCorpus()
	if err != nil {
		return host.ReviewMatch{}, err
	}
	rec, cfg, err := matchrecord.Load(path, reg)
	if err != nil {
		return host.ReviewMatch{}, err
	}
	if len(rec.Chance) > 0 {
		return host.ReviewMatch{}, fmt.Errorf("record carries a chance transcript; review of kernel-sourced records needs replay.WalkChance (T6)")
	}
	return host.ReviewMatch{Config: cfg, Log: rec.Log, Names: rec.Match.Names,
		Policies: rec.Producer.Policies, Label: rec.Producer.Tool}, nil
}

// reviewCorpus opens the checkout's corpus once for every record input.
var reviewCorpus = sync.OnceValues(func() (*cards.Registry, error) { return feedback.Corpus() })
