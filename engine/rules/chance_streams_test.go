package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestChancePlannerSuppliesNonShuffleDraws pins the chance planner's one job:
// a coin flip past the prefix is its value, not the generator's, and it is not
// counted unplanned. The same game without a planner still counts the flip.
func TestChancePlannerSuppliesNonShuffleDraws(t *testing.T) {
	t.Parallel()
	cfg := chanceConfig(t)
	coin := card(t, "Name:Chance Coin\nManaCost:0\nTypes:Sorcery\n"+
		"A:SP$ FlipCoin | WinSubAbility$ DBGain\n"+
		"SVar:DBGain:DB$ GainLife | Life$ 1\n"+
		"Oracle:Flip a coin.\n")
	for i := range cfg.Decks[0] {
		cfg.Decks[0][i] = coin
		cfg.Decks[1][i] = coin
	}

	// The genesis transcript is the prefix both engines below consume, so the
	// coin flip is the first draw past it.
	base, err := NewHypothetical(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := base.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	genesis := base.ChanceTranscript()
	unplannedBefore := base.UnplannedDraws()
	intents, generatorFlip := castFirstFlip(t, base, len(genesis))
	if got := base.UnplannedDraws(); got != unplannedBefore+1 {
		t.Fatalf("baseline unplanned draws = %d, want the one flip", got-unplannedBefore)
	}

	var calls int
	e, err := NewHypotheticalChancePlanned(cfg, genesis, nil, func(ctx ChanceContext) (int, error) {
		calls++
		if ctx.Ordinal != 0 || ctx.Bound != 2 {
			t.Errorf("planner context = %+v, want ordinal 0 bound 2", ctx)
		}
		return ctx.Bound - 1, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	for i, in := range intents {
		if err := e.SubmitHypothetical(in); err != nil {
			t.Fatalf("planned intent %d: %v", i, err)
		}
	}
	if calls != 1 {
		t.Fatalf("planner calls = %d, want the one flip", calls)
	}
	if got := e.UnplannedDraws(); got != 0 {
		t.Fatalf("planned flip counted %d unplanned draws", got)
	}
	tr := e.ChanceTranscript()
	if len(tr) != len(genesis)+1 {
		t.Fatalf("transcript has %d draws, want genesis plus the flip", len(tr))
	}
	if got, want := tr[len(tr)-1], (ChanceDraw{Bound: 2, Value: 1}); got != want {
		t.Fatalf("planned flip = %v, want %v (generator would give %d)", got, want, generatorFlip)
	}

	plain, err := NewHypothetical(cfg, genesis)
	if err != nil {
		t.Fatal(err)
	}
	if err := plain.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	for i, in := range intents {
		if err := plain.SubmitHypothetical(in); err != nil {
			t.Fatalf("unplanned intent %d: %v", i, err)
		}
	}
	if got := plain.UnplannedDraws(); got != 1 {
		t.Fatalf("no-planner unplanned draws = %d, want the one flip", got)
	}
}

// TestChancePlannerNilIsByteIdentical pins the default: passing nil to the
// new constructor is exactly NewHypotheticalPlanned.
func TestChancePlannerNilIsByteIdentical(t *testing.T) {
	t.Parallel()
	cfg := chanceConfig(t)
	prefix := []ChanceDraw{{Bound: 2, Value: 0}}
	a, err := NewHypotheticalPlanned(cfg, prefix, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewHypotheticalChancePlanned(cfg, prefix, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	if err := b.AdvanceHypothetical(); err != nil {
		t.Fatal(err)
	}
	passAll(t, a, 20)
	passAll(t, b, 20)
	if a.L.Head() != b.L.Head() {
		t.Fatalf("heads differ: %s vs %s", a.L.Head(), b.L.Head())
	}
	if a.RNGDraws() != b.RNGDraws() {
		t.Fatalf("draw counts differ: %d vs %d", a.RNGDraws(), b.RNGDraws())
	}
	ta, tb := a.ChanceTranscript(), b.ChanceTranscript()
	if len(ta) != len(tb) {
		t.Fatalf("transcripts differ in length: %d vs %d", len(ta), len(tb))
	}
	for i := range ta {
		if ta[i] != tb[i] {
			t.Fatalf("transcript draw %d differs: %v vs %v", i, ta[i], tb[i])
		}
	}
}

// castFirstFlip drives e to its first draw past before, casting the coin spell
// once. It returns the intents submitted and that draw's value.
func castFirstFlip(t *testing.T, e *Engine, before int) ([]decision.Intent, int) {
	t.Helper()
	var intents []decision.Intent
	cast := false
	for n := 0; n < 200; n++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("game ended before the coin flip")
		}
		choice := -1
		if !cast {
			for i, o := range d.Options {
				if o.Kind == "cast" {
					choice = i
					cast = true
					break
				}
			}
		}
		if choice < 0 {
			for i, o := range d.Options {
				if o.Kind == "pass" {
					choice = i
					break
				}
			}
		}
		if choice < 0 {
			t.Fatalf("decision has neither a cast nor a pass option: %+v", d)
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{choice}}
		intents = append(intents, in)
		if err := e.SubmitHypothetical(in); err != nil {
			t.Fatalf("intent %d: %v", n, err)
		}
		tr := e.ChanceTranscript()
		if len(tr) > before {
			if !cast {
				t.Fatalf("a draw appeared before the coin spell was cast: %v", tr[before:])
			}
			if len(tr) != before+1 {
				t.Fatalf("coin spell drew %d values, want 1", len(tr)-before)
			}
			return intents, tr[before].Value
		}
	}
	t.Fatal("coin spell did not resolve")
	return nil, 0
}
