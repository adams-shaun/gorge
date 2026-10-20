package bench

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/bots/explain"
)

// TestExplainTapeDriverRouteKeepsTheSearch: a driver route overrides the
// seat's Source (and Reason, Rung) but keeps the Search the seat reported;
// a seat that reported nothing is noted heuristic; the frozen note survives
// a re-ask; and a note's slices are the tape's own copy.
func TestExplainTapeDriverRouteKeepsTheSearch(t *testing.T) {
	x := &explainTape{policies: []string{"az-redeal", "bot"}}
	choices := []int{3}
	x.begin(0)
	x.capture(explain.Note{Seat: 0, Source: explain.Policy, Bot: choices,
		Search: &explain.Search{Engine: "azmcts", Rows: []explain.Row{{Choices: choices, Visits: 5, Chosen: true, Bot: true}}}})
	x.route(explain.FallbackRoot, "deal refused", 0)
	choices[0] = 99 // the seat's scratch is reused before commit
	x.commit(0)

	x.begin(1)
	x.commit(1)

	x.begin(0)
	x.capture(explain.Note{Source: explain.Policy, Reason: "first"})
	x.refused()
	x.capture(explain.Note{Source: explain.Heuristic, Reason: "re-ask"})
	x.route(explain.RefusalFallback, "illegal", 2)
	x.commit(2)

	notes, err := x.final(3)
	if err != nil {
		t.Fatal(err)
	}
	var got [3]explain.Note
	for i, raw := range notes {
		if err := json.Unmarshal(raw, &got[i]); err != nil {
			t.Fatal(err)
		}
	}
	if n := got[0]; n.Source != explain.FallbackRoot || n.Reason != "deal refused" || n.Policy != "az-redeal" ||
		n.Search == nil || n.Search.Rows[0].Choices[0] != 3 || n.Bot[0] != 3 {
		t.Fatalf("routed note %+v (search %+v)", n, n.Search)
	}
	if n := got[1]; n.Source != explain.Heuristic || n.Seat != 1 || n.Policy != "bot" {
		t.Fatalf("plain seat note %+v", n)
	}
	if n := got[2]; n.Source != explain.RefusalFallback || n.Rung != 2 || n.Reason != "illegal" {
		t.Fatalf("ladder note %+v", n)
	}
	if _, err := x.final(4); err == nil {
		t.Fatal("final accepted an intent count the tape did not answer")
	}
}
