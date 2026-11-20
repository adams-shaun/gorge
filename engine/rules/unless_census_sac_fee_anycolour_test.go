package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The unless census must price the self-sacrifice mana fee beside a colour
// chosen on activation (Chromatic Star's "{1}, {T}, Sacrifice Chromatic Star:
// Add one mana of any color"): the payer may activate mana abilities to meet
// an unless payment (CR 605.3a/b), so a fee the floating mana an earlier
// same-window activation produced can fund is a reachable pay offer, not a
// decline. kfuzz pair 37 of pool tail-62 (Vaporous Djinn's upkeep
// "phases out unless you pay {U}{U}"): gorge offered the election decline
// only, the kernel pay and decline.
func TestUnlessCensusPricesSacFeeAnyColour(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)

	// Island + Plains + Chromatic Star, empty pool: the Island's {U} floats,
	// the Plains' {W} funds the {1} fee and Chromatic Star adds the second
	// {U}, so {U}{U} is reachable and the pay offer must exist.
	e, _, _ := counterFixture(t, reg, "Mana Leak", "Grizzly Bears")
	e.G.Players[0].Pool = state.Mana{}
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Island"))
	onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Plains"))
	longSettled(t, e, onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Chromatic Star")))
	if !e.UnlessCostPayable(0, "U U") {
		t.Fatal("Island + Plains + Chromatic Star must reach {U}{U} inside an unless window")
	}

	// Island + Chromatic Star alone: the fee eats the Island's only {U}, so
	// one net {U} floats and {U}{U} stays unreachable — decline only.
	e2, _, _ := counterFixture(t, reg, "Mana Leak", "Grizzly Bears")
	e2.G.Players[0].Pool = state.Mana{}
	onBoardCard(t, e2, 0, mustCorpusCard(t, reg, "Island"))
	longSettled(t, e2, onBoardCard(t, e2, 0, mustCorpusCard(t, reg, "Chromatic Star")))
	if e2.UnlessCostPayable(0, "U U") {
		t.Fatal("Island + Chromatic Star alone must not reach {U}{U}: the {1} fee eats the Island's {U}")
	}

	// Island + Plains without Chromatic Star: the pre-census route, one {U}
	// beside two {W} — still unreachable, the new shape must not widen it.
	e3, _, _ := counterFixture(t, reg, "Mana Leak", "Grizzly Bears")
	e3.G.Players[0].Pool = state.Mana{}
	onBoardCard(t, e3, 0, mustCorpusCard(t, reg, "Island"))
	onBoardCard(t, e3, 0, mustCorpusCard(t, reg, "Plains"))
	if e3.UnlessCostPayable(0, "U U") {
		t.Fatal("Island + Plains must not reach {U}{U}")
	}
}

// longSettled clears the summoning-sickness gate onBoardCard sets: a source
// the census prices has been under its controller's control since a turn
// began, the shape the games this census serves present.
func longSettled(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	e.G.Obj(id).SummonSick = false
}
