package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// cast_liveness_test.go pins the E2 property that a no-progress Delve cast can
// never spin (nor kill the match), as tightened by Ruling F05-2 (CR 733.2):
// the FIRST no-progress cast/activation abort leaves the declined card's
// option offered, so a merely-reversed illegal action may be redone legally,
// and only the SECOND identical no-progress abort of the SAME card in the SAME
// priority window holds that card's option out of the current window
// (suppressedCast, cast.go) -- so the re-offer loop can run at most twice
// before the thing being repeated is no longer offered, while every legal
// answer stays legal and the match stays alive. Round 1 shipped a counting
// fatal instead (two declines killed the match permanently -- a legal human
// action, b7dead); round 2 replaced it with per-card suppression; F05-2
// delays only the suppression, not the no-match-ever-dies guarantee.
//
// The sequence the engine reaches for an ordinary declined Delve cast:
//
//	priority -> (choose "cast") -> delve KChoose (Min 0, Max = shortfall)
//	  -> (answer fewer than Max) -> commitCast: payMana fails, no state
//	     change, a Note. The FIRST abort re-offers angler (CR 733.2, the
//	     legal retry); the SECOND identical abort holds angler's cast out
//	     and priority is re-offered without angler -> ... any state change
//	     clears the hold-out and the option comes back, which is also when
//	     a re-attempt can work.
//
// A legal client may answer the delve ask with any count from Min (0) to Max,
// so answering with fewer than Max is the conforming decline that used to
// spin the engine until the host's intent cap fired (E2 main).

const (
	livenessAngler = "Name:Angler\nManaCost:6 B\nTypes:Creature Zombie Fish\nPT:5/5\nK:Delve\nOracle:x\n"
	livenessGurmag = "Name:Gurmag\nManaCost:6 B\nTypes:Creature Zombie\nPT:4/4\nK:Delve\nOracle:x\n"
	livenessJunk   = "Name:Junk\nManaCost:1\nTypes:Sorcery\nOracle:x\n"
)

// fundDeclinedDelve sets up seat 0 for the declined-delve sequence: four junk
// cards in the graveyard and BGG in the pool, so the generic requirement
// (from {6}{B}, two covered by the pool) leaves a shortfall of 4 -- more than
// the zero exiles a decline answers with, so every decline aborts with no
// progress.
func fundDeclinedDelve(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 4; i++ {
		addToGraveyard(t, e, 0, livenessJunk)
	}
	addMana(t, e, 0, "BGG")
}

// castOptionFor returns the current priority decision's "cast" option for id.
func castOptionFor(t *testing.T, e *Engine, id state.ObjID) decision.Option {
	t.Helper()
	for _, o := range castOptions(t, e) {
		if o.Obj == id {
			return o
		}
	}
	t.Fatalf("no cast option for object %d", id)
	return decision.Option{}
}

// castOffered reports whether the current priority decision offers a "cast"
// option for id (without the t.Fatal castOptions does, so a test can assert
// for absence).
func castOffered(e *Engine, id state.ObjID) bool {
	d := e.Pending()
	if d == nil {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id {
			return true
		}
	}
	return false
}

// beginDelveCast submits id's cast option and asserts the delve exile ask
// follows (the castable gate offered it, so the shortfall was > 0).
func beginDelveCast(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	submitChoices(t, e, castOptionFor(t, e, id).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "exile" {
		t.Fatalf("delve exile ask not offered after choosing cast: %+v", d)
	}
}

// declineDelve answers the pending delve exile ask with nothing (Min is 0),
// the legal decline.
func declineDelve(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "exile" {
		t.Fatalf("expected the delve exile ask, got %+v", d)
	}
	submitChoices(t, e) // no indices
}

// playLand plays the first available land from the current priority decision
// (a genuine state-changing action a seat can always fall back on).
func playLand(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("not at priority to play a land: %+v", d)
	}
	for _, o := range d.Options {
		if o.Kind == "play_land" {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no play_land option available: %+v", d.Options)
}

// driver for the "unbounded spin is bounded" property: issue one delve decline
// per attempt and stop the moment angler's cast option is held out, returning
// how many delve asks were actually answered (a tiny number -- the supression
// makes a second impossible) and whether the game stayed alive throughout.
func driveDeclinedSpin(t *testing.T, e *Engine, angler state.ObjID) (asks int, alive bool) {
	t.Helper()
	for steps := 0; steps < 64; steps++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision pending (over=%v)", e.G.Over)
		}
		switch d.Kind {
		case decision.KPriority:
			if !castOffered(e, angler) {
				// The held-out suppression is what bounds the spin: the thing
				// being repeated is no longer offered.
				return asks, true
			}
			submitChoices(t, e, castOptionFor(t, e, angler).Index)
		case decision.KChoose:
			asks++
			declineDelve(t, e)
		default:
			t.Fatalf("unexpected decision kind %s", d.Kind)
		}
		if e.G.Over {
			return asks, false
		}
	}
	t.Fatalf("spin did not stop within the step budget")
	return asks, false
}

// TestDeclineThenOtherLegalPlayDoesNotEndTheMatch is the finding-1 regression
// (and would fail against round 1's counting fatal), tightened for Ruling
// F05-2 (CR 733.2): a seat that declines a Delve cast and then does anything
// else legal must not lose the match, and a productive action brings the
// declined card's option back so a re-attempt is even still possible. Under
// F05-2 the FIRST decline leaves the option offered (the legal retry), and
// only the SECOND identical one holds it out.
func TestDeclineThenOtherLegalPlayDoesNotEndTheMatch(t *testing.T) {
	e, cfg, angler := newFixtureDeck(t, 50, livenessAngler, livenessJunk, livenessJunk, livenessJunk, livenessJunk)
	fundDeclinedDelve(t, e)

	// First decline: legal (Min:0), so it must not end anything -- and, per
	// CR 733.2, it leaves angler's cast option OFFERED because a reversed
	// illegal action may be redone legally.
	beginDelveCast(t, e, angler)
	declineDelve(t, e)
	if e.G.Over {
		t.Fatal("a legal (Min:0) Delve decline ended the match")
	}
	if !castOffered(e, angler) {
		t.Fatal("a first no-progress decline still held angler out; CR 733.2 allows a legal retry")
	}

	// The retry is again declined: the SECOND identical no-progress abort of
	// the same card in the same window holds angler out, so the repeated
	// action cannot loop.
	beginDelveCast(t, e, angler)
	declineDelve(t, e)
	if e.G.Over {
		t.Fatal("a second identical decline ended the match")
	}
	if castOffered(e, angler) {
		t.Fatal("declined angler is still offered in the same no-progress window")
	}

	// Other legal play, a land (a state-changing event). This must not poke a
	// match-ending error, and -- because progress resets the hold-out -- it
	// brings angler's cast option back.
	playLand(t, e)
	if e.G.Over {
		t.Fatal("match ended after a decline and a legal land play")
	}
	if !castOffered(e, angler) {
		t.Fatal("angler's cast option did not return after a state-changing play")
	}

	// Re-declining after productive play is still legal (the state-changing
	// land reset the per-card count, so this is a fresh FIRST strike in a new
	// window) and still ends in nothing.
	beginDelveCast(t, e, angler)
	declineDelve(t, e)
	if e.G.Over {
		t.Fatal("a decline after productive play ended the match")
	}
	replayCheck(t, e, cfg)
}

// TestDeclinesOnDifferentCardsDoNotEndTheMatch is the finding-2 regression,
// strengthened for Ruling F05-2 (CR 733.2): two DIFFERENT unpayable casts in
// a row (decline on spell A, then spell B) are ordinary exploration and must
// never be treated as one accumulating spin. Each card's no-progress count is
// per-card, so one decline on each of two different cards combines into
// nothing -- neither is held out (a single strike is never enough), and the
// two counts never add up.
func TestDeclinesOnDifferentCardsDoNotEndTheMatch(t *testing.T) {
	e, cfg, angler := newFixtureDeck(t, 51, livenessAngler, livenessGurmag, livenessJunk, livenessJunk, livenessJunk, livenessJunk)
	var gurmag state.ObjID
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(id).Face().Name == "Gurmag" {
			gurmag = id
		}
	}
	if gurmag == 0 {
		// The shuffle put gurmag back in the library; find and bridge it.
		for _, id := range e.G.Zone(state.ZLibrary, 0) {
			if e.G.Obj(id).Face().Name == "Gurmag" {
				gurmag = id
			}
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: gurmag, From: state.ZLibrary, To: state.ZHand})
	}
	if gurmag == 0 {
		t.Fatal("gurmag not found in seat 0's hand or library")
	}
	fundDeclinedDelve(t, e)

	// Decline on spell A, then on spell B: both legal, both per-card. Each
	// card now has exactly ONE no-progress abort, so neither is held out --
	// the two counts never combine into a shared hold-out.
	beginDelveCast(t, e, angler)
	declineDelve(t, e)
	beginDelveCast(t, e, gurmag)
	declineDelve(t, e)

	if e.G.Over {
		t.Fatal("two declines on different cards ended the match")
	}
	if !castOffered(e, angler) {
		t.Fatal("angler held out after a single (CR 733.2 retryable) decline")
	}
	if !castOffered(e, gurmag) {
		t.Fatal("gurmag held out after a single (CR 733.2 retryable) decline")
	}

	// A SECOND identical abort on angler holds ONLY angler out; gurmag's
	// separate count of one is untouched, so gurmag stays offered. The two
	// per-card counts do not interact.
	beginDelveCast(t, e, angler)
	declineDelve(t, e)
	if e.G.Over {
		t.Fatal("a second decline on angler ended the match")
	}
	if castOffered(e, angler) {
		t.Fatal("angler still offered after its own second decline")
	}
	if !castOffered(e, gurmag) {
		t.Fatal("gurmag's offer was affected by angler's second decline")
	}
	replayCheck(t, e, cfg)
}

// TestDeclinedDelveSpinIsBounded is the "an unbounded spin is still bounded"
// property, asserted with a tiny, specific number rather than round 1's baked
// stall-threshold. Under Ruling F05-2 (CR 733.2) the FIRST no-progress decline
// leaves the card offered (the legal retry), and only the SECOND identical one
// holds it out -- so a seat can ask the delve ask exactly twice before the
// engine no longer re-offers the card. No match dies and no card moves.
func TestDeclinedDelveSpinIsBounded(t *testing.T) {
	e, cfg, angler := newFixtureDeck(t, 52, livenessAngler, livenessJunk, livenessJunk, livenessJunk, livenessJunk)
	fundDeclinedDelve(t, e)

	asks, alive := driveDeclinedSpin(t, e, angler)
	if !alive {
		t.Fatal("the driver found the match over during the (bounded) spin")
	}
	// Bounded to two declines -- the first is the CR 733.2 legal retry, the
	// second identical one triggers the hold-out that removes the re-offered
	// thing, so there is no loop to cap.
	if asks != 2 {
		t.Fatalf("declined asks before suppression = %d, want 2", asks)
	}
	if e.G.Over {
		t.Fatal("engine ended the match on the (bounded) spin")
	}
	// Halted, not corrupted: nothing moved, the card is still in hand.
	if e.G.Obj(angler).Zone != state.ZHand {
		t.Fatalf("angler %s, want hand (no card should have moved)", e.G.Obj(angler).Zone)
	}
	replayCheck(t, e, cfg)
}

// TestAuthorizedDelveAnswersStayLegal pins that the honest, legal Delve
// answers the brief calls out still work after the liveness fix, and that
// none of them leaves any card held out of future windows.
func TestAuthorizedDelveAnswersStayLegal(t *testing.T) {
	// (1) Paying the full shortfall still casts, and a successful cast leaves
	// nothing suppressed.
	t.Run("full payment casts", func(t *testing.T) {
		e, cfg, angler := newFixtureDeck(t, 53, livenessAngler, livenessJunk, livenessJunk, livenessJunk, livenessJunk)
		gy := []state.ObjID{}
		for i := 0; i < 4; i++ {
			gy = append(gy, addToGraveyard(t, e, 0, livenessJunk))
		}
		addMana(t, e, 0, "BGG")
		submitChoices(t, e, castOptionFor(t, e, angler).Index)
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.Max != 4 {
			t.Fatalf("delve decision %+v", d)
		}
		var idxs []int
		for i := 0; i < d.Max; i++ {
			idxs = append(idxs, i)
		}
		submitChoices(t, e, idxs...)
		if e.G.Obj(angler).Zone != state.ZStack {
			t.Fatalf("angler %s, want stack", e.G.Obj(angler).Zone)
		}
		if e.suppressedCast != nil && len(e.suppressedCast) != 0 {
			t.Fatal("a successful cast left cards held out")
		}
		for _, id := range gy {
			if e.G.Obj(id).Zone != state.ZExile {
				t.Fatal("delved card not exiled")
			}
		}
		replayCheck(t, e, cfg)
	})

	// (2) A partial-but-sufficient cover still casts: the shortfall is less
	// than the whole generic requirement because the pool already pays part,
	// so the delve cover (here 4 of the 6 generic) is partial yet sufficient.
	t.Run("partial-but-sufficient cover casts", func(t *testing.T) {
		e, cfg, angler := newFixtureDeck(t, 46, livenessAngler, livenessJunk, livenessJunk, livenessJunk, livenessJunk)
		for i := 0; i < 4; i++ {
			addToGraveyard(t, e, 0, livenessJunk)
		}
		addMana(t, e, 0, "BGG")
		submitChoices(t, e, castOptionFor(t, e, angler).Index)
		d := e.Pending()
		if d == nil || d.Kind != decision.KChoose {
			t.Fatalf("delve decision %+v", d)
		}
		var idxs []int
		for i := 0; i < d.Max; i++ {
			idxs = append(idxs, i)
		}
		submitChoices(t, e, idxs...)
		if e.G.Obj(angler).Zone != state.ZStack || (e.suppressedCast != nil && len(e.suppressedCast) != 0) {
			t.Fatalf("angler %s, want stack with nothing held out", e.G.Obj(angler).Zone)
		}
		replayCheck(t, e, cfg)
	})

	// (3) Exiling nothing when nothing is required still works: with a big
	// enough pool the delve ask is skipped entirely and the cast just goes
	// through (already pinned by TestDelveSkipsCleanlyWithAnEmptyGraveyard;
	// re-asserted here so the honest "nothing required" arm of the brief is
	// covered in the regression that guards the liveness bound).
	t.Run("nothing required casts", func(t *testing.T) {
		e, cfg, angler := newFixtureDeck(t, 47, livenessAngler)
		addMana(t, e, 0, "BBBBBBB")
		submitChoices(t, e, castOptions(t, e)[0].Index)
		if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
			t.Fatalf("delve asked an exile decision when nothing was required: %+v", d)
		}
		if e.G.Obj(angler).Zone != state.ZStack || (e.suppressedCast != nil && len(e.suppressedCast) != 0) {
			t.Fatalf("angler %s, want stack with nothing held out", e.G.Obj(angler).Zone)
		}
		replayCheck(t, e, cfg)
	})
}
