package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// minTargetsPlanRepros are the recorded games of the min-target survivor check.
//
// CR 601.2c-h with gorge's survivorsCanTarget: a spell announcing at least Min
// targets, paid by sacrificing some of its own candidates, is offered only
// while the plan gorge's search prices leaves Min candidates standing. That is
// the planner's plan (the cheapest sacrifice by its ranking), not whatever
// plan would keep the most candidates.
//
//   - sac_mana_cant_pay_min_targets_spell: Aether Gale's six nonland
//     permanents, {3}{U}{U} paid by the Eldrazi Scions among them (tail-14).
//   - min_targets_plan_consumes_a_candidate_beside_a_spare_sacrifice: Dust to
//     Dust ("two target artifacts", {1}{W}{W}) over a Remote Farm's {W}{W}, a
//     Lotus Petal, an Ornithopter and a Blood Pet. The Petal is the cheapest
//     sacrifice for the {1} (an artifact's weighs 10, a creature's 20), which
//     leaves one artifact standing, so gorge refuses the cast. A greedy
//     hold-out that protects every candidate it can (the Petal and the
//     Ornithopter, paying with the Pet) offered it (seed 2 of the deck on the
//     full surface, decision 7).
func minTargetsPlanRepros() []string {
	return []string{
		"sac_mana_cant_pay_min_targets_spell",
		"min_targets_plan_consumes_a_candidate_beside_a_spare_sacrifice",
	}
}

func TestMBCMinTargetsPlanReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range minTargetsPlanRepros() {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}

// TestMBCMinTargetsPlanMutation: the recordings exercise the survivor count.
// With the slot's minimum read as one survivor (the plan's candidates counted
// as gorge's single-target census counts them) the kernel offers a cast gorge
// refuses, in both.
func TestMBCMinTargetsPlanMutation(t *testing.T) {
	diverges := func(name string) bool {
		r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
		if err != nil {
			t.Fatal(err)
		}
		pol := &ScriptPolicy{Actions: r.Actions}
		return replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol).Div != nil
	}
	for _, name := range minTargetsPlanRepros() {
		if diverges(name) {
			t.Fatalf("%s: diverges before the mutation", name)
		}
	}
	dirac.DebugMinTargetsOneSurvivor = true
	defer func() { dirac.DebugMinTargetsOneSurvivor = false }()
	for _, name := range minTargetsPlanRepros() {
		if !diverges(name) {
			t.Errorf("%s: no divergence with the minimum read as one survivor", name)
		}
	}
}
