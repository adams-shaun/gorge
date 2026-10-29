package agent

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// Plan is the bot's answer to one native decision, spent substep by substep.
type Plan struct {
	native uint64
	intent decision.Intent
	follow map[string]decision.Intent
	used   []int
}

// NewPlan starts the plan for native decision `native` from the bot's intent.
func NewPlan(native uint64, in decision.Intent) *Plan {
	return &Plan{native: native, intent: in, follow: map[string]decision.Intent{}}
}

// followup asks the bot a folded follow-up once, with the same view.
func (pl *Plan) followup(p xview.Payload, key string, ask func(decision.Decision) decision.Intent) []int {
	in, ok := pl.follow[key]
	if !ok {
		fd, ok := p.Followups[key]
		if !ok {
			return nil
		}
		in = ask(fd)
		pl.follow[key] = in
	}
	return in.Choices
}

// complement lists the options outside chosen, in offered order.
func complement(chosen []int, n int) []int {
	var out []int
	for i := 0; i < n; i++ {
		if !slices.Contains(chosen, i) {
			out = append(out, i)
		}
	}
	return out
}

func unitChosen(p xview.Payload, in decision.Intent, unit int) bool {
	for _, c := range in.Choices {
		if c < len(p.Decision.Options) && int(p.Decision.Options[c].Obj) == unit {
			return true
		}
	}
	return false
}

// match reports whether a candidate's native op agrees with the plan.
func (pl *Plan) match(p xview.Payload, op mapping.NativeOp, ask func(decision.Decision) decision.Intent) bool {
	in := pl.intent
	if in.Payment != nil && op.Payment != nil && reflect.DeepEqual(in.Payment, op.Payment) {
		return true
	}
	if in.Payment != nil && slices.ContainsFunc(op.Pays, func(p *decision.PaymentSelection) bool { return reflect.DeepEqual(in.Payment, p) }) {
		return true
	}
	switch op.Op {
	case "choose":
		var ok bool
		if op.Unit != 0 {
			// Declaration units are posed in the adapter's unit order: membership.
			ok = slices.Contains(in.Choices, op.Option) && !slices.Contains(pl.used, op.Option)
		} else {
			// Everything else follows the bot's answer order, which native asks
			// read (target slots, cards put on the library in order).
			ok = len(pl.used) < len(in.Choices) && in.Choices[len(pl.used)] == op.Option
		}
		if ok && len(op.Followup) > 0 {
			got := pl.followup(p, fmt.Sprint(op.Option), ask)
			ok = len(got) > 0 && got[0] == op.Followup[0]
			if ok && len(op.Followup) > 1 {
				got2 := pl.followup(p, fmt.Sprint(op.Option, "/", op.Followup[0]), ask)
				ok = len(got2) > 0 && got2[0] == op.Followup[1]
			}
		}
		return ok
	case "none":
		return !unitChosen(p, in, int(op.Unit))
	case "cast":
		for _, c := range op.Covers {
			if slices.Contains(in.Choices, c) {
				return true
			}
		}
	case "dest":
		inside := op.Option >= 0 && slices.Contains(in.Choices, op.Option)
		want := op.List == "top" || op.List == "hand"
		if op.Inside != nil {
			want = *op.Inside
		}
		return inside == want
	case "list":
		list := in.Choices
		if op.List == "rest" {
			list = in.Rest
			if len(list) == 0 { // no pile-B order given: gorge's default, the offered order
				list = complement(in.Choices, len(p.Decision.Options))
			}
		} else if key, ok := strings.CutPrefix(op.List, "followup:"); ok {
			list = pl.followup(p, key, ask)
		}
		return op.Position < len(list) && list[op.Position] == op.Option
	case "finish":
		return len(pl.used) >= len(in.Choices)
	case "fixed":
		// An order gorge's engine fixes without asking (cards a reveal
		// sends to the graveyard): the plan has no say, so any one answer
		// is gorge's.
		return true
	}
	return false
}

// Pick returns the candidate matching the plan, with miss "". When no op
// matches, a single candidate is answered as "forced" (an engine-fixed order,
// or pay:false left alone by the unless-cost restriction of controller
// decision 2); otherwise the agent falls back to pay:false, then finish, then
// candidate 0, as "fallback".
func Pick(p xview.Payload, sems []map[string]any, pl *Plan, ask func(decision.Decision) decision.Intent) (int, string) {
	for i, op := range p.Ops {
		if pl.match(p, op, ask) {
			if op.Op == "choose" {
				pl.used = append(pl.used, op.Option)
			}
			return i, ""
		}
	}
	if len(sems) == 1 {
		return 0, "forced"
	}
	for i, s := range sems {
		if s["kind"] == "optional_cost" && s["pay"] == false {
			return i, "fallback"
		}
	}
	for i, s := range sems {
		if s["kind"] == "finish_selection" || s["kind"] == "finish_target_selection" {
			return i, "fallback"
		}
	}
	// A combat declaration the plan cannot express in this substep's offer:
	// the plan named another unit or a pair the engine omitted (an unblockable
	// attacker, say), so declining for THIS unit is the faithful projection.
	// Picking candidate 0 would substitute a block or attack the plan never
	// chose.
	for i, s := range sems {
		switch s["kind"] {
		case "declare_attack":
			if s["defender"] == nil {
				return i, "fallback"
			}
		case "declare_block":
			if s["attacker"] == nil {
				return i, "fallback"
			}
		}
	}
	return 0, "fallback"
}
