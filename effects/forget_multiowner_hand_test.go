package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestChangeZoneHandOwnersForgetOtherRememberedResume is the regression for
// the multi-owner suspended-walk bug on the hand-owner shape. Two owners each
// hold a card matching the Card.IsRemembered filter, the walk asks owner 0,
// and owner 0's answered move clears the remembered set (ForgetOtherRemembered$
// True, and NO RememberChanged$ -- the finding's exact broken shape). On
// re-entry the owner selector (DefinedPlayer$ RememberedOwner) must still
// yield the ORIGINAL two owners, not the now-empty remembered set; otherwise
// effChangeZoneHandOwners returns at its empty-owner guard before
// handMoveOwnersWalk can restore the captured owner list and owner 1's
// already-answered card never moves.
//
// The walk is entered through the production Resolve routing, not the internal
// handMoveOwnersWalk, and the second ask is rebuilt from the first decision's
// resume fields the way a real host does.
func TestChangeZoneHandOwnersForgetOtherRememberedResume(t *testing.T) {
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
	// DefinedPlayer$ RememberedOwner, ForgetOtherRemembered$ True, and NO
	// RememberChanged$: the first move clears the set and re-remembers nothing.
	body := sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Exile | DefinedPlayer$ RememberedOwner | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Optional$ True | ForgetOtherRemembered$ True")
	if first.ID == second.ID || second.ID == unremembered.ID || first.Zone != state.ZHand || second.Zone != state.ZHand || unremembered.Zone != state.ZHand {
		t.Fatal("precondition: two distinct remembered creatures and one unremembered creature are in the owners' hands")
	}
	if got := rememberedIDs(h.g.Obj(src.ID).Remembered); !sameIDs(got, []state.ObjID{first.ID, second.ID, stale.ID}) || containsID(got, unremembered.ID) {
		t.Fatalf("precondition: persistent remembered = %v, want the three seeded cards", got)
	}
	c := &Ctx{Source: src.ID, Controller: 0, Remembered: remembered}
	h.askResult = true
	Resolve(h, c, body)
	if h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != first.ID {
		t.Fatalf("first owner not offered its remembered card: %+v", h.lastAsk)
	}
	// Answer owner 0 by moving the first card.
	c.HandMove, c.HandMoveDone = []state.ObjID{first.ID}, true
	Resolve(h, c, body)
	if first.Zone != state.ZExile {
		t.Fatalf("first owner's answered card did not move: zone=%s", first.Zone)
	}
	if !c.ForgetOtherReady || len(c.ForgetOtherOwners) != 2 {
		t.Fatalf("precondition: the owner cursor was captured before the clear: ready=%v owners=%v", c.ForgetOtherReady, c.ForgetOtherOwners)
	}
	if h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != second.ID {
		t.Fatalf("second owner's remembered choice lost after first move: first=%s ask=%+v", first.Zone, h.lastAsk)
	}
	// Rebuild the resumed Ctx from the second decision alone, as a real host
	// does -- the original Ctx's owner list must not be needed.
	d := h.lastAsk
	if !d.ResumeForgetOtherReady || len(d.ResumeForgetOtherOwners) != 2 {
		t.Fatalf("precondition: second decision did not ride the captured owners: %v", d.ResumeForgetOtherOwners)
	}
	if len(d.ResumeRemembered) != 0 {
		t.Fatalf("precondition: the clear must leave nothing re-remembered (no RememberChanged$): %v", d.ResumeRemembered)
	}
	c = &Ctx{Source: src.ID, Controller: 0, Remembered: d.ResumeRemembered,
		ForgetOtherSnapshot: d.ResumeForgetOtherSnapshot, ForgetOtherOwners: d.ResumeForgetOtherOwners,
		ForgetOtherReady: d.ResumeForgetOtherReady, ForgetOtherCleared: d.ResumeForgetOtherCleared,
		HandMove: []state.ObjID{second.ID}, HandMoveDone: true, HandMoveTarget: d.ResumeTarget}
	Resolve(h, c, body)
	if second.Zone != state.ZExile {
		t.Fatalf("second=%s, want the later owner's answered card moved on resume", second.Zone)
	}
	if unremembered.Zone != state.ZHand {
		t.Errorf("unremembered card was disturbed: zone=%s", unremembered.Zone)
	}
	if c.ForgetOtherReady || c.ForgetOtherCleared {
		t.Errorf("multi-owner snapshot leaked into the next ability: ready=%v cleared=%v", c.ForgetOtherReady, c.ForgetOtherCleared)
	}
}

// TestChangeZoneHandOwnersForgetOtherRememberedDeclineKeepsMemory establishes
// and pins the clearing semantics when the asked owner declines the optional
// move: the already-remembered card stays put and the persistent remembered
// list is NOT cleared. The sub-run order is deliberate -- the baseline run
// (the same fixture, the same posted decision answered WITH the card) proves
// ForgetOtherRemembered$ does clear on a real move, so the decline assertion
// is sensitive to the clearing mechanism and cannot pass with the primitive
// unregistered or the parameter ignored.
//
// Forge (ChangeZoneEffect's fetch path) calls source.clearRemembered() BEFORE
// the choose, so its memory is cleared even when the player picks nothing;
// this engine clears lazily as part of settling a moved card. That difference
// is pre-existing contract, unchanged by this ticket, and is called out in
// the report's Issues section -- it is not claimed as fixed here.
func TestChangeZoneHandOwnersForgetOtherRememberedDeclineKeepsMemory(t *testing.T) {
	declineRun := func(t *testing.T) (*fakeHost, *state.Object, *state.Object) {
		t.Helper()
		h, src, cards := forgetFixtureHost(t, "Remembered")
		card := h.g.Obj(cards[0].ID)
		h.g.SetZone(state.ZHand, 0, []state.ObjID{card.ID})
		card.Zone = state.ZHand
		seedRemembered(h, src, card.ID)
		if card.Zone != state.ZHand || !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{card.ID}) {
			t.Fatal("precondition: the remembered creature is in the asking player's hand and the persistent memory holds it")
		}
		return h, src, card
	}
	body := sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Exile | DefinedPlayer$ You | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Optional$ True | ForgetOtherRemembered$ True")

	t.Run("baseline taken clears", func(t *testing.T) {
		h, src, card := declineRun(t)
		c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: card.ID}}}
		h.askResult = true
		Resolve(h, c, body)
		if h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != card.ID {
			t.Fatalf("precondition: the optional hand move posted a real one-card decision, got %+v", h.lastAsk)
		}
		c = &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: card.ID}},
			HandMove: []state.ObjID{card.ID}, HandMoveDone: true, HandMoveTarget: 0}
		Resolve(h, c, body)
		if card.Zone != state.ZExile {
			t.Fatalf("baseline card did not move: zone=%s", card.Zone)
		}
		if got := rememberedIDs(h.g.Obj(src.ID).Remembered); len(got) != 0 {
			t.Errorf("baseline move did not clear remembered state: %v", got)
		}
	})

	t.Run("declined keeps", func(t *testing.T) {
		h, src, card := declineRun(t)
		c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: card.ID}}}
		h.askResult = true
		Resolve(h, c, body)
		if h.lastAsk == nil || len(h.lastAsk.Options) != 1 || h.lastAsk.Options[0].Obj != card.ID {
			t.Fatalf("precondition: the optional hand move posted a real one-card decision, got %+v", h.lastAsk)
		}
		// The empty answer is the legal decline: re-enter with an empty pick.
		c = &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: card.ID}},
			HandMoveDone: true, HandMoveTarget: 0}
		Resolve(h, c, body)
		if card.Zone != state.ZHand {
			t.Fatalf("declined card moved: zone=%s", card.Zone)
		}
		if got := rememberedIDs(h.g.Obj(src.ID).Remembered); !sameIDs(got, []state.ObjID{card.ID}) {
			t.Errorf("decline cleared remembered state: %v, want [%d]", got, card.ID)
		}
	})
}

// TestChangeZoneHandOwnersForgetOtherRememberedNoEligibleKeepsMemory pins the
// no-movable-card path: when the owner's hand holds nothing matching the
// filter, the walk is never asked and the remembered state is left intact.
// The baseline sub-run has an eligible remembered card, which is asked and
// whose move clears -- so a green no-eligible run cannot be the primitive
// silently doing nothing.
func TestChangeZoneHandOwnersForgetOtherRememberedNoEligibleKeepsMemory(t *testing.T) {
	body := sa(t, "DB$ ChangeZone | Origin$ Hand | Destination$ Exile | DefinedPlayer$ You | ChangeType$ Creature.IsRemembered | ChangeNum$ 1 | Optional$ True | ForgetOtherRemembered$ True")

	baseRun := func(t *testing.T, rememberedInHand bool) (*fakeHost, *state.Object, *state.Object) {
		t.Helper()
		h, src, cards := forgetFixtureHost(t, "Remembered")
		remembered := h.g.Obj(cards[0].ID)
		other := h.g.AddObject(mkCard(t, "Name:Not Remembered\nTypes:Creature\nPT:1/1\nOracle:x\n"), 0)
		// Re-resolve after the AddObject: the earlier pointer predates the
		// g.Objs append and is stale (the fixture helper's own warning).
		remembered, other = h.g.Obj(cards[0].ID), h.g.Obj(other.ID)
		zone := state.ZExile
		hand := []state.ObjID{other.ID}
		if rememberedInHand {
			zone = state.ZHand
			hand = []state.ObjID{remembered.ID, other.ID}
		}
		h.g.SetZone(zone, 0, []state.ObjID{remembered.ID})
		remembered.Zone = zone
		h.g.SetZone(state.ZHand, 0, hand)
		other.Zone = state.ZHand
		seedRemembered(h, src, remembered.ID)
		ctx := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: remembered.ID}}}
		if !MatchesSpecCtx(h.g, "Creature.IsRemembered", remembered.ID, ctx.SpecContext(0)) {
			t.Fatal("precondition: the seeded remembered card matches the filter")
		}
		if MatchesSpecCtx(h.g, "Creature.IsRemembered", other.ID, ctx.SpecContext(0)) {
			t.Fatal("precondition: the hand card must NOT match the remembered filter")
		}
		if !sameIDs(rememberedIDs(h.g.Obj(src.ID).Remembered), []state.ObjID{remembered.ID}) {
			t.Fatal("precondition: the persistent memory holds exactly the remembered card")
		}
		return h, src, remembered
	}

	t.Run("baseline eligible taken clears", func(t *testing.T) {
		h, src, remembered := baseRun(t, true)
		c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: remembered.ID}}}
		h.askResult = true
		Resolve(h, c, body)
		if h.lastAsk == nil {
			t.Fatalf("baseline: an eligible card must post a decision")
		}
		c = &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: remembered.ID}},
			HandMove: []state.ObjID{remembered.ID}, HandMoveDone: true, HandMoveTarget: 0}
		Resolve(h, c, body)
		if remembered.Zone != state.ZExile {
			t.Fatalf("baseline eligible card did not move: zone=%s", remembered.Zone)
		}
		if got := rememberedIDs(h.g.Obj(src.ID).Remembered); len(got) != 0 {
			t.Errorf("baseline move did not clear remembered state: %v", got)
		}
	})

	t.Run("no eligible keeps", func(t *testing.T) {
		h, src, remembered := baseRun(t, false)
		c := &Ctx{Source: src.ID, Controller: 0, Remembered: []state.Target{{Obj: remembered.ID}}}
		h.askResult = true
		Resolve(h, c, body)
		if h.lastAsk != nil {
			t.Fatalf("no-eligible hand must not post a decision, got %+v", h.lastAsk)
		}
		if got := rememberedIDs(h.g.Obj(src.ID).Remembered); !sameIDs(got, []state.ObjID{remembered.ID}) {
			t.Errorf("no-eligible path cleared remembered state: %v, want [%d]", got, remembered.ID)
		}
		for _, ev := range h.log {
			if ev.Kind == events.Note && strings.Contains(ev.Text, "unimplemented API ChangeZone") {
				t.Fatal("precondition: the ChangeZone handler ran, not the unimplemented-API fallback")
			}
		}
	})
}
