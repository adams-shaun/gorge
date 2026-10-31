package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/draft"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// wubrgLabels names the five colour bits, in cards.ColourIdentity order.
var wubrgLabels = [5]string{"W", "U", "B", "R", "G"}

// bestPairNonLand returns the colour pair (labelled "W/U"-style) that holds
// the most non-land pool cards whose colour identity is a subset of the
// pair — colourless counts in every pair, the builder's step-1 shape — and
// that count. The ten pairs are scanned in fixed i<j bit order, so the
// result is deterministic.
func bestPairNonLand(reg *cards.Registry, pool []limited.CardRef) (string, int) {
	type facts struct {
		mask   uint8
		isLand bool
	}
	fs := make([]facts, len(pool))
	for i, ref := range pool {
		c, ok := reg.Lookup(ref.Name)
		if !ok {
			continue
		}
		fs[i] = facts{mask: c.ColourIdentity(), isLand: len(c.Faces) > 0 && c.Faces[0].IsLand()}
	}
	bestLabel, best := "", -1
	for a := 0; a < 5; a++ {
		for b := a + 1; b < 5; b++ {
			allowed := uint8(1<<a | 1<<b)
			n := 0
			for i := range pool {
				if fs[i].isLand || fs[i].mask&^allowed != 0 {
					continue
				}
				n++
			}
			if n > best {
				best, bestLabel = n, fmt.Sprintf("%s/%s", wubrgLabels[a], wubrgLabels[b])
			}
		}
	}
	return bestLabel, best
}

// TestDraftsimPoolCommitsToColourPair is E4-2's draft-only gate: with the
// default tuning (ColourFitMax 8) a seeded pod's every seat ends the draft
// with at least 23 non-land cards inside its best colour pair, the depth
// the best23 builder needs to field a real two-colour main. The drafter and
// builder come through personaPair, so the registry path with the lookup
// injected is what the pod actually runs. No games are played.
//
// The seeds cover one event each because the brief's single seed 1 does not
// bind: measured on this corpus, seed 1's four pools clear 23 under the old
// k=4 too. Seeds 1..4 is the smallest window that separates the two knobs —
// k=4 leaves seeds 3 and 4 with a 22-card best pair, while k=8 holds 23 or
// better throughout (its rare thin pools, 3 of 192 probed, live at seeds
// 6, 11 and 45).
func TestDraftsimPoolCommitsToColourPair(t *testing.T) {
	dir := corpusReady(t)
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata: %v", err)
	}
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		t.Fatalf("corpus: %v", err)
	}
	lookup := func(name string) *cards.Card { c, _ := reg.Lookup(name); return c }
	drafter, builder, err := personaPair("rating-colour", "best23", lookup)
	if err != nil {
		t.Fatalf("personaPair: %v", err)
	}
	r := &runner{
		reg:     reg,
		spec:    spec,
		drafter: drafter,
		builder: builder,
		players: strings.Split("bot,bot,bot,bot", ","),
		noGames: true,
		out:     &bytes.Buffer{},
	}
	for seed := uint64(1); seed <= 4; seed++ {
		ev, er, err := runOneEvent(r, seed, false)
		if err != nil {
			t.Fatalf("runOneEvent seed %d: %v", seed, err)
		}

		// Precondition: draft-only really skipped the games, and every seat
		// has a drafted pool and a built deck — the objects the assertion
		// below reads. A vacuous setup must fail here, not pass below.
		if len(er.games) != 0 {
			t.Fatalf("draft-only run seed %d played %d games, want 0", seed, len(er.games))
		}
		if ev.Phase() == event.PhaseDraft || ev.Phase() == event.PhaseLobby {
			t.Fatalf("event phase after the draft-only run is %s, want the rounds with the series pending", ev.Phase())
		}
		if len(ev.PendingSeries()) == 0 {
			t.Fatalf("seed %d: no series pending after the draft-only run; the draft did not finish", seed)
		}
		for s := 0; s < draft.Seats; s++ {
			pool := ev.Pool(s)
			if len(pool) == 0 {
				t.Fatalf("seed %d seat %d drafted an empty pool", seed, s)
			}
			if len(er.decks[s]) == 0 {
				t.Fatalf("seed %d seat %d built an empty deck", seed, s)
			}
			pair, n := bestPairNonLand(reg, pool)
			if n < 23 {
				t.Fatalf("seed %d seat %d's best colour pair %s holds %d non-land cards of a %d-card pool, want >=23",
					seed, s, pair, n, len(pool))
			}
		}
	}
}
