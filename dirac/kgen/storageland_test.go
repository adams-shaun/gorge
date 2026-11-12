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

// TestStorageLandLowers: a single-colour storage-battery mana ability
// (SubCounter<X/STORAGE>, Amount$ X, no generic fee) lowers to the kernel's
// counter-X gap, whether it is the card's first mana ability (Bottomless
// Vault) or a later one beside a plain {T} first (Mage-Ring Network).
func TestStorageLandLowers(t *testing.T) {
	for _, c := range []struct {
		name   string
		src    string
		colors kop.ColorMask
	}{
		{"later", storeLandHead + "A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ C | Amount$ X | SpellDescription$ Make {C} for each counter taken off.\nSVar:X:Count$xPaid\nOracle:x\n", kop.ColC},
		{"first", "Name:Store Probe\nManaCost:no cost\nTypes:Land\nA:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ B | Amount$ X | SpellDescription$ Make {B} for each counter taken off.\nSVar:X:Count$xPaid\nOracle:x\n", kop.ColB},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, rs := kwardpowBuild(t, "Store Probe", c.src)
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

// TestStorageLandColourCombinationRefused: a combination (Molten Slagheap's
// Combo B R) or any-colour (Haruspex) production is a colour per counter the
// kernel's counter-X gap does not model, so it stays refused with a reason
// naming the shape.
func TestStorageLandColourCombinationRefused(t *testing.T) {
	for _, cols := range []string{"Combo B R", "Any"} {
		mana := "A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ " + cols + " | Amount$ X | SpellDescription$ Make X mana.\n"
		_, rs := kwardpowBuild(t, "Store Probe", storeLandHead+mana+"SVar:X:Count$xPaid\nOracle:x\n")
		if len(rs) != 1 || !strings.Contains(rs[0].Reason, "colour combination or any colour") {
			t.Fatalf("%s: refusals = %v, want the colour-combination reason", cols, rs)
		}
	}
}

// TestStorageLandFeesRefused: the generic fee beside an announced-X storage
// cost has no kernel unit, the first ability (Cost$ 1 SubCounter, the shape
// the kernel's mana walk refuses outright) as well as a later one beside a
// plain first. Both stay refused with a reason naming the cost.
func TestStorageLandFeesRefused(t *testing.T) {
	const fee = "A:AB$ Mana | Cost$ 1 SubCounter<X/STORAGE> | Produced$ B | Amount$ X | SpellDescription$ Make X mana.\n"
	for _, c := range []struct {
		name string
		src  string
		want string
	}{
		{"later", storeLandHead + fee + "SVar:X:Count$xPaid\nOracle:x\n", "announced-X counter cost beside a generic fee"},
		{"first", "Name:Store Probe\nManaCost:no cost\nTypes:Land\n" + fee + "SVar:X:Count$xPaid\nOracle:x\n", "the kernel's mana walk cannot activate it"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, rs := kwardpowBuild(t, "Store Probe", c.src)
			if len(rs) != 1 || !strings.Contains(rs[0].Reason, c.want) {
				t.Fatalf("refusals = %v, want %q", rs, c.want)
			}
		})
	}
}
