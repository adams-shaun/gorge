package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGrantedReplacementCrossObjectGate pins the granting-face SVar frame of
// a granted static replacement. Scion
// of Halaster grants the commander creatures you own "The first time you would
// draw a card each turn, instead look at the top two cards ... Then draw a
// card." The granted R: body is `Event$ Draw | ValidPlayer$ You | ReplaceWith$
// DBDig | CheckSVar$ X | SVarCompare$ EQ0` with `SVar:X:Count$YouDrewThisTurn`
// defined on SCION, while the HOST the static affects is the commander
// creature. The gate must resolve X from the GRANTING face (Scion), so the
// replacement fires only on the turn's FIRST draw. Resolving X from the host's
// face (the bug this test guards) reads the host's own missing X as 0, EQ0
// holds every time, and the replacement fires on every draw.
//
// The observable is Scion's Dig: each applied replacement puts one card of the
// top two into its controller's graveyard (DestinationZone$ Graveyard), so
// after two draws the graveyard holds one card (only the first draw replaced)
// and not two (every draw replaced). A control with no Scion on the board
// proves the graveyard growth comes from the grant.
func TestGrantedReplacementCrossObjectGate(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	run := func(t *testing.T, withScion bool) (graveyard, hand int) {
		t.Helper()
		e := stealEngine(t, 820)
		// A commander creature Scion's Affected$ Creature.IsCommander+YouOwn
		// matches. It is NOT the source of the granted SVar: the X the gate
		// reads lives on Scion, not here.
		cmdr := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Isamaru, Hound of Konda"))
		e.G.Players[0].Commanders = append(e.G.Players[0].Commanders, cmdr)
		if withScion {
			scion := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Scion of Halaster"))
			if got := e.G.Obj(scion).Zone; got != state.ZBattlefield {
				t.Fatalf("precondition: Scion zone = %v, want battlefield", got)
			}
		}
		// Precondition: the commander is on the battlefield and really is one
		// of seat 0's commanders, so the grant's Affected$ spec matches it.
		if got := e.G.Obj(cmdr).Zone; got != state.ZBattlefield {
			t.Fatalf("precondition: commander zone = %v, want battlefield", got)
		}
		if !e.isCommanderObj(cmdr) {
			t.Fatal("precondition: Isamaru is not flagged a commander")
		}
		// A library deep enough for two draws, each of which may Dig two cards.
		lib := make([]state.ObjID, 0, 8)
		for i := 0; i < 8; i++ {
			o := e.G.AddObject(mustCorpusCard(t, reg, "Mountain"), 0)
			o.Zone = state.ZLibrary
			lib = append(lib, o.ID)
		}
		e.G.SetZone(state.ZLibrary, 0, lib)
		if got := len(e.G.Zone(state.ZLibrary, 0)); got != 8 {
			t.Fatalf("precondition: library = %d, want 8", got)
		}
		beforeG, beforeH := len(e.G.Zone(state.ZGraveyard, 0)), len(e.G.Zone(state.ZHand, 0))
		emitDraw(t, e, 0)
		emitDraw(t, e, 0)
		return len(e.G.Zone(state.ZGraveyard, 0)) - beforeG, len(e.G.Zone(state.ZHand, 0)) - beforeH
	}

	t.Run("granting face gates: only the first draw is replaced", func(t *testing.T) {
		gy, hand := run(t, true)
		// One Dig: one card to the graveyard. The second draw's X is no longer
		// 0 (YouDrewThisTurn counts the first replacement's own DBDraw), so the
		// gate blocks it.
		if gy != 1 {
			t.Fatalf("graveyard growth = %d, want 1 (only the first draw replaced)", gy)
		}
		// Both draws still happen: the first via the Dig body's DBDraw, the
		// second unreplaced. So the hand grows by two.
		if hand != 2 {
			t.Fatalf("hand growth = %d, want 2", hand)
		}
	})

	t.Run("control: no Scion, no graveyard growth", func(t *testing.T) {
		gy, hand := run(t, false)
		if gy != 0 {
			t.Fatalf("graveyard growth = %d without Scion, want 0", gy)
		}
		if hand != 2 {
			t.Fatalf("hand growth = %d without Scion, want 2", hand)
		}
	})
}

// isCommanderObj reports whether id is one of any seat's commanders, the same
// predicate the Card.IsCommander filter word reads.
func (e *Engine) isCommanderObj(id state.ObjID) bool {
	for i := range e.G.Players {
		for _, c := range e.G.Players[i].Commanders {
			if c == id {
				return true
			}
		}
	}
	return false
}
