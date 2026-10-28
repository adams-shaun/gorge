package neutral_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
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

func init() {
	bots.Register(bots.Entry{Info: bots.Info{Name: "test-bot-auto-pay", Label: "test bot auto-pay", Tier: bots.Experimental},
		New: func(o bots.Options) (seat.Seat, error) {
			return noFloat{heuristic.NewBot(o.Seed).EnableAutoPayMana()}, nil
		}})
}

// noFloat is the bot of an engine_autopay host: gorge's decision still lists
// a tap of every mana source, but the host hides those taps from the v2
// candidates, so floating mana by hand has nothing to answer with. The bot
// pays by plan only, and neither world shows it the taps.
//
// It also reads the payment actions in card-name order. gorge lists them in
// the engine's hand order, which the observation hides (a hand is shown
// sorted), so which of two equally ranked casts comes first is not something
// the neutral world can know; the name order is the same in both worlds.
type noFloat struct{ *heuristic.Bot }

func (n noFloat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	if d.Kind != decision.KPriority {
		return n.Bot.Decide(ctx, v, d)
	}
	kept := make([]decision.Option, 0, len(d.Options))
	var back []int
	for _, o := range d.Options {
		if o.Kind != "activate" {
			back = append(back, o.Index)
			o.Index = len(kept)
			kept = append(kept, o)
		}
	}
	d.Options = kept
	names := map[state.ObjID]string{}
	for _, p := range v.Players {
		for _, zone := range [][]view.CardView{p.Hand, p.Graveyard, p.Exile, p.Battlefield} {
			for _, c := range zone {
				names[c.ID] = c.Name
			}
		}
	}
	d.PaymentActions = slices.Clone(d.PaymentActions)
	slices.SortStableFunc(d.PaymentActions, func(a, b decision.PaymentAction) int {
		return strings.Compare(names[a.Cast.Object], names[b.Cast.Object])
	})
	in, err := n.Bot.Decide(ctx, v, d)
	for i, c := range in.Choices {
		in.Choices[i] = back[c]
	}
	return in, err
}

// recorder passes requests to a gorge agent and keeps every choose request.
type recorder struct {
	inner minihost.Link
	seen  [][]byte
}

func (r *recorder) Round(req []byte) ([]byte, error) {
	if strings.Contains(string(req[:min(len(req), 64)]), `"choose"`) {
		r.seen = append(r.seen, slices.Clone(req))
	}
	return r.inner.Round(req)
}

// gorgeGames plays gorge bot mirrors on gorge's own engine and returns each
// choose request: the v2 observation with x_gorge_view_v1 beside it.
func gorgeGames(t *testing.T, games int) [][]byte {
	t.Helper()
	reg := testcorpus.Registry(t)
	kinds := map[string]bool{}
	for _, k := range env.DecisionKinds {
		kinds[k] = true
	}
	var out [][]byte
	for i := 0; i < games; i++ {
		d := catalog.Decks()[i%len(catalog.Decks())]
		h := &minihost.Host{RunSecret: make([]byte, 32), Engine: &minihost.EngineLink{S: env.New(reg, nil)},
			Profile:  validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{"x_gorge_view_v1": true}},
			MaxSteps: 20000, MaxDecisions: 9999}
		var links [2]minihost.Link
		var recs [2]*recorder
		for s := range links {
			a, err := agent.New("bot")
			if err != nil {
				t.Fatal(err)
			}
			recs[s] = &recorder{inner: a}
			links[s] = recs[s]
		}
		if _, err := h.Play(uint64(i), d, "london", []string{"x_gorge_view_v1"}, links); err != nil {
			t.Fatalf("game %d (%s): %v", i, d.CatalogID, err)
		}
		out = append(out, recs[0].seen...)
		out = append(out, recs[1].seen...)
	}
	return out
}

type chooseReq struct {
	Decision protocol.SeatDecision `json:"decision"`
}

// cardFacts is what gorge's Board reads off one card view, without its id.
func cardFacts(zone string, cv view.CardView) string {
	kw := slices.Clone(cv.Keywords)
	sort.Strings(kw)
	var prod string
	if cv.Produces != nil {
		prod = fmt.Sprint(cv.Produces.Colour, cv.Produces.Any)
	}
	return fmt.Sprintf("%s|%s|%q|%s|%s|%d/%d dmg%d tapped=%v sick=%v att=%v attached=%v blocked=%d kw=%v api=%s prod=%s",
		zone, cv.Name, cv.Types, cv.ManaCost, map[bool]string{true: "facedown"}[cv.FaceDown], cv.Power, cv.Toughness,
		cv.Damage, cv.Tapped, cv.SummonSick && zone == "battlefield", cv.Attacking, cv.AttachedTo != 0, len(cv.BlockedBy), kw, cv.SpellAPI, prod)
}

// facts lists a view's Board-relevant facts as sorted lines.
func facts(v view.View) []string {
	out := []string{fmt.Sprintf("viewer=%d active=%d phase=%s turn=%d", v.Viewer, v.Active, v.Phase, v.Turn)}
	for _, p := range v.Players {
		pool := []string{}
		for k, n := range p.Pool {
			if n != 0 {
				pool = append(pool, fmt.Sprint(k, n))
			}
		}
		sort.Strings(pool)
		out = append(out, fmt.Sprintf("p%d life=%d lib=%d hand=%d pool=%v", p.ID, p.Life, p.LibrarySize, p.HandSize, pool))
		for _, z := range []struct {
			name string
			cvs  []view.CardView
		}{{"hand", p.Hand}, {"battlefield", p.Battlefield}, {"graveyard", p.Graveyard}, {"exile", p.Exile}} {
			for _, cv := range z.cvs {
				out = append(out, fmt.Sprintf("p%d %s", p.ID, cardFacts(z.name, cv)))
			}
		}
	}
	for _, sv := range v.Stack {
		cost := ""
		if sv.Kind == "spell" && sv.Card != nil {
			cost = sv.Card.ManaCost
		}
		out = append(out, fmt.Sprintf("stack %v p%d %s", sv.Kind == "spell", sv.Controller, cost))
	}
	sort.Strings(out)
	return out
}

func TestViewMatchesGorgeViewOnGorgeGames(t *testing.T) {
	reg := testcorpus.Registry(t)
	reqs := gorgeGames(t, 5)
	builders := map[string]*neutral.Builder{}
	var checked, differ int
	diffs := map[string]int{}
	for _, raw := range reqs {
		var q chooseReq
		if err := json.Unmarshal(raw, &q); err != nil {
			t.Fatal(err)
		}
		var p xview.Payload
		if err := json.Unmarshal(q.Decision.Extensions["x_gorge_view_v1"], &p); err != nil {
			t.Fatal(err)
		}
		b := builders[q.Decision.ActingSeat]
		if b == nil {
			b = neutral.NewBuilder(reg)
			builders[q.Decision.ActingSeat] = b
		}
		got, err := b.View(&q.Decision.Observation)
		if err != nil {
			t.Fatal(err)
		}
		want, _ := agent.Rebuild(p)
		g, w := facts(got), facts(want)
		checked++
		if !slices.Equal(g, w) {
			differ++
			for _, l := range w {
				if !slices.Contains(g, l) {
					diffs["want "+l]++
				}
			}
			for _, l := range g {
				if !slices.Contains(w, l) {
					diffs["got  "+l]++
				}
			}
		}
	}
	var lines []string
	for l, n := range diffs {
		lines = append(lines, fmt.Sprintf("%5d %s", n, l))
	}
	sort.Strings(lines)
	if os.Getenv("NEUTRAL_VERBOSE") != "" {
		t.Log(strings.Join(lines, "\n"))
	}
	t.Logf("%d of %d views differ", differ, checked)
	if differ != 0 {
		t.Errorf("%d of %d views differ from gorge's own view; first lines:\n%s", differ, checked, strings.Join(lines[:min(len(lines), 40)], "\n"))
	}
}

// twin forwards every request to a gorge agent reading x_gorge_view_v1 and
// to one translating the bare observation, answers with the first and
// counts agreement by the decision's first candidate kind.
type twin struct {
	reg           *cards.Registry
	view, neutral *agent.Server
	agree, total  map[string]int
	examples      map[string][]string
}

func (tw *twin) Round(req []byte) ([]byte, error) {
	a, _ := tw.view.Round(req)
	b, _ := tw.neutral.Round(req)
	var q struct {
		RequestType string                `json:"request_type"`
		Decision    protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(req, &q); err != nil || q.RequestType != "choose" {
		if q.RequestType == "game_start" && !strings.Contains(string(b), `"ack"`) {
			return nil, fmt.Errorf("neutral game_start: %s", b)
		}
		return a, nil
	}
	kind := q.Decision.Candidates[0].Semantic.Kind
	if q.Decision.Context.Kind == "priority" {
		kind = "priority"
	}
	if len(q.Decision.Candidates) == 1 {
		kind += "(single)"
	}
	tw.total[kind]++
	var ca, cb struct {
		Selection struct {
			CandidateID int `json:"candidate_id"`
		} `json:"selection"`
	}
	json.Unmarshal(a, &ca)
	json.Unmarshal(b, &cb)
	if tw.chosenColour(&q.Decision) {
		// A source whose mana is a colour chosen as it entered: the
		// observation does not carry that choice (permanent.chosen is null),
		// so the neutral planner cannot see what the source makes.
		tw.total[kind]--
		tw.total["(chosen-colour source in play)"]++
		return a, nil
	}
	if ca.Selection.CandidateID == cb.Selection.CandidateID ||
		sameCard(q.Decision.Candidates[ca.Selection.CandidateID].Semantic, q.Decision.Candidates[cb.Selection.CandidateID].Semantic) {
		tw.agree[kind]++
	} else if len(tw.examples[kind]) < 14 {
		sa, _ := json.Marshal(q.Decision.Candidates[ca.Selection.CandidateID].Semantic)
		sb, _ := json.Marshal(q.Decision.Candidates[cb.Selection.CandidateID].Semantic)
		tw.examples[kind] = append(tw.examples[kind], fmt.Sprintf("gorge %s\n      neutral %s", sa, sb))
	}
	return a, nil
}

// sameCard reports whether two candidates play the same card: the engine
// hides the order of interchangeable copies in a hand, so which of them a
// seat plays first is not a decision.
func sameCard(x, y protocol.Semantic) bool {
	if x.Kind != y.Kind || x.Kind != "cast_spell" && x.Kind != "play_land" {
		return false
	}
	anon := func(s protocol.Semantic) string {
		raw, _ := json.Marshal(s)
		var m map[string]any
		json.Unmarshal(raw, &m)
		if src, ok := m["source"].(map[string]any); ok {
			delete(src, "object_id")
		}
		out, _ := json.Marshal(m)
		return string(out)
	}
	return anon(x) == anon(y)
}

// chosenColour reports whether the acting seat controls a permanent whose
// mana depends on a colour chosen as it entered.
func (tw *twin) chosenColour(sd *protocol.SeatDecision) bool {
	for _, p := range sd.Observation.Players {
		for _, r := range p.Battlefield {
			if r.CardName == nil {
				continue
			}
			c, ok := tw.reg.Lookup(*r.CardName)
			if !ok {
				continue
			}
			for _, f := range c.Faces {
				for _, a := range f.Abilities {
					if a.API == "Mana" && strings.Contains(a.Params["Produced"], "Chosen") {
						return true
					}
				}
			}
		}
	}
	return false
}

func TestDecisionsMatchGorgeOnGorgeGames(t *testing.T) {
	reg := testcorpus.Registry(t)
	kinds := map[string]bool{}
	for _, k := range env.DecisionKinds {
		kinds[k] = true
	}
	tw := &twin{reg: reg, agree: map[string]int{}, total: map[string]int{}, examples: map[string][]string{}}
	games := 10
	for i := 0; i < games; i++ {
		d := catalog.Decks()[i%len(catalog.Decks())]
		// Engine auto-pay, the default path: the native agent reads gorge's
		// payment plans (PaymentActions) off the engine; the neutral one has
		// to plan the same payments on the board it rebuilds from the
		// observation.
		srv := env.New(reg, nil)
		srv.EnableAutoPay()
		h := &minihost.Host{RunSecret: make([]byte, 32), Engine: &minihost.EngineLink{S: srv},
			Profile:  validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{"x_gorge_view_v1": true}},
			MaxSteps: 20000, MaxDecisions: 9999}
		var links [2]minihost.Link
		for s := range links {
			va, _ := agent.New("test-bot-auto-pay")
			na, _ := agent.New("test-bot-auto-pay")
			na.SetRegistry(reg)
			if err := na.EnableNeutral(); err != nil {
				t.Fatal(err)
			}
			links[s] = &twinSeat{tw: tw, view: va, neutral: na}
		}
		if _, err := h.Play(uint64(i), d, "london", []string{"x_gorge_view_v1"}, links); err != nil {
			t.Fatalf("game %d (%s): %v", i, d.CatalogID, err)
		}
	}
	var lines []string
	agree, total := 0, 0
	for k, n := range tw.total {
		agree += tw.agree[k]
		total += n
		lines = append(lines, fmt.Sprintf("%-32s %5d/%5d", k, tw.agree[k], n))
		for _, e := range tw.examples[k] {
			lines = append(lines, "    "+e)
		}
	}
	sort.Strings(lines)
	t.Logf("agreement %d/%d\n%s", agree, total, strings.Join(lines, "\n"))
	skipped := tw.total["(chosen-colour source in play)"]
	if agree != total-skipped {
		t.Errorf("the translated decision disagrees with gorge's own %d times in %d", total-skipped-agree, total-skipped)
	}
}

// twinSeat binds one seat's pair of agents to the shared tally.
type twinSeat struct {
	tw            *twin
	view, neutral *agent.Server
}

func (s *twinSeat) Round(req []byte) ([]byte, error) {
	s.tw.view, s.tw.neutral = s.view, s.neutral
	return s.tw.Round(req)
}
