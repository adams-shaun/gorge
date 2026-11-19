//go:build manabrew

package mbtest

import (
	"context"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	manabrew "github.com/adams-shaun/gorge/server/manabrew"
	"github.com/adams-shaun/gorge/server/protocol"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// idRecordSeat records every promptId the translator mints for the seat's
// decisions (and the decision's Seq) before delegating to the usual seat.
type idRecordSeat struct {
	*TranslatingSeat
	ids  []int64
	seqs []uint64
}

func (s *idRecordSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if msg, _, err := s.Translator.PromptFlagged(&d, &v); err == nil {
		s.ids = append(s.ids, msg.PromptID)
		s.seqs = append(s.seqs, d.Seq)
	}
	return s.TranslatingSeat.Decide(ctx, v, d)
}

// TestPromptIDFitsU32FourSeats plays recorded 4-seat games and asserts every
// promptId the wire carries -- decision prompts, the synthetic prompts the
// game's own log mints, and the terminal gameOver -- is in [0, 2^32) (the
// protocol types promptId as u32), and that the three id spaces are disjoint.
func TestPromptIDFitsU32FourSeats(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	names := testutil.LegacyDeckNames()
	const n = 4
	var decisionIDs, syntheticIDs int
	for _, seed := range []uint64{1, 2, 3} {
		playerNames := make([]string, n)
		decks := make([][]*cards.Card, n)
		for i := range playerNames {
			playerNames[i] = names[(int(seed)+i)%len(names)]
			decks[i] = testutil.RepoDeck(t, reg, playerNames[i])
		}
		cfg := rules.Config{Seed: seed, Names: playerNames, Decks: decks, Tokens: reg.Tokens, NameUniverse: reg.Universe()}
		client := NewSeededRandomClient(seed)
		census := NewCensus()
		seats := make([]*idRecordSeat, n)
		ss := make([]seat.Seat, n)
		for i := range ss {
			seats[i] = &idRecordSeat{TranslatingSeat: NewTranslatingSeat("u32", int64(seed), client, census)}
			ss[i] = seats[i]
		}
		_, e, err := bench.PlayGame(cfg, ss, censusMaxTurns, censusMaxIntents, bench.Hooks{})
		if err != nil {
			t.Fatalf("seed %d: PlayGame: %v", seed, err)
		}
		inU32 := func(what string, id int64) {
			t.Helper()
			if id < 0 || id >= math.MaxUint32+1 {
				t.Fatalf("seed %d: %s promptId %d outside [0, 2^32)", seed, what, id)
			}
		}
		for _, s := range seats {
			for i, id := range s.ids {
				inU32("decision", id)
				if id != int64(s.seqs[i]) || uint64(id) >= manabrew.MaxDecisionSeq {
					t.Fatalf("seed %d: decision id %d is not its Seq %d below %d", seed, id, s.seqs[i], manabrew.MaxDecisionSeq)
				}
				decisionIDs++
			}
		}
		evs := make([]protocol.EventBody, len(e.L.Events))
		for i, ev := range e.L.Events {
			evs[i] = protocol.EventBody{Event: protocol.EventFrom(ev)}
		}
		for _, msg := range manabrew.New("u32", int64(seed), nil).Synthetic(evs, nil) {
			pm := msg.Value.(mb.PromptMessage)
			inU32("synthetic", pm.PromptID)
			if pm.PromptID < manabrew.SyntheticPromptIDFloor || pm.PromptID >= manabrew.GameOverPromptID {
				t.Fatalf("seed %d: synthetic id %d outside [%d, %d)", seed, pm.PromptID, manabrew.SyntheticPromptIDFloor, manabrew.GameOverPromptID)
			}
			syntheticIDs++
		}
		inU32("gameOver", manabrew.GameOverPromptID)
	}
	if decisionIDs == 0 {
		t.Fatal("no decision prompts recorded: the precondition of the range check is unmet")
	}
	t.Logf("%d decision ids, %d synthetic ids checked", decisionIDs, syntheticIDs)
}
