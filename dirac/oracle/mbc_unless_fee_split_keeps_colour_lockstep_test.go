package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// CR 605.3a with CR 601.2h (through CR 602.2b): an unless cost's offer gate prices a mana source
// whose activation carries a generic mana fee from the floating mana the
// window produced first. The fee is the payer's to assign (CR 601.2h), so the
// reach search must be able to charge it to a colour the unless cost does not
// need. The board is kfuzz pair 49 of synthetic pool tail-49 (surface full,
// seed 218725): Eldest Dragon Highlander#9's upkeep trigger asks its
// controller to pay {W}{U}{B}{R}{G} or sacrifice it. The payer's only green
// source is Overflowing Basin#64, "{1}, {T}: Add {G}{U}"; the {1} must come
// from the second Mountain, leaving the Plains' {W} for the cost. The gate's
// fee transition spent the first available colour (white) on the {1}, found
// no white left, and offered decline only (decision 316); the kernel offered
// pay as well.
func TestMBCTail49ReprosLockstep(t *testing.T) {
	t.Parallel()
	if s := cardsettest.Set(t, "mbc"); s == nil {
		t.Fatal("precondition: the mbc card set is registered")
	}
	stdReproNamed(t, "unless_fee_split_keeps_colour")
}
