package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// testGen is the scenario's prep with an empty generated-table prep, for the
// paths that read prep.gen (the table's own prep is shared by every scenario).
func testGen(sc *scen) *prep {
	k := *sc.k
	k.gen = &genPrep{flags: make([]uint32, len(sc.k.t.Cards))}
	return &k
}

// The window census's self Taps arm must read the live permanent, not just the
// card definition: gorge's paymentPlanTapObservers skips a carrier whose
// printed abilities are gone (Honest Work's Humble Merchant keeps only an
// effect-granted {T}) and a latent line is never subscribed. Before the gate,
// selfTapsDefer reported the removed watcher and the census over-deferred, so
// the waterbend announcement offered Done where gorge prices the granted unit.
func TestWindowSelfTapsCensusSkipsLostAndLatentWatchers(t *testing.T) {
	tbl, ids := kcapsTable(t,
		kop.CardDef{Name: "Test Self Tap Watch", Types: kop.TCreature,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigTaps, Flags: kop.TfSelf}}}},
		kop.CardDef{Name: "Test Latent Self Tap Watch", Types: kop.TCreature,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, AFlags: kop.AfLatent,
				Trig: kop.Trigger{Event: kop.TrigTaps, Flags: kop.TfSelf}}}},
	)

	t.Run("lost all abilities", func(t *testing.T) {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		k := testGen(sc)
		sl := sc.slot(sc.perm(ids[0], 0))
		if !sc.s.selfTapsDefer(k, sl) {
			t.Fatal("precondition: the printed self Taps watcher does not defer its own tap")
		}
		sc.s.perms[sl].ovr.flags |= ovLoseAll
		if sc.s.selfTapsDefer(k, sl) {
			t.Fatal("a permanent that lost all abilities still defers for its removed printed Taps watcher")
		}
	})

	t.Run("latent watcher", func(t *testing.T) {
		if !kHasSelfTaps(tbl, ids[1]) {
			t.Fatal("precondition: the hand-built card does not carry the latent self Taps watcher")
		}
		sc := newScenTable(t, tbl, Config{NoElide: true})
		if sc.s.selfTapsDefer(testGen(sc), sc.slot(sc.perm(ids[1], 0))) {
			t.Fatal("a latent self Taps watcher (never subscribed) defers its source's tap")
		}
	})
}

// kHasSelfTaps is the test-side check that the hand-built card's Trigger is
// what the arm reads, so the latent assertion above cannot pass vacuously.
func kHasSelfTaps(tbl *kop.Table, c CardID) bool {
	for i := range tbl.Cards[c].Abilities {
		a := &tbl.Cards[c].Abilities[i]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Flags&kop.TfSelf != 0 {
			return true
		}
	}
	return false
}

// A Taps watcher's ValidPlayer$ names the TAPPER (gorge tapsMatches/TapActor),
// so an opponent's "whenever you tap" watcher cannot defer the payer's own
// mana tap. Before the actor gate, tapsWatchClass matched only the filter and
// classified the payer's source as deferred.
func TestTapsWatchClassReadsTheTapper(t *testing.T) {
	tbl, ids := kcapsTable(t,
		kop.CardDef{Name: "Test Tapped Creature", Types: kop.TCreature},
		kop.CardDef{Name: "Test Opp Taps Watch", Types: kop.TCreature},
	)
	fid := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	tbl.Cards[ids[1]].Abilities = []kop.Ability{{Kind: kop.AbTriggered,
		Trig: kop.Trigger{Event: kop.TrigTaps, Filter: fid, Whose: kop.WhoseYou, TapPlan: kop.TapPlanDefer}}}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	k := testGen(sc)
	k.gen.anyTapsDefer = true
	k.gen.setFlag(ids[1], gfTapsDefer)
	src := sc.slot(sc.perm(ids[0], 0))
	sc.perm(ids[1], 1)
	if !sc.s.tapsDeferred(k, 1, src) {
		t.Fatal("the watcher's own controller is the tapper: the watcher must defer the source")
	}
	if sc.s.tapsDeferred(k, 0, src) {
		t.Fatal("the payer is not the watcher's controller, so its mana tap cannot fire the watcher")
	}
}
