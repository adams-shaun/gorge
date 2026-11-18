package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// scriptRangeAsk builds a standalone ask: one option per n, each index its
// own, no mana symbol (an object election, not a colour allocation).
func scriptRangeAsk(n int) *decision.Decision {
	d := &decision.Decision{Seq: 7, Player: 0, Kind: decision.KTarget, Prompt: "pick"}
	for i := 0; i < n; i++ {
		d.Options = append(d.Options, decision.Option{Index: i, Kind: "target", Label: "t"})
	}
	return d
}

// countSizes buckets answers by the number of choices each names.
func countSizes(ans []decision.Intent) map[int]int {
	out := map[int]int{}
	for _, in := range ans {
		out[len(in.Choices)]++
	}
	return out
}

// TestScriptAnswersRangeAskEnumeratesSizes pins scriptAnswers on a range ask
// ("return up to two target ... cards", CR 601.2c): the search must branch
// over the combinations of every size the ask admits, the empty answer
// included when Min is 0, or a prefix past the ask is unanswerable and a
// legal play hides behind it (Rewrite History's fourth-plan-counter
// sacrifice chain returning up to two instant/sorcery cards).
func TestScriptAnswersRangeAskEnumeratesSizes(t *testing.T) {
	t.Parallel()
	// Min 0, Max 2 over four options: 1 empty + 4 singles + 6 pairs.
	d := scriptRangeAsk(4)
	d.Min, d.Max = 0, 2
	ans := scriptAnswers(d)
	if got := countSizes(ans); got[0] != 1 || got[1] != 4 || got[2] != 6 || len(ans) != 11 {
		t.Fatalf("min0 max2 n4 sizes: %v (want 0:1 1:4 2:6, total 11)", got)
	}
	// Every enumerated answer validates.
	for _, in := range ans {
		if err := d.Validate(in); err != nil {
			t.Fatalf("answer %v does not validate: %v", in.Choices, err)
		}
	}
	// Min 1, Max 2: no empty answer, the same singles and pairs.
	d.Min, d.Max = 1, 2
	ans = scriptAnswers(d)
	if got := countSizes(ans); got[0] != 0 || got[1] != 4 || got[2] != 6 || len(ans) != 10 {
		t.Fatalf("min1 max2 n4 sizes: %v (want 1:4 2:6, total 10)", got)
	}
	// A fixed-count ask keeps its n-combinations: Min == Max == 2.
	d.Min, d.Max = 2, 2
	ans = scriptAnswers(d)
	if len(ans) != 6 {
		t.Fatalf("min2 max2 n4: %d answers (want 6)", len(ans))
	}
}

// TestScriptAnswersRangeAskSkipsColourAllocations pins the range case's
// boundary: a unit-major colour allocation (every option names a mana
// symbol) is not a subset shape -- its sizes mix units -- so a range-shaped
// allocation keeps the answer set it had before (empty), not subsets of
// single units.
func TestScriptAnswersRangeAskSkipsColourAllocations(t *testing.T) {
	t.Parallel()
	d := &decision.Decision{Seq: 9, Player: 0, Kind: decision.KChoose, Prompt: "colours", Min: 1, Max: 2}
	for _, sym := range []string{"W", "U", "B", "R"} {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mana", Label: sym, ManaSymbol: sym})
	}
	if !scriptManaUnits(d) {
		t.Fatal("precondition: the ask is a unit-major colour allocation")
	}
	if ans := scriptAnswers(d); len(ans) != 0 {
		t.Fatalf("colour allocation range ask: %d answers (want 0, the pre-fix behaviour)", len(ans))
	}
}
