package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The TapAll lockstep (CR 603.2c, Mode$ TapAll): Deeproot Pilgrimage and
// Rewrite History hear "one or more" of their permanents become tapped.
// gorge queues each once per tapping action -- the declare-attackers taps,
// Sleep's resolution, a two-target Frost Breath, Stoke the Flames' convoke
// taps, a crew cost, Springleaf Drum's tapped creature -- and once per tap
// outside one (an Elf tapped for mana, a single tap).

var tapAllDecks = [2][]cardset.Entry{
	{
		{Name: "Deeproot Pilgrimage", Count: 3}, {Name: "Rewrite History", Count: 3},
		{Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Coral Merfolk", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Springleaf Drum", Count: 2},
		{Name: "Sleek Schooner", Count: 2}, {Name: "Frost Breath", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Forest", Count: 7},
	},
	{
		{Name: "Rewrite History", Count: 3}, {Name: "Sleep", Count: 2}, {Name: "Stoke the Flames", Count: 3},
		{Name: "Coral Merfolk", Count: 4}, {Name: "Merfolk of the Pearl Trident", Count: 3},
		{Name: "Hill Giant", Count: 3}, {Name: "Raging Goblin", Count: 4}, {Name: "Smuggler's Copter", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 7},
	},
}

func TestTapAllLockstep(t *testing.T) {
	t.Parallel()
	j0 := dirac.DebugTapAllJoins
	// serial: DebugTapAllJoins is a plain package int
	lockstepSetSerial(t, "tapall", tapAllDecks, 300, "KTAPALL_SEEDS")
	if joins := dirac.DebugTapAllJoins - j0; joins < 20 {
		t.Errorf("%d taps joined a TapAll instance already queued in their batch, want at least 20", joins)
	}
}

// TestTapAllLockstepMutation: queued once per tap, batches ignored, the
// carriers make some seed diverge, so the pool does exercise the batches.
func TestTapAllLockstepMutation(t *testing.T) {
	dirac.DebugTapAllPerTap = true
	defer func() { dirac.DebugTapAllPerTap = false }()
	if n := lockstepRun(t, "tapall", tapAllDecks, 100, false, nil); n == 0 {
		t.Fatal("DebugTapAllPerTap: no seed diverged")
	}
}
