package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// Both owners must be offered the remembered cards that were eligible when
// the ChangeZone began, even after the first move clears persistent memory.
func TestChangeZoneHiddenForgetOtherRememberedMultiOwner(t *testing.T) {
	h, src, cards := forgetFixtureHost(t, "First", "Stale")
	first, stale := cards[0], cards[1]
	second := h.g.AddObject(mkCard(t, "Name:Second\nTypes:Creature\nPT:3/3\nOracle:x\n"), 1)
	first, stale, second = h.g.Obj(first.ID), h.g.Obj(stale.ID), h.g.Obj(second.ID)
	h.g.SetZone(state.ZGraveyard, 0, []state.ObjID{first.ID})
	h.g.SetZone(state.ZGraveyard, 1, []state.ObjID{second.ID})
	first.Zone, second.Zone = state.ZGraveyard, state.ZGraveyard
	h.g.SetZone(state.ZExile, 0, []state.ObjID{stale.ID})
	stale.Zone = state.ZExile
	seedRemembered(h, src, first.ID, second.ID, stale.ID)
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: first.ID}, {Obj: second.ID}, {Obj: stale.ID}}}
	body := sa(t, "DB$ ChangeZone | Hidden$ True | Origin$ Graveyard | Destination$ Hand | DefinedPlayer$ RememberedOwner | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Mandatory$ True | RememberChanged$ True | ForgetOtherRemembered$ True")
	if first.ID == second.ID || first.Zone != state.ZGraveyard || second.Zone != state.ZGraveyard || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{first.ID, second.ID, stale.ID}) {
		t.Fatal("precondition: distinct remembered creatures in two owners' graveyards")
	}
	h.askResult = true
	Resolve(h, c, body)
	if h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != first.ID {
		t.Fatalf("first owner not offered its remembered card: %+v", h.lastAsk)
	}
	c.HiddenPick, c.HiddenPickDone = []state.ObjID{first.ID}, true
	Resolve(h, c, body)
	if first.Zone != state.ZHand || h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != second.ID {
		t.Fatalf("second owner's remembered choice lost after first move: first=%s ask=%+v", first.Zone, h.lastAsk)
	}
	// A real host rebuilds Ctx from the decision on resume; exercise that
	// boundary instead of relying on the original Ctx surviving both asks.
	d := h.lastAsk
	c = &Ctx{Source: src.ID, Controller: 0, Targets: []state.Target{{IsPlayer: true, Player: 0}, {IsPlayer: true, Player: 1}},
		Remembered: d.ResumeRemembered, ForgetOtherSnapshot: d.ResumeForgetOtherSnapshot, ForgetOtherOwners: d.ResumeForgetOtherOwners,
		ForgetOtherReady: d.ResumeForgetOtherReady, ForgetOtherCleared: d.ResumeForgetOtherCleared,
		HiddenPick: []state.ObjID{second.ID}, HiddenPickDone: true, HiddenPickTarget: 1}
	Resolve(h, c, body)
	if c.ForgetOtherReady || c.ForgetOtherCleared {
		t.Fatal("multi-owner snapshot leaked into the next ability")
	}
	if second.Zone != state.ZHand || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{first.ID, second.ID}) {
		t.Fatalf("second=%s remembered=%v, want both moved cards", second.Zone, rememberedIDs(h.g.Obj(src.ID).Remembered))
	}
}

func TestChangeZoneSearchForgetOtherRememberedMultiOwner(t *testing.T) {
	h, src, cards := forgetFixtureHost(t, "Stale")
	stale := cards[0]
	first := h.g.AddObject(mkCard(t, "Name:First\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	second := h.g.AddObject(mkCard(t, "Name:Second\nTypes:Creature\nPT:3/3\nOracle:x\n"), 1)
	first, second, stale = h.g.Obj(first.ID), h.g.Obj(second.ID), h.g.Obj(stale.ID)
	h.g.SetZone(state.ZExile, 0, []state.ObjID{stale.ID})
	stale.Zone = state.ZExile
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{first.ID})
	h.g.SetZone(state.ZLibrary, 1, []state.ObjID{second.ID})
	first.Zone, second.Zone = state.ZLibrary, state.ZLibrary
	seedRemembered(h, src, first.ID, second.ID, stale.ID)
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: first.ID}, {Obj: second.ID}, {Obj: stale.ID}}}
	body := sa(t, "DB$ ChangeZone | Origin$ Library | Destination$ Exile | DefinedPlayer$ RememberedOwner | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Mandatory$ True | RememberChanged$ True | ForgetOtherRemembered$ True")
	if first.ID == second.ID || first.Zone != state.ZLibrary || second.Zone != state.ZLibrary || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{first.ID, second.ID, stale.ID}) {
		t.Fatal("precondition: distinct remembered creatures in two owners' libraries")
	}
	h.askResult = true
	Resolve(h, c, body)
	if h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != first.ID {
		t.Fatalf("first search options: %+v", h.lastAsk)
	}
	c.Search, c.SearchDone = []state.ObjID{first.ID}, true
	Resolve(h, c, body)
	if first.Zone != state.ZExile || h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != second.ID {
		t.Fatalf("second search options lost after first move: first=%s ask=%+v", first.Zone, h.lastAsk)
	}
	d := h.lastAsk
	c = &Ctx{Source: src.ID, Controller: 0, Targets: []state.Target{{IsPlayer: true, Player: 0}, {IsPlayer: true, Player: 1}},
		Remembered: d.ResumeRemembered, ForgetOtherSnapshot: d.ResumeForgetOtherSnapshot, ForgetOtherOwners: d.ResumeForgetOtherOwners,
		ForgetOtherReady: d.ResumeForgetOtherReady, ForgetOtherCleared: d.ResumeForgetOtherCleared,
		Search: []state.ObjID{second.ID}, SearchDone: true, LibraryTarget: 1}
	Resolve(h, c, body)
	if c.ForgetOtherReady || c.ForgetOtherCleared {
		t.Fatal("multi-owner snapshot leaked into the next ability")
	}
	if second.Zone != state.ZExile || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{first.ID, second.ID}) {
		t.Fatalf("second=%s remembered=%v, want both found cards", second.Zone, rememberedIDs(h.g.Obj(src.ID).Remembered))
	}
}
