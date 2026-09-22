package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// These pins deliberately use the real face statics and change the objects
// the shared IsPresent$/PresentCompare$ grammar counts.
func TestVeteranBrawlers(t *testing.T) {
	e := layerEngine(t)
	v := onBoardCard(t, e, 0, corpusCard(t, "Veteran Brawlers"))
	blocker := onBoard(t, e, 0, "Name:Attacker\nTypes:Creature\nPT:2/2\nOracle:x\n")
	land := onBoard(t, e, 0, "Name:Untapped Land\nTypes:Land\nPT:0/0\nOracle:x\n")
	e.G.Obj(v).SummonSick = false
	e.G.Obj(blocker).SummonSick = false
	if e.G.Obj(v).Zone != state.ZBattlefield || e.G.Obj(land).Zone != state.ZBattlefield {
		t.Fatal("fixture did not put Veteran Brawlers and land on battlefield")
	}
	if !e.blockRestricted(v, blocker) {
		t.Fatal("Veteran Brawlers was blockable with an untapped land")
	}
	e.G.Obj(land).Tapped = true
	if e.blockRestricted(v, blocker) {
		t.Fatal("Veteran Brawlers remained unable to block without an untapped land")
	}
}

func TestVortexRunner(t *testing.T) {
	e := layerEngine(t)
	runner := onBoardCard(t, e, 0, corpusCard(t, "Vortex Runner"))
	blocker := onBoard(t, e, 1, "Name:Blocker\nTypes:Creature\nPT:2/2\nOracle:x\n")
	for i := 0; i < 7; i++ {
		onBoard(t, e, 0, "Name:Runner Land\nTypes:Land\nPT:0/0\nOracle:x\n")
	}
	if e.blockRestricted(blocker, runner) {
		t.Fatal("Vortex Runner was unblockable below eight lands")
	}
	onBoard(t, e, 0, "Name:Eighth Runner Land\nTypes:Land\nPT:0/0\nOracle:x\n")
	e.staticEpoch = -1
	if !e.blockRestricted(blocker, runner) {
		t.Fatal("Vortex Runner was blockable at eight lands")
	}
}

func TestMaraudingMaulhorn(t *testing.T) {
	e := layerEngine(t)
	m := onBoardCard(t, e, 0, corpusCard(t, "Marauding Maulhorn"))
	e.G.Obj(m).SummonSick = false
	if !e.mustAttackRequired(m) {
		t.Fatal("Marauding Maulhorn was not required to attack without Advocate")
	}
	onBoardCard(t, e, 0, corpusCard(t, "Advocate of the Beast"))
	if e.mustAttackRequired(m) {
		t.Fatal("Marauding Maulhorn remained required with Advocate controlled")
	}
}

func TestBlizzard(t *testing.T) {
	e := handEngine(t, corpusCard(t, "Blizzard"))
	blizzard := e.G.Zone(state.ZHand, 0)[0]
	addMana(t, e, 0, "GGGG")
	if castOptionNamed(e, blizzard) != nil {
		t.Fatal("Blizzard was offered without a snow land")
	}
	onBoard(t, e, 0, "Name:Snow Land\nTypes:Snow Land\nPT:0/0\nOracle:x\n")
	e.staticEpoch = -1
	e.priorityRound()
	if castOptionNamed(e, blizzard) == nil {
		t.Fatal("Blizzard was not offered with a snow land")
	}
}
