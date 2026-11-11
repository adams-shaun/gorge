package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// Fireball announces X (CR 601.2b) and then any number of targets (601.2c),
// and "costs {1} more to cast for each target beyond the first" (601.2f): a
// player who announced X for what the sources make and then chooses more
// targets owes more than they can pay. Gorge lets the choice stand and
// reverses the cast at the payment (CR 733.1, 601.2h). The kernel's plain
// configuration (the rules census, no gorge surface: what kplay plays) planned
// the payment with tapForRider, which panics "autopay found no plan"; it now
// reverses the cast too. The lockstep surfaces already did (xWindowPay).

// fireballXRaiseDecks: eight Fireballs over a thin mana base, so the sources
// pay a wide cast rarely, against creatures that give the Fireball targets.
var fireballXRaiseDecks = [2][]cardset.Entry{
	{
		{Name: "Fireball", Count: 8}, {Name: "Raging Goblin", Count: 8},
		{Name: "Mountain", Count: 24},
	},
	{
		{Name: "Raging Goblin", Count: 16}, {Name: "Savannah Lions", Count: 8},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8},
	},
}

// fireballXRaiseSetup is the divideevenly fixture set (Fireball's lowering),
// the corpus decks and the kernel table built from them.
func fireballXRaiseSetup(t *testing.T) (cardset.Set, [2][]*cards.Card, [2][]cardset.Entry) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("divideevenly")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range fireballXRaiseDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	return set, decks, fireballXRaiseDecks
}

// hasPerTargetRaise reports that the matchup table carries the per-target
// raise, the static whose charge the cast prices after its targets (Fireball
// is the only card of the decks that has it).
func hasPerTargetRaise(tbl *kop.Table) bool {
	for id := range tbl.Cards {
		for _, a := range tbl.Cards[id].Abilities {
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIncreaseCost && a.Static.StFlags&kop.StRaisePerTarget != 0 {
				return true
			}
		}
	}
	return false
}

// TestReproFireballXRaisePlainNoPanic plays the kernel alone on its plain
// configuration with random choices: every game ends without the planner
// panic.
func TestReproFireballXRaisePlainNoPanic(t *testing.T) {
	set, _, md := fireballXRaiseSetup(t)
	tbl, da, db, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	if !hasPerTargetRaise(tbl) {
		t.Fatal("the table's Fireball has no per-target raise: the test would not reach the charge")
	}
	for seed := uint64(1); seed <= 400; seed++ {
		func() {
			defer func() {
				if e := recover(); e != nil {
					t.Errorf("seed %d: %v", seed, e)
				}
			}()
			var s dirac.State
			var d dirac.Decision
			s.Init(tbl, da, db, seed, dirac.Config{})
			rnd := dirac.NewRandomPolicy(seed ^ 0x6669726562616c6c)
			for n := 0; n < 4000; n++ {
				s.StepInto(&d)
				if d.Kind == dirac.DkNone || s.Turn() >= 30 {
					return
				}
				if d.Kind == dirac.DkMulligan {
					s.Apply(d.Options[0])
					continue
				}
				s.Apply(rnd.Pick(&d))
			}
		}()
	}
}

// TestReproFireballXRaiseLockstep plays gorge and the kernel in lockstep on both
// surfaces over the same decks: no divergence, and at least one Fireball cast
// is reversed at its payment by the targets' raise, so the reversal itself is
// compared (the gorge side reverses; the kernel matched it already).
func TestReproFireballXRaiseLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	set, decks, md := fireballXRaiseSetup(t)
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	reversed := 0
	for seed := uint64(1); seed <= 120; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"fbx-a", "fbx-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		for i := range g.E.L.Events {
			if g.E.L.Events[i].Kind == events.SpellCastReversed {
				reversed++
			}
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	t.Logf("%d reversed casts", reversed)
	if reversed == 0 {
		t.Error("no cast was reversed: the games never reach a raise the sources cannot pay")
	}
}
