package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestChooseCardForgetChosenFallbackAndRandom(t *testing.T) {
	for _, random := range []bool{false, true} {
		name := "fallback"
		body := "DB$ ChooseCard | Defined$ You | Amount$ 1 | Choices$ Creature.IsRemembered | ChoiceZone$ Exile | Mandatory$ True | ForgetChosen$ True"
		if random {
			name = "random"
			body += " | AtRandom$ True"
		}
		t.Run(name, func(t *testing.T) {
			h, src, cards := forgetFixtureHost(t, "Picked", "Retained")
			picked, retained := cards[0], cards[1]
			h.g.SetZone(state.ZExile, 0, []state.ObjID{picked.ID, retained.ID})
			picked.Zone, retained.Zone = state.ZExile, state.ZExile
			seedRemembered(h, src, picked.ID, retained.ID)
			c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: picked.ID}, {Obj: retained.ID}}}
			if picked.ID == retained.ID || picked.Zone != state.ZExile || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{picked.ID, retained.ID}) {
				t.Fatal("precondition: distinct exiled remembered candidates")
			}
			effChooseCard(h, c, sa(t, body))
			if len(c.Chosen) != 1 || c.Chosen[0].Obj != picked.ID {
				t.Fatalf("selected %v, want %d", c.Chosen, picked.ID)
			}
			if got := rememberedIDs(c.Remembered); !sameIDs(got, []state.ObjID{retained.ID}) {
				t.Errorf("ctx remembered %v", got)
			}
			if got := rememberedIDs(h.g.Obj(src.ID).Remembered); !sameIDs(got, []state.ObjID{retained.ID}) {
				t.Errorf("persistent remembered %v", got)
			}
		})
	}
}

func TestChangeZoneHandForgetOtherRememberedAnsweredSelector(t *testing.T) {
	h, src, cards := forgetFixtureHost(t, "Hand Pick", "Other Hand Pick", "Stale")
	picked, other, stale := cards[0], cards[1], cards[2]
	h.g.SetZone(state.ZHand, 0, []state.ObjID{picked.ID, other.ID})
	picked.Zone, other.Zone = state.ZHand, state.ZHand
	h.g.SetZone(state.ZExile, 0, []state.ObjID{stale.ID})
	stale.Zone = state.ZExile
	seedRemembered(h, src, picked.ID, other.ID, stale.ID)
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: picked.ID}, {Obj: other.ID}, {Obj: stale.ID}}}
	body := sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Exile | ChangeType$ Card.IsRemembered | ChangeNum$ 1 | Mandatory$ True | RememberChanged$ True | ForgetOtherRemembered$ True")
	if picked.ID == other.ID || picked.Zone != state.ZHand || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{picked.ID, other.ID, stale.ID}) {
		t.Fatal("precondition: distinct remembered cards in hand")
	}
	h.askResult = true
	Resolve(h, c, body)
	if h.lastAsk == nil || len(h.lastAsk.Options) != 2 {
		t.Fatalf("expected answered hand ask with two remembered options: %+v", h.lastAsk)
	}
	c.HandMove = []state.ObjID{picked.ID}
	c.HandMoveDone = true
	Resolve(h, c, body)
	if picked.Zone != state.ZExile || other.Zone != state.ZHand {
		t.Errorf("zones picked=%s other=%s", picked.Zone, other.Zone)
	}
	for _, got := range [][]state.ObjID{rememberedIDs(c.Remembered), rememberedIDs(h.g.Obj(src.ID).Remembered)} {
		if !sameIDs(got, []state.ObjID{picked.ID}) {
			t.Errorf("remembered %v, want picked only", got)
		}
	}
}

func TestChangeZoneHiddenForgetOtherRememberedSelector(t *testing.T) {
	h, src, cards := forgetFixtureHost(t, "Hidden Pick", "Stale")
	picked, stale := cards[0], cards[1]
	h.g.SetZone(state.ZGraveyard, 0, []state.ObjID{picked.ID})
	picked.Zone = state.ZGraveyard
	h.g.SetZone(state.ZExile, 0, []state.ObjID{stale.ID})
	stale.Zone = state.ZExile
	seedRemembered(h, src, picked.ID, stale.ID)
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: picked.ID}, {Obj: stale.ID}}}
	body := sa(t, "DB$ ChangeZone | Hidden$ True | Origin$ Graveyard,Exile | Destination$ Hand | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | RememberChanged$ True | ForgetOtherRemembered$ True")
	if picked.ID == stale.ID || picked.Zone != state.ZGraveyard || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{picked.ID, stale.ID}) {
		t.Fatal("precondition: remembered graveyard candidate and stale exile candidate")
	}
	h.askResult = true
	Resolve(h, c, body)
	if h.lastAsk == nil || len(h.lastAsk.Options) != 2 {
		t.Fatalf("hidden ask missing remembered options: %+v", h.lastAsk)
	}
	c.HiddenPick = []state.ObjID{picked.ID}
	c.HiddenPickDone = true
	Resolve(h, c, body)
	if picked.Zone != state.ZHand {
		t.Errorf("picked zone %s", picked.Zone)
	}
	for label, got := range map[string][]state.ObjID{"ctx": rememberedIDs(c.Remembered), "persistent": rememberedIDs(h.g.Obj(src.ID).Remembered)} {
		if !sameIDs(got, []state.ObjID{picked.ID}) {
			t.Errorf("%s remembered %v", label, got)
		}
	}
}
