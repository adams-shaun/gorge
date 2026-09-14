package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The dig1 look-and-take tests. The pinned primitives_test.go Dig tests pin
// the NO-CHOICE and no-host behaviour; these pin the ask itself, the look
// Note, the Optional$ Min, the re-entry contract and the strict-supersets
// gate, using the askHost double (primitives_test.go) whose Ask captures the
// posed decision and suspends -- the effects-package stand-in for
// rules.Engine.

// digAskFixture builds seat 0's library as [c0 bear, c1 land, c2 land, c3
// bear] and returns (host, ids).
func digAskFixture(t *testing.T) (*askHost, []state.ObjID) {
	t.Helper()
	h := &askHost{}
	h.g = state.NewGame(names(2))
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	ids := []state.ObjID{
		h.g.AddObject(bear, 0).ID,
		h.g.AddObject(land, 0).ID,
		h.g.AddObject(land, 0).ID,
		h.g.AddObject(bear, 0).ID,
	}
	h.g.SetZone(state.ZLibrary, 0, ids)
	return h, ids
}

const digAskSA = "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 1 | Optional$ True | ChangeValid$ Land | DestinationZone$ Hand"

// TestDigAsksWhenTheWindowHoldsMoreEligibleCardsThanChangeNum is the dig1
// core leaf: a window with STRICTLY more ChangeValid$-eligible cards than
// ChangeNum poses a real KChoose to the library's owner -- the look recorded
// first as a Secret Note carrying the window, the options the ELIGIBLE cards
// only, Min 0 (Optional$ honoured) and Max ChangeNum -- and the resolution
// suspends with nothing moved until the answer arrives.
func TestDigAsksWhenTheWindowHoldsMoreEligibleCardsThanChangeNum(t *testing.T) {
	h, ids := digAskFixture(t)
	Resolve(h, &Ctx{Controller: 0}, sa(t, digAskSA))
	if h.asked == nil {
		t.Fatal("no decision was posed: a window with more eligible cards than ChangeNum must ask")
	}
	d := h.asked
	if d.Kind != decision.KChoose || d.Player != 0 || d.Min != 0 || d.Max != 1 {
		t.Fatalf("decision = %+v, want a Min==0/Max==1 KChoose for the library's owner", d)
	}
	if d.ResumeKind != "dig" {
		t.Fatalf("ResumeKind = %q, want \"dig\"", d.ResumeKind)
	}
	if !strings.Contains(d.Prompt, "Look at the top 3") {
		t.Fatalf("prompt = %q, want it to name the look in the card's own terms", d.Prompt)
	}
	// The options are the ELIGIBLE cards only, in library order: the bear
	// must not be pickable for a ChangeValid$ Land take.
	if len(d.Options) != 2 || d.Options[0].Kind != "dig" || d.Options[0].Obj != ids[1] || d.Options[1].Obj != ids[2] {
		t.Fatalf("options = %+v, want the two lands in library order, Kind \"dig\"", d.Options)
	}
	// The look is recorded BEFORE the ask, as a Secret Note to the owner
	// carrying the whole window, and nothing has moved yet.
	var look *events.Event
	for i := range h.log {
		e := &h.log[i]
		if e.Kind == events.Note && e.Text == "looks at the top of the library" {
			look = e
			break
		}
	}
	if look == nil {
		t.Fatalf("no look Note in %+v", h.log)
	}
	if !look.Secret || look.Player != 0 {
		t.Fatalf("look Note = %+v, want Secret to its owner (seat 0)", look)
	}
	if len(look.IDs) != 3 || look.IDs[0] != ids[0] || look.IDs[1] != ids[1] || look.IDs[2] != ids[2] {
		t.Fatalf("look Note IDs = %v, want the full 3-card window %v", look.IDs, ids[:3])
	}
	if len(h.g.Zone(state.ZHand, 0)) != 0 {
		t.Fatal("cards moved before the answer: the ask must suspend the resolution")
	}
	// Re-entry, the engine's contract: Ctx.Dig/DigDone carry the chosen
	// objects (here the SECOND land, to prove the choice is honoured, not a
	// first-eligible default), the move is Secret to the owner, and the
	// passed Ctx is consumed and cleared (the fx42 scoping discipline).
	ctx := &Ctx{Controller: 0, Dig: []state.ObjID{ids[2]}, DigDone: true}
	Resolve(h, ctx, sa(t, digAskSA))
	if ctx.Dig != nil || ctx.DigDone {
		t.Fatal("re-entry left Ctx.Dig/DigDone set: the answer field must be consumed and cleared")
	}
	if hand := h.g.Zone(state.ZHand, 0); len(hand) != 1 || hand[0] != ids[2] {
		t.Fatalf("hand = %v, want [%d] (the SECOND land, the one the player picked)", hand, ids[2])
	}
	if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != 3 || lib[0] != ids[0] || lib[1] != ids[1] || lib[2] != ids[3] {
		t.Fatalf("library = %v, want [%d %d %d], the rest on top in their existing order", lib, ids[0], ids[1], ids[3])
	}
	found := false
	for _, e := range h.log {
		if e.Kind == events.MoveZone && e.Obj == ids[2] && e.Secret && e.Player == 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("the answered take's MoveZone is not a Secret move naming the library's owner")
	}
}

// TestDigOptionalDeclineMovesNothing is the Optional$ leaf on the answer
// side: an answered ask carrying NO cards (Min 0 lets the player decline the
// take) moves nothing and leaves the whole window in place.
func TestDigOptionalDeclineMovesNothing(t *testing.T) {
	h, ids := digAskFixture(t)
	Resolve(h, &Ctx{Controller: 0}, sa(t, digAskSA))
	if h.asked == nil {
		t.Fatal("no decision was posed")
	}
	Resolve(h, &Ctx{Controller: 0, DigDone: true}, sa(t, digAskSA))
	if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != 4 || lib[0] != ids[0] || lib[3] != ids[3] {
		t.Fatalf("library = %v, want the decline to move nothing", lib)
	}
	if len(h.g.Zone(state.ZHand, 0)) != 0 {
		t.Fatal("a declined Optional take moved a card into hand")
	}
}

// TestDigMandatoryAskMinsAtChangeNum is the Optional$-absent leaf: without
// Optional$ the take is mandatory, so the ask's Min is ChangeNum (the
// answer must take that many) rather than 0.
func TestDigMandatoryAskMinsAtChangeNum(t *testing.T) {
	h, _ := digAskFixture(t)
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 1 | ChangeValid$ Land | DestinationZone$ Hand"))
	if h.asked == nil {
		t.Fatal("no decision was posed")
	}
	if h.asked.Min != 1 || h.asked.Max != 1 {
		t.Fatalf("Min/Max = %d/%d, want 1/1 for a mandatory take of 1", h.asked.Min, h.asked.Max)
	}
}

// TestDigStaysSilentWhenTheWindowHoldsNoChoice pins the strict-supersets
// gate: a window whose eligible count EQUALS ChangeNum admits no real pick,
// so no decision is posed, no look Note is emitted, and the M1 silent
// behaviour (first ChangeNum eligible in zone order move) runs unchanged --
// byte-identical replay for games that never reach a strict-superset Dig.
func TestDigStaysSilentWhenTheWindowHoldsNoChoice(t *testing.T) {
	h, ids := digAskFixture(t)
	Resolve(h, &Ctx{Controller: 0},
		sa(t, "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 2 | Optional$ True | ChangeValid$ Land | DestinationZone$ Hand"))
	if h.asked != nil {
		t.Fatalf("a decision was posed for a no-choice window: %+v", h.asked)
	}
	for _, e := range h.log {
		if e.Kind == events.Note && e.Text == "looks at the top of the library" {
			t.Fatal("a look Note was emitted on the no-choice path: games without the decision must replay byte-identically")
		}
	}
	if hand := h.g.Zone(state.ZHand, 0); len(hand) != 2 || hand[0] != ids[1] || hand[1] != ids[2] {
		t.Fatalf("hand = %v, want [%d %d] (both lands, the silent M1 behaviour)", hand, ids[1], ids[2])
	}
}

// TestDigNoHostFallbackKeepsTheFirstEligibleWithANote pins the R-9 fallback:
// a host that cannot answer keeps today's behaviour -- the first ChangeNum
// eligible cards in zone order -- with the Note that records why the richer
// path did not run (after the look Note the ask itself emitted).
func TestDigNoHostFallbackKeepsTheFirstEligibleWithANote(t *testing.T) {
	h := newHost(t, 2)
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	ids := []state.ObjID{
		h.g.AddObject(land, 0).ID,
		h.g.AddObject(bear, 0).ID,
		h.g.AddObject(land, 0).ID,
	}
	h.g.SetZone(state.ZLibrary, 0, ids)
	Resolve(h, &Ctx{Controller: 0}, sa(t, digAskSA))
	if hand := h.g.Zone(state.ZHand, 0); len(hand) != 1 || hand[0] != ids[0] {
		t.Fatalf("hand = %v, want [%d] (the FIRST eligible card, the deterministic stand-in)", hand, ids[0])
	}
	if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != 2 || lib[0] != ids[1] || lib[1] != ids[2] {
		t.Fatalf("library = %v, want [%d %d]", lib, ids[1], ids[2])
	}
	found := false
	for _, e := range h.log {
		if e.Kind == events.Note && strings.Contains(e.Text, "no engine host to ask") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no fallback Note in %+v", h.log)
	}
}

// TestDigReentryIgnoresIdsOutsideTheWindow keeps a stray or stale answer
// from moving an object that is not (or no longer) in the target's look
// window -- the per-window filter, the mirror of effDiscard's per-hand
// filter on its own re-entry.
func TestDigReentryIgnoresIdsOutsideTheWindow(t *testing.T) {
	h, ids := digAskFixture(t)
	Resolve(h, &Ctx{Controller: 0}, sa(t, digAskSA))
	if h.asked == nil {
		t.Fatal("no decision was posed")
	}
	// The answer names the bear at index 3 (outside DigNum$ 3's window) and
	// a bogus id; neither may move. The in-window land at index 1 does.
	Resolve(h, &Ctx{Controller: 0, Dig: []state.ObjID{ids[3], 999, ids[1]}, DigDone: true},
		sa(t, digAskSA))
	if hand := h.g.Zone(state.ZHand, 0); len(hand) != 1 || hand[0] != ids[1] {
		t.Fatalf("hand = %v, want [%d] (only the in-window pick moved)", hand, ids[1])
	}
}

// TestDigLookNoteIsSecretToItsOwner is the multi-seat leaf: a Dig resolved
// against seat 1's library records its look (when it asks) and its take as
// Secret events naming seat 1, never seat 0.
func TestDigLookNoteIsSecretToItsOwner(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	ids := []state.ObjID{
		h.g.AddObject(land, 1).ID,
		h.g.AddObject(bear, 1).ID,
		h.g.AddObject(land, 1).ID,
	}
	h.g.SetZone(state.ZLibrary, 1, ids)
	Resolve(h, &Ctx{Controller: 1},
		sa(t, "SP$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 1 | Optional$ True | ChangeValid$ Land | DestinationZone$ Hand"))
	if h.asked == nil || h.asked.Player != 1 {
		t.Fatalf("decision = %+v, want one posed to seat 1, the library's owner", h.asked)
	}
	for _, e := range h.log {
		switch e.Kind {
		case events.Note:
			if !e.Secret {
				continue
			}
			if e.Player != 1 {
				t.Fatalf("Secret Note has Player %d, want the library's owner (1): %+v", e.Player, e)
			}
		case events.MoveZone:
			if !e.Secret {
				t.Fatalf("Dig MoveZone not Secret: %+v", e)
			}
			if e.Player != 1 {
				t.Fatalf("Secret MoveZone has Player %d, want the library's owner (1): %+v", e.Player, e)
			}
		}
	}
}
