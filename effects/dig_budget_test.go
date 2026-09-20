package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// The WithTotalCMC$ budget tests: a Dig's "total mana value N or less" cap.
// The askHost double (primitives_test.go) captures the posed decision and
// suspends; re-entry is a second Resolve with Ctx.Dig/DigDone set, exactly
// the engine's contract. Every fixture builds its own cards with explicit
// ManaCost values so Face().Cmc() is deterministic.

// digBudgetFixture builds seat 0's library as one nonland permanent per MV,
// in the given order, with the given mana costs, and returns (host, ids).
func digBudgetFixture(t *testing.T, mvs ...int) (*askHost, []state.ObjID) {
	t.Helper()
	h := &askHost{}
	h.g = state.NewGame(names(2))
	ids := make([]state.ObjID, 0, len(mvs))
	for i, mv := range mvs {
		col := "W"
		if i%2 == 1 {
			col = "U"
		}
		pips := make([]string, 0, mv)
		for j := 0; j < mv; j++ {
			pips = append(pips, "{"+col+"}")
		}
		src := "Name:Artifact" + string(rune('A'+i)) + "\nTypes:Artifact\nManaCost:" +
			strings.Join(pips, "") + "\nOracle:x\n"
		ids = append(ids, h.g.AddObject(mkCard(t, src), 0).ID)
	}
	h.g.SetZone(state.ZLibrary, 0, ids)
	return h, ids
}

// TestDigWithTotalCMCNarrowsEligible: a card whose own mana value exceeds the
// budget can never be picked. Window [5, 2, 2, 2], budget 4, ChangeNum Any --
// the 5-MV card is absent from the options while the three 2-MV cards are
// offered, and the ask is posed because the forced greedy take (first two,
// sum 4) cannot consume all three.
func TestDigWithTotalCMCNarrowsEligible(t *testing.T) {
	h, ids := digBudgetFixture(t, 5, 2, 2, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ Dig | Defined$ You | DigNum$ 4 | ChangeNum$ Any | ChangeValid$ Artifact | WithTotalCMC$ 4 | DestinationZone$ Hand"))
	if h.asked == nil {
		t.Fatal("no decision: the budget forces a choice the forced take cannot make")
	}
	d := h.asked
	if d.MaxSum != 4 {
		t.Fatalf("MaxSum = %d, want 4", d.MaxSum)
	}
	if len(d.Options) != 3 {
		t.Fatalf("options = %+v, want exactly the three 2-MV cards (the 5-MV excluded)", d.Options)
	}
	for _, o := range d.Options {
		if o.Obj == ids[0] {
			t.Fatalf("the 5-MV card %d was offered under a budget of 4", ids[0])
		}
		if o.Value != 2 {
			t.Fatalf("option %+v Value = %d, want each offered card's mana value 2", o, o.Value)
		}
	}
	if !strings.Contains(d.Prompt, "total mana value 4 or less") {
		t.Fatalf("prompt = %q, want it to name the budget in the card's own terms", d.Prompt)
	}
}

// TestDigWithTotalCMCCapsCumulative: the budget is cumulative, not per card.
// Window [2, 2, 2, 2], budget 4, ChangeNum Any -- all four are individually
// affordable but only two fit together, so Validate rejects a four-pick and
// accepts a two-pick summing to 4, and the greedy take moves exactly two.
func TestDigWithTotalCMCCapsCumulative(t *testing.T) {
	h, ids := digBudgetFixture(t, 2, 2, 2, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ Dig | Defined$ You | DigNum$ 4 | ChangeNum$ Any | ChangeValid$ Artifact | WithTotalCMC$ 4 | DestinationZone$ Hand"))
	if h.asked == nil {
		t.Fatal("no decision: two picks fit but four do not, so a real choice exists")
	}
	d := h.asked
	// Over-budget answer rejected by Validate.
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0, 1, 2, 3}}); err == nil {
		t.Fatal("an over-budget intent passed Validate; the cumulative cap must be enforced")
	}
	// At-budget answer accepted.
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0, 1}}); err != nil {
		t.Fatalf("an at-budget intent %v was rejected: %v", []int{0, 1}, err)
	}
	// Re-entry with the greedy two-pick moves exactly those two.
	ctx := &Ctx{Controller: 0, Dig: []state.ObjID{ids[0], ids[1]}, DigDone: true}
	Resolve(h, ctx, sa(t,
		"DB$ Dig | Defined$ You | DigNum$ 4 | ChangeNum$ Any | ChangeValid$ Artifact | WithTotalCMC$ 4 | DestinationZone$ Hand"))
	if hand := h.g.Zone(state.ZHand, 0); len(hand) != 2 || hand[0] != ids[0] || hand[1] != ids[1] {
		t.Fatalf("hand = %v, want exactly the greedy [%d %d]", hand, ids[0], ids[1])
	}
	if lib := h.g.Zone(state.ZLibrary, 0); len(lib) != 2 || lib[0] != ids[2] || lib[1] != ids[3] {
		t.Fatalf("library = %v, want the untaken [%d %d]", lib, ids[2], ids[3])
	}
}

// TestDigWithTotalCMCNoAskWhenAllFit: when the whole affordable set fits under
// the budget there is no choice to pose -- the forced greedy take consumes
// every eligible card -- so no decision is emitted and every card moves.
func TestDigWithTotalCMCNoAskWhenAllFit(t *testing.T) {
	h, ids := digBudgetFixture(t, 2, 2)
	Resolve(h, &Ctx{Controller: 0}, sa(t,
		"DB$ Dig | Defined$ You | DigNum$ 2 | ChangeNum$ 2 | ChangeValid$ Artifact | WithTotalCMC$ 4 | DestinationZone$ Hand"))
	if h.asked != nil {
		t.Fatalf("a decision was posed (%+v); the whole affordable set fits, so the take is forced", h.asked)
	}
	if hand := h.g.Zone(state.ZHand, 0); len(hand) != 2 || hand[0] != ids[0] || hand[1] != ids[1] {
		t.Fatalf("hand = %v, want both cards", hand)
	}
}

// TestDigWithTotalCMCSVarSacrificedX: the budget value resolves through the
// same Num grammar as every numeric parameter -- here smelting_vat's
// SVar:X:Sacrificed$CardManaCost, which reads the LKI of the artifact
// sacrificed to pay the activation cost. A 3-MV artifact was sacrificed, so
// the budget is 3: a 2-MV card fits and a 4-MV card does not.
func TestDigWithTotalCMCSVarSacrificedX(t *testing.T) {
	h, ids := digBudgetFixture(t, 4, 2)
	ctx := &Ctx{Controller: 0,
		SVars:      map[string]string{"X": "Sacrificed$CardManaCost"},
		Sacrificed: []state.SacrificedInfo{{ManaValue: 3}}}
	Resolve(h, ctx, sa(t,
		"AB$ Dig | Defined$ You | DigNum$ 2 | ChangeNum$ Any | ChangeValid$ Artifact | WithTotalCMC$ X | DestinationZone$ Hand"))
	if h.asked == nil {
		t.Fatal("no decision: budget 3 admits only the 2-MV card, so the forced take cannot consume both")
	}
	if len(h.asked.Options) != 1 || h.asked.Options[0].Obj != ids[1] {
		t.Fatalf("options = %+v, want only the 2-MV card %d (the 4-MV card exceeds the resolved budget 3)", h.asked.Options, ids[1])
	}
	if h.asked.MaxSum != 3 {
		t.Fatalf("MaxSum = %d, want the resolved budget 3", h.asked.MaxSum)
	}
}
