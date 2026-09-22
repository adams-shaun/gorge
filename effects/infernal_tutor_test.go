package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// infernalTutorCorpus returns the REAL compiled corpus card Infernal Tutor
// (.cards/cardsfolder/i/infernal_tutor.txt), failing the test when the corpus
// pin moved. Its main SA is
//
//	SP$ Reveal | RememberRevealed$ True | Defined$ You | SubAbility$ DBChangeZone
//	SVar:DBChangeZone:DB$ ChangeZone | Origin$ Library | Destination$ Hand
//	    | ChangeType$ Remembered.sameName | ChangeNum$ 1 ...
//	SVar:DBChangeZone2:DB$ ChangeZone | ... ChangeType$ Card ...  (hellbent)
//
// so the reveal's Remembered capture is exactly what the chained search
// reads. The test drives the real script, never a synthetic fixture.
func infernalTutorCorpus(t *testing.T) (*cards.Card, *cards.SA) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup("Infernal Tutor")
	if !ok {
		t.Skip("corpus missing Infernal Tutor")
	}
	if len(c.Faces) == 0 {
		t.Fatal("corpus card Infernal Tutor has no face")
	}
	face := c.Faces[0]
	var main *cards.SA
	for _, a := range face.Abilities {
		if a.API == "Reveal" {
			main = a
			break
		}
	}
	if main == nil {
		t.Fatal("corpus pin moved: Infernal Tutor carries no SP$ Reveal")
	}
	if main.Sub == nil {
		t.Fatal("corpus card's Reveal SA is not linked to its SubAbility chain")
	}
	if main.Sub.Params["ChangeType"] != "Remembered.sameName" {
		t.Fatalf("corpus pin moved: chained search ChangeType$ = %q, want Remembered.sameName", main.Sub.Params["ChangeType"])
	}
	return c, main
}

// infernalTutorBoard builds a 2-seat game whose seat 0 holds a two-card hand
// (handOrder[0] first) and a library holding libOrder (in order), and returns
// the host, the ctx (with the card's real SVar table bound) and the ids keyed
// by name. Cards are inline fixtures; the behaviour under test is the REAL
// Infernal Tutor script, which matches them by name.
func infernalTutorBoard(t *testing.T, handOrder, libOrder []string) (*fakeHost, *Ctx, map[string]state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	ids := map[string]state.ObjID{}
	add := func(name string, zone state.Zone, owner state.PlayerID) state.ObjID {
		o := h.g.AddObject(mkCard(t, "Name:"+name+"\nTypes:Sorcery\nOracle:x\n"), owner)
		o.Zone = zone
		ids[name] = o.ID
		return o.ID
	}
	var hand []state.ObjID
	for _, n := range handOrder {
		if id, ok := ids[n]; ok {
			hand = append(hand, id)
			continue
		}
		hand = append(hand, add(n, state.ZHand, 0))
	}
	h.g.SetZone(state.ZHand, 0, hand)
	var lib []state.ObjID
	for _, n := range libOrder {
		// A library card may share a name with a hand card; both objects are
		// distinct, so key them by a library-qualified name for the caller.
		id := h.g.AddObject(mkCard(t, "Name:"+n+"\nTypes:Sorcery\nOracle:x\n"), 0).ID
		h.g.Obj(id).Zone = state.ZLibrary
		lib = append(lib, id)
		ids["lib:"+n] = id
	}
	h.g.SetZone(state.ZLibrary, 0, lib)
	src := h.g.AddObject(mkCard(t, "Name:Infernal Tutor\nManaCost:1 B\nTypes:Sorcery\nOracle:x\n"), 0)
	ctx := &Ctx{Source: src.ID, Controller: 0}
	return h, ctx, ids
}

// revealPickNotes returns the single public reveal Note carrying ids.
func revealPickNotes(t *testing.T, log []events.Event) *events.Event {
	t.Helper()
	var note *events.Event
	for i := range log {
		if log[i].Kind == events.Note && !log[i].Secret && len(log[i].IDs) > 0 {
			if note != nil {
				t.Fatalf("more than one reveal Note: %+v", log)
			}
			note = &log[i]
		}
	}
	return note
}

// TestInfernalTutorHandRevealAsksWhichCard is the reporter's card: with two
// cards in hand, "Reveal a card from your hand" is a CHOICE, not the front of
// the hand. Pre-fix effReveal took pool[:1] silently, so Infernal Tutor
// revealed the first hand card and searched for its name (the probe's "a
// Forest instead of the Lightning Bolt").
//
// This pins the ask itself: a KChoose to the revealing player (seat 0) over
// the eligible hand cards, ResumeKind "reveal_pick", exactly one pick.
func TestInfernalTutorHandRevealAsksWhichCard(t *testing.T) {
	card, main := infernalTutorCorpus(t)
	h, ctx, ids := infernalTutorBoard(t, []string{"Lightning Bolt", "Forest"}, []string{"Forest", "Lightning Bolt"})
	SetSVars(ctx, card.Faces[0].SVars)
	sh := &suspendHost{fakeHost: *h}

	Resolve(sh, ctx, main)

	if !sh.suspended || sh.asked == nil {
		t.Fatalf("a two-card hand reveal posed no decision: %+v", sh.log)
	}
	d := sh.asked
	if d.Player != 0 {
		t.Fatalf("decider = seat %d, want the revealing player (seat 0)", d.Player)
	}
	if d.Kind != decision.KChoose || d.ResumeKind != "reveal_pick" {
		t.Fatalf("ask = kind %s resume %q, want KChoose/reveal_pick", d.Kind, d.ResumeKind)
	}
	if d.Min != 1 || d.Max != 1 {
		t.Fatalf("Min/Max = %d/%d, want 1/1 (NumCards default)", d.Min, d.Max)
	}
	if len(d.Options) != 2 {
		t.Fatalf("options = %d, want the 2 eligible hand cards", len(d.Options))
	}
	if d.Options[0].Obj != ids["Lightning Bolt"] || d.Options[1].Obj != ids["Forest"] {
		t.Fatalf("options = %+v, want the hand in zone order", d.Options)
	}
	for _, e := range sh.log {
		if e.Kind == events.Note {
			t.Fatalf("a reveal Note landed before the answer: %+v", e)
		}
	}
}

// TestInfernalTutorChosenCardDrivesTheSearch answers the pick with the SECOND
// hand card (Forest) and proves the whole real chain honours it: the reveal
// Note names the Forest, RememberRevealed$ captures the Forest, and the
// chained ChangeZone searches for a card NAMED FOREST -- which it finds in the
// library. Pre-fix the front card (Lightning Bolt) was revealed and the
// searched name would have been Lightning Bolt.
func TestInfernalTutorChosenCardDrivesTheSearch(t *testing.T) {
	card, main := infernalTutorCorpus(t)
	h, ctx, ids := infernalTutorBoard(t, []string{"Lightning Bolt", "Forest"}, []string{"Lightning Bolt", "Forest"})
	SetSVars(ctx, card.Faces[0].SVars)
	sh := &suspendHost{fakeHost: *h}

	Resolve(sh, ctx, main) // suspends on the pick
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_pick" {
		t.Fatalf("no reveal_pick ask: %+v", sh.asked)
	}
	// Answer: reveal the SECOND hand card (the Forest).
	sh.suspended = false
	sh.asked = nil
	ctx.RevealPick = []state.ObjID{ids["Forest"]}
	Resolve(sh, ctx, main)

	// The reveal itself: exactly the chosen card.
	note := revealPickNotes(t, sh.log)
	if note == nil {
		t.Fatalf("no public reveal Note: %+v", sh.log)
	}
	if !slices.Equal(note.IDs, []state.ObjID{ids["Forest"]}) {
		t.Fatalf("reveal ids = %v, want exactly the chosen Forest %d", note.IDs, ids["Forest"])
	}
	if len(ctx.Remembered) != 1 || ctx.Remembered[0].Obj != ids["Forest"] {
		t.Fatalf("Remembered = %+v, want the chosen Forest", ctx.Remembered)
	}
	// The chained ChangeZone now searches for Remembered.sameName. Its
	// hidden-library pick is a real ask (KChoose with ResumeKind "search")
	// whose ONLY eligible option is the library card sharing the CHOSEN card's name -- proof the
	// sub read the chosen card and not the front of the hand.
	if sh.asked == nil || sh.asked.ResumeKind != "search" {
		t.Fatalf("chained search did not ask over the remembered name: %+v", sh.asked)
	}
	if len(sh.asked.Options) != 1 || sh.asked.Options[0].Obj != ids["lib:Forest"] {
		t.Fatalf("search offered %+v, want only the chosen name's library card", sh.asked.Options)
	}
}

// TestInfernalTutorHellbentEmptyHandSearchesAnyCard pins the hellbent leg: an
// empty hand has no eligible card, so the reveal poses NO pick and captures
// nothing, and the card's own SVar gate routes to DBChangeZone2 (the any-card
// search). The failure mode this guards is a regression where the empty-hand
// reveal wedges on a pick ask over zero options.
func TestInfernalTutorHellbentEmptyHandSearchesAnyCard(t *testing.T) {
	card, main := infernalTutorCorpus(t)
	h, ctx, ids := infernalTutorBoard(t, nil, []string{"Lightning Bolt"})
	h.g.SetZone(state.ZHand, 0, nil)
	SetSVars(ctx, card.Faces[0].SVars)
	sh := &suspendHost{fakeHost: *h}

	Resolve(sh, ctx, main)

	// An empty hand has no card to reveal: no reveal_pick ask of any kind.
	if sh.asked != nil && sh.asked.ResumeKind == "reveal_pick" {
		t.Fatalf("an empty hand posed a reveal pick: %+v", sh.asked)
	}
	if len(ctx.Remembered) != 0 {
		t.Fatalf("Remembered = %+v, want nothing revealed from an empty hand", ctx.Remembered)
	}
	// The hellbent branch (ConditionSVarCompare$ LT1) runs the any-card
	// search; its hidden-library pick offers whatever the library holds (no
	// same-name restriction).
	if sh.asked == nil || sh.asked.ResumeKind != "search" {
		t.Fatalf("hellbent search did not ask: %+v", sh.asked)
	}
	if len(sh.asked.Options) != 1 || sh.asked.Options[0].Obj != ids["lib:Lightning Bolt"] {
		t.Fatalf("hellbent search offered %+v, want the any-card library pick", sh.asked.Options)
	}
}

// TestOptionalHandRevealPosesThePickAfterYes pins the Optional$ interaction
// the pick introduces: a "you may reveal" hand reveal with more eligible
// cards than it must show keeps its reveal_optional yes/no ask, and an
// accepted "yes" then poses the reveal_pick over the eligible cards -- with
// no re-posed optional ask on the pick's resume (the fx42 re-ask that would
// be a soft-lock in a client). This is the `picks == nil` guard on the
// optional ask, not the peek shape (PeekAndReveal has no hand pick).
func TestOptionalHandRevealPosesThePickAfterYes(t *testing.T) {
	h := newHost(t, 2)
	var hand []state.ObjID
	for _, n := range []string{"Alpha", "Beta", "Gamma"} {
		o := h.g.AddObject(mkCard(t, "Name:"+n+"\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
		o.Zone = state.ZHand
		hand = append(hand, o.ID)
	}
	h.g.SetZone(state.ZHand, 0, hand)
	sh := &suspendHost{fakeHost: *h}
	ctx := &Ctx{Controller: 0, Source: hand[0]}
	m := sa(t, "SP$ Reveal | Defined$ You | Optional$ True")

	// Pass 1: the may-reveal yes/no, unchanged.
	Resolve(sh, ctx, m)
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_optional" {
		t.Fatalf("first pass = %+v, want the reveal_optional ask", sh.asked)
	}
	// Pass 2 ("yes"): the pick over the whole eligible hand, Min == Max == 1.
	sh.suspended, sh.asked = false, nil
	ctx.RevealOpt = "yes"
	Resolve(sh, ctx, m)
	if sh.asked == nil || sh.asked.ResumeKind != "reveal_pick" || sh.asked.Min != 1 || sh.asked.Max != 1 || len(sh.asked.Options) != 3 {
		t.Fatalf("after yes = %+v, want a 1-of-3 reveal_pick", sh.asked)
	}
	// Pass 3 (the pick): exactly the chosen card, and NO re-posed optional.
	sh.suspended, sh.asked = false, nil
	ctx.RevealPick = []state.ObjID{hand[1]}
	Resolve(sh, ctx, m)
	if sh.asked != nil {
		t.Fatalf("the pick resume re-posed an ask (soft-lock): %+v", sh.asked)
	}
	note := revealPickNotes(t, sh.log)
	if note == nil || !slices.Equal(note.IDs, []state.ObjID{hand[1]}) {
		t.Fatalf("reveal note = %+v, want exactly the chosen Beta", note)
	}
}
