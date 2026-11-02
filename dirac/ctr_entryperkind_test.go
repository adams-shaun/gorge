package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// entryCounterScen is a generated-table scenario over the entryctr pool.
func entryCounterScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Wildgrowth Archaic", Count: 1}, {Name: "Hardened Scales", Count: 1}, {Name: "Thunderous Velocipede", Count: 1},
		{Name: "Doubling Season", Count: 1}, {Name: "Squire", Count: 1}, {Name: "Forest", Count: 1},
	}
	set := cardsettest.Set(t, "entryctr")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Forest"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// follow35Cast enters a card as the resolving creature spell: converge colours
// of mana spent (G only) and effects extra counters from earlier Archaics
// (each of 1). It returns the +1/+1 counters it entered with.
func follow35Cast(sc *scen, c CardID, effects int) int {
	o := sc.obj(c, 0)
	code := 0
	for i := 0; i < effects; i++ {
		code, _ = etbCodeAdd(code, 1)
	}
	sc.s.res.active = true
	sc.s.res.e = stackEntry{kind: seSpell, obj: o, card: c, spc: 1<<8 | uint16(code)<<spcEtbShift}
	sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneStack)
	sc.s.res.active = false
	return sc.s.perms[sl].ctrLow(int(kop.CtrP1P1))
}

// TestEntryCountersOnePut: gorge puts every +1/+1 counter a permanent
// enters with in ONE event, so an additive replacement applies once to the
// sum of the entrant's own converge counters, an earlier Archaic's effect and
// a lent body. Cast for G only (converge 1), an earlier Archaic's effect 1.
func TestEntryCountersOnePut(t *testing.T) {
	for _, c := range []struct {
		name    string
		onBF    []string
		effects int
		want    int
	}{
		{"scales: (1+1)+1", []string{"Hardened Scales"}, 1, 3},
		{"scales, no effect", []string{"Hardened Scales"}, 0, 2},
		{"scales + velocipede, effect: 1+1+1 +1", []string{"Hardened Scales", "Thunderous Velocipede"}, 1, 4},
		{"scales + velocipede, no effect", []string{"Hardened Scales", "Thunderous Velocipede"}, 0, 3},
		{"doubling season: 2(1+1)", []string{"Doubling Season"}, 1, 4},
		{"nothing", nil, 1, 2},
	} {
		sc, id := entryCounterScen(t)
		for _, n := range c.onBF {
			sc.perm(id[n], 0)
		}
		if got := follow35Cast(sc, id["Wildgrowth Archaic"], c.effects); got != c.want {
			t.Errorf("%s: entered with %d counters, want %d", c.name, got, c.want)
		}
		if sc.s.Stalled() {
			t.Errorf("%s: stalled (reason %d)", c.name, sc.s.StallReason())
		}
	}
}

// TestEntryCountersTwoEffects: two effects on an entrant with no other
// source of entry counters are two puts (gorge folds Effect-borne bodies only
// into a staged entry: entryBodyCandidates), so Hardened Scales adds twice.
// With a lent body on ANY permanent (an opponent's Velocipede stages the entry
// though it lends nothing to this one) the effects fold into one put.
func TestEntryCountersTwoEffects(t *testing.T) {
	sc, id := entryCounterScen(t)
	sc.perm(id["Hardened Scales"], 0)
	if got := follow35Cast(sc, id["Squire"], 2); got != 4 {
		t.Errorf("plain creature, two effects: %d counters, want (1+1)+(1+1) = 4", got)
	}
	sc, id = entryCounterScen(t)
	sc.perm(id["Hardened Scales"], 0)
	sc.perm(id["Thunderous Velocipede"], 1)
	if got := follow35Cast(sc, id["Squire"], 2); got != 3 {
		t.Errorf("two effects, an opponent's lent body (stages the entry, does not apply): %d counters, want one put (1+1)+1 = 3", got)
	}
}
