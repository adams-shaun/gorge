package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestStaticLandwalkChosenTypeGrant proves Traveler's Cloak's static
// `AddKeyword$ Landwalk:ChosenType`: the enchanted creature has landwalk of the
// type the AURA chose as it entered (CR 702.14), so it is unblockable only
// while the defending player controls a land of that type. The attacker's own
// chosen type is empty, so a literal "ChosenType" spec would never match.
func TestStaticLandwalkChosenTypeGrant(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	atk := onBoardReady(t, e, 1, "Name:Attacker\nManaCost:1\nTypes:Creature\nPT:2/2\nOracle:x\n")
	e.G.Obj(atk).IsAttacking, e.G.Obj(atk).Attacking = true, 0
	cloak := onBoardCard(t, e, 1, corpusCardAtPath(t, "t/travelers_cloak.txt"))
	e.emit(events.Event{Kind: events.Attach, Obj: cloak, IDs: []state.ObjID{atk}})
	e.emit(events.Event{Kind: events.Choose, Obj: cloak, Counter: "type", Text: "Swamp"})
	if o := e.G.Obj(cloak); o == nil || o.Zone != state.ZBattlefield || o.ChosenType != "Swamp" || o.AttachedTo != atk {
		t.Fatalf("precondition: cloak %+v not attached to the attacker with Swamp chosen", e.G.Obj(cloak))
	}
	if !e.HasKeyword(atk, "Landwalk") {
		t.Fatal("precondition: the enchanted creature carries no Landwalk keyword")
	}
	blocker := onBoard(t, e, 0, "Name:Blocker\nManaCost:1\nTypes:Creature\nPT:2/2\nOracle:x\n")
	onBoard(t, e, 0, "Name:For\nTypes:Basic Land Forest\nOracle:x\n")
	if !e.canBlock(blocker, atk) {
		t.Fatal("swamp-walk attacker was unblockable over a defender with only a Forest")
	}
	onBoard(t, e, 0, "Name:Swp\nTypes:Basic Land Swamp\nOracle:x\n")
	if e.canBlock(blocker, atk) {
		t.Fatal("swamp-walk attacker was blockable although the defender controls a Swamp")
	}
}
