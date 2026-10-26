package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestChangeZoneAllZeroPlayerTargetsMovesNothing pins CR 601.2c's "any
// number" on a ChangeZoneAll player-target mode: Thraben Charm's "Exile any
// number of target players' graveyards" (TargetMin$ 0) cast with zero
// targets must exile nothing. Before the fix, the empty chosen set fell into
// changeZoneAllPlayers' unresolved-scope fallback and swept every player's
// graveyard. The second half pins that a real target still scopes the sweep
// to exactly that player.
func TestChangeZoneAllZeroPlayerTargetsMovesNothing(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	charm := corpusCard(t, "Thraben Charm")
	mountain := corpusCard(t, "Mountain")

	setup := func(t *testing.T) (*Engine, Config, state.ObjID, state.ObjID, state.ObjID) {
		t.Helper()
		e, cfg := corpusEngineCfg(t, reg, []*cards.Card{charm}, nil)
		id := moveSeededCard(t, e, 0, charm, state.ZHand)
		mine := moveSeededCard(t, e, 0, mountain, state.ZGraveyard)
		theirs := moveSeededCard(t, e, 1, mountain, state.ZGraveyard)
		if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 1 {
			t.Fatalf("precondition: seat 0 graveyard = %d cards, want 1", got)
		}
		if got := len(e.G.Zone(state.ZGraveyard, 1)); got != 1 {
			t.Fatalf("precondition: seat 1 graveyard = %d cards, want 1", got)
		}
		addMana(t, e, 0, "WW")
		return e, cfg, id, mine, theirs
	}

	castExileMode := func(t *testing.T, e *Engine, id state.ObjID) *decision.Decision {
		t.Helper()
		d := castFixture(t, e, id, -1)
		if d == nil || d.Kind != decision.KModes {
			t.Fatalf("after casting Thraben Charm, pending = %+v, want the mode ask", d)
		}
		mode := modeOptionContaining(t, d, "Exile any number of target players")
		submitChoices(t, e, mode)
		d = e.Pending()
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("after choosing the exile-graveyards mode, pending = %+v, want the target ask", d)
		}
		return d
	}

	t.Run("zero targets", func(t *testing.T) {
		e, cfg, id, mine, theirs := setup(t)
		d := castExileMode(t, e, id)
		if d.Min != 0 {
			t.Fatalf("precondition: exile mode target minimum = %d, want 0", d.Min)
		}
		submitChoices(t, e) // "any number" includes zero: no targets
		passUntilStackEmpty(t, e, 20)

		if o := e.G.Obj(mine); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("seat 0's graveyard card moved on a zero-target cast: %+v", o)
		}
		if o := e.G.Obj(theirs); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("seat 1's graveyard card moved on a zero-target cast: %+v", o)
		}
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("precondition: resolved Thraben Charm zone = %+v, want graveyard", o)
		}
		if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 2 {
			// `mine` plus the resolved Charm itself.
			t.Fatalf("seat 0 graveyard = %d cards after a zero-target cast, want 2 (mine + resolved Charm)", got)
		}
		if got := len(e.G.Zone(state.ZGraveyard, 1)); got != 1 {
			t.Fatalf("seat 1 graveyard = %d cards after a zero-target cast, want 1", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("seat 1 targeted", func(t *testing.T) {
		e, cfg, id, mine, theirs := setup(t)
		d := castExileMode(t, e, id)
		idx := indexOfPlayerOption(d, 1)
		if idx < 0 {
			t.Fatalf("exile mode did not offer seat 1: %+v", d.Options)
		}
		submitChoices(t, e, idx)
		passUntilStackEmpty(t, e, 20)

		if o := e.G.Obj(theirs); o == nil || o.Zone != state.ZExile {
			t.Fatalf("targeted seat 1's graveyard card not exiled: %+v", o)
		}
		if got := len(e.G.Zone(state.ZGraveyard, 1)); got != 0 {
			t.Fatalf("seat 1 graveyard = %d cards after being targeted, want 0", got)
		}
		if o := e.G.Obj(mine); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("seat 0's graveyard card exiled by a seat-1-targeted cast: %+v", o)
		}
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("precondition: resolved Thraben Charm zone = %+v, want graveyard", o)
		}
		if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 2 {
			// `mine` plus the resolved Charm itself.
			t.Fatalf("seat 0 graveyard = %d cards after seat 1 was targeted, want 2 (mine + resolved Charm)", got)
		}
		replayCheck(t, e, cfg)
	})
}
