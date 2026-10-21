package bot

import (
	"context"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

type unwrapper interface{ UnwrapSeat() seat.Seat }

func TestCommonSenseOffReturnsThePlainSeat(t *testing.T) {
	s, err := bots.New("bot", bots.Options{Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*heuristic.Bot); !ok {
		t.Fatalf("manual bot = %T, want *heuristic.Bot", s)
	}
	s, err = bots.New("bot", bots.Options{Seed: 1, AutoPayMana: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.(*heuristic.Bot); !ok {
		t.Fatalf("auto-pay bot = %T, want *heuristic.Bot", s)
	}
}

func TestCommonSenseOnWrapsAndKeepsSurfaces(t *testing.T) {
	for _, auto := range []bool{false, true} {
		s, err := bots.New("bot", bots.Options{Seed: 1, AutoPayMana: auto, CommonSense: true})
		if err != nil {
			t.Fatal(err)
		}
		u, ok := s.(unwrapper)
		if !ok {
			t.Fatalf("auto=%v: %T is not wrapped", auto, s)
		}
		if _, ok := u.UnwrapSeat().(*heuristic.Bot); !ok {
			t.Fatalf("auto=%v: inner = %T, want *heuristic.Bot", auto, u.UnwrapSeat())
		}
		if c, ok := s.(seat.TargetHintConsumer); !ok || !c.WantsTargetHints() {
			t.Fatalf("auto=%v: wrapper does not request target hints", auto)
		}
		pc, ok := s.(seat.PaymentPlanConsumer)
		if !ok || pc.WantsPaymentActions() != auto {
			t.Fatalf("auto=%v: payment-plan opt-in not delegated (%v)", auto, ok)
		}
	}
}

func TestCommonSenseIgnoredByOtherPolicies(t *testing.T) {
	for _, name := range []string{"lethal-pressure", "sb-uniform", "sb-first"} {
		s, err := bots.New(name, bots.Options{Seed: 1, AutoPayMana: true, CommonSense: true})
		if err != nil {
			continue // needs deps; not wrapped either way
		}
		if _, ok := s.(unwrapper); ok {
			t.Errorf("%s was wrapped; CommonSense is for bot and sb-tactical only", name)
		}
	}
}

func mulliganConfig(t *testing.T, seed uint64) rules.Config {
	t.Helper()
	_, decks := testutil.SampleDecks(t, 2)
	return rules.Config{Seed: seed, Names: []string{"p0", "p1"}, Decks: decks, Mulligans: 2}
}

// TestCommonSenseOffMulliganIsTheForgeRule pins the off path: with
// CommonSense false every London ask is answered by MulliganForge, for both
// mana modes, over whole rounds.
func TestCommonSenseOffMulliganIsTheForgeRule(t *testing.T) {
	ctx := context.Background()
	var board botpolicy.Board
	asks, differs := 0, 0
	for _, auto := range []bool{false, true} {
		for seed := uint64(1); seed <= 20; seed++ {
			s, err := bots.New("bot", bots.Options{Seed: seed, AutoPayMana: auto})
			if err != nil {
				t.Fatal(err)
			}
			e := rules.New(mulliganConfig(t, seed))
			for step := 0; step < 400; step++ {
				e.Advance()
				d := e.Pending()
				if d == nil || d.Kind != decision.KMulligan {
					break
				}
				b := botpolicy.BoardFromGameInto(e.G, e, d.Player, &board)
				got, err1 := s.(heuristic.BoardSeat).DecideBoard(ctx, b, *d)
				want, ok := botpolicy.MulliganAnswer(b, botpolicy.MulliganForge, d)
				if err1 != nil || !ok || !slices.Equal(got.Choices, want) {
					t.Fatalf("auto=%v seed %d: off answer %v (%v), forge rule %v (ok %v)", auto, seed, got.Choices, err1, want, ok)
				}
				coin, _ := botpolicy.MulliganAnswer(b, botpolicy.MulliganCoin, d)
				if !slices.Equal(want, coin) {
					differs++
				}
				asks++
				if err := e.Submit(got); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	if asks == 0 || differs == 0 {
		t.Fatalf("asks %d, forge differs from the coin %d: the round never exercised the rule", asks, differs)
	}
}

// TestCommonSenseOnMulliganIsTheForgeRule drives real London rounds and
// checks the guarded bot answers every ask with botpolicy.MulliganForge on
// the same Board, and that the engine accepts it.
func TestCommonSenseOnMulliganIsTheForgeRule(t *testing.T) {
	ctx := context.Background()
	var board botpolicy.Board
	asks, mulls := 0, 0
	for _, auto := range []bool{false, true} {
		for seed := uint64(1); seed <= 20; seed++ {
			s, err := bots.New("bot", bots.Options{Seed: seed, AutoPayMana: auto, CommonSense: true})
			if err != nil {
				t.Fatal(err)
			}
			e := rules.New(mulliganConfig(t, seed))
			for step := 0; step < 400; step++ {
				e.Advance()
				d := e.Pending()
				if d == nil || d.Kind != decision.KMulligan {
					break
				}
				b := botpolicy.BoardFromGameInto(e.G, e, d.Player, &board)
				got, err := s.(heuristic.BoardSeat).DecideBoard(ctx, b, *d)
				if err != nil {
					t.Fatal(err)
				}
				want, ok := botpolicy.MulliganAnswer(b, botpolicy.MulliganForge, d)
				if !ok || !slices.Equal(got.Choices, want) {
					t.Fatalf("auto=%v seed %d: guarded %v, rule %v (ok %v)", auto, seed, got.Choices, want, ok)
				}
				asks++
				if d.Options[got.Choices[0]].Kind == "mulligan" {
					mulls++
				}
				if err := e.Submit(got); err != nil {
					t.Fatalf("engine refused: %v", err)
				}
			}
		}
	}
	if asks == 0 || mulls == 0 {
		t.Fatalf("asks %d, mulligans %d: the round never posed a mulligan", asks, mulls)
	}
}
