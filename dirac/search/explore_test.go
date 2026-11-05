package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestExploreDeterministicAndOff: explore is a pure function of the seed,
// leaves the recorded prior clean, and with Sample picks a visited move;
// with both knobs off the search is the eval search.
func TestExploreDeterministicAndOff(t *testing.T) {
	eo := DefaultOptions()
	eo.Noise, eo.Sample = true, true
	ex := NewSearcher(eo, Uniform{}, Heuristic{})
	ev := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
	var s dirac.State
	s.Init(v.Table, v.Decks[0], v.Decks[1], 7, dirac.Config{})
	pol := dirac.NewRandomPolicy(3)
	var d dirac.Decision
	checked, differ := 0, 0
	for i := 0; i < 400 && checked < 20; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if d.Player == 0 {
			a := *ex.Search(&s, 99)
			b := *ex.Search(&s, 99)
			if a.Visits != b.Visits || a.Choice != b.Choice {
				t.Fatalf("decision %d: explore searches differ", i)
			}
			c := *ev.Search(&s, 99)
			if a.Prior != c.Prior {
				t.Fatalf("decision %d: recorded prior is not the clean prior", i)
			}
			if a.Stats.Searched > 0 {
				checked++
				if a.Visits[a.Choice] == 0 {
					t.Fatalf("decision %d: sampled an unvisited move", i)
				}
				if a.Visits != c.Visits || a.Choice != c.Choice {
					differ++
				}
			}
			c.Play(&s)
			continue
		}
		s.Apply(pol.Pick(&d))
	}
	if checked == 0 || differ == 0 {
		t.Fatalf("checked %d searched decisions, %d differ from the eval search", checked, differ)
	}
}
