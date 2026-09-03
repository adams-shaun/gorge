package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func pool(w, u, b, r, g, c int32) state.Mana { return state.Mana{w, u, b, r, g, c} }

func TestParseCostForms(t *testing.T) {
	for src, want := range map[string]Cost{
		"R":       {Colored: pool(0, 0, 0, 1, 0, 0)},
		"2 U U":   {Colored: pool(0, 2, 0, 0, 0, 0), Generic: 2},
		"1 W":     {Colored: pool(1, 0, 0, 0, 0, 0), Generic: 1},
		"{2}{R}":  {Colored: pool(0, 0, 0, 1, 0, 0), Generic: 2},
		"no cost": {},
		"":        {},
		"X R":     {Colored: pool(0, 0, 0, 1, 0, 0), X: true},
		"C":       {Colored: pool(0, 0, 0, 0, 0, 1)},
	} {
		got := ParseCost(src)
		if got.Colored != want.Colored || got.Generic != want.Generic || got.X != want.X {
			t.Errorf("ParseCost(%q) = %+v, want %+v", src, got, want)
		}
	}
}

func TestCMCCountsColoredAndGeneric(t *testing.T) {
	if got := ParseCost("2 U U").CMC(); got != 4 {
		t.Errorf("CMC = %d, want 4", got)
	}
	if got := ParseCost("no cost").CMC(); got != 0 {
		t.Errorf("CMC = %d, want 0", got)
	}
}

func TestCanPayRequiresTheRightColors(t *testing.T) {
	c := ParseCost("1 R")
	if !c.CanPay(pool(0, 0, 0, 1, 0, 1)) {
		t.Error("R + C should pay {1}{R}")
	}
	if c.CanPay(pool(1, 1, 0, 0, 0, 0)) {
		t.Error("W + U must not pay {1}{R}")
	}
	if c.CanPay(pool(0, 0, 0, 1, 0, 0)) {
		t.Error("a single R must not pay {1}{R}")
	}
}

// Generic cost must not consume mana the coloured requirement still needs.
func TestPaySpendsGenericLast(t *testing.T) {
	c := ParseCost("1 R R")
	after, ok := c.Pay(pool(0, 0, 0, 3, 0, 0))
	if !ok {
		t.Fatal("RRR should pay {1}{R}{R}")
	}
	if after.Total() != 0 {
		t.Fatalf("pool after = %v, want empty", after)
	}

	after, ok = c.Pay(pool(1, 0, 0, 2, 0, 0))
	if !ok {
		t.Fatal("W + RR should pay {1}{R}{R}")
	}
	if after[state.MR] != 0 || after[state.MW] != 0 {
		t.Fatalf("pool after = %v, want empty", after)
	}
}

func TestPayFailsCleanly(t *testing.T) {
	before := pool(0, 0, 0, 1, 0, 0)
	after, ok := ParseCost("2 R").Pay(before)
	if ok {
		t.Fatal("insufficient mana was accepted")
	}
	if after != before {
		t.Fatal("a failed payment must not mutate the pool")
	}
}
