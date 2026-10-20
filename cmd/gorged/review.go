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
	explain := rec.Explain
	if len(explain) > 0 && len(explain) != len(rec.Log.Intents) {
		// The seam (bots/explain) is one note per intent or none at all; a
		// misaligned sidecar would pin notes on the wrong decisions, so it
		// is dropped -- the game itself is still reviewable.
		fmt.Fprintf(os.Stderr, "gorged review: %s: explain has %d entries for %d intents; ignoring it\n", path, len(explain), len(rec.Log.Intents))
		explain = nil
	}
	return host.ReviewMatch{Config: cfg, Log: rec.Log, Names: rec.Match.Names,
		Policies: rec.Producer.Policies, Label: rec.Producer.Tool, Explain: explain}, nil
}

// reviewCorpus opens the checkout's corpus once for every record input.
var reviewCorpus = sync.OnceValues(func() (*cards.Registry, error) { return feedback.Corpus() })
