package oracle

import "testing"

// lockstepWithCard plays recorded games of synthetic pool name in lockstep on the
// surface cfg names, in each of which card reaches the stack, so a deal that drifts
// cannot move them.
func lockstepWithCard(t *testing.T, name string, cfg Config, card string) {
	t.Helper()
	pinnedGames(t, name, cfg, onStack(card))
}

// A may-play over a remembered nonland card reads the card's type once, as the
// record is made. It is no condition on the permanents its caster controls, so
// Cruelclaw's Heist's exiled card stays castable on a board of lands alone.
func TestRememberedNonlandPermissionOnAllLands(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-blb", Config{}, "Cruelclaw's Heist")
}

// Floating restricted mana pays an activated ability its restriction admits
// (CR 106.6): Flamebraider's mana is counted at the offer, comes off the plan
// and is spent first by the payment.
func TestFloatingRestrictedManaPaysAnActivation(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-ecl", Config{GorgeAutopaySurface: true}, "Flamebraider")
	lockstepWithCard(t, "std-ecl", Config{}, "Flamebraider")
}

// An attached mana grant counts beside the host's own ability (New Horizons on a
// Forest). A countered spell's "its controller" is the player who controlled it
// as it last existed, not its owner (CR 608.2h): An Offer You Can't Refuse's
// Treasures go to that player.
func TestAttachedGrantAndCounteredSpellController(t *testing.T) {
	t.Parallel()
	for _, cfg := range []Config{{GorgeAutopaySurface: true}, {}} {
		lockstepWithCard(t, "std-fdn", cfg, "New Horizons")
		lockstepWithCard(t, "std-fdn", cfg, "An Offer You Can't Refuse")
	}
}

// With a census no relaxed proof can run on, a hybrid cast is one of the four
// plays the planner searches, ahead of the battlefield's activations, so an
// activation past the fourth play is not offered.
func TestHybridCastTakesASearchSlotWhenUnrelaxable(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-spm", Config{GorgeAutopaySurface: true}, hybridCast)
}

// A plain cast beside a free one is a play of its own: gorge's potential planner
// prices the printed cost and leaves only the free cast unpriced, so with a
// mana-spent reader out (the cast planner declines every cast) the plain cast of
// Prismari Charm is offered beside Zaffai and the Tempests' free one.
func TestPlainCastBesideAFreeCastIsPricedLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "plain_cast_beside_free_cast_priced", "std-sos", Config{})
}
