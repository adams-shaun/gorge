package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMadnessDiscardDuringARepeatExilesInPlace pins cardfuzz batch8 line 1
// on real corpus cards: Kindle the Carnage ("Discard a card at random ...
// You may repeat this process any number of times") discards Violent
// Eruption. Its madness replacement used to be an optional ask that the
// Repeat body ran past (the parked discard never moved, and a bot saying
// yes looped forever). Madness's replacement is mandatory (CR 702.35a;
// cr-g29): the discard moves to exile at the point of the discard, Kindle
// deals the discarded card's mana value, the Repeat election follows, and
// once Kindle has resolved the madness trigger is on the stack.
func TestMadnessDiscardDuringARepeatExilesInPlace(t *testing.T) {
	t.Parallel()
	kindle := tokenReplCorpusCard(t, "Kindle the Carnage")
	eruption := tokenReplCorpusCard(t, "Violent Eruption")
	bear := tokenReplCorpusCard(t, "Grizzly Bears")
	e, cfg := tokenReplGame(t, 9181, kindle, eruption, bear)
	bearID := moveSeededCard(t, e, 0, bear, state.ZBattlefield)
	kindleID := moveSeededCard(t, e, 0, kindle, state.ZHand)
	eruptionID := moveSeededCard(t, e, 0, eruption, state.ZHand)
	e.priorityRound()
	gainsDriveToStep(t, e, 3, 0, state.StepMain1)
	// Empty the rest of the hand so the random discard can only take
	// Violent Eruption.
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...) {
		if id != kindleID && id != eruptionID {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
		}
	}

	floatMana(t, e, 0, "RRR")
	e.pending = nil
	e.askPriority(0)
	d := e.Pending()
	cast := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == kindleID {
			cast = o.Index
		}
	}
	if cast < 0 {
		t.Fatalf("Kindle the Carnage not castable: %+v", d.Options)
	}
	submitChoices(t, e, cast)
	for i := 0; i < 4; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			break
		}
		passPriorityOnce(t, e)
	}

	if o := e.G.Obj(eruptionID); o.Zone != state.ZExile {
		t.Fatalf("Violent Eruption zone = %v after the madness discard, want exile (CR 702.35a)", o.Zone)
	}
	if got := e.G.Obj(bearID).Damage; got != 4 {
		t.Fatalf("Grizzly Bears damage = %d, want 4 (Violent Eruption's mana value)", got)
	}

	// The Repeat election comes only now; saying yes with an empty hand
	// changes nothing, so the process ends and the spell leaves the stack.
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Prompt != "Repeat this process?" {
		t.Fatalf("pending after the madness discard = %+v, want the Repeat election", d)
	}
	submitChoices(t, e, 0)
	if o := e.G.Obj(kindleID); o.Zone != state.ZGraveyard {
		t.Fatalf("Kindle the Carnage zone = %v, want it resolved into the graveyard; pending %+v", o.Zone, e.Pending())
	}
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != eruptionID {
		t.Fatalf("stack = %v, want only Violent Eruption's madness trigger", e.G.Stack)
	}
	replayCheck(t, e, cfg)
}

// TestGatedRepeatStopsWhenAnIterationChangesNothing pins cardfuzz batch8
// line 4 on the real corpus card: Rally the Horde ("Exile the top card of
// your library" three times; "If the last card exiled isn't a land card,
// repeat this process") resolved over an EMPTY library. Nothing is exiled,
// so the land-count gate keeps holding, and every pass logged only a Note
// and an Imprint "clear" of an already-empty list -- 1000 identical passes,
// which the livelock watcher killed. An empty clear is no longer an event,
// and a gated repeat whose pass posed nothing and changed nothing stops.
func TestGatedRepeatStopsWhenAnIterationChangesNothing(t *testing.T) {
	t.Parallel()
	rally := tokenReplCorpusCard(t, "Rally the Horde")
	e, cfg := tokenReplGame(t, 9182, rally)
	rallyID := moveSeededCard(t, e, 0, rally, state.ZHand)
	e.priorityRound()
	gainsDriveToStep(t, e, 3, 0, state.StepMain1)
	for _, id := range append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, 0)...) {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZGraveyard})
	}
	floatMana(t, e, 0, "RRRRRR")
	e.pending = nil
	e.askPriority(0)
	d := e.Pending()
	cast := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == rallyID {
			cast = o.Index
		}
	}
	if cast < 0 {
		t.Fatalf("Rally the Horde not castable: %+v", d.Options)
	}
	submitChoices(t, e, cast)
	mark := len(e.L.Events)
	for i := 0; i < 4 && len(e.G.Stack) > 0; i++ {
		passPriorityOnce(t, e)
	}
	if o := e.G.Obj(rallyID); o.Zone != state.ZGraveyard || len(e.G.Stack) != 0 {
		t.Fatalf("Rally the Horde zone = %v, stack %v: want it resolved", o.Zone, e.G.Stack)
	}
	stops, clears := 0, 0
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Note && ev.Text == "the repeated process changed nothing; it is not repeated again" {
			stops++
		}
		if ev.Kind == events.Imprint && ev.Text == "clear" && ev.Obj == rallyID {
			clears++
		}
	}
	if stops != 1 || clears != 0 {
		t.Fatalf("no-progress stops = %d, empty imprint clears = %d; want 1 and 0", stops, clears)
	}
	replayCheck(t, e, cfg)
}
