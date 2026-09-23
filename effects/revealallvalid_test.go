package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// revealAllCorpus returns the REAL compiled corpus card named name, failing the
// test when the corpus pin moved. The tests below drive the real Break
// Expectations / Mind Spike scripts, never a synthetic RevealAllValid$ fixture.
func revealAllCorpus(t *testing.T, name string) *cards.Card {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	if len(c.Faces) == 0 {
		t.Fatalf("corpus card %q has no face", name)
	}
	return c
}

// revealAllMain returns the card's first SP$ Reveal SA (the spell's own reveal
// line, not a trigger's DB$ Reveal sub-ability), failing the test if the
// corpus pin moved the shape.
func revealAllMain(t *testing.T, name string) (*cards.Card, *cards.SA) {
	t.Helper()
	c := revealAllCorpus(t, name)
	var main *cards.SA
	for _, a := range c.Faces[0].Abilities {
		if a.API == "Reveal" {
			main = a
			break
		}
	}
	if main == nil {
		t.Fatalf("corpus pin moved: %q carries no Reveal ability on its face", name)
	}
	if main.Params["RevealAllValid"] == "" {
		t.Fatalf("corpus pin moved: %q carries no RevealAllValid$", name)
	}
	if main.Kind != "SP" {
		t.Fatalf("corpus pin moved: %q's Reveal is not the spell ability (SP$)", name)
	}
	return c, main
}

// revealAllHandBoard builds a 2-seat game whose seat handOwner holds the named
// corpus cards in the given order (index 0 first), with the named cards' REAL
// mana costs and types so the RevealAllValid$ filter sees the honest values.
// Returns the host, the ctx (source = Break Expectations / Mind Spike on the
// stack, targeting seat handOwner), and the ids keyed by name.
func revealAllHandBoard(t *testing.T, source *cards.Card, handOwner state.PlayerID, hand []string) (*fakeHost, *Ctx, map[string]state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	ids := map[string]state.ObjID{}
	var cardsInHand []state.ObjID
	for _, name := range hand {
		c, ok := testutil.CorpusRegistry(t).Lookup(name)
		if !ok {
			t.Fatalf("corpus has no %q", name)
		}
		o := h.g.AddObject(c, handOwner)
		live := h.g.Obj(o.ID)
		live.Zone = state.ZHand
		ids[name] = o.ID
		cardsInHand = append(cardsInHand, o.ID)
	}
	h.g.SetZone(state.ZHand, handOwner, cardsInHand)
	src := h.g.AddObject(source, 0)
	ctx := &Ctx{
		Source:         src.ID,
		Controller:     0,
		Targets:        []state.Target{{Player: handOwner, IsPlayer: true}},
		TargetsOffered: true,
	}
	return h, ctx, ids
}

// TestRevealAllValidBreakExpectationsRevealsEveryMatch is the reporter's card.
// Break Expectations' SP$ Reveal carries
//
//	RevealAllValid$ Card.cmcGE2+TargetedPlayerCtrl
//
// so with a hand of [Plains (cmc 0), Lightning Bolt (cmc 1), Grizzly Bears
// (cmc 2), Hill Giant (cmc 4)] the reveal must show the TWO cmc>=2 cards in
// hand order, never the first hand card. Pre-fix effReveal never fed the spec
// to the filter and took pool[:1] -- the Plains, which matches neither half of
// the spec.
func TestRevealAllValidBreakExpectationsRevealsEveryMatch(t *testing.T) {
	card, main := revealAllMain(t, "Break Expectations")
	h, ctx, ids := revealAllHandBoard(t, card, 1, []string{"Plains", "Lightning Bolt", "Grizzly Bears", "Hill Giant"})
	SetSVars(ctx, card.Faces[0].SVars)

	// Precondition: the compared mana values really differ, and the hand is
	// in the zone the reveal reads.
	if got := cmcOf(t, h, ids["Plains"]); got != 0 {
		t.Fatalf("precondition: Plains cmc = %d, want 0", got)
	}
	if got := cmcOf(t, h, ids["Lightning Bolt"]); got != 1 {
		t.Fatalf("precondition: Lightning Bolt cmc = %d, want 1", got)
	}
	if got := cmcOf(t, h, ids["Grizzly Bears"]); got != 2 {
		t.Fatalf("precondition: Grizzly Bears cmc = %d, want 2", got)
	}
	if got := cmcOf(t, h, ids["Hill Giant"]); got != 4 {
		t.Fatalf("precondition: Hill Giant cmc = %d, want 4", got)
	}
	if h.g.Obj(ids["Grizzly Bears"]).Zone != state.ZHand {
		t.Fatal("precondition: Grizzly Bears is not in the hand the reveal reads")
	}

	sh := &suspendHost{fakeHost: *h}
	Resolve(sh, ctx, main)

	note := revealPickNotes(t, sh.log)
	if note == nil {
		t.Fatalf("no public reveal Note: %+v", sh.log)
	}
	want := []state.ObjID{ids["Grizzly Bears"], ids["Hill Giant"]}
	if !slices.Equal(note.IDs, want) {
		t.Fatalf("reveal ids = %v, want exactly the two cmc>=2 hand cards %v", note.IDs, want)
	}
	// The revealer picks nothing: a RevealAllValid$ reveal is not a choice.
	if sh.asked != nil && sh.asked.ResumeKind == "reveal_pick" {
		t.Fatalf("a RevealAllValid$ reveal posed a revealer pick: %+v", sh.asked)
	}
}

// TestRevealAllValidMindSpikeRemembersEveryMatch pins the RememberRevealed$
// capture: Mind Spike's SP$ Reveal carries
//
//	RevealAllValid$ Card.nonLand+nonCreature+TargetedPlayerCtrl
//	RememberRevealed$ True
//
// so a hand of two noncreature, nonland cards plus a creature and a land must
// reveal exactly the two matching cards AND capture exactly those two into the
// walk's Remembered set (a chained ConditionDefined$ Remembered gate reads it).
func TestRevealAllValidMindSpikeRemembersEveryMatch(t *testing.T) {
	card, main := revealAllMain(t, "Mind Spike")
	h, ctx, ids := revealAllHandBoard(t, card, 1, []string{"Lightning Bolt", "Grizzly Bears", "Plains", "Counterspell"})
	SetSVars(ctx, card.Faces[0].SVars)

	// Precondition: the pool really holds two of each class the spec splits on.
	bolt := h.g.Obj(ids["Lightning Bolt"])
	bears := h.g.Obj(ids["Grizzly Bears"])
	plains := h.g.Obj(ids["Plains"])
	counter := h.g.Obj(ids["Counterspell"])
	if !isInstantOrSorcery(t, bolt) || !isInstantOrSorcery(t, counter) {
		t.Fatal("precondition: Lightning Bolt / Counterspell are not noncreature nonland cards")
	}
	if !hasType(bears, "Creature") || hasType(bears, "Land") {
		t.Fatal("precondition: Grizzly Bears must be a creature for the nonCreature branch to exclude it")
	}
	if !hasType(plains, "Land") {
		t.Fatal("precondition: Plains must be a land for the nonLand branch to exclude it")
	}

	sh := &suspendHost{fakeHost: *h}
	Resolve(sh, ctx, main)

	note := revealPickNotes(t, sh.log)
	if note == nil {
		t.Fatalf("no public reveal Note: %+v", sh.log)
	}
	want := []state.ObjID{ids["Lightning Bolt"], ids["Counterspell"]}
	if !slices.Equal(note.IDs, want) {
		t.Fatalf("reveal ids = %v, want exactly the two matching cards %v", note.IDs, want)
	}
	// The RememberRevealed$ capture must carry the whole matching set.
	var remembered []state.ObjID
	for _, tr := range ctx.Remembered {
		if tr.Obj != 0 {
			remembered = append(remembered, tr.Obj)
		}
	}
	if !slices.Equal(remembered, want) {
		t.Fatalf("Remembered = %v, want exactly the revealed set %v", remembered, want)
	}
}

// TestRevealAllValidZeroMatchSkipsCleanly pins the fail-closed direction: with
// no card in the hand satisfying Break Expectations' Card.cmcGE2+..., the
// reveal emits no Note with ids, captures no RememberRevealed$, and does not
// panic. The n == 0 skip is the same convention RevealValid$/RevealType$ use.
func TestRevealAllValidZeroMatchSkipsCleanly(t *testing.T) {
	card, main := revealAllMain(t, "Break Expectations")
	h, ctx, ids := revealAllHandBoard(t, card, 1, []string{"Plains", "Lightning Bolt", "Savannah Lions"})
	SetSVars(ctx, card.Faces[0].SVars)

	// Precondition: every hand card really is below cmc 2.
	for _, name := range []string{"Plains", "Lightning Bolt", "Savannah Lions"} {
		if got := cmcOf(t, h, ids[name]); got >= 2 {
			t.Fatalf("precondition: %s cmc = %d, want < 2", name, got)
		}
	}

	sh := &suspendHost{fakeHost: *h}
	Resolve(sh, ctx, main)

	if note := revealPickNotes(t, sh.log); note != nil {
		t.Fatalf("a zero-match reveal emitted ids %v, want none", note.IDs)
	}
	if len(ctx.Remembered) != 0 {
		t.Fatalf("Remembered = %+v, want nothing revealed", ctx.Remembered)
	}
	// The handler RAN: with the fix reverted this same walk emits a reveal
	// Note carrying a non-matching card. Assert no Note carries ids, the one
	// channel a reveal uses, so a regression cannot read as a clean skip.
	for _, e := range sh.log {
		if e.Kind == events.Note && len(e.IDs) > 0 {
			t.Fatalf("a zero-match reveal emitted Note ids %v, want none", e.IDs)
		}
	}
}

// cmcOf reads the live object's mana value the way the cmcGE2 predicate does.
func cmcOf(t *testing.T, h *fakeHost, id state.ObjID) int {
	t.Helper()
	o := h.g.Obj(id)
	if o == nil || o.Face() == nil {
		t.Fatalf("object %d has no face", id)
	}
	return int(parseCMC(o.Face().ManaCost))
}

// isInstantOrSorcery reports whether o is an instant or sorcery, the class the
// nonCreature branch must admit.
func isInstantOrSorcery(t *testing.T, o *state.Object) bool {
	t.Helper()
	return hasType(o, "Instant") || hasType(o, "Sorcery")
}
