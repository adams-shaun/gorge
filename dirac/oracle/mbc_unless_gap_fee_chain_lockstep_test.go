package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// CR 601.2g with CR 118.12: Mana Leak's "counter target spell unless its
// controller pays {3}" offer gate must price a mana source whose activation
// cost carries a mana fee, because the fee is payable from the floating mana
// an earlier same-window activation produced. The board is kfuzz pair 193 of
// pool dr-mbc-baab (surface autopay, seed 648290): the payer's floating pool
// is empty, Island#67 taps for {U} and Gilanra#59 for {G}, and Apprentice
// Wizard#41's "{U}, {T}: Add {C}{C}{C}" is a gap the gate once refused --
// its {U} fee is unpayable from the empty pool alone, but tapping the Island
// first floats exactly that {U}. Two mana of the three untapped sources plus
// the wizard's three make {3} payable, so the pay option is legal and the
// kernel offered it; gorge's unless window census withheld the wizard (a
// literal mana fee is a shape only the cast window priced) and offered
// decline only (decision 386).
func TestMBCManaLeakReprosLockstep(t *testing.T) {
	t.Parallel()
	if s := cardsettest.Set(t, "mbc"); s == nil {
		t.Fatal("precondition: the mbc card set is registered")
	}
	stdReproNamed(t, "unless_gap_fee_chain")
}
