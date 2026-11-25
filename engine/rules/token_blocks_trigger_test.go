package rules

// TokenBlocks becomes-blocked triggers (CR 509.3d): a creature put onto the
// battlefield blocking an attacker (TokenBlocking$, CR 509.4) triggers the
// attacker's "becomes blocked" abilities -- Brimaz, King of Oreskos's Cat
// Soldier enters blocking the attacker Brimaz blocked, and the attacker's
// "whenever this creature becomes blocked by a creature" fires for it. The
// token itself never blocked (CR 509.4), so nothing triggers on the token's
// own side.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// tokenBlocksSettle pushes the resolution's queued triggers onto the stack and
// resolves them, answering trigger-order asks with the offered order, until
// the stack and the pending queue are both empty. A player-facing ask that is
// not a trigger order (the CR 509.2 priority window this fixture parks in) is
// left for the caller: resolving the stack directly must never pass priority,
// which with an empty stack would cascade into combat damage.
func tokenBlocksSettle(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 40; i++ {
		if d := e.Pending(); d != nil && d.Kind == decision.KTriggerOrder {
			choices := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				choices = append(choices, o.Index)
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices}); err != nil {
				t.Fatalf("submit trigger order: %v", err)
			}
			continue
		}
		if len(e.G.Stack) > 0 {
			e.resolveTop()
			continue
		}
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		return
	}
	t.Fatal("triggers did not settle within the budget")
}

// TestTokenBlocksFiresBecomesBlockedByCreature is the filing card, end to end
// on Brimaz's real block trigger: seat 0 attacks with a bear carrying the
// inline "whenever CARDNAME becomes blocked by a creature, CARDNAME gets
// +1/+1" line (Forge Mode$ AttackerBlocked | ValidCard$ Card.Self |
// ValidBlocker$ Creature), seat 1 blocks with Brimaz, and Brimaz's block
// trigger mints the Cat Soldier blocking the attacker. The token's TokenBlocks
// entry is the second become-blocked transition (CR 509.3d), so the trigger
// fires twice -- once for Brimaz's declared block, once for the Cat's entry --
// and each instance pumps the attacker. The token's own side never fires
// (CR 509.4: it never blocked), so nothing is ever pushed with the token as
// its source.
func TestTokenBlocksFiresBecomesBlockedByCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const bearer = "Name:Blocked Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n" +
		"T:Mode$ AttackerBlocked | ValidCard$ Card.Self | ValidBlocker$ Creature | TriggerZones$ Battlefield | Execute$ TrigBlockedPump | TriggerDescription$ Whenever CARDNAME becomes blocked by a creature, CARDNAME gets +1/+1 until end of turn.\n" +
		"SVar:TrigBlockedPump:DB$ Pump | Defined$ Self | NumAtt$ 1 | NumDef$ 1\nOracle:x\n"
	e, cfg := combatTriggerBoard(t, reg,
		nil, []string{bearer},
		[]string{"Brimaz, King of Oreskos"}, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	atk := findBattlefield(t, e, 0, "Blocked Bear", 0)
	brimaz := findBattlefield(t, e, 1, "Brimaz, King of Oreskos", 0)

	// Preconditions the assertions depend on: the attacker is a live 2/2
	// carrying the become-blocked line, and Brimaz is a battlefield creature
	// whose own block trigger can mint the Cat Soldier.
	if o := e.G.Obj(atk); o == nil || o.Zone != state.ZBattlefield || e.Power(atk) != 2 || e.Toughness(atk) != 2 {
		t.Fatalf("precondition: attacker = %+v at %d/%d, want a live battlefield 2/2", o, e.Power(atk), e.Toughness(atk))
	}
	if o := e.G.Obj(brimaz); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Brimaz = %+v, want a battlefield creature", o)
	}

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockersOnly(t, e, brimaz)
	tokenBlocksSettle(t, e)

	// Brimaz's block trigger minted the Cat Soldier blocking the attacker
	// (the TokenBlocks fold), so the attacker is blocked by exactly two
	// creatures, Brimaz first.
	cat := tokenBlockingCat(e)
	if cat == 0 {
		t.Fatal("Brimaz's block trigger never minted the Cat Soldier token")
	}
	if o := e.G.Obj(cat); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("Cat Soldier = %+v, want a battlefield token under seat 1", o)
	}
	if !e.G.Obj(cat).IsBlocking {
		t.Fatal("Cat Soldier IsBlocking=false, want true (the token is a blocking creature, CR 509.4a)")
	}
	if got := e.G.Obj(atk).BlockedBy; len(got) != 2 || got[0] != brimaz || got[1] != cat {
		t.Fatalf("attacker BlockedBy = %v, want [%d %d] (Brimaz then the Cat)", got, brimaz, cat)
	}

	// The rule: BOTH become-blocked transitions fired the attacker's trigger,
	// each to one +1/+1 pump -- the declared block of Brimaz, then the Cat's
	// entry (CR 509.3d).
	if n := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.TriggerPush && ev.Obj == atk
	}); n != 2 {
		t.Fatalf("become-blocked trigger instances = %d, want 2 (declared block + the Cat's entry)", n)
	}
	if e.Power(atk) != 4 || e.Toughness(atk) != 4 {
		t.Fatalf("attacker = %d/%d, want 4/4 (two +1/+1 pumps, one per transition)", e.Power(atk), e.Toughness(atk))
	}

	// The token's own side stayed inert (CR 509.4): nothing was ever pushed
	// with the token as its source.
	if n := countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.TriggerPush && ev.Obj == cat
	}); n != 0 {
		t.Fatalf("trigger instances pushed with the token as source = %d, want 0 (the token never blocked)", n)
	}
	replayCheck(t, e, cfg)
}

// TestTokenBlocksFiresBecomesBlockedByCreaturePairMode pins the pair-mode
// spelling (Forge Mode$ AttackerBlockedByCreature, CR 509.3d's "becomes
// blocked by a creature"): the same Brimaz scenario, with the attacker's line
// debuffing TriggeredBlockerLKICopy instead. Brimaz's declared block debuffs
// Brimaz (3/4 -> 2/3), and the Cat Soldier's entry debuffs the token
// (1/1 -> 0/0).
func TestTokenBlocksFiresBecomesBlockedByCreaturePairMode(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const bearer = "Name:Debuffing Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n" +
		"T:Mode$ AttackerBlockedByCreature | ValidCard$ Card.Self | ValidBlocker$ Creature | TriggerZones$ Battlefield | Execute$ TrigBlockedDebuff | TriggerDescription$ Whenever CARDNAME becomes blocked by a creature, that creature gets -1/-1 until end of turn.\n" +
		"SVar:TrigBlockedDebuff:DB$ Pump | Defined$ TriggeredBlockerLKICopy | NumAtt$ -1 | NumDef$ -1\nOracle:x\n"
	e, _ := combatTriggerBoard(t, reg,
		nil, []string{bearer},
		[]string{"Brimaz, King of Oreskos"}, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	atk := findBattlefield(t, e, 0, "Debuffing Bear", 0)
	brimaz := findBattlefield(t, e, 1, "Brimaz, King of Oreskos", 0)

	if o := e.G.Obj(atk); o == nil || o.Zone != state.ZBattlefield || e.Power(atk) != 2 || e.Toughness(atk) != 2 {
		t.Fatalf("precondition: attacker = %+v at %d/%d, want a live battlefield 2/2", o, e.Power(atk), e.Toughness(atk))
	}
	if o := e.G.Obj(brimaz); o == nil || o.Zone != state.ZBattlefield || e.Power(brimaz) != 3 || e.Toughness(brimaz) != 4 {
		t.Fatalf("precondition: Brimaz = %+v at %d/%d, want a live 3/4", o, e.Power(brimaz), e.Toughness(brimaz))
	}

	e.askAttackers()
	submitAttackers(t, e, atk)
	submitBlockersOnly(t, e, brimaz)
	tokenBlocksSettle(t, e)

	// The declared block of Brimaz debuffed Brimaz (the control arm), and the
	// Cat Soldier's entry into blocking debuffed the token itself (CR 509.3d).
	if e.Power(brimaz) != 2 || e.Toughness(brimaz) != 3 {
		t.Fatalf("Brimaz = %d/%d, want 2/3 (the declared-block instance named it)", e.Power(brimaz), e.Toughness(brimaz))
	}
	cat := tokenBlockingCat(e)
	if cat == 0 {
		t.Fatal("Brimaz's block trigger never minted the Cat Soldier token")
	}
	if e.Power(cat) != 0 || e.Toughness(cat) != 0 {
		t.Fatalf("Cat Soldier = %d/%d, want 0/0 (the token-entry instance named it)", e.Power(cat), e.Toughness(cat))
	}
}
