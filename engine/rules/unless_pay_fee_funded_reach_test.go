package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestUnlessPayOfferedWithFeeSourceFundedByFreeMana pins the unless offer
// gate's fee walk: an untapped Island's {U} funds Desolate Mire's
// "{1}, {T}: Add {W}{B}" (the fee leaves the pool before the activation
// resolves, CR 605.3b), and the {W}{B} covers a generic "counter that spell
// unless its controller pays {2}" tax (CR 107.4). The gate once priced the
// tax as unpayable and offered decline only, stranding the payer in a hard
// counter dirac offers the pay branch for.
func TestUnlessPayOfferedWithFeeSourceFundedByFreeMana(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	mire := mustCorpusCard(t, reg, "Desolate Mire")
	island := mustCorpusCard(t, reg, "Island")
	if mire == nil || island == nil {
		t.Fatal("premise: corpus lacks Desolate Mire or Island")
	}

	e := handEngine(t, mire, island)
	e.G.Players[0].Pool = state.Mana{}
	mireID := onBoardCard(t, e, 0, mire)
	islandID := onBoardCard(t, e, 0, island)
	for _, id := range []state.ObjID{mireID, islandID} {
		if e.G.Obj(id).Zone != state.ZBattlefield || e.G.Obj(id).Tapped {
			t.Fatalf("payment source precondition failed for %d: %+v", id, e.G.Obj(id))
		}
	}
	if !e.untappedManaSource(0, islandID) {
		t.Fatal("Island is not a payment-window source -- premise false")
	}
	if !e.UnlessCostPayable(0, "2") {
		t.Fatal("{2} not payable from an Island's {U} funding Desolate Mire's {1} -> {W}{B}")
	}
}
