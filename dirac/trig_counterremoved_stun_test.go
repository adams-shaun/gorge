package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// counterRemovedStunScen: an FDN Plains table with a test creature carrying
// the Card.Self "whenever a counter is removed from this" line, listening for
// a stun counter (Mode$ CounterRemovedOnce, CounterType$ stun).
func counterRemovedStunScen(t *testing.T) (*scen, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Stun Watcher", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigCounterRemovedOnce, N: int16(kop.CtrStun)},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	return newScenTable(t, tbl, Config{NoElide: true}), n
}

// TestCounterRemovedStunUntap: the untap replacement spends a stun counter
// instead of untapping (CR 122.1d), and that removal is a CounterChange with a
// negative amount, so a Mode$ CounterRemoved line on the permanent queues
// (gorge's counterRemovedMatches). Every stun site goes through subCtrFire.
func TestCounterRemovedStunUntap(t *testing.T) {
	sc, c := counterRemovedStunScen(t)
	o := sc.perm(c, 0)
	sl := sc.slot(o)
	p := &sc.s.perms[sl]
	p.flags |= pfTapped
	p.addCtrN(sc.s, int(kop.CtrStun), 1)
	sc.s.npend = 0
	sc.s.untapSlot(sc.k, sl)
	if got := p.ctrN(sc.s, int(kop.CtrStun)); got != 0 {
		t.Fatalf("stun counters left %d after the untap replacement, want 0", got)
	}
	if p.flags&pfTapped == 0 {
		t.Fatal("the permanent untapped: the stun replacement did not run")
	}
	if int(sc.s.npend) != 1 {
		t.Fatalf("queued %d counter-removed triggers, want 1", sc.s.npend)
	}
	if sc.s.pend[0].trigObj != o || sc.s.pend[0].trigAmt != 1 {
		t.Fatalf("queued object %d amount %d, want %d and 1", sc.s.pend[0].trigObj, sc.s.pend[0].trigAmt, o)
	}
}
