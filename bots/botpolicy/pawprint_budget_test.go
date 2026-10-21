package botpolicy

// The bot arm's answer to a pawprint-budgeted KModes (CR 700.2i, CR 107.18,
// the BLB Seasons): the greedy budget fill (policy.go's d.HasBudget() arm)
// must produce an answer Decision.Validate accepts -- the sum of the chosen
// occurrences' Option.Value stays within MaxSum, repeats counted. Before
// the budget was published on the ask, a blind first-Min answer could exceed
// the cap and wedge the seat re-deriving a rejected intent; here the same
// fill the shared repair (Clamp) reads keeps the bot inside the budget.
// The Season's Min is 0 (MinCharmNum$ 0), so the fixed first-option policy
// declines -- unchanged from the pre-budget behaviour.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// pawprintSeasonShapes the Season of the Bold ask as the rules engine
// publishes it: Min 0 (MinCharmNum$ 0), Max 5 (CharmNum$ 5), budget 5
// (root Pawprint$ 5), modes priced 1/2/3 by their own Pawprint$, repeats
// allowed (CanRepeatModes$ True).
func pawprintSeason(min int) *decision.Decision {
	return &decision.Decision{
		Seq: 12, Player: 0, Kind: decision.KModes, Min: min, Max: 5, MaxSum: 5, Repeatable: true,
		Options: []decision.Option{
			{Index: 0, Kind: "mode", Label: "Create a tapped Treasure token.", Value: 1},
			{Index: 1, Kind: "mode", Label: "Exile the top two cards of your library.", Value: 2},
			{Index: 2, Kind: "mode", Label: "Whenever you cast a spell, deal 2 damage.", Value: 3},
		},
	}
}

func TestBotPawprintBudgetAnswerValidates(t *testing.T) {
	// The real Season shape: Min 0, the bot declines (nothing is required
	// and no pass option is offered on a mode ask), and the empty answer
	// trivially sits inside the budget.
	for seed := uint64(0); seed < 30; seed++ {
		d := pawprintSeason(0)
		in := Decide(Board{}, d, rng(seed))
		if err := d.Validate(in); err != nil {
			t.Fatalf("seed %d: season answer %+v failed Validate: %v", seed, in, err)
		}
	}

	// A mandatory budget ask (a hypothetical MinCharmNum$ 2 Season): the
	// greedy fill takes the 3-{P} mode once, skips the second (6 > 5) and
	// tops the count up with the 1-{P} mode -- 4 {P} of 5, both picks
	// distinct, Min satisfied.
	d := pawprintSeason(2)
	in := Decide(Board{}, d, rng(1))
	if err := d.Validate(in); err != nil {
		t.Fatalf("mandatory budget answer %+v failed Validate: %v", in, err)
	}
	if len(in.Choices) != 2 {
		t.Fatalf("mandatory budget answer = %+v, want two picks (Min 2)", in)
	}
	sum := 0
	for _, c := range in.Choices {
		sum += d.Options[c].Value
	}
	if sum > d.MaxSum {
		t.Fatalf("bot spent %d {P} of budget %d: %+v", sum, d.MaxSum, in)
	}
}
