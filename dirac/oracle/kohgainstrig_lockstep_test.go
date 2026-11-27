package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Koh, the Face Stealer gained-trigger pool (dirac/kgen/lower_gainsof.go,
// the GainsTriggerAbsOf$ half; the kernel's gained-trigger walk is
// dirac/trig_gainedtrig.go): Koh exiles up to one other creature, and once its
// Pay-life ChooseCard activation names the last chosen card, Koh has all that
// card's activated and triggered abilities. Blood Artist is the lender with a
// trigger that fires from exile: its "whenever ~ or another creature dies,
// target player loses 1 life and you gain 1 life" line is the granted-trigger
// walk's assertion, and its Card.Self is the trigger's source, Koh (gorge
// checkGrantedStaticTriggersUsing). Serendib Efreet is the battlefield lender
// with an upkeep line, and its self-burn is how seat 1's creatures die, so the
// granted queue carries a battlefield as well as an exile-zone shape and each
// must resolve as gorge's does. The activated half of the grant is
// static_gainsof.go's offer walk, not this pool's.

func kohGainsTrigDecks() [2][]cardset.Entry {
	a := []cardset.Entry{
		{Name: "Koh, the Face Stealer", Count: 10},
		{Name: "Blood Artist", Count: 12}, {Name: "Swamp", Count: 18},
	}
	b := []cardset.Entry{
		{Name: "Serendib Efreet", Count: 16}, {Name: "Island", Count: 24},
	}
	return [2][]cardset.Entry{a, b}
}

func TestKohGainsTrigLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "kohgainstrig", kohGainsTrigDecks(), 200, "KOHGAINSTRIG_SEEDS")
	// The castFirst bot never activates Koh's Pay-life naming, so the
	// coverage bot plays the pool too: it tries every ability, naming a
	// lender and firing the gained triggers (the walk's queued arm is
	// exercised; the Mutation test drops it and diverges under this policy).
	if n := lockstepRunPolicy(t, "kohgainstrig", kohGainsTrigDecks(), 200, false, nil, nil, ExplorePolicy); n > 0 {
		t.Fatalf("kohgainstrig: %d seed(s) diverged under the coverage bot", n)
	}
}

// TestKohGainsTrigLockstepMutation: with the kernel's gained-trigger walk
// dropped, some seed diverges, so the pool reaches a lender whose trigger
// fires as Koh's (the Blood Artist death drain).
func TestKohGainsTrigLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoGainsTrig = true
	defer func() { dirac.DebugNoGainsTrig = false }()
	// ExplorePolicy: the castFirst bot never names a lender, so the walk is
	// only reached under the coverage bot (which tries every ability).
	if n := lockstepRunPolicy(t, "kohgainstrig", kohGainsTrigDecks(), 200, false, nil, nil, ExplorePolicy); n > 0 {
		return
	}
	t.Fatal("no seed diverged with the gained-trigger walk dropped: the pool never reaches it")
}
