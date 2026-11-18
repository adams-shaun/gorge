//go:build manabrew

package mbtest

// The documented pass response (docs/site/api/manabrew.md, "Skipping to your
// next turn") is the way a ManaBrew client skips gorge's per-step priority
// stops. These tests read that response out of the page, so a doc edit that
// breaks it fails here, and play four-seat games with every seat sending it.

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	manabrew "github.com/adams-shaun/gorge/server/manabrew"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const passUntilDocPath = "../../../docs/site/api/manabrew.md"

// passUntilFence is the first json block under the page's "Skipping to your next turn" heading.
var passUntilFence = regexp.MustCompile("(?s)\n## Skipping to your next turn\n.*?```json\n(.*?)\n```")

// docPassResponse decodes the documented send body and aims it at prompt
// promptID from player pid, the two values the page names as placeholders.
func docPassResponse(t testing.TB, promptID int64, pid string) mb.ClientMessage {
	t.Helper()
	page, err := os.ReadFile(passUntilDocPath)
	if err != nil {
		t.Fatalf("read the docs page: %v", err)
	}
	m := passUntilFence.FindSubmatch(page)
	if m == nil {
		t.Fatal("docs/site/api/manabrew.md has no json block under the 'Skipping to your next turn' heading")
	}
	var msg mb.ClientMessage
	if _, err := mb.Decode(m[1], &msg); err != nil {
		t.Fatalf("the documented response does not decode: %v\n%s", err, m[1])
	}
	resp, ok := msg.Value.(mb.ClientResponse)
	if !ok {
		t.Fatalf("documented body is a %T, want a prompt response", msg.Value)
	}
	if resp.Action.Type != "pass" {
		t.Fatalf("documented action type is %q, want pass", resp.Action.Type)
	}
	out, ok := resp.Action.Output.Value.(mb.PassOutput)
	if !ok || out.Until == nil {
		t.Fatalf("documented output is %T, want a pass with an until clause", resp.Action.Output.Value)
	}
	if out.Until.PlayerID != "player-0" {
		t.Fatalf("documented until player is %q; the page promises player-0 stands for your own id", out.Until.PlayerID)
	}
	out.Until.PlayerID = pid
	resp.PromptID = promptID
	resp.Action.Output.Value = out
	return mb.ClientMessage{Value: resp}
}

// passUntilSeat is a seat that plays like the first-legal client but answers
// every pass with the documented response, and services the auto-pass rule the
// way the manabrewhttp stream does: AutoPass first, the rule cleared by a
// stop or by a prompt that isn't a priority prompt.
type passUntilSeat struct {
	*TranslatingSeat
	policy   *manabrew.PassPolicy
	skipped  int  // priority decisions the rule answered
	posed    int  // priority prompts shown to the client
	armed    bool // the last answer installed the documented rule
	stops    int  // armed windows that ended at the until clause
	cleared  int  // armed windows that a non-priority prompt ended
	badStops []string
	noRule   bool // answer a plain pass instead: the control
	t        testing.TB
}

func (s *passUntilSeat) Decide(_ context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	s.Census.addEnumerated(d.Kind, d.Options)
	if in, ok := s.Translator.AutoPass(s.policy, &d, v); ok {
		s.skipped++
		return in, nil
	}
	if d.Kind != decision.KPriority {
		if s.armed {
			s.cleared++
		}
		s.armed, s.policy = false, nil
	} else if s.armed {
		// The rule ended at a priority prompt: that has to be its own stop.
		s.armed, s.policy = false, nil
		if int(v.Active) != int(d.Player) || v.Step != "upkeep" {
			s.badStops = append(s.badStops, fmt.Sprintf("turn %d active %d step %s, seat %d seq %d", v.Turn, v.Active, v.Step, d.Player, d.Seq))
		} else {
			s.stops++
		}
	}
	msg, _, err := s.Translator.PromptFlagged(&d, &v)
	if err != nil {
		s.Census.addUnmapped(d.Kind)
		return decision.Intent{}, err
	}
	s.Census.addPosed(msg.Input.Value.PromptType())
	if d.Kind == decision.KPriority {
		s.posed++
	}
	s.Client.SetTurn(v.Turn)
	resp, err := s.Client.Answer(msg)
	if err != nil {
		return decision.Intent{}, err
	}
	rv, _ := resp.Value.(mb.ClientResponse)
	_, isPass := rv.Action.Output.Value.(mb.PassOutput)
	if isPass && !s.noRule {
		resp = docPassResponse(s.t, int64(msg.PromptID), msg.DecidingPlayerID)
	}
	raw, err := mb.Encode(resp)
	if err != nil {
		return decision.Intent{}, err
	}
	var wire mb.ClientMessage
	if _, err := mb.Decode(raw, &wire); err != nil {
		return decision.Intent{}, err
	}
	out := s.Translator.TranslateResponse(wire, &manabrew.Pending{Prompt: msg, Decision: &d, View: v}, d.Player)
	if out.Err != nil {
		s.Census.addRejected(out.Err.Code)
		return decision.Intent{}, fmt.Errorf("%s (%s) on seq %d", out.Err.Code, out.Err.Message, d.Seq)
	}
	if out.Intent == nil {
		return decision.Intent{}, fmt.Errorf("no intent for seq %d", d.Seq)
	}
	if isPass && !s.noRule {
		s.policy, s.armed = out.Policy, out.Policy != nil && out.Policy.UntilPlayer != ""
	}
	return *out.Intent, nil
}

// playPassUntil plays one four-seat game with every seat a passUntilSeat.
func playPassUntil(t *testing.T, reg *cards.Registry, seed uint64, noRule bool) ([]*passUntilSeat, *rules.Engine, bench.Outcome, *Census) {
	t.Helper()
	const n = 4
	names := testutil.LegacyDeckNames()
	playerNames := make([]string, n)
	decks := make([][]*cards.Card, n)
	for i := 0; i < n; i++ {
		playerNames[i] = names[(int(seed)+i)%len(names)]
		decks[i] = testutil.RepoDeck(t, reg, playerNames[i])
	}
	cfg := rules.Config{Seed: seed, Names: playerNames, Decks: decks, Tokens: reg.Tokens, NameUniverse: reg.Universe()}
	census := NewCensus()
	client := NewFirstLegalClient()
	seats := make([]*passUntilSeat, n)
	ss := make([]seat.Seat, n)
	for i := range ss {
		seats[i] = &passUntilSeat{TranslatingSeat: NewTranslatingSeat("passuntil", int64(seed), client, census), noRule: noRule, t: t}
		ss[i] = seats[i]
	}
	o, e, err := bench.PlayGame(cfg, ss, censusMaxTurns, censusMaxIntents, bench.Hooks{})
	if err != nil {
		t.Fatalf("seed %d: PlayGame: %v", seed, err)
	}
	if bench.IsAbort(o.StallOn) {
		t.Fatalf("seed %d: engine abort (%s): %s", seed, o.StallOn, o.Livelock)
	}
	return seats, e, o, census
}

// TestPassUntilOwnTurnDocumentedResponse: the page's response decodes, and the
// translator turns it into an ordinary pass plus an active rule aimed at the
// asking seat's own upkeep.
func TestPassUntilOwnTurnDocumentedResponse(t *testing.T) {
	msg := docPassResponse(t, 22, "player-3")
	resp := msg.Value.(mb.ClientResponse)
	out := resp.Action.Output.Value.(mb.PassOutput)
	if out.Until.Phase != mb.StepUpkeep || out.Until.PlayerID != "player-3" || out.ExhaustStack || resp.PromptID != 22 {
		t.Fatalf("documented pass = %+v until %+v prompt %d", out, *out.Until, resp.PromptID)
	}
}

// TestPassUntilOwnTurnFourSeats plays four-seat games where every pass is the
// documented response. The rule must skip prompts, every skip must end at the
// seat's own upkeep (or at a prompt that is not a priority prompt), the games
// must finish with no rejection, and the seats must be posed far fewer priority
// prompts than the same seats sending a plain pass.
func TestPassUntilOwnTurnFourSeats(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	var posedRule, posedPlain, skipped, stops int
	for g := 0; g < 3; g++ {
		seed := uint64(4)*3000 + uint64(g)
		ruled, _, _, census := playPassUntil(t, reg, seed, false)
		plain, _, _, _ := playPassUntil(t, reg, seed, true)
		if n := census.TotalRejected(); n > 0 {
			t.Fatalf("seed %d: %d answer(s) refused: %v", seed, n, census.Rejected)
		}
		if n := census.TotalUnmapped(); n > 0 {
			t.Fatalf("seed %d: %d unmapped: %v", seed, n, census.Unmapped)
		}
		for i, s := range ruled {
			if len(s.badStops) > 0 {
				t.Fatalf("seed %d seat %d: the rule ended away from its own upkeep: %v", seed, i, s.badStops)
			}
			posedRule += s.posed
			skipped += s.skipped
			stops += s.stops
		}
		for _, s := range plain {
			if s.skipped != 0 {
				t.Fatalf("seed %d: the control seat skipped %d prompts with no rule", seed, s.skipped)
			}
			posedPlain += s.posed
		}
	}
	t.Logf("priority prompts posed: %d with the documented pass, %d with a plain pass; %d skipped, %d stops at own upkeep", posedRule, posedPlain, skipped, stops)
	if skipped == 0 || stops == 0 {
		t.Fatalf("the rule never engaged: %d skipped, %d stops", skipped, stops)
	}
	if posedRule*2 > posedPlain {
		t.Fatalf("the documented pass posed %d priority prompts against %d for a plain pass; want under half", posedRule, posedPlain)
	}
}
