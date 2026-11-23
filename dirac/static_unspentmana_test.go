package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestUnspentManaKeeps: a printed S:Mode$ UnspentMana static (CR 500.4,
// gorge unspentManaKeep) protects its pool slots at a step boundary: a
// ValidPlayer$ You + ManaType$ Green static keeps its controller's green and
// nothing else's, the Upwelling spelling (no ValidPlayer$, no ManaType$)
// keeps every seat's every slot, and a table without the static keeps
// nothing. The precondition each case pins is the boundary emptier itself
// (endStep drops every unprotected slot).
func TestUnspentManaKeeps(t *testing.T) {
	omnath, upwelling := CardID(len(v.Table.Cards)), CardID(len(v.Table.Cards)+1)
	tbl := *v.Table
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Omnath", Types: kop.TCreature, Power: 0, Toughness: 0,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatUnspentMana, A: int16(kop.ColG), StFlags: kop.StUnspentYou}}}},
		kop.CardDef{Name: "Test Upwelling", Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatUnspentMana, A: int16(kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG | kop.ColC)}}}})

	for _, c := range []struct {
		name                           string
		card                           CardID
		staticBearer                   bool
		want0G, want1G, want0R, want1R uint8
	}{
		{"You scope: controller's green only", omnath, true, 3, 0, 0, 0},
		{"bare spelling: every seat's every slot", upwelling, true, 3, 2, 1, 1},
		{"no static: nothing kept", 0, false, 0, 0, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc := newScenTable(t, &tbl, Config{NoElide: true})
			if c.staticBearer {
				sc.perm(c.card, 0)
			}
			// the precondition: the pools hold what the boundary must judge
			sc.s.pl[0].pool[kop.G], sc.s.pl[0].pool[kop.R] = 3, 1
			sc.s.pl[1].pool[kop.G], sc.s.pl[1].pool[kop.R] = 2, 1
			sc.s.endStep(kop.StepBeginCombat)
			p0, p1 := sc.s.pl[0].pool, sc.s.pl[1].pool
			if p0[kop.G] != c.want0G || p1[kop.G] != c.want1G || p0[kop.R] != c.want0R || p1[kop.R] != c.want1R {
				t.Fatalf("pools after endStep: seat0 G%d R%d, seat1 G%d R%d; want %d/%d, %d/%d",
					p0[kop.G], p0[kop.R], p1[kop.G], p1[kop.R], c.want0G, c.want0R, c.want1G, c.want1R)
			}
		})
	}
}

// TestUnspentManaKeepsThroughCleanup: the keep rides the cleanup boundary and
// the turn change too, so Upwelling's mana survives into the next turn and
// empties at the first boundary after the static's gate stops (its source
// leaves). The precondition is the ordinary cleanup emptier, which zeroes
// every pool in a table without the static.
func TestUnspentManaKeepsThroughCleanup(t *testing.T) {
	upwelling := CardID(len(v.Table.Cards))
	tbl := *v.Table
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Upwelling", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
			Kind: kop.StatUnspentMana, A: int16(kop.ColW | kop.ColU | kop.ColB | kop.ColR | kop.ColG | kop.ColC)}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	up := sc.perm(upwelling, 0)
	sc.s.pl[0].pool[kop.R] = 2
	// the cleanup emptier, with the keep: the mana rides into the next turn
	sc.s.endStep(kop.StepCleanup)
	if sc.s.pl[0].pool[kop.R] != 2 {
		t.Fatalf("cleanup emptied a protected slot: pool R %d", sc.s.pl[0].pool[kop.R])
	}
	sc.s.beginTurn(0)
	if sc.s.pl[0].pool[kop.R] != 2 {
		t.Fatalf("turn change emptied a protected slot: pool R %d", sc.s.pl[0].pool[kop.R])
	}
	// the source leaves; the next boundary empties everything
	sc.s.leave(sc.k, sc.slot(up), kop.ZoneGraveyard, false)
	sc.s.pl[0].pool[kop.R] = 2 // refill after the leave's own steps
	sc.s.endStep(kop.StepBeginCombat)
	if sc.s.pl[0].pool[kop.R] != 0 {
		t.Fatalf("pool kept after the static's source left: pool R %d", sc.s.pl[0].pool[kop.R])
	}
}
