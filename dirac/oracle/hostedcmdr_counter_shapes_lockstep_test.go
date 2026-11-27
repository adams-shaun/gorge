package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander counter and animate shapes of ticket
// cli-20261127T171904Z-e493ca42, played decision by decision against gorge
// (gorge leads) through the cedh pod harness: each case builds its kernel
// table with kgen, so a case is RED while kgen still refuses the card, and
// fails on the first divergence once it lowers. The tally is the reached
// check: a game that never put the counters on the animated artifact cannot
// pass it.
//
//   - Rise and Shine: `SP$ Animate | ... | RememberAnimated$ True |
//     SubAbility$ DBPutCounter` over `DB$ PutCounter | Defined$ Remembered |
//     CounterType$ P1P1 | CounterNum$ 4`. The Animate's grant and its
//     remembered set are one op (OpOverride, W = RefRemembered, whose apply
//     tail joins each animated permanent to the resolution's set; gorge
//     effAnimate's rememberAnimated, engine/effects/combatfx.go:763), and the
//     chained PutCounter reads exactly those. Sol Ring is the victim: a
//     noncreature artifact that becomes a 0/0 and lives only because the four
//     +1/+1 counters landed, so a kernel that animates without remembering
//     leaves a dead 0/0 where gorge has a 4/4.
var hostedCmdrRiseAndShineDeck = []cardset.Entry{
	{Name: "Rise and Shine", Count: 8}, {Name: "Sol Ring", Count: 8}, {Name: "Island", Count: 24},
}

// TestHostedCmdrCounterShapesLockstep plays each lowered shape.
func TestHostedCmdrCounterShapesLockstep(t *testing.T) {
	t.Parallel()
	t.Run("rise_and_shine_animate_rememberanimated", func(t *testing.T) {
		d := hostedCmdrRiseAndShineDeck
		// an animated Sol Ring: a permanent named Sol Ring on the battlefield
		// holding +1/+1 counters (the chained PutCounter's four), the only way
		// a 0/0 artifact creature survives.
		tally := func(g *state.Game) int {
			n := 0
			for p := range g.Players {
				for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
					o := g.Obj(id)
					if o == nil || o.Face() == nil || o.Face().Name != "Sol Ring" {
						continue
					}
					if o.Counter("P1P1") >= 4 {
						n++
					}
				}
			}
			return n
		}
		got := cedhPodRun(t, "hcc-riseandshine", [][]cardset.Entry{d, d}, []string{"Rise and Shine"}, 40, tally)
		if got == 0 {
			t.Fatal("no game animated an artifact with +1/+1 counters on it: the RememberAnimated$ chain was never resolved")
		}
		t.Logf("%d animated artifacts with counters", got)
	})
}
