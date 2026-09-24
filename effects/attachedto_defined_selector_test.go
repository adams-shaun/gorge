package effects

// This file pins the DOTTED `AttachedTo <referent>[.<quals>]` selector in a
// Defined$/Object$/ChooseFromDefined$ position (definedSpec's
// attachedToDefinedSelector): "the objects attached to whatever <referent>
// names". Before it landed the spelling was only a FILTER predicate
// (effects/filter.go's wordAttachedTo), so in a Defined position the whole
// selector was unknown and every consumer either no-opped or attached the
// wrong object (Murderous Spoils, Fumble, Rhuk, Cass).
//
// It covers both reads the selector needs -- the live `AttachedTo == bearer`
// link and the `LastBearer == bearer` were-attached fallback the events.Apply
// folds preserve -- the qualifier list, the referent family, and the plural
// fail-closed cardinality rule shared with the predicate.

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// attachedSelectorBoard builds a two-seat board: a battlefield creature
// bearer, a battlefield creature "other" (a second potential bearer), an Aura
// and an Equipment attached to the bearer, a graveyard Aura that WAS attached
// to the bearer, and a battlefield Equipment attached to "other". Ids are
// returned by key.
func attachedSelectorBoard(t *testing.T) (*fakeHost, *Ctx, map[string]state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	other := h.g.AddObject(mkCard(t, "Name:Other\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	aura := h.g.AddObject(mkCard(t, "Name:Favor\nManaCost:1 W\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"), 0)
	equip := h.g.AddObject(mkCard(t, "Name:Splitter\nManaCost:1\nTypes:Artifact Equipment\nOracle:x\n"), 0)
	graveAura := h.g.AddObject(mkCard(t, "Name:Grave Favor\nManaCost:1 W\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"), 0)
	otherEquip := h.g.AddObject(mkCard(t, "Name:Other Splitter\nManaCost:1\nTypes:Artifact Equipment\nOracle:x\n"), 0)
	h.g.Obj(bear.ID).Zone = state.ZBattlefield
	h.g.Obj(other.ID).Zone = state.ZBattlefield
	h.g.Obj(aura.ID).Zone = state.ZBattlefield
	h.g.Obj(equip.ID).Zone = state.ZBattlefield
	h.g.Obj(otherEquip.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{bear.ID, other.ID, aura.ID, equip.ID, otherEquip.ID})
	h.g.Obj(graveAura.ID).Zone = state.ZGraveyard
	h.g.SetZone(state.ZGraveyard, 0, []state.ObjID{graveAura.ID})
	h.g.Obj(aura.ID).AttachedTo = bear.ID
	h.g.Obj(equip.ID).AttachedTo = bear.ID
	h.g.Obj(otherEquip.ID).AttachedTo = other.ID
	// The were-attached Aura: swept off the battlefield by the CR 704.5m
	// arm, so it is a graveyard card whose live link is clear and whose
	// LastBearer still names the dead bearer. Set through the real event fold
	// so the test proves the fold, not a hand-set field.
	h.Emit(events.Event{Kind: events.Unattached, Obj: graveAura.ID, IDs: []state.ObjID{bear.ID}})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: graveAura.ID,
		From: state.ZBattlefield, To: state.ZGraveyard})
	c := &Ctx{Source: bear.ID, Controller: 0, Targets: []state.Target{{Obj: bear.ID}}}
	return h, c, map[string]state.ObjID{
		"bear": bear.ID, "other": other.ID, "aura": aura.ID,
		"equip": equip.ID, "graveAura": graveAura.ID, "otherEquip": otherEquip.ID,
	}
}

// TestAttachedToDefinedSelectorTargetedLiveAndWereAttached pins the core
// selector against a resolution-time target: the live attachments resolve,
// the swept were-attached Aura resolves from LastBearer, and the same object
// re-attached elsewhere stops resolving.
func TestAttachedToDefinedSelectorTargetedLiveAndWereAttached(t *testing.T) {
	h, c, ids := attachedSelectorBoard(t)

	// Preconditions: the bearer is on the battlefield; the Aura and Equipment
	// really are live-attached to it; the graveyard Aura is really OFF the
	// battlefield with a cleared live link and the fold's LastBearer set; the
	// other Equipment is attached to the OTHER bearer (so it must not resolve).
	if o := h.g.Obj(ids["bear"]); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition failed: bearer zone %v, want battlefield", o)
	}
	if got := h.g.Obj(ids["aura"]).AttachedTo; got != ids["bear"] {
		t.Fatalf("precondition failed: aura AttachedTo = %d, want bearer %d", got, ids["bear"])
	}
	if got := h.g.Obj(ids["equip"]).AttachedTo; got != ids["bear"] {
		t.Fatalf("precondition failed: equip AttachedTo = %d, want bearer %d", got, ids["bear"])
	}
	if got := h.g.Obj(ids["graveAura"]).AttachedTo; got != 0 {
		t.Fatalf("precondition failed: graveAura AttachedTo = %d, want 0", got)
	}
	if got := h.g.Obj(ids["graveAura"]).LastBearer; got != ids["bear"] {
		t.Fatalf("precondition failed: graveAura LastBearer = %d, want the folded bearer %d", got, ids["bear"])
	}
	if h.g.Obj(ids["graveAura"]).Zone != state.ZGraveyard {
		t.Fatalf("precondition failed: graveAura zone %v, want graveyard", h.g.Obj(ids["graveAura"]).Zone)
	}
	if got := h.g.Obj(ids["otherEquip"]).AttachedTo; got != ids["other"] {
		t.Fatalf("precondition failed: otherEquip AttachedTo = %d, want other %d", got, ids["other"])
	}

	ts, ok := definedSpec(h, c, "AttachedTo Targeted")
	if !ok {
		t.Fatalf("AttachedTo Targeted classified unknown")
	}
	got := map[state.ObjID]bool{}
	for _, t2 := range ts {
		got[t2.Obj] = true
	}
	if !got[ids["aura"]] || !got[ids["equip"]] || !got[ids["graveAura"]] {
		t.Fatalf("targets %+v missing live Aura/Equipment or were-attached graveAura", ts)
	}
	if got[ids["otherEquip"]] {
		t.Fatalf("the other bearer's Equipment %d must not resolve for this bearer", ids["otherEquip"])
	}

	// A re-attach clears LastBearer (the Attach fold), so the formerly
	// attached Aura stops resolving for the old bearer.
	h.Emit(events.Event{Kind: events.Attach, Obj: ids["graveAura"], IDs: []state.ObjID{ids["other"]}})
	if got := h.g.Obj(ids["graveAura"]).LastBearer; got != 0 {
		t.Fatalf("precondition failed after re-attach: LastBearer = %d, want 0", got)
	}
	ts2, ok := definedSpec(h, c, "AttachedTo Targeted")
	if !ok {
		t.Fatalf("AttachedTo Targeted classified unknown after re-attach")
	}
	for _, t2 := range ts2 {
		if t2.Obj == ids["graveAura"] {
			t.Fatalf("a re-attached object must no longer resolve as were-attached to the old bearer")
		}
	}
}

// TestAttachedToDefinedSelectorQualifierList pins the comma-OR qualifier
// reading: `AttachedTo Targeted.Aura,Equipment` admits both kinds, while a
// single qualifier narrows to its own kind.
func TestAttachedToDefinedSelectorQualifierList(t *testing.T) {
	h, c, ids := attachedSelectorBoard(t)
	ts, ok := definedSpec(h, c, "AttachedTo Targeted.Aura,Equipment")
	if !ok {
		t.Fatalf("AttachedTo Targeted.Aura,Equipment classified unknown")
	}
	got := map[state.ObjID]bool{}
	for _, t2 := range ts {
		got[t2.Obj] = true
	}
	if !got[ids["aura"]] || !got[ids["equip"]] || !got[ids["graveAura"]] {
		t.Fatalf("qualifier list %+v missing the live Aura/Equipment or the were-attached Aura", ts)
	}
	if got[ids["otherEquip"]] {
		t.Fatalf("the qualifier list must exclude the other bearer's Equipment")
	}

	only := func(spec string) map[state.ObjID]bool {
		res, ok := definedSpec(h, c, spec)
		if !ok {
			t.Fatalf("%s classified unknown", spec)
		}
		out := map[state.ObjID]bool{}
		for _, t2 := range res {
			out[t2.Obj] = true
		}
		return out
	}
	auras := only("AttachedTo Targeted.Aura")
	if !auras[ids["aura"]] || !auras[ids["graveAura"]] {
		t.Fatalf("AttachedTo Targeted.Aura %v missing a live or were-attached Aura", auras)
	}
	if auras[ids["equip"]] {
		t.Fatalf("AttachedTo Targeted.Aura must exclude the Equipment")
	}
	equips := only("AttachedTo Targeted.Equipment")
	if !equips[ids["equip"]] {
		t.Fatalf("AttachedTo Targeted.Equipment %v missing the Equipment", equips)
	}
	if equips[ids["aura"]] || equips[ids["graveAura"]] {
		t.Fatalf("AttachedTo Targeted.Equipment must exclude the Auras")
	}
}

// TestAttachedToDefinedSelectorTriggerReferents pins the trigger family: the
// referent is the Remembered object a trigger captured, and an absent
// binding fails closed.
func TestAttachedToDefinedSelectorTriggerReferents(t *testing.T) {
	h, c, ids := attachedSelectorBoard(t)
	c.Remembered = []state.Target{{Obj: ids["bear"]}}
	ts, ok := definedSpec(h, c, "AttachedTo TriggeredCardLKICopy.Equipment")
	if !ok {
		t.Fatalf("AttachedTo TriggeredCardLKICopy.Equipment classified unknown")
	}
	got := map[state.ObjID]bool{}
	for _, t2 := range ts {
		got[t2.Obj] = true
	}
	if !got[ids["equip"]] {
		t.Fatalf("trigger referent %+v missing the Equipment attached to the remembered creature", ts)
	}
	if got[ids["aura"]] || got[ids["otherEquip"]] {
		t.Fatalf("the .Equipment qualifier must exclude the Aura and the other bearer's Equipment")
	}
	// TriggeredAttackerLKICopy reads the same Remembered set.
	tsA, ok := definedSpec(h, c, "AttachedTo TriggeredAttackerLKICopy.Equipment")
	if !ok || len(tsA) != 1 || tsA[0].Obj != ids["equip"] {
		t.Fatalf("AttachedTo TriggeredAttackerLKICopy.Equipment = %+v, %v; want the Equipment %d", tsA, ok, ids["equip"])
	}
	// Absent binding: unbound, the whole selector is unknown.
	empty := &Ctx{Source: ids["bear"], Controller: 0}
	if _, ok := definedSpec(h, empty, "AttachedTo TriggeredCardLKICopy.Equipment"); ok {
		t.Fatalf("an absent trigger referent binding must make the selector unknown")
	}
}

// TestAttachedToDefinedSelectorPluralBearerFailsClosed pins the 82db540a
// cardinality rule reused from the predicate: a plural bearer binding is
// ambiguous, so the selector is unknown rather than an any-of guess.
func TestAttachedToDefinedSelectorPluralBearerFailsClosed(t *testing.T) {
	h, c, ids := attachedSelectorBoard(t)
	plural := &Ctx{Source: ids["bear"], Controller: 0, Targets: []state.Target{
		{Obj: ids["bear"]}, {Obj: ids["other"]},
	}}
	if ts, ok := definedSpec(h, plural, "AttachedTo Targeted.Equipment"); ok {
		t.Fatalf("a plural bearer binding must be unknown, got targets %+v", ts)
	}
	// And knownDefinedTargets, the fail-closed caller, agrees.
	if _, ok := knownDefinedTargets(h, plural, "AttachedTo Targeted.Equipment"); ok {
		t.Fatalf("knownDefinedTargets must classify a plural bearer binding unknown")
	}
	if _, ok := definedSpec(h, c, "AttachedTo NotAReferent.Equipment"); ok {
		t.Fatalf("an unknown referent token must stay unknown")
	}
}

// TestAttachedToDefinedSelectorCompoundSpellingFailsClosed pins that a
// ` & ` conjunction spelling is owned by knownDefinedTargets' splitter, not
// consumed whole by the dotted selector (which would silently resolve the
// malformed qualifier to an empty set).
func TestAttachedToDefinedSelectorCompoundSpellingFailsClosed(t *testing.T) {
	h, c, _ := attachedSelectorBoard(t)
	// The selector itself refuses the compound value...
	if ts, ok := definedSpec(h, c, "AttachedTo Targeted.Equipment & NotAThing"); ok {
		t.Fatalf("a compound ` & ` spelling must not be consumed by the selector, got %+v", ts)
	}
	// ...and knownDefinedTargets' conjunction splitter fails closed on it: the
	// second part (NotAThing) is an unknown defined target.
	if _, ok := knownDefinedTargets(h, c, "AttachedTo Targeted.Equipment & NotAThing"); ok {
		t.Fatalf("knownDefinedTargets must fail closed on a partly-unknown conjunction")
	}
}
func TestAttachedToDefinedSelectorBareFormUnchanged(t *testing.T) {
	h, c, ids := attachedSelectorBoard(t)
	// Source is the Aura; its own bearer is the bear.
	auraCtx := &Ctx{Source: ids["aura"], Controller: 0}
	ts, ok := definedSpec(h, auraCtx, "AttachedTo")
	if !ok || len(ts) != 1 || ts[0].Obj != ids["bear"] {
		t.Fatalf("bare AttachedTo = %+v, %v; want the source's own bearer %d", ts, ok, ids["bear"])
	}
	// The dotted form must NOT be claimed by the bare case's prefix.
	if ts, ok := definedSpec(h, c, "AttachedTo Targeted.Equipment"); !ok || len(ts) == 0 {
		t.Fatalf("dotted AttachedTo Targeted.Equipment = %+v, %v; want the Equipment", ts, ok)
	}
}
