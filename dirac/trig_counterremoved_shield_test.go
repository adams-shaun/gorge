package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCounterRemovedShield: a shield counter spent instead of destruction
// (CR 122.1c) is a CounterChange with a negative amount in gorge, so a
// Mode$ CounterRemoved line on the permanent queues (counterRemovedMatches).
func TestCounterRemovedShield(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Shield Watcher", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigCounterRemovedOnce, N: int16(kop.CtrShield)},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.perm(n, 0)
	sl := sc.slot(o)
	p := &sc.s.perms[sl]
	p.addCtrN(sc.s, int(kop.CtrShield), 1)
	sc.s.npend = 0
	if !sc.s.shieldDestroy(sc.k, sl) {
		t.Fatal("shieldDestroy did not spend the counter")
	}
	if got := p.ctrN(sc.s, int(kop.CtrShield)); got != 0 {
		t.Fatalf("shield counters left %d, want 0", got)
	}
	if int(sc.s.npend) != 1 {
		t.Fatalf("queued %d counter-removed triggers, want 1", sc.s.npend)
	}
	if sc.s.pend[0].trigObj != o || sc.s.pend[0].trigAmt != 1 {
		t.Fatalf("queued object %d amount %d, want %d and 1", sc.s.pend[0].trigObj, sc.s.pend[0].trigAmt, o)
	}
}
