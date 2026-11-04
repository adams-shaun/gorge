package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kiraDrain is drainQueuedTriggers that answers a trigger-order ask (Kira's
// pump and the watcher can queue together).
func kiraDrain(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 50; i++ {
		answerTriggerOrders(t, e)
		if len(e.pendingTriggers) > 0 {
			e.putTriggersOnStack()
			continue
		}
		if len(e.G.Stack) == 0 {
			return
		}
		e.resolveTop()
	}
	t.Fatalf("trigger drain did not settle: %d pending, %d on the stack", len(e.pendingTriggers), len(e.G.Stack))
}

// kiraBoard seats Kira, Nerys Firebrand on seat 0's battlefield beside an
// attack-trigger watcher (Test Watcher: gain 1 life whenever a creature its
// controller controls attacks), then declares `attackers` Test Bears attacking
// seat 1 (0 = no combat, the step stays main phase 1). It returns the engine,
// its config and the top library card (the Test Entrant seed).
func kiraBoard(t *testing.T, attackers int) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	bear := "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	watcher := "Name:Test Watcher\nManaCost:1 G\nTypes:Creature Spirit\nPT:1/1\n" +
		"T:Mode$ Attacks | ValidCard$ Creature.YouCtrl | Execute$ TrigGain | TriggerZones$ Battlefield | TriggerDescription$ x\n" +
		"SVar:TrigGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	entrant := "Name:Test Entrant\nManaCost:1 G\nTypes:Creature Elf\nPT:3/3\nOracle:x\n"
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{
		lookup(t, reg, "Kira Nerys, Firebrand"), card(t, bear), card(t, watcher), card(t, entrant),
	}, nil)
	moveByName(t, e, 0, "Kira Nerys, Firebrand", state.ZBattlefield)
	moveByName(t, e, 0, "Test Watcher", state.ZBattlefield)
	var atk []state.ObjID
	if attackers > 0 {
		b := moveByName(t, e, 0, "Test Bear", state.ZBattlefield)
		atk = append(atk, b)
	}
	kiraDrain(t, e)
	if attackers > 0 {
		for _, id := range atk {
			e.G.Obj(id).SummonSick = false
		}
		e.G.Active = 0
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: atk})
		e.G.Step = state.StepDeclareAttackers
		kiraDrain(t, e)
		if o := e.G.Obj(atk[0]); !o.IsAttacking {
			t.Fatal("precondition: the Bear is not attacking")
		}
	}
	return e, cfg, e.G.Zone(state.ZLibrary, 0)[0]
}

// TestKiraGorge: Kira Nerys, Firebrand's replacement (CR 614.12): a creature
// its controller would put onto the battlefield while that player controls an
// attacking creature enters tapped and attacking instead (CR 508.4: it was
// never declared as an attacker, so no "whenever attacks" trigger fires). A
// token entering counts, the gate is read when the entry happens, and an
// opponent's creature is untouched.
func TestKiraGorge(t *testing.T) {
	t.Parallel()

	enter := func(t *testing.T, e *Engine, p state.PlayerID) *state.Object {
		t.Helper()
		id := moveByName(t, e, p, "Test Entrant", state.ZBattlefield)
		kiraDrain(t, e)
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: the entrant is not on the battlefield: %+v", o)
		}
		return o
	}

	t.Run("creature enters tapped and attacking", func(t *testing.T) {
		e, _, _ := kiraBoard(t, 1)
		life := e.G.Players[0].Life
		o := enter(t, e, 0)
		if !o.Tapped || !o.IsAttacking || o.Attacking != 1 {
			t.Fatalf("entrant tapped=%v attacking=%v defender=%d, want tapped and attacking seat 1", o.Tapped, o.IsAttacking, o.Attacking)
		}
		if e.G.Players[0].Life != life {
			t.Fatalf("an attack trigger fired for the entrant: life %d -> %d (CR 508.4)", life, e.G.Players[0].Life)
		}
	})

	t.Run("no attacker: enters normally", func(t *testing.T) {
		e, _, _ := kiraBoard(t, 0)
		o := enter(t, e, 0)
		if o.Tapped || o.IsAttacking {
			t.Fatalf("entrant tapped=%v attacking=%v, want untapped and not attacking with no attacker", o.Tapped, o.IsAttacking)
		}
	})

	t.Run("a token enters tapped and attacking", func(t *testing.T) {
		e, _, _ := kiraBoard(t, 1)
		want := e.G.NextID
		e.EmitTokenCreate(events.Event{Kind: events.TokenCreate, Player: 0, Text: "r_1_1_goblin"})
		kiraDrain(t, e)
		tok := e.G.Obj(want)
		if tok == nil || !tok.IsToken || tok.Zone != state.ZBattlefield {
			t.Fatalf("precondition: the token did not enter: %+v", tok)
		}
		if !tok.Tapped || !tok.IsAttacking || tok.Attacking != 1 {
			t.Fatalf("token tapped=%v attacking=%v defender=%d, want tapped and attacking seat 1", tok.Tapped, tok.IsAttacking, tok.Attacking)
		}
	})

	t.Run("a cast creature spell resolves tapped and attacking", func(t *testing.T) {
		e, _, _ := kiraBoard(t, 1)
		id := moveByName(t, e, 0, "Test Entrant", state.ZHand)
		e.G.Players[0].Pool[state.MG], e.G.Players[0].Pool[state.MC] = 1, 1
		castMode(t, e, id, "")
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZStack {
			t.Fatalf("precondition: the entrant is not on the stack: %+v", o)
		}
		kiraDrain(t, e)
		o := e.G.Obj(id)
		if o.Zone != state.ZBattlefield || !o.Tapped || !o.IsAttacking || o.Attacking != 1 {
			t.Fatalf("spell zone=%s tapped=%v attacking=%v defender=%d, want a tapped attacker on the battlefield", o.Zone, o.Tapped, o.IsAttacking, o.Attacking)
		}
	})

	t.Run("Kira herself entering with another attacker present", func(t *testing.T) {
		reg := testutil.CorpusRegistry(t)
		e, _ := corpusEngineCfg(t, reg, []*cards.Card{
			lookup(t, reg, "Kira Nerys, Firebrand"),
			card(t, "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
			card(t, "Name:Test Elder\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
		}, nil)
		moveByName(t, e, 0, "Test Elder", state.ZBattlefield) // filler creature
		b := moveByName(t, e, 0, "Test Bear", state.ZBattlefield)
		e.G.Obj(b).SummonSick = false
		e.G.Active = 0
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{b}})
		e.G.Step = state.StepDeclareAttackers
		k := moveByName(t, e, 0, "Kira Nerys, Firebrand", state.ZBattlefield)
		kiraDrain(t, e)
		o := e.G.Obj(k)
		if !o.Tapped || !o.IsAttacking || o.Attacking != 1 {
			t.Fatalf("Kira herself tapped=%v attacking=%v defender=%d, want tapped and attacking seat 1 (CR 614.12: her own entry sees her replacement)", o.Tapped, o.IsAttacking, o.Attacking)
		}
	})

	t.Run("an opponent's creature is not replaced", func(t *testing.T) {
		reg := testutil.CorpusRegistry(t)
		e, _ := corpusEngineCfg(t, reg, []*cards.Card{
			lookup(t, reg, "Kira Nerys, Firebrand"),
			card(t, "Name:Test Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"),
		}, []*cards.Card{card(t, "Name:Test Entrant\nManaCost:1 G\nTypes:Creature Elf\nPT:3/3\nOracle:x\n")})
		moveByName(t, e, 0, "Kira Nerys, Firebrand", state.ZBattlefield)
		b := moveByName(t, e, 0, "Test Bear", state.ZBattlefield)
		e.G.Obj(b).SummonSick = false
		e.G.Active = 0
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{b}})
		e.G.Step = state.StepDeclareAttackers
		o := enter(t, e, 1)
		if o.Tapped || o.IsAttacking {
			t.Fatalf("opponent's entrant tapped=%v attacking=%v, want it untouched", o.Tapped, o.IsAttacking)
		}
	})
}
