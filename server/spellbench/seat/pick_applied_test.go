package seat

import (
	"context"
	"encoding/json"
	"fmt"
	_ "github.com/adams-shaun/gorge/bots/all"
	"io"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	engineseat "github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
	gbench "github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// tappedAgent serves a neutral gorge agent on the seat's pipe pair and records
// the kind of the candidate it picked at each choose request.
type tappedAgent struct {
	reg    *cards.Registry
	picked []string
}

func (ta *tappedAgent) serve(r io.Reader, w io.Writer) error {
	s, err := agent.New("bot")
	if err != nil {
		return err
	}
	s.SetRegistry(ta.reg)
	if err := s.EnableNeutral(); err != nil {
		return err
	}
	in := wire.NewReader(r)
	for {
		line, err := in.ReadLine()
		if err != nil {
			return nil
		}
		resp := s.Handle(line)
		var q struct {
			RequestType string `json:"request_type"`
			Decision    struct {
				Candidates []struct {
					Semantic struct {
						Kind string `json:"kind"`
					} `json:"semantic"`
				} `json:"candidates"`
			} `json:"decision"`
		}
		var a struct {
			Selection *struct {
				CandidateID int `json:"candidate_id"`
			} `json:"selection"`
		}
		if json.Unmarshal(line, &q) == nil && q.RequestType == "choose" && json.Unmarshal(resp, &a) == nil && a.Selection != nil {
			ta.picked = append(ta.picked, q.Decision.Candidates[a.Selection.CandidateID].Semantic.Kind)
		}
		if _, err := w.Write(append(resp, '\n')); err != nil {
			return err
		}
	}
}

// pickRecorder is the seat under test plus a tally of what it applied for the
// agent's first pick in each priority window.
type pickRecorder struct {
	*Agent
	ta       *tappedAgent
	picks    map[string]int
	mismatch []string
	offered  int // priority windows the driver built payment plans for
}

// nativeKind is the native option kind a v2 priority candidate kind stands for.
var nativeKind = map[string]string{"pass": "pass", "play_land": "play_land", "cast_spell": "cast"}

func (p *pickRecorder) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	before := len(p.ta.picked)
	if d.Kind == decision.KPriority && len(d.PaymentActions) > 0 {
		p.offered++
	}
	in, err := p.Agent.Decide(ctx, v, d)
	if err != nil || d.Kind != decision.KPriority || len(p.ta.picked) == before {
		return in, err
	}
	pick := p.ta.picked[before]
	p.picks[pick]++
	want, ok := nativeKind[pick]
	if !ok {
		return in, err
	}
	applied := ""
	if len(in.Choices) == 1 && in.Choices[0] < len(d.Options) {
		applied = d.Options[in.Choices[0]].Kind
	}
	if in.Payment != nil {
		applied = "cast"
	}
	if applied != want {
		p.mismatch = append(p.mismatch, fmt.Sprintf("seq %d: agent picked %s, seat applied %q", d.Seq, pick, applied))
	}
	return in, err
}

// TestSpellbenchSeatAppliesAgentPick plays a Burn mirror through
// testkit/bench.PlayGame, the driver cmd/cardfuzz and mtgsim use, with the
// neutral gorge agent in seat 0, and checks that the native intent the seat
// applies is the one the agent's pick stands for. The seat declares
// engine_autopay, so the mapping offers a cast only through
// Decision.PaymentActions, which a driver builds only for a
// seat.PaymentPlanConsumer. Without that declaration the agent is never offered
// a cast_spell candidate: it passes every window the native bot would play in.
func TestSpellbenchSeatAppliesAgentPick(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	dk, ok := catalog.ByID("Burn")
	if !ok {
		t.Fatal("catalog deck Burn is missing")
	}
	burn, err := catalog.Resolve(reg, dk)
	if err != nil {
		t.Fatal(err)
	}
	for _, seed := range []uint64{7, 8} {
		ta := &tappedAgent{reg: reg}
		a, err := NewInProcess("sb-bot", ta.serve, bots.Options{Seed: seed, AutoPayMana: true})
		if err != nil {
			t.Fatal(err)
		}
		rec := &pickRecorder{Agent: a, ta: ta, picks: map[string]int{}}
		native, err := bots.New("bot", bots.Options{Seed: seed + 1, AutoPayMana: true})
		if err != nil {
			t.Fatal(err)
		}
		cfg := rules.Config{Names: []string{"Burn", "Burn"}, Decks: [][]*cards.Card{burn, burn}, Tokens: reg.Tokens, Seed: seed, NameUniverse: reg.Universe()}
		if _, _, err := gbench.PlayGame(cfg, []engineseat.Seat{rec, native}, 60, 20000, gbench.Hooks{}); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if f := a.Forfeit(); f != "" {
			t.Fatalf("seed %d: the seat forfeited: %s", seed, f)
		}
		if rec.offered == 0 {
			t.Fatalf("seed %d: the driver built no payment plan for the seat in %d picks; a declared engine_autopay seat can cast nothing", seed, len(ta.picked))
		}
		if rec.picks["cast_spell"] == 0 || rec.picks["pass"] == 0 {
			t.Fatalf("seed %d: the agent's priority picks were %v; it must both cast and pass", seed, rec.picks)
		}
		if len(rec.mismatch) != 0 {
			t.Fatalf("seed %d: %d picks not applied: %v", seed, len(rec.mismatch), rec.mismatch)
		}
	}
}

// TestSpellbenchSeatAnswersUnposableDecisionWithFallback: a decision the v2
// mapping has no route for is the adapter's limit, not the agent's error. The
// seat answers it with its fallback and keeps playing instead of forfeiting
// the match as agent_error.
func TestSpellbenchSeatAnswersUnposableDecisionWithFallback(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _, _ := testEngineSeed(t, 7)
	a, err := NewInProcess("sb-bot", func(r io.Reader, w io.Writer) error {
		s, err := agent.New("bot")
		if err != nil {
			return err
		}
		s.SetRegistry(reg)
		if err := s.EnableNeutral(); err != nil {
			return err
		}
		return agent.Serve(r, w, s)
	}, bots.Options{Seed: 1, AutoPayMana: true})
	if err != nil {
		t.Fatal(err)
	}
	mc := &bots.MatchContext{Context: context.Background(), Engine: e, GameID: "g-0000000000000001", Seat: 0, Seats: 2, Format: "constructed",
		Seed: []byte("test"), OwnDeck: e.OwnDeck(state.PlayerID(0)), Mulligan: "none", StartingSeat: int(e.G.Active), Logf: t.Logf, Registry: reg}
	if err := a.AttachMatch(mc); err != nil {
		t.Fatal(err)
	}
	defer endTestMatch([]engineseat.Seat{a})
	d := decision.Decision{Seq: e.Pending().Seq, Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "hidden_pick"}, {Index: 1, Kind: "hidden_pick"}}}
	in, err := a.Decide(context.Background(), view.View{}, d)
	if err != nil {
		t.Fatal(err)
	}
	if a.Forfeit() != "" {
		t.Fatalf("an unposable decision forfeited the seat: %s", a.Forfeit())
	}
	if a.Unposed() != 1 {
		t.Fatalf("Unposed = %d, want 1", a.Unposed())
	}
	if in.Seq != d.Seq || in.Player != d.Player {
		t.Fatalf("fallback answered %d/%d, want %d/%d", in.Seq, in.Player, d.Seq, d.Player)
	}
}
