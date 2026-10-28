package neutral

import (
	"slices"

	"github.com/adams-shaun/gorge/bots/spellbench/payexec"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// Payment on the neutral world.
//
// gorge's bot pays a spell by choosing a PaymentAction of the priority
// decision (Decision.PaymentActions): it ranks the casts the planner can pay
// and answers with the plan it offers. The neutral world has no engine to ask,
// so it rebuilds one (shadow) from the observation, asks gorge's own planner
// (Engine.EnsurePaymentActions, the call gamecfg.Game makes) and hands the
// bot the actions it would have had natively. The bot's answer is a plan; how
// that reaches the host depends on the host's mana surface:
//
//   - engine autopay (no activate_mana_ability candidate): the host pays by its
//     own rule, which is gorge's Plans[0] when gorge hosts. The plan tags the
//     action's cast_spell candidate (NativeOp.Payment) so the agent picks it.
//   - manual mana (activate_mana_ability candidates): the plan is lowered onto
//     the candidates by bots/spellbench/payexec, the lowering the SpellBench
//     engines already use. Every action's first step tags the candidate that
//     takes it (NativeOp.Pays); once the bot chose, the Session remembers the
//     Execution and tags one candidate per later priority decision (the next
//     source, finally the cast) under the same native index, so the agent
//     keeps the bot's plan. A plan payexec cannot lower falls back to the first
//     mana candidate and is counted in Counts["pay_fallback"].

// Counts keys of payment events (Session.Counts).
const (
	countFallback = "pay_fallback" // a plan that could not be lowered: first mana candidate taken
	countAbort    = "pay_abort"    // a lowering that diverged mid-way: replanned
	countNoShadow = "pay_noshadow" // no planner board could be rebuilt: the bot got no plans
)

// planned is one payment action offered on a native priority decision.
type planned struct {
	action  decision.PaymentAction
	sel     *decision.PaymentSelection
	x       *payexec.Execution // manual host: the lowering, already one step in; nil when it cannot be lowered
	cand    int                // the candidate that takes its first step (autopay: its cast)
	covered bool               // autopay: a cast candidate was tied to this plan
	// poolOnly: the floating pool pays, so gorge keeps the plain cast option
	// and the plan only tags it (the plan taps nothing).
	poolOnly bool
	base     *int // poolOnly: the plain cast option's index
}

// pending is a lowering in progress across priority decisions.
type pending struct {
	index   uint64 // the native index its substeps share
	x       *payexec.Execution
	sel     *decision.PaymentSelection
	actions []decision.PaymentAction
}

// candidates sorts a priority decision's candidates by what payment reads.
type candidates struct {
	mana []int                 // activate_mana_ability, in order
	srcs map[state.ObjID][]int // activate_mana_ability by source
	cast map[castKey]int       // cast_spell by source and mode
	pass int                   // the pass candidate, -1 for none
	// handLike holds the objects the host offers to cast from outside the
	// hand, other than by flashback: the planner sees them as hand cards.
	handLike map[string]bool
}

type castKey struct {
	obj      state.ObjID
	flashbck bool
}

func (s *Session) candidates(sd *protocol.SeatDecision) *candidates {
	c := &candidates{srcs: map[state.ObjID][]int{}, cast: map[castKey]int{}, pass: -1, handLike: map[string]bool{}}
	for i, cd := range sd.Candidates {
		sem := cd.Semantic
		switch sem.Kind {
		case "pass":
			if c.pass < 0 {
				c.pass = i
			}
		case "activate_mana_ability":
			if src, err := ref(sem, "source"); err == nil {
				id := s.B.IDs.Of(src.ObjectID)
				c.mana = append(c.mana, i)
				c.srcs[id] = append(c.srcs[id], i)
			}
		case "cast_spell":
			src, err := ref(sem, "source")
			if err != nil {
				continue
			}
			k := castKey{obj: s.B.IDs.Of(src.ObjectID), flashbck: str(sem, "method") == "flashback"}
			if m := str(sem, "method"); m != "" && m != "normal" && m != "flashback" {
				continue
			}
			if (src.Zone == "exile" || src.Zone == "graveyard") && !k.flashbck {
				c.handLike[src.ObjectID] = true
			}
			if _, dup := c.cast[k]; !dup {
				c.cast[k] = i
			}
		}
	}
	return c
}

// prepared is what payment contributes to one priority decision, worked out
// before the options are built because on an autopay host a planned cast has
// no option of its own (it is the action's plan-only cast, as natively).
type prepared struct {
	c        *candidates
	actions  []decision.PaymentAction
	floats   []decision.Option
	byCast   map[castKey]*planned // autopay: the action a cast candidate pays
	plans    []*planned           // every offered action's lowering, in action order
	autoPay  bool                 // the host pays plans itself (engine_autopay)
	planOnly bool                 // autopay: a planned cast is the action's plan-only cast
	resume   int                  // the candidate a pending lowering takes now, -1 for none
	resumed  payexec.Status
	// resumedCast reports that the pending lowering takes a cast candidate
	// now. A manual host decides the cast's method at that window (it has no
	// plan to apply), so the plan is not put back in the decision's actions.
	resumedCast bool
}

// prepay plans the payments of a priority decision: it continues the pending
// lowering, or asks gorge's planner on a rebuilt board for fresh actions.
func (s *Session) prepay(sd *protocol.SeatDecision, v *view.View) *prepared {
	c := s.candidates(sd)
	pre := &prepared{c: c, resume: -1, byCast: map[castKey]*planned{}, autoPay: s.autoPay}
	if len(c.mana) == 0 && len(c.cast) == 0 {
		return pre
	}
	if s.pend != nil {
		j, st := s.step(s.pend.x, c, v)
		if st == payexec.InProgress || st == payexec.Done {
			pre.resume, pre.resumed = j, st
			pre.resumedCast = j >= 0 && j < len(sd.Candidates) && sd.Candidates[j].Semantic.Kind == "cast_spell"
			return pre
		}
		s.pend = nil
		s.Counts[countAbort]++
	}
	// Only an engine_autopay host pays a plan itself; a manual host pays the
	// plan by the taps it offers, so with no tap to lower onto there is
	// nothing to plan. (A no-mana window on a manual host is not autopay: the
	// pool already pays, and the cast is offered as itself.)
	pre.planOnly = pre.autoPay && len(c.mana) == 0
	if !pre.autoPay && len(c.mana) == 0 {
		return pre
	}
	var actions []decision.PaymentAction
	actions, pre.floats = s.plans(sd, c)
	for _, a := range actions {
		pl := &planned{action: a, sel: &decision.PaymentSelection{ActionID: a.ID, Plan: decision.ClonePaymentPlan(a.Plans[0])}, cand: -1,
			poolOnly: len(a.Plans[0].Activations) == 0}
		if pre.planOnly {
			// Engine autopay: the cast candidate carries the plan.
			j, ok := c.cast[castKey{obj: a.Cast.Object, flashbck: a.Cast.Origin == decision.OriginFlashback}]
			if !ok {
				continue
			}
			pl.cand = j
			pre.byCast[castKey{obj: a.Cast.Object, flashbck: a.Cast.Origin == decision.OriginFlashback}] = pl
		} else {
			pl.cand = c.mana[0] // payexec cannot lower it: the first candidate
			if !sacrifices(a.Plans[0]) {
				x := payexec.StartPlay(s.seat, playOf(a), a.Plans[0], payexec.PoolFromView(*v, s.seat))
				if cj, st := s.step(x, c, v); st == payexec.InProgress || st == payexec.Done {
					pl.x, pl.cand = x, cj
				}
			}
		}
		pre.plans = append(pre.plans, pl)
	}
	return pre
}

// cover reports the action whose plan replaces the plain cast option of a
// cast candidate on an autopay host, nil when none does.
func (pre *prepared) cover(o decision.Option) *planned {
	if !pre.planOnly {
		return nil
	}
	if o.Mode != "" && o.Mode != "flashback" || o.AltCostIndex != 0 {
		return nil
	}
	pl := pre.byCast[castKey{obj: o.Obj, flashbck: o.Mode == "flashback"}]
	if pl != nil {
		pl.covered = true
	}
	return pl
}

// finish gives the decision n its payment actions and tags the ops they take.
func (s *Session) finish(pre *prepared, n *native, ops []mapping.NativeOp) {
	tag := func(j int, sel *decision.PaymentSelection) {
		if ops[j].Op == noOp.Op {
			ops[j] = mapping.NativeOp{Op: "pay", Option: -1}
		}
		ops[j].Pays = append(ops[j].Pays, sel)
	}
	if p := s.pend; p != nil && pre.resume >= 0 {
		// A manual host has no plan to submit: at the cast step the seat
		// picks the cast (and its method) as it would without a plan, while
		// the lowering's taps have already been taken. Leaving the pending
		// native index and its actions in place would replay the plan's
		// payment intent there and shadow another cast method of the same
		// card (an adventure beside its creature), so the cast is a fresh
		// decision for the agent to ask.
		if pre.autoPay || !pre.resumedCast {
			tag(pre.resume, p.sel)
			n.index = p.index
			n.d.PaymentActions = p.actions
			n.pays = map[string]*planned{p.sel.ActionID: {sel: p.sel, x: p.x}}
		}
		if pre.resumed == payexec.Done || pre.resumedCast {
			s.pend = nil
		}
		return
	}
	// gorge's decision offers a tap of every mana source as an option of its
	// own, and the bot floats mana by hand when no plan serves it. A manual
	// host answers that with the candidate; an autopay host hides the taps
	// (floating there would strand the bot), so it gets none.
	if len(pre.c.mana) > 0 {
		for _, o := range pre.floats {
			o.Index = len(n.d.Options)
			n.d.Options = append(n.d.Options, o)
			for _, j := range pre.c.srcs[o.Obj] {
				if ops[j].Op == noOp.Op {
					ops[j] = mapping.NativeOp{Op: "choose", Option: o.Index}
				}
			}
		}
	}
	n.pays = map[string]*planned{}
	for _, pl := range pre.plans {
		if pre.planOnly && !pl.covered {
			continue
		}
		if !pre.planOnly {
			tag(pl.cand, pl.sel)
		}
		act := pl.action
		if pl.base != nil && pre.planOnly {
			act.BaseOptionIndex = pl.base
		}
		n.pays[pl.action.ID] = pl
		n.d.PaymentActions = append(n.d.PaymentActions, act)
	}
}

// PickedPayment tells the session which payment the bot's plan was when the
// agent took a candidate tagged for it. A lowerable plan becomes the pending
// one the next priority decisions continue.
func (s *Session) PickedPayment(sel *decision.PaymentSelection) {
	if s.cur == nil || sel == nil {
		return
	}
	pl := s.cur.pays[sel.ActionID]
	if pl == nil {
		return
	}
	if pl.x == nil {
		s.Counts[countFallback]++
		return
	}
	if pl.x.Status() == payexec.Done {
		s.pend = nil
		return
	}
	s.pend = &pending{index: s.cur.index, x: pl.x, sel: pl.sel, actions: slices.Clone(s.cur.d.PaymentActions)}
}

// plans runs gorge's planner on the rebuilt board and returns the actions
// the host can carry out, in the builder's object ids.
func (s *Session) plans(sd *protocol.SeatDecision, c *candidates) ([]decision.PaymentAction, []decision.Option) {
	sh, err := s.B.shadow(&sd.Observation, c.handLike)
	if err != nil {
		s.Counts[countNoShadow]++
		return nil, nil
	}
	if sh == nil {
		return nil, nil
	}
	var floats []decision.Option
	for _, o := range sh.e.Pending().Options {
		if o.Kind != "activate" {
			continue
		}
		if id, ok := sh.ids[o.Obj]; ok {
			o.Obj = id
			floats = append(floats, o)
		}
	}
	var out []decision.PaymentAction
	for _, a := range sh.e.EnsurePaymentActions() {
		if len(a.Plans) == 0 {
			continue
		}
		t := decision.ClonePaymentAction(a)
		t.BaseOptionIndex = nil
		var ok bool
		if t.Cast.Object, ok = sh.ids[a.Cast.Object]; !ok {
			continue
		}
		t.Plans = t.Plans[:0]
		for i, p := range a.Plans {
			p = decision.ClonePaymentPlan(p)
			mapped := true
			for k := range p.Activations {
				act := &p.Activations[k]
				if act.Source, mapped = sh.ids[act.Source]; !mapped {
					break
				}
				if _, offered := c.srcs[act.Source]; len(c.mana) > 0 && !offered {
					mapped = false // the host offers no tap of that source
					break
				}
				act.SourceZoneSeq = 0
			}
			if !mapped && i == 0 {
				break
			}
			if mapped {
				t.Plans = append(t.Plans, p)
			}
		}
		if len(t.Plans) > 0 {
			out = append(out, t)
		}
	}
	return out, floats
}

func playOf(a decision.PaymentAction) payexec.Play {
	p := payexec.Play{Kind: "cast", Obj: a.Cast.Object}
	if a.Cast.Origin == decision.OriginFlashback {
		p.Mode = "flashback"
	}
	return p
}

// sacrifices reports whether a plan has a source sacrificed for its mana,
// which payexec runs last and the neutral world does not lower.
func sacrifices(p decision.PaymentPlan) bool {
	for _, a := range p.Activations {
		if a.Consequence != nil && a.Consequence.Sacrifice {
			return true
		}
	}
	return false
}

// step runs the lowering one step against this decision: payexec is given
// the priority decision the candidates stand for (one activate option per
// source, the plan's cast, pass) and answers with the option it takes, which
// is mapped back to a candidate. A source with several candidates (a colour
// to name) gets payexec's mana ask too.
func (s *Session) step(x *payexec.Execution, c *candidates, v *view.View) (int, payexec.Status) {
	d := decision.Decision{Kind: decision.KPriority, Player: s.seat, Min: 1, Max: 1}
	opt := func(o decision.Option) {
		o.Index = len(d.Options)
		d.Options = append(d.Options, o)
	}
	srcs := make([]state.ObjID, 0, len(c.srcs))
	for id := range c.srcs {
		srcs = append(srcs, id)
	}
	slices.SortFunc(srcs, func(a, b state.ObjID) int { return c.srcs[a][0] - c.srcs[b][0] })
	for _, id := range srcs {
		opt(decision.Option{Kind: "activate", Obj: id})
	}
	castObjs := make([]castKey, 0, len(c.cast))
	for k := range c.cast {
		castObjs = append(castObjs, k)
	}
	slices.SortFunc(castObjs, func(a, b castKey) int { return c.cast[a] - c.cast[b] })
	for _, k := range castObjs {
		o := decision.Option{Kind: "cast", Obj: k.obj}
		if k.flashbck {
			o.Mode = "flashback"
		}
		opt(o)
	}
	if c.pass >= 0 {
		opt(decision.Option{Kind: "pass"})
	}
	surf := payexec.Surface{Pool: payexec.PoolFromView(*v, s.seat), Stack: len(v.Stack)}
	in, st := x.StepOn(&d, surf)
	if st != payexec.InProgress && st != payexec.Done || len(in.Choices) != 1 {
		return -1, payexec.Aborted
	}
	o := d.Options[in.Choices[0]]
	switch o.Kind {
	case "pass":
		return c.pass, st
	case "cast":
		return c.cast[castKey{obj: o.Obj, flashbck: o.Mode == "flashback"}], st
	}
	js := c.srcs[o.Obj]
	if len(js) == 1 {
		return js[0], st
	}
	return s.colour(x, js, o.Obj, surf), st
}

// colour answers payexec's mana ask for a source whose candidates differ in
// the mana they name.
func (s *Session) colour(x *payexec.Execution, js []int, src state.ObjID, surf payexec.Surface) int {
	ask := decision.Decision{Kind: decision.KChoose, Player: s.seat, Min: 1, Max: 1}
	for k, j := range js {
		sym := ""
		if s.last != nil {
			sym = str(s.last.Candidates[j].Semantic, "mana_choice")
		}
		ask.Options = append(ask.Options, decision.Option{Index: k, Kind: "mana", Obj: src, ManaSymbol: sym, Label: "Add " + sym})
	}
	in, st := x.StepOn(&ask, surf)
	if st != payexec.InProgress || len(in.Choices) != 1 {
		return js[0]
	}
	return js[in.Choices[0]]
}
