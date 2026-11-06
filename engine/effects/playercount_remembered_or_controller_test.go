package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPlayerCountRememberedOrController pins the
// HasPropertyIsRememberedOrController player property (Cut a Deal's follow-up
// draw amount, "you draw a card for each opponent who drew a card this way"):
// a group member counts exactly when it is the resolving ability's
// controller, a remembered PLAYER entry, or the live controller of a
// remembered OBJECT entry. An empty remembered set is a modelled count (0 on
// an opponent group, 1 on Players$ from the OrController half), never the
// unresolved (0, false) that made NumCards read zero.
func TestPlayerCountRememberedOrController(t *testing.T) {
	h, c := fixtureHost(t)
	card := mkCard(t, "Name:Drawn\nTypes:Creature\nPT:1/1\nOracle:x\n")
	seat1Obj := h.g.AddObject(card, 1)
	seat0Obj := h.g.AddObject(card, 0)
	// PRECONDITION: the remembered-set halves are distinguishable -- the two
	// objects really sit under different controllers.
	if h.g.Obj(seat1Obj.ID).Controller != 1 || h.g.Obj(seat0Obj.ID).Controller != 0 {
		t.Fatalf("fixture controllers = %d,%d, want 1,0",
			h.g.Obj(seat1Obj.ID).Controller, h.g.Obj(seat0Obj.ID).Controller)
	}
	opp := "Count$PlayerCountOpponents$HasPropertyIsRememberedOrController"
	players := "Count$PlayerCountPlayers$HasPropertyIsRememberedOrController"

	// Nothing remembered: the OrController half alone. Seat 0 is never in
	// the Opponents group, so the two groups must differ.
	if got, ok := EvalCountOK(h, c, opp); !ok || got != 0 {
		t.Fatalf("empty set, opponents = (%d, %v), want (0, true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, players); !ok || got != 1 {
		t.Fatalf("empty set, players = (%d, %v), want (1, true)", got, ok)
	}

	// One remembered seat-1 object: the IsRemembered half adds seat 1 to both
	// groups.
	c.Remembered = []state.Target{{Obj: seat1Obj.ID}}
	if got, ok := EvalCountOK(h, c, opp); !ok || got != 1 {
		t.Fatalf("one seat-1 object, opponents = (%d, %v), want (1, true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, players); !ok || got != 2 {
		t.Fatalf("one seat-1 object, players = (%d, %v), want (2, true)", got, ok)
	}

	// A second object under the SAME controller: distinct players, not
	// per-object counting.
	c.Remembered = []state.Target{{Obj: seat1Obj.ID}, {Obj: secondSeat1Obj(t, h)}}
	if got, ok := EvalCountOK(h, c, opp); !ok || got != 1 {
		t.Fatalf("two seat-1 objects, opponents = (%d, %v), want (1, true)", got, ok)
	}

	// A remembered seat-0 object only: seat 0 is the controller (Players$
	// stays 1) and never an opponent.
	c.Remembered = []state.Target{{Obj: seat0Obj.ID}}
	if got, ok := EvalCountOK(h, c, opp); !ok || got != 0 {
		t.Fatalf("one seat-0 object, opponents = (%d, %v), want (0, true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, players); !ok || got != 1 {
		t.Fatalf("one seat-0 object, players = (%d, %v), want (1, true)", got, ok)
	}

	// Both controllers remembered: 1 opponent, 2 players.
	c.Remembered = []state.Target{{Obj: seat1Obj.ID}, {Obj: seat0Obj.ID}}
	if got, ok := EvalCountOK(h, c, opp); !ok || got != 1 {
		t.Fatalf("both objects, opponents = (%d, %v), want (1, true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, players); !ok || got != 2 {
		t.Fatalf("both objects, players = (%d, %v), want (2, true)", got, ok)
	}

	// A remembered PLAYER entry counts its seat directly (the IsRemembered
	// half controllersOf passes through); an unresolvable object id
	// contributes nothing on either group.
	c.Remembered = []state.Target{{Player: 1, IsPlayer: true}}
	if got, ok := EvalCountOK(h, c, opp); !ok || got != 1 {
		t.Fatalf("remembered player 1, opponents = (%d, %v), want (1, true)", got, ok)
	}
	c.Remembered = []state.Target{{Obj: 9999}}
	if got, ok := EvalCountOK(h, c, opp); !ok || got != 0 {
		t.Fatalf("unresolvable id, opponents = (%d, %v), want (0, true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, players); !ok || got != 1 {
		t.Fatalf("unresolvable id, players = (%d, %v), want (1, true)", got, ok)
	}
}

// secondSeat1Obj adds another seat-1-controlled object.
func secondSeat1Obj(t *testing.T, h *fakeHost) state.ObjID {
	t.Helper()
	o := h.g.AddObject(mkCard(t, "Name:Drawn\nTypes:Creature\nPT:1/1\nOracle:x\n"), 1)
	return o.ID
}

// TestCutADealCountHeadSupported is the corpus probe: Cut a Deal is the only
// corpus author of Count$PlayerCountOpponents$HasPropertyIsRememberedOrController,
// so with the evaluator arm landed its face must carry the head AND the card
// must clear effects.Supported() with no registration gap.
func TestCutADealCountHeadSupported(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Cut a Deal")
	if !ok {
		t.Fatal("corpus has no Cut a Deal")
	}
	// PRECONDITION: the face really carries the head this ticket models.
	const head = "count:PlayerCountOpponents$HasPropertyIsRememberedOrController"
	found := false
	for _, vh := range card.ValueHeads() {
		if vh == head {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("Cut a Deal ValueHeads() = %v, want %q among them", card.ValueHeads(), head)
	}
	if gaps := reg.Unsupported(card, Supported()); len(gaps) != 0 {
		t.Fatalf("Cut a Deal unsupported = %v, want none", gaps)
	}
}
