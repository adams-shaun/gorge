package host

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	sbseat "github.com/adams-shaun/gorge/server/spellbench/seat"
)

type recordingSeat struct {
	seat.Seat
	mu     sync.Mutex
	ctx    *bots.MatchContext
	result bots.MatchResult
	ended  bool
}

func (s *recordingSeat) AttachMatch(mc *bots.MatchContext) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx = mc
	return nil
}

func (s *recordingSeat) EndMatch(result bots.MatchResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result, s.ended = result, true
}

var _ bots.MatchSeat = (*recordingSeat)(nil)
var _ bots.MatchEnd = (*recordingSeat)(nil)

func TestTableRefusesPolicyOverItsMaxSeats(t *testing.T) {
	const name = "host-test-two-seat-policy"
	sbseat.Register(name, []string{"true"})
	_, err := (TableConfig{ID: "t1", Seats: 3, SeatBotPolicies: []string{name}, AutoMana: true, BotAutoPayMana: true}).validatedSeatPolicies(gorgegame.Engine{})
	if err == nil || !strings.Contains(err.Error(), "seats at most 2") {
		t.Fatalf("validation error = %v, want a MaxSeats refusal", err)
	}
}

func TestTableRefusesDefaultPolicyOverItsMaxSeats(t *testing.T) {
	const name = "host-test-two-seat-default-policy"
	sbseat.Register(name, []string{"true"})
	_, err := (TableConfig{
		ID: "t1", Seats: 3, BotPolicy: name, Decks: []string{"a", "b", "c"},
		Spectator: view.Public, AutoMana: true, BotAutoPayMana: true,
	}).validated(func(string) (Deck, error) { return Deck{}, nil }, map[string]game.Engine{gorgegame.Name: gorgegame.Engine{}})
	if err == nil || !strings.Contains(err.Error(), "seats at most 2") {
		t.Fatalf("validation error = %v, want a MaxSeats refusal", err)
	}
}

func TestHostAttachesAndEndsMatchSeats(t *testing.T) {
	var mu sync.Mutex
	var seats []*recordingSeat
	o := testOptions(t)
	o.BotDeps = bots.Deps{Cards: &cards.Registry{}}
	o.Seats = func(names []string, seed uint64) []seat.Seat {
		mu.Lock()
		defer mu.Unlock()
		seats = nil
		for i := range names {
			seats = append(seats, &recordingSeat{Seat: heuristic.NewBot(seed + uint64(i))})
		}
		out := make([]seat.Seat, len(seats))
		for i, s := range seats {
			out[i] = s
		}
		return out
	}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(TableConfig{ID: "t1", Seats: 2, Decks: []string{"a", "b"}, Seed: 3, Pace: 0, Spectator: view.Omniscient}); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	mu.Lock()
	defer mu.Unlock()
	if len(seats) != 2 {
		t.Fatalf("built %d seats, want 2", len(seats))
	}
	for i, s := range seats {
		s.mu.Lock()
		if s.ctx == nil {
			t.Fatalf("seat %d was not attached", i)
		}
		if s.ctx.Engine == nil || gorgegame.Rules(s.ctx.Game) != s.ctx.Engine || s.ctx.Seats != 2 || s.ctx.Seat != i {
			t.Fatalf("seat %d context = %+v", i, s.ctx)
		}
		if !strings.HasPrefix(s.ctx.GameID, "g-") || len(s.ctx.Secret) == 0 {
			t.Fatalf("seat %d identity = %q secret %d bytes", i, s.ctx.GameID, len(s.ctx.Secret))
		}
		if s.ctx.Format != "constructed" || s.ctx.OwnDeck == nil {
			t.Fatalf("seat %d format/deck = %q/%v", i, s.ctx.Format, s.ctx.OwnDeck)
		}
		if s.ctx.Registry != o.BotDeps.Cards {
			t.Fatalf("seat %d registry = %p, want the host's BotDeps.Cards", i, s.ctx.Registry)
		}
		if !s.ended || s.result.Classification != "natural" {
			t.Fatalf("seat %d end = %v %+v", i, s.ended, s.result)
		}
		s.mu.Unlock()
	}
}
