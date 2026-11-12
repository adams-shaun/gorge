package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// The opponents'-production reflected mana (AB$ ManaReflected,
// ReflectProperty$ Produce, every Valid$ alternative OppCtrl: Exotic
// Orchard, Fellwar Stone, Quirion Explorer, Sylvok Explorer, "{T}: Add one
// mana of any color that a land an opponent controls could produce").
// gorge's planner census lists each source as one reflected unit over the
// colours its resolution's object walk names now
// (rules.PaymentPlanReflectedShape, planreflected.go); kgen lowers it as a
// lone AfManaReflect Ability with ManaProduce and a ManaAmong filter, and
// the kernel reads the union of the OPPONENTS' matching permanents' cards'
// ProduceCol into the imprint mox's per-colour views (dirac/mana_reflected.go
// produceColours). The decks carry few basics and lean on the reflected
// sources for their colours; the opponents' lands hold Island, Forest,
// Mountain and Command Tower, so the read moves across the table and
// mid-game. The fixture pool is dirac/cardset/pools/fixtures/orchard.
var orchardDecks = []fixtureDecks{{
	{
		{Name: "Fellwar Stone", Count: 6}, {Name: "Exotic Orchard", Count: 4}, {Name: "Quirion Explorer", Count: 4},
		{Name: "Sylvok Explorer", Count: 2}, {Name: "Isamaru, Hound of Konda", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Shock", Count: 6}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Exotic Orchard", Count: 6}, {Name: "Fellwar Stone", Count: 2}, {Name: "Sylvok Explorer", Count: 3},
		{Name: "Savannah Lions", Count: 3}, {Name: "Shock", Count: 7}, {Name: "Command Tower", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 2}, {Name: "Plains", Count: 1},
	},
}}

var orchardWatch = map[string]bool{
	"Fellwar Stone": true, "Exotic Orchard": true, "Quirion Explorer": true, "Sylvok Explorer": true, "Shock": true, "Lightning Bolt": true,
}

func TestOrchardLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "orchard", orchardDecks, orchardWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	// Exotic Orchard is a land: it is played, not cast, so the watch
	// policy's cast tally never counts it; its activations are covered by
	// the games themselves and by the mutation test's divergence.
	for _, n := range []string{"Fellwar Stone", "Quirion Explorer", "Sylvok Explorer"} {
		if run.casts[n] < 100 {
			t.Errorf("only %d %s casts", run.casts[n], n)
		}
	}
	if run.casts["Shock"] < 40 {
		t.Errorf("only %d Shock casts: the mana windows never ran", run.casts["Shock"])
	}
}

// TestOrchardLockstepMutation: the switch reads no colours, so the reflected
// sources add none (gorge taps one for a live land's colour). It must
// diverge.
func TestOrchardLockstepMutation(t *testing.T) {
	dirac.SetDebugReflMutate(1)
	defer dirac.SetDebugReflMutate(0)
	run := runFixtureLockstep(t, "orchard", orchardDecks, orchardWatch, 120, false, nil)
	run.requireDivergence(t, "the reflected sources read no colours")
}
