package sbv2http

import (
	"encoding/json"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/humanseat"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// humanExtension is x_gorge_human_v1: what the v2 human wire carries beyond
// the SpellBench observation. It is emitted only by this transport.
type humanExtension struct {
	Log        []string       `json:"log"`
	Labels     map[int]string `json:"labels"`
	Plans      []humanPlan    `json:"plans"`
	CanUndo    bool           `json:"can_undo"`
	NativeOnly bool           `json:"native_only"`
	// NameDomain is set only on a choose_name ask whose candidates are a
	// window of the engine's name list; choose_name answers any of Total.
	NameDomain *nameDomain `json:"name_domain,omitempty"`
}

// nameDomain sizes a windowed card-name ask: Total is every option of the
// pending native decision, Offered the candidates in this seat_decision.
type nameDomain struct {
	Total   int `json:"total"`
	Offered int `json:"offered"`
}

// humanPlan is one payment witness for a plan-bearing candidate. CandidateID
// names the v2 candidate whose native action the plan pays.
type humanPlan struct {
	CandidateID int                  `json:"candidate_id"`
	ActionID    string               `json:"action_id"`
	Plan        decision.PaymentPlan `json:"plan"`
}

// extensionBuilder owns the per-seat state both extensions need: xview's
// re-keying tables and the log cursor.
type extensionBuilder struct {
	x       *xview.Extender
	lastSeq uint64
	haveSeq bool
}

func newExtensionBuilder() *extensionBuilder {
	return &extensionBuilder{x: xview.New()}
}

// Extend implements humanseat.ExtendFunc.
func (b *extensionBuilder) Extend(e humanseat.Extension) (protocol.ExtensionMap, error) {
	ext, err := b.x.Extend(e.Env, e.Pose, e.NativeIndex)
	if err != nil {
		return nil, err
	}
	out := make(protocol.ExtensionMap, len(ext)+1)
	for k, v := range ext {
		out[k] = v
	}
	payload := humanExtension{
		Log:     b.logSince(e.Game, e.Events, e.Seat),
		Labels:  candidateLabels(e.Pose),
		Plans:   candidatePlans(e.Seat, e.Pose, b.x),
		CanUndo: e.CanUndo,
	}
	if p := e.Pose; p != nil && p.Native != nil && mapping.Route(p.Native) == "choose/name" && len(p.Native.Options) > len(p.Candidates) {
		payload.NameDomain = &nameDomain{Total: len(p.Native.Options), Offered: len(p.Candidates)}
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	out["x_gorge_human_v1"] = raw
	return out, nil
}

// NativeExtension builds both extensions for a decision v2 cannot pose. There
// is no mapping pose to re-key, so x_gorge_view_v1 carries the native view and
// decision as the seat sees them: the client answers on the native routes and
// uses native object ids, exactly as the native wire would.
func (b *extensionBuilder) NativeExtension(n humanseat.NativeOnly) (protocol.ExtensionMap, error) {
	payload := humanExtension{
		Log:        b.logSince(n.Game, n.Events, n.Seat),
		Labels:     map[int]string{},
		Plans:      []humanPlan{},
		CanUndo:    n.CanUndo,
		NativeOnly: true,
	}
	human, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	render := xview.Payload{Version: 1, View: n.View, Decision: n.Decision}
	render.View.Decision = &render.Decision
	view, err := json.Marshal(render)
	if err != nil {
		return nil, err
	}
	return protocol.ExtensionMap{
		"x_gorge_view_v1":  view,
		"x_gorge_human_v1": human,
	}, nil
}

// logSince describes the events appended since the previous request, redacted
// and described for seat exactly as the host's seat transcript does. A rewind
// truncates the log; the cursor then moves to the new head without replaying
// lines the seat already saw.
func (b *extensionBuilder) logSince(g *state.Game, evs []events.Event, seat state.PlayerID) []string {
	out := []string{}
	if len(evs) == 0 {
		b.lastSeq, b.haveSeq = 0, false
		return out
	}
	head := evs[len(evs)-1].Seq
	if b.haveSeq && head < b.lastSeq {
		b.lastSeq, b.haveSeq = head, true
		return out
	}
	start := 0
	if b.haveSeq {
		for start < len(evs) && evs[start].Seq <= b.lastSeq {
			start++
		}
	}
	for _, ev := range evs[start:] {
		redacted := view.RedactEventFor(g, ev, seat, view.Seat)
		if line := view.Describe(g, redacted); line != "" {
			out = append(out, line)
		}
	}
	b.lastSeq, b.haveSeq = head, true
	return out
}

// candidateLabels maps every v2 candidate to the native option label it
// stands for. Payment-only candidates use the payment action's label; a cast
// candidate uses the cast option it covers.
func candidateLabels(p *mapping.Pose) map[int]string {
	labels := make(map[int]string, len(p.Candidates))
	for i, c := range p.Candidates {
		labels[i] = candidateLabel(p, c.Op)
	}
	return labels
}

func candidateLabel(p *mapping.Pose, op mapping.NativeOp) string {
	if op.Payment != nil {
		for i := range p.Native.PaymentActions {
			a := &p.Native.PaymentActions[i]
			if a.ID == op.Payment.ActionID && a.Label != "" {
				return a.Label
			}
		}
	}
	d := p.Native
	idx := op.Option
	if key, ok := strings.CutPrefix(op.List, "followup:"); ok {
		if f := p.Followups[key]; f != nil {
			d = f
		}
	} else if op.Op == "cast" && len(op.Covers) > 0 {
		idx = op.Covers[0]
	}
	if idx >= 0 && idx < len(d.Options) {
		if label := d.Options[idx].Label; label != "" {
			return label
		}
		return d.Options[idx].Kind
	}
	return ""
}

// candidatePlans lists every offered payment witness for the pose's
// plan-bearing candidates, re-keyed through xview exactly as the render plane
// is. xview exposes only a candidate's first plan through Op.Payment, so this
// is the one place a client can see the alternatives.
func candidatePlans(seat state.PlayerID, p *mapping.Pose, x *xview.Extender) []humanPlan {
	out := []humanPlan{}
	for i, c := range p.Candidates {
		if c.Op.Payment == nil {
			continue
		}
		for ai := range p.Native.PaymentActions {
			a := &p.Native.PaymentActions[ai]
			if a.ID != c.Op.Payment.ActionID {
				continue
			}
			for pi := range a.Plans {
				public := x.PublicPayment(seat, &decision.PaymentSelection{ActionID: a.ID, Plan: a.Plans[pi]})
				if public == nil {
					continue
				}
				out = append(out, humanPlan{CandidateID: i, ActionID: public.ActionID, Plan: public.Plan})
			}
		}
	}
	return out
}
