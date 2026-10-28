package humanseat_test

import (
	"context"
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
	bot     seat.Seat
	plan    *agent.Plan
	planSeq uint64

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
	// answerNative, when set, answers a native-only decision over the native
	// routes, as a client would.
	answerNative func(humanseat.NativeOnly)
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
		a.plan, a.planSeq = agent.NewPlan(nd.Seq, askBot(nd)), nd.Seq
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
	i, _ := agent.Pick(xview.Payload{Decision: nd, Ops: ops, Followups: follow}, sems, a.plan, askBot)
	return i, nil
}

func (a *scripted) NativeOnly(n humanseat.NativeOnly) {
	a.mu.Lock()
	a.nativeOn++
	f := a.answerNative
	a.mu.Unlock()
	if f != nil {
		go f(n)
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

func play(t *testing.T, o host.Options, cfg host.TableConfig, ans *scripted) (head string, events int) {
	t.Helper()
	r, err := host.New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if ans != nil {
		ans.mu.Lock()
		ans.answerNative = func(n humanseat.NativeOnly) {
			in, err := ans.bot.Decide(context.Background(), n.View, n.Decision)
			if err == nil {
				err = r.SubmitIntent(cfg.ID, 1, n.Seat, in)
			}
			if err != nil {
				t.Errorf("native answer for seq %d (%s): %v", n.Seq, n.Route, err)
			}
		}
		ans.mu.Unlock()
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

// v2Seats puts a tapped human seat at slot 0 and the table's bot at slot 1.
func v2Seats(t *testing.T, ans humanseat.Answerer, hs **host.HumanSeat) func([]string, uint64) []seat.Seat {
	return func(names []string, seed uint64) []seat.Seat {
		tap, err := humanseat.New(humanseat.Config{Seats: len(names), Format: "constructed", Answerer: ans})
		if err != nil {
			t.Errorf("humanseat.New: %v", err)
			return nil
		}
		h := host.NewHumanSeat()
		h.SetTap(tap)
		*hs = h
		out := make([]seat.Seat, len(names))
		out[0] = h
		for i := 1; i < len(names); i++ {
			b, err := host.NewBotPolicySeatWithAutoPayMana(host.BotPolicy, seed^uint64(i+1), true)
			if err != nil {
				t.Errorf("bot: %v", err)
				return nil
			}
			out[i] = b
		}
		return out
	}
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
// by a scripted answerer ends on the chain head of the same table played by
// the production bots.
func TestV2HumanSeatHeadsMatchNative(t *testing.T) {
	for g := uint64(0); g < 20; g++ {
		tableSeed := 3000 + g
		t.Run(fmt.Sprint(tableSeed), func(t *testing.T) {
			t.Parallel()
			native, nEvents := play(t, repoOptions(t), tableCfg("native", tableSeed), nil)

			ans := &scripted{bot: referenceBot(t, tableSeed)}
			o := repoOptions(t)
			var hs *host.HumanSeat
			o.Seats = v2Seats(t, ans, &hs)
			v2, vEvents := play(t, o, tableCfg("v2", tableSeed), ans)

			if hs == nil {
				t.Fatal("the V2 table never built its tapped seat")
			}
			ans.mu.Lock()
			asked, nativeOnly := 0, ans.nativeOn
			for _, n := range ans.substeps {
				asked += n
			}
			ans.mu.Unlock()
			if asked == 0 {
				t.Fatal("the answerer was never asked: the tap did not run")
			}
			if v2 != native {
				t.Fatalf("seed %d: heads differ: native %s (%d events), v2 %s (%d events); %d asks, %d native-only, misses %q",
					tableSeed, native, nEvents, v2, vEvents, asked, nativeOnly, ans.misses)
			}
			t.Logf("seed %d: %d events, %d substeps asked %v, %d native-only", tableSeed, nEvents, asked, ans.substeps, nativeOnly)
		})
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
