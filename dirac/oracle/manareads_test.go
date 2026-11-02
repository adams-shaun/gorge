package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// lockstepWithCard plays the pairs of synthetic pool name in lockstep on the
// surface cfg names and requires card, when one is named, on the stack in each
// game, so a pool that drifts away from the rule fails here rather than passing
// without playing it.
func lockstepWithCard(t *testing.T, name string, cfg Config, card string, pairs ...int) {
	t.Helper()
	stacked := 0
	rc := RunConfig{Config: cfg}
	if card != "" {
		rc.Finish = func(g *GorgeSide) {
			for i := range g.E.L.Events {
				ev := &g.E.L.Events[i]
				if ev.To != state.ZStack {
					continue
				}
				if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == card {
					stacked++
					return
				}
			}
		}
	}
	synthPairsRun(t, name, rc, pairs, func(pair int, res *Result) {
		if card != "" && stacked == 0 {
			t.Errorf("%s pair %d: %s never reached the stack", name, pair, card)
		}
		stacked = 0
	})
}

// A may-play over a remembered nonland card reads the card's type once, as the
// record is made. It is no condition on the permanents its caster controls, so
// Cruelclaw's Heist's exiled card stays castable on a board of lands alone.
func TestRememberedNonlandPermissionOnAllLands(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-blb", Config{}, "Cruelclaw's Heist", 153, 316, 332)
}

// Floating restricted mana pays an activated ability its restriction admits
// (CR 106.6): Flamebraider's mana is counted at the offer, comes off the plan
// and is spent first by the payment.
func TestFloatingRestrictedManaPaysAnActivation(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-ecl", Config{GorgeAutopaySurface: true}, "Flamebraider", 118)
	lockstepWithCard(t, "std-ecl", Config{}, "Flamebraider", 335)
}

// The prefix search activates every source the planner's census misses before it
// funds a fee: a restricted any-colour source, a source that taps another
// permanent and a granted mana ability all stand beside a filter as gaps.
func TestFilterSearchActivatesTheOtherGaps(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-spm", Config{GorgeAutopaySurface: true}, "", 419, 193)
	lockstepWithCard(t, "std-tla", Config{GorgeAutopaySurface: true}, "", 171)
}

// A filter's fee may be paid with the restricted mana its restriction admits, so
// that mana counts toward whether the filter can be funded at all.
func TestFilterFeeReadsRestrictedMana(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-msh", Config{}, "", 272)
}

// Under an any-type rider a pip is spent from the first of W U B R G C the pool
// holds, and only then does the generic part take what is left, so {C} survives
// a pip a coloured unit could pay.
func TestAnyTypeRiderSpendsPipsBeforeGeneric(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-spm", Config{}, "", 15)
}

// An attached mana grant counts beside the host's own ability (New Horizons on a
// Forest). A countered spell's "its controller" is the player who controlled it
// as it last existed, not its owner (CR 608.2h): An Offer You Can't Refuse's
// Treasures go to that player.
func TestAttachedGrantAndCounteredSpellController(t *testing.T) {
	t.Parallel()
	for _, cfg := range []Config{{GorgeAutopaySurface: true}, {}} {
		lockstepWithCard(t, "std-fdn", cfg, "New Horizons", 486)
		lockstepWithCard(t, "std-fdn", cfg, "An Offer You Can't Refuse", 486)
	}
}

// With a census no relaxed proof can run on, a hybrid cast is one of the four
// plays the planner searches, ahead of the battlefield's activations, so an
// activation past the fourth play is not offered.
func TestHybridCastTakesASearchSlotWhenUnrelaxable(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-spm", Config{GorgeAutopaySurface: true}, "", 226)
}

// A plain cast beside a free one shares one potential play's identity, so the
// planner calls the pair ambiguous and prices neither (Zaffai and the Tempests).
func TestPlainCastBesideAFreeCastIsNotLowered(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-sos", Config{}, "Zaffai and the Tempests", 69)
}
