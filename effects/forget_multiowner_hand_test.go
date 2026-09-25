package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestChangeZoneHandForgetOtherRememberedMultiOwner(t *testing.T) {
	h, src, initial := forgetFixtureHost(t, "Stale")
	first := h.g.AddObject(mkCard(t, "Name:First\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	second := h.g.AddObject(mkCard(t, "Name:Second\nTypes:Creature\nPT:3/3\nOracle:x\n"), 1)
	unremembered := h.g.AddObject(mkCard(t, "Name:Unremembered\nTypes:Creature\nPT:4/4\nOracle:x\n"), 1)
	first, second, unremembered = h.g.Obj(first.ID), h.g.Obj(second.ID), h.g.Obj(unremembered.ID)
	stale := h.g.Obj(initial[0].ID)
	h.g.SetZone(state.ZHand, 0, []state.ObjID{first.ID})
	h.g.SetZone(state.ZHand, 1, []state.ObjID{second.ID, unremembered.ID})
	first.Zone, second.Zone, unremembered.Zone = state.ZHand, state.ZHand, state.ZHand
	h.g.SetZone(state.ZExile, 0, []state.ObjID{stale.ID})
	stale.Zone = state.ZExile
	seedRemembered(h, src, first.ID, second.ID, stale.ID)
	remembered := []state.Target{{Obj: first.ID}, {Obj: second.ID}, {Obj: stale.ID}}
	body := sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Exile | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Optional$ True | RememberChanged$ True | ForgetOtherRemembered$ True")
	if first.ID == second.ID || second.ID == unremembered.ID || first.Zone != state.ZHand || second.Zone != state.ZHand || unremembered.Zone != state.ZHand || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{first.ID, second.ID, stale.ID}) || containsID(rememberedIDs(h.g.Obj(src.ID).Remembered), unremembered.ID) {
		t.Fatal("precondition: two distinct remembered creatures and one unremembered creature are in the owners' hands")
	}
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: remembered}
	h.askResult = true
	owners := []state.PlayerID{0, 1}
	walk := func(ctx *Ctx) {
		handMoveOwnersWalk(h, ctx, body, state.ZExile, owners, handMoveCount{fixed: 1}, false, nil, true)
	}
	walk(c)
	if h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != first.ID {
		t.Fatalf("first owner not offered its remembered card: %+v", h.lastAsk)
	}
	c.HandMove, c.HandMoveDone = []state.ObjID{first.ID}, true
	walk(c)
	if first.Zone != state.ZExile || h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != second.ID {
		t.Fatalf("second owner's remembered choice lost after first move: first=%s ask=%+v", first.Zone, h.lastAsk)
	}
	d := h.lastAsk
	c = &Ctx{Source: src.ID, Controller: 0, Remembered: d.ResumeRemembered,
		ForgetOtherSnapshot: d.ResumeForgetOtherSnapshot, ForgetOtherOwners: d.ResumeForgetOtherOwners,
		ForgetOtherReady: d.ResumeForgetOtherReady, ForgetOtherCleared: d.ResumeForgetOtherCleared,
		HandMove: []state.ObjID{second.ID}, HandMoveDone: true, HandMoveTarget: d.ResumeTarget}
	walk(c)
	if second.Zone != state.ZExile || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{first.ID, second.ID}) {
		t.Fatalf("second=%s remembered=%v, want both moved cards", second.Zone, rememberedIDs(h.g.Obj(src.ID).Remembered))
	}
}

func TestForgetOtherRememberedDeclineAndNoEligibleDoNotClear(t *testing.T) {
	for _, tc := range []struct {
		name string
		zone state.Zone
		pick []state.ObjID
	}{
		{name: "declined", zone: state.ZGraveyard},
		{name: "no eligible", zone: state.ZExile},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, src, cards := forgetFixtureHost(t, "Remembered")
			card := h.g.Obj(cards[0].ID)
			h.g.SetZone(tc.zone, 0, []state.ObjID{card.ID})
			card.Zone = tc.zone
			seedRemembered(h, src, card.ID)
			c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: card.ID}}}
			body := sa(t, "DB$ ChangeZone | Hidden$ True | Origin$ Graveyard | Destination$ Exile | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | ForgetOtherRemembered$ True")
			if tc.name == "declined" {
				c.HiddenPickDone = true // empty answer is the legal optional decline
			}
			Resolve(h, c, body)
			for _, ev := range h.log {
				if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented API ChangeZone") {
					t.Fatal("precondition: ChangeZone handler ran, not the unimplemented-API fallback")
				}
			}
			if card.Zone != tc.zone || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{card.ID}) {
				t.Fatalf("no move should preserve remembered state: zone=%s remembered=%v", card.Zone, rememberedIDs(h.g.Obj(src.ID).Remembered))
			}
		})
	}
}
