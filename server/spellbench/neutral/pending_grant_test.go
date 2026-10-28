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
