package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestEquipAttachesAndTheStaticFollowsTheBearer(t *testing.T) {
	sword := "Name:Sword\nManaCost:3\nTypes:Artifact Equipment\nK:Equip:2\n" +
		"S:Mode$ Continuous | Affected$ Creature.EquippedBy | AddPower$ 2 | AddToughness$ 2 | AddKeyword$ Vigilance | Description$ x\nOracle:x\n"
	e, cfg, sw := newFixtureDeck(t, 61, sword, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: sw, From: state.ZHand, To: state.ZBattlefield})
	bear := putCreature(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	addMana(t, e, 0, "GG")
	e.Advance()
	opt := abilityOption(t, e, sw, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || len(d.Options) != 1 || d.Options[0].Obj != bear {
		t.Fatalf("equip target %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(sw).AttachedTo != bear || e.Power(bear) != 4 || e.Toughness(bear) != 4 || !e.HasKeyword(bear, "Vigilance") {
		t.Fatalf("attached to %d, bear power %d tough %d vig %v", e.G.Obj(sw).AttachedTo, e.Power(bear), e.Toughness(bear), e.HasKeyword(bear, "Vigilance"))
	}
	// The bearer dies: the Equipment stays on the battlefield, detached.
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.checkStateBased()
	if e.G.Obj(sw).Zone != state.ZBattlefield || e.G.Obj(sw).AttachedTo != 0 {
		t.Fatalf("sword %s attached %d", e.G.Obj(sw).Zone, e.G.Obj(sw).AttachedTo)
	}
	replayCheck(t, e, cfg)
}

func TestAuraTargetsOnCastAttachesOnResolutionAndDiesWithItsBearer(t *testing.T) {
	rancor := "Name:Rancor\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ Continuous | Affected$ Creature.EnchantedBy | AddPower$ 2 | AddKeyword$ Trample | Description$ x\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ TrigChangeZone | TriggerDescription$ x\n" +
		"SVar:TrigChangeZone:DB$ ChangeZone | Origin$ Graveyard | Destination$ Hand | Defined$ TriggeredCardLKICopy\nOracle:x\n"
	e, cfg, ra := newFixtureDeck(t, 62, rancor)
	bear := putToken(t, e, 1, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", state.ZBattlefield)
	addMana(t, e, 0, "G")
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Options[0].Obj != bear {
		t.Fatalf("aura target %+v", d)
	}
	submitChoices(t, e, 0)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(ra).Zone != state.ZBattlefield || e.G.Obj(ra).AttachedTo != bear || e.Power(bear) != 4 || !e.HasKeyword(bear, "Trample") {
		t.Fatalf("rancor %s on %d; bear power %d trample %v", e.G.Obj(ra).Zone, e.G.Obj(ra).AttachedTo, e.Power(bear), e.HasKeyword(bear, "Trample"))
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	e.checkStateBased()
	// passUntilStackEmpty above left seat 0 holding a stale priority decision
	// (the active player's, once the Rancor resolved off the now-empty
	// stack), so e.Advance() would not run a single step -- the Rancor's
	// return-to-hand trigger, queued by the checkStateBased above, would
	// never be drained. Clearing the stale pending is the same net state the
	// live Submit loop reaches (cast_test.go and friends do the same
	// e.pending = nil to re-drive after a run of stack-draining passes).
	e.pending = nil
	e.Advance()
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(ra).Zone != state.ZHand {
		t.Fatalf("Rancor should have died and returned to hand, is in %s", e.G.Obj(ra).Zone)
	}
	replayCheck(t, e, cfg)
}

func TestLivingWeaponCreatesAGermAndAttaches(t *testing.T) {
	skull := "Name:Skull\nManaCost:5\nTypes:Artifact Equipment\nK:Living Weapon\nK:Equip:5\n" +
		"S:Mode$ Continuous | Affected$ Creature.EquippedBy | AddPower$ 4 | AddToughness$ 4 | Description$ x\nOracle:x\n"
	e, cfg, sk := newFixtureDeckWithTokens(t, 63, skull)
	addMana(t, e, 0, "GGGGG")
	castFirst(t, e, "cast")
	passUntilStackEmpty(t, e, 30)
	bf := e.G.Zone(state.ZBattlefield, 0)
	var germ state.ObjID
	for _, id := range bf {
		if e.G.Obj(id).IsToken {
			germ = id
		}
	}
	if germ == 0 || e.G.Obj(sk).AttachedTo != germ || e.Power(germ) != 4 || e.Toughness(germ) != 4 {
		t.Fatalf("germ %d attached %d power %d tough %d", germ, e.G.Obj(sk).AttachedTo, e.Power(germ), e.Toughness(germ))
	}
	replayCheck(t, e, cfg)
}

func TestIllegalAttachmentsAreCleanedUp(t *testing.T) {
	e, _, aura := newFixtureDeck(t, 64, "Name:Aura\nManaCost:G\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n")
	e.emit(events.Event{Kind: events.MoveZone, Obj: aura, From: state.ZHand, To: state.ZBattlefield}) // attached to nothing
	e.checkStateBased()
	if e.G.Obj(aura).Zone != state.ZGraveyard {
		t.Fatal("an Aura attached to nothing must go to the graveyard")
	}
}
