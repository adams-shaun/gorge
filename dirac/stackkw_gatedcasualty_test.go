package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStackKwGatedCasualtyReadsPriorCasts pins a gated Casualty grant's
// cast-time read (Anhelo, the Painter: "the first instant or sorcery spell
// you cast each turn has casualty 2"). gorge reads the gate before the
// in-flight spell is pushed, so the first instant or sorcery of the turn is
// offered its casualty cast and the second is not. A lockstep game cannot
// reach it reliably (the gate is false under a cast-first policy by the time
// the spell is cast), so the two polarities are pinned on one scenario.
func TestStackKwGatedCasualtyReadsPriorCasts(t *testing.T) {
	set := cardsettest.Set(t, "stackgrant4")
	deck := []cardset.Entry{
		{Name: "Anhelo, the Painter", Count: 1},
		{Name: "Lightning Bolt", Count: 2},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Mountain", Count: 2},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(id["Anhelo, the Painter"], 0)
	sc.perm(id["Grizzly Bears"], 0) // the casualty creature: power 2
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Mountain"], 0)
	bolt := sc.hand(id["Lightning Bolt"], 0)
	casualty := func() int {
		n := 0
		for _, a := range sc.castOptions(bolt) {
			if a.Kind() == AkCastCasualty {
				n++
			}
		}
		return n
	}
	if got := len(sc.castOptions(bolt)); got == 0 {
		t.Fatal("precondition: the Bolt is not castable at all")
	}
	if casualty() == 0 {
		t.Fatal("the first instant of the turn was not offered Anhelo's casualty cast")
	}
	// a second instant already cast this turn: the gate (Count$ThisTurnCast ==
	// 0) is false, so no casualty cast is offered
	prior := sc.hand(id["Lightning Bolt"], 0)
	sc.s.objs[prior].zone = kop.ZoneStack
	sc.s.stack[0] = stackEntry{kind: seSpell, obj: prior, card: id["Lightning Bolt"], ctrl: 0, prov: uint8(kop.ZoneHand)}
	sc.s.sp = 1
	sc.s.noteCast(prior, 0, kop.ZoneHand)
	if !sc.k.gen.castRingOn {
		t.Fatal("precondition: the table does not record the cast ring")
	}
	if casualty() != 0 {
		t.Fatal("a second instant of the turn was offered Anhelo's casualty cast")
	}
}
