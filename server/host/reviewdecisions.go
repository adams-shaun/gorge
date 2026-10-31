package host

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

// ReviewDecisions is a review table's decision index (match reviewer,
// design T4): one entry per intent of match k, in log order, built once by
// AddReviewTable's load walk. Only a review table has one; a live table, a
// missing table and a missing match are all ErrNotFound, so the route that
// serves this (mounted only by `gorged review`) can never reach a live
// match's state. A table on another engine refuses with ErrEngineUnsupported:
// the index is built by a walk of gorge's engine. The slice is shared and
// read-only.
func (r *Registry) ReviewDecisions(id TableID, k int) ([]protocol.ReviewDecision, error) {
	r.mu.RLock()
	t, ok := r.tables[id]
	r.mu.RUnlock()
	if ok && t.cfg.Engine != "" {
		return nil, engineRefusal("a decision index", t.cfg.Engine)
	}
	if !ok || !t.review || k != 1 {
		return nil, fmt.Errorf("%w: no review decisions for table %s match %d", ErrNotFound, id, k)
	}
	if t.decisions == nil {
		return []protocol.ReviewDecision{}, nil
	}
	return t.decisions, nil
}

// reviewDecisionIndex accumulates ReviewDecision entries during the review
// load walk. made[i] is the seq of the i-th DecisionMade event: every
// Submit emits exactly one, first, so it is intent i's.
type reviewDecisionIndex struct {
	l       *events.Log
	explain []json.RawMessage
	made    []uint64
	out     []protocol.ReviewDecision
	// turn and round memoise view.RoundOf: the round only moves at a
	// TurnChange, so it is refolded only when the turn number changed.
	turn  int32
	round int32
}

func newReviewDecisionIndex(l *events.Log, explain []json.RawMessage) *reviewDecisionIndex {
	ix := &reviewDecisionIndex{l: l, explain: explain, out: make([]protocol.ReviewDecision, 0, len(l.Intents)), turn: -1}
	ix.made = make([]uint64, 0, len(l.Intents))
	for _, ev := range l.Events {
		if ev.Kind == events.DecisionMade {
			ix.made = append(ix.made, ev.Seq)
		}
	}
	return ix
}

// at indexes intent i: e is the replay engine at boundary i, so its pending
// decision is the one Log.Intents[i] answered. The option labels are the
// omniscient spectator projection's own -- the text the live client renders
// for that pending decision -- never a second labeller.
func (ix *reviewDecisionIndex) at(e *rules.Engine, i int) error {
	d := e.Pending()
	if d == nil {
		return fmt.Errorf("intent %d: no pending decision at its boundary", i)
	}
	in := ix.l.Intents[i]
	if g := e.G.Turn; g != ix.turn {
		ix.turn, ix.round = g, view.RoundOf(e.G, e.L.Events)
	}
	v := view.ProjectFor(e.G, e, view.NoSeat, view.Omniscient, d)
	pd := v.Decision
	if pd == nil {
		pd = d
	}
	var note json.RawMessage
	if ix.explain != nil {
		note = ix.explain[i]
		if len(note) == 0 || string(note) == "null" {
			note = nil
		}
	}
	ent := protocol.ReviewDecision{
		Intent: i, AskSeq: d.Seq, Turn: e.G.Turn, Round: ix.round, Seat: uint8(d.Player),
		Kind: string(d.Kind), Prompt: pd.Prompt, Min: d.Min, Max: d.Max,
		OptionsTotal: len(pd.Options), Chosen: append([]int{}, in.Choices...),
		Rest: append([]int(nil), in.Rest...), Explain: note,
	}
	if i < len(ix.made) {
		ent.MadeSeq = ix.made[i]
		ent.Made = ix.l.Events[ix.made[i]].Text
	}
	ent.Options = reviewOptions(pd.Options, in, note)
	ent.Answer = reviewAnswer(pd, in)
	ix.out = append(ix.out, ent)
	return nil
}

// reviewOptions lists opts, capped at protocol.ReviewDecisionOptionCap,
// always keeping every index the answer names (Choices, Rest) and every
// index the explain note names (its bot answer and search rows' choices),
// so each of those can be labelled; the result is in index order.
func reviewOptions(opts []decision.Option, in decision.Intent, note json.RawMessage) []protocol.ReviewOption {
	keep := func(o decision.Option) protocol.ReviewOption {
		return protocol.ReviewOption{Index: o.Index, Label: o.Label, Kind: o.Kind}
	}
	if len(opts) <= protocol.ReviewDecisionOptionCap {
		out := make([]protocol.ReviewOption, len(opts))
		for j, o := range opts {
			out[j] = keep(o)
		}
		return out
	}
	want := map[int]bool{}
	for j := 0; j < protocol.ReviewDecisionOptionCap; j++ {
		want[j] = true
	}
	for _, c := range in.Choices {
		want[c] = true
	}
	for _, c := range in.Rest {
		want[c] = true
	}
	for _, c := range noteIndices(note) {
		want[c] = true
	}
	idx := make([]int, 0, len(want))
	for j := range want {
		if j >= 0 && j < len(opts) {
			idx = append(idx, j)
		}
	}
	sort.Ints(idx) // a set: the order is the index order, never the map's
	out := make([]protocol.ReviewOption, len(idx))
	for n, j := range idx {
		out[n] = keep(opts[j])
	}
	return out
}

// noteIndices reads the option indices an explain note names -- its "bot"
// answer and each search row's "choices" -- and nothing else. The host does
// not import bots/explain: the note stays opaque bytes on the wire, and this
// narrow read only widens the option cap for a note whose choices fall past
// it. A note that does not decode names nothing.
func noteIndices(note json.RawMessage) []int {
	if len(note) == 0 {
		return nil
	}
	var n struct {
		Bot    []int `json:"bot"`
		Search *struct {
			Rows []struct {
				Choices []int `json:"choices"`
			} `json:"rows"`
		} `json:"search"`
	}
	if json.Unmarshal(note, &n) != nil {
		return nil
	}
	out := append([]int(nil), n.Bot...)
	if n.Search != nil {
		for _, r := range n.Search.Rows {
			out = append(out, r.Choices...)
		}
	}
	return out
}

// reviewAnswer is the answer's short label: the payment action a planned
// or announced cast selected, else the chosen options' labels in answer
// order (an arrange answer adds its Rest order), "none" for an empty answer.
// The decision_made text (ReviewDecision.Made) is the verbatim record
// beside it; this only names what the indices mean.
func reviewAnswer(d *decision.Decision, in decision.Intent) string {
	label := func(j int) string {
		if j >= 0 && j < len(d.Options) {
			return d.Options[j].Label
		}
		return fmt.Sprintf("option %d", j)
	}
	action := func(id string) string {
		for _, a := range d.PaymentActions {
			if a.ID == id {
				return a.Label
			}
		}
		return id
	}
	switch {
	case in.Payment != nil:
		return "cast with planned payment: " + action(in.Payment.ActionID)
	case in.Announce != nil:
		return "announce, then pay: " + action(in.Announce.ActionID)
	}
	if len(in.Choices) == 0 && len(in.Rest) == 0 {
		return "none"
	}
	parts := make([]string, len(in.Choices))
	for n, c := range in.Choices {
		parts[n] = label(c)
	}
	s := strings.Join(parts, "; ")
	if len(in.Choices) == 0 {
		s = "none"
	}
	if len(in.Rest) > 0 {
		rest := make([]string, len(in.Rest))
		for n, c := range in.Rest {
			rest[n] = label(c)
		}
		s += " | rest: " + strings.Join(rest, "; ")
	}
	return s
}
