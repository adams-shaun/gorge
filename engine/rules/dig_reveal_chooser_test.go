package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// digRiderDrive answers decisions with their defaults (pass priority, the
// first Min options otherwise) until stop reports true, checked before each
// answer (with the pending decision, nil when none), and returns that decision.
func digRiderDrive(t *testing.T, e *Engine, stop func(*decision.Decision) bool) *decision.Decision {
	t.Helper()
	for i := 0; i < 300 && !e.G.Over; i++ {
		d := e.Pending()
		if stop(d) {
			return d
		}
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			passOnce(t, e)
			continue
		}
		ch := []int{}
		for k := 0; k < d.Min && k < len(d.Options); k++ {
			ch = append(ch, d.Options[k].Index)
		}
		submitChoices(t, e, ch...)
	}
	t.Fatal("digRiderDrive: stop condition never reached")
	return nil
}

// digRiderDriveTo drives to a pending decision whose ResumeKind is resume.
func digRiderDriveTo(t *testing.T, e *Engine, resume string) *decision.Decision {
	t.Helper()
	return digRiderDrive(t, e, func(d *decision.Decision) bool { return d != nil && d.ResumeKind == resume })
}

// urzaChapterOne puts Urza Assembles the Titans onto seat 0's battlefield
// (chapter I triggers on entry) with nPW planeswalker cards on top of the
// library, and drives to chapter I's reveal election. It returns the engine, the
// replay config, the top card at the election and its derived name.
func urzaChapterOne(t *testing.T, nPW int) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	jace := lookup(t, reg, "Jace, the Mind Sculptor")
	deck := []*cards.Card{lookup(t, reg, "Urza Assembles the Titans")}
	for range nPW {
		deck = append(deck, jace)
	}
	e, cfg := corpusEngineCfg(t, reg, deck, nil)
	// Every Jace goes to the top of the library (a logged LibraryOrder), so
	// whatever Scry 4 does with them a planeswalker is still on top for the Dig.
	var top []state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if e.G.Obj(id).Face().Name == "Jace, the Mind Sculptor" {
				top = append(top, id)
			}
		}
	}
	if len(top) != nPW {
		t.Fatalf("precondition: found %d Jace cards, want %d", len(top), nPW)
	}
	for _, id := range top {
		if inZone(e, state.ZHand, 0, id) {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
		}
	}
	inTop := map[state.ObjID]bool{}
	for _, id := range top {
		inTop[id] = true
	}
	order := append([]state.ObjID(nil), top...)
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if !inTop[id] {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order})
	moveByName(t, e, 0, "Urza Assembles the Titans", state.ZBattlefield)
	d := digRiderDriveTo(t, e, "dig_reveal_optional")
	if d.Player != 0 || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
		t.Fatalf("reveal election = %+v, want seat 0's yes/no", d)
	}
	return e, cfg, e.G.Zone(state.ZLibrary, 0)[0]
}

func answerDigReveal(t *testing.T, e *Engine, yes bool) {
	t.Helper()
	answerYesNoElection(t, e, "dig_reveal_optional", yes)
	answerQuiet(t, e, 80)
}

func TestUrzaAssemblesRevealOptionalTakesPlaneswalker(t *testing.T) {
	t.Parallel()
	e, cfg, top := urzaChapterOne(t, 5)
	if name := e.G.Obj(top).Face().Name; name != "Jace, the Mind Sculptor" {
		t.Fatalf("precondition: top card = %q, want a planeswalker", name)
	}
	answerDigReveal(t, e, true)
	if !inZone(e, state.ZHand, 0, top) {
		t.Fatalf("revealed planeswalker %d is not in seat 0's hand (zone %v)", top, e.G.Obj(top).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestUrzaAssemblesRevealDeclinedKeepsCard(t *testing.T) {
	t.Parallel()
	e, cfg, top := urzaChapterOne(t, 5)
	if name := e.G.Obj(top).Face().Name; name != "Jace, the Mind Sculptor" {
		t.Fatalf("precondition: top card = %q, want a planeswalker", name)
	}
	answerDigReveal(t, e, false)
	if got := e.G.Zone(state.ZLibrary, 0); len(got) == 0 || got[0] != top {
		t.Fatalf("declined reveal moved the top card: library head %v, want %d", got, top)
	}
	if inZone(e, state.ZHand, 0, top) {
		t.Fatalf("declined reveal still put %d into hand", top)
	}
	replayCheck(t, e, cfg)
}

func TestUrzaAssemblesNonPlaneswalkerStays(t *testing.T) {
	t.Parallel()
	e, cfg, top := urzaChapterOne(t, 0)
	if name := e.G.Obj(top).Face().Name; name != "Mountain" {
		t.Fatalf("precondition: top card = %q, want a non-planeswalker Mountain", name)
	}
	answerDigReveal(t, e, true)
	if inZone(e, state.ZHand, 0, top) {
		t.Fatalf("revealed Mountain %d was put into hand", top)
	}
	if got := e.G.Zone(state.ZLibrary, 0); len(got) == 0 || got[0] != top {
		t.Fatalf("revealed Mountain left the top: library head %v, want %d", got, top)
	}
	replayCheck(t, e, cfg)
}

// TestMemoriesReturningSecondPickIsOpponent: Choser$ Opponent chooses seat 1
// for the first bottom pick and SetChosenPlayer$ records it, so the last Dig's
// Choser$ ChosenPlayer is seat 1 too, not the caster.
func TestMemoriesReturningSecondPickIsOpponent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Memories Returning")}, nil)
	moveByName(t, e, 0, "Memories Returning", state.ZHand)
	addMana(t, e, 0, "UUUU")
	castNamed(t, e, "Memories Returning")
	var seats []state.PlayerID
	for len(seats) < 4 {
		d := digRiderDriveTo(t, e, "dig")
		seats = append(seats, d.Player)
		if len(d.Options) < 2 {
			t.Fatalf("precondition: dig %d offers %d cards, want a real choice", len(seats), len(d.Options))
		}
		submitChoices(t, e, d.Options[0].Index)
	}
	want := []state.PlayerID{0, 1, 0, 1}
	for i := range want {
		if seats[i] != want[i] {
			t.Fatalf("dig decisions posed to seats %v, want %v", seats, want)
		}
	}
	answerQuiet(t, e, 80)
	replayCheck(t, e, cfg)
}

// TestGontiForceRevealLookRecordedForSourceController: Gonti, Night Minister's
// Dig names ForceReveal$ TriggeredSourceController, so the look at the top card
// of the damaged opponent's library is recorded for that seat. (With two seats
// the creature's controller is also the ability's controller, so this pins the
// look being recorded at all; the DefinedExiler$ layout already names the seat.)
func TestGontiForceRevealLookRecordedForSourceController(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Gonti, Night Minister")}, nil)
	gonti := moveByName(t, e, 0, "Gonti, Night Minister", state.ZBattlefield)
	attackWith(t, e, gonti)
	oppTop := e.G.Zone(state.ZLibrary, 1)[0]
	mark := len(e.L.Events)
	digRiderDrive(t, e, func(*decision.Decision) bool { return e.G.Obj(oppTop).Zone != state.ZLibrary })
	answerQuiet(t, e, 60)
	if o := e.G.Obj(oppTop); o == nil || o.Zone != state.ZExile || !o.FaceDown {
		t.Fatalf("precondition: seat 1's top card %d is not face down in exile: %+v", oppTop, o)
	}
	var looked bool
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.Note && ev.Secret && ev.Player == 0 && len(ev.IDs) == 1 && ev.IDs[0] == oppTop {
			looked = true
		}
	}
	if !looked {
		t.Fatal("no Secret look Note for seat 0 naming seat 1's top card")
	}
	replayCheck(t, e, cfg)
}
