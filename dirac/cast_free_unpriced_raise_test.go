package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// freeRaiseTable is the vanilla table plus two free may-play statics (each a
// once-per-turn MayPlayLimit$ grant over exile) and one tax on the opponent's
// spells, so a free may-play cast from exile owes the tax (CR 601.2f, 118.9d).
func freeRaiseTable() (t *kop.Table, perm [2]CardID, tax CardID) {
	t = v.NewTable()
	t.Exprs = []kop.Expr{}
	for i := range perm {
		perm[i] = CardID(len(t.Cards))
		t.Cards = append(t.Cards, kop.CardDef{Name: "Test Free Permission " + string(rune('A'+i)), Types: kop.TEnchantment,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
				Kind: kop.StatMayPlayFrom, Zone: kop.ZoneExile,
				StFlags: kop.StOncePerTurn | kop.StWithoutManaCost}}}})
	}
	tax = CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test Tax", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{
			Kind: kop.StatIncreaseCost, A: 1, StFlags: kop.StRaiseOpp}}}})
	return t, perm, tax
}

// TestFreeCastAmbiguousRaiseFollowsPool: when gorge's potential planner leaves
// the plain may-play offer unpriced, the offer is listed only when the
// floating pool pays. Two limited free permissions over one exiled card are
// the planner's "ambiguous" shape (mayPlayOffers >= 2, no fold): a tax the
// empty pool cannot pay is not relaxed to an untapped source, as the ordinary
// walk does. With one permission the offer is priced, so the same source
// does pay the tax and the cast is listed.
func TestFreeCastAmbiguousRaiseFollowsPool(t *testing.T) {
	for _, tc := range []struct {
		name     string
		perms    int
		unpriced bool
		offered  bool
	}{
		{"two permissions", 2, true, false},
		{"one permission", 1, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl, perm, tax := freeRaiseTable()
			sc := newScenTable(t, tbl, Config{NoElide: true, GorgeUnpriced: true, GorgeLowering: true})
			s, k := sc.s, sc.k
			for i := 0; i < tc.perms; i++ {
				sc.perm(perm[i], 0)
			}
			sc.perm(tax, 1) // p1's tax: p0's free cast owes {1}
			sc.perm(v.Plains, 0)
			o := sc.obj(v.ThinkTwice, 0)
			s.place(o, kop.ZoneExile, false)

			var av avail
			s.availFor(k, 0, &av)
			if got := s.mayPlayOffers(k, 0, o); got != tc.perms {
				t.Fatalf("precondition: mayPlayOffers %d, want %d", got, tc.perms)
			}
			if s.mayPlayFolds(k, 0, o) {
				t.Fatal("precondition: a permission folds into another")
			}
			if got := s.mayPlayUnpriced(k, 0, o, false, &av); got != tc.unpriced {
				t.Fatalf("precondition: mayPlayUnpriced %v, want %v", got, tc.unpriced)
			}
			cost := s.adjustCostGen(k, 0, v.ThinkTwice, o, kop.ManaCost{})
			if cost != (kop.ManaCost{Generic: 1}) {
				t.Fatalf("precondition: the free cast owes %+v, want the tax's {1}", cost)
			}
			if !s.castAffordsR(k, 0, v.ThinkTwice, s.restrOriginOfObj(o), &cost, &av) {
				t.Fatal("precondition: the untapped Plains does not pay the tax")
			}
			if s.windowPoolPays(0, &cost, false) {
				t.Fatal("precondition: the empty pool pays the tax")
			}

			var d Decision
			s.priorityGen(k, &d, 0, true, &av, true)
			if got := hasMayCast(&d, o); got != tc.offered {
				t.Fatalf("free cast offered %v, want %v (unpriced %v); options %v", got, tc.offered, tc.unpriced, d.Options[:d.N])
			}
		})
	}
}
