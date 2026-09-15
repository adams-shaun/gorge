package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
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

const handAskSA = "DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land"

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

// TestHandMoveChangeZoneNonLiteralChangeNumStaysOnObjectPath pins the R2
// scope restriction: a non-literal ChangeNum$ (an SVar name or inline
// Count$) does NOT reach the hand path -- it stays on the pre-existing
// object path, which for a selectorless Origin$ Hand shape moves nothing
// (the same silent no-op the fix found before handmove1).
func TestHandMoveChangeZoneNonLiteralChangeNumStaysOnObjectPath(t *testing.T) {
	for _, num := range []string{"NumInHand", "X", "Count$Valid Land.YouCtrl"} {
		h, ids := handAskFixture(t)
		Resolve(h, &Ctx{Controller: 0}, sa(t,
			"DB$ ChangeZone | Origin$ Hand | Destination$ Battlefield | ChangeType$ Land | ChangeNum$ "+num))
		if h.asked != nil {
			t.Fatalf("ChangeNum$ %s posed a decision: %+v", num, h.asked)
		}
		for _, id := range ids {
			if o := h.g.Obj(id); o.Zone != state.ZHand {
				t.Fatalf("ChangeNum$ %s moved ids[%d] to %s: must stay on the object path", num, id, o.Zone)
			}
		}
	}
}

// TestHandMoveChangeZoneRejectsOutOfRangeChangeNum proves a literal that
// cannot fit Decision.Min/Max's int32 count stays on the old object path.
// In particular, it must not narrow to a negative bound and create a
// permanently unanswerable KChoose decision.
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
			t.Fatalf("out-of-range ChangeNum moved ids[%d] to %s: must stay on the object path", id, o.Zone)
		}
	}
}
