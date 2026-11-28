package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Two DIFFERENT "Sacrifice another permanent: Add" sources over one creature
// (CR 601.2g-h): only the any-colour altar can pay the {U}, so a plan that
// spends both -- fewer sources than any other -- must not be the answer, and
// failing to fund it must not cost the planner the one-altar plan. Board order
// decides which step is assigned the creature first, so both orders are run.

const (
	mixSacSpellSrc     = "Name:MixSac Gyre\nManaCost:2 U\nTypes:Instant\nA:SP$ Draw | NumCards$ 1 | SpellDescription$ x\nOracle:x\n"
	mixSacPhyrexianSrc = "Name:MixSac Prism Altar\nTypes:Artifact\nA:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ Any | SpellDescription$ x\nOracle:x\n"
	mixSacAshnodSrc    = "Name:MixSac Bone Altar\nTypes:Artifact\nA:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ C | Amount$ 2 | SpellDescription$ x\nOracle:x\n"
	mixSacLandSrc      = "Name:MixSac Peak\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R | SpellDescription$ x\nOracle:x\n"
)

// mixSacBoard places the named altars in order, one creature and two red
// lands on seat 0's battlefield; the spell stays in hand.
func mixSacBoard(t *testing.T, seed uint64, altars ...string) (*Engine, state.ObjID, state.ObjID, []state.ObjID) {
	t.Helper()
	e, _, spell := newFixtureDeck(t, seed, mixSacSpellSrc)
	var ids []state.ObjID
	for _, src := range altars {
		ids = append(ids, onBoard(t, e, 0, src))
	}
	prey := onBoardReady(t, e, 0, twoSacPreySrc)
	for i := 0; i < 2; i++ {
		onBoardReady(t, e, 0, mixSacLandSrc)
	}
	// Preconditions: exactly one battlefield creature, and it is the prey.
	var creatures []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.IsCreature(id) {
			creatures = append(creatures, id)
		}
	}
	if len(creatures) != 1 || creatures[0] != prey {
		t.Fatalf("precondition: creatures %v, want only the prey %d", creatures, prey)
	}
	return e, spell, prey, ids
}

func TestPaymentPlanMixedSacSourcesOneVictim(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		altars []string
		prism  int // index of the only altar that can pay {U}
	}{
		{"any-colour altar alone", []string{mixSacPhyrexianSrc}, 0},
		{"any-colour altar first", []string{mixSacPhyrexianSrc, mixSacAshnodSrc}, 0},
		{"colourless altar first", []string{mixSacAshnodSrc, mixSacPhyrexianSrc}, 1},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, spell, prey, ids := mixSacBoard(t, 4751+uint64(i), tc.altars...)
			got := e.PlanCastPayment(0, paymentCast(spell))
			if got.Plan == nil || got.Reason != "" {
				t.Fatalf("plan = reason %q detail %q, want a plan through the any-colour altar", got.Reason, got.Detail)
			}
			victims := twoSacVictims(got.Plan)
			if len(victims) != 1 || victims[0] != prey {
				t.Fatalf("victims = %v, want the prey %d named exactly once", victims, prey)
			}
			used := false
			for _, a := range got.Plan.Activations {
				if a.Consequence != nil && a.Consequence.SacrificeOthers {
					used = a.Source == ids[tc.prism]
				}
			}
			if !used {
				t.Fatalf("plan %+v does not sacrifice through the any-colour altar %d", got.Plan.Activations, ids[tc.prism])
			}
		})
	}
}
