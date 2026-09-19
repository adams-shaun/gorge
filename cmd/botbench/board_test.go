package main

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/host"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// These wrappers exercise the real Bot through both supported interfaces,
// recording its actual intents rather than supplying canned decisions.
type viewTraceSeat struct {
	bot   *seat.Bot
	trace *[]decision.Intent
	calls int
}

func (s *viewTraceSeat) record(in decision.Intent) {
	in.Choices = slices.Clone(in.Choices)
	*s.trace = append(*s.trace, in)
}

func (s *viewTraceSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	s.calls++
	in, err := s.bot.Decide(ctx, v, d)
	s.record(in)
	return in, err
}

type boardTraceSeat struct {
	*viewTraceSeat
	boardCalls int
}

func (s *boardTraceSeat) DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	s.boardCalls++
	in, err := s.bot.DecideBoard(ctx, b, d)
	s.record(in)
	return in, err
}

func TestPlayMatchUsesBoardSeatWithViewParity(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := []string{"death-n-taxes", "dimir-tempo"}
	decks := make([][]*cards.Card, len(names))
	for i, name := range names {
		decks[i] = testutil.RepoDeck(t, reg, name)
	}
	for _, tc := range []struct {
		name string
		new  func(uint64) *seat.Bot
	}{{"bot", seat.NewBot}, {"lethal-pressure", seat.NewLethalPressureBot}} {
		for _, seed := range []uint64{0, 7} {
			cfg := rules.Config{Seed: seed, Names: names, Decks: decks, Tokens: reg.Tokens}
			var boardTrace, viewTrace []decision.Intent
			boardSeats, viewSeats := make([]seat.Seat, 2), make([]seat.Seat, 2)
			for i := range boardSeats {
				botSeed := seed ^ uint64(i+1)
				boardSeats[i] = &boardTraceSeat{viewTraceSeat: &viewTraceSeat{bot: tc.new(botSeed), trace: &boardTrace}}
				viewSeats[i] = &viewTraceSeat{bot: tc.new(botSeed), trace: &viewTrace}
			}
			got, err := playMatch(cfg, names, boardSeats, 200, 20000, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			want, err := playMatch(cfg, names, viewSeats, 200, 20000, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if got.stallOn != "" || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(boardTrace, viewTrace) {
				t.Fatalf("%s seed %d: Board and View must finish with identical intents and outcomes", tc.name, seed)
			}
			for i, s := range boardSeats {
				b := s.(*boardTraceSeat)
				if b.boardCalls == 0 || b.calls != 0 {
					t.Fatalf("seat %d: Board calls=%d, View calls=%d; want Board only", i, b.boardCalls, b.calls)
				}
			}
		}
	}
}

// TestAr8PolicyIsBenchedButNotHosted pins the brief's exposure rule: the
// "ar8" policy is registered in the bench's policies map and builds a seat,
// but host.NormalizeBotPolicy does NOT know the name -- the combined-attacker
// experiment can only be benched, never hosted on a live table.
func TestAr8PolicyIsBenchedButNotHosted(t *testing.T) {
	newSeat, ok := policies["ar8"]
	if !ok {
		t.Fatal(`policies["ar8"] is not registered`)
	}
	if s := newSeat(1); s == nil {
		t.Fatal("policies[\"ar8\"] built a nil seat")
	}
	if _, err := host.NormalizeBotPolicy("ar8"); err == nil {
		t.Fatal("host.NormalizeBotPolicy accepted \"ar8\"; the ar8 policy must NOT be hosted")
	}
}
