package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 614.12: the replacement effects that apply to a permanent entering the
// battlefield are decided against its characteristics as it would exist
// there, its own static abilities included. A second Metallic Mimic naming
// the same type is that type as it enters (its own "is the chosen type"
// static), so the first Mimic's rider puts a counter on it.

// castMimicNaming casts the hand card id as a Metallic Mimic naming typ and
// finishes the resolution.
func castMimicNaming(t *testing.T, e *Engine, id state.ObjID, typ string) {
	t.Helper()
	e.G.Players[0].Pool[state.MC] += 2
	castMode(t, e, id, "")
	chooseETBType(t, e, typ)
	kr5FinishCast(t, e, id)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Mimic %d did not reach the battlefield", id)
	}
}

func TestSecondMetallicMimicSameTypeGetsCounter(t *testing.T) {
	t.Parallel()
	e := handEngine(t,
		corpusAlternativeCard(t, "Metallic Mimic"),
		corpusAlternativeCard(t, "Metallic Mimic"))
	hand := e.G.Zone(state.ZHand, 0)
	a, b := hand[0], hand[1]
	castMimicNaming(t, e, a, "Bear")
	castMimicNaming(t, e, b, "Bear")
	if got := e.G.Obj(a).Counter("P1P1"); got != 0 {
		t.Fatalf("first Mimic entered with %d P1P1, want 0", got)
	}
	if got := e.G.Obj(b).Counter("P1P1"); got != 1 {
		t.Fatalf("second Mimic entered with %d P1P1, want 1 (it is a Bear as it enters, CR 614.12)", got)
	}
}

func TestMetallicMimicDifferentTypeNoCounter(t *testing.T) {
	t.Parallel()
	e := handEngine(t,
		corpusAlternativeCard(t, "Metallic Mimic"),
		corpusAlternativeCard(t, "Metallic Mimic"))
	hand := e.G.Zone(state.ZHand, 0)
	a, b := hand[0], hand[1]
	castMimicNaming(t, e, a, "Bear")
	castMimicNaming(t, e, b, "Elf")
	if got := e.G.Obj(b).Counter("P1P1"); got != 0 {
		t.Fatalf("second Mimic naming Elf entered with %d P1P1, want 0", got)
	}
}

func TestMetallicMimicHardenedScalesSecondMimic(t *testing.T) {
	t.Parallel()
	e := handEngine(t,
		corpusAlternativeCard(t, "Hardened Scales"),
		corpusAlternativeCard(t, "Metallic Mimic"),
		corpusAlternativeCard(t, "Metallic Mimic"))
	hand := e.G.Zone(state.ZHand, 0)
	scales, a, b := hand[0], hand[1], hand[2]
	placeFromHand(t, e, scales)
	castMimicNaming(t, e, a, "Bear")
	castMimicNaming(t, e, b, "Bear")
	if got := e.G.Obj(b).Counter("P1P1"); got != 2 {
		t.Fatalf("second Mimic under Hardened Scales entered with %d P1P1, want 2 (one put of 1, plus 1)", got)
	}
}
