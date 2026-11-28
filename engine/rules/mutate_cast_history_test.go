package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestMutateTopMergeKeepsTheCastSpellInCastHistory pins that this turn's cast
// history still describes the spell that was cast after it merges on top of a
// creature. The history readers resolve the PutOnStack event's object ID to
// the live object, so a merge that hands that object to another card (the
// demoted one) rewrites the history: the caster would be counted as having
// cast the creature they mutated onto.
func TestMutateTopMergeKeepsTheCastSpellInCastHistory(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	// Red Phoenix with mana value 4, cast onto a green Bear with mana value 2:
	// every spec below tells the two cards apart.
	phoenix := mustCorpusCard(t, reg, "Everquill Phoenix")
	e, cfg := tokenReplGame(t, 403, phoenix, card(t, mutateBearSrc))
	phoenixID := moveSeededCard(t, e, 0, phoenix, state.ZHand)
	bear := moveSeeded(t, e, 0, mutateBearSrc, state.ZBattlefield)
	addMana(t, e, 0, "RRRR")

	want := []struct {
		spec string
		n    int
	}{
		{"Card.Red+YouCtrl", 1},
		{"Card.Green+YouCtrl", 0},
		{"Phoenix.YouCtrl", 1},
		{"Bear.YouCtrl", 0},
		{"Card.YouCtrl+cmcGE4", 1},
		{"Card.YouCtrl+cmcLE3", 0},
	}
	check := func(when string) {
		t.Helper()
		for _, w := range want {
			if got := e.SpellsCastThisTurnMatching(0, w.spec, effects.TriggerContext{}); got != w.n {
				t.Errorf("%s: SpellsCastThisTurnMatching(%q) = %d, want %d", when, w.spec, got, w.n)
			}
		}
	}

	mutateCastOntoUndrained(t, e, mutatedCastOption(t, e, phoenixID), bear, true)
	if o := e.G.Obj(phoenixID); o.Zone != state.ZStack {
		t.Fatalf("precondition: the mutating spell is not on the stack: %+v", o)
	}
	check("on the stack")

	passUntilStackEmpty(t, e, 40)
	if pile := e.G.Obj(bear); len(pile.MergedCards) != 1 || pile.Face().Name != "Everquill Phoenix" {
		t.Fatalf("precondition: the spell did not merge on top: %+v", pile)
	}
	check("merged")
	replayCheck(t, e, cfg)
}
