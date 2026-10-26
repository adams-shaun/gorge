package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g195: DealDamage's NoPrevention$ True ("the damage can't be prevented",
// CR 615.12) is published around each Damage event the ability emits, and
// cantPreventDamage honours it for protection (CR 702.16e) and prevention
// shields alike. These fixtures copy the printed lines of Combust, Lava Burst
// and Volcano Hellion (the Hellion without its ChooseNumber head) and
// Lightning Bolt for the control.

const (
	npCombust = "Name:NPCombust\nManaCost:1 R\nTypes:Instant\n" +
		"A:SP$ DealDamage | ValidTgts$ Creature.White,Creature.Blue | NumDmg$ 5 | NoPrevention$ True\nOracle:x\n"
	npBolt = "Name:NPBolt\nManaCost:R\nTypes:Instant\n" +
		"A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 3\nOracle:x\n"
	npLavaBurst = "Name:NPLavaBurst\nManaCost:R\nTypes:Sorcery\n" +
		"A:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 4 | NoPrevention$ True\nOracle:x\n"
	npShield = "Name:NPShield\nManaCost:W\nTypes:Instant\n" +
		"A:SP$ PreventDamage | ValidTgts$ Any | Amount$ 5\nOracle:x\n"
	npHellion = "Name:NPHellion\nManaCost:2 R R\nTypes:Creature Hellion\nPT:6/5\n" +
		"SVar:DBDmg:DB$ DealDamage | ValidTgts$ Creature | NumDmg$ 3 | NoPrevention$ True | DamageMap$ True | SubAbility$ DmgYou\n" +
		"SVar:DmgYou:DB$ DealDamage | Defined$ You | NumDmg$ 3 | NoPrevention$ True | SubAbility$ DmgResolve\n" +
		"SVar:DmgResolve:DB$ DamageResolve\nOracle:x\n"
	npWhiteProRed = "Name:NPKnight\nManaCost:1 W\nTypes:Creature Human Knight\nPT:2/3\nK:Protection from red\nOracle:x\n"
)

// npResolve resolves id's spell ability (or, with svar != "", that SVar's
// chain) the way a cast-and-resolve would, naming the targets directly: a
// protected creature is never a legal target for the red spell, so the
// fixture forces the recipient (the damage-time check is what is under test).
func npResolve(t *testing.T, e *Engine, id state.ObjID, svar string, targets ...state.Target) {
	t.Helper()
	face := e.G.Obj(id).Face()
	sa := face.SpellAbility()
	if svar != "" {
		sa = cards.ResolveSVar(face.SVars, svar)
	}
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0, SVars: face.SVars,
		Targets: targets, TargetsOffered: true}, sa)
}

func TestNoPreventionCombustThroughProtection(t *testing.T) {
	t.Parallel()
	e, cfg, combust := newFixtureDeck(t, 71, npCombust, npWhiteProRed)
	knight := putCreature(t, e, 0, npWhiteProRed)
	if !e.protectedFrom(knight, combust) {
		t.Fatal("precondition: the knight must have protection from the red Combust")
	}
	npResolve(t, e, combust, "", state.Target{Obj: knight})
	// 5 damage on a 2/3 is lethal; the state-based action runs when
	// priority next opens, so the marked damage is the landed fact here.
	if o := e.G.Obj(knight); o.Damage != 5 {
		t.Fatalf("knight damage %d, want 5: Combust's damage can't be prevented", o.Damage)
	}
	replayCheck(t, e, cfg)
}

func TestPreventionStillAppliesWithoutKey(t *testing.T) {
	t.Parallel()
	e, cfg, bolt := newFixtureDeck(t, 72, npBolt, npWhiteProRed)
	knight := putCreature(t, e, 0, npWhiteProRed)
	if !e.protectedFrom(knight, bolt) {
		t.Fatal("precondition: the knight must have protection from the red bolt")
	}
	npResolve(t, e, bolt, "", state.Target{Obj: knight})
	if o := e.G.Obj(knight); o.Zone != state.ZBattlefield || o.Damage != 0 {
		t.Fatalf("knight zone %s damage %d, want untouched: no NoPrevention$ key, protection prevents", o.Zone, o.Damage)
	}
	replayCheck(t, e, cfg)
}

func TestNoPreventionIgnoresPreventionShield(t *testing.T) {
	t.Parallel()
	e, cfg, shield := newFixtureDeck(t, 73, npShield, npLavaBurst)
	burst := addToHand(t, e, 0, npLavaBurst)
	npResolve(t, e, shield, "", state.Target{Player: 1, IsPlayer: true})
	if got := countShieldCEs(e); got != 1 {
		t.Fatalf("precondition: shields = %d, want 1 on seat 1", got)
	}
	npResolve(t, e, burst, "", state.Target{Player: 1, IsPlayer: true})
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("seat 1 life = %d, want 16: Lava Burst's 4 damage can't be prevented", got)
	}
	if got := countShieldCEs(e); got != 1 {
		t.Fatalf("shields = %d, want the untouched shield still in place", got)
	}
	replayCheck(t, e, cfg)
}

func TestNoPreventionVolcanoHellionDamageMap(t *testing.T) {
	t.Parallel()
	e, cfg, hellion := newFixtureDeck(t, 74, npHellion, npWhiteProRed)
	knight := putCreature(t, e, 0, npWhiteProRed)
	if !e.protectedFrom(knight, hellion) {
		t.Fatal("precondition: the knight must have protection from the red Hellion")
	}
	life := e.G.Players[0].Life
	npResolve(t, e, hellion, "DBDmg", state.Target{Obj: knight})
	if o := e.G.Obj(knight); o.Damage != 3 {
		t.Fatalf("knight damage %d, want 3 from the flushed DamageMap entry", o.Damage)
	}
	if got := e.G.Players[0].Life; got != life-3 {
		t.Fatalf("seat 0 life %d, want %d: the self-damage lands in the same flush", got, life-3)
	}
	replayCheck(t, e, cfg)
}
