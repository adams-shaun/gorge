package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDiesNCPrepOffOnFrozenPools: the FDN table has no dies trigger that can
// match a noncreature, so fireLeave's widened gate is never taken there. (The
// BLB table has one since lowered Ygra, Eater of All: "whenever a
// Food is put into a graveyard from the battlefield" matches a Food token.)
func TestDiesNCPrepOffOnFrozenPools(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	for _, f := range []func() (*kop.Table, error){set.Pool} {
		tb, err := f()
		if err != nil {
			t.Fatal(err)
		}
		var g genPrep
		prepDiesNC(tb, &g)
		if g.anyDiesNC {
			t.Fatal("anyDiesNC on a frozen pool")
		}
	}
}
