package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCtrGapsOutsideGrantSearch: the grant search lists only grantStoreGaps (a
// storage battery with no plain first mana ability), so a battery that is the
// only gap is covered by it, but a ctrSecond battery (Mage-Ring Network's
// {T}: Add {C} beside its counter-X ability) is priced only by the counter-gap
// search and stays an outside gap.
func TestCtrGapsOutsideGrantSearch(t *testing.T) {
	t.Run("ctrSecond battery is outside the grant search", func(t *testing.T) {
		p := laterStoreProbeMatchup(t)
		sc := newScenTableLib(t, p.tbl, p.storage, Config{NoElide: true})
		o := sc.perm(p.storage, 0)
		sl := sc.slot(o)
		sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStorage), 2)
		if !sc.s.scriptCtr(sc.k, 0).has(sl) {
			t.Fatalf("precondition: the battery is not a counter gap")
		}
		if sc.s.grantStoreGaps(sc.k, 0).has(sl) {
			t.Fatalf("precondition: a ctrSecond battery must not be a grant-search candidate")
		}
		if !sc.s.ctrGapsOutsideGrantSearch(sc.k, 0) {
			t.Fatalf("a ctrSecond battery is priced only by the counter-gap search: it is an outside gap")
		}
	})
	t.Run("grant-search battery is not outside", func(t *testing.T) {
		p := storeProbeMatchup(t)
		sc := newScenTableLib(t, p.tbl, p.storage, Config{NoElide: true})
		o := sc.perm(p.storage, 0)
		sl := sc.slot(o)
		sc.s.perms[sl].flags &^= pfTapped
		sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrStorage), 2)
		if !sc.s.grantStoreGaps(sc.k, 0).has(sl) {
			t.Fatalf("precondition: Bottomless Vault must be a grant-search candidate")
		}
		if sc.s.ctrGapsOutsideGrantSearch(sc.k, 0) {
			t.Fatalf("a battery the grant search lists is not an outside gap")
		}
	})
}
