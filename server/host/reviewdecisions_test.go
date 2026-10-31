package host

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/protocol"
)

// TestReviewDecisionsOnlyOnAReviewTable: the shared finished match loaded
// as a review table carries one decision entry per intent, its explain
// entry riding on the matching intent; the live registry it came from has
// no index at all (ErrNotFound), and neither has a wrong match number.
func TestReviewDecisionsOnlyOnAReviewTable(t *testing.T) {
	t.Parallel()
	live, m := finishedTable(t)
	if _, err := live.ReviewDecisions("t1", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("live ReviewDecisions = %v, want ErrNotFound", err)
	}
	r := reviewRegistry(t)
	n := len(m.e.L.Intents)
	explain := make([]json.RawMessage, n)
	explain[2] = json.RawMessage(`{"seat":0,"source":"forced"}`)
	explain[3] = json.RawMessage(`null`)
	if err := r.AddReviewTable("rv", ReviewMatch{Config: m.rulesConfig(), Log: m.e.L.Clone(), Explain: explain}); err != nil {
		t.Fatal(err)
	}
	// A misaligned sidecar is ignored by the host too (the loader warns).
	if err := r.AddReviewTable("rv2", ReviewMatch{Config: m.rulesConfig(), Log: m.e.L.Clone(), Explain: explain[:n-1]}); err != nil {
		t.Fatal(err)
	}
	ds, err := r.ReviewDecisions("rv", 1)
	if err != nil || len(ds) != n {
		t.Fatalf("ReviewDecisions = %d entries, %v; want %d", len(ds), err, n)
	}
	for i, d := range ds {
		want := ""
		if i == 2 {
			want = string(explain[2])
		}
		if string(d.Explain) != want {
			t.Fatalf("entry %d explain %q, want %q", i, d.Explain, want)
		}
		if i < len(m.bounds) && d.AskSeq != m.bounds[i]-1 {
			t.Fatalf("entry %d ask_seq %d, boundary %d", i, d.AskSeq, m.bounds[i])
		}
	}
	ds2, _ := r.ReviewDecisions("rv2", 1)
	for i, d := range ds2 {
		if d.Explain != nil {
			t.Fatalf("rv2 entry %d kept explain %s from a misaligned sidecar", i, d.Explain)
		}
	}
	if _, err := r.ReviewDecisions("rv", 2); !errors.Is(err, ErrNotFound) {
		t.Fatalf("match 2: %v", err)
	}
}

// TestReviewOptionsCap: a long option list keeps its first
// ReviewDecisionOptionCap options plus every index the answer and the
// explain note name, in index order; a short list is kept whole.
func TestReviewOptionsCap(t *testing.T) {
	t.Parallel()
	opts := make([]decision.Option, 1000)
	for i := range opts {
		opts[i] = decision.Option{Index: i, Kind: "name", Label: fmt.Sprintf("Card %d", i)}
	}
	in := decision.Intent{Choices: []int{700}}
	note := json.RawMessage(`{"bot":[900],"search":{"rows":[{"choices":[64]},{"choices":[999]}]}}`)
	got := reviewOptions(opts, in, note)
	limit := protocol.ReviewDecisionOptionCap
	if len(got) != limit+4 {
		t.Fatalf("%d options listed, want %d", len(got), limit+4)
	}
	for i := 0; i < limit; i++ {
		if got[i].Index != i || got[i].Label != opts[i].Label || got[i].Kind != "name" {
			t.Fatalf("option %d = %+v", i, got[i])
		}
	}
	tail := []int{got[limit].Index, got[limit+1].Index, got[limit+2].Index, got[limit+3].Index}
	if fmt.Sprint(tail) != "[64 700 900 999]" {
		t.Fatalf("kept beyond the cap: %v", tail)
	}
	if short := reviewOptions(opts[:3], decision.Intent{}, nil); len(short) != 3 {
		t.Fatalf("short list cut to %d", len(short))
	}
	if got := reviewOptions(opts, in, json.RawMessage(`not json`)); len(got) != limit+1 {
		t.Fatalf("an undecodable note widened the list to %d", len(got))
	}
}

// TestReviewAnswer: the answer label names the chosen options, an arrange
// answer's rest order, an empty answer, and a payment action.
func TestReviewAnswer(t *testing.T) {
	t.Parallel()
	d := &decision.Decision{Options: []decision.Option{{Index: 0, Label: "A"}, {Index: 1, Label: "B"}, {Index: 2, Label: "C"}},
		PaymentActions: []decision.PaymentAction{{ID: "cast:7", Label: "Cast Bolt"}}}
	for _, tc := range []struct {
		in   decision.Intent
		want string
	}{
		{decision.Intent{Choices: []int{1}}, "B"},
		{decision.Intent{Choices: []int{2, 0}}, "C; A"},
		{decision.Intent{}, "none"},
		{decision.Intent{Choices: []int{0}, Rest: []int{2, 1}}, "A | rest: C; B"},
		{decision.Intent{Payment: &decision.PaymentSelection{ActionID: "cast:7"}}, "cast with planned payment: Cast Bolt"},
		{decision.Intent{Announce: &decision.AnnounceSelection{ActionID: "cast:9"}}, "announce, then pay: cast:9"},
	} {
		if got := reviewAnswer(d, tc.in); got != tc.want {
			t.Errorf("reviewAnswer(%+v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
