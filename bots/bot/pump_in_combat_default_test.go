package bot

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
)

func TestBotArmsPumpInCombatOnly(t *testing.T) {
	s, err := newBot(bots.Options{Seed: 7, AutoPayMana: true})
	if err != nil {
		t.Fatal(err)
	}
	hb, ok := s.(*heuristic.Bot)
	if !ok {
		t.Fatalf("newBot = %T, want *heuristic.Bot", s)
	}
	want := botpolicy.TimingRules{PumpInCombat: true}
	if got := hb.TimingRules(); got != want {
		t.Fatalf("armed TimingRules = %+v, want PumpInCombat only (%+v)", got, want)
	}
}

func TestBaseTimingRulesBothBranches(t *testing.T) {
	p := botpolicy.DefaultAttackSimParams()
	auto := BaseWithSim(bots.Options{Seed: 7, AutoPayMana: true}, p)
	plain := BaseWithSim(bots.Options{Seed: 7, AutoPayMana: false}, p)
	if auto == nil || plain == nil {
		t.Fatalf("BaseWithSim returned nil: auto=%v plain=%v", auto, plain)
	}
	want := botpolicy.TimingRules{PumpInCombat: true}
	if got := auto.TimingRules(); got != want {
		t.Errorf("auto-pay TimingRules = %+v, want %+v", got, want)
	}
	if got := plain.TimingRules(); got != want {
		t.Errorf("plain TimingRules = %+v, want %+v", got, want)
	}
	if got, other := auto.TimingRules(), plain.TimingRules(); got != other {
		t.Errorf("branches disagree: auto-pay %+v, plain %+v", got, other)
	}
}
