package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 400.7 / CR 122.2: a card moving from exile to the battlefield is a new
// object and the counters on it cease to exist. Tinybones, Bauble Burglar's
// stash counter grants the may-play while the card sits in exile; it does not
// follow a land played from exile onto the battlefield (Forge and XMage
// agree), nor a spell cast from exile onto the battlefield.
func TestStashLandPlayedFromExileEntersWithoutCounters(t *testing.T) {
	t.Parallel()
	card := corpusAlternativeCard(t, "Swamp")
	e := combatEngine(t)
	id := onBoardCard(t, e, 1, card)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "STASH", Amount: 1})
	o := e.G.Obj(id)
	if o.Zone != state.ZExile || o.Counter("STASH") != 1 {
		t.Fatalf("precondition: zone %s STASH %d, want exile / 1", o.Zone, o.Counter("STASH"))
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile, To: state.ZBattlefield})
	o = e.G.Obj(id)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("played-from-exile zone = %s, want battlefield", o.Zone)
	}
	if got := len(o.Counters); got != 0 {
		t.Fatalf("counters after playing from exile = %+v, want none (CR 400.7)", o.Counters)
	}
}

func TestStashSpellCastFromExileResolvesWithoutCounters(t *testing.T) {
	t.Parallel()
	e := mayPlayBase(t)
	stashBoard(t, e, 0, corpusAlternativeCard(t, "Tinybones, Bauble Burglar"))
	bears := graveCard(e, corpusAlternativeCard(t, "Grizzly Bears"), 1, 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: bears, From: state.ZGraveyard, To: state.ZExile})
	e.emit(events.Event{Kind: events.CounterChange, Obj: bears, Counter: "STASH", Amount: 1})
	if got := e.G.Obj(bears).Counter("STASH"); got != 1 {
		t.Fatalf("precondition: stash counters on the exiled card = %d, want 1", got)
	}
	addMana(t, e, 0, "BB")

	idx := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == bears {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("stash cast of Grizzly Bears not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, idx)
	fdnCensusDrain(t, e)
	o := e.G.Obj(bears)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("Grizzly Bears in %s after the cast resolved, want battlefield", o.Zone)
	}
	if got := len(o.Counters); got != 0 {
		t.Fatalf("counters on the resolved permanent = %+v, want none (CR 400.7)", o.Counters)
	}
}
