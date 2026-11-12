package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

const storeLandHead = `Name:Store Probe
ManaCost:no cost
Types:Land
A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Produce one {C} for the pool.
A:AB$ PutCounter | Cost$ 1 T | CounterType$ STORAGE | CounterNum$ 1 | SpellDescription$ Put one storage counter here.
`

// TestStorageLandLowers: a storage-battery mana ability (SubCounter<X/STORAGE>,
// Amount$ X) lowers to the kernel's counter-X gap. The production may be one
// colour or a two-colour combination; a generic fee is a different shape.
func TestStorageLandLowers(t *testing.T) {
	for _, c := range []struct {
		name   string
		mana   string
		colors kop.ColorMask
	}{
		{"colourless", "A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ C | Amount$ X | SpellDescription$ Make {C} for each counter taken off.\n", kop.ColC},
		{"two colours", "A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ Combo B R | Amount$ X | SpellDescription$ Make X mana, {B} and-or {R} in any mix.\n", kop.ColB | kop.ColR},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, rs := kwardpowBuild(t, "Store Probe", storeLandHead+c.mana+"SVar:X:Count$xPaid\nOracle:x\n")
			if len(rs) != 0 {
				t.Fatalf("refused: %v", rs)
			}
			var store *kop.Ability
			for j := range d.Abilities {
				if kop.StoreManaAb(&d.Abilities[j]) {
					store = &d.Abilities[j]
				}
			}
			if store == nil {
				t.Fatalf("no StoreManaAb in %+v", d.Abilities)
			}
			if store.Cost.Flags&kop.CostLoyaltyX == 0 || store.Cost.Ctr != kop.CtrStorage || store.ManaColors != c.colors ||
				store.ManaV.K != kop.VCountersSelf || store.ManaV.N != int16(kop.CtrStorage) {
				t.Fatalf("store ability = %+v", *store)
			}
		})
	}
}

// TestStorageLandFeeRefused: the generic fee beside an announced-X storage
// cost (Molten Slagheap and kin) has no kernel autopay unit; it stays refused
// with a reason naming the shape.
func TestStorageLandFeeRefused(t *testing.T) {
	const mana = "A:AB$ Mana | Cost$ 1 SubCounter<X/STORAGE> | Produced$ Combo B R | Amount$ X | SpellDescription$ Make X mana, {B} and-or {R} in any mix.\n"
	_, rs := kwardpowBuild(t, "Store Probe", storeLandHead+mana+"SVar:X:Count$xPaid\nOracle:x\n")
	if len(rs) != 1 || !strings.Contains(rs[0].Reason, "announced-X counter cost beside a generic fee") {
		t.Fatalf("refusals = %v, want the generic-fee reason", rs)
	}
}
