package azmcts

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	gbench "github.com/adams-shaun/gorge/testkit/bench"
)

// TestNoteSearchConvertsUnits pins the note's units over a synthetic
// Result: azmcts's [0,1] values become [-1,1] (FromUnit), an unvisited
// candidate reads 0, candidate 0 is the bot row and the played candidate
// the chosen row; a tap-reroot root (no bot candidate) marks no bot row.
func TestNoteSearchConvertsUnits(t *testing.T) {
	var got []explain.Note
	s := &Seat{note: func(n explain.Note) { got = append(got, n) }}
	d := &decision.Decision{Player: 1, Kind: decision.KAttackers}
	res := Result{
		Kind:       "attackers",
		Candidates: []decision.Intent{{Choices: []int{}}, {Choices: []int{0}}, {Choices: []int{0, 1}}},
		Visits:     []int{3, 0, 5}, Q: []float64{1, 0, 0.25}, Prior: []float64{0.5, 0.25, 0.25},
		RootValue: 0.75, Choice: 2,
	}
	res.Stats.Searched, res.Stats.Completed = 1, 8
	bot := decision.Intent{Choices: []int{}}
	s.noteSearch(searchNote{d: d, bot: bot, res: res, searchedOK: true})
	s.noteSearch(searchNote{d: d, bot: decision.Intent{Choices: []int{7}}, res: res, searchedOK: true, noBot: true})
	if len(got) != 2 {
		t.Fatalf("%d notes", len(got))
	}
	n := got[0]
	if n.Source != explain.Policy || n.Seat != 1 || n.Search == nil || n.Search.RootValue != 0.5 || n.Search.Sims != 8 {
		t.Fatalf("note %+v search %+v", n, n.Search)
	}
	want := []explain.Row{
		{Choices: []int{0, 1}, Visits: 5, Prior: 0.25, Q: -0.5, Chosen: true},
		{Choices: []int{}, Visits: 3, Prior: 0.5, Q: 1, Bot: true},
		{Choices: []int{0}, Visits: 0, Prior: 0.25, Q: 0},
	}
	if !slices.EqualFunc(n.Search.Rows, want, rowEqual) {
		t.Fatalf("rows %+v, want %+v", n.Search.Rows, want)
	}
	for _, r := range got[1].Search.Rows {
		if r.Bot {
			t.Fatalf("a no-bot root marked a bot row: %+v", got[1].Search.Rows)
		}
	}
	if !slices.Equal(got[1].Bot, []int{7}) {
		t.Fatalf("no-bot note's bot %v, want the bot's own answer", got[1].Bot)
	}
}

func rowEqual(a, b explain.Row) bool {
	return slices.Equal(a.Choices, b.Choices) && a.Visits == b.Visits && a.Prior == b.Prior && a.Q == b.Q && a.Chosen == b.Chosen && a.Bot == b.Bot
}

// TestNoteValuesAreTheDecidingSeatsAtAKnownWin checks the perspective
// against positions whose answer is known: in a game the az seat goes on to
// win, its last searched decision is (clairvoyantly) a won position, so the
// played row's Q and the root value must be positive -- from the deciding
// seat's side, at either seat. A value written from a fixed seat's side, or
// the opponent's, reads negative at one of the two. (The [0,1] -> [-1,1]
// scaling itself is TestNoteSearchConvertsUnits'.)
func TestNoteValuesAreTheDecidingSeatsAtAKnownWin(t *testing.T) {
	won := map[int]bool{}
	for g := uint64(0); g < 8 && !(won[0] && won[1]); g++ {
		azSeat := int(g % 2)
		if won[azSeat] {
			continue
		}
		cfg := testConfig(t, "mono-red-prowess", "mono-blue-tempo", testSeed+g)
		sc := DefaultSeatConfig()
		sc.Search.Sims = 32
		sc.Source = testSeatSource
		az, err := NewSeat(cfg.Seed^uint64(azSeat+1), nil, sc)
		if err != nil {
			t.Fatal(err)
		}
		var last *explain.Note
		az.SetExplain(func(n explain.Note) {
			if n.Source == explain.Policy && n.Search != nil {
				last = &n
			}
		})
		seats := []seat.Seat{az, heuristic.NewBot(cfg.Seed ^ 2)}
		if azSeat == 1 {
			seats = []seat.Seat{heuristic.NewBot(cfg.Seed ^ 1), az}
		}
		o, _, err := gbench.PlayGame(cfg, seats, 60, 3000, gbench.Hooks{})
		if err != nil {
			t.Fatal(err)
		}
		if o.IsStalled() || o.Draw || o.WinnerSeat != azSeat || last == nil {
			continue
		}
		if last.Seat != azSeat {
			t.Fatalf("az at seat %d emitted a note for seat %d", azSeat, last.Seat)
		}
		var q float64
		for _, r := range last.Search.Rows {
			if r.Chosen {
				q = r.Q
			}
		}
		t.Logf("seed %d: az at seat %d won; last searched decision: root value %v, played Q %v", cfg.Seed, azSeat, last.Search.RootValue, q)
		if q <= 0 || last.Search.RootValue <= 0 {
			t.Fatalf("seed %d: az (seat %d) won, but its last searched decision reads root %v, played Q %v: not the deciding seat's [-1,1] value",
				cfg.Seed, azSeat, last.Search.RootValue, q)
		}
		won[azSeat] = true
	}
	if !won[0] || !won[1] {
		t.Fatalf("az won at seats %v in 8 games; the known-win check needs a win at each seat", won)
	}
}
