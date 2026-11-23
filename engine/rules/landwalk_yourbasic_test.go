package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestLandwalkYourBasicDomain proves Magnigoth Treefolk's
// `AddKeyword$ Landwalk:YourBasic`: it has landwalk of each basic land type
// among lands ITS controller controls, so it is unblockable only while the
// defender controls a land of one of those types.
func TestLandwalkYourBasicDomain(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	tree := landwalkAttacker(t, e, corpusCardAtPath(t, "m/magnigoth_treefolk.txt"))
	blocker := onBoard(t, e, 0, "Name:Blocker\nManaCost:1\nTypes:Creature\nPT:2/2\nOracle:x\n")
	if b := e.G.Obj(blocker); b == nil || b.Zone != state.ZBattlefield || b.Controller != 0 {
		t.Fatalf("blocker precondition failed: %+v", e.G.Obj(blocker))
	}
	// The Treefolk controller owns no basic land yet: it has no landwalk, even
	// over a defender with a Forest.
	defForest := onBoard(t, e, 0, "Name:DefFor\nTypes:Basic Land Forest\nOracle:x\n")
	if e.G.Obj(defForest).Zone != state.ZBattlefield {
		t.Fatal("defender Forest precondition failed")
	}
	if !e.canBlock(blocker, tree) {
		t.Fatal("Treefolk was unblockable with no basic land type among its controller's lands")
	}
	// Its controller gains an Island: the defender has no Island, still blockable.
	onBoard(t, e, 1, "Name:MyIsle\nTypes:Basic Land Island\nOracle:x\n")
	if !e.canBlock(blocker, tree) {
		t.Fatal("Treefolk had islandwalk over a defender with no Island")
	}
	// Its controller gains a Forest: forestwalk, and the defender has a Forest.
	onBoard(t, e, 1, "Name:MyFor\nTypes:Basic Land Forest\nOracle:x\n")
	if e.canBlock(blocker, tree) {
		t.Fatal("Treefolk was blockable although it has forestwalk and the defender controls a Forest")
	}
}
