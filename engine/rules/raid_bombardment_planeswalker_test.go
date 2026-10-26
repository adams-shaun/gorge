package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// raidBombardmentSrc is Raid Bombardment's trigger: its Defined$ unions the
// player defender ("TriggeredDefender.Opponent") with the planeswalker one
// ("Valid Planeswalker.TriggeredDefender").
const raidBombardmentSrc = "Name:Raid Bombardment\nManaCost:2 R\nTypes:Enchantment\n" +
	"T:Mode$ Attacks | ValidCard$ Creature.powerLE2+YouCtrl | TriggerZones$ Battlefield | Execute$ TrigDamage | TriggerDescription$ x\n" +
	"SVar:TrigDamage:DB$ DealDamage | Defined$ TriggeredDefender.Opponent & Valid Planeswalker.TriggeredDefender | NumDmg$ 1\n" +
	"Oracle:x\n"

const raidBombardmentAttacker = "Name:Raid Probe\nManaCost:1 R\nTypes:Creature Goblin\nPT:2/2\nOracle:x\n"

// submitRaidAttack submits the one KAttackers option naming attacker at the
// given defender (battle 0 = the seat itself) through Submit, so the attack
// triggers queue and the decision is answered.
func submitRaidAttack(t *testing.T, e *Engine, attacker state.ObjID, seat state.PlayerID, battle state.ObjID) {
	t.Helper()
	e.askAttackers()
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("declare attackers decision = %+v", d)
	}
	for _, opt := range d.Options {
		if opt.Obj == attacker && opt.Player == seat && opt.Battle == battle {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{opt.Index}}); err != nil {
				t.Fatalf("submit attack: %v", err)
			}
			o := e.G.Obj(attacker)
			if !o.IsAttacking || o.Attacking != seat || o.AttackingBattle != battle {
				t.Fatalf("attacker state = %+v, want attacking seat %d battle %d", o, seat, battle)
			}
			return
		}
	}
	t.Fatalf("no attack option for %d at seat %d battle %d: %+v", attacker, seat, battle, d.Options)
}

func TestRaidBombardmentHitsAttackedPlaneswalker(t *testing.T) {
	t.Parallel()
	t.Run("planeswalker", func(t *testing.T) {
		e := combatEngine(t)
		onBoard(t, e, 0, raidBombardmentSrc)
		attacker := onBoardReady(t, e, 0, raidBombardmentAttacker)
		pw := onBoard(t, e, 1, combatPlaneswalker)
		e.emit(events.Event{Kind: events.CounterChange, Obj: pw, Counter: "LOYALTY", Amount: 5})
		if got := e.G.Obj(pw).Counter("LOYALTY"); got != 5 {
			t.Fatalf("precondition: loyalty = %d, want 5", got)
		}
		submitRaidAttack(t, e, attacker, 1, pw)
		if n := len(e.pendingTriggers) + len(e.G.Stack); n != 1 {
			t.Fatalf("queued/stacked %d triggers, want 1 Raid Bombardment instance", n)
		}
		life := e.G.Players[1].Life
		drainTriggers(t, e)
		if got := e.G.Obj(pw).Counter("LOYALTY"); got != 4 {
			t.Fatalf("planeswalker loyalty = %d, want 4", got)
		}
		if got := e.G.Players[1].Life; got != life {
			t.Fatalf("defending player life = %d, want unchanged %d", got, life)
		}
	})
	t.Run("player", func(t *testing.T) {
		e := combatEngine(t)
		onBoard(t, e, 0, raidBombardmentSrc)
		attacker := onBoardReady(t, e, 0, raidBombardmentAttacker)
		pw := onBoard(t, e, 1, combatPlaneswalker)
		e.emit(events.Event{Kind: events.CounterChange, Obj: pw, Counter: "LOYALTY", Amount: 5})
		submitRaidAttack(t, e, attacker, 1, 0)
		if n := len(e.pendingTriggers) + len(e.G.Stack); n != 1 {
			t.Fatalf("queued/stacked %d triggers, want 1 Raid Bombardment instance", n)
		}
		life := e.G.Players[1].Life
		drainTriggers(t, e)
		if got := e.G.Players[1].Life; got != life-1 {
			t.Fatalf("defending player life = %d, want %d", got, life-1)
		}
		if got := e.G.Obj(pw).Counter("LOYALTY"); got != 5 {
			t.Fatalf("unattacked planeswalker loyalty = %d, want 5", got)
		}
	})
}
