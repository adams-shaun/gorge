package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 400.7, lockstep pairs 52526/97100: a SECOND Curator of Destinies
// resolution — the Sphinx left the battlefield and returned as a new object —
// must remember only ITS OWN resolution's cards. The card's Forge script
// never clears its remembered list (no DBCleanup).IsRemembered
// | Origin$ Library`) swept a first-resolution card that was back in the
// library into the graveyard with the second pile. The kernel (resolve.go
// "a resolution remembers afresh") leaves that card in the library.
//
// The harness follows twopiles_fact_or_fiction_test.go: real compiled corpus
// cards only, decisions driven through the ordinary ask seam, log-only
// replay byte-identical at the end. No Forge script text is committed.

const curatorRevealTop = 5

// curatorReentryEngine deals seat 0 a 40-card deck: Curator of Destinies,
// five distinct reveal-window cards, then fillers; reorders the library so
// the top five are the reveal cards; adds 4UU U to the pool; casts the
// Curator; and drives through the FIRST resolution (split ask to seat 0,
// the separator "You"; pile pick to seat 1, the chooser "Opponent") with the
// opponent taking pile A. Returns the engine, config, the five first-
// resolution reveal ids in reveal order, and the Curator's id.
func curatorReentryEngine(t *testing.T) (*Engine, Config, []state.ObjID, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	curator := searchCorpusCard(t, reg, "Curator of Destinies")
	revealNames := []string{"Grizzly Bears", "Forest", "Mountain", "Island", "Swamp"}
	deck := []*cards.Card{curator}
	for _, name := range revealNames {
		deck = append(deck, searchCorpusCard(t, reg, name))
	}
	forest, mountain := searchCorpusCard(t, reg, "Forest"), searchCorpusCard(t, reg, "Mountain")
	for i := 0; i < 8; i++ {
		deck = append(deck, forest, mountain)
	}
	for len(deck) < 40 {
		deck = append(deck, searchCorpusCard(t, reg, "Grizzly Bears"))
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 52526, Names: []string{"curator", "chooser"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	curatorID := searchMoveByName(t, e, "Curator of Destinies", state.ZHand)

	// The five reveal cards must sit in the library: the opening hand may
	// have taken some, so put them back before pinning the order.
	revealIDs := make([]state.ObjID, 0, curatorRevealTop)
	for _, name := range revealNames {
		id := state.ObjID(0)
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand} {
			for _, cand := range e.G.Zone(z, 0) {
				if o := e.G.Obj(cand); o != nil && o.Face() != nil && o.Face().Name == name {
					id = cand
					break
				}
			}
			if id != 0 {
				break
			}
		}
		if id == 0 {
			t.Fatalf("reveal card %q absent from library/hand", name)
		}
		if e.G.Obj(id).Zone == state.ZHand {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZLibrary})
		}
		revealIDs = append(revealIDs, id)
	}
	lib := e.G.Zone(state.ZLibrary, 0)
	order := append([]state.ObjID(nil), revealIDs...)
	for _, id := range lib {
		if !contains(revealIDs, id) {
			order = append(order, id)
		}
	}
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order})

	addMana(t, e, 0, "UUUUUU")
	d := e.Pending()
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == curatorID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Curator of Destinies: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = curatorUntilSplit(t, e)
	// The split ask is the SEPARATOR's (Separator$ You -> seat 0) over the
	// five reveal cards in reveal order.
	if d.Player != 0 {
		t.Fatalf("split ask player = %d, want the separator 0", d.Player)
	}
	if len(d.Options) != curatorRevealTop {
		t.Fatalf("split options = %d, want %d", len(d.Options), curatorRevealTop)
	}
	for i, o := range d.Options {
		if o.Obj != revealIDs[i] {
			t.Fatalf("split option %d = %d, want %d (reveal order)", i, o.Obj, revealIDs[i])
		}
	}
	// Pile A = the first and third revealed cards; the CHOOSER (seat 1)
	// then takes pile A.
	submitChoices(t, e, d.Options[0].Index, d.Options[2].Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "twopiles_pick" {
		t.Fatalf("after the split: %+v, want the TwoPiles pile pick", d)
	}
	if d.Player != 1 {
		t.Fatalf("pick ask player = %d, want the chooser 1", d.Player)
	}
	submitChoices(t, e, d.Options[0].Index) // pile A
	for _, id := range []state.ObjID{revealIDs[0], revealIDs[2]} {
		if z := e.G.Obj(id).Zone; z != state.ZHand {
			t.Fatalf("first chosen-pile card %d zone = %s, want hand", id, z)
		}
	}
	for _, id := range []state.ObjID{revealIDs[1], revealIDs[3], revealIDs[4]} {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("first unchosen-pile card %d zone = %s, want graveyard", id, z)
		}
	}
	// Precondition for the bug: the first resolution DID write its remembered
	// set on the Curator's persistent (event-backed) list.
	if cur := e.G.Obj(curatorID); cur == nil || len(cur.Remembered) != curatorRevealTop {
		t.Fatalf("Curator remembered set = %+v, want %d entries after the first resolution", cur.Remembered, curatorRevealTop)
	}
	return e, cfg, revealIDs, curatorID
}

// curatorUntilSplit drives the decisions before a Curator split ask: the
// PeekAndReveal look acknowledgment (a Continue/yes-only KChoose) is answered
// with its single option, and any other shape is fatal. Returns the split
// KChoose.
func curatorUntilSplit(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 20; i++ {
		d := passUntilNonPriority(t, e, 40)
		if d.Kind == decision.KChoose && d.ResumeKind == "twopiles_split" {
			return d
		}
		if d.Kind == decision.KChoose && len(d.Options) == 1 && d.Options[0].Kind == "yes" {
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		t.Fatalf("unexpected decision waiting for the split: kind=%v resume=%s options=%+v", d.Kind, d.ResumeKind, d.Options)
	}
	t.Fatal("the split ask never came")
	return nil
}

// curatorDrainAcks answers any trailing yes-only acknowledgment after a pile
// pick so the resolution has fully finished before the zone asserts read it.
func curatorDrainAcks(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 10; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			return
		}
		if d.Kind == decision.KChoose && len(d.Options) == 1 && d.Options[0].Kind == "yes" {
			submitChoices(t, e, d.Options[0].Index)
			continue
		}
		t.Fatalf("unexpected decision after the pick: kind=%v resume=%s", d.Kind, d.ResumeKind)
	}
}

// TestCuratorSecondResolutionMovesOnlyItsOwnCards resolves the Curator once,
// sends it to the graveyard, returns a first-resolution card to the BOTTOM of
// the library, re-enters the Curator from the graveyard (a CR 400.7 new
// object), and drives the second resolution. The second resolution's
// unchosen pile must sweep only the second reveal window's cards: the
// recycled first-resolution card stays in the library.
func TestCuratorSecondResolutionMovesOnlyItsOwnCards(t *testing.T) {
	t.Parallel()
	e, cfg, revealIDs, curatorID := curatorReentryEngine(t)

	// Recycle a first-resolution UNCHOSEN-pile card back into the library,
	// at the BOTTOM so it is outside the second reveal window.
	recycled := revealIDs[1]
	e.emit(events.Event{Kind: events.MoveZone, Obj: recycled, From: state.ZGraveyard, To: state.ZLibrary})
	lib := e.G.Zone(state.ZLibrary, 0)
	order := make([]state.ObjID, 0, len(lib))
	for _, id := range lib {
		if id != recycled {
			order = append(order, id)
		}
	}
	order = append(order, recycled) // the bottom: outside the reveal window
	e.emit(events.Event{Kind: events.LibraryOrder, Player: 0, IDs: order})

	// The Curator leaves the battlefield and returns from the graveyard: a
	// new object under CR 400.7 (same ObjID, fresh memory).
	e.emit(events.Event{Kind: events.MoveZone, Obj: curatorID, From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: curatorID, From: state.ZGraveyard, To: state.ZBattlefield})
	if z := e.G.Obj(curatorID).Zone; z != state.ZBattlefield {
		t.Fatalf("Curator zone after re-entry = %s, want battlefield (precondition)", z)
	}

	// The second ETB trigger resolves: the split ask over the SECOND reveal
	// window (the top five fillers now). The recycled card is outside it.
	d := curatorUntilSplit(t, e)
	if d.Player != 0 {
		t.Fatalf("second split ask player = %d, want 0", d.Player)
	}
	if len(d.Options) != curatorRevealTop {
		t.Fatalf("second split options = %d, want %d over the second reveal window", len(d.Options), curatorRevealTop)
	}
	secondReveal := make([]state.ObjID, 0, curatorRevealTop)
	for _, o := range d.Options {
		if obj := e.G.Obj(o.Obj); obj == nil || obj.Zone != state.ZLibrary {
			t.Fatalf("second split option %d not in the library (precondition)", o.Obj)
		}
		secondReveal = append(secondReveal, o.Obj)
	}
	if contains(secondReveal, recycled) {
		t.Fatalf("the recycled card %d is in the second reveal window; move it to the library bottom", recycled)
	}
	submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "twopiles_pick" {
		t.Fatalf("after the second split: %+v, want the second pile pick", d)
	}
	submitChoices(t, e, d.Options[0].Index) // pile A again
	curatorDrainAcks(t, e)

	// The second resolution's own cards move as the oracle text says ...
	for _, id := range secondReveal[:2] {
		if z := e.G.Obj(id).Zone; z != state.ZHand {
			t.Fatalf("second chosen-pile card %d zone = %s, want hand", id, z)
		}
	}
	for _, id := range secondReveal[2:] {
		if z := e.G.Obj(id).Zone; z != state.ZGraveyard {
			t.Fatalf("second unchosen-pile card %d zone = %s, want graveyard", id, z)
		}
	}
	// ... and the recycled first-resolution card, which the second resolution
	// never remembered, STAYS in the library.
	if z := e.G.Obj(recycled).Zone; z != state.ZLibrary {
		t.Fatalf("recycled first-resolution card %d zone = %s, want library: the second resolution must not move a card it did not remember", recycled, z)
	}
	replayCheck(t, e, cfg)
}
