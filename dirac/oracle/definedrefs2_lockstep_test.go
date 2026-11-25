package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Defined$ referent lockstep, slice 2: the hand moves whose owner is a
// target's controller, a parent target or the upkeep's player (Divine
// Gambit, Metamorphose, Prying Questions, Braids), the phase-outs over a
// Defined$ host, target or Valid <filter> (Robe of Stars, Vanishing, Slip
// Out the Back, Taniwha), the source-first control exchanges (Avarice Totem,
// Phyrexian Infiltrator, Eyes Everywhere, Conjured Currency), the Aura's
// SacrificeAll of its host (Parasitic Implant, Slow Motion), the damaged
// creature's controller (Bellowing Fiend, Death Charmer, Flayed Nim, Soul
// Charmer) and the blocked attacker's controller (Meglonoth, Assembled Alphas).
var definedRefs2Decks = [2][]cardset.Entry{
	{{Name: "Metamorphose", Count: 2}, {Name: "Slow Motion", Count: 2}, {Name: "Divine Gambit", Count: 2}, {Name: "Robe of Stars", Count: 2},
		{Name: "Vanishing", Count: 2}, {Name: "Slip Out the Back", Count: 2},
		{Name: "Taniwha", Count: 2}, {Name: "Braids, Conjurer Adept", Count: 2}, {Name: "Eyes Everywhere", Count: 1},
		{Name: "Conjured Currency", Count: 1}, {Name: "Avarice Totem", Count: 2}, {Name: "Phyrexian Infiltrator", Count: 2},
		{Name: "Soul Charmer", Count: 2}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 5}},
	{{Name: "Bellowing Fiend", Count: 3}, {Name: "Death Charmer", Count: 3}, {Name: "Flayed Nim", Count: 3},
		{Name: "Parasitic Implant", Count: 3}, {Name: "Assembled Alphas", Count: 2}, {Name: "Meglonoth", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Prying Questions", Count: 2},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 1}},
}

func TestDefinedRefs2Lockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "definedrefs2", definedRefs2Decks, 200, "KDEFINEDREFS2_SEEDS")
}

// The seats swapped: the second deck's damage and block triggers on the
// first seat, the hand moves and exchanges on the second.
func TestDefinedRefs2SwappedLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "definedrefs2", [2][]cardset.Entry{definedRefs2Decks[1], definedRefs2Decks[0]}, 200, "KDEFINEDREFS2_SEEDS")
}
