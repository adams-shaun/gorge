package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The collect-evidence cost lockstep (CR 701.59a): Forensic
// Researcher and Polygraph Orb pay it as an activation cost (a pick of any
// number of graveyard cards after the target, an activation gorge offers
// whatever the graveyard holds and reverses when it cannot reach the
// threshold); Izoni and Sample Collector pay it as a trigger's own optional
// cost (a short graveyard declines, one the threshold needs whole is exiled
// unasked, else the pick). The second deck's burn and removal fill both
// graveyards. The expected answer at every decision is gorge's.
var kcostevidenceDecks = [2][]cardset.Entry{
	{
		{Name: "Izoni, Center of the Web", Count: 4}, {Name: "Sample Collector", Count: 4},
		{Name: "Forensic Researcher", Count: 4}, {Name: "Polygraph Orb", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2}, {Name: "Giant Growth", Count: 2},
		{Name: "Forest", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Murder", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

func TestCostevidenceLockstep(t *testing.T) {
	var st dirac.CostEvidenceStats
	dirac.DebugCostEvidenceStats = &st
	defer func() { dirac.DebugCostEvidenceStats = nil }()
	lockstepSet(t, "costevidence", kcostevidenceDecks, 1000, "KCOSTEVIDENCE_SEEDS")
	t.Logf("costevidence: %d evidence activations, %d paid trigger-own evidence costs (%d auto-settled)",
		st.Activations.Load(), st.Paid.Load(), st.Auto.Load())
	if testing.Short() {
		return
	}
	if a, p, au := st.Activations.Load(), st.Paid.Load(), st.Auto.Load(); a < 20 || p < 20 || au < 5 {
		t.Fatalf("the pool barely pays evidence: %d activations (want >= 20), %d paid trigger costs (want >= 20), %d auto-settled (want >= 5)", a, p, au)
	}
}

// TestCostevidenceMutation proves the pool picks the evidence: with Done
// offered after any single pick, whatever the total, some seed diverges.
func TestCostevidenceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCostEvidenceAnyDone = true
	defer func() { dirac.DebugCostEvidenceAnyDone = false }()
	if !lockstepDiverges(t, "costevidence", kcostevidenceDecks, 200) {
		t.Fatal("no seed diverged with Done offered after any single evidence pick: the pool never picks evidence")
	}
}
