package oracle

import "testing"

// lockstepWithCard plays games of synthetic pool name in lockstep on the surface cfg
// names. With a card named, the games are recorded ones in which it reaches the stack,
// so a deal that drifts cannot move them; with none, they are the pairs given.
func lockstepWithCard(t *testing.T, name string, cfg Config, card string, pairs ...int) {
	t.Helper()
	if card != "" {
		pinnedGames(t, name, cfg, onStack(card))
		return
	}
	synthPairsRun(t, name, RunConfig{Config: cfg}, pairs, nil)
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

// The prefix search activates every source the planner's census misses before it
// funds a fee: a restricted any-colour source, a source that taps another
// permanent and a granted mana ability all stand beside a filter as gaps.
func TestFilterSearchActivatesTheOtherGaps(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-spm", Config{GorgeAutopaySurface: true}, "", 419, 193)
	lockstepWithCard(t, "std-tla", Config{GorgeAutopaySurface: true}, "", 171)
}

// A filter's fee may be paid with the restricted mana its restriction admits, so
// that mana counts toward whether the filter can be funded at all: Surveillance
// Room's {1},{T} any-colour beside a restricted any-colour land.
func TestFilterFeeReadsRestrictedMana(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-msh", Config{}, onBattlefield("Surveillance Room"),
		anyOf(onBattlefield("Avengers Tower"), onBattlefield("Castle Doom"), onBattlefield("Villainous Hideout")))
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
		lockstepWithCard(t, "std-fdn", cfg, "New Horizons")
		lockstepWithCard(t, "std-fdn", cfg, "An Offer You Can't Refuse")
	}
}

// With a census no relaxed proof can run on, a hybrid cast is one of the four
// plays the planner searches, ahead of the battlefield's activations, so an
// activation past the fourth play is not offered.
func TestHybridCastTakesASearchSlotWhenUnrelaxable(t *testing.T) {
	t.Parallel()
	lockstepWithCard(t, "std-spm", Config{GorgeAutopaySurface: true}, "", 226)
}

// A plain cast beside a free one is a play of its own: gorge's potential planner
// prices the printed cost and leaves only the free cast unpriced, so with a
// mana-spent reader out (the cast planner declines every cast) the plain cast of
// Prismari Charm is offered beside Zaffai and the Tempests' free one.
func TestPlainCastBesideAFreeCastIsPricedLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "plain_cast_beside_free_cast_priced", "std-sos", Config{})
}
