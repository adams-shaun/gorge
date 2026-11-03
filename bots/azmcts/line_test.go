package azmcts

import (
	"context"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/engine/decision"
)

// lineTree is root -> "a" -> {x: 5 visits, y: 2 visits} -> x -> {p: 3}, with
// "q" a hidden candidate that is visited most below p.
func lineTree() *edge {
	deep := &node{kids: []*edge{
		{key: "p", label: "Attack with Bear", n: 3, w: 2.1},
		{key: "q", label: "Cast a drawn card", n: 4, w: 2, hidden: true},
	}}
	mid := &node{kids: []*edge{
		{key: "x", label: "Cast Shock", n: 5, w: 3, next: &node{kids: []*edge{
			{key: "p", label: "Pass", n: 3, w: 1.8, opp: true, next: deep},
		}}},
		{key: "y", label: "Pass", n: 2, w: 1},
	}}
	return &edge{key: "a", n: 7, next: mid}
}

func TestPrincipalLineFollowsTheMostVisitedChild(t *testing.T) {
	line, end := principalLine(lineTree(), 4)
	want := []LineStep{
		{Label: "Cast Shock", Visits: 5, Share: 5.0 / 7, Q: 0.6},
		{Opp: true, Label: "Pass", Visits: 3, Share: 1, Q: 0.6},
	}
	// The third step is the hidden candidate "q" (4 visits beat "p"'s 3):
	// the line stops before it rather than skip to "p".
	if end != LineEndHidden {
		t.Errorf("end %q, want %q", end, LineEndHidden)
	}
	if !reflect.DeepEqual(line, want) {
		t.Errorf("line\n got %+v\nwant %+v", line, want)
	}
}

func TestPrincipalLineStopsAtTheRequestedLengthAndAtUnexplored(t *testing.T) {
	if line, end := principalLine(lineTree(), 1); len(line) != 1 || end != "" {
		t.Errorf("1 ply: %+v end %q", line, end)
	}
	if line, end := principalLine(&edge{key: "a", n: 1}, 4); len(line) != 0 || end != LineEndUnexplored {
		t.Errorf("unexpanded: %+v end %q", line, end)
	}
	fresh := &edge{next: &node{kids: []*edge{{key: "k"}}}}
	if line, end := principalLine(fresh, 4); len(line) != 0 || end != LineEndUnexplored {
		t.Errorf("unvisited children: %+v end %q", line, end)
	}
}

func TestRunTreeKeepsItsRootOnlyForALine(t *testing.T) {
	g := &fakeGame{nodes: map[string]fakeNode{
		"":   {keys: []Key{"a", "b"}, prior: []float64{0.5, 0.5}, value: 0.5},
		"/a": {value: 0.1, terminal: true},
		"/b": {value: 0.9, terminal: true},
	}}
	for _, line := range []int{0, 3} {
		o := treeOpts(4)
		o.Line = line
		var st Stats
		tr, err := RunTree(context.Background(), rootOf(g), &fakeSource{g: g}, o, &st)
		if err != nil {
			t.Fatal(err)
		}
		if (tr.top != nil) != (line > 0) {
			t.Errorf("Line %d: top kept = %v", line, tr.top != nil)
		}
	}
}

// Asking for the line reads the tree only: the choice, the visits and the
// values are those of a search that did not ask.
func TestSearchLineDoesNotMoveTheSearch(t *testing.T) {
	cfg := testConfig(t, "mono-red-prowess", "mono-blue-tempo", testSeed)
	e, d, bot := botPosition(t, cfg, decision.KPriority, 2, 3000)
	opts := DefaultOptions()
	opts.Sims, opts.Seed = 40, 11
	plain := searchAt(t, e, d, bot, nil, opts)
	opts.Line = 4
	with := searchAt(t, e, d, bot, nil, opts)
	if len(with.Line) > 4 {
		t.Errorf("line of %d plies, asked for 4", len(with.Line))
	}
	for i, st := range with.Line {
		if st.Label == "" || st.Visits < 1 || st.Share <= 0 || st.Share > 1 || st.Q < 0 || st.Q > 1 {
			t.Errorf("ply %d malformed: %+v", i, st)
		}
	}
	if len(with.Line) == 0 {
		t.Fatalf("fixture: no line (end %q)", with.LineEnd)
	}
	with.Line, with.LineEnd = nil, ""
	if !reflect.DeepEqual(plain, with) {
		t.Fatalf("Line moved the search:\n%+v\n%+v", plain, with)
	}
}

// The note words each ply by who acts, and rounds its numbers like a row's.
func TestSearchRowsWordTheLine(t *testing.T) {
	s := &Seat{}
	res := Result{
		Candidates: []decision.Intent{{}, {Choices: []int{1}}}, Visits: []int{1, 3}, Q: []float64{0.5, 0.6}, Choice: 1,
		Line:    []LineStep{{Label: "Cast Shock", Visits: 3, Share: 2.0 / 3, Q: 0.75}, {Opp: true, Label: "Pass", Visits: 1, Share: 1, Q: 0.5}},
		LineEnd: LineEndHidden,
	}
	got := s.searchRows(&searchNote{res: res, searchedOK: true}).Line
	want := []explain.Ply{
		{Actor: "you", Label: "Cast Shock", Visits: 3, Share: 0.667, Q: 0.5},
		{Actor: "opponent", Label: "Pass", Visits: 1, Share: 1, Q: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v want %+v", got, want)
	}
}
