package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestSurgeEngineAnimateUnblockable pins Animate's staticAbilities$ grant of a
// Mode$ CantBlockBy body (ValidAttacker$ Creature.Self): {U} makes Surge
// Engine lose defender AND become unblockable for the animation's lifetime.
func TestSurgeEngineAnimateUnblockable(t *testing.T) {
	t.Parallel()
	card := mshCorpusCard(t, "Surge Engine")
	e := chargeEngine(t, 8201, card)
	bear := onBoardReady(t, e, 1, bearBlockSrc)
	e.Advance()
	eng := moveSeededCard(t, e, 0, card, state.ZBattlefield)

	if !e.HasKeyword(eng, "Defender") {
		t.Fatal("precondition: Surge Engine lacks defender before activation")
	}
	if e.blockRestricted(bear, eng) {
		t.Fatal("precondition: bear cannot block the Surge Engine before activation")
	}

	addMana(t, e, 0, "U")
	submitChoices(t, e, abilityOption(t, e, eng, 0).Index)
	passUntilStackEmpty(t, e, 20)

	if e.HasKeyword(eng, "Defender") {
		t.Fatal("Surge Engine kept defender after the Animate")
	}
	if !e.blockRestricted(bear, eng) {
		t.Fatal("bear can still block the Surge Engine after it gained 'can't be blocked'")
	}
}
