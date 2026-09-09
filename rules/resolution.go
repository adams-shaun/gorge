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
// the stack exactly as resolveTop's own tail would have.
//
// fx32's defect: an ask NESTED inside a mode (a Charm whose chosen mode is
// itself an asking primitive, and where the Charm — or that mode — carries
// its own SubAbility$ continuation) used to record ONE resume point and
// overwrite it at each nested ask, so the OUTER continuation the enclosing
// effect was still carrying was dropped. The resolution mechanism is now a
// chain of resume points: the innermost point holds the answer the player
// is being asked, and each point links an `outer` continuation that must
// run once everything inside it resolves. effects.Resolve reports each
// suspended loop through effects.Host.SuspendContinuation, and
// resumeResolution links those reports into the chain, so a nested ask
// finishes its own continuation AND then continues outward until the chain
// is empty — every suspended continuation runs, each exactly once.
//
// All resume state is plain value/pointer data (kind/obj plus *cards.SA
// into the shared, immutable compiled corpus, and the linked outer chain is
// rebuilt deterministically from the same stack object and the same
// recorded answers), never a closure, so Engine.Clone carries it like
// cast/choosing and a replay re-derives the same branch from the same
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
// for a CopySpellAbility may-pay, "discard" for a mid-resolution discard
// choice, and "" for a pure outer continuation that carries no answer),
// which stack object's resolution is paused, and the exact sub-ability
// whose effect asked — or, for an outer continuation, the sub-ability to
// resume walking. `outer` is the continuation that must run after this
// point (and everything nested inside it) resolves: the rest of the chain
// the ENCLOSING effect was walking when it launched the nested resolution.
// A nil `outer` means this is the outermost point of the resolution, whose
// completion moves the object off the stack. Plain data, cloned by value
// (the *cards.SA is shared immutable card data, the same class
// Engine.Clone already shares everywhere).
type resumePoint struct {
	kind  string
	obj   state.ObjID
	sa    *cards.SA
	outer *resumePoint
}

// Ask implements effects.Host.Ask (rules' side of the interface, and the
// only place a mid-resolution decision is born). It records the resume
// point — the suspended object is always the top of stack, because a
// decision is pending from this moment until it is answered and Advance's
// loop never runs while one is, so nothing in between can resolve or move —
// and hands the decision to the ordinary ask path. Its `outer` is nil here:
// if a resume re-entry posed this nested ask, the enclosing resumeResolution
// (which owns the continuation of the SA it was re-entering) links it once
// effects.Resolve returns. Always returns true: this engine can always ask.
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

// Suspended implements effects.Host.Suspended: the resolution is suspended
// when a mid-resolution ask set e.resume and the answer has not yet arrived
// to clear it. effects.Resolve checks this after every sub-ability so that a
// suspended ask stops the SubAbility chain instead of running what sits
// beneath it (B1). It is the rules-side half of the pairing with Ask: Ask
// sets e.resume, and handleModes clears it the moment the answer lands, so
// the resume pass re-enters the chain with nothing suspended and walks the
// rest of it exactly once.
func (e *Engine) Suspended() bool { return e.resume != nil }

// SuspendContinuation implements effects.Host.SuspendContinuation: an
// effects.Resolve loop stopped because the resolution suspended at a
// mid-resolution ask and is reporting its own suspension point `sa`, so its
// chain must later resume at sa.Sub. The innermost loop (the one whose `sa`
// IS the pending ask's ResumeSA) is dropped: the pending frame re-enters
// that SA itself, which already walks sa.Sub, so recording it would run it
// twice. Every enclosing loop is recorded, in unwind order — inner loops
// report before outer ones, which is also the order their continuations run
// once the innermost resolves.
func (e *Engine) SuspendContinuation(sa *cards.SA) {
	if e.resume == nil {
		return
	}
	if sa == e.resume.sa {
		return // this loop is the one that asked; its own re-entry walks sa.Sub.
	}
	e.contChain = append(e.contChain, sa)
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
	rp := e.resume
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
// new pending point is linked after this one's own continuation and the
// object stays on the stack; otherwise — once the re-entry and any outer
// continuation it carries have all completed — the fully-resolved object
// goes where resolveTop's own tail would have sent it.
func (e *Engine) resumeResolution(rp *resumePoint, chosen []decision.Option) {
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
			paid := ParseCost(rp.sa.Params["UnlessCost"])
			if !paid.Priceable() {
				// I-5: an unless-cost the payment API cannot price is a hard
				// DECLINE. ParseCost("X") is {Generic:0, X:1}; payMana never
				// charges the unfolded X, so an empty pool "pays" it for free
				// and the counterspell stays inert. An unpriceable cost must
				// counter, never resolve at zero. This is the conservative
				// correct behaviour: a cleared counter is closer to the card
				// than a no-op. The real fix (M4) is cost-grammar work — a
				// value for X from CastInfo/ModeChosen or an SVar folded into
				// Generic via WithX before payment, and a payer that can
				// actually tap-to-pay mid-resolution — and belongs in
				// rules/mana.go's cost grammar, not here. Until then the
				// ask is still posed to the payer (the answer is recorded by
				// ModeChosen) but neither "pay" nor "decline" can save the
				// spell, so every unpriceable unless-pay resolves to the
				// counter. Declining here (rather than suppressing the ask in
				// effects, which cannot import rules' cost type) keeps the
				// decision on the wire for hosts to observe while never
				// letting an empty pool satisfy it.
				ctx.UnlessPay = "decline"
			} else if len(chosen) > 0 && chosen[0].Index == 0 {
				if e.payMana(chosen[0].Player, paid) {
					ctx.UnlessPay = "pay"
				} else {
					ctx.UnlessPay = "decline"
				}
			} else {
				ctx.UnlessPay = "decline"
			}
		case "discard":
			// A mid-resolution discard choice ("Mode$ RevealYouChoose"
			// Thoughtseize/Duress — the CASTER picks out of the target's
			// revealed hand; or "Mode$ TgtChoose" Mind Rot / Faithless
			// Looting — the DISCARDING player picks out of their own hand)
			// was answered. The chosen options carry the object in Obj (the
			// same Obj a cleanup-step discard option carries), so the id
			// list is read straight off them — the one place a
			// mid-resolution answer moves an object by identity rather than
			// an SVar name, which is why Ctx carries a Discard []ObjID
			// rather than a Modes []string.
			// Counter's UnlessCost$ and RearrangeTopOfLibrary (Ponder) can
			// both reuse this same answer-shape and resume retrofitted onto
			// their own asking primitive — see task-dc1-brief scope.
			ids := make([]state.ObjID, 0, len(chosen))
			for _, o := range chosen {
				if o.Obj != 0 {
					ids = append(ids, o.Obj)
				}
			}
			ctx.Discard = ids
		default: // "modes", and "" (a pure outer continuation with no answer)
			ctx.Modes = modeChoiceNames(rp.sa, chosen)
		}
		src := rp.obj
		if o.Ability != nil {
			src = o.Source
		}
		e.damaging = src
		// A fresh re-entry: reset the enclosing-loop continuation reports the
		// loops of THIS effects.Resolve call will accumulate. This reset is
		// safe even though resumeResolution recurses through rp.outer below:
		// an enclosing frame can only reach that recursion after its own
		// effects.Resolve returned with NO nested ask (e.resume == nil, else
		// the nested-ask branch above consumes contChain into e.resume.outer
		// and returns without recursing), at which point its contChain was
		// never needed again — so a deeper frame's reset discards only
		// reports no live frame still needs (measured: the full rules suite
		// runs no path where a recursive reset clobbers a needed report).
		e.contChain = e.contChain[:0]
		effects.Resolve(e, ctx, rp.sa)
		e.damaging = 0
		if e.resume != nil {
			// The re-entry posed a nested mid-resolution ask. The new
			// pending point (e.resume) has no outer yet: it must, once its
			// own answer is applied, continue at the rest of THIS re-entry's
			// chain (the loops effects.Resolve reported via
			// SuspendContinuation this pass) and then at rp.outer — the
			// continuation the frame we were re-entering was itself carrying.
			// Linking them now means the nested ask, when answered, resumes
			// every suspended continuation rather than dropping the outer
			// ones (fx32).
			e.resume.outer = e.buildContinuationChain(e.contChain, rp.obj, rp.outer)
			return
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
	if rp.outer != nil {
		// No nested ask this pass and the frame itself completed: continue
		// outward through the runner-up continuations this frame carried.
		e.resumeResolution(rp.outer, nil)
		return
	}
	e.finishResumption(rp.obj)
}

// buildContinuationChain turns the enclosing-loop suspension points reported
// for one re-entry into a linked run of pure-continuation frames (kind ""),
// in report order — inner continuations first, outer last — and chains the
// given `tail` (the outer continuation of the frame being re-entered) onto
// the end. The resulting head is the what the new pending point must run
// after its own answer, or nil if there is nothing left to continue.
func (e *Engine) buildContinuationChain(sas []*cards.SA, obj state.ObjID, tail *resumePoint) *resumePoint {
	var head, prev *resumePoint
	for _, sa := range sas {
		f := &resumePoint{obj: obj, sa: sa.Sub}
		if head == nil {
			head = f
		} else {
			prev.outer = f
		}
		prev = f
	}
	if prev != nil {
		prev.outer = tail
	} else {
		head = tail
	}
	return head
}

// finishResumption is the shared tail of resolveTop and resumeResolution: a
// fully resolved spell leaves the stack for the battlefield when it is a
// permanent (CR 608.3), otherwise to its resting zone (exile for a
// Flashback cast or a copy, the graveyard for the rest).
// ensureLeftTheStack then guards the same replacement-discarded-the-move
// corner both callers already guard, so a resolution can never leave its
// object resolving forever.
func (e *Engine) finishResumption(id state.ObjID) {
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZStack {
		return // the continuation already moved it (or it ceased to exist).
	}
	if e.G.Obj(id).Ability != nil {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZExile})
		e.ensureLeftTheStack(id, state.ZExile, "a replacement fully discarded this resolved "+
			"ability's own move off the stack without relocating it anywhere; sent to exile "+
			"instead of re-resolving forever")
		return
	}
	e.moveResolvedOffStack(e.G.Obj(id))
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
