package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 509.3d: a creature put onto the battlefield blocking the Ring-bearer
// (TokenBlocking$, CR 509.4) is a become-blocked transition, so the Ring
// emblem's level 3 ("whenever your Ring-bearer becomes blocked by a creature,
// that creature's controller sacrifices it at end of combat") fires for the
// token as well as for the declared blocker.

// ringBearerBear is a 4/4 so Brimaz (power 3) may block it despite the
// Ring's level-1 "can't be blocked by creatures with greater power".
const ringBearerBear = "Name:Ring Bear\nManaCost:2 G\nTypes:Creature Bear\nPT:4/4\nOracle:x\n"

// TestRingEmblemLevel3FiresOnTokenBlocks: seat 0's lone creature is the
// Ring-bearer (three temptations), seat 1 blocks it with Brimaz, and Brimaz's
// block trigger mints a Cat Soldier blocking it. Both blocking creatures queue
// one level-3 ability and both are sacrificed at end of combat.
func TestRingEmblemLevel3FiresOnTokenBlocks(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := combatTriggerBoard(t, reg,
		nil, []string{ringBearerBear},
		[]string{"Brimaz, King of Oreskos"}, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	atk := findBattlefield(t, e, 0, "Ring Bear", 0)
	brimaz := findBattlefield(t, e, 1, "Brimaz, King of Oreskos", 0)
	temptByBirthdayEscape(t, e, reg, 0, 3)

	// Preconditions: the attacker is the Ring-bearer at level 3, and no
	// emblem ability has been pushed yet.
	if got := e.G.Players[0].RingBearer; got != atk {
		t.Fatalf("precondition: Ring-bearer = %d, want the attacker %d", got, atk)
	}
	if got := e.G.Players[0].RingTempted; got != 3 {
		t.Fatalf("precondition: RingTempted = %d, want 3", got)
	}
	if n := ringEmblemPushes(e); n != 0 {
		t.Fatalf("precondition: %d emblem pushes before combat, want 0", n)
	}

	e.askAttackers()
	submitAttackersOnly(t, e, atk)
	// Level 2 loots on the attack: resolve it, discarding the first offer,
	// until the blockers decision arrives.
	for i := 0; i < 20; i++ {
		if d := e.Pending(); d != nil && d.Kind == decision.KBlockers {
			break
		}
		if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
			if d.Kind == decision.KTriggerOrder {
				answerTriggerOrders(t, e)
				continue
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
				t.Fatalf("answer %v: %v", d.Kind, err)
			}
			continue
		}
		if len(e.G.Stack) > 0 {
			e.resolveTop()
		} else if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
		} else {
			drainCombatPriority(t, e)
		}
	}
	lootPushes := ringEmblemPushes(e) // the level-2 loot ability
	if lootPushes != 1 {
		t.Fatalf("precondition: %d emblem pushes after the attack, want 1 (the loot)", lootPushes)
	}
	submitBlockersOnly(t, e, brimaz)
	tokenBlocksSettle(t, e)

	cat := tokenBlockingCat(e)
	if cat == 0 {
		t.Fatal("Brimaz's block trigger never minted the Cat Soldier token")
	}
	if got := e.G.Obj(atk).BlockedBy; len(got) != 2 || got[0] != brimaz || got[1] != cat {
		t.Fatalf("attacker BlockedBy = %v, want [%d %d]", got, brimaz, cat)
	}
	// One level-3 ability per blocking creature: Brimaz's declared block and
	// the Cat's entry (CR 509.3d).
	if n := ringEmblemPushes(e) - lootPushes; n != 2 {
		t.Fatalf("level-3 emblem abilities = %d, want 2 (declared blocker + the token)", n)
	}

	e.emit(events.Event{Kind: events.StepChange, Step: state.StepEndCombat})
	tokenBlocksSettle(t, e)
	for _, id := range []state.ObjID{brimaz, cat} {
		if o := e.G.Obj(id); o == nil || o.Zone == state.ZBattlefield {
			t.Fatalf("blocker %d still on the battlefield after end of combat, want sacrificed", id)
		}
		if countEvents(e, func(ev events.Event) bool { return events.IsSacrifice(ev) && ev.Obj == id }) != 1 {
			t.Fatalf("blocker %d was not sacrificed exactly once", id)
		}
	}
	if o := e.G.Obj(atk); o.Zone != state.ZBattlefield {
		t.Fatalf("the Ring-bearer left the battlefield (zone %v)", o.Zone)
	}
}
