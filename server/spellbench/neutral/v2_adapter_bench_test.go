package neutral_test

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/minihost"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// This file is the sbp-9 spike for docs/design/v2-default-interface.md: the
// heuristic bot deciding through the v2 candidate path (the observation and
// candidates alone, translated by neutral.Session) against the same bot on
// gorge's own view and decision (x_gorge_view_v1), over a recorded corpus.

// v2Policy is the bot both paths run: the heuristic with seat auto-pay, as
// TestDecisionsMatchGorgeOnGorgeGames runs it.
const v2Policy = "test-bot-auto-pay"

// v2Step is one recorded choose request, decoded once outside any timer.
type v2Step struct {
	line   []byte
	sd     protocol.SeatDecision
	sems   []map[string]any
	gorge  xview.Payload // gorge's own payload, the native path's input
	picked int           // the candidate the recording seat answered
}

// v2Stream is one seat's choose requests of one game, in order.
type v2Stream struct {
	seed  uint64
	steps []v2Step
}

// v2Recorder keeps a seat's agent seed and every choose request with the
// candidate the agent answered.
type v2Recorder struct {
	inner minihost.Link
	s     *v2Stream
}

func (r *v2Recorder) Round(req []byte) ([]byte, error) {
	resp, err := r.inner.Round(req)
	var q struct {
		RequestType string `json:"request_type"`
		AgentSeed   uint64 `json:"agent_seed"`
	}
	json.Unmarshal(req, &q)
	switch q.RequestType {
	case "game_start":
		r.s.seed = q.AgentSeed
	case "choose":
		var a struct {
			Selection struct {
				CandidateID int `json:"candidate_id"`
			} `json:"selection"`
		}
		json.Unmarshal(resp, &a)
		r.s.steps = append(r.s.steps, v2Step{line: bytes.Clone(req), picked: a.Selection.CandidateID})
	}
	return resp, err
}

// v2Corpus plays gorge bot mirrors on gorge's engine (engine auto-pay, as
// TestDecisionsMatchGorgeOnGorgeGames) and returns each seat's decoded stream.
func v2Corpus(tb testing.TB, reg *cards.Registry, games int) []*v2Stream {
	tb.Helper()
	kinds := map[string]bool{}
	for _, k := range env.DecisionKinds {
		kinds[k] = true
	}
	var out []*v2Stream
	for i := 0; i < games; i++ {
		d := catalog.Decks()[i%len(catalog.Decks())]
		srv := env.New(reg, nil)
		srv.EnableAutoPay()
		h := &minihost.Host{RunSecret: make([]byte, 32), Engine: &minihost.EngineLink{S: srv},
			Profile:  validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{"x_gorge_view_v1": true}},
			MaxSteps: 20000, MaxDecisions: 9999}
		var links [2]minihost.Link
		for s := range links {
			a, err := agent.New(v2Policy)
			if err != nil {
				tb.Fatal(err)
			}
			st := &v2Stream{}
			out = append(out, st)
			links[s] = &v2Recorder{inner: a, s: st}
		}
		if _, err := h.Play(uint64(i), d, "london", []string{"x_gorge_view_v1"}, links); err != nil {
			tb.Fatalf("game %d (%s): %v", i, d.CatalogID, err)
		}
	}
	for _, st := range out {
		for i := range st.steps {
			decodeV2Step(tb, &st.steps[i], true)
		}
	}
	return out
}

// decodeV2Step decodes a request line the way gorge's agent does: the typed
// seat decision for the neutral session, the semantics generically (numbers
// kept as json.Number) and, with gorge set, gorge's own payload.
func decodeV2Step(tb testing.TB, st *v2Step, gorge bool) {
	tb.Helper()
	var full struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(st.line, &full); err != nil {
		tb.Fatal(err)
	}
	st.sd = full.Decision
	var q struct {
		Decision struct {
			Candidates []struct {
				Semantic map[string]any `json:"semantic"`
			} `json:"candidates"`
			Extensions map[string]json.RawMessage `json:"extensions"`
		} `json:"decision"`
	}
	dec := json.NewDecoder(bytes.NewReader(st.line))
	dec.UseNumber()
	if err := dec.Decode(&q); err != nil {
		tb.Fatal(err)
	}
	for _, c := range q.Decision.Candidates {
		st.sems = append(st.sems, c.Semantic)
	}
	if !gorge {
		return
	}
	if err := json.Unmarshal(q.Decision.Extensions["x_gorge_view_v1"], &st.gorge); err != nil {
		tb.Fatal(err)
	}
}

// v2Seat answers candidate decisions with a native seat.Seat: the shape of
// the proposed bots/v2api NativeAdapter. With a session it reads only the
// v2 observation and candidates; without one it reads gorge's own payload
// (the native path).
type v2Seat struct {
	bot    seat.Seat
	sess   *neutral.Session
	plan   *agent.Plan
	native uint64
	asks   int // bot.Decide calls
}

func newV2Seat(tb testing.TB, reg *cards.Registry, seed uint64, adapter bool) *v2Seat {
	bot, err := bots.New(v2Policy, bots.Options{Seed: seed, AutoPayMana: true, Deps: bots.Deps{Cards: reg}})
	if err != nil {
		tb.Fatal(err)
	}
	s := &v2Seat{bot: bot}
	if adapter {
		s.sess = neutral.NewSession(reg)
		s.sess.SetAutoPay(true) // the corpus host declares engine_autopay
	}
	return s
}

// Decide returns the index of the candidate the bot picks.
func (s *v2Seat) Decide(st *v2Step) int {
	p := st.gorge
	if s.sess != nil {
		var err error
		if p, err = s.sess.Payload(&st.sd); err != nil || len(p.Ops) != len(st.sd.Candidates) {
			return 0 // untranslatable: the agent's fallback
		}
	}
	v, d := agent.Rebuild(p)
	ask := func(nd decision.Decision) decision.Intent {
		vv := v
		vv.Decision = &nd
		s.asks++
		in, err := s.bot.Decide(context.Background(), vv, nd)
		if err != nil {
			panic(err)
		}
		return in
	}
	if s.plan == nil || s.native != p.NativeIndex {
		s.plan, s.native = agent.NewPlan(p.NativeIndex, ask(d)), p.NativeIndex
	}
	i, _ := agent.Pick(p, st.sems, s.plan, ask)
	if s.sess != nil {
		s.sess.Picked(i)
	}
	return i
}

// comparable reports whether candidate i of a step can be compared across
// the two paths: gorge's seats float mana, which the neutral world never
// does under engine_autopay (see twin.Round).
func comparable(st *v2Step, i int) bool {
	return st.sd.Candidates[i].Semantic.Kind != "activate_mana_ability"
}

// sameCopy reports whether two candidates play the same card: the engine
// hides the order of interchangeable copies in a hand, so which of them a seat
// plays first is not a decision (as twin.Seat counts it).
func sameCopy(x, y map[string]any) bool {
	if x["kind"] != y["kind"] || x["kind"] != "cast_spell" && x["kind"] != "play_land" {
		return false
	}
	anon := func(m map[string]any) string {
		c := maps.Clone(m)
		if src, ok := c["source"].(map[string]any); ok {
			src = maps.Clone(src)
			delete(src, "object_id")
			c["source"] = src
		}
		out, _ := json.Marshal(c)
		return string(out)
	}
	return anon(x) == anon(y)
}

func TestV2AdapterAgreesWithNative(t *testing.T) {
	reg := testcorpus.Registry(t)
	streams := v2Corpus(t, reg, 1)
	compared, multi, replayed := 0, 0, 0
	for si, str := range streams {
		nat, ad := newV2Seat(t, reg, str.seed, false), newV2Seat(t, reg, str.seed, true)
		for k := range str.steps {
			st := &str.steps[k]
			n, a := nat.Decide(st), ad.Decide(st)
			// Precondition: the native path is the recording seat, so the
			// corpus is what native play does.
			if n != st.picked {
				t.Fatalf("seat %d step %d: the native path picks %d, the recording seat %d", si, k, n, st.picked)
			}
			replayed++
			if !comparable(st, n) {
				continue
			}
			compared++
			if len(st.sd.Candidates) > 1 {
				multi++
			}
			if a != n && !sameCopy(st.sems[a], st.sems[n]) {
				t.Errorf("seat %d step %d (%s): adapter picks %s, native %s", si, k, st.sd.Context.Kind,
					st.sd.Candidates[a].Semantic.Kind, st.sd.Candidates[n].Semantic.Kind)
			}
		}
	}
	t.Logf("replayed %d recorded decisions; compared %d (%d with more than one candidate)", replayed, compared, multi)
	if multi < 20 {
		t.Fatalf("only %d comparable decisions with a real choice; the agreement needs at least 20", multi)
	}
}

// BenchmarkV2Adapter replays the recorded corpus through each path; one op
// is one pass over the corpus with a fresh bot (and session), and
// ns/decision divides by the recorded v2 decisions.
//
//   - native: the bot on gorge's own view and decision, pre-decoded.
//   - adapter: the bot on the view and decision neutral.Session builds from
//     the typed observation and candidates (no JSON).
//   - adapter+json: adapter, also decoding each request line as the wire
//     agent does.
func BenchmarkV2Adapter(b *testing.B) {
	reg := testcorpus.Registry(b)
	streams := v2Corpus(b, reg, 2)
	n := 0
	for _, s := range streams {
		n += len(s.steps)
	}
	run := func(b *testing.B, adapter, decode bool) {
		b.ReportAllocs()
		asks := 0
		for b.Loop() {
			for _, str := range streams {
				s := newV2Seat(b, reg, str.seed, adapter)
				for k := range str.steps {
					st := &str.steps[k]
					if decode {
						c := v2Step{line: st.line}
						decodeV2Step(b, &c, false)
						st = &c
					}
					s.Decide(st)
				}
				asks += s.asks
			}
		}
		b.ReportMetric(float64(b.Elapsed().Nanoseconds())/float64(b.N*n), "ns/decision")
		b.ReportMetric(float64(asks)/float64(b.N*n), "decides/decision")
	}
	b.Logf("corpus: %d games, %d recorded decisions", len(streams)/2, n)
	b.Run("native", func(b *testing.B) { run(b, false, false) })
	b.Run("adapter", func(b *testing.B) { run(b, true, false) })
	b.Run("adapter+json", func(b *testing.B) { run(b, true, true) })
}
