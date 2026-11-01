package manabrewhttp

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/manabrew"
	"github.com/adams-shaun/gorge/server/protocol"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// mustAttackCard is a free 2/2 that must attack, so the engine's KAttackers
// decision always carries a Required option and an empty declaration is
// wire-legal but engine-refused (CR 508.1d's required quota).
func mustAttackCard(t *testing.T) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("engine_refusal_test.txt", []byte(
		"Name:Charger\nManaCost:0\nTypes:Creature Charger\nPT:2/2\n"+
			"S:Mode$ MustAttack | ValidCreature$ Card.Self | Description$ attacks each combat if able.\n"+
			"Oracle:x\n"))
	if len(diags) != 0 {
		t.Fatalf("parse MustAttack card: %v", diags)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

func mustAttackDeck(t *testing.T) []*cards.Card {
	t.Helper()
	c := mustAttackCard(t)
	out := make([]*cards.Card, 40)
	for i := range out {
		out[i] = c
	}
	return out
}

// newRefusalTestServer is newTestServer with the MustAttack deck at every
// seat, so the human at seat 0 reaches a Required attackers decision.
func newRefusalTestServer(t *testing.T) *testServer {
	t.Helper()
	deck := mustAttackDeck(t)
	r, err := host.New(host.Options{
		LoadDeck: func(name string) (host.Deck, error) {
			switch name {
			case "a", "b", "c", "d":
				return host.Deck{Name: name, Cards: deck}, nil
			}
			return host.Deck{}, host.ErrNotFound
		},
		Sleep: func(time.Duration, <-chan struct{}) {},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := host.TableConfig{ID: "t1", Name: "T1", Seats: 4, Decks: []string{"a", "b", "c", "d"}, Seed: 42,
		Spectator: view.Omniscient, Humans: []int{0}, AutoMana: true}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	byToken := map[string]SeatClaim{"s0": {Table: "t1", Seat: 0}}
	h, mux := newHandler(r, Options{Seat: func(req *http.Request) (SeatClaim, bool) {
		tok := strings.TrimPrefix(req.Header.Get("Authorization"), "Bearer ")
		c, ok := byToken[tok]
		return c, ok
	}})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &testServer{h: h, srv: srv, reg: r}
}

// legalIntent is the host tests' answer shape: the first Min options. For a
// priority decision (Min 1) that is the first offered action, which develops
// the board so a Required attackers decision eventually appears.
func legalIntent(d *decision.Decision) decision.Intent {
	n := d.Min
	if n > len(d.Options) {
		n = len(d.Options)
	}
	ch := make([]int, 0, n)
	for i := 0; i < n; i++ {
		ch = append(ch, d.Options[i].Index)
	}
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch}
}

func requiredAssignments(d *decision.Decision) []mb.AttackerAssignment {
	seen := map[state.ObjID]bool{}
	var out []mb.AttackerAssignment
	for _, o := range d.Options {
		if !o.Required || seen[o.Obj] {
			continue
		}
		seen[o.Obj] = true
		out = append(out, mb.AttackerAssignment{
			AttackerID: manabrew.CardID(o.Obj),
			TargetID:   manabrew.AttackTargetID(o),
		})
	}
	return out
}

// TestSendEngineRefusalNamesReasonAndKeepsPrompt pins the /send path: an
// empty attackers declaration that only the engine refuses gets a
// ProtocolError naming that refusal (not stalePrompt), the same decision
// stays pending, and a legal declaration then answers it.
func TestSendEngineRefusalNamesReasonAndKeepsPrompt(t *testing.T) {
	ts := newRefusalTestServer(t)
	deadline := time.Now().Add(30 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("match never reached a Required attackers decision")
		}
		ms, err := ts.reg.Matches("t1")
		if err != nil {
			t.Fatal(err)
		}
		for _, mi := range ms {
			if mi.State == protocol.MatchCrashed {
				t.Fatalf("match crashed before the refusal: %s", mi.Head)
			}
		}
		d, err := ts.reg.Pending("t1", 1, 0)
		if err != nil {
			time.Sleep(time.Millisecond)
			continue
		}
		if d.Kind == decision.KAttackers && d.RequiredQuota() > 0 {
			empty := mb.ClientMessage{Value: mb.ClientResponse{PromptID: int64(d.Seq), Action: mb.PromptOutput{
				Type: "chooseAttackers", Output: mb.PromptOutputData{Value: mb.DeclareAttackersDecision{}}}}}
			code, pe := send(t, ts, "s0", empty)
			if code != http.StatusBadRequest {
				t.Fatalf("refused declaration: status %d body %+v, want 400", code, pe)
			}
			if pe.Code != mb.CodeInvalidShape {
				t.Fatalf("refused declaration code = %q, want invalidShape (not stalePrompt)", pe.Code)
			}
			if !strings.Contains(pe.Message, "engine refused the intent:") ||
				!strings.Contains(pe.Message, "must attack with as many required creatures as possible") {
				t.Fatalf("refusal message = %q, want the engine's required-quota reason", pe.Message)
			}
			if pe.PromptID == nil || *pe.PromptID != int64(d.Seq) {
				t.Fatalf("refusal promptId = %v, want %d", pe.PromptID, d.Seq)
			}

			again, err := ts.reg.Pending("t1", 1, 0)
			if err != nil {
				t.Fatalf("pending after refusal: %v", err)
			}
			if again.Seq != d.Seq || again.Kind != d.Kind {
				t.Fatalf("after refusal pending = seq %d kind %s, want seq %d kind %s", again.Seq, again.Kind, d.Seq, d.Kind)
			}

			legal := mb.ClientMessage{Value: mb.ClientResponse{PromptID: int64(d.Seq), Action: mb.PromptOutput{
				Type: "chooseAttackers", Output: mb.PromptOutputData{Value: mb.DeclareAttackersDecision{
					Assignments: requiredAssignments(d)}}}}}
			if code, pe := send(t, ts, "s0", legal); code != http.StatusNoContent {
				t.Fatalf("legal declaration after refusal: status %d body %+v, want 204", code, pe)
			}
			return
		}
		if err := ts.reg.SubmitIntent("t1", 1, 0, legalIntent(d)); err != nil {
			t.Fatalf("SubmitIntent(seq %d kind %s): %v", d.Seq, d.Kind, err)
		}
	}
}

// TestStreamAutoPassRefusalIsReportedAndPromptStaysAnswerable pins the stream
// path: when the transport's own auto-pass submit is refused, the stream
// emits a ProtocolError naming the refusal, clears the policy, and still
// writes the open prompt; a later /send answers that same prompt.
func TestStreamAutoPassRefusalIsReportedAndPromptStaysAnswerable(t *testing.T) {
	ts := newTestServer(t, []int{0}, 0, 0)
	d := waitForPending(t, ts.reg, "t1", 1, 0)
	if d.Kind != decision.KPriority {
		t.Fatalf("first decision is %s, want priority", d.Kind)
	}

	sc := ts.h.connFor(connKey{table: "t1", match: 1, seat: 0})
	sc.setPolicy(&manabrew.PassPolicy{UntilPlayer: "player-3", UntilPhase: mb.StepEndOfTurn})

	var refused atomic.Bool
	ts.h.submitIntentFn = func(tid host.TableID, k int, seat state.PlayerID, in decision.Intent) error {
		if refused.CompareAndSwap(false, true) {
			return errors.New("auto-pass refused by test")
		}
		return ts.reg.SubmitIntent(tid, k, seat, in)
	}

	s := openStream(t, ts.srv, "/t1/matches/1/stream", "s0")
	defer s.close()

	var gotErr *mb.ProtocolError
	var gotPrompt *mb.PromptMessage
	for gotErr == nil || gotPrompt == nil {
		msg := s.next(t)
		switch v := msg.Value.(type) {
		case mb.ErrorMessage:
			e := v.Error
			gotErr = &e
		case mb.PromptMessage:
			pm := v
			gotPrompt = &pm
		}
	}
	if gotErr.Code != mb.CodeInvalidShape {
		t.Fatalf("stream refusal code = %q, want invalidShape", gotErr.Code)
	}
	if !strings.Contains(gotErr.Message, "engine refused the intent: auto-pass refused by test") {
		t.Fatalf("stream refusal message = %q, want the engine's refusal reason", gotErr.Message)
	}
	if gotErr.PromptID == nil || *gotErr.PromptID != int64(d.Seq) {
		t.Fatalf("stream refusal promptId = %v, want %d", gotErr.PromptID, d.Seq)
	}
	if gotPrompt.PromptID != int64(d.Seq) {
		t.Fatalf("prompt after refusal = %d, want the still-open %d", gotPrompt.PromptID, d.Seq)
	}

	resp := mb.ClientMessage{Value: mb.ClientResponse{PromptID: int64(d.Seq), Action: mb.PromptOutput{
		Type: "chooseAction", Output: mb.PromptOutputData{Value: mb.PassOutput{}}}}}
	if code, pe := send(t, ts, "s0", resp); code != http.StatusNoContent {
		t.Fatalf("answering the prompt after the stream refusal: status %d body %+v, want 204", code, pe)
	}
}
