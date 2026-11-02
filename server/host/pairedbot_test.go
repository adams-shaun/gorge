package host

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/azredeal"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/explain"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// pairedTestEntry is the real closed search at a small budget.
func pairedTestEntry(t *testing.T) *bots.Entry {
	t.Helper()
	return &bots.Entry{
		Info: bots.Info{Name: "test-paired", Tier: bots.Experimental, Env: true, Search: true, ClosedLists: true,
			Formats: []string{"constructed"}, MaxSeats: 2},
		New: azredeal.ClosedFactory(enableClosedTest(t), 12),
	}
}

// passSeat always passes, and otherwise gives the fewest choices it may: it
// keeps the public game independent of the cards it holds.
type passSeat struct{}

func (passSeat) Decide(_ context.Context, _ view.View, d decision.Decision) (decision.Intent, error) {
	return minimalAnswer(&d), nil
}

func minimalAnswer(d *decision.Decision) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	for _, o := range d.Options {
		if o.Kind == "pass" {
			in.Choices = []int{o.Index}
			return in
		}
	}
	for i := 0; i < d.Min; i++ {
		in.Choices = append(in.Choices, d.Options[i].Index)
	}
	return in
}

// landFirstAnswer plays a land when it can and otherwise answers minimally, so
// the seat holds mana to search over and the game stays a function of its hand.
func landFirstAnswer(d *decision.Decision) decision.Intent {
	for _, o := range d.Options {
		if o.Kind == "play_land" {
			return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}}
		}
	}
	return minimalAnswer(d)
}

// parkedAfter returns the human seat's decision once it is parked past seq.
func parkedAfter(t *testing.T, r *Registry, player state.PlayerID, after uint64) *decision.Decision {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if d, err := r.Pending("t1", 1, player); err == nil && (d.Seq > after || after == 0) {
			return d
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("no decision parked after seq %d", after)
	return nil
}

func pairedTable(humans []int) TableConfig {
	return TableConfig{ID: "t1", Name: "paired", Seats: 2, Decks: []string{"opponent", "human"}, Seed: 20261031,
		Spectator: view.Public, Humans: humans, BotPolicy: "bot", AutoMana: true, BotAutoPayMana: true}
}

// pairedDecks loads the human's list and an opponent's list under one name, so
// a deck's name cannot tell two games apart.
func pairedDecks(t *testing.T, oppDeck string) func(string) (Deck, error) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	own := testutil.RepoDeck(t, reg, "mono-white-equipment")
	opp := testutil.RepoDeck(t, reg, oppDeck)
	return func(name string) (Deck, error) {
		if name == "human" {
			return Deck{Name: "own", Cards: own}, nil
		}
		return Deck{Name: "opposing", Cards: opp}, nil
	}
}

// Two games whose only difference is the opponent's list and so its hidden
// cards: the seat sees the same game, and the advice has the same bytes at
// every decision the seat is asked before the opponent shows a card.
func TestPairedBotAdviceIgnoresHiddenOpponentCards(t *testing.T) {
	play := func(oppDeck string) ([]protocol.Advice, []string) {
		o := testOptions(t)
		o.LoadDeck = pairedDecks(t, oppDeck)
		o.PairedBot = pairedTestEntry(t)
		o.Seats = func([]string, uint64) []seat.Seat { return []seat.Seat{passSeat{}, passSeat{}} }
		r, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		if err := r.AddTable(pairedTable([]int{0})); err != nil {
			t.Fatal(err)
		}
		if err := r.Start("t1"); err != nil {
			t.Fatal(err)
		}
		var out []protocol.Advice
		var after uint64
		for {
			d := parkedAfter(t, r, 0, after)
			_, m, err := r.lookup("t1", 1)
			if err != nil {
				t.Fatal(err)
			}
			m.mu.RLock()
			turn := m.e.G.Turn
			m.mu.RUnlock()
			// The passing opponent discards to hand size at the end of its
			// first full turn, which shows a card.
			if turn > 2 {
				m.mu.RLock()
				defer m.mu.RUnlock()
				var hand []string
				for _, id := range m.e.G.Zone(state.ZHand, 1) {
					hand = append(hand, m.e.G.Obj(id).Card.Faces[0].Name)
				}
				return out, hand
			}
			a, err := r.Advice(context.Background(), "t1", 1, 0, d.Seq)
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, a)
			after = d.Seq
			if err := r.SubmitIntent("t1", 1, 0, landFirstAnswer(d)); err != nil {
				t.Fatal(err)
			}
		}
	}
	a, handA := play("mono-red-goblins")
	b, handB := play("mono-blue-tempo")
	if strings.Join(handA, ",") == strings.Join(handB, ",") {
		t.Fatalf("fixture: the two opponents hold the same hand %v", handA)
	}
	if len(a) != len(b) {
		t.Fatalf("%d decisions against %d", len(a), len(b))
	}
	searched := 0
	for i := range a {
		x, _ := json.Marshal(a[i])
		y, _ := json.Marshal(b[i])
		if !bytes.Equal(x, y) {
			t.Fatalf("decision %d advice differs:\n%s\n%s", i, x, y)
		}
		var n explain.Note
		if a[i].State == protocol.AdviceReady {
			if err := json.Unmarshal(a[i].Explain, &n); err != nil {
				t.Fatal(err)
			}
			if n.Search != nil {
				searched++
			}
		}
	}
	if searched == 0 {
		t.Fatalf("fixture: none of the %d decisions was searched: %+v", len(a), a)
	}
	t.Logf("%d decisions, %d searched", len(a), searched)
}

// One game played with and without advice has the same hash chain.
func TestPairedBotLeavesTheHashChainAlone(t *testing.T) {
	const decisions = 40
	play := func(ask bool) (string, int) {
		o := testOptions(t)
		o.LoadDeck = pairedDecks(t, "mono-red-goblins")
		o.PairedBot = pairedTestEntry(t)
		r, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		if err := r.AddTable(pairedTable([]int{0})); err != nil {
			t.Fatal(err)
		}
		if err := r.Start("t1"); err != nil {
			t.Fatal(err)
		}
		var after uint64
		var d *decision.Decision
		for i := 0; i < decisions; i++ {
			d = parkedAfter(t, r, 0, after)
			if ask {
				if _, err := r.Advice(context.Background(), "t1", 1, 0, d.Seq); err != nil {
					t.Fatal(err)
				}
			}
			after = d.Seq
			if err := r.SubmitIntent("t1", 1, 0, landFirstAnswer(d)); err != nil {
				t.Fatal(err)
			}
		}
		parkedAfter(t, r, 0, after)
		_, m, err := r.lookup("t1", 1)
		if err != nil {
			t.Fatal(err)
		}
		m.mu.Lock()
		defer m.mu.Unlock()
		return m.g.Log().Head(), m.intents
	}
	headOn, nOn := play(true)
	headOff, nOff := play(false)
	if headOn != headOff || nOn != nOff {
		t.Fatalf("advice moved the game: on %s (%d intents), off %s (%d intents)", headOn, nOn, headOff, nOff)
	}
}

// The real search beside the game moving on: run under -race, a request never
// reads what the match goroutine writes, and it answers or says the decision
// moved. A table without payment plans searches its manual options.
func TestPairedBotSearchesBesideTheGame(t *testing.T) {
	for _, autoMana := range []bool{true, false} {
		o := testOptions(t)
		o.LoadDeck = pairedDecks(t, "mono-red-goblins")
		o.PairedBot = pairedTestEntry(t)
		r, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		defer r.Close()
		c := pairedTable([]int{0})
		c.AutoMana = autoMana
		if err := r.AddTable(c); err != nil {
			t.Fatal(err)
		}
		if err := r.Start("t1"); err != nil {
			t.Fatal(err)
		}
		var after uint64
		for i := 0; i < 30; i++ {
			d := parkedAfter(t, r, 0, after)
			after = d.Seq
			got := make(chan error, 1)
			go func() {
				_, err := r.Advice(context.Background(), "t1", 1, 0, d.Seq)
				got <- err
			}()
			if err := r.SubmitIntent("t1", 1, 0, landFirstAnswer(d)); err != nil {
				t.Fatal(err)
			}
			if err := <-got; err != nil && !errors.Is(err, errAdviceStale) && !strings.Contains(err.Error(), "no decision pending") {
				t.Fatalf("auto mana %v, decision %d: %v", autoMana, d.Seq, err)
			}
		}
	}
}

// stubAdvisor is a closed-list seat whose search blocks until its context ends
// or release closes, and counts how many run at once.
type stubAdvisor struct {
	running, peak *atomic.Int64
	started       chan<- uint64
	release       <-chan struct{}
	belief        *searchprobe.Belief
	note          func(explain.Note)
}

func (s *stubAdvisor) Decide(context.Context, view.View, decision.Decision) (decision.Intent, error) {
	return decision.Intent{}, errors.New("stubAdvisor: not a playing seat")
}
func (s *stubAdvisor) WantsEnv(*decision.Decision) bool  { return true }
func (s *stubAdvisor) SetExplain(fn func(explain.Note))  { s.note = fn }
func (s *stubAdvisor) SearchBelief() *searchprobe.Belief { return s.belief }
func (s *stubAdvisor) DecideBoard(context.Context, botpolicy.Board, decision.Decision) (decision.Intent, error) {
	return decision.Intent{}, nil
}
func (s *stubAdvisor) DecideEnv(ctx context.Context, _ bots.Env, d decision.Decision) (decision.Intent, error) {
	n := s.running.Add(1)
	defer s.running.Add(-1)
	for {
		p := s.peak.Load()
		if n <= p || s.peak.CompareAndSwap(p, n) {
			break
		}
	}
	s.started <- d.Seq
	select {
	case <-ctx.Done():
		return decision.Intent{}, ctx.Err()
	case <-s.release:
	}
	s.note(explain.Note{Seat: int(d.Player), Source: explain.Policy})
	return decision.Intent{}, nil
}

// stubRegistry builds a registry whose paired bot is a stubAdvisor at the given
// limit, with one table per id, each with a parked human.
type stubFixture struct {
	r             *Registry
	running, peak atomic.Int64
	started       chan uint64
	release       chan struct{}
}

func newStubFixture(t *testing.T, limit int, ids ...TableID) *stubFixture {
	t.Helper()
	f := &stubFixture{started: make(chan uint64, 16), release: make(chan struct{})}
	belief := enableClosedTest(t)
	o := testOptions(t)
	o.LoadDeck = pairedDecks(t, "mono-red-goblins")
	o.PairedBotSearches = limit
	o.PairedBot = &bots.Entry{Info: bots.Info{Name: "stub"}, New: func(bots.Options) (seat.Seat, error) {
		return &stubAdvisor{running: &f.running, peak: &f.peak, started: f.started, release: f.release, belief: belief}, nil
	}}
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	for _, id := range ids {
		c := pairedTable([]int{0})
		c.ID = id
		if err := r.AddTable(c); err != nil {
			t.Fatal(err)
		}
		if err := r.Start(id); err != nil {
			t.Fatal(err)
		}
	}
	f.r = r
	return f
}

type adviceAnswer struct {
	a   protocol.Advice
	err error
}

func (f *stubFixture) ask(ctx context.Context, id TableID, seq uint64) <-chan adviceAnswer {
	ch := make(chan adviceAnswer, 1)
	go func() {
		a, err := f.r.Advice(ctx, id, 1, 0, seq)
		ch <- adviceAnswer{a, err}
	}()
	return ch
}

// A search whose decision moves on is cancelled, and its answer never comes
// back. The next decision's search runs after it, never beside it.
func TestPairedBotDiscardsAnAnswerOnceTheDecisionMoves(t *testing.T) {
	f := newStubFixture(t, 2, "t1")
	d := parkedAfter(t, f.r, 0, 0)
	got := f.ask(context.Background(), "t1", d.Seq)
	if seq := <-f.started; seq != d.Seq {
		t.Fatalf("searched seq %d, want %d", seq, d.Seq)
	}
	if err := f.r.SubmitIntent("t1", 1, 0, landFirstAnswer(d)); err != nil {
		t.Fatal(err)
	}
	ans := <-got
	if ans.err == nil {
		t.Fatalf("a stale answer came back: %+v", ans.a)
	}
	if _, err := f.r.Advice(context.Background(), "t1", 1, 0, d.Seq); err == nil {
		t.Fatal("advice on a decision that has moved on was answered")
	}
}

// A second request for the same decision joins the running search, and the
// answer serves later requests without another search.
func TestPairedBotJoinsAndKeepsTheSameDecision(t *testing.T) {
	f := newStubFixture(t, 2, "t1")
	d := parkedAfter(t, f.r, 0, 0)
	first := f.ask(context.Background(), "t1", d.Seq)
	<-f.started
	second := f.ask(context.Background(), "t1", d.Seq)
	close(f.release)
	for _, ch := range []<-chan adviceAnswer{first, second} {
		if ans := <-ch; ans.err != nil || ans.a.State != protocol.AdviceReady {
			t.Fatalf("answer %+v", ans)
		}
	}
	if _, err := f.r.Advice(context.Background(), "t1", 1, 0, d.Seq); err != nil {
		t.Fatal(err)
	}
	select {
	case seq := <-f.started:
		t.Fatalf("a repeated request searched seq %d again", seq)
	default:
	}
}

// The last request to hang up cancels the search.
func TestPairedBotCancelsWhenTheAskerLeaves(t *testing.T) {
	f := newStubFixture(t, 2, "t1")
	d := parkedAfter(t, f.r, 0, 0)
	ctx, cancel := context.WithCancel(context.Background())
	got := f.ask(ctx, "t1", d.Seq)
	<-f.started
	cancel()
	if ans := <-got; !errors.Is(ans.err, context.Canceled) {
		t.Fatalf("answer %+v", ans)
	}
	deadline := time.Now().Add(5 * time.Second)
	for f.running.Load() != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the search kept running after its asker left")
		}
		time.Sleep(time.Millisecond)
	}
}

// The process-wide limit holds across tables: the second table's search waits
// for the first's.
func TestPairedBotProcessLimit(t *testing.T) {
	f := newStubFixture(t, 1, "t1", "t2")
	var wg sync.WaitGroup
	for _, id := range []TableID{"t1", "t2"} {
		deadline := time.Now().Add(30 * time.Second)
		var d *decision.Decision
		for d == nil && time.Now().Before(deadline) {
			d, _ = f.r.Pending(id, 1, 0)
			time.Sleep(time.Millisecond)
		}
		if d == nil {
			t.Fatalf("table %s parked nothing", id)
		}
		ch := f.ask(context.Background(), id, d.Seq)
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ans := <-ch; ans.err != nil {
				t.Errorf("table %s: %v", id, ans.err)
			}
		}()
	}
	<-f.started
	// The other table's search reaches the gate and queues; it never starts.
	queued := func() int {
		g := f.r.adviceGate
		g.mu.Lock()
		defer g.mu.Unlock()
		return len(g.waiters)
	}
	for queued() != 1 {
		select {
		case seq := <-f.started:
			t.Fatalf("a second search (seq %d) started while the first held the only slot", seq)
		default:
		}
		time.Sleep(time.Millisecond)
	}
	close(f.release)
	<-f.started
	wg.Wait()
	if p := f.peak.Load(); p != 1 {
		t.Fatalf("%d searches ran at once, limit 1", p)
	}
}

func TestPairedBotRefusals(t *testing.T) {
	open := func(entry *bots.Entry, c TableConfig) *Registry {
		o := testOptions(t)
		o.LoadDeck = pairedDecks(t, "mono-red-goblins")
		o.PairedBot = entry
		r, err := New(o)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { r.Close() })
		if err := r.AddTable(c); err != nil {
			t.Fatal(err)
		}
		return r
	}
	entry := pairedTestEntry(t)
	if st, err := open(nil, pairedTable([]int{0})).PairedBotStatus("t1"); err != nil || st.Offered || st.Reason != "" {
		t.Fatalf("off: %+v, %v", st, err)
	}
	if st, err := open(entry, pairedTable([]int{0})).PairedBotStatus("t1"); err != nil || !st.Offered || st.Shared || st.Policy != entry.Name {
		t.Fatalf("one human: %+v, %v", st, err)
	}
	two := pairedTable([]int{0, 1})
	if st, err := open(entry, two).PairedBotStatus("t1"); err != nil || st.Offered || st.Reason == "" {
		t.Fatalf("two humans, no opt-in: %+v, %v", st, err)
	}
	two.AllowPairedBot = true
	if st, err := open(entry, two).PairedBotStatus("t1"); err != nil || !st.Offered || !st.Shared {
		t.Fatalf("two humans, opt-in: %+v, %v", st, err)
	}
	four := pairedTable([]int{0})
	four.Seats, four.Decks = 4, []string{"opponent", "human", "opponent", "opponent"}
	if st, err := open(entry, four).PairedBotStatus("t1"); err != nil || st.Offered {
		t.Fatalf("four seats: %+v, %v", st, err)
	}
	r := open(entry, pairedTable([]int{0}))
	if _, err := r.PairedBotStatus("nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown table: %v", err)
	}
}

// An entry that would read real lists, or could not explain, is refused at
// startup however it is declared.
func TestPairedBotRefusesAnOpenListSeat(t *testing.T) {
	o := testOptions(t)
	o.PairedBot = &bots.Entry{Info: bots.Info{Name: "open", ClosedLists: true}, New: func(o bots.Options) (seat.Seat, error) {
		return azredeal.New(o, azredeal.Hosted())
	}}
	if _, err := New(o); err == nil || !strings.Contains(err.Error(), "real decklists") {
		t.Fatalf("open-list seat: %v", err)
	}
	o.PairedBot = &bots.Entry{Info: bots.Info{Name: "plain"}, New: func(bots.Options) (seat.Seat, error) { return passSeat{}, nil }}
	if _, err := New(o); err == nil {
		t.Fatal("a seat that does not search was accepted")
	}
}
