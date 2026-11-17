package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The end-of-turn recipient-capture emblem lockstep (dirac/kgen's emblem gate,
// lower_effeottrig.go emblemCaptureFilter and dirac/trig_effectuntileot.go
// fireEmblemsDmg): an Effect that remembers its targeted creature and whose
// DamageDone line has ValidTarget$ ...IsRemembered fires for that creature as
// the damaged permanent only, queuing it as the trigger object the body reads.
//
// Arcbond's DamageAll body needed ticket A (the captured-source sweep) to
// lower, so this deck uses Glyph of Life, the same recipient-capture shape on
// a plain Mode$ DamageDone line (ValidSource$ Creature.attacking |
// ValidTarget$ Creature.IsRemembered, ExileOnMoved$ Battlefield). Both shapes
// reach fireEmblemsDmg through the same recipient half; the exact
// DamageDoneOnce lowering is
// TestEmblemDamageDoneOnceRecipientCapture in dirac/kgen and the per-recipient
// kernel behaviour is TestEffectEOTEmblemRecipientCapture in dirac.
//
// The policy casts Glyph of Life, attacks with every Bear and blocks with
// every Wall, so an attacking Bear damages a Wall the emblem remembers; the
// second Wall/Glyph and the other seat's damage supply the captured-recipient
// skips the hook must take (DebugEffEOTTrigStats.DmgCapSkips).
func kemblemRecipientDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Glyph of Life", Count: 4},
		{Name: "Wall of Wood", Count: 8},
		{Name: "Grizzly Bears", Count: 8},
		{Name: "Forest", Count: 14},
		{Name: "Plains", Count: 6},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

type kemblemRecipientStats struct {
	glyphs, blocks int
}

func kemblemRecipientPolicy(seed uint64, st *kemblemRecipientStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && optionFaceName(c, a) == "Glyph of Life" {
				st.glyphs++
				return a
			}
		}
		switch c.D.Kind {
		case DAttackers:
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					return a
				}
			}
		case DBlockers:
			for _, a := range c.D.Options {
				if a.Kind() == ActBlock && optionFaceName(c, a) == "Wall of Wood" {
					st.blocks++
					return a
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActPlayLand || k == ActCast && optionFaceName(c, a) != "Glyph of Life" {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestEmblemDamageDoneOnceRecipientCaptureLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	decks := kemblemRecipientDecks()
	var names [2][]string
	for s := range decks {
		for _, e := range decks[s] {
			for i := 0; i < e.Count; i++ {
				names[s] = append(names[s], e.Name)
			}
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var cardDecks [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cardDecks[s] = append(cardDecks[s], c)
			}
		}
		if len(cardDecks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(cardDecks[s]))
		}
	}
	st := new(kemblemRecipientStats)
	var eff dirac.EffEOTTrigStats
	old := dirac.DebugEffEOTTrigStats
	dirac.DebugEffEOTTrigStats = &eff
	defer func() { dirac.DebugEffEOTTrigStats = old }()
	const seeds = 96
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kemblem-a", "kemblem-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kemblemRecipientPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	t.Logf("Glyph of Life casts %d, Wall blocks %d, recipient-capture emblem firings %d, captured-recipient skips %d",
		st.glyphs, st.blocks, eff.DmgFires, eff.DmgCapSkips)
	if st.glyphs == 0 {
		t.Fatal("Glyph of Life was never cast; the lockstep proves nothing")
	}
	if st.blocks == 0 {
		t.Fatal("no Wall blocked an attacker; no attacking creature damaged the captured creature")
	}
	if eff.DmgFires < 10 {
		t.Errorf("only %d recipient-capture emblem firings (need 10)", eff.DmgFires)
	}
	if eff.DmgCapSkips < 10 {
		t.Errorf("only %d damage events skipped for the captured recipient (need 10); a second creature damaged the same turn must not fire the emblem", eff.DmgCapSkips)
	}
}
