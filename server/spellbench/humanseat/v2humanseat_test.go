package humanseat_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/humanseat"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// scripted is the far end of a V2HumanSeat in these tests: at each substep it
// picks the candidate that carries out what the production bot would answer
// natively, the way the native agent mode does (agent.Plan, agent.Pick). The
// reference bot is the one defaultSeats builds for the slot, so a game it
// plays through the codec must be the game a bot plays natively.
type scripted struct {
	bot        seat.Seat
	plan       *agent.Plan
	planSeq    uint64
	planIntent decision.Intent

	// stallAt, when non-zero, makes the answerer hold the SECOND substep of
	// the first transaction whose native decision is at least stallAt until
	// the seat gives the decision up (the think timeout fires).
	stallAt   uint64
	stalledAt atomic.Uint64
	stalls    atomic.Int32
	// substeps counts asks by substep index.
	mu       sync.Mutex
	substeps map[uint32]int
	nativeOn int
	misses   []string
	// answerNative, when set, answers a native-only decision over the native
	// routes, as a client would.
	answerNative func(n humanseat.NativeOnly, in *decision.Intent)
}

func (a *scripted) Choose(ctx context.Context, ask humanseat.Ask) (int, error) {
	nd := *ask.Pose.Native
	a.mu.Lock()
	if a.substeps == nil {
		a.substeps = map[uint32]int{}
	}
	a.substeps[ask.Pose.SubstepIndex]++
	a.mu.Unlock()
	if a.stallAt != 0 && a.stalledAt.Load() == 0 && nd.Seq >= a.stallAt && ask.Pose.SubstepIndex >= 1 {
		a.stalledAt.Store(nd.Seq)
		a.stalls.Add(1)
		<-ctx.Done()
		return 0, ctx.Err()
	}
	askBot := func(d decision.Decision) decision.Intent {
		v := ask.View
		v.Decision = &d
		in, err := a.bot.Decide(ctx, v, d)
		if err != nil {
			panic(err)
		}
		return in
	}
	if a.plan == nil || a.planSeq != nd.Seq {
		a.planIntent = askBot(nd)
		a.plan, a.planSeq = agent.NewPlan(nd.Seq, a.planIntent), nd.Seq
	}
	ops := make([]mapping.NativeOp, len(ask.Pose.Candidates))
	sems := make([]map[string]any, len(ask.Pose.Candidates))
	for i, c := range ask.Pose.Candidates {
		ops[i] = c.Op
		m := map[string]any{"kind": c.Sem.Kind}
		for k, v := range c.Sem.Fields {
			m[k] = v
		}
		sems[i] = m
	}
	follow := map[string]decision.Decision{}
	for k, f := range ask.Pose.Followups {
		follow[k] = *f
	}
	i, miss := agent.Pick(xview.Payload{Decision: nd, Ops: ops, Followups: follow}, sems, a.plan, askBot)
	if miss != "" {
		a.mu.Lock()
		a.misses = append(a.misses, fmt.Sprintf("seq %d kind %s %s sub %d/%d ncand %d", nd.Seq, nd.Kind, miss, ask.Pose.SubstepIndex, ask.Pose.SubstepCount, len(sems)))
		f := a.answerNative
		a.mu.Unlock()
		// The bot's answer is not among the candidates (an engine default or an
		// unmapped option): give v2 up for this decision and answer it over the
		// native routes, as the client does for a native_only decision.
		if f != nil {
			in := a.planIntent
			go f(humanseat.NativeOnly{Seat: nd.Player, Seq: nd.Seq, Decision: nd, View: ask.View}, &in)
		}
		return 0, errors.New("bot answer is not a candidate")
	}
	return i, nil
}

func (a *scripted) NativeOnly(n humanseat.NativeOnly) {
	a.mu.Lock()
	a.nativeOn++
	f := a.answerNative
	a.mu.Unlock()
	if f != nil {
		go f(n, nil)
	}
}

func repoOptions(t *testing.T) host.Options {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	return host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			cs, err := testutil.LoadRepoDeck(reg, name)
			if err != nil {
				return host.Deck{}, err
			}
			return host.Deck{Name: name, Cards: cs}, nil
		},
		Tokens:       reg.Tokens,
		NameUniverse: reg.Universe(),
		Sleep:        func(time.Duration, <-chan struct{}) {},
	}
}

func tableCfg(id host.TableID, tableSeed uint64) host.TableConfig {
	names := testutil.LegacyDeckNames()
	return host.TableConfig{ID: id, Name: string(id), Seats: 2,
		Decks: []string{names[int(tableSeed)%len(names)], names[(int(tableSeed)+1)%len(names)]},
		Seed:  tableSeed, Mulligans: 0, Spectator: view.Omniscient, AutoMana: true, BotAutoPayMana: true}
}

func play(t *testing.T, o host.Options, cfg host.TableConfig) (head string, events int) {
	t.Helper()
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(cfg.ID); err != nil {
		t.Fatal(err)
	}
	return waitHead(t, r, cfg.ID, 3*time.Minute, nil)
}

// waitHead polls until match 1 finishes; between polls it runs tick (the
// native human driver).
func waitHead(t *testing.T, r *host.Registry, id host.TableID, limit time.Duration, tick func()) (string, int) {
	t.Helper()
	deadline := time.Now().Add(limit)
	for {
		ms, err := r.Matches(id)
		if err == nil {
			for _, m := range ms {
				if m.Match != 1 {
					continue
				}
				switch m.State {
				case protocol.MatchFinished:
					return m.Head, m.Events
				case protocol.MatchCrashed:
					t.Fatalf("table %s crashed: %s", id, m.Head)
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("table %s did not finish within %v", id, limit)
		}
		if tick != nil {
			tick()
		}
		time.Sleep(time.Millisecond)
	}
}

// recorder keeps every intent the human seat's decisions were answered with,
// by decision sequence number: the tapped answers and the native ones.
type recorder struct {
	mu sync.Mutex
	in map[uint64]decision.Intent
}

func (r *recorder) add(in decision.Intent) {
	r.mu.Lock()
	if r.in == nil {
		r.in = map[uint64]decision.Intent{}
	}
	r.in[in.Seq] = in
	r.mu.Unlock()
}

func (r *recorder) get(seq uint64) (decision.Intent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	in, ok := r.in[seq]
	return in, ok
}

// recTap records what the wrapped tap's answers commit.
type recTap struct {
	*humanseat.Seat
	rec *recorder
}

func (r recTap) Parked(ctx context.Context, v view.View, d decision.Decision, answer func(decision.Intent) error, gone <-chan struct{}) {
	r.Seat.Parked(ctx, v, d, func(in decision.Intent) error {
		err := answer(in)
		if err == nil {
			r.rec.add(in)
		}
		return err
	}, gone)
}

// playV2 plays the table with seat 0 a tapped human seat driven by ans, and
// returns the chain head and the intents the seat was answered with.
func playV2(t *testing.T, tableSeed uint64, think time.Duration, ans *scripted) (string, *recorder) {
	t.Helper()
	rec := &recorder{}
	o := repoOptions(t)
	o.ThinkTimeout = think
	o.Seats = func(names []string, seed uint64) []seat.Seat {
		tap, err := humanseat.New(humanseat.Config{Seats: len(names), Format: "constructed", Answerer: ans})
		if err != nil {
			t.Errorf("humanseat.New: %v", err)
			return nil
		}
		h := host.NewHumanSeat()
		h.SetTap(recTap{tap, rec})
		out := hostBots(t, len(names), seed)
		out[0] = h
		return out
	}
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := tableCfg("v2", tableSeed)
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	ans.mu.Lock()
	ans.answerNative = func(n humanseat.NativeOnly, in *decision.Intent) {
		var nin decision.Intent
		if in != nil {
			nin = *in
		} else {
			var err error
			if nin, err = ans.bot.Decide(context.Background(), n.View, n.Decision); err != nil {
				t.Errorf("native answer for seq %d: %v", n.Seq, err)
				return
			}
		}
		if err := r.SubmitIntent(cfg.ID, 1, n.Seat, nin); err != nil {
			t.Errorf("native answer for seq %d (%s): %v", n.Seq, n.Route, err)
			return
		}
		rec.add(nin)
	}
	ans.mu.Unlock()
	if err := r.Start(cfg.ID); err != nil {
		t.Fatal(err)
	}
	head, _ := waitHead(t, r, cfg.ID, 3*time.Minute, nil)
	return head, rec
}

// playHuman plays the same table with a plain, untapped human seat 0 that is
// answered with rec's intents, leaving unanswered any decision rec lacks (the
// think timeout then hands it to the caretaker).
func playHuman(t *testing.T, tableSeed uint64, think time.Duration, rec *recorder) (string, int) {
	t.Helper()
	o := repoOptions(t)
	o.ThinkTimeout = think
	o.Seats = func(names []string, seed uint64) []seat.Seat {
		out := hostBots(t, len(names), seed)
		out[0] = host.NewHumanSeat()
		return out
	}
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := tableCfg("human", tableSeed)
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start(cfg.ID); err != nil {
		t.Fatal(err)
	}
	skipped := map[uint64]bool{}
	head, _ := waitHead(t, r, cfg.ID, 3*time.Minute, func() {
		d, err := r.Pending(cfg.ID, 1, 0)
		if err != nil || skipped[d.Seq] {
			return
		}
		in, ok := rec.get(d.Seq)
		if !ok {
			skipped[d.Seq] = true
			return
		}
		if err := r.SubmitIntent(cfg.ID, 1, 0, in); err != nil {
			t.Errorf("replay seq %d: %v", d.Seq, err)
		}
	})
	return head, len(skipped)
}

// hostBots builds the bots defaultSeats builds for a table, one per slot.
func hostBots(t *testing.T, n int, seed uint64) []seat.Seat {
	t.Helper()
	out := make([]seat.Seat, n)
	for i := range out {
		b, err := host.NewBotPolicySeatWithAutoPayMana(host.BotPolicy, seed^uint64(i+1), true)
		if err != nil {
			t.Fatalf("bot: %v", err)
		}
		out[i] = b
	}
	return out
}

func referenceBot(t *testing.T, tableSeed uint64) seat.Seat {
	t.Helper()
	b, err := host.NewBotPolicySeatWithAutoPayMana(host.BotPolicy, host.MatchSeed(tableSeed, 0)^1, true)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// TestV2HumanSeatHeadsMatchNative: a game whose seat 0 is a V2HumanSeat driven
// by a scripted answerer (at each substep the candidate that carries out what
// the production bot answers natively) ends on the chain head of the same
// table with a plain human seat answered with the very intents the v2 seat
// committed. The tap changes nothing a replay can see.
//
// The same game is also compared with the table played by the production
// bots. That one is not required to match: the v2 mapping answers some
// decisions itself or words them differently (engine-ordered damage division,
// a life-paying plan as a plain cast), which the log records. It must match
// for most seeds, and it is reported per seed.
func TestV2HumanSeatHeadsMatchNative(t *testing.T) {
	const seeds = 20
	var botIdentical atomic.Int32
	t.Run("seeds", func(t *testing.T) {
		for g := uint64(0); g < seeds; g++ {
			tableSeed := 3000 + g
			t.Run(fmt.Sprint(tableSeed), func(t *testing.T) {
				t.Parallel()
				ans := &scripted{bot: referenceBot(t, tableSeed)}
				v2, rec := playV2(t, tableSeed, 0, ans)
				ans.mu.Lock()
				asked := 0
				for _, n := range ans.substeps {
					asked += n
				}
				ans.mu.Unlock()
				if asked == 0 {
					t.Fatal("the answerer was never asked: the tap did not run")
				}
				human, _ := playHuman(t, tableSeed, 0, rec)
				if v2 != human {
					t.Fatalf("seed %d: heads differ: v2 seat %s, plain human seat %s", tableSeed, v2, human)
				}
				bots, _ := play(t, repoOptions(t), tableCfg("native", tableSeed))
				same := bots == v2
				if same {
					botIdentical.Add(1)
				}
				t.Logf("seed %d: %d asks (by substep %v), %d native-only, %d not a candidate; same head as the production bots: %v",
					tableSeed, asked, ans.substeps, ans.nativeOn, len(ans.misses), same)
			})
		}
	})
	if got := botIdentical.Load(); got < seeds/2 {
		t.Errorf("only %d of %d seeds match the production bots' head", got, seeds)
	} else {
		t.Logf("%d of %d seeds match the production bots' head", got, seeds)
	}
}

// TestV2HumanSeatTimeoutFallsToCaretaker: a think timeout in the middle of a
// transaction (the answerer holds a later substep) drops the partial picks and
// the caretaker answers the whole native decision, so the game ends on the head
// of a plain human seat that timed out at that decision.
func TestV2HumanSeatTimeoutFallsToCaretaker(t *testing.T) {
	const tableSeed, think = 3000, 2 * time.Second
	ans := &scripted{bot: referenceBot(t, tableSeed), stallAt: 1}
	v2, rec := playV2(t, tableSeed, think, ans)
	stalled := ans.stalledAt.Load()
	if stalled == 0 || ans.stalls.Load() != 1 {
		t.Fatalf("no transaction was held past its first substep (stalls %d)", ans.stalls.Load())
	}
	if _, ok := rec.get(stalled); ok {
		t.Fatalf("decision %d was answered although its transaction stalled", stalled)
	}
	human, skipped := playHuman(t, tableSeed, think, rec)
	if skipped != 1 {
		t.Fatalf("the plain human seat left %d decisions to the caretaker, want 1", skipped)
	}
	if v2 != human {
		t.Fatalf("heads differ after a timeout at decision %d: v2 seat %s, plain human seat %s", stalled, v2, human)
	}
}

// TestV2HumanSeatRefusesMultiplayer: v2 covers two seats only.
func TestV2HumanSeatRefusesMultiplayer(t *testing.T) {
	ans := &scripted{}
	if _, err := humanseat.New(humanseat.Config{Seats: 4, Format: "constructed", Answerer: ans}); err == nil {
		t.Fatal("4 seats: want an error")
	}
	if _, err := humanseat.New(humanseat.Config{Seats: 2, Format: "commander", Answerer: ans}); err == nil {
		t.Fatal("commander: want an error")
	}
	if _, err := humanseat.New(humanseat.Config{Seats: 2, Format: "constructed"}); err == nil {
		t.Fatal("nil answerer: want an error")
	}
	s, err := humanseat.New(humanseat.Config{Seats: 2, Format: "constructed", Answerer: ans})
	if err != nil {
		t.Fatalf("two seats: %v", err)
	}
	// A table that turns out wider than the seat was built for refuses the
	// match at attach, whatever the constructor was told.
	if err := s.AttachMatch(&bots.MatchContext{Seats: 4, Format: "constructed"}); err == nil {
		t.Fatal("attach to 4 seats: want an error")
	}
}
