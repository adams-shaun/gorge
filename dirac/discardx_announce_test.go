package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// discardXScen is a table with one instant whose own cost is a
// Discard<X/Card> part and no printed {X} (Channeled Force's shape).
func discardXScen(t *testing.T) (*scen, CardID, CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, da, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	lib := idOf(tbl, da, "Plains")
	spell := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:  "Test Discard X",
		Types: kop.TInstant,
		Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Cost: kop.Cost{Mana: kop.ManaCost{Generic: 2}, Flags: kop.CostDiscard, DiscardX: true},
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}},
		}},
	})
	return newScenTableLib(t, tbl, lib, Config{NoElide: true}), spell, lib
}

// TestDiscardXWithoutPrintedXAnnouncesTheHand: a Discard<X/Spec> part with no
// {X} mana is the only X the cost carries, so it is the bound (gorge
// cast_subcounter.go xAsk: "when an announced part is the ONLY X the cost
// carries it IS the bound"). The cast asks X = 0..matching hand cards and the
// announced count is what the discard picks owe (CR 601.2b, 601.2h). Before
// the announced-only path the bsX gate skipped the ask and the card paid X = 0
// (Channeled Force / Insidious Dreams).
func TestDiscardXWithoutPrintedXAnnouncesTheHand(t *testing.T) {
	sc, spell, lib := discardXScen(t)
	if sc.k.gen == nil || !sc.k.gen.uses.has(useDiscardX) {
		t.Fatal("prep missed the table's announced-X discard cost")
	}
	for i := 0; i < 3; i++ {
		sc.perm(lib, 0)
	}
	card := sc.hand(spell, 0)
	for i := 0; i < 4; i++ {
		sc.hand(lib, 0) // the four cards the discard can pick
	}
	sc.do(t, AkCast, TCode(card), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	if d.N != 5 {
		t.Fatalf("X ask offers %d values, want 0..4 (the four other hand cards)", d.N)
	}
	for x := 0; x < int(d.N); x++ {
		if d.Options[x].Mode() != uint8(x) {
			t.Fatalf("option %d is X=%d", x, d.Options[x].Mode())
		}
	}
	// X = 2 owes two discard picks, one per decision.
	sc.s.Apply(d.Options[2])
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCostDiscard {
		t.Fatalf("decision kind %d sub %d after X = 2, want the discard pick", d.Kind, d.Sub)
	}
	if d.N != 4 {
		t.Fatalf("discard pick offers %d cards, want the four hand cards", d.N)
	}
}
