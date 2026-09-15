package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// The handmove1 effects-level leaves for the "choose N cards matching
// ChangeType$ from Origin$ Hand" ChangeZone shape, using the askHost double
// (primitives_test.go) whose Ask captures the posed decision and suspends,
// and the embedded fakeHost whose Ask returns false -- the R-9 no-host
// stand-in. The rules-package halves (the real-engine end to end through
// Burgeoning's trigger) live in rules/hand_move_test.go.

// handAskFixture seats [bear, isle, isle, bear] in seat 0's HAND and returns
// (host, ids). SetZone moves the zone slice only; each object's own Zone
// field must be set to match (the same two-step every effects hand fixture
// uses).
func handAskFixture(t *testing.T) (*askHost, []state.ObjID) {
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
	h.g.SetZone(state.ZHand, 0, ids)
	for _, id := range ids {
		h.g.Obj(id).Zone = state.ZHand
	}
	return h, ids
}

// handNoHostFixture is handAskFixture over the bare fakeHost (Ask false --
// the R-9 no-host stand-in) rather than the capturing askHost.
func handNoHostFixture(t *testing.T) (*fakeHost, []state.ObjID) {
	t.Helper()
	h := &fakeHost{}
	h.g = state.NewGame(names(2))
	bear := mkCard(t, "Name:Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	land := mkCard(t, "Name:Isle\nTypes:Basic Land Island\nOracle:x\n")
	ids := []state.ObjID{
		h.g.AddObject(land, 0).ID,
		h.g.AddObject(land, 0).ID,
		h.g.AddObject(land, 0).ID,
		h.g.AddObject(bear, 0).ID,
	}
	h.g.SetZone(state.ZHand, 0, ids)
	for _, id := range ids {
		h.g.Obj(id).Zone = state.ZHand
	}
	return h, ids
}

const handAskSA = "DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | Mandatory$ True"

// TestHandMoveChangeZoneAsksWhenMoreEligibleThanChangeNum is the ask leaf: a
// hand with STRICTLY more ChangeType$-eligible cards than ChangeNum poses a
// real KChoose to the controller -- Min ChangeNum, Max ChangeNum, the
// eligible cards only, in hand order -- and the resolution suspends with
// nothing moved until the answer arrives.
func TestHandMoveChangeZoneAsksWhenMoreEligibleThanChangeNum(t *testing.T) {
	h, ids := handAskFixture(t)
	Resolve(h, &Ctx{Controller: 0}, sa(t, handAskSA))
	if h.asked == nil {
		t.Fatal("no decision was posed: a hand with more eligible cards than ChangeNum must ask")
	}
	d := h.asked
	if d.Kind != decision.KChoose || d.Player != 0 || d.Min != 1 || d.Max != 1 {
		t.Fatalf("decision = %+v, want a Min==1/Max==1 KChoose for the controller", d)
	}
	if d.ResumeKind != "hand_move" {
		t.Fatalf("ResumeKind = %q, want \"hand_move\"", d.ResumeKind)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "hand_move" ||
		d.Options[0].Obj != ids[1] || d.Options[1].Obj != ids[2] {
		t.Fatalf("options = %+v, want the two Isles in hand order, Kind \"hand_move\"", d.Options)
	}
	for _, o := range d.Options {
		if o.Label != "Isle" {
			t.Fatalf("option label = %q, want the face name", o.Label)
		}
	}
	// Nothing moved while suspended.
	for i, id := range ids {
		if o := h.g.Obj(id); o.Zone != state.ZHand {
			t.Fatalf("ids[%d] moved during suspension: %s", i, o.Zone)
		}
	}
}

// TestHandMoveChangeZoneReentryMovesExactlyTheAnswer is the re-entry
// contract: Ctx.HandMove/HandMoveDone set (here the SECOND eligible card, to
// prove the answer — not a first-eligible default — is honoured) moves
// exactly that card, the fx42 scoping discipline clears both fields, and a
// stray answer naming a card no longer in hand moves nothing.
func TestHandMoveChangeZoneReentryMovesExactlyTheAnswer(t *testing.T) {
	h, ids := handAskFixture(t)
	c := &Ctx{Controller: 0, HandMove: []state.ObjID{ids[2], ids[0]}, HandMoveDone: true}
	Resolve(h, c, sa(t, handAskSA))
	if o := h.g.Obj(ids[2]); o.Zone != state.ZBattlefield {
		t.Fatalf("the answered card is on %s, want battlefield", o.Zone)
	}
	if o := h.g.Obj(ids[0]); o.Zone != state.ZHand {
		t.Fatalf("the ineligible answer was honoured: bear is on %s", o.Zone)
	}
	if o := h.g.Obj(ids[1]); o.Zone != state.ZHand {
		t.Fatalf("the unchosen eligible card moved: on %s", o.Zone)
	}
	if c.HandMove != nil || c.HandMoveDone {
		t.Fatalf("Ctx.HandMove/HandMoveDone not cleared (fx42 scoping): %+v/%v", c.HandMove, c.HandMoveDone)
	}
	// A stray answer naming a card that left the hand moves nothing.
	h2, ids2 := handAskFixture(t)
	h2.g.Obj(ids2[2]).Zone = state.ZExile // pretend it left between ask and answer
	Resolve(h2, &Ctx{Controller: 0, HandMove: []state.ObjID{ids2[2]}, HandMoveDone: true}, sa(t, handAskSA))
	if o := h2.g.Obj(ids2[2]); o.Zone != state.ZExile {
		t.Fatalf("a card outside the hand was moved: on %s", o.Zone)
	}
}

// TestHandMoveChangeZoneNoChoiceMovesDeterministically is the no-choice leaf:
// with exactly ChangeNum eligible cards (and with fewer), the take is
// deterministic and no decision is posed.
func TestHandMoveChangeZoneNoChoiceMovesDeterministically(t *testing.T) {
	h, ids := handAskFixture(t)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ 2"))
	if h.asked != nil {
		t.Fatalf("a decision was posed with eligible == ChangeNum: %+v", h.asked)
	}
	if o := h.g.Obj(ids[1]); o.Zone != state.ZBattlefield {
		t.Fatalf("eligible card 1 on %s, want battlefield", o.Zone)
	}
	if o := h.g.Obj(ids[2]); o.Zone != state.ZBattlefield {
		t.Fatalf("eligible card 2 on %s, want battlefield", o.Zone)
	}
	for _, id := range []state.ObjID{ids[0], ids[3]} {
		if o := h.g.Obj(id); o.Zone != state.ZHand {
			t.Fatalf("ineligible bear moved: on %s", o.Zone)
		}
	}
}

// TestHandMoveChangeZoneZeroEligibleIsASilentNoOp is the zero-eligible leaf:
// nothing matching in hand resolves doing nothing -- no ask, no move, no
// event of any kind.
func TestHandMoveChangeZoneZeroEligibleIsASilentNoOp(t *testing.T) {
	h, ids := handAskFixture(t)
	h.g.SetZone(state.ZHand, 0, []state.ObjID{ids[0], ids[3]}) // bears only
	before := len(h.log)
	Resolve(h, &Ctx{Controller: 0}, sa(t, handAskSA))
	if h.asked != nil {
		t.Fatalf("a decision was posed with zero eligible cards: %+v", h.asked)
	}
	if len(h.log) != before {
		t.Fatalf("events emitted with zero eligible cards: %+v", h.log[before:])
	}
}

// TestHandMoveChangeZoneNoHostTakesFirstEligible is the R-9 fallback leaf: a
// host without a decision channel takes the first ChangeNum eligible cards in
// the decision's own deterministic option order, with the Note recording why.
func TestHandMoveChangeZoneNoHostTakesFirstEligible(t *testing.T) {
	h, ids := handNoHostFixture(t)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Exile | ChangeType$ Land | ChangeNum$ 1"))
	var note *events.Event
	for i := range h.log {
		if h.log[i].Kind == events.Note {
			note = &h.log[i]
		}
	}
	if note == nil || note.Player != 0 {
		t.Fatalf("no R-9 Note for the controller in %+v", h.log)
	}
	if o := h.g.Obj(ids[0]); o.Zone != state.ZExile {
		t.Fatalf("first eligible card on %s, want exile", o.Zone)
	}
	if o := h.g.Obj(ids[1]); o.Zone != state.ZHand {
		t.Fatalf("second eligible card moved: on %s", o.Zone)
	}
}

// TestHandMoveChangeZoneExileDoesNotParseUnusedCounterAmount guards an
// exact-Origin$ Hand picker that moves to exile with a dynamic
// WithCountersAmount$. Counters only apply on entry to the battlefield, so
// this path must neither parse X nor emit its malformed-amount Note.
func TestHandMoveChangeZoneExileDoesNotParseUnusedCounterAmount(t *testing.T) {
	h, ids := handAskFixture(t)
	Resolve(h, &Ctx{Controller: 0, HandMove: []state.ObjID{ids[1]}, HandMoveDone: true}, sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Exile | ChangeType$ Land | WithCountersType$ TIME | WithCountersAmount$ X"))
	if o := h.g.Obj(ids[1]); o.Zone != state.ZExile {
		t.Fatalf("answered land is on %s, want exile", o.Zone)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			t.Fatalf("unused exile counter amount emitted Note: %+v", ev)
		}
	}
}

// TestHandMoveChangeZoneHonoursRememberChanged pins the one settle extra
// the brief authorised: RememberChanged$ True joins the moved card to
// Ctx.Remembered in answer order. Tapped$ is deliberately NOT read on this
// path (unread on the object path too -- a recorded follow-up, not a
// handmove1 behaviour).
func TestHandMoveChangeZoneHonoursRememberChanged(t *testing.T) {
	h, ids := handAskFixture(t)
	c := &Ctx{Controller: 0, HandMove: []state.ObjID{ids[2]}, HandMoveDone: true}
	Resolve(h, c, sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | RememberChanged$ True"))
	o := h.g.Obj(ids[2])
	if o.Zone != state.ZBattlefield {
		t.Fatalf("moved land zone = %s, want battlefield", o.Zone)
	}
	if o.Tapped {
		t.Fatalf("moved land is tapped: Tapped$ must stay unread on the hand path")
	}
	for i := range h.log {
		if h.log[i].Kind == events.Tap && h.log[i].Obj == ids[2] {
			t.Fatalf("a Tap event was emitted on the hand path: %+v", h.log[i])
		}
	}
	if len(c.Remembered) != 1 || c.Remembered[0].Obj != ids[2] {
		t.Fatalf("Remembered = %+v, want exactly the moved card", c.Remembered)
	}
}

// TestHandMoveChangeZoneNonLiteralChangeNumEmitsNoteAndStaysSilent pins the
// rv2b routing change for a non-literal ChangeNum$ (an SVar name, an inline
// Count$, or a literal that cannot fit the count's range): it no longer
// falls to the pre-handmove1 silent no-op -- the routing emits a Note naming
// the unreadable count -- and it still never asks and never moves (the count
// expression itself is the scoped-out follow-up).
func TestHandMoveChangeZoneNonLiteralChangeNumEmitsNoteAndStaysSilent(t *testing.T) {
	for _, num := range []string{"NumInHand", "X", "Count$Valid Land.YouCtrl", "2147483648"} {
		h, ids := handAskFixture(t)
		before := len(h.log)
		Resolve(h, &Ctx{Controller: 0}, sa(t,
			"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ "+num))
		if h.asked != nil {
			t.Fatalf("ChangeNum$ %s posed a decision: %+v", num, h.asked)
		}
		for _, id := range ids {
			if o := h.g.Obj(id); o.Zone != state.ZHand {
				t.Fatalf("ChangeNum$ %s moved ids[%d] to %s", num, id, o.Zone)
			}
		}
		notes := 0
		for _, ev := range h.log[before:] {
			if ev.Kind == events.Note {
				notes++
			}
		}
		if notes != 1 {
			t.Fatalf("ChangeNum$ %s emitted %d Notes, want exactly 1: %+v", num, notes, h.log[before:])
		}
	}
}

// TestHandMoveChangeZoneRejectsOutOfRangeChangeNum proves a literal that
// cannot fit Decision.Min/Max's int32 count routes through the non-literal
// Note path: no ask, no move, no silent no-op.
func TestHandMoveChangeZoneRejectsOutOfRangeChangeNum(t *testing.T) {
	const overflow = "2147483648"
	if n, ok := handChangeNum(sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ "+overflow)); ok || n != 0 {
		t.Fatalf("handChangeNum(%s) = %d, %v; want 0, false", overflow, n, ok)
	}

	h, ids := handAskFixture(t)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ "+overflow))
	if h.asked != nil {
		t.Fatalf("out-of-range ChangeNum posed a decision: %+v", h.asked)
	}
	for _, id := range ids {
		if o := h.g.Obj(id); o.Zone != state.ZHand {
			t.Fatalf("out-of-range ChangeNum moved ids[%d] to %s", id, o.Zone)
		}
	}
	sawNote := false
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			sawNote = true
		}
	}
	if !sawNote {
		t.Fatalf("out-of-range ChangeNum emitted no Note: %+v", h.log)
	}
}

// TestHandMoveChangeZoneUntypedSpecDefaultsToWholeHand is the rv2b core
// leaf: with NO ChangeType$ the eligible pool is the whole hand --
// Brainstorm's "put two cards from your hand on top" must offer every card
// in hand, not silently no-op the way the pre-rv2b object path did.
func TestHandMoveChangeZoneUntypedSpecDefaultsToWholeHand(t *testing.T) {
	h, _ := handAskFixture(t)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Library | ChangeNum$ 2 | Mandatory$ True | Reorder$ True"))
	if h.asked == nil {
		t.Fatal("no decision was posed: the untyped put-back must offer the whole hand")
	}
	d := h.asked
	if d.Min != 2 || d.Max != 2 || len(d.Options) != 4 {
		t.Fatalf("decision = Min %d Max %d with %d options, want 2/2 and the whole hand", d.Min, d.Max, len(d.Options))
	}
	for _, o := range d.Options {
		if o.Label != "Bear" && o.Label != "Isle" {
			t.Fatalf("option label %q is not a hand card", o.Label)
		}
	}
}

// TestHandMoveChangeZonePutsBackOnTopInAnswerOrder is the placement leaf for
// the Forge default: Destination$ Library with no LibraryPosition$ puts the
// chosen cards on TOP of the library, in the player's answer order (one
// Secret LibraryOrder; Brainstorm's Reorder$ True "in any order").
func TestHandMoveChangeZonePutsBackOnTopInAnswerOrder(t *testing.T) {
	h, ids := handAskFixture(t)
	// The answer names the SECOND eligible card first: the order the answer
	// gives, not hand order, must be the order on top.
	Resolve(h, &Ctx{Controller: 0, HandMove: []state.ObjID{ids[2], ids[1]}, HandMoveDone: true},
		sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Library | ChangeNum$ 2 | Mandatory$ True | Reorder$ True"))
	lib := h.g.Zone(state.ZLibrary, 0)
	if len(lib) != 2 || lib[0] != ids[2] || lib[1] != ids[1] {
		t.Fatalf("library = %v, want [%d %d] in answer order on top", lib, ids[2], ids[1])
	}
	sawOrder := false
	for _, ev := range h.log {
		if ev.Kind == events.LibraryOrder && ev.Player == 0 {
			sawOrder = true
		}
	}
	if !sawOrder {
		t.Fatalf("no LibraryOrder placement event in %+v", h.log)
	}
	for _, id := range []state.ObjID{ids[0], ids[3]} {
		if o := h.g.Obj(id); o.Zone != state.ZHand {
			t.Fatalf("unchosen card %d moved to %s", id, o.Zone)
		}
	}
}

// TestHandMoveChangeZoneLibraryPositionZeroIsTop pins the explicit top
// spelling (Jace, the Mind Sculptor's [0]: LibraryPosition$ 0).
func TestHandMoveChangeZoneLibraryPositionZeroIsTop(t *testing.T) {
	h, ids := handAskFixture(t)
	Resolve(h, &Ctx{Controller: 0, HandMove: []state.ObjID{ids[1]}, HandMoveDone: true},
		sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Library | ChangeType$ Card | ChangeNum$ 2 | LibraryPosition$ 0 | Mandatory$ True"))
	lib := h.g.Zone(state.ZLibrary, 0)
	if len(lib) != 1 || lib[0] != ids[1] {
		t.Fatalf("library = %v, want the chosen card on top", lib)
	}
}

// TestHandMoveChangeZoneLibraryPositionMinusOneIsBottom pins the bottom
// spelling (Sawtooth Loon, Amass the Components: LibraryPosition$ -1); the
// Move itself appends at the bottom, so the placement re-affirms the chosen
// order there and the rest of the library stays above.
func TestHandMoveChangeZoneLibraryPositionMinusOneIsBottom(t *testing.T) {
	h, ids := handAskFixture(t)
	Resolve(h, &Ctx{Controller: 0, HandMove: []state.ObjID{ids[2], ids[1]}, HandMoveDone: true},
		sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Library | LibraryPosition$ -1 | ChangeNum$ 2 | Mandatory$ True"))
	lib := h.g.Zone(state.ZLibrary, 0)
	if len(lib) != 2 || lib[0] != ids[2] || lib[1] != ids[1] {
		t.Fatalf("library = %v, want [%d %d] at the bottom in answer order", lib, ids[2], ids[1])
	}
}

// TestHandMoveChangeZoneOptionalTakeAsksWithMinZero is the Mandatory$/
// Optional$ leaf: Optional$ You (the 35 raw explicit "may" lines) and the
// neither-parameter default (the 151 typed "you may put" lines, Burgeoning
// among them) lower the ask's Min to 0, and an empty answer moves nothing.
func TestHandMoveChangeZoneOptionalTakeAsksWithMinZero(t *testing.T) {
	for _, saLine := range []string{
		"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | Optional$ You",
		"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land",
	} {
		h, _ := handAskFixture(t)
		Resolve(h, &Ctx{Controller: 0}, sa(t, saLine))
		if h.asked == nil {
			t.Fatalf("%s: no decision posed", saLine)
		}
		if h.asked.Min != 0 || h.asked.Max != 1 {
			t.Fatalf("%s: Min/Max = %d/%d, want 0/1 (the optional take)", saLine, h.asked.Min, h.asked.Max)
		}
	}
	// The empty answer is legal: HandMoveDone with no ids moves nothing.
	h, ids := handAskFixture(t)
	Resolve(h, &Ctx{Controller: 0, HandMove: nil, HandMoveDone: true}, sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | Optional$ You"))
	for _, id := range ids {
		if o := h.g.Obj(id); o.Zone != state.ZHand {
			t.Fatalf("an empty optional answer moved ids[%d] to %s", id, o.Zone)
		}
	}
	// Mandatory$ True keeps the take required: Min ChangeNum.
	h2, _ := handAskFixture(t)
	Resolve(h2, &Ctx{Controller: 0}, sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | Optional$ You | Mandatory$ True"))
	if h2.asked == nil || h2.asked.Min != 1 {
		t.Fatalf("Mandatory$ True Min = %d, want 1 (the take is required)", h2.asked.Min)
	}
}

// TestHandMoveChangeZoneShuffleParamShufflesTheLibrary pins Shuffle$ True on
// a hand put-back (Slowtrip's "shuffle a card from your hand into your
// library"): the move happens, then one Shuffle randomises the library, and
// no LibraryPosition$ placement rides on top of it.
func TestHandMoveChangeZoneShuffleParamShufflesTheLibrary(t *testing.T) {
	h, ids := handAskFixture(t)
	Resolve(h, &Ctx{Controller: 0, HandMove: []state.ObjID{ids[1]}, HandMoveDone: true}, sa(t,
		"DB$ ChangeZone | Origin$ Hand | Destination$ Library | ChangeType$ Card | Shuffle$ True | RememberChanged$ True"))
	if o := h.g.Obj(ids[1]); o.Zone != state.ZLibrary {
		t.Fatalf("moved card on %s, want library", o.Zone)
	}
	sawShuffle, sawOrder := false, false
	for _, ev := range h.log {
		if ev.Kind == events.Shuffle && ev.Player == 0 {
			sawShuffle = true
		}
		if ev.Kind == events.LibraryOrder {
			sawOrder = true
		}
	}
	if !sawShuffle || sawOrder {
		t.Fatalf("Shuffle$ True: shuffle %v, placement %v (want shuffle, no placement)", sawShuffle, sawOrder)
	}
}

// --- rv2b: the two remaining brief leaves on the real corpus scripts. ---

// brainstormSubAbility returns the real compiled ChangeZoneDB sub-ability of
// the real corpus Brainstorm card (the Draw's SubAbility$ chain), so the
// whole-hand mandatory put-back is exercised on the script the card ships.
func brainstormSubAbility(t *testing.T, reg *cards.Registry) *cards.SA {
	t.Helper()
	brainstorm, ok := reg.Lookup("Brainstorm")
	if !ok {
		t.Fatal("corpus has no Brainstorm")
	}
	for _, ab := range brainstorm.Faces[0].Abilities {
		if ab.API == "Draw" && ab.Sub != nil && ab.Sub.API == "ChangeZone" {
			return ab.Sub
		}
	}
	t.Fatal("Brainstorm's Draw has no ChangeZone sub-ability in the compiled corpus")
	return nil
}

// TestBrainstormRealScriptMandatoryPutBackWithFewerEligibleCards runs the
// REAL compiled Brainstorm sub-ability (ChangeNum$ 2, Mandatory$ True, no
// ChangeType$) with only ONE eligible card in hand: the mandatory put-back
// takes it without an ask (a decision nobody could answer differently), and
// the card lands on top (Forge's absent-LibraryPosition$ default).
func TestBrainstormRealScriptMandatoryPutBackWithFewerEligibleCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	db := brainstormSubAbility(t, reg)
	if db.Params["ChangeNum"] != "2" || db.Params["Mandatory"] != "True" || db.Params["ChangeType"] != "" {
		t.Fatalf("Brainstorm's compiled sub-ability drifted: %+v", db.Params)
	}
	g := state.NewGame(names(2))
	bear := corpusObject(t, reg, g, "Grizzly Bears")
	g.SetZone(state.ZHand, 0, []state.ObjID{bear.ID})
	bear.Zone = state.ZHand

	h := &askHost{}
	h.g = g
	Resolve(h, &Ctx{Controller: 0}, db)
	if h.asked != nil {
		t.Fatalf("a decision was posed with eligible (1) < ChangeNum (2): %+v", h.asked)
	}
	lib := h.g.Zone(state.ZLibrary, 0)
	if len(lib) != 1 || lib[0] != bear.ID {
		t.Fatalf("library = %v, want exactly the bear on top", lib)
	}
	if o := h.g.Obj(bear.ID); o.Zone != state.ZLibrary {
		t.Fatalf("bear on %s, want library", o.Zone)
	}
}

// TestOviyaRealScriptFilteredHandPutBack runs the real compiled Oviya,
// Automech Artisan ability (Cost$ G T | Origin$ Hand | Destination$
// Battlefield | ChangeType$ Creature,Vehicle | ChangeNum$ 1, neither
// Mandatory$ nor Optional$ -- the Forge "you may" default): the ask offers
// ONLY the creature and Vehicle cards, never the land or the instant beside
// them, and Min is 0 (the take is optional).
func TestOviyaRealScriptFilteredHandPutBack(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	oviya, ok := reg.Lookup("Oviya, Automech Artisan")
	if !ok {
		t.Fatal("corpus has no Oviya, Automech Artisan")
	}
	var ab *cards.SA
	for _, a := range oviya.Faces[0].Abilities {
		if a.API == "ChangeZone" && a.Params["Origin"] == "Hand" {
			ab = a
		}
	}
	if ab == nil {
		t.Fatal("Oviya has no Origin$ Hand ChangeZone ability")
	}
	g := state.NewGame(names(2))
	bear := corpusObject(t, reg, g, "Grizzly Bears")
	mtn := corpusObject(t, reg, g, "Mountain")
	bolt := corpusObject(t, reg, g, "Lightning Bolt")
	copter := corpusObject(t, reg, g, "Smuggler's Copter")
	ids := []state.ObjID{bear.ID, mtn.ID, bolt.ID, copter.ID}
	g.SetZone(state.ZHand, 0, ids)
	for _, id := range ids {
		g.Obj(id).Zone = state.ZHand
	}

	h := &askHost{}
	h.g = g
	Resolve(h, &Ctx{Controller: 0}, ab)
	if h.asked == nil {
		t.Fatal("no decision posed: two eligible cards (bear, copter) strictly exceed ChangeNum 1")
	}
	d := h.asked
	if d.Min != 0 || d.Max != 1 {
		t.Fatalf("Min/Max = %d/%d, want 0/1 (Forge's optional default)", d.Min, d.Max)
	}
	if len(d.Options) != 2 {
		t.Fatalf("options = %+v, want exactly the bear and the Copter", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj != bear.ID && o.Obj != copter.ID {
			t.Fatalf("option %+v is neither the creature nor the Vehicle", o)
		}
	}
	// Nothing moved while suspended.
	for _, id := range ids {
		if g.Obj(id).Zone != state.ZHand {
			t.Fatalf("id %d moved during suspension", id)
		}
	}
}

// TestDreamCacheDestinationAlternativeEmitsNote pins the one genuinely
// unsupported hidden-origin shape's loudness: Dream Cache's "both on top of
// your library or both on the bottom" (DestinationAlternative$/
// LibraryPositionAlternative$) cannot be asked yet, so the alternative is
// named in a Note and the primary destination (top) is taken
// deterministically -- never a silent no-op.
func TestDreamCacheDestinationAlternativeEmitsNote(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Dream Cache")
	if !ok {
		t.Fatal("corpus has no Dream Cache")
	}
	var db *cards.SA
	for _, f := range card.Faces {
		for _, ab := range f.Abilities {
			if ab.API == "Draw" && ab.Sub != nil && ab.Sub.API == "ChangeZone" {
				db = ab.Sub
			}
		}
	}
	if db == nil || db.Params["DestinationAlternative"] == "" {
		t.Fatalf("Dream Cache's compiled sub-ability drifted: %+v", db)
	}
	g := state.NewGame(names(2))
	bear := corpusObject(t, reg, g, "Grizzly Bears")
	mtn := corpusObject(t, reg, g, "Mountain")
	g.SetZone(state.ZHand, 0, []state.ObjID{bear.ID, mtn.ID})
	bear.Zone, mtn.Zone = state.ZHand, state.ZHand

	h := &askHost{}
	h.g = g
	Resolve(h, &Ctx{Controller: 0}, db)
	if h.asked != nil {
		t.Fatalf("a decision was posed with eligible (2) == ChangeNum (2): %+v", h.asked)
	}
	lib := h.g.Zone(state.ZLibrary, 0)
	if len(lib) != 2 {
		t.Fatalf("library = %v, want both hand cards on top", lib)
	}
	sawNote := false
	for _, ev := range h.log {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "DestinationAlternative$") {
			sawNote = true
		}
	}
	if !sawNote {
		t.Fatalf("no DestinationAlternative$ Note in %+v", h.log)
	}
}
