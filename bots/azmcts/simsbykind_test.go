package azmcts

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// SimsByKind sets a listed kind's budget and leaves every other kind on
// Search.Sims; an entry of 0 is Search.Sims too.
func TestSimsByKindSetsTheBudgetPerKind(t *testing.T) {
	cfg := testConfig(t, "mono-red-prowess", "mono-blue-tempo", testSeed)
	sc := DefaultSeatConfig()
	sc.Search.Sims = 4
	sc.Source = testSeatSource
	sc.SimsByKind = map[decision.Kind]int{decision.KPriority: 2, decision.KAttackers: 0}
	var diags []Diag
	prev := Watch
	Watch = func(d Diag) {
		if d.Searched {
			diags = append(diags, d)
		}
	}
	t.Cleanup(func() { Watch = prev })
	az, err := NewSeat(cfg.Seed^1, nil, sc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := playAZ(t, cfg, az, 300); err != nil {
		t.Fatal(err)
	}
	pri, other := 0, 0
	for _, d := range diags {
		want := 4
		if d.Kind == "priority" {
			want = 2
			pri++
		} else {
			other++
		}
		if d.Stats.Simulations != want {
			t.Fatalf("a searched %s decision ran %d simulations, want %d", d.Kind, d.Stats.Simulations, want)
		}
	}
	if pri == 0 || other == 0 {
		t.Fatalf("%d priority and %d other searched decisions: want both", pri, other)
	}
}

func TestNewSeatValidatesSimsByKind(t *testing.T) {
	for _, tc := range []struct {
		m    map[decision.Kind]int
		want string
	}{
		{map[decision.Kind]int{decision.KBlockers: -1}, "SimsByKind[blockers]"},
		{map[decision.Kind]int{"no_such_kind": 10}, "outside decision.Kinds"},
	} {
		sc := DefaultSeatConfig()
		sc.Source = testSeatSource
		sc.SimsByKind = tc.m
		if _, err := NewSeat(1, nil, sc); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("SimsByKind %v: NewSeat error %v, want one mentioning %q", tc.m, err, tc.want)
		}
	}
	sc := DefaultSeatConfig()
	sc.Source = testSeatSource
	sc.SimsByKind = map[decision.Kind]int{decision.KPriority: 200, decision.KBlockers: 0}
	if _, err := NewSeat(1, nil, sc); err != nil {
		t.Fatal(err)
	}
}
