package hindsight

import (
	"math"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

func TestWilsonAndClearMarginMath(t *testing.T) {
	lo, hi := Wilson95(8, 10)
	if math.Abs(lo-0.490162) > 1e-5 || math.Abs(hi-0.943318) > 1e-5 {
		t.Fatalf("Wilson(8/10) = [%.6f, %.6f]", lo, hi)
	}
	mean, diffLo, diffHi := PairedDiff95(30, 5, 64)
	if mean <= 0 || diffLo <= 0 || diffHi <= diffLo {
		t.Fatalf("paired interval = mean %.3f [%.3f, %.3f]", mean, diffLo, diffHi)
	}
	chosen := OptionResult{Rate: .40}
	best := OptionResult{Rate: .55, PairedDifferenceLow: .03}
	if delta, clear := ClearMargin(chosen, best); !clear || math.Abs(delta-.15) > 1e-12 {
		t.Fatalf("clear margin = %.3f, %v", delta, clear)
	}
	best.PairedDifferenceLow = 0
	if _, clear := ClearMargin(chosen, best); clear {
		t.Fatal("interval touching zero was clear")
	}
	best.Rate = .499
	best.PairedDifferenceLow = .01
	if _, clear := ClearMargin(chosen, best); clear {
		t.Fatal("sub-ten-point margin was clear")
	}
}

func TestBackwardWalkOrder(t *testing.T) {
	if got, want := Backward(5), []int{4, 3, 2, 1, 0}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Backward(5) = %v, want %v", got, want)
	}
	if got := Backward(0); len(got) != 0 {
		t.Fatalf("Backward(0) = %v", got)
	}
}

func TestGenericCandidatesStayValidForOrderedRestAndPermutationAsks(t *testing.T) {
	cases := []struct {
		name string
		d    *decision.Decision
		bot  decision.Intent
	}{
		{
			name: "arrange rest",
			d: &decision.Decision{Seq: 7, Player: 0, Kind: decision.KArrange, Min: 0, Max: 3, Restable: true, Options: []decision.Option{
				{Index: 0, Kind: "bottom"}, {Index: 1, Kind: "bottom"}, {Index: 2, Kind: "bottom"},
			}},
			bot: decision.Intent{Seq: 7, Player: 0, Choices: []int{0}, Rest: []int{2, 1}},
		},
		{
			name: "trigger permutation",
			d: &decision.Decision{Seq: 9, Player: 1, Kind: decision.KTriggerOrder, Min: 3, Max: 3, Options: []decision.Option{
				{Index: 0, Kind: "trigger"}, {Index: 1, Kind: "trigger"}, {Index: 2, Kind: "trigger"},
			}},
			bot: decision.Intent{Seq: 9, Player: 1, Choices: []int{0, 1, 2}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := genericCandidates(tc.d, tc.bot, 8)
			if len(got) < 2 {
				t.Fatalf("only %d candidates", len(got))
			}
			for _, in := range got {
				if err := tc.d.Validate(in); err != nil {
					t.Fatalf("candidate %+v: %v", in, err)
				}
			}
		})
	}
}

func TestPriorityCandidatesIgnoreBareManaWindows(t *testing.T) {
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "activate"}, {Index: 1, Kind: "pass"}, {Index: 2, Kind: "cast"},
	}}
	activate := decision.Intent{Seq: 1, Player: 0, Choices: []int{0}}
	if got := priorityCandidates(d, activate, 8); len(got) != 1 {
		t.Fatalf("mana activation produced %d hindsight arms", len(got))
	}
	pass := decision.Intent{Seq: 1, Player: 0, Choices: []int{1}}
	if got := priorityCandidates(d, pass, 8); len(got) != 2 || got[1].Choices[0] != 2 {
		t.Fatalf("pass/cast arms = %+v", got)
	}
}

func TestSeedIncludesEveryCoordinate(t *testing.T) {
	base := Seed(9, 10, 11, 12)
	for _, got := range []uint64{Seed(8, 10, 11, 12), Seed(9, 9, 11, 12), Seed(9, 10, 10, 12), Seed(9, 10, 11, 11)} {
		if got == base {
			t.Fatalf("coordinate did not change seed %d", base)
		}
	}
}
