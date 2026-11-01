package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The copy of an Aura spell (CR 707.10c, 608.3f):
// Slick Imitator's "{1}, sacrifice: copy target spell you control. You may
// choose new targets for the copy" on an Aura spell. gorge poses
// AskCopyTargets for the copy -- the Aura's "Enchant" restriction is its
// spell ability's target -- the inherited host first, then the other legal
// ones; the kernel resolved the copy at once (dirac/copy_aura.go).
// The deck is the speed-grant pool's: Slick Imitator needs speed 4 and a
// floating pool (Burning-Tree Emissary), Shock and Lightning
// Bolt raise the speed, and the Auras are Lightwheel Enhancements and Rancor.

var auraCopyDeck = []cardset.Entry{
	{Name: "Slick Imitator", Count: 4}, {Name: "Lightwheel Enhancements", Count: 4},
	{Name: "Rancor", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
	{Name: "Burning-Tree Emissary", Count: 6},
	{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 3},
	{Name: "Forest", Count: 3},
}

func TestAuracopyLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := speedGrantEnv(t)
	tally := &speedGrantTally{played: map[string]int{}, copySeen: map[state.ObjID]bool{}}
	games, report := speedGrantGamesOf(t, reg, set, [2][]cardset.Entry{auraCopyDeck, auraCopyDeck}, 128, tally)
	if report != "" {
		t.Fatal(report)
	}
	t.Logf("%d games, 0 divergences, %d Aura spell copies, plays %v", games, tally.auraCopies, tally.played)
	if tally.auraCopies < 5 {
		t.Errorf("only %d Aura spell copies, want >= 5", tally.auraCopies)
	}
}

// A copied Aura spell electing nothing (the old kernel) must diverge.
func TestAuracopyMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set := speedGrantEnv(t)
	dirac.DebugAuraCopyOld = true
	defer func() { dirac.DebugAuraCopyOld = false }()
	tally := &speedGrantTally{played: map[string]int{}, copySeen: map[state.ObjID]bool{}}
	games, report := speedGrantGamesOf(t, reg, set, [2][]cardset.Entry{auraCopyDeck, auraCopyDeck}, 128, tally)
	if report == "" {
		t.Fatalf("%d games with a copied Aura spell electing nothing: no divergence", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, report)
}
