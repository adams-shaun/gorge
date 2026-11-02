package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// stashBoard puts corpus card c onto owner's battlefield through a logged
// MoveZone (onBoardGrant's shape, for a compiled corpus card).
func stashBoard(t *testing.T, e *Engine, owner state.PlayerID, c *cards.Card) state.ObjID {
	t.Helper()
	o := e.G.AddObject(c, owner)
	o.Zone = state.ZLibrary
	ids := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, owner)...)
	e.G.SetZone(state.ZLibrary, owner, append(ids, o.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
	return o.ID
}

// TestStashCastAnyTypeManaThroughWindow pins Tinybones, Bauble Burglar's
// "mana of any type can be spent to cast those spells" across the CR 601.2g
// mana window: Eaten Alive (an opponent's card with a stash counter) is cast
// with its {3}{B} additional cost from one floating {B} and four untapped
// Forests. The rider must reach the target ask's window-reachability probe
// and the window's per-source usefulness clamp; without it the first drops
// every target ("cast aborted: no legal target") and the second withheld the
// last Forest (no {B} demand it could fill), so the cast was reversed at
// payment (census pairs 30973 40713 68533 97976).
func TestStashCastAnyTypeManaThroughWindow(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	stashBoard(t, e, 0, corpusAlternativeCard(t, "Tinybones, Bauble Burglar"))
	victim := stashBoard(t, e, 1, corpusAlternativeCard(t, "Grizzly Bears"))
	spell := graveCard(e, corpusAlternativeCard(t, "Eaten Alive"), 1, 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: spell, From: state.ZGraveyard, To: state.ZExile})
	e.emit(events.Event{Kind: events.CounterChange, Obj: spell, Counter: "STASH", Amount: 1})
	for i := 0; i < 4; i++ {
		stashBoard(t, e, 0, corpusAlternativeCard(t, "Forest"))
	}
	addMana(t, e, 0, "B")

	idx := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == spell {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("stash cast of Eaten Alive not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	sawTarget := false
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		pick := -1
		for _, o := range d.Options {
			switch {
			case o.Kind == "altaddcost" && o.Amount == 1: // "Pay 3 B"
				pick = o.Index
			case d.Kind == decision.KTarget && o.Obj == victim:
				pick, sawTarget = o.Index, true
			case o.Kind == "activate" && pick < 0:
				pick = o.Index
			}
		}
		if pick < 0 {
			t.Fatalf("unexpected ask %q: %+v", d.Prompt, d.Options)
		}
		submitChoices(t, e, pick)
	}
	if !sawTarget {
		t.Fatalf("no target ask: the proposal was reversed before targets (spell in %s)", e.G.Obj(spell).Zone)
	}
	if z := e.G.Obj(spell).Zone; z != state.ZStack {
		t.Fatalf("Eaten Alive in %s after paying {3}{B}{B} from B+GGGG under the any-type rider, want stack", z)
	}
	fdnCensusDrain(t, e)
	if z := e.G.Obj(victim).Zone; z != state.ZExile {
		t.Fatalf("victim in %s, want exile", z)
	}
}
