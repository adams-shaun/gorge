package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Contaminated Bond: "Whenever enchanted creature attacks or blocks, its
// controller loses 3 life." The attack half reads the attacker's
// controller (kop.RefTriggerPlayer, the attack trigger's player); the block
// half reads the enchanted blocker's controller live (kop.RefAttachedCtrl),
// which is not the trigger's player (a Blocks trigger names the attacker's
// controller). Both players cast it, so it sits on their own creatures and
// on the opponent's. The fixture pool is
// dirac/cardset/pools/fixtures/contaminatedbond.

var contaminatedBondDecks = []fixtureDecks{{
	{
		{Name: "Contaminated Bond", Count: 8}, {Name: "Grizzly Bears", Count: 10},
		{Name: "Swamp", Count: 11}, {Name: "Forest", Count: 11},
	},
	{
		{Name: "Contaminated Bond", Count: 4}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	},
}}

var contaminatedBondWatch = map[string]bool{"Contaminated Bond": true}

func TestContaminatedBondLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "contaminatedbond", contaminatedBondDecks, contaminatedBondWatch, 200, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Contaminated Bond"] < 60 {
		t.Errorf("only %d Contaminated Bond casts", run.casts["Contaminated Bond"])
	}
}

// TestContaminatedBondBlockMutation reads the Aura's controller where the
// enchanted blocker's belongs: the lockstep must diverge.
func TestContaminatedBondBlockMutation(t *testing.T) {
	run := runFixtureLockstep(t, "contaminatedbond", contaminatedBondDecks, contaminatedBondWatch, 64, false, func(tbl *kop.Table) {
		n := 0
		mutateCard(t, tbl, "Contaminated Bond", func(a *kop.Ability, in *kop.Instr) {
			if in.Op == kop.OpLoseLife && in.Obj == kop.RefAttachedCtrl {
				in.Obj = kop.RefController
				n++
			}
		})
		if n == 0 {
			t.Fatal("no RefAttachedCtrl LoseLife in the lowered Contaminated Bond")
		}
	})
	run.requireDivergence(t, "the Aura's controller loses the life")
}

// TestContaminatedBondAttackMutation reads the Aura's controller where the
// enchanted attacker's belongs: the lockstep must diverge.
func TestContaminatedBondAttackMutation(t *testing.T) {
	run := runFixtureLockstep(t, "contaminatedbond", contaminatedBondDecks, contaminatedBondWatch, 64, false, func(tbl *kop.Table) {
		n := 0
		mutateCard(t, tbl, "Contaminated Bond", func(a *kop.Ability, in *kop.Instr) {
			if in.Op == kop.OpLoseLife && in.Obj == kop.RefTriggerPlayer {
				in.Obj = kop.RefController
				n++
			}
		})
		if n == 0 {
			t.Fatal("no RefTriggerPlayer LoseLife in the lowered Contaminated Bond")
		}
	})
	run.requireDivergence(t, "the Aura's controller loses the life")
}
