package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A gated larger alternative (Whisperer of the Wilds, Fanatic of Rhonas: "{T}:
// Add {G}" then Ferocious "{T}: Add {G}{G}", power 4 or greater). gorge's
// census lists the gate-on source as one unit with both alternatives, so its
// budget counts the larger one (attackSourceUnits: the max over a unit's
// alternatives) while the window taps the first listed, one {G} (CR 605.3a,
// 508.1h). The mutation (DebugAtkTaxGatedOne) counts the one mana the tap makes.
var atkTaxGatedDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2},
	{Name: "Whisperer of the Wilds", Count: 4}, {Name: "Fanatic of Rhonas", Count: 2},
	{Name: "Craw Wurm", Count: 3}, {Name: "Rumbling Baloth", Count: 3},
	{Name: "Savannah Lions", Count: 3}, {Name: "Elite Vanguard", Count: 3},
	{Name: "Forest", Count: 8}, {Name: "Plains", Count: 4},
}

// TestFourSeatAtkTaxGatedPairLockstep: four-seat attack-tax games with gated
// larger mana pairs play in lockstep, the window tapping them.
func TestFourSeatAtkTaxGatedPairLockstep(t *testing.T) {
	atkShapeLockstep(t, atkTaxGatedDeck, 40, 20, 0, []string{"Whisperer of the Wilds"},
		[]string{"Whisperer of the Wilds"}, &dirac.DebugAtkTaxGatedOne)
}

// A requirement an emblem makes (Bident of Thassa: "{1}{U}, {T}: Creatures your
// opponents control attack this turn if able", an Effect with a MustAttack
// static) beside an attack tax: CR 508.1d makes a requirement that would cost
// mana no requirement, so a creature is required only while one of its pairs is
// free (gorge attackDutyDischargeable), read from an emblem as from a card.
// The mutation (DebugAtkTaxRequiredPaid) requires a creature whose every pair
// costs mana.
var atkTaxEmblemDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2}, {Name: "Bident of Thassa", Count: 4},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 4}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Island", Count: 9}, {Name: "Plains", Count: 4},
}

// TestFourSeatAtkTaxEmblemDutyLockstep: four-seat attack-tax games with an
// emblem's attack requirement play in lockstep.
func TestFourSeatAtkTaxEmblemDutyLockstep(t *testing.T) {
	podClassSteer = mustAttackSteer
	defer func() { podClassSteer = nil }()
	atkShapeLockstep(t, atkTaxEmblemDeck, 40, 20, 0, []string{"Bident of Thassa"}, nil, &dirac.DebugAtkTaxRequiredPaid)
	t.Run("reached", func(t *testing.T) {
		st := runAtkShape(t, atkTaxEmblemDeck, 40, &dirac.DebugAtkTaxRequiredPaid, false)
		if st.tally["required attack offers"] < 2 {
			t.Errorf("a required attack offer reached %d times, want >= 2", st.tally["required attack offers"])
		}
	})
}

// A self-Taps watcher with no mana ability (Scaretiller: "Whenever Scaretiller
// becomes tapped, choose one ...") beside an attack tax: no window tap of a
// source fires it, gorge's window listing only permanents with a mana ability
// (CR 605.1a), so the walk reads it as any creature; its own attack tap fires
// the trigger as in a game with no tax (CR 508.1f). The mutation
// (DebugAtkTaxCountSelfTaps) lists it as a {C} source the window taps.
var atkTaxSelfTapsDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2}, {Name: "Scaretiller", Count: 5},
	{Name: "Savannah Lions", Count: 3}, {Name: "Elite Vanguard", Count: 3}, {Name: "Llanowar Elves", Count: 2},
	{Name: "Forest", Count: 8}, {Name: "Plains", Count: 6},
}

// TestFourSeatAtkTaxSelfTapsLockstep: four-seat attack-tax games with a
// self-Taps watcher play in lockstep.
func TestFourSeatAtkTaxSelfTapsLockstep(t *testing.T) {
	atkShapeLockstep(t, atkTaxSelfTapsDeck, 32, 20, 0, []string{"Scaretiller"}, nil, &dirac.DebugAtkTaxCountSelfTaps)
}

// A Condition-split source (River of Tears: "{T}: Add {U}. If you played a land
// this turn, add {B} instead"): gorge's window census prices it by the arm's
// production while the gate holds (condSplitRootSuperseded), so the tap floats
// {B} after a land drop and {U} before (CR 605.1a, 106.1). A tap that overpays
// (a bounce land beside it) leaves the arm's colour floating, which the next
// instant (Disfigure's {B}) spends. The mutation (DebugAtkTaxSplitRoot) reads
// the root's {U} after the land drop.
var atkTaxCondSplitDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2}, {Name: "River of Tears", Count: 5},
	{Name: "Savannah Lions", Count: 3}, {Name: "Elite Vanguard", Count: 3}, {Name: "Disfigure", Count: 4},
	{Name: "Azorius Chancery", Count: 3}, {Name: "Swamp", Count: 5}, {Name: "Island", Count: 3}, {Name: "Plains", Count: 3},
}

// TestFourSeatAtkTaxCondSplitLockstep: four-seat attack-tax games with a
// Condition-split source play in lockstep, the window tapping it.
func TestFourSeatAtkTaxCondSplitLockstep(t *testing.T) {
	atkShapeLockstep(t, atkTaxCondSplitDeck, 64, 20, 0, []string{"River of Tears"}, []string{"River of Tears"}, &dirac.DebugAtkTaxSplitRoot)
}

// A declared blocker the block window sacrifices (Eldrazi Skyspawner's Scion
// token: "Sacrifice this creature: Add {C}", beside Archangel of Tithes'
// "creatures can't block unless their controller pays {1} for each"). gorge's
// block window taps its sources in the census order (the free fixed ones, then
// the paid ones: a Scion) and commits the declaration after the payment
// (completeBlockPay), so a Scion that blocks pays for its own block and the
// attacker keeps the dead blocker in its block list: blocked, though nothing
// blocks it (events.foldDeclareBlockers; CR 509.1h). The mutation
// (DebugBlkTaxSacDropPair) leaves the attacker unblocked.
var blkTaxSelfSacDeck = []cardset.Entry{
	{Name: "Archangel of Tithes", Count: 3}, {Name: "Eldrazi Skyspawner", Count: 6},
	{Name: "Savannah Lions", Count: 4}, {Name: "Elite Vanguard", Count: 3}, {Name: "Grizzly Bears", Count: 2},
	{Name: "Plains", Count: 7}, {Name: "Island", Count: 4},
}

// TestFourSeatBlkTaxSelfSacLockstep: four-seat block-tax games with a creature
// source that pays by sacrifice play in lockstep.
func TestFourSeatBlkTaxSelfSacLockstep(t *testing.T) {
	atkShapeLockstep(t, blkTaxSelfSacDeck, 64, 5, 1, []string{"Eldrazi Skyspawner"}, nil, &dirac.DebugBlkTaxSacDropPair)
	t.Run("reached", func(t *testing.T) {
		st := runAtkShape(t, blkTaxSelfSacDeck, 64, &dirac.DebugBlkTaxSacDropPair, false)
		t.Logf("%s", katTally(st.tally))
		if st.tally["tap Eldrazi Scion Token"] < 1 {
			t.Errorf("a Scion was sacrificed by %d window asks, want >= 1", st.tally["tap Eldrazi Scion Token"])
		}
	})
}

// A self-Taps watcher a mana ability is granted (Careful Cultivation's
// "enchanted creature has {T}: Add {G}{G}"): the window taps it as a granted
// source and its trigger fires with the tap (CR 605.4a), as gorge's
// resolveManaAbilityRefOriginal fires it. The mutation (DebugAtkTaxNoGrant)
// lists no granted ability.
var atkTaxGrantedTapsDeck = []cardset.Entry{
	{Name: "Propaganda", Count: 3}, {Name: "Ghostly Prison", Count: 2}, {Name: "Scaretiller", Count: 5},
	{Name: "Careful Cultivation", Count: 5}, {Name: "Savannah Lions", Count: 2}, {Name: "Elite Vanguard", Count: 2},
	{Name: "Forest", Count: 8}, {Name: "Plains", Count: 6},
}

// TestFourSeatAtkTaxGrantedSelfTapsLockstep: four-seat attack-tax games with a
// Taps watcher tapped through a granted mana ability play in lockstep.
func TestFourSeatAtkTaxGrantedSelfTapsLockstep(t *testing.T) {
	atkShapeLockstep(t, atkTaxGrantedTapsDeck, 64, 10, 0, []string{"Scaretiller"}, []string{"Scaretiller"}, &dirac.DebugAtkTaxNoGrant)
}
