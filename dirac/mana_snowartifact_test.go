package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSnowArtifactManaPaysSnow: a Snow Artifact's unit is an Artifact unit and
// a snow unit both (gorge's ManaProducerTag, "SArtifact<colour>"; CR 106.3), so
// Replicating Ring's mana pays {S}. Only a typed subtype word (Desert, Cave)
// keeps the unit from being snow.
func TestSnowArtifactManaPaysSnow(t *testing.T) {
	tbl := v.NewTable()
	ring := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Snow Ring", Cost: mc2(3, kop.W), Types: kop.TArtifact, Snow: true,
			Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColW, ManaN: 1}}},
		kop.CardDef{Name: "Test Snow Ask", Cost: mc2(2, kop.W), Types: kop.TCreature, Colors: kop.ColW, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbActivated,
				Cost: kop.Cost{Mana: kop.ManaCost{Snow: 1}, Flags: kop.CostTap},
				Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anySnow {
		t.Fatal("precondition: the table has no snow tally")
	}
	if !sc.k.cards[ring].manaTyped {
		t.Fatal("precondition: an artifact's mana is typed")
	}
	if !sc.k.cards[ring].manaSnow {
		t.Fatal("a Snow Artifact's mana is not tagged snow")
	}
	if !sc.s.tapOneForMana(sc.k, 0, sc.slot(sc.perm(ring, 0))) {
		t.Fatal("the snow artifact does not tap for mana")
	}
	if got := sc.s.pl[0].pool[kop.W]; got != 1 {
		t.Fatalf("precondition: pool W %d, want 1", got)
	}
	if got := tally(sc.s, 0, kop.W); got != 1 {
		t.Fatalf("snow tally %d, want 1", got)
	}
	if !sc.s.poolPays(0, &kop.ManaCost{Snow: 1}) {
		t.Fatal("the Snow Artifact's unit does not pay {S}")
	}
}
