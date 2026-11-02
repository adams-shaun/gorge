package oracle

import (
	"testing"

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
	res := Lockstep(m, r.Seed, k, script, rc)
	if script.Skipped != 0 {
		t.Errorf("%d scripted actions were not offered", script.Skipped)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n gorge %v\n dirac %v\n %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions, res.Div.Diff)
	}
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
