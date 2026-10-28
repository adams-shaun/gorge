package neutral

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// TestNeutralGrantMatchesNativeAtPendingGrant pins decision.Option.Grant, the
// server-side no-op fact the bot's A1 rule reads: an activated ability whose
// whole effect is a pure keyword grant is redundant while an identical grant
// from the same source is on the stack. The fixture is the recorded choose
// request at the first Fledgling Imp divergence (seed 6063..., index 426):
// the native bot passed because the engine filled Duplicate, while the
// neutral world activated because it built no Grant at all. The session is
// told it answered the activation decision that put the ability on the stack,
// exactly the state a self-played neutral game reaches.
func TestNeutralGrantMatchesNativeAtPendingGrant(t *testing.T) {
	reg := testcorpus.Registry(t)
	raw, err := os.ReadFile("testdata/pending_grant_fledgling_imp.json")
	if err != nil {
		t.Fatal(err)
	}
	var q struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	sd := q.Decision
	var p xview.Payload
	if err := json.Unmarshal(sd.Extensions["x_gorge_view_v1"], &p); err != nil {
		t.Fatal(err)
	}
	// Precondition: the native decision carries the redundant grant.
	var nativeGrant *decision.Grant
	for _, o := range p.Decision.Options {
		if o.Kind == "ability" && o.Index < len(p.Facts.Options) {
			nativeGrant = p.Facts.Options[o.Index].Grant
		}
	}
	if nativeGrant == nil || !nativeGrant.Duplicate || len(nativeGrant.Keywords) != 1 || nativeGrant.Keywords[0] != "Flying" {
		t.Fatalf("precondition: native grant = %+v, want a pending Flying grant", nativeGrant)
	}
	// Precondition: the observation shows the activated ability on the stack,
	// which is what the session's Duplicate read keys on.
	var srcID string
	for _, e := range sd.Observation.Stack {
		if e.StackKind == "activated_ability" && e.Source != nil && e.Source.CardName != nil &&
			*e.Source.CardName == "Fledgling Imp" {
			srcID = e.Source.ObjectID
		}
	}
	if srcID == "" {
		t.Fatal("precondition: no Fledgling Imp activated ability on the observed stack")
	}

	s := NewSession(reg)
	s.SetAutoPay(true)
	// The previous choose request was that activation; Picked records the
	// source's latest activation and its pure-grant keywords.
	cand := -1
	for i, c := range sd.Candidates {
		if c.Semantic.Kind == "activate_ability" {
			cand = i
		}
	}
	if cand < 0 {
		t.Fatal("precondition: no activate_ability candidate")
	}
	s.last = &sd
	s.Picked(cand)
	pl, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	var got *decision.Grant
	for _, o := range pl.Decision.Options {
		if o.Kind == "ability" && o.Grant != nil {
			got = o.Grant
		}
	}
	if got == nil {
		t.Fatalf("rebuilt ability option carries no Grant; native %+v", nativeGrant)
	}
	if !got.Duplicate || got.Already != nativeGrant.Already || len(got.Keywords) != len(nativeGrant.Keywords) ||
		got.Keywords[0] != nativeGrant.Keywords[0] {
		t.Fatalf("rebuilt grant = %+v, native %+v", got, nativeGrant)
	}
}

// TestNeutralGrantSurvivesCostTargetDecision is the MAJOR finding from review:
// the session pruned its recorded grant on every Payload, so the non-priority
// choose_cost_target decision an activation poses before its ability is on the
// stack (Fledgling Imp's "Discard a card") deleted the grant, and the later
// priority decision rebuilt Duplicate=false. Build that intermediate decision
// from the same fixture's hand (three cards, so the discard cost is really
// posed), assert it translated as a cost ask with the grant still recorded,
// then assert the stack-bearing priority decision still reports Duplicate.
func TestNeutralGrantSurvivesCostTargetDecision(t *testing.T) {
	reg := testcorpus.Registry(t)
	raw, err := os.ReadFile("testdata/pending_grant_fledgling_imp.json")
	if err != nil {
		t.Fatal(err)
	}
	var q struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	sd := q.Decision
	var p xview.Payload
	if err := json.Unmarshal(sd.Extensions["x_gorge_view_v1"], &p); err != nil {
		t.Fatal(err)
	}
	var nativeGrant *decision.Grant
	for _, o := range p.Decision.Options {
		if o.Kind == "ability" && o.Index < len(p.Facts.Options) {
			nativeGrant = p.Facts.Options[o.Index].Grant
		}
	}
	if nativeGrant == nil || !nativeGrant.Duplicate {
		t.Fatalf("precondition: native grant = %+v, want Duplicate", nativeGrant)
	}

	// The activation candidate names the source and ability; the hand must
	// hold more than one card or gorge would not pose the discard cost.
	var src protocol.ObjectRef
	cand := -1
	for i, c := range sd.Candidates {
		if c.Semantic.Kind == "activate_ability" {
			src, err = ref(c.Semantic, "source")
			if err != nil {
				t.Fatal(err)
			}
			cand = i
		}
	}
	if cand < 0 {
		t.Fatal("precondition: no activate_ability candidate")
	}
	var hand []protocol.ObjectRecord
	for _, pl := range sd.Observation.Players {
		if pl.Seat == "p1" {
			hand = pl.Hand
		}
	}
	if len(hand) < 2 {
		t.Fatalf("precondition: hand has %d cards, want >1 for a discard cost", len(hand))
	}

	// The intermediate decision: choose which card to discard, before the
	// ability is on the stack. It is a choose_cost_target, not a priority ask.
	inter := sd
	inter.Context.Kind = "choice"
	inter.Observation.Stack = nil
	inter.Candidates = nil
	for _, c := range hand[:2] {
		b, err := json.Marshal(map[string]any{"semantic": map[string]any{
			"kind": "choose_cost_target", "cost_kind": "discard", "source": src,
			"minimum": 1, "maximum": 1, "candidate": protocol.ObjectTarget(c.ObjectRef),
		}})
		if err != nil {
			t.Fatal(err)
		}
		var cost protocol.Candidate
		if err := json.Unmarshal(b, &cost); err != nil {
			t.Fatal(err)
		}
		inter.Candidates = append(inter.Candidates, cost)
	}

	s := NewSession(reg)
	s.SetAutoPay(true)
	s.last = &sd
	s.Picked(cand)
	mid, err := s.Payload(&inter)
	if err != nil {
		t.Fatal(err)
	}
	if mid.Decision.Kind != decision.KChoose {
		t.Fatalf("precondition: intermediate decision = %v, want a cost choose", mid.Decision.Kind)
	}
	if _, ok := s.grants[src.ObjectID]; !ok {
		t.Fatal("intermediate non-priority Payload dropped the recorded grant")
	}

	pl, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	var got *decision.Grant
	for _, o := range pl.Decision.Options {
		if o.Kind == "ability" && o.Grant != nil {
			got = o.Grant
		}
	}
	if got == nil {
		t.Fatalf("rebuilt ability option carries no Grant; native %+v", nativeGrant)
	}
	if !got.Duplicate || got.Already != nativeGrant.Already || len(got.Keywords) != len(nativeGrant.Keywords) ||
		got.Keywords[0] != nativeGrant.Keywords[0] {
		t.Fatalf("rebuilt grant after cost decision = %+v, native %+v", got, nativeGrant)
	}
}
