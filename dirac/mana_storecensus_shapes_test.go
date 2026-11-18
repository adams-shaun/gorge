package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStoreCensusGapShapes pins storeCensusGap's two battery shapes: a lone
// storage battery (Bottomless Vault: only "{T}, remove X: Add {B} for each")
// leaves gorge's window census incomplete, while a Mage-Ring Network (a plain
// {T}: Add {C} first, the counter ability ctrSecond) does not: gorge prices the
// plain unit, so its census stays complete (CR 601.2h).
func TestStoreCensusGapShapes(t *testing.T) {
	lone := storeProbeMatchup(t)
	later := laterStoreProbeMatchup(t)
	for _, tc := range []struct {
		name string
		tbl  *kop.Table
		card CardID
		want bool
	}{
		{"a lone battery", lone.tbl, lone.storage, true},
		{"a battery beside a plain mana ability", later.tbl, later.storage, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc := newScenTableLib(t, tc.tbl, tc.card, Config{NoElide: true})
			o := sc.perm(tc.card, 0)
			sl := sc.slot(o)
			sc.s.perms[sl].flags &^= pfTapped // Bottomless Vault enters tapped
			sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStorage), 1)
			if !sc.s.scriptCtr(sc.k, 0).has(sl) {
				t.Fatalf("precondition: the battery is not an activatable counter source")
			}
			if got := sc.k.ctrSecond(sc.s.cardAt(sl)); got != !tc.want {
				t.Fatalf("precondition: ctrSecond = %v, want %v", got, !tc.want)
			}
			if got := sc.s.storeCensusGap(sc.k, 0); got != tc.want {
				t.Fatalf("storeCensusGap = %v, want %v", got, tc.want)
			}
		})
	}
}
