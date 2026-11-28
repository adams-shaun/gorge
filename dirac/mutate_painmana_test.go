package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMutatePainTableKeepsLethalGuard: a table that builds piles registers its
// plain mana abilities as pile grants, which no battlefield card's static
// marks. With painful mana on the same table the offer census must still read
// the board: the summed lethal guard binds while no static grant exists, so
// {G}{G} off two lands that each deal 1 damage is offered at 3 life and not at
// 2 (CR 119.4, 605.1a).
func TestMutatePainTableKeepsLethalGuard(t *testing.T) {
	tbl, mutator, bird, _, _ := mutateTable()
	tbl.Cards[mutator].MutateCost = kop.ManaCost{Pips: [kop.NumManaKinds]uint8{kop.G: 2}}
	pain := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Pain Land", Types: kop.TLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1,
			Prog: []kop.Instr{{Op: kop.OpDealDamage, Obj: kop.RefController, A: kop.ValueRef{K: kop.VConst, N: 1}}}}}})

	for _, tc := range []struct {
		life  int16
		offer bool
	}{{3, true}, {2, false}} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		if !sc.k.painMana || sc.k.gen == nil || !sc.k.gen.anyMutate || !sc.k.gen.anyContGrant {
			t.Fatalf("precondition: want a mutate table with painful mana and the pile grants armed")
		}
		sc.lands(0, pain, 2)
		sc.perm(bird, 0)
		mut := sc.hand(mutator, 0)
		sc.s.pl[0].life = tc.life
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("life %d: decision kind %d, want priority", tc.life, d.Kind)
		}
		if got := mutateOffered(d, mut); got != tc.offer {
			t.Fatalf("life %d: mutate offered %v, want %v: %v", tc.life, got, tc.offer, d.Options[:d.N])
		}
	}
}
