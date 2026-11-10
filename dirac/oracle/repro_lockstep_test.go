package oracle

import (
	"github.com/adams-shaun/gorge/dirac"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// synthReproLockstep replays a recorded game on a synthetic pool's table, past its
// script, and fails on any divergence. A refused card plays: the fixture names the
// game that found a difference, whatever the list says today.
func synthReproLockstep(t *testing.T, file, pool string, cfg Config) {
	t.Helper()
	r, err := ReadRepro("testdata/" + file + ".repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set := cardsettest.Synth(t, pool)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	rc := RunConfig{Config: cfg, MaxDecisions: len(r.Actions) + 200}
	script := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, script, rc)
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n gorge %v\n dirac %v\n %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions, res.Div.Diff)
	}
	if res.Stall == KernelStallPrefix+"tapextra" {
		t.Fatalf("the kernel stalled on a tap's extra mana (tapextra) after %d decisions", len(res.Actions))
	}
}

// A trigger exiling the object that fired it, then returning it: Salvation Swan and a
// Bird entering, S.H.I.E.L.D. Flying Car's trigger copied onto the crewed Car. Both
// engines return the object at the end step, and its entering trigger follows.
func TestDelayedReturnOfTheObjectItsTriggerExiledLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "swan_exiles_its_bird", "std-blb", autopay)
	synthReproLockstep(t, "flying_car_exiles_itself", "std-msh", autopay)
}

// A land with an as-enters choice found for the battlefield, by a reveal-until (Clifftop
// Lookout and Uncharted Haven) and a dig (Fecund Greenshell and Three Tree City): both
// engines ask with it and the revealed cards still in the library.
func TestFoundCardsAsEntersAskInTheLibraryLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "clifftop_found_land_asks", "std-blb", autopay)
	synthReproLockstep(t, "greenshell_dug_land_asks", "std-blb", autopay)
}

// Scavenger's Talent's cost sacrifices a token copy, which stays in the graveyard until the
// state-based actions: no pick from there offers it (CR 707.10h, gorge's copyCeased).
func TestSacrificedTokenCopyIsNoPickLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "token_copy_not_a_pick", "std-blb", Config{})
}

// For the Common Good with a Treasure as its only target. A witness that sacrifices it pays
// nothing, so the cast needs lands for its {G}, and then pays the largest X that keeps it.
func TestXWitnessKeepsTheOnlyTargetLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "common_good_only_treasure", "std-blb", Config{})
	synthReproLockstep(t, "common_good_keeps_treasure", "std-blb", Config{})
}

// Dewdrop Cure returns two Lifecreed Duos, or a Duo beside other creatures: the targets
// enter together, so each Duo sees the others (CR 603.6a).
func TestTargetsReturnedTogetherSeeEachOtherLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "dewdrop_two_duos", "std-blb", autopay)
	synthReproLockstep(t, "dewdrop_duo_last", "std-blb", autopay)
}

// Hunter's Talent with no opposing creature for its chained target: Emberheart Challenger,
// answered as its root target, did not become a target (CR 603.3d).
func TestChainWithoutATargetFiresNoValiantLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "hunters_talent_no_chain_target", "std-blb", Config{GorgeAutopaySurface: true})
}

// Baylen with too few tokens to tap cannot be activated, so gorge's census misses nothing
// there and Hidden Grotto's filter takes the prefix search.
func TestDeadTapAnotherSourceLeavesAFilterBoardLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "baylen_no_tokens_grotto", "std-blb", Config{})
}

// Cindering Cutthroat's {1}{B/R} answered red with only Three Tree Mascot to make it: gorge's
// manual window taps a Swamp for the Mascot's fee, activates it, and taps on for the {1}.
func TestWindowActivatesAFirstFilterLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "mascot_window_filter", "std-blb", Config{})
}

// A Treasure sacrificed for a lowered cast's witness under Scavenger's Talent: the mill
// trigger resolves before the cast, its target gorge's default answer.
func TestWitnessSacrificeMillsBeforeTheCastLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "witness_sac_mill_trigger", "std-blb", Config{})
}

// An Adventure creature's face cast paid with a Treasure under The Sackville-Bagginses: the
// witness is tapped at priority, so the Treasure's sacrifice trigger (the opponent loses 1
// life) resolves before the spell is cast, not above it on the stack.
func TestFaceCastWitnessSacrificeTriggerResolvesFirstLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "face_cast_witness_sac_trigger", "std-hob", Config{GorgeAutopaySurface: true})
}

// Trystan's "you may pay {B}" with a Springleaf Drum as the only black the board could make:
// gorge's potential mana counts a tap-another-permanent source whose creature is there to
// tap, so the pay is offered beside the decline.
func TestTriggerCostCountsATapAnotherSourceLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "trigger_cost_tap_another_source", "std-ecl", Config{})
}

// Trystan's "you may pay {B}" with a Bloom Tender as the only source that could make black:
// gorge's potential mana reads its each-colour production as open, so the pay is offered.
func TestTriggerCostCountsAnEachColourSourceLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "trigger_cost_each_colour_source", "std-ecl", Config{GorgeAutopaySurface: true})
}

// An Optional$ hand-to-battlefield move that enters attacking asks whether to do it before
// it asks which player or planeswalker the creature attacks (CR 508.4, as it enters):
// Kinscaer Sentry's attack trigger against a player who controls Ajani, Outland Chaperone.
func TestEnteringAttackingAsksTheMayFirstLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "enter_attacking_declined_asks_no_defender", "std-ecl", Config{GorgeAutopaySurface: true})
}

// A mass move onto the battlefield asks each as-enters choice with that card and the later
// ones still in their zones, under the controller it will have: Rise of the Dark Realms and
// Diamond Mare, Lumra and Uncharted Haven.
func TestMassMoveAsksAsEntersInTheZoneLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "rise_diamond_mare_asks", "std-fdn", autopay)
	synthReproLockstep(t, "lumra_haven_asks", "std-blb", autopay)
}

// Zurgo's Vanguard is */3, and Fresh Start takes its power's CDA away: it stays a -5/3
// on the battlefield, whose toughness the lost ability did not set.
func TestLostCDALeavesThePrintedToughnessLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "lost_cda_toughness", "std-tdm", Config{GorgeAutopaySurface: true})
}

// A waterbend ability priced by its credit under a granted mana source: Geyser Leaper's
// credit covers the {4}, and Flexible Waterbender's {3} needs one tap of Great Divide
// Guide's granted mana, the prefix gorge's search finds.
func TestWaterbendCreditInTheGrantedPrefixSearchLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "waterbend_credit_covers", "std-tla", autopay)
	synthReproLockstep(t, "waterbend_granted_prefix", "std-tla", autopay)
}

// Ygra grants Maha its Food ability through an override static. With {2} floating after
// an {X} cast, both engines offer it.
func TestOverrideGrantedAbilityFromThePoolLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "ygra_food_from_pool", "std-blb", Config{})
}

// Flexible Waterbender carrying Trusty Boomerang: both engines price its printed
// waterbend beside the Boomerang's granted ability, whose Ability it shares in gorge.
func TestPrintedAbilityBesideAGrantLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "printed_beside_equipment_grant", "std-tla", Config{GorgeAutopaySurface: true})
}

// On the autopay surface a Gift spell is an ordinary plain cast: the promise costs no
// mana and is asked after the planner's witness is tapped.
func TestGiftCastIsPlannedOnAutopayLockstep(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-blb", Config{GorgeAutopaySurface: true}, anyOf(onStack("Longstalk Brawl"), onStack("Scrapshooter")))
}

// A land animated the turn it entered is no creature that entered: Mishra's Factory
// leaves Bristlebane Outrider's "another creature entered" unmet (CR 603.10a's reasoning,
// read at the entry), and a Grizzly Bears meets it.
func TestEnteredAsCreatureLockstep(t *testing.T) {
	t.Parallel()
	deck := []cardset.Entry{{Name: "Bristlebane Outrider", Count: 10}, {Name: "Mishra's Factory", Count: 14},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 12}}
	lockstepSet(t, "enteredcreature", [2][]cardset.Entry{deck, deck}, 64, "KENTEREDCREATURE_SEEDS")
}

// exchangeControlDecks is the fixture pool's deck: Shifting Grift's two targets in one
// slot, Trade the Helm's parent-link target beside its own, and Kitsune's trigger over
// two creatures with different controllers (CR 701.12), with artifacts, creatures and a
// Mind Stone to trade.
func exchangeControlDecks() [2][]cardset.Entry {
	deck := []cardset.Entry{{Name: "Island", Count: 17}, {Name: "Ornithopter", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Wind Drake", Count: 4}, {Name: "Trade the Helm", Count: 4}, {Name: "Shifting Grift", Count: 4},
		{Name: "Kitsune, Dragon's Daughter", Count: 3}}
	return [2][]cardset.Entry{deck, deck}
}

func TestExchangeControlLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "exchangecontrol", exchangeControlDecks(), 64, "KEXCHANGECONTROL_SEEDS")
}

// With the exchange switched off the engines part ways: the pool really exchanges.
func TestExchangeControlMutation(t *testing.T) {
	dirac.DebugExchangeControlOff = true
	defer func() { dirac.DebugExchangeControlOff = false }()
	if !lockstepDiverges(t, "exchangecontrol", exchangeControlDecks(), 64) {
		t.Fatal("no divergence in 64 games with DebugExchangeControlOff")
	}
}

// Gene Pollinator's cost taps a permanent with a "becomes tapped" trigger. The prefix
// stacks the trigger and asks its target at the activation (CR 603.3), and a sorcery-speed
// play waits for it to resolve, the mana still floating (Chrome Companion's life, Hemosymbic
// Mite's pump before an Equip); an instant-speed ability goes on top of it.
func TestTapAnotherManaTriggerStacksAndResolvesLockstep(t *testing.T) {
	t.Parallel()
	autopay := Config{GorgeAutopaySurface: true}
	synthReproLockstep(t, "pollinator_tap_gain_resolves_first", "std-eoe", autopay)
	synthReproLockstep(t, "pollinator_tap_pump_resolves_before_equip", "std-eoe", autopay)
	synthReproLockstep(t, "pollinator_tap_target_trigger_below_ability", "std-eoe", autopay)
}

// Shattered Wings destroys Tractor Beam, then surveils: the creature it was stealing is its
// owner's again when the surveil is asked (gorge settles control after every event).
func TestControlEndsBeforeTheNextResolutionAskLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "tractor_beam_destroyed_mid_resolution", "std-eoe", Config{GorgeAutopaySurface: true})
}

// Xu-Ifit returns Cosmogoyf with "has no abilities": its star is 0 and its toughness 1+*
// is 1 (CR 208.2a), so it lives as a 0/1. kgen read the toughness as 0 and it died.
func TestCreatureWithoutItsDefiningAbilityKeepsTheFixedTermLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "cosmogoyf_no_abilities_is_zero_one", "std-eoe", Config{GorgeAutopaySurface: true})
}

// Vote Out exiled by Possibility Technician with a Kavu in play: a convoke spell cast through a
// permission is offered when the creatures it could tap cover its cost, as the same card in the
// hand is (CR 702.51a). The permission walk priced the printed cost alone.
func TestConvokeSpellCastThroughAPermissionIsOfferedLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "vote_out_convoke_from_exile", "std-eoe", Config{GorgeAutopaySurface: true})
}

// Divert Disaster's unless cost {2} paid by a player holding Stomping Ground, Gene Pollinator and
// Sacred Foundry: gorge's window lists Gene Pollinator among the sources; the kernel's lists it too
// and answers the permanent it taps (mana_windowtapx.go).
func TestUnlessWindowSkipsATapAnotherSourceLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "unless_window_skips_tap_another_source", "std-eoe", Config{GorgeAutopaySurface: true})
}

// The Dawning Archaic's attack trigger aimed at Flashback with no other instant or
// sorcery in the graveyard: the card leaves the graveyard for the stack, so it has no
// target of its own and is not offered (CR 601.2c); both engines play no card.
func TestFreeCastGraveyardSpellIsNoTargetOfItselfLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "archaic_flashback_no_other_target", "std-sos", Config{GorgeAutopaySurface: true})
}

// This Town Ain't Big Enough with two mana in the pool: a permanent the caster does
// not control leaves the cost at five, which the pool cannot pay, so a multi-target ask
// offers only the caster's own permanent (CR 601.2b-f), as the single-target ask does.
func TestMultiTargetAskDropsTheTargetItsCostCannotPayLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "this_town_multi_target_unaffordable", "std-otj", Config{})
}

// A Springleaf Drum tapping a Bloom Tender is the only white mana on the board: the
// tap-another search activates the each-colour source as one of its own steps, so
// both engines offer Clachan Festival and Evershrike's Gift.
func TestTapAnotherSourceBesideAnEachColourSourceLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "tapx_beside_each_colour_source", "std-ecl", Config{})
}

// Bogslither's Embrace with a Swamp and a Bloom Tender untapped and a creature to take the
// blight: its mana part is {1}{B}, which the Swamp and the each-colour source pay, so both
// engines offer the cast (CR 601.2b, 601.2f).
func TestEitherOrAdditionalCostBesideAnEachColourSourceLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "bogslither_alt_add_each_colour_source", "std-ecl", Config{})
}

// Bogslither's Embrace, its {3} part unpayable today but its window census incomplete: the
// untapped Bloom Tender is production the census prices no unit of, so an unpayable mana
// part is not reversed at the choice -- both engines go on to the target menu, and the
// window pays in the payment (CR 601.2g, 733.1).
func TestUnpayableAltAddPartAnEachColourSourceMayFundLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "alt_add_unpayable_part_an_each_colour_source_may_fund", "std-ecl", Config{})
}

// Maralen's free cast of Bogslither's Embrace is reversed for want of a target: the
// once-each-turn permission was never used, so Unforgiving Aim exiled beside it is
// still offered in both engines (CR 733.1).
func TestReversedPermissionCastKeepsTheOncePerTurnPermissionLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "maralen_reversed_cast_spends_nothing", "std-ecl", Config{})
}

// Laughing Jasper Flint under a Stop Cold: the enchanted creature loses all
// abilities in layer 6, but the layer-4 effect that makes the creatures its
// controller does not own Mercenaries began earlier and keeps applying
// (CR 613.6), so the Outlaw Stitcher it stole is still a Mercenary.
func TestSubtypeGrantOutlivesItsSourceLosingAbilitiesLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "flint_under_stop_cold_mercenary", "std-otj", Config{})
}

// We Say Thee Nay!'s unless cost {2} paid by a player holding Hydraulic Helper, Surveillance Room and
// plain lands: the Helper's {U} may pay a payment that is no spell, so it counts toward the cost and
// the window stops with it, as gorge's does; the kernel kept tapping, over-paying with the Room (CR 106.6).
func TestUnlessWindowCountsRestrictedManaItMaySpendLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "unless_window_counts_restricted_mana", "std-msh", Config{})
}

// Seedpod Squire cast with Helga's six restricted {W} floating and Lilypad Village untapped: the
// floating mana pays the generic part, so the blue owed pip makes the window offer the Village's
// creature-only {U} before its {C}, as gorge's ask does (CR 106.6, 601.2g).
func TestFloatingRestrictedManaCountsInTheCastWindowClampLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "restricted_float_pays_hybrid_beside_village", "std-blb", Config{})
}

// Wilt in the Heat cast through Flashback: the card priced in the graveyard counts as having
// left it, so the {2} reduction is on its offer (CR 601.2a, 601.2f).
func TestSpellPricedInGraveyardCountsItsOwnDepartureLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "flashback_priced_in_graveyard_counts_its_own_departure", "std-sos", Config{})
}

// Kang, Temporal Tyrant sacrificed with its connive trigger on the stack and returned by
// Thunderbolts Conspiracy: the new object takes no counter (CR 701.50b, 400.7).
func TestConniveSourceReturnedGetsNoCounterLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "connive_source_returned_gets_no_counter", "std-msh", Config{})
}

// Black Widow, Double Agent attacks alone and is killed by Helicarrier Strike with her trigger on the
// stack; Thunderbolts Conspiracy returns her before it resolves. The returned Black Widow is a new
// object (CR 400.7), so the trigger's first strike and menace go to no one.
func TestAttackTriggerPumpMissesAnAttackerThatReturnedLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "attack_trigger_pump_misses_returned_attacker", "std-msh", Config{})
}

// Tarnation Vista beside a Lotus Ring on a creature token: both are census gaps of
// gorge's (the Vista's each-colour ability, the Ring's granted sacrifice-only mana),
// and its one prefix search activates the gaps in battlefield order, so the Vista,
// entered first, taps for its chosen colour before the Ring's bearer is sacrificed,
// though the Ring alone pays the Equip. The kernel's grant search lists the Vista as
// a candidate too (CR 601.2g, 605.3a).
func TestEachColourGapBeforeAGrantedSacrificeSourceLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "each_colour_gap_before_granted_sac_source", "std-big", Config{})
}

// Nexus of Becoming paid by a plan holding a Treasure beside Tarnation Vista, with Rest
// in Peace out: gorge activates the plan's sacrificing sources last, so when the
// Treasure's move waits on a replacement choice every other source, the Vista among
// them, is already tapped. The kernel taps in that order too (CR 601.2g, 616.1).
func TestPlanTapsItsSacrificingSourcesLastLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "plan_taps_sacrifice_last", "std-big", Config{})
}

// Lotus Ring's granted "Sacrifice this creature: Add three mana" on a tapped Ancient
// Cornucopia: the sacrifice needs no {T}, so gorge's census still counts the bearer
// and offers both Rings' Equip {3}. The kernel's availability skipped a tapped source
// unless its own view was the sacrifice-only grant (CR 602.2, 605.1a).
func TestGrantedSacrificeOnATappedBearerLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "granted_sacrifice_on_tapped_bearer", "std-big", Config{})
}

// Restless Reef's {2}{U}{B} paid beside Sunken Citadel, whose two-mana land-only
// alternative sends the kernel through its census-only trial. The trial planned a
// copy of a planner that already held the hand demand, so the demand's digit base
// was 1 and key 6 (hand reserve) ranked nothing: the kernel tapped a Swamp for
// generic where gorge taps the second Forest and keeps the black the hand needs.
func TestChosenPairTrialKeepsTheHandReserveLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "chosen_pair_trial_hand_reserve", "std-lci", Config{})
}

// Collector's Cage's {1}, {T} counter ability while the only creature its controller has
// carries Lotus Ring's granted "Sacrifice this creature: Add three mana": the sacrifice
// floats the mana but leaves the ability no target, so gorge's prefix search goes on
// through a pass, and the dies trigger's token copy is the target. The search stopped at
// the priced node, and the autopay surface dropped a play the kernel offers (CR 601.2c,
// 602.2b).
func TestPrefixSearchContinuesPastALostTargetLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "prefix_continues_past_a_lost_target", "std-big", Config{GorgeAutopaySurface: true})
}

// A Command Tower with no commander makes nothing (CR 903.4f), so gorge's census is
// complete without it: Skycloud Expanse's {1} is funded by an Island and the cast is
// offered, where the Tower used to leave the census incomplete and unfunded.
func TestEmptyIdentitySourceIsNoCensusGapLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "empty_identity_tower_beside_filter", "std-trc", Config{})
}

// Queen's Diamond with no charge counters makes nothing, so it is no census gap either:
// the prefix funds Skycloud Expanse and never taps the Diamond for no mana.
func TestZeroAmountSourceIsNoCensusGapLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "zero_amount_source_beside_filter", "std-trc", Config{})
}

// Worlds Within Worlds puts two Spider-Men and Black Panther onto the battlefield from
// the hand at once: Black Panther sees each Hero entering beside it (CR 603.6a), so it
// triggers twice whatever order the cards were picked in.
func TestHandPickEntersTogetherLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "hand_pick_enters_together", "std-msh", Config{GorgeAutopaySurface: true})
}

// Ambulatory Reactor grants artifact creatures a {C} spendable only on artifact spells and
// artifact sources' abilities. Floated in the granted-mana prefix search for an artifact
// play, that {C} cannot pay Skycloud Expanse's {1}: the land's activation is no artifact
// source's (CR 106.6), so neither engine funds the filter with it.
func TestRestrictedGrantPaysNoLandFeeLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "restricted_grant_pays_no_land_fee", "std-trc", Config{})
}

// Toxic Deluge's additional cost is an announced X life, so X = 0 always pays and the
// cast costs exactly {2}{B}. With Skycloud Expanse's filter a source the planner's census
// misses, gorge's prefix search prices the cast as it does any other, and both engines
// offer it.
func TestAnnouncedLifeCastBehindAFilterLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "announced_life_cast_behind_filter", "std-trc", Config{})
}

// A delayed return brings two exiled lands back, the first a land that asks to
// reveal a card as it enters. The election is asked inside its own move, so the
// second land is still in exile while the first one's controller answers.
func TestRememberedReturnAsksBetweenMovesLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "remembered_return_asks_between_moves", "std-trc", Config{})
}

// Two permanents that each make another permanent's exhaust abilities cost {2}
// less lower an {X}{2}{R} activation by four: the two past its generic part pay
// into {X} (CR 601.2f), so the X ask reaches two more than the mana on hand.
func TestAbilityReductionExcessPaysXLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "ability_reduction_excess_pays_x", "std-dft", Config{GorgeAutopaySurface: true})
}

// A creature that took away another's abilities for as long as it stays on the
// battlefield is blinked: the effect ends as it leaves (CR 611.2b), and the
// object that returns is a new one (CR 400.7), so the other creature's
// activated ability is offered again.
func TestBlinkedSourceEndsAbilityLossLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "blinked_source_ends_ability_loss", "std-msh", Config{GorgeAutopaySurface: true})
}

// TestFreeCastAltAddNoPartPayableLockstep: under Omniscience, Eaten Alive with
// no creature to sacrifice and no mana for its {3}{B} part is not offered; the
// free alternative cost replaces the mana cost only (CR 118.9d, 601.2b).
func TestFreeCastAltAddNoPartPayableLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "free_cast_alt_add_no_part_payable", "std-fdn", Config{GorgeAutopaySurface: true})
}

// TestFilterBesideCounterSourceShortLockstep: Three Tree Mascot's {1} filter
// pays Infernal Vessel's {B} beside Ramos, Dragon Engine with one +1/+1
// counter; a counter-cost source that cannot remove its counters is no
// census gap, so the board is still a filter-only one (CR 605.3a, 602.2b).
func TestFilterBesideCounterSourceShortLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "filter_beside_counter_source_short_of_counters", "std-fdn", Config{})
}

// TestGraveyardActivationRestrictedSourceRelaxedLockstep: Merchant of Many
// Hats' {2}{B} from the graveyard beside a granted {C} (Honest Work) is
// searched, not proven insufficient: Jasmine Dragon Tea Shop's Ally-only
// any colour pays an Ally source's ability (CR 106.6, 605.3a).
func TestGraveyardActivationRestrictedSourceRelaxedLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "graveyard_activation_restricted_source_relaxed", "std-tla", Config{})
}

// TestTokenCopyOfBackFaceMVLockstep: Ember Island Production's token copy
// of Avatar Kuruk (The Legend of Kuruk transformed) has mana value 0, so
// Epic Downfall's "mana value 3 or greater" cannot target it, while the
// transformed Kuruk itself reads its front face's {2}{U}{U} (CR 712.8e).
func TestTokenCopyOfBackFaceMVLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "token_copy_of_back_face_has_no_mana_value", "std-tla", Config{GorgeAutopaySurface: true})
}

// TestMayPlayFoldCounterRaiseAmbiguousLockstep: Quilled Greatwurm in the
// graveyard beside Muldrotha has two may-play offers; the folded one (its
// own permission) raises the cost by removing six counters, which gorge's
// payment builder does not plan, so the autopay surface offers neither.
func TestMayPlayFoldCounterRaiseAmbiguousLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "mayplay_fold_with_counter_raise_ambiguous", "std-fdn", Config{GorgeAutopaySurface: true})
}

// TestFlashbackGrantedInstanceNotPrintedOfferLockstep: Fire Nation Attacks
// prints flashback {8}{R} and Iroh grants another at its mana cost. gorge's
// prefix search for the printed instance must not stop when the cheaper
// granted instance becomes castable: that is a different option (CR 702.34a).
func TestFlashbackGrantedInstanceNotPrintedOfferLockstep(t *testing.T) {
	t.Parallel()
	synthReproLockstep(t, "flashback_granted_instance_not_printed_offer", "std-tla", Config{})
}
