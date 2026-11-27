package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestUnleashGrantAgainstLossOfAbilities: a printed-Unleash blocker carrying a
// +1/+1 counter that lost all its abilities blocks again (Stasis Field), but a
// Continuous Unleash grant made AFTER the loss holds (CR 613.7), so it can't
// block; a grant older than the loss grants nothing. The printed branch of
// unleashNoBlock must reach the grant walk.
func TestUnleashGrantAgainstLossOfAbilities(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	plains := CardID(0)
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "Plains" {
			plains = CardID(i)
		}
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	flt := kop.FilterID(len(tbl.Filters) - 1)
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Unleasher", Types: kop.TCreature, Power: 2, Toughness: 2, Unleash: true},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Grantor", Types: kop.TEnchantment, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, Filter: flt, Unleash: true}}}})
	permOv := len(tbl.Overrides)
	tbl.Overrides = append(tbl.Overrides, kop.Override{LoseAbilities: true})
	sc := newScenTableLib(t, tbl, plains, Config{NoElide: true})
	s, k := sc.s, sc.k
	if !k.unleashGrant {
		t.Fatal("precondition: the table does not arm the grant walk")
	}
	a := sc.slot(sc.perm(n, 0))
	att := sc.slot(sc.perm(n+1, 1))
	s.perms[a].addCtrN(s, int(kop.CtrP1P1), 1)
	if s.canBlock(k, a, att) {
		t.Fatal("precondition: a printed-Unleash blocker with a counter blocked")
	}
	g1 := sc.slot(sc.perm(n+2, 0)) // a grant OLDER than the loss
	s.applyOverrideFl(k, a, permOv, n, s.ovDurFl(kop.DurPermanent))
	if _, lost := s.lossAt(k, a); !lost {
		t.Fatal("precondition: the loss of abilities did not take hold")
	}
	if !s.canBlock(k, a, att) {
		t.Error("a grant older than the loss of abilities still gated the block")
	}
	g2 := sc.slot(sc.perm(n+2, 0)) // a grant NEWER than the loss
	if s.grantTs(statSrc{sl: g2}) <= s.grantTs(statSrc{sl: g1}) {
		t.Fatal("precondition: the second grantor is not newer")
	}
	if s.canBlock(k, a, att) {
		t.Error("a grant made after the loss of abilities did not gate the block (CR 613.7)")
	}
}
