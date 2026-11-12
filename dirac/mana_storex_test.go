package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStoreManaGap: a storage-battery mana ability (SubCounter<X/STORAGE>,
// Amount$ X) removes every counter and makes that many mana, split over its
// production mask to cover the payment's remaining pips first. The real
// cards' lowering is dirac/kgen's storageland_test.go; this is the kernel
// primitive the lowering targets.
func TestStoreManaGap(t *testing.T) {
	tbl := &kop.Table{Cards: []kop.CardDef{{
		Name:  "Store Probe",
		Types: kop.TLand,
		Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostLoyaltyX, Ctr: kop.CtrStorage},
			ManaColors: kop.ColB | kop.ColR,
			AFlags:     kop.AfManaDynamic,
			ManaV:      kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrStorage)},
		}},
	}}}
	sc := newScenTableLib(t, tbl, 0, Config{})
	o := sc.perm(0, 0)
	sl := sc.slot(o)
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStorage), 3)
	ab := &tbl.Cards[0].Abilities[0]

	need := [kop.NumManaKinds]int{kop.B: 1, kop.R: 2}
	sc.s.storeGap(sc.k, 0, sl, ab, &need)
	if n := sc.s.perms[sl].ctrN(sc.s, int(kop.CtrStorage)); n != 0 {
		t.Fatalf("storage counters left %d, want 0", n)
	}
	if got := sc.s.pl[0].pool; got[kop.B] != 1 || got[kop.R] != 2 || got[kop.C] != 0 {
		t.Fatalf("pool = %v, want 1 B + 2 R", got)
	}
}
