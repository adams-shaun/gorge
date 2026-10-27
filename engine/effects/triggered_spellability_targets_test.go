package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredSpellAbilityTargetsSelector pins Defined$
// TriggeredSpellAbilityTargets: Forge's SpellAbilityTargets triggering object,
// the cast spell's root targets then every chain link's, de-duplicated, with
// player entries kept. A spell that is gone is a known-empty set, never the
// source.
func TestTriggeredSpellAbilityTargetsSelector(t *testing.T) {
	h := newHost(t, 2)
	mk := func() state.ObjID {
		o := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
		events.Move(h.g, o.ID, state.ZLibrary, state.ZBattlefield)
		return o.ID
	}
	a, b := mk(), mk()
	spell := stackSpell(t, h, 0)
	sp := h.g.Obj(spell)
	sp.Targets = []state.Target{{Obj: a}}
	sp.SubTargets = []state.Target{{Obj: b}, {Player: 1, IsPlayer: true}, {Obj: a}}
	if sp.Zone != state.ZStack {
		t.Fatalf("precondition: spell zone = %s, want the stack", sp.Zone)
	}
	c := &Ctx{Source: a, Controller: 0, Remembered: []state.Target{{Obj: spell}}}

	want := []state.Target{{Obj: a}, {Obj: b}, {Player: 1, IsPlayer: true}}
	got, ok := knownDefinedTargets(h, c, "TriggeredSpellAbilityTargets")
	if !ok || !slices.Equal(got, want) {
		t.Fatalf("selector = %v, %v; want %v, true", got, ok, want)
	}

	// The fire-time TriggerAbility role wins over Remembered, as for
	// TriggeredSpellAbility.
	other := stackSpell(t, h, 0)
	h.g.Obj(other).Targets = []state.Target{{Obj: b}}
	roleCtx := &Ctx{Source: a, Remembered: []state.Target{{Obj: spell}}}
	roleCtx.TriggerAbility = other
	got, ok = knownDefinedTargets(h, roleCtx, "TriggeredSpellAbilityTargets")
	if !ok || !slices.Equal(got, []state.Target{{Obj: b}}) {
		t.Fatalf("TriggerAbility-role selector = %v, %v; want [b]", got, ok)
	}

	// A spell with no targets, and a spell that is gone: known-empty.
	sp.Targets, sp.SubTargets = nil, nil
	got, ok = knownDefinedTargets(h, c, "TriggeredSpellAbilityTargets")
	if !ok || len(got) != 0 {
		t.Fatalf("untargeted spell selector = %v, %v; want empty, true", got, ok)
	}
	got, ok = knownDefinedTargets(h, &Ctx{Source: a, Controller: 0, Remembered: []state.Target{{Obj: 9999}}}, "TriggeredSpellAbilityTargets")
	if !ok || len(got) != 0 {
		t.Fatalf("gone-spell selector = %v, %v; want empty, true (never the source)", got, ok)
	}
}

// TestTriggeredSpellAbilityTargetsPumpsTheTargets runs Storm, Windrider's real
// TrigPump: the cast spell's targeted creature gains Flying, not Storm.
func TestTriggeredSpellAbilityTargetsPumpsTheTargets(t *testing.T) {
	_, pump := corpusCardSA(t, "Storm, Windrider", "TrigPump")
	if pump.API != "Pump" || pump.Params["Defined"] != "TriggeredSpellAbilityTargets" {
		t.Fatalf("TrigPump = %s %+v, want Pump with Defined$ TriggeredSpellAbilityTargets", pump.API, pump.Params)
	}
	h := newHost(t, 2)
	mk := func() state.ObjID {
		o := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
		events.Move(h.g, o.ID, state.ZLibrary, state.ZBattlefield)
		return o.ID
	}
	storm, a := mk(), mk()
	spell := stackSpell(t, h, 0)
	h.g.Obj(spell).Targets = []state.Target{{Obj: a}}

	Resolve(h, &Ctx{Source: storm, Controller: 0, Remembered: []state.Target{{Obj: spell}}}, pump)

	var granted []state.ObjID
	for _, ce := range h.continuous {
		if slices.Contains(ce.AddKeywords, "Flying") {
			granted = append(granted, ce.Source)
		}
	}
	if !slices.Equal(granted, []state.ObjID{a}) {
		t.Fatalf("Flying granted to %v, want only the targeted creature %d (not Storm %d)", granted, a, storm)
	}
}
