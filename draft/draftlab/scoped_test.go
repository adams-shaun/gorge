package draftlab

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
)

// scopedBot is a test seat: a real *heuristic.Bot that also implements GameScoped
// and records its calls. It shadows Decide to count the decisions it
// answers, so a test can assert EndGame came after the game's last one.
type scopedBot struct {
	*heuristic.Bot
	decides      int
	decidesAtEnd int
	begins       []*GameToken
	ends         []*GameToken
}

func (s *scopedBot) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	s.decides++
	return s.Bot.Decide(ctx, v, d)
}

// DecideBoard shadows the promoted one: *heuristic.Bot is a heuristic.BoardSeat, so
// the bench driver answers from the Board path, and the decision counter
// must live there.
func (s *scopedBot) DecideBoard(ctx context.Context, brd botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	s.decides++
	return s.Bot.DecideBoard(ctx, brd, d)
}

func (s *scopedBot) BeginGame(t *GameToken) { s.begins = append(s.begins, t) }
func (s *scopedBot) EndGame(t *GameToken) {
	s.decidesAtEnd = s.decides
	s.ends = append(s.ends, t)
}

// scopedRecorder collects every scopedBot a run constructs, across the
// concurrent games of a worker run.
type scopedRecorder struct {
	mu    sync.Mutex
	seats []*scopedBot
}

func (r *scopedRecorder) newBot(b *heuristic.Bot) *scopedBot {
	s := &scopedBot{Bot: b}
	r.mu.Lock()
	r.seats = append(r.seats, s)
	r.mu.Unlock()
	return s
}

func (r *scopedRecorder) snapshot() []*scopedBot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := append([]*scopedBot(nil), r.seats...)
	return out
}

// scopedEntrant wraps deck's player so each game seat is a recorded
// scopedBot over a fresh bot.
func scopedEntrant(t *testing.T, l *Lab, deck Deck, rec *scopedRecorder) Entrant {
	t.Helper()
	p := testPlayer(t, l, "bot")
	return Entrant{Deck: deck, Player: NamedPlayer{
		Ref: p.Ref,
		New: func(seed uint64) (seat.Seat, error) {
			st, err := p.New(seed)
			if err != nil {
				return nil, err
			}
			return rec.newBot(st.(*heuristic.Bot)), nil
		},
	}}
}

// TestGameScopedSeesBeginAndEnd: each seat got exactly one BeginGame and one
// EndGame, both seats the same token, EndGame after the game's last Decide,
// and two PlayGame calls mint two different tokens.
func TestGameScopedSeesBeginAndEnd(t *testing.T) {
	l := testLab(t)
	es := testEntrants(t, l, 2)
	var rec scopedRecorder
	for i := range es {
		es[i] = scopedEntrant(t, l, es[i].Deck, &rec)
	}
	res, err := l.PlayGame(es[0], es[1], 99)
	if err != nil {
		t.Fatalf("PlayGame: %v", err)
	}
	if res.Intents == 0 {
		t.Fatal("game recorded 0 intents; the begin/end assertions would be vacuous")
	}
	seats := rec.snapshot()
	if len(seats) != 2 {
		t.Fatalf("%d scoped seats recorded, want 2", len(seats))
	}
	var tok *GameToken
	for i, s := range seats {
		if len(s.begins) != 1 || len(s.ends) != 1 {
			t.Fatalf("seat %d: %d begins, %d ends, want 1 of each", i, len(s.begins), len(s.ends))
		}
		if i == 0 {
			tok = s.begins[0]
		}
		if s.begins[0] != tok || s.ends[0] != tok {
			t.Errorf("seat %d: begin/end token is not the game's one token", i)
		}
		if s.decides == 0 {
			t.Errorf("seat %d answered 0 decisions; EndGame-after-last-Decide is vacuous", i)
		}
		if s.decidesAtEnd != s.decides {
			t.Errorf("seat %d: EndGame saw %d decides, the game's last is %d (end came before the last Decide)", i, s.decidesAtEnd, s.decides)
		}
	}
	// A second game mints a different token.
	var rec2 scopedRecorder
	for i := range es {
		es[i] = scopedEntrant(t, l, es[i].Deck, &rec2)
	}
	if _, err := l.PlayGame(es[0], es[1], 99); err != nil {
		t.Fatalf("PlayGame 2: %v", err)
	}
	seats2 := rec2.snapshot()
	if len(seats2) != 2 {
		t.Fatalf("game 2 recorded %d seats, want 2", len(seats2))
	}
	if seats2[0].begins[0] == tok {
		t.Error("game 2 reused game 1's token")
	}
}

// TestGameScopedEndsOnSeatError: a scoped seat that errors mid-game still
// gets its EndGame, exactly once.
func TestGameScopedEndsOnSeatError(t *testing.T) {
	l := testLab(t)
	es := testEntrants(t, l, 2)
	var ends int
	p := testPlayer(t, l, "bot")
	es[0] = scopedEntrant(t, l, es[0].Deck, new(scopedRecorder))
	es[1].Player = NamedPlayer{Ref: p.Ref, New: func(seed uint64) (seat.Seat, error) {
		st, err := p.New(seed)
		if err != nil {
			return nil, err
		}
		return &failingScoped{scopedBot: &scopedBot{Bot: st.(*heuristic.Bot)}, failAt: 5, onEnd: func() { ends++ }}, nil
	}}
	_, err := l.PlayGame(es[0], es[1], 17)
	if err == nil {
		t.Fatal("PlayGame succeeded despite the seat's 5th-decision error")
	}
	if !strings.Contains(err.Error(), "scoped seat died") {
		t.Errorf("error %q does not carry the seat's own failure", err)
	}
	if ends != 1 {
		t.Errorf("EndGame ran %d times, want exactly 1", ends)
	}
}

// failingScoped errors at its failAt-th decision and counts EndGame calls.
type failingScoped struct {
	*scopedBot
	failAt int
	onEnd  func()
	calls  int
}

func (s *failingScoped) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	s.calls++
	if s.calls == s.failAt {
		return decision.Intent{}, errors.New("scoped seat died")
	}
	return s.scopedBot.Decide(ctx, v, d)
}

func (s *failingScoped) DecideBoard(ctx context.Context, brd botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	s.calls++
	if s.calls == s.failAt {
		return decision.Intent{}, errors.New("scoped seat died")
	}
	return s.scopedBot.DecideBoard(ctx, brd, d)
}

func (s *failingScoped) EndGame(t *GameToken) {
	s.scopedBot.EndGame(t)
	s.onEnd()
}

// TestGameScopedWorkersBalanced: Games over 6 matchups with 3 workers leaves
// as many EndGame calls as BeginGame calls — 12 of each.
func TestGameScopedWorkersBalanced(t *testing.T) {
	l := testLabOpts(t, Options{Workers: 3})
	es := testEntrants(t, l, 6)
	var begins, ends atomic.Int64
	p := testPlayer(t, l, "bot")
	for i := range es {
		np := p
		np.New = func(seed uint64) (seat.Seat, error) {
			st, err := p.New(seed)
			if err != nil {
				return nil, err
			}
			return &countingScoped{Bot: st.(*heuristic.Bot), begins: &begins, ends: &ends}, nil
		}
		es[i].Player = np
	}
	ms := make([]Matchup, 6)
	for i := range ms {
		ms[i] = Matchup{A: es[i], B: es[(i+1)%6]}
	}
	if _, err := l.Games(ms, 555); err != nil {
		t.Fatalf("Games: %v", err)
	}
	if got := begins.Load(); got != 12 {
		t.Errorf("%d BeginGame calls, want 12 (6 games x 2 seats)", got)
	}
	if got := ends.Load(); got != 12 {
		t.Errorf("%d EndGame calls, want 12 (6 games x 2 seats)", got)
	}
}

// countingScoped counts BeginGame/EndGame calls atomically across workers.
type countingScoped struct {
	*heuristic.Bot
	begins, ends *atomic.Int64
}

func (s *countingScoped) BeginGame(*GameToken) { s.begins.Add(1) }
func (s *countingScoped) EndGame(*GameToken)   { s.ends.Add(1) }
