package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// protoCloneSrc is a prototype creature that also carries an UntilEndOfTurn
// copy ability, so a permanent copy of it can lay a second, newer copy layer.
const protoCloneSrc = "Name:Fixture Protomimic\nManaCost:6\nTypes:Artifact Creature Golem\nPT:5/5\n" +
	"K:Prototype:1 W W:3:3\n" +
	"A:AB$ Clone | Cost$ 1 | ValidTgts$ Creature | Duration$ UntilEndOfTurn | SpellDescription$ becomes a copy until end of turn.\n" +
	"Oracle:x\n"

// CR 707.2b + 718.3c: a permanent copy of a prototyped permanent is
// prototyped; when a newer copy layer ends, the re-base onto the locked copy
// keeps that prototype status (3/3), not the printed 5/5.
func TestCloneRebaseKeepsThePrototypeStatus(t *testing.T) {
	e, cfg, proto := newFixtureDeck(t, 1001, protoCloneSrc, twoCloneAbilities, cloneBruiserSrc)
	addToHand(t, e, 0, protoCloneSrc)
	addMana(t, e, 0, "WWC")
	submitChoices(t, e, prototypeCastOption(t, e, proto, "prototype").Index)
	e.resolveTop()
	if o := e.G.Obj(proto); o.Zone != state.ZBattlefield || o.CastFlags&state.FlagPrototype == 0 {
		t.Fatalf("protomimic %v flags %#x, want a prototyped battlefield permanent", o.Zone, o.CastFlags)
	}
	mimic := moveSeeded(t, e, 0, twoCloneAbilities, state.ZBattlefield)
	bruiser := moveSeeded(t, e, 0, cloneBruiserSrc, state.ZBattlefield)

	addMana(t, e, 0, "C")
	activateCloneAbilityIdx(t, e, mimic, proto, 0) // unit A, permanent: a prototyped 3/3
	if d := e.Derived(mimic); d.Power != 3 || d.Toughness != 3 {
		t.Fatalf("mimic copy of the prototyped permanent is %d/%d, want 3/3", d.Power, d.Toughness)
	}
	addMana(t, e, 0, "C")
	activateCloneAbilityIdx(t, e, mimic, bruiser, 0) // unit B, EOT: the copied ability
	if o := e.G.Obj(mimic); o.Face().Name != "Fixture Bruiser" || o.CastFlags&state.FlagPrototype != 0 {
		t.Fatalf("mimic before cleanup %v flags %#x, want an unprototyped Fixture Bruiser", o.Face(), o.CastFlags)
	}
	e.pending = nil
	e.setStep(state.StepCleanup)
	e.priorityRound()
	o := e.G.Obj(mimic)
	if o.Face() == nil || o.Face().Name != "Fixture Protomimic" {
		t.Fatalf("mimic re-based onto %v, want the locked Fixture Protomimic", o.Face())
	}
	if d := e.Derived(mimic); d.Power != 3 || d.Toughness != 3 {
		t.Fatalf("mimic re-based to %d/%d (flags %#x), want the prototyped 3/3 (CR 718.3c)", d.Power, d.Toughness, o.CastFlags)
	}
	replayCheck(t, e, cfg)
}
