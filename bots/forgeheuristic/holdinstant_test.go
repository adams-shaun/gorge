package forgeheuristic

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/bot"
	"github.com/adams-shaun/gorge/bots/heuristic"
)

// TestForgeHeuristicHoldOption pins the declared hold-instant option: unset
// and explicit false leave the armed timing rules exactly as they were, and
// true arms HoldInstant without disarming any other timing rule.
func TestForgeHeuristicHoldOption(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config map[string]string
		want   bool
	}{
		{name: "unset", config: nil, want: false},
		{name: "false", config: map[string]string{OptHoldInstant: "false"}, want: false},
		{name: "true", config: map[string]string{OptHoldInstant: "true"}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := bots.New(Policy, bots.Options{Seed: 1, Config: tc.config})
			if err != nil {
				t.Fatalf("bots.New(%q): %v", Policy, err)
			}
			hb, ok := s.(*heuristic.Bot)
			if !ok {
				t.Fatalf("built seat is %T, want *heuristic.Bot", s)
			}
			got := hb.TimingRules()
			if got.HoldInstant != tc.want {
				t.Errorf("HoldInstant = %v, want %v", got.HoldInstant, tc.want)
			}
			if !got.CounterTop || !got.DrawTokenLate || !got.PumpInCombat {
				t.Errorf("armed TimingRules = %+v, want CounterTop, DrawTokenLate and PumpInCombat still armed", got)
			}
		})
	}
}

// TestForgeHeuristicHoldPolicy pins the bench policy: a registered
// experimental constructed/commander entry whose seat arms HoldInstant.
func TestForgeHeuristicHoldPolicy(t *testing.T) {
	e, ok := bots.Lookup(HoldPolicy)
	if !ok {
		t.Fatalf("bots.Lookup(%q): not registered (known: %v)", HoldPolicy, bots.Names())
	}
	if e.Tier != bots.Experimental {
		t.Errorf("Tier = %q, want experimental", e.Tier)
	}
	for _, format := range []string{"constructed", "commander"} {
		if !slices.Contains(e.Formats, format) {
			t.Errorf("Formats = %v, want %q", e.Formats, format)
		}
	}
	s, err := bots.New(HoldPolicy, bots.Options{Seed: 1})
	if err != nil {
		t.Fatalf("bots.New(%q): %v", HoldPolicy, err)
	}
	hb, ok := s.(*heuristic.Bot)
	if !ok {
		t.Fatalf("built seat is %T, want *heuristic.Bot", s)
	}
	if got := hb.TimingRules(); !got.HoldInstant {
		t.Errorf("armed TimingRules = %+v, want HoldInstant", got)
	}
}
