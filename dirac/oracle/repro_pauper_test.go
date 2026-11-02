package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// poolReproLockstep replays a recorded game on a deck pool's table, past its
// script, and fails on any divergence. An unverified card plays: the fixture names
// the game that found a difference.
func poolReproLockstep(t *testing.T, file, pool string, cfg Config) {
	t.Helper()
	poolReproPrefixLockstep(t, file, pool, cfg, 200)
}

// poolReproPrefixLockstep is poolReproLockstep stopping tail decisions past the
// script, for a fixture whose game goes on to a difference of another cause.
func poolReproPrefixLockstep(t *testing.T, file, pool string, cfg Config, tail int) {
	t.Helper()
	res, script := poolReproRun(t, file, pool, cfg, tail)
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n gorge %v\n dirac %v\n %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions, res.Div.Diff)
	}
}

// poolReproRun plays the fixture's script and tail decisions beyond it.
func poolReproRun(t *testing.T, file, pool string, cfg Config, tail int) (Result, *ScriptPolicy) {
	t.Helper()
	r, err := ReadRepro("testdata/" + file + ".repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	set, err := cardset.RawByName(pool)
	if err != nil {
		t.Skipf("no %s table (make dirac-tables): %v", pool, err)
	}
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
	rc := RunConfig{Config: cfg, MaxDecisions: len(r.Actions) + tail}
	script := &ScriptPolicy{Actions: r.Actions}
	return Lockstep(m, r.Seed, k, script, rc), script
}

// Heap Gate's "{1}, {T}, Tap an untapped Gate": gorge arms the mana plan before the
// Gate is elected, and keeps it unless the election takes one of its sources, so the
// {1} is paid by the source the plan chose with every Gate still untapped.
func TestHeapGatePlanSurvivesTheElectionLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "heap_gate_plan_survives_election", "pauper", Config{})
}

// A dry plan for an activation, on a table that holds a sacrifice-only mana source
// (Makeshift Munitions' pool), found its plan: the retry for a census that proved
// insufficient has nothing to add, and its empty plan did not replace the real one.
func TestDryPlanBesideASacOnlySourceLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "heap_gate_dry_plan_with_sac_only_source", "pauper", Config{})
}

// Land Grant beside Saruli Caretakers: the plain cast {1}{G} is payable through a
// scripted prefix of two activations. gorge prices it on its own, the free
// alternative cost being a separate play, so both are offered.
func TestLandGrantPlainCastPricedBesideItsAltCostLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "land_grant_plain_cast_priced", "pauper", Config{})
}

// A tapped Eldrazi Spawn still sacrifices for {C} (its cost has no {T}), so both sides
// price Makeshift Munitions' {1}{R} from an untapped Bridge and the Spawn, rather than
// reaching the Spawn only through a prefix that leaves its sacrifice trigger on the stack.
func TestTappedSpawnPaysTheCastLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "tapped_spawn_pays_the_cast", "pauper", Config{})
}

// Nyxborn Hydra's {X}{G} beside two Writhing Chrysalis and three Eldrazi Spawn: the
// largest-X witness sacrifices all three, each sacrifice stacks a trigger per
// Chrysalis, and six triggers are more than gorge's witness execution waits out before
// the cast is offered again. gorge plans the largest X that keeps its sources, so the
// Spawn are still there, with the Chrysalis uncountered, when X is announced.
func TestXWitnessPastTheTriggerWaitBudgetKeepsItsSourcesLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "xcast_spawn_witness_over_wait_budget", "pauper", Config{})
}

// A bestowed Nyxborn Hydra paid with Eldrazi Spawn beside a Writhing Chrysalis: the
// oracle lowers the cast by activating its sources at priority, and the Chrysalis'
// counter triggers resolve before the cast begins, as they do for a plain cast.
func TestBestowLoweringResolvesTheSacrificeTriggersLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "bestow_lowering_waits_out_spawn_triggers", "pauper", Config{GorgeAutopaySurface: true})
}

// A bestowed Nyxborn Hydra whose largest-X witness would sacrifice the Eldrazi Spawn
// that is its only creature to enchant: gorge plans the largest X that keeps its
// sources, so the Spawn is still on the battlefield when X is announced.
func TestBestowWitnessThatWouldEatItsEnchantTargetKeepsItLockstep(t *testing.T) {
	t.Parallel()
	poolReproPrefixLockstep(t, "bestow_witness_keeps_its_enchant_target", "pauper", Config{GorgeAutopaySurface: true}, 2)
}

// Lotleth Giant paid with two Wall of Roots, an Overgrown Battlement and a Saruli
// Caretaker: the scripted Wall activations each return to priority, and a Wall whose
// toughness reached 0 dies before the Battlement counts the creatures with defender.
func TestWallOfRootsDiesBetweenScriptStepsLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "wall_of_roots_dies_between_script_steps", "pauper", Config{})
}

// Lotleth Giant paid with Tinder Wall and an Overgrown Battlement: the plan prices the
// Battlement with the Wall beside it, so the Wall's sacrifice runs after the tap.
func TestPlanSacrificeRunsAfterABoardReaderLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "plan_sacrifice_after_board_reader", "pauper", Config{GorgeAutopaySurface: true})
}

// Sagu Wildling beside a Wall of Roots at one toughness, an Overgrown Battlement, a
// Swamp and a Forest: the Wall's counter kills it before the Battlement counts the
// creatures with defender, so no chain of activations pays {4}{G} and gorge's prefix
// search offers no plain cast.
func TestWallOfRootsDeathLeavesTheCastUnofferedLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "wall_of_roots_dies_before_a_reader_prices_the_cast", "pauper", Config{})
}

// A Nyxborn Hydra in hand beside a Guardian of the Guildpact, the only creature:
// the Hydra is monocolored, so the Guardian is no creature it may enchant and the
// bestowed cast is not offered.
func TestBestowTargetProtectedFromMonocoloredLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "bestow_target_protected_from_monocolored", "pauper", Config{GorgeAutopaySurface: true})
}

// Writhing Chrysalis paid with two Bridges and two tapped Eldrazi Spawn: the planner
// prices a tapped Spawn (its sacrifice has no {T}), and the plan's step for it stays
// ready, so the cast pays by its plan and no window asks for colours.
func TestTappedSpawnPlanStepStaysReadyLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "tapped_spawn_plan_step_stays_ready", "pauper", Config{})
	poolReproLockstep(t, "tapped_spawn_plan_step_stays_ready_autopay", "pauper", Config{GorgeAutopaySurface: true})
}

// A Nyxborn Hydra beside two Priests of Titania and a board of Elves: the sources make
// 23 mana, gorge's largest-X witness pays X = 22, and the kernel's X search no longer
// stops at 20.
func TestXCastPastTwentyManaLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "x_cast_past_twenty_mana", "pauper", Config{})
}

// Force Spike's unless {1} paid by a player holding two Saruli Caretakers, two Walls of Roots and an
// Overgrown Battlement: gorge's CR 601.2g window lists the first Caretaker first, asks which creature
// to tap (seven candidates) and takes the lowest colour. The kernel's window offered only the sources
// that tap for mana alone and tapped the Battlement.
func TestUnlessWindowTapsAnotherSourceLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "unless_window_taps_another_source_pauper", "pauper", Config{})
}

// TestUnlessWindowTapsAnotherSourceMutation: with the tap-another gaps hidden (the window does not
// list the Caretaker) the same game diverges.
func TestUnlessWindowTapsAnotherSourceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoTapMana = true
	res, _ := poolReproRun(t, "unless_window_taps_another_source_pauper", "pauper", Config{}, 200)
	dirac.DebugNoTapMana = false
	if res.Div == nil {
		t.Error("no divergence without the window's tap-another source: the fixture never needs it")
	}
}

// Chain Lightning on a Refurbished Familiar whose controller holds a Silverbluff Bridge ({T}: Add {U} or {R})
// and a Great Furnace: the {R}{R} copy cost opens gorge's unless mana window, whose decisions carry no owed
// cost, so the Bridge's colour ask is answered with its lowest colour, {U}. The payment then falls short, the
// taps stay and no copy is made. The kernel's window gave the Bridge the colour the cost still owed.
func TestUnlessWindowTwoColourSourceTakesLowestColourLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "unless_window_two_colour_source_takes_lowest_colour", "pauper", Config{GorgeAutopaySurface: true})
}

// Chain Lightning's {R}{R} copy cost paid by a player holding two Walls of Roots and a Tinder Wall: gorge's
// unless mana window lists a counter-cost source like any other and its answer takes the first listed, so
// each Wall gets a -0/-1 counter and floats {G} before the Tinder Wall is sacrificed for {R}{R}. The kernel's
// window walked past the Walls.
func TestUnlessWindowActivatesCounterCostSourceLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "unless_window_activates_counter_cost_source", "pauper", Config{GorgeAutopaySurface: true})
}

// TestUnlessWindowActivatesCounterCostSourceMutation: with the counter-cost gaps hidden the same game diverges.
func TestUnlessWindowActivatesCounterCostSourceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoCtrMana = true
	res, _ := poolReproRun(t, "unless_window_activates_counter_cost_source", "pauper", Config{GorgeAutopaySurface: true}, 200)
	dirac.DebugNoCtrMana = false
	if res.Div == nil {
		t.Error("no divergence without the counter-cost gaps: the fixture never needs them")
	}
}

// Eviscerator's Insight in the graveyard, flashback {4}{B} with a sacrifice beside it, and a player holding
// three untapped lands and two Eldrazi Spawn: the Spawn can pay the mana or be the sacrifice, not both, so the
// five mana is out of reach and gorge lists no cast. The kernel counted each Spawn twice.
func TestFlashbackSacCostAndSacManaShareTheSourcesLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "flashback_sac_cost_and_sac_mana_share_the_sources", "pauper", Config{})
}

// Lembas sacrificed for its own ability, then Nihil Spellbomb exiling the graveyard in response to its
// "shuffles it into their library" trigger: the card is no longer in the graveyard, so it is not shuffled
// in and the library is not shuffled (the trigger's Origin$ Graveyard).
func TestLembasExiledInResponseIsNotShuffledInLockstep(t *testing.T) {
	t.Parallel()
	poolReproLockstep(t, "lembas_exiled_in_response_is_not_shuffled_in", "pauper", Config{})
}
