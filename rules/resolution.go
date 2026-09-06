// The mid-resolution ask: an effect that needs a player's decision in the
// middle of resolving the top of the stack (effCharm's modal pick, or
// effCopySpellAbility's UnlessCost$ may-pay) poses it through
// effects.Host.Ask, and this file owns what suspends and what resumes.
//
// Suspension is a stack object staying put: resolveTop (rules/stack.go)
// runs the object's effects and, when e.resume is set after the pass,
// leaves the object on the stack and returns with a decision pending, so
// nothing before the ask gets a second chance to run and nothing moves
// until the answer arrives. Resumption re-runs the suspended sub-ability —
// the exact one that asked, never the chain prefix before it — with the
// answer attached to the Ctx, and then moves the fully-resolved object off
// the stack exactly as resolveTop's own tail would have. All resume state
// is plain value/pointer data (kind/obj plus a *cards.SA into the shared,
// immutable compiled corpus), never a closure, so Engine.Clone carries it
// like cast/choosing and a replay re-derives the same branch from the same
// recorded intent.
package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// resumePoint is one suspended resolution: which continuation the pending
// decision's answer resumes ("modes" for a Charm modal pick, "unless_pay"
// for a CopySpellAbility may-pay), which stack object's resolution is
// paused, and the exact sub-ability whose effect asked — the point where
// the chain re-enters, so the sub-abilities before the ask are never
// re-run. Plain data, cloned by value (the *cards.SA is shared immutable
// card data, the same class Engine.Clone already shares everywhere).
type resumePoint struct {
	kind string
	obj  state.ObjID
	sa   *cards.SA
}

// Ask implements effects.Host.Ask (rules' side of the interface, and the
// only place a mid-resolution decision is born). It records the resume
// point — the suspended object is always the top of stack, because a
// decision is pending from this moment until it is answered and Advance's
// loop never runs while one is, so nothing in between can resolve or move —
// and hands the decision to the ordinary ask path. Always returns true:
// this engine can always ask.
func (e *Engine) Ask(d *decision.Decision) bool {
	obj := state.ObjID(0)
	if n := len(e.G.Stack); n > 0 {
		obj = e.G.Stack[n-1]
	}
	kind := d.ResumeKind
	if kind == "" {
		kind = "modes"
	}
	e.ask(d)
	e.resume = &resumePoint{kind: kind, obj: obj, sa: d.ResumeSA}
	return true
}

// handleModes applies an answered KModes decision — the engine's one KModes
// handler, serving both the Charm modal pick ("modes") and the UnlessCost$
// may-pay ("unless_pay"), which the decision's ResumeKind tags. It records
// the choice as a ModeChosen event (the log-carried answer a replay
// re-derives) and hands the chosen options to the suspended resolution's
// continuation. A KModes answer with no suspended resolution is only
// reachable from a hand-built decision, never from a real ask; it degrades
// with a Note rather than panicking, the same totality stance every
// handler takes.
func (e *Engine) handleModes(d *decision.Decision, in decision.Intent) {
	if e.resume == nil {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "modes answered with no resolution suspended"})
		return
	}
	rp := *e.resume
	e.resume = nil
	chosen := d.Chosen(in)
	labels := make([]string, 0, len(chosen))
	for _, o := range chosen {
		labels = append(labels, o.Label)
	}
	e.emit(events.Event{Kind: events.ModeChosen, Obj: rp.obj, Player: in.Player,
		Text: strings.Join(labels, ",")})
	e.resumeResolution(rp, chosen)
}

// resumeResolution re-enters a suspended resolution with its answer. It
// rebuilds the same Ctx resolveTop built for the object on its first pass
// (Source/Controller/Targets/Remembered and the SVar table are all
// re-derivable from the stack object, which has not moved), attaches the
// answer, and re-runs the suspended sub-ability — effects.Resolve walks
// from it through the rest of the chain, which is precisely the
// continuation that had not run yet. If that continuation asks again the
// object suspends once more (e.resume is set again and the object stays on
// the stack); otherwise the fully-resolved object goes where resolveTop's
// own tail would have sent it.
func (e *Engine) resumeResolution(rp resumePoint, chosen []decision.Option) {
	o := e.G.Obj(rp.obj)
	if o == nil || o.Zone != state.ZStack {
		// The suspended object left the stack while the decision was
		// outstanding. Nothing but the answer can un-freeze the engine, so
		// this is unreachable in a well-formed match; it degrades to a
		// no-op rather than panicking, the same totality stance as every
		// other resolution exit.
		return
	}
	ctx := &effects.Ctx{Source: rp.obj, Controller: o.Controller, Targets: o.Targets}
	var svars map[string]string
	if o.Ability != nil {
		// A triggered or activated ability: mirror resolveTop's ability
		// branch — Source is the source permanent, Remembered carries what
		// the trigger captured, and the SVar table comes from that
		// permanent's face.
		ctx.Source = o.Source
		ctx.Remembered = o.Remembered
		if src := e.G.Obj(o.Source); src != nil {
			if sf := src.Face(); sf != nil {
				svars = sf.SVars
			}
		}
	} else if f := o.Face(); f != nil {
		svars = f.SVars
	}
	effects.SetSVars(ctx, svars)
	if rp.sa != nil {
		switch rp.kind {
		case "unless_pay":
			// The payer agreed to pay (option 0 is "Pay … — make a copy") or
			// not. Payment happens HERE, in rules, because payMana owns the
			// cost grammar and emits the ManaAdd events — so a replay
			// re-derives the identical payment. An answer to pay from a pool
			// that cannot cover it is a decline: the copy is not made,
			// deterministically.
			if len(chosen) > 0 && chosen[0].Index == 0 {
				if e.payMana(chosen[0].Player, ParseCost(rp.sa.Params["UnlessCost"])) {
					ctx.UnlessPay = "pay"
				} else {
					ctx.UnlessPay = "decline"
				}
			} else {
				ctx.UnlessPay = "decline"
			}
		default: // "modes"
			ctx.Modes = modeChoiceNames(rp.sa, chosen)
		}
		src := rp.obj
		if o.Ability != nil {
			src = o.Source
		}
		e.damaging = src
		effects.Resolve(e, ctx, rp.sa)
		e.damaging = 0
		if e.resume != nil {
			return // the continuation asked again: still suspended, still on the stack.
		}
	} else {
		// A resume with no sub-ability recorded: only reachable from a
		// hand-built Ask (every real asking primitive sets ResumeSA). The
		// resolution still finishes — the object leaves the stack with no
		// effect, the same degrade-to-nothing stance as an unrecognised
		// choice, rather than stalling the match forever.
		e.emit(events.Event{Kind: events.Note, Obj: rp.obj,
			Text: "mid-resolution answer resumed with no sub-ability recorded"})
	}
	if o := e.G.Obj(rp.obj); o == nil || o.Zone != state.ZStack {
		return // the continuation already moved it (or it ceased to exist).
	}
	if e.G.Obj(rp.obj).Ability != nil {
		e.emit(events.Event{Kind: events.MoveZone, Obj: rp.obj, From: state.ZStack, To: state.ZExile})
		e.ensureLeftTheStack(rp.obj, state.ZExile, "a replacement fully discarded this resolved "+
			"ability's own move off the stack without relocating it anywhere; sent to exile "+
			"instead of re-resolving forever")
		return
	}
	e.moveResolvedOffStack(e.G.Obj(rp.obj))
}

// modeChoiceNames maps the chosen modal options back to the SVar names of
// the Choices$ sub-abilities they pick, in the order chosen — the answer
// effCharm's re-entry reads (Ctx.Modes). The option list is built by
// effCharm in Choices$ order, so index i names choices[i]; a selected index
// out of range is dropped, the same degrade-to-nothing stance every
// resolution path takes.
func modeChoiceNames(sa *cards.SA, chosen []decision.Option) []string {
	if sa == nil {
		return nil
	}
	choices := strings.Split(sa.Params["Choices"], ",")
	for i := range choices {
		choices[i] = strings.TrimSpace(choices[i])
	}
	names := make([]string, 0, len(chosen))
	for _, o := range chosen {
		if o.Index >= 0 && o.Index < len(choices) {
			names = append(names, choices[o.Index])
		}
	}
	return names
}

// moveResolvedOffStack is the shared tail of resolveTop and
// resumeResolution: a fully resolved spell leaves the stack for the
// battlefield when it is a permanent (CR 608.3), otherwise to its resting
// zone (exile for a Flashback cast or a copy, the graveyard for the rest).
// ensureLeftTheStack then guards the same replacement-discarded-the-move
// corner both callers already guard, so a resolution can never leave its
// object resolving forever.
func (e *Engine) moveResolvedOffStack(o *state.Object) {
	id := o.ID
	if f := o.Face(); f != nil && f.IsPermanent() {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZBattlefield})
		e.ensureLeftTheStack(id, spellRestZone(o), "an ETB replacement fully replaced this "+
			"permanent's entry to the battlefield without moving it anywhere; sent to its "+
			"resting zone instead of re-resolving forever")
		return
	}
	rest := spellRestZone(o)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: rest})
	e.ensureLeftTheStack(id, rest, "a replacement fully discarded this resolved "+
		"spell's own move off the stack without relocating it anywhere; sent to its "+
		"resting zone instead of re-resolving forever")
}
