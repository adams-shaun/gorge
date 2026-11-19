package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestUnlessCostPayableFeeSourceBeforeFunder pins that the unless-cost reach
// walk does not depend on which source the census lists first: a Mire-like
// land ("{1}, {T}: Add {W}{B}") placed BEFORE the land whose {R} funds its fee
// still makes a {2} tax payable (CR 605.3b: the fee is paid from mana the
// other land floated first).
func TestUnlessCostPayableFeeSourceBeforeFunder(t *testing.T) {
	t.Parallel()
	mire := "Name:Fee Mire\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ 1 T | Produced$ W B\nOracle:x\n"
	bridge := "Name:Fee Bridge\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Combo R G\nOracle:x\n"
	e := handEngine(t)
	e.G.Players[0].Pool = state.Mana{}
	mireID := onBoard(t, e, 0, mire)
	bridgeID := onBoard(t, e, 0, bridge)
	if mireID >= bridgeID {
		t.Fatalf("precondition: the fee land must list first (ids %d, %d)", mireID, bridgeID)
	}
	for _, id := range []state.ObjID{mireID, bridgeID} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
			t.Fatalf("precondition: source %d not untapped on the battlefield: %+v", id, o)
		}
	}
	if !e.UnlessCostPayable(0, "2") {
		t.Fatal("the bridge's mana funds the mire's {1} fee and the mire floats {W}{B}: a {2} tax is payable")
	}
	if e.UnlessCostPayable(0, "3") {
		t.Fatal("the chain floats only two mana; a {3} tax is not payable")
	}
}
