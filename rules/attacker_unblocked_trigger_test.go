package rules

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func unblockedCorpusCard(t *testing.T, path string) *cards.Card {
	t.Helper()
	c, ds := cards.Parse(filepath.Join("..", ".cards", "cardsfolder", path))
	if len(ds) != 0 {
		t.Fatalf("parse %s: %v", path, ds)
	}
	if ds = c.Link(); len(ds) != 0 {
		t.Fatalf("link %s: %v", path, ds)
	}
	return c
}

func TestSwampMosquitoUnblockedAttackPoisonsDefender(t *testing.T) {
	e := combatEngine(t)
	mosquito := onBoardCard(t, e, 0, unblockedCorpusCard(t, "s/swamp_mosquito.txt"))
	e.G.Obj(mosquito).SummonSick = false
	onBoard(t, e, 1, "Name:Memnite\nManaCost:0\nTypes:Artifact Creature\nK:Flying\nPT:1/1\nOracle:x\n")
	e.askAttackers()
	submitAttackersOnly(t, e, mosquito)
	drainCombatPriority(t, e)
	before := e.G.Players[1].Counter("POISON")
	clone := e.Clone()
	for _, eng := range []*Engine{e, clone} {
		declareUnblocked(t, eng)
		eng.putTriggersOnStack()
		if len(eng.G.Stack) != 1 {
			t.Fatalf("stack = %d, want one trigger", len(eng.G.Stack))
		}
		if got := eng.G.Players[1].Counter("POISON"); got != before {
			t.Fatalf("poison changed before resolution: %d", got)
		}
		if len(eng.pendingTriggers) != 0 && eng.pendingTriggers[0].Ctx.TriggerContext.DefendingPlayer.Player != 1 {
			t.Fatal("queued poison trigger captured the wrong defender")
		}
	}
	if got := len(e.G.Stack); got != 1 {
		t.Fatalf("trigger stack = %d, want 1", got)
	}
	if !reflect.DeepEqual(e.L.Events, clone.L.Events) {
		t.Fatal("clone diverged")
	}
}

func TestStinkdrinkerBanditPumpsEachUnblockedRogue(t *testing.T) {
	e := combatEngine(t)
	bandit := onBoardCard(t, e, 0, unblockedCorpusCard(t, "s/stinkdrinker_bandit.txt"))
	r1 := onBoardReady(t, e, 0, "Name:Rogue One\nManaCost:0\nTypes:Creature Rogue\nPT:1/1\nOracle:x\n")
	r2 := onBoardReady(t, e, 0, "Name:Rogue Two\nManaCost:0\nTypes:Creature Rogue\nPT:2/2\nOracle:x\n")
	onBoard(t, e, 1, "Name:Memnite\nManaCost:0\nTypes:Artifact Creature\nPT:1/1\nOracle:x\n")
	if e.Power(r1) != 1 || e.Power(r2) != 2 {
		t.Fatal("rogue precondition failed")
	}
	e.askAttackers()
	submitAttackersOnly(t, e, r1, r2)
	drainCombatPriority(t, e)
	declareUnblocked(t, e)
	e.putTriggersOnStack()
	answerTriggerOrders(t, e)
	if len(e.G.Stack) != 2 {
		t.Fatalf("stack = %d, want two trigger instances", len(e.G.Stack))
	}
	if len(e.G.Stack) != 2 {
		t.Fatalf("trigger stack = %d, want 2", len(e.G.Stack))
	}
	_ = bandit
}

func TestAttackerUnblockedValidCardAndDefenderGates(t *testing.T) {
	e := combatEngine(t)
	source := onBoard(t, e, 1, "Name:Watcher\nTypes:Creature\nPT:1/1\nT:Mode$ AttackerUnblocked | ValidCard$ Rogue | ValidDefender$ You | TriggerZones$ Battlefield | Execute$ X\nSVar:X:DB$ Draw | Defined$ TriggeredDefendingPlayer | NumCards$ 1\nOracle:x\n")
	attacker := onBoardReady(t, e, 0, "Name:Rogue\nTypes:Creature Rogue\nPT:1/1\nOracle:x\n")
	if e.G.Obj(source).Zone != state.ZBattlefield || e.G.Obj(attacker).Zone != state.ZBattlefield {
		t.Fatal("trigger setup not on battlefield")
	}
	e.G.Obj(attacker).IsAttacking = true
	e.G.Obj(attacker).Attacking = 1
	e.checkAttackerUnblockedTriggers()
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("matching attacker queued %d triggers, want 1", len(e.pendingTriggers))
	}
	e.pendingTriggers = nil
	e.G.Obj(attacker).Attacking = 0
	e.checkAttackerUnblockedTriggers()
	if len(e.pendingTriggers) != 0 {
		t.Fatal("ValidDefender$ You accepted a different defender")
	}
}
