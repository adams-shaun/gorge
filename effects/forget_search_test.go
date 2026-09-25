package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// Myr Incubator's library search replaces the source memory with found cards.
func TestChangeZoneSearchForgetOtherRememberedReplacesSet(t *testing.T) {
	h, src, staleCards := forgetFixtureHost(t, "Stale")
	stale := staleCards[0]
	found := h.g.AddObject(mkCard(t, "Name:Found Artifact\nTypes:Artifact\nOracle:x\n"), 0)
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{found.ID})
	found.Zone = state.ZLibrary
	h.g.SetZone(state.ZExile, 0, []state.ObjID{stale.ID})
	stale.Zone = state.ZExile
	seedRemembered(h, src, stale.ID)
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: stale.ID}}}
	body := sa(t, "DB$ ChangeZone | Origin$ Library | Destination$ Exile | ChangeType$ Artifact | ChangeNum$ 1 | RememberChanged$ True | ForgetOtherRemembered$ True")
	if found.ID == stale.ID || found.Zone != state.ZLibrary || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{stale.ID}) {
		t.Fatal("precondition: artifact library find and distinct stale remembered object")
	}
	h.askResult = true
	Resolve(h, c, body)
	if h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != found.ID {
		t.Fatalf("search did not offer artifact: %+v", h.lastAsk)
	}
	c.Search = []state.ObjID{found.ID}
	c.SearchDone = true
	Resolve(h, c, body)
	if found.Zone != state.ZExile {
		t.Errorf("found zone %s", found.Zone)
	}
	for label, got := range map[string][]state.ObjID{"ctx": rememberedIDs(c.Remembered), "persistent": rememberedIDs(h.g.Obj(src.ID).Remembered)} {
		if !sameIDs(got, []state.ObjID{found.ID}) {
			t.Errorf("%s remembered %v", label, got)
		}
	}
}
