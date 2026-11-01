package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// castFracturedRealm resolves Mirror Room // Fractured Realm cast as its doubling
// door, leaving that door live for seat 0.
func castFracturedRealm(t *testing.T, e *Engine) {
	t.Helper()
	moveByName(t, e, 0, "Mirror Room", state.ZHand)
	addMana(t, e, 0, "UUUUUUU")
	castNamed(t, e, "Fractured Realm")
	passUntilStackEmpty(t, e, 30)
	answerQuiet(t, e, 60)
}

// CR 603.2d: a door's "when you unlock this door" ability triggers an additional
// time under a live Panharmonicon static, whether a paid unlock or a cast Room's
// entry unlocked it.
func TestUnlockTriggerDoubledByFracturedRealm(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)

	t.Run("unlock", func(t *testing.T) {
		e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Mirror Room"), lookup(t, reg, "Spiked Corridor")}, nil)
		castFracturedRealm(t, e)
		room := moveByName(t, e, 0, "Spiked Corridor", state.ZBattlefield)
		e.emit(events.Event{Kind: events.DoorUnlock, Obj: room, Amount: 1})
		passUntilStackEmpty(t, e, 30)
		answerQuiet(t, e, 60)
		if got := countDevils(e); got != 6 {
			t.Fatalf("doubled unlock trigger made %d Devils, want 6", got)
		}
	})
	t.Run("cast entry", func(t *testing.T) {
		e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Mirror Room"), lookup(t, reg, "Spiked Corridor")}, nil)
		castFracturedRealm(t, e)
		moveByName(t, e, 0, "Spiked Corridor", state.ZHand)
		addMana(t, e, 0, "RRRR")
		castNamed(t, e, "Spiked Corridor")
		passUntilStackEmpty(t, e, 30)
		answerQuiet(t, e, 60)
		if got := countDevils(e); got != 6 {
			t.Fatalf("doubled entry trigger made %d Devils, want 6", got)
		}
	})
}

// A static scoped to entering permanents does not reach a door's trigger: the
// unlock causes it, even when the unlock happens as the Room enters.
func TestEntryScopedDoublerLeavesUnlockTriggerAlone(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Yarok, the Desecrated"), lookup(t, reg, "Spiked Corridor")}, nil)
	moveByName(t, e, 0, "Yarok, the Desecrated", state.ZBattlefield)
	moveByName(t, e, 0, "Spiked Corridor", state.ZHand)
	addMana(t, e, 0, "RRRR")
	castNamed(t, e, "Spiked Corridor")
	passUntilStackEmpty(t, e, 30)
	answerQuiet(t, e, 60)
	if got := countDevils(e); got != 3 {
		t.Fatalf("entry-scoped doubler left %d Devils, want 3", got)
	}
}
