package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// nibelheimFixture seats Nibelheim Aflame in the zone named by `from` with a
// creature to target, three other cards in hand and 40 library cards, at
// seat 0's Main1 of turn 1. withFestival adds Festival of Embers on the
// battlefield AFTER Nibelheim is in the graveyard (its replacement would
// otherwise exile it on the way in).
func nibelheimFixture(t *testing.T, from state.Zone, withFestival bool) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	nib := lookup(t, reg, "Nibelheim Aflame")
	bear := card(t, "Name:Fixture Bear\nManaCost:1\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	fest := lookup(t, reg, "Festival of Embers")
	cfg := seatZeroStart(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{append([]*cards.Card{nib, bear, fest}, mountainDeck(t, 37)...), mountainDeck(t, 40)}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
	}
	nibID := moveByName(t, e, 0, "Nibelheim Aflame", from)
	moveByName(t, e, 0, "Fixture Bear", state.ZBattlefield)
	for i := 0; i < 3; i++ {
		for _, id := range e.G.Zone(state.ZLibrary, 0) {
			if o := e.G.Obj(id); o.Face().Name == "Mountain" {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
				break
			}
		}
	}
	if withFestival {
		moveByName(t, e, 0, "Festival of Embers", state.ZBattlefield)
	}
	want := 3
	if from == state.ZHand {
		want = 4
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != want {
		t.Fatalf("precondition: hand = %d cards, want %d", got, want)
	}
	if e.G.Obj(nibID).Zone != from {
		t.Fatalf("precondition: Nibelheim Aflame in %s, want %s", e.G.Obj(nibID).Zone, from)
	}
	return e, cfg, nibID
}

// castNibelheim casts nib with the option of the given Mode, targets the
// first legal creature and resolves the stack.
func castNibelheim(t *testing.T, e *Engine, nib state.ObjID, mode string, castFrom state.Zone, flashbackFlag bool) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority: %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == nib && o.Mode == mode {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no %q cast option for Nibelheim Aflame: %+v", mode, d.Options)
	}
	submitChoices(t, e, idx)
	for i := 0; i < 10; i++ {
		d = e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("precondition: the cast did not put Nibelheim Aflame on the stack")
	}
	// Asserted on the stack: the resolved spell leaves its cast provenance
	// behind when it changes zone (CR 400.7; Festival exiles it).
	o := e.G.Obj(nib)
	if !o.HasLatestCast || o.LatestCastFrom != castFrom {
		t.Fatalf("precondition: latest cast = %v from %s, want %s", o.HasLatestCast, o.LatestCastFrom, castFrom)
	}
	if (o.CastFlags&state.FlagFlashback != 0) != flashbackFlag {
		t.Fatalf("precondition: CastFlags %#x, flashback bit want %v", o.CastFlags, flashbackFlag)
	}
	resolveStack(t, e)
}

// TestWasCastFromGraveyardMayPlayFestivalNibelheim: a spell cast from the
// graveyard through Festival of Embers' may-play permission carries no
// keyword flag, but it WAS cast from a graveyard (CR 601.2a), so Nibelheim
// Aflame's Count$wasCastFromGraveyard gate discards the hand and draws four.
func TestWasCastFromGraveyardMayPlayFestivalNibelheim(t *testing.T) {
	e, cfg, nib := nibelheimFixture(t, state.ZGraveyard, true)
	addMana(t, e, 0, "RRRR")
	castNibelheim(t, e, nib, "mayplay", state.ZGraveyard, false)
	if got := len(e.G.Zone(state.ZHand, 0)); got != 4 {
		t.Fatalf("hand = %d, want 4 (three discarded, four drawn)", got)
	}
	replayCheck(t, e, cfg)
}

// TestWasCastFromGraveyardStillHoldsForFlashback: the keyword cast still reads
// as cast from a graveyard.
func TestWasCastFromGraveyardStillHoldsForFlashback(t *testing.T) {
	e, cfg, nib := nibelheimFixture(t, state.ZGraveyard, false)
	addMana(t, e, 0, "RRRRRRR")
	castNibelheim(t, e, nib, "flashback", state.ZGraveyard, true)
	if got := len(e.G.Zone(state.ZHand, 0)); got != 4 {
		t.Fatalf("hand = %d, want 4 (three discarded, four drawn)", got)
	}
	replayCheck(t, e, cfg)
}

// TestWasCastFromGraveyardFalseFromHand: cast from hand, neither discard nor
// draw.
func TestWasCastFromGraveyardFalseFromHand(t *testing.T) {
	e, cfg, nib := nibelheimFixture(t, state.ZHand, false)
	addMana(t, e, 0, "RRRR")
	if got := len(e.G.Zone(state.ZHand, 0)); got != 4 {
		t.Fatalf("precondition: hand = %d, want 4", got)
	}
	castNibelheim(t, e, nib, "", state.ZHand, false)
	if got := len(e.G.Zone(state.ZHand, 0)); got != 3 {
		t.Fatalf("hand = %d, want 3 (the other cards kept, nothing drawn)", got)
	}
	replayCheck(t, e, cfg)
}
