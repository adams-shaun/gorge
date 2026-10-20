package forgeheuristic

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
)

func TestForgeHeuristicArmsCombatRules(t *testing.T) {
	s, err := bots.New(Policy, bots.Options{Seed: 1, AutoPayMana: true})
	if err != nil {
		t.Fatalf("bots.New(%q): %v", Policy, err)
	}
	hb, ok := s.(*heuristic.Bot)
	if !ok {
		t.Fatalf("built seat is %T, want *heuristic.Bot", s)
	}
	got, ok := hb.AttackSimParams()
	if !ok {
		t.Fatal("AttackSimParams: no combat simulation on an auto-pay table")
	}
	if want := (botpolicy.CombatRules{GangBlocks: true, GangPredict: true, DoubleStrike: true}); got.Combat != want {
		t.Errorf("Combat = %+v, want %+v", got.Combat, want)
	}
	got.Combat = botpolicy.CombatRules{}
	if want := botpolicy.DefaultAttackSimParams(); got != want {
		t.Errorf("params other than Combat = %+v, want the defaults %+v", got, want)
	}
}
