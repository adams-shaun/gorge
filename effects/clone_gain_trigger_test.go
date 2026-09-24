package effects

// api:Clone GainThisAbility$ True under a TRIGGER: the copy must keep exactly
// the ROOT trigger of the resolving chain, not the become object's whole
// ability/trigger list and not nothing (ticket agent-20260923T090459Z-a4d7eb3b).
// Forge's CloneEffect.getCloneStates appends root.getTrigger().copy(...) when
// the root is a trigger; the DB$-under-trigger carriers (Cryptoplasm, Lazav,
// Crystalline Resonance, ...) are recurring copies whose trigger is what makes
// the copy happen again, so losing it silently ends the card's own future.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// cryptoplasmCarrier is Cryptoplasm's own compiled shape: an upkeep trigger
// whose Execute$ is the DB$ Clone body with GainThisAbility$ True.
const cryptoplasmCarrier = "Name:Cryptoplasm\n" +
	"ManaCost:1 U U\n" +
	"Types:Creature Shapeshifter\n" +
	"PT:2/2\n" +
	"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ CryptoplasmCopy | OptionalDecider$ You | TriggerDescription$ At the beginning of your upkeep, you may have CARDNAME become a copy of another target creature, except it has this ability.\n" +
	"SVar:CryptoplasmCopy:DB$ Clone | ValidTgts$ Creature.Other | TgtPrompt$ Select another target creature to copy. | Optional$ True | GainThisAbility$ True | AddSVars$ CryptoplasmCopy | AILogic$ CloneBestCreature\n" +
	"Oracle:x\n"

// cloneTriggerFixture puts the Cryptoplasm-like carrier (the become operand and
// the SA's source) and a Dragon-like copy target on seat 0's battlefield and
// returns (host, carrierID, dragonID). The carrier's single upkeep trigger is
// returned so the caller resolves the exact body the trigger points at -- the
// same pointer-identity contract rules' trigger machinery uses.
func cloneTriggerFixture(t *testing.T) (*fakeHost, state.ObjID, state.ObjID, *cards.SA) {
	t.Helper()
	h := &fakeHost{}
	h.g = state.NewGame(names(2))
	carrier := mkCard(t, cryptoplasmCarrier)
	dragon := mkCard(t, "Name:Dragon Hatchling\nManaCost:1 R\nTypes:Creature Dragon\nPT:0/1\nK:Flying\nOracle:x\n")
	carrierID := h.g.AddObject(carrier, 0).ID
	dragonID := h.g.AddObject(dragon, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrierID, dragonID})
	// SetZone moves the zone LIST only; the cached Zone field is what the
	// become-object check reads (the clone_optional_test.go convention).
	h.g.Obj(carrierID).Zone = state.ZBattlefield
	h.g.Obj(dragonID).Zone = state.ZBattlefield
	if h.g.Obj(carrierID).Zone != state.ZBattlefield || h.g.Obj(dragonID).Zone != state.ZBattlefield {
		t.Fatal("precondition: both clone operands must be on the battlefield")
	}
	if len(carrier.Faces[0].Triggers) != 1 || carrier.Faces[0].Triggers[0].Effect == nil {
		t.Fatalf("precondition: carrier must have exactly one compiled trigger with an Effect, got %+v", carrier.Faces[0].Triggers)
	}
	if len(carrier.Faces[0].Abilities) != 0 {
		t.Fatalf("precondition: the carrier must carry NO printed A: abilities (the trigger-only shape), got %d", len(carrier.Faces[0].Abilities))
	}
	return h, carrierID, dragonID, carrier.Faces[0].Triggers[0].Effect
}

// TestCloneGainThisAbilityTriggerRootRidesTheCopy pins the recurring-copy
// contract: resolving the carrier's trigger body copies the Dragon onto the
// carrier, and the copy face must carry exactly one trigger -- the DB body's
// own trigger -- while its Abilities stay exactly the copied face's (the
// Dragon's zero) and the name copies.
func TestCloneGainThisAbilityTriggerRootRidesTheCopy(t *testing.T) {
	h, carrierID, dragonID, body := cloneTriggerFixture(t)
	ctx := &Ctx{Controller: 0, Source: carrierID, Targets: []state.Target{{Obj: dragonID}}}
	Resolve(h, ctx, body)

	o := h.g.Obj(carrierID)
	f := o.Face()
	if f == nil {
		t.Fatal("precondition: the become object must still have a face after the copy")
	}
	if f.Name != "Dragon Hatchling" {
		t.Fatalf("copy name = %q, want the copied face's Dragon Hatchling (the copy must have happened)", f.Name)
	}
	if len(f.Triggers) != 1 {
		t.Fatalf("copy triggers = %d, want exactly 1 (the ROOT trigger must ride the copy); face: %+v", len(f.Triggers), f)
	}
	if f.Triggers[0].Effect != body {
		t.Fatalf("copy trigger Effect = %p, want the resolving DB body %p (the gained trigger must point back at the body so the NEXT copy can chain)", f.Triggers[0].Effect, body)
	}
	if len(f.Abilities) != 0 {
		t.Fatalf("copy abilities = %d, want the copied face's 0 (GainThisAbility$ must append only the root TRIGGER here); abilities: %+v", len(f.Abilities), f.Abilities)
	}
	// The event that carried the gain must name the new form, not the
	// already-fixed direct-ability form (whose index -1 appends nothing).
	var sawTriggerGain bool
	for _, e := range h.log {
		if e.Kind == events.ClonePermanent && e.Counter == "gain-this-trigger" {
			sawTriggerGain = true
			if e.Amount != 1 {
				t.Fatalf("gain-this-trigger Amount = %d, want 1 (the carrier's only trigger)", e.Amount)
			}
		}
	}
	if !sawTriggerGain {
		t.Fatalf("no ClonePermanent event with Counter gain-this-trigger; log: %+v", h.log)
	}
}

// TestCloneGainThisAbilityChainedCopyKeepsTheTrigger is the chained-copies
// contract: after the first copy, the permanent's face carries the gained
// trigger, so resolving the SAME trigger again (the next upkeep) must copy
// again and keep the trigger on the second copy. This is what makes a
// recurring Copy carrier recur.
func TestCloneGainThisAbilityChainedCopyKeepsTheTrigger(t *testing.T) {
	h, carrierID, dragonID, _ := cloneTriggerFixture(t)
	// First copy, resolved through the original printed trigger.
	first := h.g.Obj(carrierID).Face()
	if first == nil || len(first.Triggers) != 1 || first.Triggers[0].Effect == nil {
		t.Fatal("precondition: carrier's printed trigger must be present before the first copy")
	}
	Resolve(h, &Ctx{Controller: 0, Source: carrierID, Targets: []state.Target{{Obj: dragonID}}}, first.Triggers[0].Effect)

	// The copy face now owns the gained trigger; resolve THAT (the same body)
	// a second time, exactly as the next upkeep would.
	second := h.g.Obj(carrierID).Face()
	if second == nil || len(second.Triggers) != 1 {
		t.Fatalf("precondition: the first copy must carry the gained trigger, got %+v", second)
	}
	if second.Triggers[0].Effect == nil {
		t.Fatal("precondition: the gained trigger must point at the DB body")
	}
	// A second Dragon (the first is already the become object and is no
	// longer "another" creature for a fresh copy); the carrier is now a
	// Dragon Hatchling copy, so copy a different creature to make the second
	// copy observable.
	snake := mkCard(t, "Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	snakeID := h.g.AddObject(snake, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrierID, dragonID, snakeID})
	h.g.Obj(snakeID).Zone = state.ZBattlefield
	Resolve(h, &Ctx{Controller: 0, Source: carrierID, Targets: []state.Target{{Obj: snakeID}}}, second.Triggers[0].Effect)

	third := h.g.Obj(carrierID).Face()
	if third == nil {
		t.Fatal("precondition: the become object must still have a face")
	}
	if third.Name != "Grizzly Bears" {
		t.Fatalf("second copy name = %q, want Grizzly Bears (the chained copy must have happened)", third.Name)
	}
	if len(third.Triggers) != 1 {
		t.Fatalf("second copy triggers = %d, want exactly 1 (the gained trigger must keep chaining); face: %+v", len(third.Triggers), third)
	}
	if third.Triggers[0].Effect == nil {
		t.Fatalf("second copy trigger Effect is nil; want the DB body so a third copy is still possible")
	}
}

// kimahriCarrier is the TRIGGER-SUBCHAIN shape (Kimahri, Valiant Guardian):
// the trigger's Execute$ root is RonsoCounter, and the Clone body is reached
// through a SubAbility$ chain. Forge appends the ROOT trigger, so the copy
// must still gain the trigger -- a pointer scan that only compares the
// innermost sa with the face's abilities/triggers misses it entirely.
const kimahriCarrier = "Name:Fixture Ronso\n" +
	"ManaCost:1 G\n" +
	"Types:Creature Cat\n" +
	"PT:2/2\n" +
	"T:Mode$ Phase | Phase$ BeginCombat | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ Root | TriggerDescription$ At the beginning of combat, gain 1 life, then optionally copy another creature.\n" +
	"SVar:Root:DB$ GainLife | LifeAmount$ 1 | Defined$ You | SubAbility$ DeepBody\n" +
	"SVar:DeepBody:DB$ Clone | ValidTgts$ Creature.Other | GainThisAbility$ True\n" +
	"Oracle:x\n"

// TestCloneGainThisAbilityTriggerSubChainRootRidesTheCopy covers the
// trigger root reached through a SubAbility$ chain; it asserts the gained
// trigger is the ROOT trigger, not the innermost Clone body.
func TestCloneGainThisAbilityTriggerSubChainRootRidesTheCopy(t *testing.T) {
	h := &fakeHost{}
	h.g = state.NewGame(names(2))
	carrier := mkCard(t, kimahriCarrier)
	dragon := mkCard(t, "Name:Dragon Hatchling\nManaCost:1 R\nTypes:Creature Dragon\nPT:0/1\nK:Flying\nOracle:x\n")
	carrierID := h.g.AddObject(carrier, 0).ID
	dragonID := h.g.AddObject(dragon, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrierID, dragonID})
	h.g.Obj(carrierID).Zone = state.ZBattlefield
	h.g.Obj(dragonID).Zone = state.ZBattlefield
	root := carrier.Faces[0].Triggers[0].Effect
	if root == nil || root.Sub == nil || root.Sub.Sub != nil {
		t.Fatalf("precondition: the trigger's Execute body must be a >=2-deep Sub chain, got %+v", root)
	}
	if root.API == root.Sub.API {
		t.Fatal("precondition: the chain root and the Clone body must be different bodies")
	}
	Resolve(h, &Ctx{Controller: 0, Source: carrierID, Targets: []state.Target{{Obj: dragonID}}}, root)
	f := h.g.Obj(carrierID).Face()
	if f == nil || f.Name != "Dragon Hatchling" {
		t.Fatalf("copy name = %v, want Dragon Hatchling (the chained clone must have resolved)", f)
	}
	if len(f.Triggers) != 1 || f.Triggers[0].Effect != root {
		t.Fatalf("copy triggers = %+v, want exactly the ROOT trigger (Effect %p)", f.Triggers, root)
	}
	if len(f.Abilities) != 0 {
		t.Fatalf("copy abilities = %d, want the copied face's 0", len(f.Abilities))
	}
}

// volatileChimeraCarrier is the ABILITY-SUBCHAIN shape (Volatile Chimera,
// Dimir Doppelganger): an ACTIVATED ability's body reaches the Clone through
// SubAbility$, so the root is the activated ability itself and Forge appends
// root.copy(...) -- a spelled ability, not a trigger.
const volatileChimeraCarrier = "Name:Fixture Chimera\n" +
	"ManaCost:2 R\n" +
	"Types:Creature Chimera\n" +
	"PT:3/3\n" +
	"A:AB$ GainLife | Cost$ 1 R | LifeAmount$ 1 | Defined$ You | SubAbility$ ChimeraClone | SpellDescription$ Gain 1 life, then become a copy.\n" +
	"SVar:ChimeraClone:DB$ Clone | ValidTgts$ Creature | GainThisAbility$ True\n" +
	"Oracle:x\n"

// TestCloneGainThisAbilityAbilitySubChainRootRidesTheCopy covers the
// activated root reached through a SubAbility$ chain: the copy must gain the
// whole activated root ability (Forge's addSpellAbility(root.copy(...))), not
// nothing and not the innermost Clone body.
func TestCloneGainThisAbilityAbilitySubChainRootRidesTheCopy(t *testing.T) {
	h := &fakeHost{}
	h.g = state.NewGame(names(2))
	carrier := mkCard(t, volatileChimeraCarrier)
	dragon := mkCard(t, "Name:Dragon Hatchling\nManaCost:1 R\nTypes:Creature Dragon\nPT:0/1\nK:Flying\nOracle:x\n")
	carrierID := h.g.AddObject(carrier, 0).ID
	dragonID := h.g.AddObject(dragon, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{carrierID, dragonID})
	h.g.Obj(carrierID).Zone = state.ZBattlefield
	h.g.Obj(dragonID).Zone = state.ZBattlefield
	root := carrier.Faces[0].Abilities[0]
	if root.Sub == nil {
		t.Fatal("precondition: the activated root must reach the Clone body through a Sub")
	}
	Resolve(h, &Ctx{Controller: 0, Source: carrierID, Targets: []state.Target{{Obj: dragonID}}}, root)
	f := h.g.Obj(carrierID).Face()
	if f == nil || f.Name != "Dragon Hatchling" {
		t.Fatalf("copy name = %v, want Dragon Hatchling (the chained clone must have resolved)", f)
	}
	if len(f.Abilities) != 1 || f.Abilities[0].API != "GainLife" {
		t.Fatalf("copy abilities = %+v, want exactly the ROOT activated GainLife ability", f.Abilities)
	}
	if len(f.Triggers) != 0 {
		t.Fatalf("copy triggers = %d, want 0 (the root here is a spelled ability)", len(f.Triggers))
	}
}
