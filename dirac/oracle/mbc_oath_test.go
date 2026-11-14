package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Oath cycle (Druids, Scholars, Mages) against gorge in a fixture-table lockstep game
// (printedDelveSet). Its upkeep trigger's target is ValidTgts$
// Player.OpponentToActive+withMoreCreaturesThanActive, answered by the player
// whose upkeep it is (TargetingPlayer$ TriggeredPlayer), and its DigUntil is
// Optional$ True, a yes/no ask of that player. The Oath deck holds creatures
// to find and few of its own on the board; the aggro deck builds a board, so
// the trigger is removed on some upkeeps (the active player does not trail)
// and fires on others. The mutation drops the target's active-player reading
// from the table, so the trigger fires on every upkeep and must diverge.
func oathDecks(oath string) [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: oath, Count: 8}, {Name: "Craw Wurm", Count: 6},
			{Name: "Forest", Count: 8}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 10},
		},
		okagachiDecks[1],
	}
}

// oathTally counts, from gorge's events, the Oath triggers pushed at an
// upkeep (pushed), those that reached resolution with a legal target
// (resolved; the rest were removed or fizzled for want of one), and the
// creatures put onto the battlefield from a library (dug: only the Oath does
// that in these decks).
type oathTally struct {
	pushed, resolved, dug int
}

func oathCheck(oath string, tal *oathTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
		}
		isOath := func(id state.ObjID) bool {
			o := ref.E.G.Obj(id)
			return o != nil && o.Face() != nil && o.Face().Name == oath
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			switch ev := evs[seen]; ev.Kind {
			case events.TriggerPush:
				if isOath(ev.Obj) {
					tal.pushed++
				}
			case events.Resolve:
				if isOath(ev.Obj) {
					tal.resolved++
				}
			case events.MoveZone:
				if ev.From == state.ZLibrary && ev.To == state.ZBattlefield {
					tal.dug++
				}
			}
		}
		return nil
	}
}

// oathRun plays n seeds of oathDecks and returns the divergences; with mut
// set it stops at the first one.
func oathRun(t *testing.T, oath string, n uint64, tal *oathTally, mut func(*kop.Table)) int {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	var names [2][]string
	decks := oathDecks(oath)
	for s, d := range decks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut != nil {
		mut(tbl)
	}
	div := 0
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"oath", "aggro"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{MaxDecisions: 6000, MaxTurns: 40, Check: oathCheck(oath, tal)})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if mut != nil {
				return div
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div
}

// dropOathTargetReading clears the active-player flags of every target slot
// in the table that carries them, and returns how many it cleared.
func dropOathTargetReading(tbl *kop.Table) int {
	dropped := 0
	cds := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cds {
		abs := append([]kop.Ability(nil), cds[i].Abilities...)
		for j := range abs {
			for ti := 0; ti < int(abs[j].NTargets); ti++ {
				if f := abs[j].Targets[ti].TFlags; f&kop.TgfPlayerCompareOK != 0 {
					abs[j].Targets[ti].TFlags = f &^ kop.TgfPlayerCompareOK
					dropped++
				}
			}
		}
		cds[i].Abilities = abs
	}
	tbl.Cards = cds
	return dropped
}
