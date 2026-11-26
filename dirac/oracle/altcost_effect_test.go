package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Effect-delivered AlternativeCost static lockstep (Marshland
// Bloodcaster's {1}{B},{T}: "Rather than pay the mana cost of the next spell
// you cast this turn, you may pay life equal to that spell's mana value";
// gorge alternativeCosts' registry arm, engine/rules/statics_altcost.go, CR
// 118.9, 119.4). Seat 0's deck carries no green source, so its Grizzly
// Bears' plain cast is never offered: the only cast is the alternative, paid
// in life, beside the Bloodcaster's activation. The grant ends at end of
// turn and on its controller's next completed cast, whether or not that cast
// used the alternative (gorge effectCastSweep; kernel embFlashForget). One
// Bloodcaster: a second live emblem would put a second identical
// alternative on gorge's list, which the kernel stalls on
// (StallEmbAltShared). The expected answer at every decision is gorge's.
var kembaltcostDecks = [2][]cardset.Entry{
	{
		{Name: "Marshland Bloodcaster", Count: 1}, {Name: "Grizzly Bears", Count: 9}, {Name: "Swamp", Count: 30},
	},
	{
		{Name: "Grizzly Bears", Count: 10}, {Name: "Forest", Count: 30},
	},
}

func TestEffectAltCostLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "embaltcost", kembaltcostDecks, 1000, "EMBALTCOST_SEEDS")
}

// TestEffectAltCostMutation proves the pool reaches the grant: with the
// kernel's emblem scan dropped, some seed diverges. The emblem exists only
// after the Bloodcaster's activation resolved, so a divergent game is one
// whose cast decision stands on the live emblem -- the activation resolved
// before that cast, and the kernel arm (cast_embaltcost.go) is what answers
// gorge's option.
func TestEffectAltCostMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugEmbAltCostOff = true
	defer func() { dirac.DebugEmbAltCostOff = false }()
	if lockstepDiverges(t, "embaltcost", kembaltcostDecks, 100) {
		return
	}
	t.Fatal("no seed diverged with the emblem scan dropped: the pool does not exercise the grant")
}
