package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestScriptSpendableKeepsNonPlainGap: the prefix search's gap filter drops a
// restricted any-colour source only for a plain cast or an activation
// (caSpendable; gorge's scriptSpendableGaps returns every gap for any other
// play). A bestowed cast is neither kind, so its restricted source stays a
// funder even though wideAdmits cannot read a bestow's restriction: the
// search taps it and the bestowed cast is offered once its mana floats.
//
// The source is a wide any-colour restricted land ({T}: Add two mana in any
// combination of colours. Spend this mana only to cast a spell.) so it is a
// scriptAnyR gap, not an each-colour census gap; the bestowed card's bestow
// cost is one green mana.
func TestScriptSpendableKeepsNonPlainGap(t *testing.T) {
	tbl := *v.Table
	tbl.Conds = []kop.Cond{{}} // a generated table (prepareGen's gate)
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	// an unrestricted restriction (any spell): the source is a restricted
	// any-colour gap whose mana every payment admits, so only the play's kind
	// can keep it out of the search
	tbl.Filters = append(tbl.Filters, kop.Filter{})
	restr := kop.FilterID(len(tbl.Filters) - 1)
	src := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Wide Source", Types: kop.TLand, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColAny, ManaN: 2,
			AFlags: kop.AfManaChoice, ManaRestrict: restr},
	}})
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	ench := kop.FilterID(len(tbl.Filters) - 1)
	var oneGreen kop.ManaCost
	oneGreen.Pips[kop.G] = 1
	bestowed := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Bestowed", Types: kop.TCreature, Cost: oneGreen,
		HasBestow: true, BestowCost: oneGreen, BestowTgt: kop.TargetSpec{Kind: kop.TgtObject, Filter: ench}})

	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.perm(v.GrizzlyBears, 0) // the bestowed spell's enchant-creature target
	srcO := sc.perm(src, 0)
	handO := sc.hand(bestowed, 0)
	s, k := sc.s, sc.k
	sl := sc.slot(srcO)
	a := MakeAction(AkBestow, TCode(handO), 0, 0)

	// preconditions: the source is a live any-colour gap and wideAdmits
	// cannot price it for the bestowed cast (a non-cast kind), so an
	// unguarded filter would drop it; the bestowed cast is not listed before
	// the source is tapped
	if !s.scriptAnyR(k, 0).has(sl) {
		t.Fatalf("precondition: the source is no live any-colour gap (anyR %v)", s.scriptAnyR(k, 0))
	}
	if s.wideAdmits(k, a, 0, slotBit(sl)) {
		t.Fatal("precondition: wideAdmits admits a bestowed cast; the kind guard is not exercised")
	}
	var d Decision
	s.plainOptions(k, &d, 0)
	if hasOption(&d, a) {
		t.Fatal("precondition: the bestowed cast is already offered before the source is tapped")
	}

	// the search's replay (scriptPrefixAny) must keep the gap and tap it
	s.scriptPrefixAny(k, a, 0, s.scriptRestr(k, 0), s.scriptAnyR(k, 0), s.scriptTapsDefer(k, 0),
		s.scriptCtr(k, 0), s.anyDifferentGaps(k, 0))
	if s.perms[sl].flags&pfTapped == 0 {
		t.Fatal("the prefix search dropped the bestowed cast's restricted any-colour gap and tapped nothing")
	}
	s.plainOptions(k, &d, 0)
	if !hasOption(&d, a) {
		t.Fatal("the tapped source's mana does not offer the bestowed cast")
	}
}
