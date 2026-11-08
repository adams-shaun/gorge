package rules

import (
	"fmt"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// scriptTapElectionBoard puts Supportive Parents, three Islands and bears
// Grizzly Bears on seat 0's battlefield and Robotics Mastery in its hand, at
// seat 0's first main phase of turn 3, with summoning sickness cleared. It
// returns the engine and the Parents and Robotics Mastery object ids.
func scriptTapElectionBoard(t *testing.T, bears int) (e *Engine, parents, robotics state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	board := []string{"Supportive Parents", "Island", "Island", "Island"}
	for i := 0; i < bears; i++ {
		board = append(board, "Grizzly Bears")
	}
	var deck []*cards.Card
	for _, n := range board {
		deck = append(deck, lookup(t, reg, n))
	}
	deck = append(deck, lookup(t, reg, "Robotics Mastery"))
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e = New(seatZeroStart(cfg))
	e.Advance()
	for _, n := range board {
		moveToBattlefield(t, e, findAndMoveToHand(t, e, 0, n))
	}
	robotics = findAndMoveToHand(t, e, 0, "Robotics Mastery")
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o.Face().Name == "Supportive Parents" {
			parents = id
		}
		o.SummonSick = false // summoning sickness is not what these tests exercise
	}
	if parents == 0 {
		t.Fatal("Supportive Parents is not on the battlefield")
	}
	return e, parents, robotics
}

// TestScriptAnswersEnumeratesEveryPairOfAnOddTapElection: Supportive Parents'
// "Tap two untapped creatures you control" cost asks a Min == Max == 2 object
// election over five candidates. Five is not a multiple of two, so the
// unit-major colour arm cannot answer it; every C(5,2) pair must be tried.
func TestScriptAnswersEnumeratesEveryPairOfAnOddTapElection(t *testing.T) {
	t.Parallel()
	e, parents, _ := scriptTapElectionBoard(t, 4)
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("expected a priority decision, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "activate" && o.Obj == parents {
			idx = o.Index
			break
		}
	}
	if idx < 0 {
		t.Fatalf("Supportive Parents' mana ability is not offered: %+v", d.Options)
	}
	c := e.Clone()
	if err := c.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit the Parents activation: %v", err)
	}
	nd := c.Pending()
	if nd == nil || nd.Kind != decision.KChoose || nd.Min != 2 || nd.Max != 2 {
		t.Fatalf("expected the Min == Max == 2 tap election after the activation, got %+v", nd)
	}
	if len(nd.Options) != 5 {
		t.Fatalf("the tap election has %d options, want 5: %+v", len(nd.Options), nd.Options)
	}
	for i := range nd.Options {
		if nd.Options[i].Kind != "tapcost" {
			t.Fatalf("option %d is %q, want a tapcost option: %+v", i, nd.Options[i].Kind, nd.Options[i])
		}
	}
	answers := scriptAnswers(nd)
	if len(answers) != 10 {
		t.Fatalf("scriptAnswers returned %d answers, want all 10 pairs: %+v", len(answers), answers)
	}
	seen := make(map[string]bool, len(answers))
	for _, in := range answers {
		if err := nd.Validate(in); err != nil {
			t.Fatalf("scriptAnswers returned an answer Validate rejects: %v: %+v", err, in)
		}
		set := append([]int(nil), in.Choices...)
		sort.Ints(set)
		key := fmt.Sprint(set)
		if seen[key] {
			t.Fatalf("scriptAnswers returned the pair %v twice: %+v", set, answers)
		}
		seen[key] = true
	}
}

// TestPrefixScriptPaysThroughTwoTapElections: with five untapped creatures,
// Supportive Parents' mana is two tap elections over five candidates. The
// prefix search must find the line that activates it twice and still cast
// Robotics Mastery off the three Islands.
func TestPrefixScriptPaysThroughTwoTapElections(t *testing.T) {
	t.Parallel()
	e, parents, robotics := scriptTapElectionBoard(t, 4)
	inHand := false
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if id == robotics {
			inHand = true
		}
	}
	if !inHand {
		t.Fatal("Robotics Mastery is not in seat 0's hand")
	}
	creatures, islands := 0, 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o.Tapped {
			continue
		}
		if o.Face().IsCreature() {
			creatures++
		}
		if o.Face().Name == "Island" {
			islands++
		}
	}
	if creatures != 5 || islands != 3 {
		t.Fatalf("board has %d untapped creatures and %d untapped Islands, want 5 and 3", creatures, islands)
	}
	var found bool
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "cast" || pp.Action.Obj != robotics {
			continue
		}
		found = true
		if pp.Reason != "" || len(pp.Script) == 0 {
			t.Fatalf("Robotics Mastery: reason %q detail %q, %d script steps, want a script", pp.Reason, pp.Detail, len(pp.Script))
		}
		acts := 0
		for _, st := range pp.Script {
			for _, pk := range st.Picks {
				if pk.Kind == "activate" && pk.Obj == parents {
					acts++
				}
			}
		}
		if acts != 2 {
			t.Fatalf("the script activates Supportive Parents %d times, want 2: %+v", acts, pp.Script)
		}
	}
	if !found {
		t.Fatal("Robotics Mastery is not among seat 0's potential plays")
	}
}

// TestScriptAnswersKeepsTheUnitMajorColourAllocation: a Min == Max == 2
// colour allocation is two units laid out per unit, not an object election,
// and keeps producing every per-unit combination in the same order.
func TestScriptAnswersKeepsTheUnitMajorColourAllocation(t *testing.T) {
	t.Parallel()
	colours := []string{"W", "U", "B", "R", "G"}
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KChoose, Min: 2, Max: 2}
	// Unit-major layout: the first five options are unit 0, the next five unit 1.
	for u := 0; u < 2; u++ {
		for _, sym := range colours {
			d.Options = append(d.Options, decision.Option{
				Index: len(d.Options), Kind: "mana", Label: "Add " + sym, ManaSymbol: sym,
			})
		}
	}
	if len(d.Options) != 10 {
		t.Fatalf("built %d options, want 10", len(d.Options))
	}
	answers := scriptAnswers(d)
	if len(answers) != 25 {
		t.Fatalf("scriptAnswers returned %d answers, want the 25 per-unit combinations", len(answers))
	}
	var want [][]int
	for u0 := 0; u0 < len(colours); u0++ {
		for u1 := 0; u1 < len(colours); u1++ {
			want = append(want, []int{u0, len(colours) + u1})
		}
	}
	for i, in := range answers {
		if len(in.Choices) != 2 || in.Choices[0] != want[i][0] || in.Choices[1] != want[i][1] {
			t.Fatalf("answer %d is %v, want %v (unit-major order changed)", i, in.Choices, want[i])
		}
	}
}
