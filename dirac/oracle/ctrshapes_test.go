package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Counter-shapes lockstep: Louisoix's Sacrifice
// (Activated,Triggered,Spell.nonCreature), Spider-Sense (Instant,Sorcery,
// Triggered), Fugitive Droid (TargetValidTargeting$) and Hurl into History
// (RememberCounteredCMC$ read back by Discover) against the Prodigals'
// activations, Man-o'-War and Elvish Visionary's enters triggers, burn aimed at
// the Ornithopter, creature spells and Divination. Swallowed by Leviathan and
// Thranduil's Decree are not in the pool: gorge diverges from Forge on both
// (the kgen refusals in lowerCtx.unless and the Counter case).
//
// Hurl and Louisoix's Sacrifice share a deck: Hurl's Discover can free-cast
// Louisoix's Sacrifice, whose either-or additional cost gorge prices against
// what the window can pay (altAddAsk) and aborts the cast of when neither part
// is payable (CR 733.1); the kernel follows (cast.go bsAltAdd, altAddPayable).
var kctrshapesDecks = [2][]cardset.Entry{
	{
		{Name: "Louisoix's Sacrifice", Count: 3}, {Name: "Hurl into History", Count: 3}, {Name: "Spider-Sense", Count: 3},
		{Name: "Fugitive Droid", Count: 3}, {Name: "Ornithopter", Count: 4}, {Name: "Island", Count: 24},
	},
	kctrshapesDeckB,
}

// kctrshapesSideDecks: the cards the same shapes lifted beside the first six
// (RememberCounteredCMC$ read back by tokens, cards, counters and
// a draw; TargetValidTargeting$ on two instants and two sacrifice abilities),
// against the same opponent.
var kctrshapesSideDecks = [2][]cardset.Entry{
	{
		{Name: "Access Denied", Count: 3}, {Name: "Overwhelming Intellect", Count: 3}, {Name: "Spell Swindle", Count: 3},
		{Name: "Draining Whelk", Count: 3}, {Name: "Keep Safe", Count: 3}, {Name: "Turn Aside", Count: 3},
		{Name: "Hydromorph Guardian", Count: 3}, {Name: "Hydromorph Gull", Count: 2}, {Name: "Ornithopter", Count: 4},
		{Name: "Island", Count: 13},
	},
	kctrshapesDeckB,
}

var kctrshapesDeckB = []cardset.Entry{
	{Name: "Prodigal Pyromancer", Count: 4}, {Name: "Elvish Visionary", Count: 4}, {Name: "Man-o'-War", Count: 3},
	{Name: "Shock", Count: 4}, {Name: "Divination", Count: 3}, {Name: "Grizzly Bears", Count: 4},
	{Name: "Hill Giant", Count: 3}, {Name: "Llanowar Elves", Count: 3}, {Name: "Mountain", Count: 6},
	{Name: "Forest", Count: 6},
}

func TestCtrshapesLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ctrshapes", kctrshapesDecks, 1000, "KCTRSHAPES_SEEDS")
}

// TestCtrshapesSideLockstep plays the cards the shapes lifted beside the
// first four (kctrshapesSideDecks); its games run long, so fewer seeds.
func TestCtrshapesSideLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ctrshapes", kctrshapesSideDecks, 250, "KCTRSHAPES_SIDE_SEEDS")
}

// TestCtrshapesLockstepMutation proves the two target shapes: with
// TgtStackObjTrig admitting activated abilities and TgtSpellTgting ignoring
// the targeting clause, some seed diverges.
func TestCtrshapesLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCtrShapesOff = true
	defer func() { dirac.DebugCtrShapesOff = false }()
	if !lockstepDiverges(t, "ctrshapes", kctrshapesDecks, 1000) {
		t.Fatal("no seed diverged with the shapes off: no activated ability met Spider-Sense, no untargeting spell met Fugitive Droid")
	}
}
