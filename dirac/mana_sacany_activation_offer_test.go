package dirac

import (
	"strings"
	"testing"
)

// TestSacAnyRepeatFundsActivationOffer: a closed-bound sacrifice-any mana
// source (Jungle Patrol's "Sacrifice a token named Wood: Add {R}") whose
// sacrifice part is not the source is repeatable in gorge's payment planner
// (paymentPlanRelaxedAlternatives once is false), so an activation two of its
// activations and a plain source pay is offered: Bone Sabres' Equip {3} beside
// an untapped Forest is paid by the Forest's {G} and two Wood sacrifices
// (tail-14-c000 pair 1, decision 546; CR 605.3a-b). With one Wood the two
// sources make two mana and the activation stays unoffered.
func TestSacAnyRepeatFundsActivationOffer(t *testing.T) {
	for _, c := range []struct {
		name  string
		woods int
		want  bool
	}{
		{"one Wood and the Forest cannot fund the equip", 1, false},
		{"two Wood sacrifices and the Forest pay the equip", 2, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc, id := synthScen(t, "tail-14", "Bone Sabres", "Jungle Patrol", "Forest")
			var wood = CardID(0xffff)
			for cid := range sc.k.t.Cards {
				if strings.EqualFold(sc.k.t.Cards[cid].Name, "wood") {
					wood = CardID(cid)
				}
			}
			if wood == 0xffff {
				t.Fatal("the matchup table holds no wood token")
			}
			// Preconditions: the equip ability's mana cost is {3}, the board
			// holds the untapped land the pool completes with, Jungle
			// Patrol's mana ability is the modelled sacrifice-any gap and
			// the board holds the Wood candidates its cost sacrifices.
			sabres := sc.perm(id["Bone Sabres"], 0)
			patrol := sc.perm(id["Jungle Patrol"], 0)
			forest := sc.perm(id["Forest"], 0)
			if mc := &sc.k.t.Cards[sc.s.objs[sabres].card].Abilities[0].Cost.Mana; mc.MV() != 3 {
				t.Fatalf("the equip ability's cost %v, want {3}", mc)
			}
			if sc.s.perms[sc.slot(forest)].flags&pfTapped != 0 {
				t.Fatal("the Forest is tapped")
			}
			for i := 0; i < c.woods; i++ {
				w := sc.perm(wood, 0)
				sc.s.objs[w].of |= ofToken
			}
			var cs [MaxPerm]Slot
			if got := sc.s.tapXCands(sc.k, 0, sc.slot(patrol), &cs); got != c.woods {
				t.Fatalf("Jungle Patrol has %d sacrifice candidates, want %d", got, c.woods)
			}
			if !sc.s.scriptTapXModelled(sc.k, 0).has(sc.slot(patrol)) {
				t.Fatal("Jungle Patrol is not a modelled sacrifice-any gap")
			}
			if got := offersAb(sc.s, sabres, 0); got != c.want {
				t.Errorf("equip activation offered %v, want %v", got, c.want)
			}
		})
	}
}
