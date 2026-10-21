package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g09: CR 701.23 / 701.24. "Search your library ..., then shuffle" makes
// the shuffle part of the instruction: once the library is searched the
// player cannot decline it, and CR 701.24c/d shuffle even when nothing was
// found. Forge's ShuffleNonMandatory$ is NOT a "you may shuffle" marker on a
// library search: ChangeZoneEffect.changeHiddenOriginResolve reads it only to
// skip the shuffle when the searcher was not allowed to search at all
// ("if you do, shuffle" / "each player who searched shuffles"). The library
// search tail must therefore shuffle unconditionally. Every card here is the
// real compiled corpus script loaded through the gitignored registry.

// cr701NoMayShuffle fails if the pending decision is the retired library
// may-shuffle confirm.
func cr701NoMayShuffle(t *testing.T, e *Engine) {
	t.Helper()
	if d := e.Pending(); d != nil && d.ResumeKind == "search_mayshuffle" {
		t.Fatalf("CR 701.23/701.24: a library search posed %q (%q, %d options); "+
			"\"then shuffle\" is mandatory and must not be offered", d.ResumeKind, d.Prompt, len(d.Options))
	}
}

// TestCR70123SearchThenShuffleIsMandatory: Squadron Hawk's ETB ("you may
// search your library for up to three cards named Squadron Hawk, reveal
// them, put them into your hand, then shuffle") is accepted and one Hawk is
// taken. The shuffle follows with no question asked.
func TestCR70123SearchThenShuffleIsMandatory(t *testing.T) {
	t.Parallel()
	e, cfg, id := hawkSeats(t, 1, 2)
	d := hawkCastAndAccept(t, e, id)
	if d.Min != 0 || d.Max != 2 || len(d.Options) != 2 {
		t.Fatalf("precondition: search Min/Max/options = %d/%d/%d, want 0/2/2", d.Min, d.Max, len(d.Options))
	}
	start := len(e.L.Events)
	submitChoices(t, e, d.Options[0].Index) // take one Hawk
	cr701NoMayShuffle(t, e)
	passUntilStackEmpty(t, e, 20)
	if got := shuffleCountFrom(e, start, 0); got != 1 {
		t.Fatalf("seat-0 shuffles after the search = %d, want exactly 1 (CR 701.23 \"then shuffle\")", got)
	}
	if n := hawkIn(e, state.ZHand); n != 1 {
		t.Fatalf("hand Hawks after the search = %d, want the taken one", n)
	}
	replayCheck(t, e, cfg)
}

// TestCR70124FailToFindSearchStillShuffles: the same search with no Hawk left
// in the library finds nothing (CR 701.23b), and the library is still
// shuffled (CR 701.24d: "that library is shuffled even if there are no
// objects in that set").
func TestCR70124FailToFindSearchStillShuffles(t *testing.T) {
	t.Parallel()
	e, cfg, id := hawkSeats(t, 1, 0)
	addMana(t, e, 0, "WC")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KTriggerOptional || d.Player != 0 {
		t.Fatalf("pending = %+v, want seat 0's optional-trigger ask", d)
	}
	if got := hawkIn(e, state.ZLibrary); got != 0 {
		t.Fatalf("precondition: %d Hawk(s) in the library, want 0 for a fail-to-find", got)
	}
	start := len(e.L.Events)
	submitChoices(t, e, 0) // accept the trigger; the empty search completes directly
	cr701NoMayShuffle(t, e)
	passUntilStackEmpty(t, e, 20)
	if got := shuffleCountFrom(e, start, 0); got != 1 {
		t.Fatalf("seat-0 shuffles after a fail-to-find search = %d, want exactly 1 (CR 701.24d)", got)
	}
	replayCheck(t, e, cfg)
}

// TestCR70123StoneforgeSearchShufflesAwayKnownTop is the reported repro: two
// known cards sit on top of the library (what Brainstorm's put-back leaves),
// then Stoneforge Mystic's search fetches an Equipment. The search's shuffle
// is mandatory, so the player can never keep those two cards on top.
func TestCR70123StoneforgeSearchShufflesAwayKnownTop(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := searchEngine(t, reg, "Stoneforge Mystic", "Umezawa's Jitte")
	sfm := searchMoveByName(t, e, "Stoneforge Mystic", state.ZHand)
	var jitte state.ObjID
	var bears, rest []state.ObjID
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		o := e.G.Obj(id)
		switch {
		case o != nil && o.Face() != nil && o.Face().Name == "Umezawa's Jitte":
			jitte = id
			rest = append(rest, id)
		case o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" && len(bears) < 2:
			bears = append(bears, id)
		default:
			rest = append(rest, id)
		}
	}
	if jitte == 0 || len(bears) != 2 {
		t.Fatalf("precondition: Jitte %d, %d Bears in the library; want the Jitte and two Bears", jitte, len(bears))
	}
	// The Brainstorm put-back: two known cards on top, as a logged
	// LibraryOrder so the replay still verifies.
	order := append(append([]state.ObjID(nil), bears...), rest...)
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order, Secret: true})

	e.emit(events.Event{Kind: events.MoveZone, Obj: sfm, From: state.ZHand, To: state.ZBattlefield})
	e.Advance()
	d := drainUntilAsk(t, e, 30)
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("Stoneforge Mystic's optional-trigger ask missing: %+v", d)
	}
	yesIdx := -1
	for _, o := range d.Options {
		if o.Kind == "yes" {
			yesIdx = o.Index
		}
	}
	submitChoices(t, e, yesIdx)
	d = drainUntilAsk(t, e, 30)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "search" {
		t.Fatalf("pending = %+v, want the Equipment search", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == jitte {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("the Jitte %d was not offered: %+v", jitte, d.Options)
	}
	start := len(e.L.Events)
	submitChoices(t, e, pick)
	cr701NoMayShuffle(t, e)
	if got := shuffleCountFrom(e, start, 0); got != 1 {
		t.Fatalf("seat-0 shuffles after Stoneforge's search = %d, want exactly 1: the two known cards must not stay on top", got)
	}
	if o := e.G.Obj(jitte); o == nil || o.Zone != state.ZHand {
		t.Fatalf("Jitte zone after the search = %v, want hand", o)
	}
	replayCheck(t, e, cfg)
}

// TestCR70123DeclinedOptionalSearchDoesNotShuffle is the guard: "you may
// search ..., then shuffle" declined at the CR 603.5 optional trigger means
// no search happened, so there is nothing to shuffle. This is correct today
// and must stay correct.
func TestCR70123DeclinedOptionalSearchDoesNotShuffle(t *testing.T) {
	t.Parallel()
	e, cfg, id := hawkSeats(t, 1, 2)
	addMana(t, e, 0, "WC")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KTriggerOptional || d.Player != 0 {
		t.Fatalf("pending = %+v, want seat 0's optional-trigger ask", d)
	}
	noIdx := -1
	for _, o := range d.Options {
		if o.Kind == "no" {
			noIdx = o.Index
		}
	}
	if noIdx < 0 {
		t.Fatalf("optional-trigger ask has no \"no\" option: %+v", d.Options)
	}
	start := len(e.L.Events)
	submitChoices(t, e, noIdx)
	passUntilStackEmpty(t, e, 20)
	if got := shuffleCountFrom(e, start, 0); got != 0 {
		t.Fatalf("a declined optional search shuffled %d time(s), want 0", got)
	}
	if n := hawkIn(e, state.ZLibrary); n != 2 {
		t.Fatalf("library Hawks after a declined search = %d, want 2", n)
	}
	replayCheck(t, e, cfg)
}
