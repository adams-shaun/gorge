package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The each-player discard lockstep (CR 701.9, 800.4): Rankle's Prank's
// "each player discards two cards" is the only unfiltered, non-optional
// each-player discard with a choice in the lifted corpus. The kernel asks the
// controller first, then the opponent, and answers both asks in place; gorge
// does the same. Rankle's Prank is the discard mode of a multi-pick charm, so
// this also exercises the mode block before the each-player ask.
var rankleDeck = []cardset.Entry{
	{Name: "Rankle's Prank", Count: 4}, {Name: "Swamp", Count: 18},
	{Name: "Ornithopter", Count: 18},
}

// ranklePolicy always casts and plays when it can, so the deck reaches
// Rankle's Prank; every other decision (including the two discard choices)
// is random.
func ranklePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority {
			var opts []Canon
			for _, a := range p.D.Options {
				if k := a.Kind(); k == ActCast || k == ActPlayLand {
					opts = append(opts, a)
				}
			}
			if len(opts) > 0 {
				d := *p.D
				d.Options = opts
				return rnd.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
			}
		}
		return rnd.Pick(p)
	})
}

func TestRankleLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("charmnum")
	if err != nil {
		t.Fatal(err)
	}
	md := [2][]cardset.Entry{rankleDeck, rankleDeck}
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	casts, seen := 0, 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			e := &evs[seen]
			if e.Kind != events.PutOnStack {
				continue
			}
			if o := ref.E.G.Obj(e.Obj); o != nil && o.Card != nil && len(o.Card.Faces) > 0 &&
				o.Card.Faces[0].Name == "Rankle's Prank" {
				casts++
			}
		}
		return nil
	}
	for seed := uint64(1); seed <= 64; seed++ {
		seen = 0
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"rankle-a", "rankle-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, ranklePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 12000, MaxTurns: 200, Check: check})
		CloseSide(k)
		if res.Stall != "" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if casts == 0 {
		t.Errorf("Rankle's Prank was never cast: the each-player discard was not exercised")
	}
	t.Logf("Rankle's Prank cast %d times", casts)
}

// wheelDeck reaches Wheel of Fortune's Mode$ Hand each-player discard: the
// kernel's RefEachPlayer guard also covers the A=127 whole-hand sentinel,
// which the test scoped out but which gorge resolves the same way.
var wheelDeck = []cardset.Entry{
	{Name: "Wheel of Fortune", Count: 4}, {Name: "Mountain", Count: 20},
	{Name: "Grizzly Bears", Count: 16},
}

func TestWheelLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("eachdiscard")
	if err != nil {
		t.Fatal(err)
	}
	md := [2][]cardset.Entry{wheelDeck, wheelDeck}
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	casts, seen := 0, 0
	check := func(ref *GorgeSide, _ Side) *Divergence {
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			e := &evs[seen]
			if e.Kind != events.PutOnStack {
				continue
			}
			if o := ref.E.G.Obj(e.Obj); o != nil && o.Card != nil && len(o.Card.Faces) > 0 &&
				o.Card.Faces[0].Name == "Wheel of Fortune" {
				casts++
			}
		}
		return nil
	}
	for seed := uint64(1); seed <= 64; seed++ {
		seen = 0
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"wheel-a", "wheel-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, ranklePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 12000, MaxTurns: 200, Check: check})
		CloseSide(k)
		if res.Stall != "" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	if casts == 0 {
		t.Errorf("Wheel of Fortune was never cast: the Mode$ Hand each-player discard was not exercised")
	}
	t.Logf("Wheel of Fortune cast %d times", casts)
}
