package sbsearch

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/engine/seat"
)

// TestExplainNotesDescribeTheSearch: with a note sink installed, every
// decision the search path answers emits one note, the searched ones carry
// the search's table (one row per root, sb-tactical's pick marked Bot, the
// played root marked Chosen, values in [-1,1]), and installing the sink
// never changes an answer: the game's chain head is the one played without
// it.
func TestExplainNotesDescribeTheSearch(t *testing.T) {
	g := newTestGame(t)
	plain, _ := g.play(t, testSeed, func(s uint64) seat.Seat { return New(g.tactical(s), s, quick()) })

	ds := watch(t)
	var notes []explain.Note
	noted, _ := g.play(t, testSeed, func(s uint64) seat.Seat {
		st := New(g.tactical(s), s, quick())
		st.SetExplain(func(n explain.Note) { notes = append(notes, n) })
		return st
	})
	if noted != plain {
		t.Fatalf("chain head with notes %s, without %s: a note sink changed an answer", noted, plain)
	}
	searched := 0
	for _, d := range *ds {
		if d.Worlds > 0 {
			searched++
		}
	}
	var withSearch int
	for i, n := range notes {
		if n.Search == nil {
			if n.Source == explain.Policy {
				t.Errorf("note %d: source policy without a search table", i)
			}
			continue
		}
		withSearch++
		if n.Source != explain.Policy || n.Search.Engine != "sbsearch" || n.Search.Sims <= 0 {
			t.Errorf("note %d: source %q engine %q sims %d, want a policy note from sbsearch", i, n.Source, n.Search.Engine, n.Search.Sims)
		}
		chosen, bot := 0, 0
		for _, r := range n.Search.Rows {
			if r.Chosen {
				chosen++
			}
			if r.Bot {
				bot++
			}
			if r.Q < -1 || r.Q > 1 {
				t.Errorf("note %d: row Q %v outside [-1,1]", i, r.Q)
			}
		}
		if chosen != 1 || bot != 1 {
			t.Errorf("note %d: %d chosen and %d bot rows, want one each: %+v", i, chosen, bot, n.Search.Rows)
		}
	}
	if searched == 0 || withSearch != searched {
		t.Fatalf("%d decisions searched with a valid world, %d notes carry a search table; want them equal and non-zero", searched, withSearch)
	}
	t.Logf("%d notes, %d with a search table", len(notes), withSearch)
}
