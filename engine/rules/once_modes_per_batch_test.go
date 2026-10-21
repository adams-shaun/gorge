package rules

// The Once trigger modes are "one or more" BATCH readings (CR 603.2c): one
// sacrifice action, or one token-creation resolution, queues the line once
// however many objects it moves/creates; a later, separate action is a new
// batch and queues again. These tests pin both halves on the real corpus
// carriers -- Camellia, the Seedmiser (Mode$ SacrificedOnce) and Staff of the
// Storyteller (Mode$ TokenCreatedOnce without OnlyFirst$) -- against the
// previous once-per-turn stand-in, which collapsed every action after the
// first in the same turn.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSacrificedOncePerBatch: two SEPARATE two-Food sacrifice actions in one
// turn each create one Squirrel. The first action already proves the batch
// half (two Foods, one token); the second proves the cadence is per action,
// not per turn -- the old implicit once-per-turn limit suppressed it.
func TestSacrificedOncePerBatch(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	camellia := lookup(t, reg, "Camellia, the Seedmiser")
	food := lookup(t, reg, "Bagel and Schmear")
	feast := card(t, "Name:Feast Twice\nManaCost:0\nTypes:Sorcery\n"+
		"A:SP$ Sacrifice | SacValid$ Food | Amount$ 2\nOracle:x\n")
	e := corpusEngine(t, reg, []*cards.Card{camellia, food, food, food, food, feast, feast}, nil)
	camID := moveByName(t, e, 0, "Camellia, the Seedmiser", state.ZBattlefield)
	foods := []state.ObjID{
		moveByName(t, e, 0, "Bagel and Schmear", state.ZBattlefield),
		moveByName(t, e, 0, "Bagel and Schmear", state.ZBattlefield),
		moveByName(t, e, 0, "Bagel and Schmear", state.ZBattlefield),
		moveByName(t, e, 0, "Bagel and Schmear", state.ZBattlefield),
	}
	// moveByName finds the hand copy first, so collect the two distinct
	// objects directly and move each library copy to hand with a logged event.
	var feasts []state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Feast Twice" {
				feasts = append(feasts, id)
			}
		}
	}
	if len(feasts) < 2 {
		t.Fatalf("precondition: found %d Feast Twice copies, want 2", len(feasts))
	}
	feasts = feasts[:2]
	for _, id := range feasts {
		if o := e.G.Obj(id); o.Zone == state.ZLibrary {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
		}
	}
	if o := e.G.Obj(camID); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Camellia is not on the battlefield")
	}
	isFood := map[state.ObjID]bool{}
	for _, id := range foods {
		isFood[id] = true
	}
	seen := map[state.ObjID]bool{}
	for _, id := range foods {
		if seen[id] {
			t.Fatal("precondition: duplicate Food object")
		}
		seen[id] = true
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: Food %d is not on the battlefield", id)
		}
	}
	if feasts[0] == feasts[1] {
		t.Fatal("precondition: the two Feasts are the same object")
	}
	if n := countTokensNamed(t, e, "Squirrel Token"); n != 0 {
		t.Fatalf("precondition: %d Squirrels already on the battlefield", n)
	}

	castFeast := func(id state.ObjID) {
		t.Helper()
		e.pending = nil
		e.priorityRound()
		castIdx := -1
		for _, o := range e.Pending().Options {
			if o.Kind == "cast" && o.Obj == id {
				castIdx = o.Index
			}
		}
		if castIdx < 0 {
			t.Fatalf("precondition: Feast Twice %d is not offered as a cast: %+v", id, e.Pending().Options)
		}
		submitChoices(t, e, castIdx)
		drainSetMechanic(t, e, func(d *decision.Decision) []int {
			if d.Kind == decision.KChoose {
				var out []int
				for _, o := range d.Options {
					if isFood[o.Obj] {
						out = append(out, o.Index)
						if len(out) == 2 {
							return out
						}
					}
				}
			}
			return firstOption(d)
		})
	}

	castFeast(feasts[0])
	if n := countTokensNamed(t, e, "Squirrel Token"); n != 1 {
		t.Fatalf("after the first two-Food action Squirrels = %d, want 1 (one batch, one trigger)", n)
	}

	castFeast(feasts[1])
	if n := countTokensNamed(t, e, "Squirrel Token"); n != 2 {
		t.Fatalf("after a second, separate two-Food action in the same turn Squirrels = %d, want 2 (per action, not per turn)", n)
	}
}

// TestTokenCreatedOncePerBatch: Staff of the Storyteller's TokenCreatedOnce
// line has no OnlyFirst$ rider, so it fires once per token-creation
// resolution. Its own ETB makes the first batch (one Spirit -> one story
// counter); an authored maker then creates TWO Squirrels in one resolution
// (one more counter, not two), and a second, separate activation adds one
// more. The old implicit once-per-turn limit left the count at 1.
func TestTokenCreatedOncePerBatch(t *testing.T) {
	t.Parallel()
	staff := tokenReplCorpusCard(t, "Staff of the Storyteller")
	maker := cardByName(t, "Name:Double Forge\nTypes:Artifact\n"+
		"A:AB$ Token | Cost$ T | TokenScript$ g_1_1_squirrel | TokenOwner$ You | TokenAmount$ 2\nOracle:x\n")
	e, cfg := tokenReplGame(t, 41, staff, maker)
	staffID := moveSeededCard(t, e, 0, staff, state.ZBattlefield)
	makerID := moveSeededCard(t, e, 0, maker, state.ZBattlefield)

	// Settle Staff's ETB (its own Spirit token, one TokenCreatedOnce batch).
	addMana(t, e, 0, "")
	drainSetMechanic(t, e, firstOption)
	if got := e.G.Obj(staffID).Counter("STORY"); got != 1 {
		t.Fatalf("after Staff's ETB story counters = %d, want 1 (one batch from its own Spirit)", got)
	}

	activateTokenForge(t, e, makerID) // two Squirrels in ONE resolution
	if got := e.G.Obj(staffID).Counter("STORY"); got != 2 {
		t.Fatalf("after one two-token resolution story counters = %d, want 2 (one trigger for the batch)", got)
	}
	if got := countTokensNamedOnSeat(t, e, 0, "Squirrel Token"); got != 2 {
		t.Fatalf("precondition: Squirrel Tokens = %d, want 2", got)
	}

	untapUntaps(t, e, makerID)
	activateTokenForge(t, e, makerID) // a second, separate resolution
	if got := e.G.Obj(staffID).Counter("STORY"); got != 3 {
		t.Fatalf("after a second token-creation resolution story counters = %d, want 3 (per resolution, not per turn)", got)
	}
	replayCheck(t, e, cfg)
}
