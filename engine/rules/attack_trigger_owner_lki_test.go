// An Attacks trigger's Defined$ TriggeredAttackerOwner is the OWNER of the
// attacking creature (CR 108.3), and that owner is still named when the
// attacker has left the battlefield before the trigger resolves: the owner
// comes from last known information (CR 608.2h). This is the second reader of
// the arm the TriggeredAttacker CR 400.7 drop must not erase.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const attackOwnerEnchantment = "Name:Attack Owner Test\nManaCost:1 B\nTypes:Enchantment\n" +
	"T:Mode$ Attacks | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ TrigDrain | TriggerDescription$ x\n" +
	"SVar:TrigDrain:DB$ LoseLife | Defined$ TriggeredAttackerOwner | LifeAmount$ 2\n" +
	"Oracle:x\n"

// TestAttackTriggerOwnerUsesGoneAttackerLKI: the bear attacks, dies while the
// trigger waits on the stack, and "that player loses 2 life" still resolves
// against the bear's owner. On the pre-fix arm the CR 400.7 drop leaves the
// owner set empty and the LoseLife silently skips.
func TestAttackTriggerOwnerUsesGoneAttackerLKI(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	bear := onBoardReady(t, e, 0, "Name:Owner Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	onBoard(t, e, 0, attackOwnerEnchantment)
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %d, want the one attack trigger", len(e.G.Stack))
	}
	if got := e.G.Players[0].Life; got != 20 {
		t.Fatalf("precondition: owner life = %d, want 20 before the trigger resolves", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	if e.G.Obj(bear).Zone != state.ZGraveyard {
		t.Fatalf("precondition: bear zone = %s, want graveyard", e.G.Obj(bear).Zone)
	}
	e.resolveTop()
	if len(e.G.Stack) != 0 {
		t.Fatalf("precondition: the attack trigger did not resolve")
	}
	if got := e.G.Players[0].Life; got != 18 {
		t.Fatalf("owner life = %d after resolution, want 18 (LoseLife 2 on TriggeredAttackerOwner from LKI, CR 608.2h)", got)
	}
}
