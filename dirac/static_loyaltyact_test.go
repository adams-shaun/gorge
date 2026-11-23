package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// numLoyaltyActTable is vanilla plus one planeswalker carrying a self-scoped
// StatNumLoyaltyAct static, the shape Urza, Planeswalker's printed line
// lowers to.
func numLoyaltyActTable(t *testing.T) (*kop.Table, CardID) {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	tbl.Conds = append([]kop.Cond(nil), v.Table.Conds...)
	self := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FSelf})
	w := v.Table.Cards[v.EmberWalker]
	w.Name = "Test Twice Walker"
	w.Abilities = append(append([]kop.Ability(nil), w.Abilities...),
		kop.Ability{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatNumLoyaltyAct, Filter: self}})
	id := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, w)
	return &tbl, id
}

// loyaltyActivate drives one +1 loyalty activation of o and resolves it.
func loyaltyActivate(t *testing.T, sc *scen, o Obj) {
	t.Helper()
	sc.do(t, AkActivate, TCode(uint8(o)), scAny)
	sc.do(t, AkTarget, scAny, TCode(PlayerTarget1))
	for i := 0; i < 8 && sc.s.sp != 0; i++ {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("resolving the loyalty ability: decision kind %d", d.Kind)
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if sc.s.sp != 0 {
		t.Fatalf("the loyalty ability did not resolve (%d stack entries)", sc.s.sp)
	}
}

// loyaltyOffered reports whether the priority decision offers an activation of o.
func loyaltyOffered(sc *scen, o Obj) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) {
			return true
		}
	}
	return false
}

// TestNumLoyaltyActTwice: the static raises one permanent's CR 606.3 limit to
// two activations, and a third is withheld. The cold count is the fact the
// limit reads, and the mutation (DebugNumLoyaltyActOff) restores the one.
func TestNumLoyaltyActTwice(t *testing.T) {
	tbl, id := numLoyaltyActTable(t)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.uses.has(useNumLoyaltyAct) {
		t.Fatal("precondition: the table does not arm the loyalty-activation count")
	}
	if got := len(coldLoyaltyAct.of(sc.s)); got != MaxPerm {
		t.Fatalf("precondition: cold loyalty region holds %d slots, want %d", got, MaxPerm)
	}
	w := sc.perm(id, 0)
	sl := sc.slot(w)
	if !loyaltyOffered(sc, w) {
		t.Fatal("precondition: the first loyalty ability is not offered")
	}
	loyaltyActivate(t, sc, w)
	if got := sc.s.loyaltyActUsed(sl); got != 1 {
		t.Fatalf("after one activation the count is %d, want 1", got)
	}
	if sc.s.loyaltyUsedAtLimit(sc.k, sl) {
		t.Fatal("the Twice$ static did not raise the limit: a second activation is refused")
	}
	if !loyaltyOffered(sc, w) {
		t.Fatal("a second loyalty ability is not offered under the static")
	}
	loyaltyActivate(t, sc, w)
	if got := sc.s.loyaltyActUsed(sl); got != 2 {
		t.Fatalf("after two activations the count is %d, want 2", got)
	}
	if !sc.s.loyaltyUsedAtLimit(sc.k, sl) {
		t.Fatal("a third loyalty activation is not refused at the raised limit")
	}
	if loyaltyOffered(sc, w) {
		t.Fatal("a third loyalty ability was offered")
	}
}

// TestNumLoyaltyActMutation: with the read switched off, the second activation
// is refused after one, proving the test above depends on the new read.
func TestNumLoyaltyActMutation(t *testing.T) {
	tbl, id := numLoyaltyActTable(t)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.uses.has(useNumLoyaltyAct) {
		t.Fatal("precondition: the table does not arm the loyalty-activation count")
	}
	w := sc.perm(id, 0)
	sl := sc.slot(w)
	old := DebugNumLoyaltyActOff
	DebugNumLoyaltyActOff = true
	defer func() { DebugNumLoyaltyActOff = old }()
	loyaltyActivate(t, sc, w)
	if !sc.s.loyaltyUsedAtLimit(sc.k, sl) {
		t.Fatal("the mutation left the Twice$ limit in force")
	}
	if loyaltyOffered(sc, w) {
		t.Fatal("the mutation offered a second loyalty ability")
	}
}
