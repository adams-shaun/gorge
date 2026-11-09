package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSetStateTurnFaceUpInExileEmitsExileTurnFaceUp pins the exile branch of
// Mode$ TurnFaceUp: every face-down exiled card (an instant included -- the
// battlefield CR 701.40g gate does not apply) gets an ExileTurnFaceUp event,
// a face-up one gets none, and no battlefield TurnFaceUp is emitted.
func TestSetStateTurnFaceUpInExileEmitsExileTurnFaceUp(t *testing.T) {
	h := newHost(t, 2)
	instant := mkCard(t, "Name:Bolt\nManaCost:R\nTypes:Instant\nOracle:x\n")
	creature := mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	mk := func(name string, faceDown bool) *state.Object {
		card := creature
		if name == "instant" {
			card = instant
		}
		o := h.g.AddObject(card, 0)
		h.g.SetZone(state.ZExile, 0, append(h.g.Zone(state.ZExile, 0), o.ID))
		o.Zone = state.ZExile
		o.FaceDown = faceDown
		return o
	}
	inst, crea, up := mk("instant", true), mk("creature", true), mk("creature", false)
	for _, o := range []*state.Object{h.g.Obj(inst.ID), h.g.Obj(crea.ID)} {
		if o.Zone != state.ZExile || !o.FaceDown {
			t.Fatalf("precondition: %d zone=%v faceDown=%v", o.ID, o.Zone, o.FaceDown)
		}
	}
	if h.g.Obj(up.ID).FaceDown {
		t.Fatalf("precondition: the control card is face down")
	}
	c := &Ctx{Source: inst.ID, Controller: 0, Remembered: []state.Target{{Obj: inst.ID}, {Obj: crea.ID}, {Obj: up.ID}}}
	Resolve(h, c, sa(t, "DB$ SetState | Defined$ Remembered | Mode$ TurnFaceUp"))

	var got []state.ObjID
	for _, ev := range h.log {
		switch ev.Kind {
		case events.ExileTurnFaceUp:
			got = append(got, ev.Obj)
		case events.TurnFaceUp:
			t.Fatalf("battlefield TurnFaceUp emitted for an exiled card: %+v", ev)
		case events.Note:
			t.Fatalf("unexpected Note %q", ev.Text)
		}
	}
	if len(got) != 2 || got[0] != inst.ID || got[1] != crea.ID {
		t.Fatalf("ExileTurnFaceUp objs = %v, want [%d %d]", got, inst.ID, crea.ID)
	}
	if h.g.Obj(inst.ID).FaceDown || h.g.Obj(crea.ID).FaceDown {
		t.Fatalf("cards still face down after the turn-up")
	}
	handlerRan(t, h)
}
