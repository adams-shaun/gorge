package kgen

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestAttackTaxInteraction: Archangel of Tithes, Propaganda and Carnelian Orb
// of Dragonkind lower without a refusal and a matchup holding all three builds, as does
// one holding Propaganda and the Orb beside an attack requirement (Juggernaut): the kernel's
// attack-tax walk (dirac/combat_attacktax.go atkTaxGap) reads the Orb's
// spent-mana haste rider as the {T}: Add {R} source gorge's attack window lists
// (the rider rides a batch the window pays an attack cost from, CR 106.6, 508.1d).
// The lockstep over the played games is dirac/oracle atktax_orb_test.go.
func TestAttackTaxInteraction(t *testing.T) {
	cardsDir := ratchetCards(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"Archangel of Tithes", "Propaganda", "Carnelian Orb of Dragonkind", "Juggernaut", "Mountain", "Plains"}
	var pool []*cards.Card
	for _, n := range names {
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the corpus", n)
		}
		pool = append(pool, c)
	}
	for _, c := range lowerNamed(t, names) {
		for _, r := range c.Refusals {
			t.Errorf("%s: refused %s: %s", c.Name, r.Node, r.Reason)
		}
	}
	pd, err := TableFromCards(reg, pool, "attacktax-interaction")
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	e := func(n string, c int) cardset.Entry { return cardset.Entry{Name: n, Count: c} }
	for _, c := range []struct {
		name string
		a, b []cardset.Entry
	}{
		{"the trio", []cardset.Entry{e("Propaganda", 2), e("Archangel of Tithes", 2), e("Plains", 20)},
			[]cardset.Entry{e("Carnelian Orb of Dragonkind", 4), e("Mountain", 20)}},
		{"the trio in one deck", []cardset.Entry{e("Propaganda", 2), e("Archangel of Tithes", 2), e("Carnelian Orb of Dragonkind", 2), e("Mountain", 10), e("Plains", 10)},
			[]cardset.Entry{e("Propaganda", 2), e("Archangel of Tithes", 2), e("Carnelian Orb of Dragonkind", 2), e("Mountain", 10), e("Plains", 10)}},
		// a requirement beside the attack tax and the Orb; the block tax Archangel
		// also carries stays refused beside a MustAttack (combat_blocktax.go
		// blkTaxMustAttack), so Juggernaut sits with Propaganda only
		{"a requirement", []cardset.Entry{e("Propaganda", 4), e("Plains", 20)},
			[]cardset.Entry{e("Carnelian Orb of Dragonkind", 4), e("Juggernaut", 4), e("Mountain", 20)}},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, _, _, err := set.Matchup(c.a, c.b)
			var r *cardset.InteractionRefusal
			if errors.As(err, &r) {
				t.Fatalf("refused: %s: %v: %s", r.Reason, r.Cards, r.Why)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
