package forgeheuristic

import (
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
)

func TestForgeHeuristicTimingRulesUnchanged(t *testing.T) {
	s, err := New(bots.Options{Seed: 7, AutoPayMana: true})
	if err != nil {
		t.Fatal(err)
	}
	hb, ok := s.(*heuristic.Bot)
	if !ok {
		t.Fatalf("New = %T, want *heuristic.Bot", s)
	}
	want := botpolicy.TimingRules{CounterTop: true, DrawTokenLate: true, PumpInCombat: true}
	if got := hb.TimingRules(); got != want {
		t.Fatalf("armed TimingRules = %+v, want CounterTop, DrawTokenLate and PumpInCombat with HoldInstant off (%+v)", got, want)
	}
}
