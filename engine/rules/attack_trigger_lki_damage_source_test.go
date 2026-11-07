// An attack trigger whose damage names DamageSource$ TriggeredAttackerLKICopy
// is dealt by the attacker even when that attacker left the battlefield
// before the trigger resolved: the source uses its last known information
// (CR 608.2h). The CR 400.7 drop on TriggeredAttacker applies only to the
// affected-object reads, never to the damage source.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const attackDamageEnchantment = "Name:Attack Shock Test\nManaCost:1 R\nTypes:Enchantment\n" +
	"T:Mode$ Attacks | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ TrigShock | TriggerDescription$ x\n" +
	"SVar:TrigShock:DB$ DealDamage | Defined$ Player.Opponent | DamageSource$ TriggeredAttackerLKICopy | NumDmg$ 1\n" +
	"Oracle:x\n"

func TestAttackTriggerDamageSourceIsGoneAttackerLKI(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	bear := onBoardReady(t, e, 0, "Name:Shock Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	ench := onBoard(t, e, 0, attackDamageEnchantment)
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %d, want the one attack trigger", len(e.G.Stack))
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	if e.G.Obj(bear).Zone != state.ZGraveyard {
		t.Fatalf("precondition: bear zone = %s, want graveyard", e.G.Obj(bear).Zone)
	}
	mark := len(e.L.Events)
	e.resolveTop()
	var srcs []state.ObjID
	for _, ev := range e.L.Events[mark:] {
		if ev.Kind == events.DamageProvenance {
			srcs = append(srcs, ev.Obj)
		}
	}
	if len(srcs) != 1 {
		t.Fatalf("precondition: %d damage provenance events, want 1 (the trigger dealt its damage)", len(srcs))
	}
	if srcs[0] == ench {
		t.Fatalf("damage source = the enchantment %d, want the gone attacker %d (CR 608.2h)", ench, bear)
	}
	if srcs[0] != bear {
		t.Fatalf("damage source = %d, want the gone attacker %d", srcs[0], bear)
	}
}
