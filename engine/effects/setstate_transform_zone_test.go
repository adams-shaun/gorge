package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// transformingDFC is a minimal transforming double-faced card (CR 712): the
// AlternateMode marker is what makes events' FlipFace fold restamp the layer
// (foldFlipFace's IsTransformingDFC check).
func transformingDFC(t *testing.T) *cards.Card {
	t.Helper()
	return mkCard(t, "Name:Front\nTypes:Creature\nPT:1/1\nAlternateMode:DoubleFaced\nOracle:x\n\n"+
		"ALTERNATE\n\nName:Back\nTypes:Creature\nPT:3/3\nOracle:x\n")
}

// TestTransformInGraveyardDoesNothing pins CR 701.27a: only a permanent can
// be transformed. A transforming DFC that died before its transform effect
// resolved is a card in the graveyard, not a permanent, so the mode must be
// a no-op -- no face change and no FlipFace event.
func TestTransformInGraveyardDoesNothing(t *testing.T) {
	h := newHost(t, 2)
	o := h.g.AddObject(transformingDFC(t), 0)
	o.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), o.ID))
	moveTo(h.g, o.ID, state.ZGraveyard)

	o = h.g.Obj(o.ID)
	if o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: object zone = %v, want Graveyard", o.Zone)
	}
	if !o.Card.IsTransformingDFC() || len(o.Card.Faces) < 2 {
		t.Fatalf("precondition: card is not a two-faced transforming DFC")
	}
	if o.FaceIdx != 0 {
		t.Fatalf("precondition: FaceIdx = %d, want the front face 0", o.FaceIdx)
	}

	Resolve(h, &Ctx{Source: o.ID, Controller: 0, Targets: []state.Target{{Obj: o.ID}}},
		sa(t, "DB$ SetState | Defined$ Self | Mode$ Transform"))

	if after := h.g.Obj(o.ID); after.FaceIdx != 0 {
		t.Fatalf("graveyard card flipped to FaceIdx %d, want it left on 0", after.FaceIdx)
	}
	if n := len(flipFaceEvents(h)); n != 0 {
		t.Fatalf("FlipFace events = %d, want none for an off-battlefield Transform (%+v)", n, h.log)
	}
	handlerRan(t, h)
}

// TestIncubatorTransformAfterRemovalDoesNothing pins the reported shape: an
// Incubator token's {2}: Transform is activated, the token is removed in
// response, and the ability resolves with the token in the graveyard. The
// token is no longer a permanent, so nothing flips.
func TestIncubatorTransformAfterRemovalDoesNothing(t *testing.T) {
	h, c := incubatorHost(t)
	Resolve(h, c, sa(t, "DB$ Incubate | Amount$ 2"))
	toks := battlefieldTokens(h)
	if len(toks) != 1 {
		t.Fatalf("precondition: %d Incubator tokens minted, want 1", len(toks))
	}
	tok := toks[0]
	if len(tok.Card.Faces) != 2 || tok.FaceIdx != 0 {
		t.Fatalf("precondition: token has %d faces at FaceIdx %d, want 2 at 0", len(tok.Card.Faces), tok.FaceIdx)
	}
	moveTo(h.g, tok.ID, state.ZGraveyard)
	tok = h.g.Obj(tok.ID)
	if tok == nil || tok.Zone != state.ZGraveyard {
		t.Fatalf("precondition: token zone = %v, want Graveyard", tok.Zone)
	}

	Resolve(h, &Ctx{Source: tok.ID, Controller: 0, Targets: []state.Target{{Obj: tok.ID}}},
		sa(t, "DB$ SetState | Defined$ Self | Mode$ Transform"))

	after := h.g.Obj(tok.ID)
	if after.FaceIdx != 0 {
		t.Fatalf("removed Incubator flipped to FaceIdx %d, want it left on 0", after.FaceIdx)
	}
	if after.Face() == nil || after.Face().Name != "Incubator Token" {
		t.Fatalf("removed Incubator face = %+v, want the front Incubator Token face", after.Face())
	}
	if n := len(flipFaceEvents(h)); n != 0 {
		t.Fatalf("FlipFace events = %d, want none for a removed Incubator (%+v)", n, h.log)
	}
	handlerRan(t, h)
}

// TestTransformOnBattlefieldStillFlips is the positive control: a
// transforming DFC still on the battlefield transforms to its back face, with
// the CR 613.7g layer restamp the fold gives a transforming DFC.
func TestTransformOnBattlefieldStillFlips(t *testing.T) {
	h := newHost(t, 2)
	o := h.g.AddObject(transformingDFC(t), 0)
	o.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, append(h.g.Zone(state.ZBattlefield, 0), o.ID))

	if !o.Card.IsTransformingDFC() || o.FaceIdx != 0 {
		t.Fatalf("precondition: card is not a front-face transforming DFC")
	}
	before := h.g.Clock

	Resolve(h, &Ctx{Source: o.ID, Controller: 0, Targets: []state.Target{{Obj: o.ID}}},
		sa(t, "DB$ SetState | Defined$ Self | Mode$ Transform"))

	after := h.g.Obj(o.ID)
	if after.FaceIdx != 1 {
		t.Fatalf("battlefield DFC FaceIdx = %d, want 1 (the back face)", after.FaceIdx)
	}
	if after.Face() == nil || after.Face().Name != "Back" {
		t.Fatalf("battlefield DFC face = %+v, want the Back face", after.Face())
	}
	flips := flipFaceEvents(h)
	if len(flips) != 1 {
		t.Fatalf("FlipFace events = %d, want exactly 1 (%+v)", len(flips), h.log)
	}
	if flips[0].Obj != o.ID || flips[0].Amount != 1 || flips[0].Text != "Transformed" {
		t.Fatalf("FlipFace = {obj %d amount %d text %q}, want {obj %d amount 1 text Transformed}",
			flips[0].Obj, flips[0].Amount, flips[0].Text, o.ID)
	}
	if after.LayerStamp <= before {
		t.Fatalf("LayerStamp = %d, want a restamp later than the pre-resolution clock %d", after.LayerStamp, before)
	}
	handlerRan(t, h)
}
