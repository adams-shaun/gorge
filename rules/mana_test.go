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
		"X R":     {Colored: pool(0, 0, 0, 1, 0, 0), X: 1},
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

// This test pins the known M1 approximation: hybrid and Phyrexian symbols
// are treated as generic mana, not as alternative payments. This is deliberately
// over-permissive and affects Dismember and Gitaxian Probe in M1, which are
// mispriced (accepting regular mana, not life). A future milestone must add
// proper alternative-payment modelling.
func TestHybridAndPhyrexianAreApproximatedAsGeneric(t *testing.T) {
	// Forge spells color hybrid as "GW" (Kitchen Finks), "RW" (Figure of Destiny).
	// "1 GW" parses as numeric "1" (Generic: 1) + unrecognized "GW" (Generic: 1) = Generic: 2.
	gwCost := ParseCost("1 GW")
	if gwCost.Generic != 2 || gwCost.Colored.Total() != 0 {
		t.Errorf("ParseCost(\"1 GW\") = %+v, want generic=2, colored=0", gwCost)
	}
	// Cost should be payable by any mana (over-permissive: no color requirement).
	if !gwCost.CanPay(pool(2, 0, 0, 0, 0, 0)) {
		t.Error("GW cost should be payable by WW")
	}
	if !gwCost.CanPay(pool(0, 0, 2, 0, 0, 0)) {
		t.Error("GW cost should be payable by BB (over-permissive)")
	}
	if !gwCost.CanPay(pool(0, 0, 0, 0, 0, 2)) {
		t.Error("GW cost should be payable by CC (over-permissive)")
	}

	// Forge spells monocolour hybrid as "2B" (Beseech the Queen).
	// "2B" is unrecognized as a numeric token, so Generic: 1.
	monoCost := ParseCost("2B")
	if monoCost.Generic != 1 || monoCost.Colored.Total() != 0 {
		t.Errorf("ParseCost(\"2B\") = %+v, want generic=1, colored=0", monoCost)
	}

	// Forge rarely spells hybrid as "W/U" (one card out of 33,669).
	slashCost := ParseCost("W/U")
	if slashCost.Generic != 1 || slashCost.Colored.Total() != 0 {
		t.Errorf("ParseCost(\"W/U\") = %+v, want generic=1, colored=0", slashCost)
	}

	// Phyrexian mana (UP, BP) is also approximated as generic.
	// Dismember is "ManaCost:1 BP BP", which parses to "1" (Generic: 1) + "BP" (Generic: 1) + "BP" (Generic: 1) = Generic: 3.
	dismemberCost := ParseCost("1 BP BP")
	if dismemberCost.Generic != 3 || dismemberCost.Colored.Total() != 0 {
		t.Errorf("ParseCost(\"1 BP BP\") = %+v, want generic=3, colored=0", dismemberCost)
	}
	// Should be payable by any mana (BP misprice: no life payment).
	if !dismemberCost.CanPay(pool(0, 0, 0, 3, 0, 0)) {
		t.Error("Dismember cost should be payable by RRR (misprice: no life)")
	}

	// Gitaxian Probe is "ManaCost:UP".
	probeCost := ParseCost("UP")
	if probeCost.Generic != 1 || probeCost.Colored.Total() != 0 {
		t.Errorf("ParseCost(\"UP\") = %+v, want generic=1, colored=0", probeCost)
	}
	if !probeCost.CanPay(pool(0, 0, 0, 0, 1, 0)) {
		t.Error("Gitaxian Probe cost should be payable by G alone (misprice: no life)")
	}
}

// This test pins the known numeric validation: negative and out-of-range
// numeric tokens are treated as unrecognized symbols and contribute +1 generic.
func TestNumericTokenValidation(t *testing.T) {
	// Negative tokens should fall through to +1 generic.
	negCost := ParseCost("-1")
	if negCost.Generic != 1 || negCost.Colored.Total() != 0 {
		t.Errorf("ParseCost(\"-1\") = %+v, want generic=1", negCost)
	}

	// Tokens above int32 max should fall through to +1 generic.
	// int32 max is 2147483647.
	overflowCost := ParseCost("2147483648")
	if overflowCost.Generic != 1 || overflowCost.Colored.Total() != 0 {
		t.Errorf("ParseCost(\"2147483648\") = %+v, want generic=1", overflowCost)
	}

	// Tokens above int64 max should fall through to +1 generic.
	// int64 max is 9223372036854775807.
	largeOverflowCost := ParseCost("9223372036854775808")
	if largeOverflowCost.Generic != 1 || largeOverflowCost.Colored.Total() != 0 {
		t.Errorf("ParseCost(\"9223372036854775808\") = %+v, want generic=1", largeOverflowCost)
	}

	// Valid boundary: int32 max should parse correctly.
	validMaxCost := ParseCost("2147483647")
	if validMaxCost.Generic != 2147483647 || validMaxCost.Colored.Total() != 0 {
		t.Errorf("ParseCost(\"2147483647\") = %+v, want generic=2147483647", validMaxCost)
	}
}

func TestParseCostNonManaParts(t *testing.T) {
	c := ParseCost("2 C Sac<1/Land>")
	if c.Generic != 2 || c.Colored[state.MC] != 1 || len(c.Sac) != 1 || c.Sac[0] != (CostPart{1, "Land"}) {
		t.Fatalf("%+v", c)
	}
	c = ParseCost("SubCounter<2/P1P1>")
	if c.CMC() != 0 || len(c.SubCounter) != 1 || c.SubCounter[0] != (CostPart{2, "P1P1"}) || !c.HasNonMana() {
		t.Fatalf("%+v", c)
	}
	c = ParseCost("T")
	if !c.Tap || c.CMC() != 0 {
		t.Fatalf("%+v", c)
	}
	c = ParseCost("X X W W W")
	if c.X != 2 || c.Colored[state.MW] != 3 || c.CMC() != 3 {
		t.Fatalf("%+v", c)
	}
	if w := c.WithX(2); w.X != 0 || w.Generic != 4 || w.CMC() != 7 {
		t.Fatalf("WithX %+v", w)
	}
	if p := ParseCost("R").Plus(ParseCost("R")); p.Colored[state.MR] != 2 {
		t.Fatalf("Plus %+v", p)
	}
	if ParseCost("3 U").HasNonMana() {
		t.Fatal("mana-only cost reports non-mana parts")
	}
}
