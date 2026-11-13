package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAttackTaxPerAttackerCounters: a dynamic StatAttackTax whose VA reads
// kop.RefTaxedAttacker (Nils, Discipline Enforcer's RememberingAttacker$
// Remembered$CardCounters.ALL charge) prices each attacking creature by its
// own counters: atkTaxCharge binds the priced creature (evalCtx.taxed1) and
// refCountersAll sums its live counters. A charge priced with the attacker
// unbound reads none.
func TestAttackTaxPerAttackerCounters(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	// X = all counters on the priced creature, read live (x.N 1)
	i := len(tbl.Exprs)
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExCountersOn, Ref: kop.RefTaxedAttacker, N: 1, Ctr: kop.CtrAll})
	va := kop.ValueRef{K: kop.VExpr, N: int16(i)}
	fid := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	taxer := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Per-Attacker Taxer", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatAttackTax, Filter: fid, VA: va, StFlags: kop.StDynamic | kop.StAtkTaxPlayer}}}})
	bearer := taxer + 1
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Counter Bearer", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTableLib(t, tbl, da[0], Config{NoElide: true})

	taxO := sc.perm(taxer, 0)
	taxSl := sc.s.objs[taxO].slot
	// PRECONDITION: the static is in force and the two creatures carry
	// different counter sums.
	if sc.k.t.Cards[sc.s.cardAt(taxSl)].Abilities[0].Static.Kind != kop.StatAttackTax {
		t.Fatalf("the taxer's first ability is not a StatAttackTax")
	}
	a1 := sc.perm(bearer, 1)
	a3 := sc.perm(bearer, 1)
	sl1, sl3 := sc.s.objs[a1].slot, sc.s.objs[a3].slot
	sc.s.perms[sl1].p1 = 1
	sc.s.perms[sl3].p1 = 3
	if sc.s.perms[sl1].p1 != 1 || sc.s.perms[sl3].p1 != 3 {
		t.Fatalf("precondition: counter sums %d and %d; want 1 and 3", sc.s.perms[sl1].p1, sc.s.perms[sl3].p1)
	}
	tgt := PlayerCode(0) // the taxer's controller: StAtkTaxPlayer
	for _, tc := range []struct {
		name string
		sl   Slot
		want int
	}{
		{"one counter", sl1, 1},
		{"three counters", sl3, 3},
	} {
		n, ok := sc.s.atkTaxCharge(sc.k, tc.sl, TCode(tgt))
		if !ok || n != tc.want {
			t.Fatalf("%s: charge %d ok %v; want %d", tc.name, n, ok, tc.want)
		}
	}
	// MUTATION: with the priced attacker left unbound (evalCtx.taxed1 0) the
	// charge reads none — the two creatures' different counters change
	// nothing, so this is not counting the reader's own source.
	st := statSrc{a: &sc.k.t.Cards[sc.s.cardAt(taxSl)].Abilities[0], sl: taxSl, ctrl: 0, obj: taxO}
	if v := sc.s.evalV(sc.k, st.a.Static.VA, sc.s.ctxStatic(st)); v != 0 {
		t.Fatalf("charge %d read with the priced attacker unbound; want 0", v)
	}
}
