package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestLibrarySearchMandatoryQualityRequiresFinding(t *testing.T) {
	reg := hostlessSearchTestRegistry(t)
	tutor, ok := reg.Lookup("Demonic Tutor")
	if !ok {
		t.Fatal("missing corpus Demonic Tutor")
	}
	forest, ok := reg.Lookup("Forest")
	if !ok {
		t.Fatal("missing corpus Forest")
	}

	for _, mandatory := range []bool{false, true} {
		base := newHost(t, 2)
		h := &suspendHost{}
		h.g = base.g
		src := h.g.AddObject(tutor, 0)
		card := h.g.AddObject(forest, 0)
		h.g.SetZone(state.ZHand, 0, []state.ObjID{src.ID})
		h.g.SetZone(state.ZLibrary, 0, []state.ObjID{card.ID})
		h.g.Obj(src.ID).Zone = state.ZHand
		h.g.Obj(card.ID).Zone = state.ZLibrary
		ability := sa(t, "DB$ ChangeZone | Origin$ Library | Destination$ Hand | ChangeType$ Land | ChangeNum$ 1")
		if mandatory {
			ability.Params["Mandatory"] = "True"
		}

		Resolve(h, &Ctx{Source: src.ID, Controller: 0, SVars: tutor.Faces[0].SVars}, ability)
		if h.asked == nil || h.asked.Kind != decision.KChoose || len(h.asked.Options) != 1 {
			t.Fatalf("mandatory=%v decision=%+v, want one eligible quality option", mandatory, h.asked)
		}
		if h.asked.Options[0].Obj != card.ID || h.g.Obj(card.ID).Zone != state.ZLibrary {
			t.Fatalf("mandatory=%v candidate precondition/zone wrong: decision=%+v zone=%s", mandatory, h.asked, h.g.Obj(card.ID).Zone)
		}
		wantMin := 0
		if mandatory {
			wantMin = 1
		}
		if h.asked.Min != wantMin {
			t.Fatalf("mandatory=%v Min=%d, want %d (quality filter=%v)", mandatory, h.asked.Min, wantMin, SearchStatesQuality("Land"))
		}
	}
}
