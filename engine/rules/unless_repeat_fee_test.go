package rules

import (
	"testing"
)

// unlessRepeatFeeCensus lists seat 0's unless-cost window source names, so a
// failing test can say what the census admitted instead.
func unlessRepeatFeeCensus(t *testing.T, e *Engine) []string {
	t.Helper()
	var out []string
	for _, u := range e.unlessWindowUnits(0) {
		o := e.G.Obj(u.ID)
		if o == nil || o.Face() == nil {
			continue
		}
		out = append(out, o.Face().Name)
	}
	return out
}

func unlessRepeatFeeHasElf(t *testing.T, e *Engine) {
	t.Helper()
	for _, u := range e.unlessWindowUnits(0) {
		if o := e.G.Obj(u.ID); o != nil && o.Face() != nil && o.Face().Name == "Nomadic Elf" {
			if len(u.Alts) == 0 {
				t.Fatalf("precondition: Nomadic Elf listed with no alternatives (fee-carrying any-colour source must join the unless census)")
			}
			return
		}
	}
	t.Fatalf("precondition: Nomadic Elf missing from the unless-cost window census (sources: %v)", unlessRepeatFeeCensus(t, e))
}

// CR 605.3a/602.5: Nomadic Elf's "{1}{G}: Add one mana of any color" has no
// {T} and no activation limit, so a payment activates it as often as it can
// fund the fee. An unless cost of {3}{U}{U} beside 2 Forests, 2 Mountains and
// 3 other lands is payable ONLY through two Elf activations (each eats {G} and
// one generic, makes one blue); the reach walk must re-enter the Elf's unit.
func TestUnlessRepeatFeeElfFundsTwoBlue(t *testing.T) {
	t.Parallel()
	board := []string{"Nomadic Elf", "Forest", "Forest", "Mountain", "Mountain",
		"Plains", "Plains", "Plains"}
	e := boundBoard(t, board, nil)
	unlessRepeatFeeHasElf(t, e)
	if !e.UnlessCostPayable(0, "3 U U") {
		t.Fatal("3 U U is payable through two Nomadic Elf activations: the unless gate must offer Pay")
	}
}

// One Forest, one Mountain and one other land fund exactly one Elf activation:
// 1 blue plus 2 mana cannot cover {3}{U}{U}.
func TestUnlessRepeatFeeElfSingleActivationCannotPay(t *testing.T) {
	t.Parallel()
	board := []string{"Nomadic Elf", "Forest", "Mountain", "Plains"}
	e := boundBoard(t, board, nil)
	unlessRepeatFeeHasElf(t, e)
	if e.UnlessCostPayable(0, "3 U U") {
		t.Fatal("3 U U is out of reach with one Elf activation available: the unless gate must not offer Pay")
	}
}
