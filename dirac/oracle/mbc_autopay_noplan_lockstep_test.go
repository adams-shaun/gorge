package oracle

import (
	"testing"
)

// CR 601.2g/h and 107.4f: the only green the controller had left for Mutagenic
// Growth's announced {G} face was Gilanra, Caller of Wirewood's
// TriggersWhenSpent$ {G}, a deferred source the planner's census omits
// (mana_deferred.go). The pip ask counted it, so the green face was offered;
// the Phyrexian manual window's reach check did not, so it never opened and
// the cast's payment planned against no sources. The dirac kernel panicked
// with `kernel: autopay found no plan` at decision 412 of pool dr-mbc-baab
// (pair 120, seed 400742), parting from gorge, which cast it.
func TestMBCQuillmaneBakuReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mbc_autopay_noplan")
}
