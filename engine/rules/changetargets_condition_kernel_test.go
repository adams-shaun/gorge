package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMeddleRedirectsSingleCreatureTargetKernel: CR 115.7a through the
// ConditionTargetValidTargeting$/ConditionTargetsSingleTarget$ gate -- Murder
// has one target and that target is a creature, so Meddle asks its controller
// for another one, and the answer moves the spell's target.
func TestMeddleRedirectsSingleCreatureTargetKernel(t *testing.T) {
	t.Parallel()
	e := stealEngine(t, 745)
	mineA := onBoard(t, e, 0, "Name:A\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	mineB := onBoard(t, e, 0, "Name:B\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	murder := e.G.AddObject(card(t, "Name:Murder\nTypes:Instant\nManaCost:B\nA:SP$ Destroy | ValidTgts$ Creature.OppCtrl\nOracle:x\n"), 1)
	meddle := e.G.AddObject(choiceCorpusCard(t, "Meddle"), 0)
	for _, id := range []state.ObjID{murder.ID, meddle.ID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZStack})
	}
	if z := e.G.Obj(mineA).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: creature A zone = %v, want battlefield", z)
	}
	if z := e.G.Obj(mineB).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: creature B zone = %v, want battlefield", z)
	}
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: murder.ID, IDs: []state.ObjID{mineA}})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: meddle.ID, IDs: []state.ObjID{murder.ID}})
	if got := e.G.Obj(murder.ID).Targets; len(got) != 1 || got[0].IsPlayer || got[0].Obj != mineA {
		t.Fatalf("precondition: Murder targets = %+v, want creature %d", got, mineA)
	}
	if got := e.G.Obj(meddle.ID).Targets; len(got) != 1 || got[0].Obj != murder.ID {
		t.Fatalf("precondition: Meddle targets = %+v, want the Murder spell", got)
	}
	resolveUnderProbe(e, func() *effects.Ctx {
		return &effects.Ctx{Source: meddle.ID, Controller: 0, Targets: []state.Target{{Obj: murder.ID}},
			TargetsOffered: true, SVars: meddle.Face().SVars}
	}, meddle.Face().SpellAbility())
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("Meddle did not ask: %+v", d)
	}
	if offersObj(d, mineA) {
		t.Fatalf("offered Murder's current target: %+v", d.Options)
	}
	submitChoices(t, e, optionOfObj(t, d, mineB))
	if got := e.G.Obj(murder.ID).Targets; len(got) != 1 || got[0].Obj != mineB {
		t.Fatalf("Murder targets = %+v, want creature %d", got, mineB)
	}
}

// TestMeddleLeavesNonCreatureAndMultiTargetsAlone: the gate is a real gate.
// (a) Murder-style spell targeting a player never matches the creature spec;
// (b) a spell with two creature targets fails the single-target leg; neither
// asks, and the spell's targets are untouched.
func TestMeddleLeavesNonCreatureAndMultiTargetsAlone(t *testing.T) {
	t.Parallel()
	e := stealEngine(t, 746)
	mineA := onBoard(t, e, 0, "Name:A\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	mineB := onBoard(t, e, 0, "Name:B\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	burn := e.G.AddObject(card(t, "Name:Burn\nTypes:Instant\nManaCost:R\nA:SP$ DealDamage | ValidTgts$ Player | NumDmg$ 1\nOracle:x\n"), 1)
	double := e.G.AddObject(card(t, "Name:Doom\nTypes:Instant\nManaCost:B\nA:SP$ Destroy | ValidTgts$ Creature.OppCtrl | TargetMin$ 2 | TargetMax$ 2\nOracle:x\n"), 1)
	meddle := e.G.AddObject(choiceCorpusCard(t, "Meddle"), 0)
	for _, id := range []state.ObjID{burn.ID, double.ID, meddle.ID} {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZStack})
	}
	// (a) the spell's one target is a player.
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: burn.ID, Player: 0, Amount: 1})
	if got := e.G.Obj(burn.ID).Targets; len(got) != 1 || !got[0].IsPlayer || got[0].Player != 0 {
		t.Fatalf("precondition: Burn targets = %+v, want player 0", got)
	}
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: meddle.ID, IDs: []state.ObjID{burn.ID}})
	resolveUnderProbe(e, func() *effects.Ctx {
		return &effects.Ctx{Source: meddle.ID, Controller: 0, Targets: []state.Target{{Obj: burn.ID}},
			TargetsOffered: true, SVars: meddle.Face().SVars}
	}, meddle.Face().SpellAbility())
	if d := e.Pending(); d != nil {
		t.Fatalf("Meddle asked over a player target: %+v", d)
	}
	if got := e.G.Obj(burn.ID).Targets; len(got) != 1 || !got[0].IsPlayer || got[0].Player != 0 {
		t.Fatalf("Burn targets = %+v, want player 0 untouched", got)
	}
	// (b) the spell's two targets are both creatures.
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: double.ID, IDs: []state.ObjID{mineA}})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: double.ID, IDs: []state.ObjID{mineB}, Amount: 2})
	if got := e.G.Obj(double.ID).Targets; len(got) != 2 || got[0].Obj != mineA || got[1].Obj != mineB {
		t.Fatalf("precondition: Doom targets = %+v, want creatures %d and %d", got, mineA, mineB)
	}
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: meddle.ID, IDs: []state.ObjID{double.ID}})
	resolveUnderProbe(e, func() *effects.Ctx {
		return &effects.Ctx{Source: meddle.ID, Controller: 0, Targets: []state.Target{{Obj: double.ID}},
			TargetsOffered: true, SVars: meddle.Face().SVars}
	}, meddle.Face().SpellAbility())
	if d := e.Pending(); d != nil {
		t.Fatalf("Meddle asked over a two-target spell: %+v", d)
	}
	if got := e.G.Obj(double.ID).Targets; len(got) != 2 || got[0].Obj != mineA || got[1].Obj != mineB {
		t.Fatalf("Doom targets = %+v, want %d and %d untouched", got, mineA, mineB)
	}
}

// TestQuicksilverDragonRedirectsOnlyWhenItIsTheTarget: the ability's
// Card.Self spec -- the spell's one target must be the Dragon itself.
// Targeting the Dragon asks for another creature; targeting any other
// creature does nothing.
func TestQuicksilverDragonRedirectsOnlyWhenItIsTheTarget(t *testing.T) {
	t.Parallel()
	e := stealEngine(t, 747)
	dragon := onBoardCard(t, e, 0, choiceCorpusCard(t, "Quicksilver Dragon"))
	other := onBoard(t, e, 0, "Name:B\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	dobj := e.G.Obj(dragon)
	sa := dobj.Face().Abilities[0]
	if sa.API != "ChangeTargets" {
		t.Fatalf("precondition: Dragon ability = %s, want ChangeTargets", sa.API)
	}
	if _, ok := sa.Params["ConditionTargetValidTargeting"]; !ok {
		t.Fatal("precondition: Dragon ability carries no ConditionTargetValidTargeting$")
	}
	murder := e.G.AddObject(card(t, "Name:Murder\nTypes:Instant\nManaCost:B\nA:SP$ Destroy | ValidTgts$ Creature.OppCtrl\nOracle:x\n"), 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: murder.ID, From: state.ZLibrary, To: state.ZStack})
	// Case 1: Murder's one target is the Dragon.
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: murder.ID, IDs: []state.ObjID{dragon}})
	if got := e.G.Obj(murder.ID).Targets; len(got) != 1 || got[0].Obj != dragon {
		t.Fatalf("precondition: Murder targets = %+v, want the Dragon", got)
	}
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: dragon, IDs: []state.ObjID{murder.ID}})
	resolveUnderProbe(e, func() *effects.Ctx {
		return &effects.Ctx{Source: dragon, Controller: 0, Targets: []state.Target{{Obj: murder.ID}},
			TargetsOffered: true, SVars: dobj.Face().SVars}
	}, sa)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("the Dragon did not ask: %+v", d)
	}
	if offersObj(d, dragon) {
		t.Fatalf("offered the Dragon, Murder's current target: %+v", d.Options)
	}
	submitChoices(t, e, optionOfObj(t, d, other))
	if got := e.G.Obj(murder.ID).Targets; len(got) != 1 || got[0].Obj != other {
		t.Fatalf("Murder targets = %+v, want creature %d", got, other)
	}
	// Case 2: Murder's one target is another creature.
	murder2 := e.G.AddObject(card(t, "Name:Murder\nTypes:Instant\nManaCost:B\nA:SP$ Destroy | ValidTgts$ Creature.OppCtrl\nOracle:x\n"), 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: murder2.ID, From: state.ZLibrary, To: state.ZStack})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: murder2.ID, IDs: []state.ObjID{other}})
	if got := e.G.Obj(murder2.ID).Targets; len(got) != 1 || got[0].Obj != other {
		t.Fatalf("precondition: Murder targets = %+v, want creature %d", got, other)
	}
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: dragon, IDs: []state.ObjID{murder2.ID}})
	resolveUnderProbe(e, func() *effects.Ctx {
		return &effects.Ctx{Source: dragon, Controller: 0, Targets: []state.Target{{Obj: murder2.ID}},
			TargetsOffered: true, SVars: dobj.Face().SVars}
	}, sa)
	if d := e.Pending(); d != nil {
		t.Fatalf("the Dragon asked over a spell not targeting it: %+v", d)
	}
	if got := e.G.Obj(murder2.ID).Targets; len(got) != 1 || got[0].Obj != other {
		t.Fatalf("Murder targets = %+v, want creature %d untouched", got, other)
	}
}
