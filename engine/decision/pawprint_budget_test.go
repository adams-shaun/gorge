package decision

// Pins the cumulative-budget rule's repeat accounting (the {P} budget CR
// 700.2i / CR 107.18 rides): MaxSum is enforced over the SUM of the chosen
// occurrences' Option.Value, and a Repeatable ask's repeats each COUNT
// toward that sum -- three picks of the 2-{P} mode spend 6, against a
// budget of 5, and are refused where two are legal. This is the rule the
// BLB Seasons' mode budgets are published through (the engine-side ask in
// engine/rules and the mid-resolution ask in engine/effects both set
// MaxSum/Value from the card's Pawprint$ tables).

import (
	"strings"
	"testing"
)

// pawprintSeason is the Season-shaped ask: one covering KModes over the
// BLB Season of the Bold's three modes ({P} 1, 2, 3), budget 5, repeats
// allowed, no required picks (MinCharmNum$ 0).
func pawprintSeason() *Decision {
	return &Decision{
		Seq: 9, Player: 0, Kind: KModes, Min: 0, Max: 5, MaxSum: 5, Repeatable: true,
		Options: []Option{
			{Index: 0, Kind: "mode", Label: "Create a tapped Treasure token.", Value: 1},
			{Index: 1, Kind: "mode", Label: "Exile the top two cards of your library.", Value: 2},
			{Index: 2, Kind: "mode", Label: "Whenever you cast a spell, deal 2 damage.", Value: 3},
		},
	}
}

func TestBudgetCountsRepeats(t *testing.T) {
	d := pawprintSeason()
	if err := d.Validate(Intent{Seq: 9, Player: 0, Choices: []int{2, 2, 2}}); err == nil {
		t.Error("accepted 3 repeated 3-{P} picks (9 {P}) against a budget of 5")
	} else if !strings.Contains(err.Error(), "exceeds the budget") {
		t.Errorf("err = %v, want the cumulative-budget rejection", err)
	}
	if err := d.Validate(Intent{Seq: 9, Player: 0, Choices: []int{1, 2}}); err != nil {
		t.Errorf("rejected a 2-{P} and a 3-{P} pick (exactly 5 {P}): %v", err)
	}
	// A repeated pick counts again: two 1-{P} picks plus the 3-{P} one also
	// spend exactly 5 {P}.
	if err := d.Validate(Intent{Seq: 9, Player: 0, Choices: []int{0, 0, 2}}); err != nil {
		t.Errorf("rejected 1+1+3 {P} (exactly 5, with a repeat): %v", err)
	}
	// One more 1-{P} pick breaches the budget even though the count cap (5)
	// would still admit the answer.
	if err := d.Validate(Intent{Seq: 9, Player: 0, Choices: []int{0, 0, 1, 2}}); err == nil {
		t.Error("accepted 1+1+2+3 {P} (6) against a budget of 5")
	}
	// Five repeats of the 1-{P} mode spend exactly the budget: legal.
	if err := d.Validate(Intent{Seq: 9, Player: 0, Choices: []int{0, 0, 0, 0, 0}}); err != nil {
		t.Errorf("rejected five 1-{P} picks (exactly 5 {P}): %v", err)
	}
	// Six would breach it even though Max (5) is the binding floor check.
	if err := d.Validate(Intent{Seq: 9, Player: 0, Choices: []int{0, 0, 0, 0, 0, 0}}); err == nil {
		t.Error("accepted six 1-{P} picks past both the count cap and the budget")
	}
}
