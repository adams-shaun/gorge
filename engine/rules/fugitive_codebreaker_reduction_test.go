// fugitive_codebreaker_reduction_test.go — the two price extremes of
// Fugitive Codebreaker's printed Disguise rider ("This cost is reduced by {1}
// for each instant and sorcery card in your graveyard"). The pinning test for
// the mid case lives in keyword_cost_param_rules_test.go.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestFugitiveCodebreakerDisguiseTurnFaceUpReductionExtremes pins the clamp on
// both sides of the rider: with an empty graveyard the full printed {5}{R} is
// still required (the rider is not a baseline discount), and with six cards it
// clamps at the generic floor — the price is {R}, the coloured pip is never
// eaten, and the reducer's overflow is discarded.
func TestFugitiveCodebreakerDisguiseTurnFaceUpReductionExtremes(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)

	t.Run("empty graveyard still pays the full printed cost", func(t *testing.T) {
		t.Parallel()
		e, cfg := manifestEngine(t, reg, "Fugitive Codebreaker")
		id := morphDownCast(t, e, "Fugitive Codebreaker", "disguised", "RRR", 0)

		mf, ok := morphFaceUpCost(e.G.Obj(id))
		if !ok || mf.reduceToken != "X" {
			t.Fatalf("precondition: morphFaceUpCost = %+v ok=%v reduceToken=%q, want the rider token", mf.cost, ok, mf.reduceToken)
		}
		mods, mok := e.morphTurnUpMods(0, id, mf)
		if !mok {
			t.Fatal("precondition: morphTurnUpMods withheld the turn-up")
		}
		// Precondition: with zero matching graveyard cards the reduction
		// evaluates to 0, so the composed price still differs from nothing.
		if got := mods.Apply(mf.cost); got.Generic != 5 || got.Colored[state.MR] != 1 || got.CMC() != 6 {
			t.Fatalf("precondition: composed turn-up cost with an empty graveyard = %+v, want the printed {5}{R}", got)
		}

		// Five mana is one short of the full {5}{R}: the offer stays withheld.
		if turnFaceUpOptionPresent(t, e, id) {
			t.Fatal("turn_face_up offered with an empty pool, want withheld")
		}
		addMana(t, e, 0, "RRRRR")
		if turnFaceUpOptionPresent(t, e, id) {
			t.Fatal("turn_face_up offered for 5 mana with an empty graveyard, want withheld (the full cost is 6)")
		}
		// Six mana pays the full printed cost, proving no reduction leaked in.
		addMana(t, e, 0, "R")
		if !turnFaceUpOptionPresent(t, e, id) {
			t.Fatal("turn_face_up not offered after funding the full {5}{R}")
		}
		mark := len(e.L.Events)
		before := e.G.Players[0].Pool.Total()
		submitChoices(t, e, turnFaceUpIndex(t, e, id))
		assertTurnUpEventOnce(t, e, id, mark)
		if got, want := e.G.Players[0].Pool.Total(), before-6; got != want {
			t.Fatalf("pool after turn-up = %d, want %d (exactly the full {5}{R})", got, want)
		}
		if e.G.Obj(id).FaceDown {
			t.Fatal("Fugitive Codebreaker FaceDown=true after the turn-up, want false")
		}
		replayCheck(t, e, cfg)
	})

	t.Run("six counted cards clamp the generic and never eat the red pip", func(t *testing.T) {
		t.Parallel()
		e, cfg := manifestEngine(t, reg, "Fugitive Codebreaker",
			"Lightning Bolt", "Lightning Bolt", "Lightning Bolt",
			"Lightning Bolt", "Lightning Bolt", "Lightning Bolt")
		id := morphDownCast(t, e, "Fugitive Codebreaker", "disguised", "RRR", 0)

		mf, ok := morphFaceUpCost(e.G.Obj(id))
		if !ok || mf.reduceToken != "X" {
			t.Fatalf("precondition: morphFaceUpCost = %+v ok=%v reduceToken=%q, want the rider token", mf.cost, ok, mf.reduceToken)
		}
		// Seed SIX instants in seat 0's graveyard: the rider is 6, one more
		// than the {5} generic, so any pip spill or negative generic would
		// show in the composed price.
		seen := map[state.ObjID]bool{}
		for i := 0; i < 6; i++ {
			g := searchMoveByName(t, e, "Lightning Bolt", state.ZGraveyard)
			if seen[g] || e.G.Obj(g).Zone != state.ZGraveyard || e.G.Obj(g).Owner != 0 {
				t.Fatalf("precondition: graveyard card %d = %d zone %s owner %d, want six distinct seat-0 Bolts in ZGraveyard",
					i, g, e.G.Obj(g).Zone, e.G.Obj(g).Owner)
			}
			seen[g] = true
		}
		mods, mok := e.morphTurnUpMods(0, id, mf)
		if !mok {
			t.Fatal("precondition: morphTurnUpMods withheld the turn-up")
		}
		// Precondition: the clamp lands on {R}, which differs from the printed
		// {5}{R} and from 0, so the offer/pool assertions below test the clamp.
		composed := mods.Apply(mf.cost)
		if composed.Generic != 0 || composed.Colored[state.MR] != 1 || composed.CMC() != 1 {
			t.Fatalf("precondition: composed turn-up cost with six graveyard cards = %+v, want the clamped {R}", composed)
		}

		// One red mana and nothing else is exactly the clamped price: the
		// offer must appear, and the pool at that point is exactly 1.
		addMana(t, e, 0, "R")
		if got := e.G.Players[0].Pool.Total(); got != 1 {
			t.Fatalf("precondition: pool before the clamped turn-up = %d, want exactly 1", got)
		}
		if !turnFaceUpOptionPresent(t, e, id) {
			t.Fatal("turn_face_up withheld with 1 red mana, want offered for the clamped {R}")
		}
		mark := len(e.L.Events)
		submitChoices(t, e, turnFaceUpIndex(t, e, id))
		assertTurnUpEventOnce(t, e, id, mark)
		if got, want := e.G.Players[0].Pool.Total(), int32(0); got != want {
			t.Fatalf("pool after the clamped turn-up = %d, want %d (the {R} pip, never the generic overflow)", got, want)
		}
		if e.G.Obj(id).FaceDown {
			t.Fatal("Fugitive Codebreaker FaceDown=true after the turn-up, want false")
		}
		replayCheck(t, e, cfg)
	})
}
