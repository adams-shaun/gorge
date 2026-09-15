// The mid-resolution ask: an effect that needs a player's decision in the
// middle of resolving the top of the stack (a nested Charm pick, or
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
	"strconv"
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
// choice, "search" for a hidden-library KChoose, "dig" for a Dig
// look-and-take pick, and "" for a pure outer
// continuation that carries no answer),
// which stack object's resolution is paused, and the exact sub-ability
// whose effect asked — or, for an outer continuation, the sub-ability to
// resume walking. `outer` is the continuation that must run after this
// point (and everything nested inside it) resolves: the rest of the chain
// the ENCLOSING effect was walking when it launched the nested resolution.
// A nil `outer` means this is the outermost point of the resolution, whose
// completion moves the object off the stack. Plain data, cloned by value
// (the *cards.SA is shared immutable card data, the same class
// Engine.Clone already shares everywhere).
//
// `replacement` records whether the suspended resolution is running inside
// a replacement effect's ReplaceWith$ body (fx44). applyReplacements sets
// e.applyingReplacement while it resolves that body and resets it to false
// when the body returns — but a body that SUSPENDS at an ask returns before
// the answer arrives, so the flag is lost across the suspension. The resumed
// body then emits its own completion move with applyingReplacement false,
// and that move is re-intercepted by the SAME replacement it is the product
// of. Capturing the flag at ask time and restoring it for the whole resumed
// resolution closes that loop (see resumeResolution); it is the answer to
// the question "does applyingReplacement survive the suspension".
type resumePoint struct {
	kind        string
	obj         state.ObjID
	sa          *cards.SA
	outer       *resumePoint
	replacement bool
	// replaced is the object the replaced event was about (Ctx.Replaced =
	// ev.Obj), captured at ask time when the ask is posed from inside a
	// replacement body. The resume rebuilds Ctx.Replaced (and Remembered =
	// [that object]) from it, so a ReplaceWith$ body whose completion move
	// is gated on Defined$ ReplacedCard / SVar:X Remembered$Amount finds its
	// subject after the suspension (fx44, Mox Diamond). Zero for an ordinary
	// (non-replacement) ask.
	replaced state.ObjID
	// action is the replaced event's action marker (Engine.replAction),
	// captured with replaced so a body that suspends before its move still
	// labels that move a sacrifice or discard on the resume.
	action string
	before *triggerSnapshot // immutable look-back if a batch replacement suspends
	// target is Dig's index into its deterministic Defined$ target list. It
	// keeps a resumed answer attached to the library that actually asked.
	target      int
	choices     []state.Target
	chosenValid bool
	remembered  []state.Target
	// replSource is the host of the replacement whose body asked (the
	// ReplaceWith$ body's own Ctx.Source); zero outside a replacement.
	replSource state.ObjID
	// loopBound frames resume inside a RepeatEach iteration (or at the
	// RepeatEach itself, kind "repeat"): Ctx.Remembered is rebuilt from
	// loopRemembered rather than from the stack object, because the loop
	// binds its current subject there and the stack object never saw it.
	loopBound      bool
	loopRemembered []state.Target
	// repeat is a kind "repeat" frame's loop cursor.
	repeat *repeatCursor
}

// repeatCursor is the loop position a kind "repeat" frame re-enters with.
// last is the final Remembered of the iteration that completed just before
// the frame runs, handed over by that iteration's own frame.
type repeatCursor struct {
	subjects []state.Target
	next     int
	last     []state.Target
	hasLast  bool
}

// contFrame is one enclosing-loop suspension reported during a resolution
// pass: a plain Resolve loop (resume at sa.Sub) or a RepeatEach loop
// (repeat != nil; re-enter sa itself at the cursor).
type contFrame struct {
	sa          *cards.SA
	repeat      *repeatCursor
	bound       bool
	remembered  []state.Target
	choices     []state.Target
	chosenValid bool
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
	// Capture whether the ask is being posed from inside a replacement
	// effect's ReplaceWith$ body (fx44). e.applyingReplacement is true for
	// the whole of that body's resolution, so an ask posed from within it
	// must resume still under the flag — see the resumePoint field's
	// comment and resumeResolution's restore of it.
	// A replacement body resumes from the object whose resolution it
	// interrupted -- normally still on top of the stack (a sorcery that
	// reanimates Mox Diamond), so the rest of that spell's chain and its
	// completion still run after the answer. The one exception is a permanent
	// spell whose own Updated entry replacement asks (Sower of Discord): the
	// move has already taken the resolving object off the stack, so the top
	// of the stack is some unrelated object and the resume must rebuild from
	// the entering permanent itself. replSource keeps the replacement's host
	// for the resumed body's own Source either way.
	var replSource state.ObjID
	if e.applyingReplacement && d.Source != 0 {
		replSource = d.Source
		if d.Source == e.resolvingObj {
			if so := e.G.Obj(d.Source); so != nil && so.Zone != state.ZStack {
				obj = d.Source
			}
		}
	}
	e.resume = &resumePoint{kind: kind, obj: obj, sa: d.ResumeSA, replSource: replSource,
		replacement: e.applyingReplacement, replaced: e.replReplaced, action: e.replAction,
		before: e.triggerBefore, target: d.ResumeTarget, choices: append([]state.Target(nil), d.ResumeChoices...),
		chosenValid: d.ResumeChosenValid, remembered: append([]state.Target(nil), d.ResumeRemembered...)}
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
	if sa == e.repeatReported {
		// The RepeatEach just recorded its own loop frame, whose re-entry runs
		// the remaining iterations and then walks sa.Sub itself.
		e.repeatReported = nil
		return
	}
	e.contChain = append(e.contChain, contFrame{sa: sa})
}

// SuspendRepeat implements effects.Host.SuspendRepeat. Everything recorded so
// far in this pass -- the pending ask and the continuation frames of loops
// nested inside the iteration -- resumes inside that iteration, so each is
// bound to the iteration's Remembered unless a deeper loop already bound it.
// The loop's own frame follows them, bound to the RepeatEach's Remembered.
func (e *Engine) SuspendRepeat(s effects.RepeatSuspension) {
	if e.resume == nil {
		return
	}
	body := append([]state.Target(nil), s.Body...)
	if !e.resume.loopBound {
		e.resume.loopBound, e.resume.loopRemembered = true, body
	}
	for i := range e.contChain {
		if !e.contChain[i].bound {
			e.contChain[i].bound, e.contChain[i].remembered = true, body
		}
	}
	e.contChain = append(e.contChain, contFrame{
		sa:          s.SA,
		repeat:      &repeatCursor{subjects: append([]state.Target(nil), s.Subjects...), next: s.Next},
		bound:       true,
		remembered:  append([]state.Target(nil), s.Outer...),
		choices:     append([]state.Target(nil), s.Chosen...),
		chosenValid: s.ChosenValid,
	})
	e.repeatReported = s.SA
}

// handleModes applies an answered KModes decision. ResumeKind and the trigger
// drain flag distinguish three lifetimes: a modal spell's CR 601.2b cast
// proposal, a modal trigger's CR 603.3c placement, and an effect suspended in
// mid-resolution (including unless-pay). Every branch records ModeChosen; the
// first two also cache the chosen SVar names on the stack object so resolution
// executes the announcement without asking again.
func (e *Engine) handleModes(d *decision.Decision, in decision.Intent) {
	// CR 601.2b cast branch: the spell is already provisionally on the stack,
	// but no targets have been selected and no cost has been paid. Record the
	// answer on that spell, then resume the cast transaction at target choice.
	if d.ResumeKind == "cast_modes" {
		pc := e.cast
		if pc == nil || pc.ability >= 0 || pc.stackObj == 0 {
			e.emit(events.Event{Kind: events.Note, Player: in.Player,
				Text: "cast modes answered with no spell proposal pending"})
			return
		}
		chosen := d.Chosen(in)
		names := modeChoiceNames(d.ResumeSA, chosen, d.ResumeModes)
		labels := chosenModeLabels(chosen)
		if o := e.G.Obj(pc.stackObj); o != nil {
			if !pc.modeChosen {
				pc.preModes = append([]string(nil), o.ChosenModes...)
				pc.modeChosen = true
			}
			o.ChosenModes = append([]string(nil), names...)
		}
		e.emit(events.Event{Kind: events.ModeChosen, Obj: pc.stackObj, Player: in.Player,
			Text: strings.Join(labels, ",")})
		e.continueCast()
		return
	}

	// CR 603.3c placement branch: this KModes decision was asked by the
	// trigger drain (pushTrigger's askTriggerModes) rather than posed
	// mid-resolution by an effect. There is no suspension to resume -- the
	// answer is recorded onto the trigger's stack object (ChosenModes, a
	// cache of the logged answer) so resolveTop builds Ctx.Modes from it and
	// effCharm runs exactly the chosen modes instead of asking again -- and
	// the drain resumes through the same continuation every other trigger
	// drain answer uses. drainAwaitsModes identifies the branch; it is false
	// for a mid-resolution ask, which falls through to the resume path below.
	if e.drainAwaitsModes {
		e.drainAwaitsModes = false
		chosen := d.Chosen(in)
		labels := chosenModeLabels(chosen)
		names := modeChoiceNames(d.ResumeSA, chosen, d.ResumeModes)
		if len(e.G.Stack) > 0 {
			id := e.G.Stack[len(e.G.Stack)-1]
			if o := e.G.Obj(id); o != nil {
				o.ChosenModes = names
			}
			e.emit(events.Event{Kind: events.ModeChosen, Obj: id, Player: in.Player,
				Text: strings.Join(labels, ",")})
		}
		e.resumeTriggerDrain()
		return
	}
	if e.resume == nil {
		e.emit(events.Event{Kind: events.Note, Player: in.Player,
			Text: "modes answered with no resolution suspended"})
		return
	}
	rp := e.resume
	e.resume = nil
	chosen := d.Chosen(in)
	labels := chosenModeLabels(chosen)
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
	before := e.triggerBefore
	e.triggerBefore = rp.before
	defer func() { e.triggerBefore = before }()
	savedResolving := e.resolvingObj
	e.resolvingObj = rp.obj
	defer func() { e.resolvingObj = savedResolving }()
	o := e.G.Obj(rp.obj)
	if o == nil || (o.Zone != state.ZStack && !rp.replacement) {
		// The suspended object left the stack while the decision was
		// outstanding. Nothing but the answer can un-freeze the engine, so
		// this is unreachable in a well-formed match; it degrades to a
		// no-op rather than panicking, the same totality stance as every
		// other resolution exit.
		return
	}
	ctx := &effects.Ctx{Source: rp.obj, Controller: o.Controller, Targets: o.Targets,
		Chosen: append([]state.Target(nil), rp.choices...), ChosenValid: rp.chosenValid,
		ChoiceTarget: rp.target}
	// CR 107.3i: X is the value paid for the object's {X}, preserved on the
	// stack object by CastInfo -- the same binding resolveTop's spell and
	// ability branches now carry. A spell whose resolution suspends on a
	// mid-resolution ask (modes, discard, dig, unless-pay) keeps the paid
	// X for the rest of the walk instead of resuming with 0. Set here for
	// every resume; the replacement arm below has no other X to restore.
	// For a trigger that suspends, CR 107.3m's binding applies exactly as
	// resolveTop's ability branch does it: the trigger object was never
	// paid an X, so the causing event's card supplies the value.
	ctx.X = o.X
	if ctx.X == 0 {
		ctx.X = e.triggerPaidX(rp.obj, o)
	}
	var svars map[string]string
	if o.Ability != nil {
		// A triggered or activated ability: mirror resolveTop's ability
		// branch — Source is the source permanent, Remembered carries what
		// the trigger captured, and the SVar table comes from that
		// permanent's face.
		ctx.Source = o.Source
		if !rp.replacement {
			ctx.TriggerContext = e.triggerContexts[rp.obj]
		}
		ctx.Remembered = o.Remembered
		ctx.Captured = o.Remembered
		if lki, ok := e.triggerLKI[rp.obj]; ok {
			ctx.LKI = lki.object
			ctx.LKIPower, ctx.LKIToughness, ctx.LKIPTValid =
				lki.power, lki.toughness, lki.ptValid
		}
		if link, ok := e.sourceLifelinkLKI[rp.obj]; ok {
			ctx.SourceLifelinkLKI = link
			ctx.SourceLifelinkLKIValid = true
		}
		if controller, ok := e.sourceControllerLKI[rp.obj]; ok {
			ctx.SourceControllerLKI = controller
			ctx.SourceControllerLKIValid = true
		}
		if lki := e.damageSourceLKI[rp.obj]; lki != nil {
			ctx.DamageSourceLKI = cloneDamageSourceLKI(lki)
		}
		// CR 603.3c: keep the placement-announced mode choice across the
		// suspension, so the resumed resolution of a modal trigger runs
		// exactly the modes chosen when the ability was put on the stack
		// rather than re-asking. The switch below overrides it (with the
		// NESTED answer) only for a nested "modes" resume, which is the
		// correct scoping -- a nested Charm below this one poses its own ask.
		ctx.Modes = o.ChosenModes
		if src := e.G.Obj(o.Source); src != nil {
			if sf := src.Face(); sf != nil {
				svars = sf.SVars
			}
		}
	} else if f := o.Face(); f != nil {
		svars = f.SVars
	}
	if rp.replacement {
		// fx44: this suspended frame is a ReplaceWith$ body, so restore the
		// replacement context applyReplacements seeded for it. Ctx.Replaced is
		// the object the replaced event was about (ev.Obj, threaded via
		// rp.replaced) and Ctx.Remembered is the single-element list seeded
		// from that same object, so a Defined$ ReplacedCard resolution and an
		// SVar:X Remembered$Amount gate find their subject after the
		// suspension. Without these the completed move (Mox Diamond's
		// MoveToBattlefield) targets nothing and the object never leaves the
		// stack.
		ctx.Replaced = rp.replaced
		ctx.Remembered = []state.Target{{Obj: rp.replaced}}
		ctx.Captured = ctx.Remembered
		if rp.remembered != nil {
			ctx.Remembered = append([]state.Target(nil), rp.remembered...)
		}
		// The body's own Source is the replacement's host, which differs from
		// the resolving object when that object caused another permanent's
		// replacement (a reanimation spell and Mox Diamond's discard): the
		// body's choices and SVars belong to the host.
		if rs := rp.replSource; rs != 0 && rs != rp.obj {
			if src := e.G.Obj(rs); src != nil {
				ctx.Source, ctx.Controller, ctx.X = rs, src.Controller, src.X
				ctx.TriggerContext = effects.TriggerContext{}
				svars = nil
				if f := src.Face(); f != nil {
					svars = f.SVars
				}
			}
		}
	}
	if rp.loopBound {
		ctx.Remembered = append([]state.Target(nil), rp.loopRemembered...)
	}
	// A mid-resolution picker may have built a Remembered fetch list before
	// it suspended. The stack object only carries trigger-time remembered
	// entries, so restore the asking effect's snapshot after rebuilding this
	// fresh context; otherwise Card.IsRemembered and Defined$ Remembered in a
	// chained hidden-origin ChangeZone see an empty list on re-entry.
	if rp.remembered != nil {
		ctx.Remembered = append([]state.Target(nil), rp.remembered...)
	}
	effects.SetSVars(ctx, svars)
	if rp.sa != nil {
		switch rp.kind {
		case "repeat":
			// A RepeatEach loop re-entered after one of its iterations
			// suspended: no answer, just the cursor (CR 608.2c).
			if cur := rp.repeat; cur != nil {
				ctx.Repeat = &effects.RepeatCursor{SA: rp.sa, Subjects: cur.subjects, Next: cur.next,
					Last: cur.last, HasLast: cur.hasLast}
			}
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
		case "choice":
			// ChooseCard, ChoosePlayer and ChangeTargets all use KChoose. Keep
			// the concrete target shape rather than just an ObjID because player
			// zero is a real target too.
			ctx.Choice = make([]state.Target, 0, len(chosen))
			for _, o := range chosen {
				if o.Kind == "player" {
					ctx.Choice = append(ctx.Choice, state.Target{Player: o.Player, IsPlayer: true})
				} else if o.Obj != 0 {
					ctx.Choice = append(ctx.Choice, state.Target{Obj: o.Obj})
				}
			}
			ctx.ChoiceDone = true
		case "search":
			// A hidden-library KChoose answer is an ordered subset. Preserve
			// that order for ChangeZone's MoveZone sequence, and set a separate
			// marker so choosing no cards still means "answered; do not re-ask".
			ctx.Search = make([]state.ObjID, 0, len(chosen))
			for _, o := range chosen {
				if o.Obj != 0 {
					ctx.Search = append(ctx.Search, o.Obj)
				}
			}
			ctx.SearchDone = true
		case "dig":
			// A Dig look-and-take pick was answered: the library owner chose
			// which of the window's ChangeValid$-eligible cards to move to
			// DestinationZone$. The chosen options carry the object in Obj
			// (the same shape the "search" and "discard" arms read), so the
			// id list is read straight off them, in the player's answer order.
			// DigDone distinguishes "answered, possibly with no cards" (an
			// Optional$ decline) from the first pass. DigTarget keeps that
			// answer attached to the exact Defined$ target that asked, even
			// when earlier targets completed before suspension. effDig consumes
			// and clears all three at the top of its own walk, so a nested Dig
			// cannot inherit the outer answer.
			ctx.Dig = make([]state.ObjID, 0, len(chosen))
			for _, o := range chosen {
				if o.Obj != 0 {
					ctx.Dig = append(ctx.Dig, o.Obj)
				}
			}
			ctx.DigDone = true
			ctx.DigTarget = rp.target
		case "hand_move":
			// A "choose N cards matching ChangeType$ from Origin$ Hand" pick was
			// answered (handmove1): the hand's owner chose which of the
			// ChangeType$-eligible cards to move to Destination$. The chosen
			// options carry the object in Obj (the same shape the "search",
			// "discard" and "dig" arms read), so the id list is read straight
			// off them, in the player's answer order. HandMoveDone distinguishes
			// "answered, possibly with no cards" from the first pass.
			// effChangeZoneHand consumes and clears both at the top of its own
			// walk (the fx42 scoping discipline), so a nested hand move cannot
			// inherit the outer answer.
			ctx.HandMove = make([]state.ObjID, 0, len(chosen))
			for _, o := range chosen {
				if o.Obj != 0 {
					ctx.HandMove = append(ctx.HandMove, o.Obj)
				}
			}
			ctx.HandMoveDone = true
		case "arrange":
			// Ruling J0: rules' handleArrange already applied the answered
			// arrangement and emitted the LibraryOrder event before calling
			// resumeResolution, so the re-entered effect needs only to know
			// not to re-ask -- the arrangement lives on the LibraryOrder
			// event, not on Ctx, so this is a done-marker rather than an
			// answer the effect re-reads.
			ctx.Arrange = true
		case "reveal_optional":
			// Task fb-3f1cc033 (Delver of Secrets): the peeking player's
			// RevealOptional$ yes/no was answered. Option 0 is "yes"; anything
			// else (option 1, an empty or malformed answer) is a decline — the
			// conservative read of an ambiguous answer is "no reveal". The
			// re-entered effReveal applies the answer: "yes" emits the Note
			// (which names the cards) and fires RememberRevealed$; "no" does
			// neither, so the chained ConditionDefined$ Remembered gate does
			// not fire either.
			if len(chosen) > 0 && chosen[0].Kind == "yes" {
				ctx.RevealOpt = "yes"
			} else {
				ctx.RevealOpt = "no"
			}
		case "optional":
			// CR 603.5: the decider answered yes to applying this optional
			// triggered ability's effect. The answer is a yes/no, not a mode
			// choice, so nothing is written to Ctx.Modes -- it was already
			// seeded from the stack object's ChosenModes by the o.Ability
			// branch above, exactly as resolveTop's own first pass would
			// have. The re-entry below just runs the ability's effect.
		default: // "modes", and "" (a pure outer continuation with no answer)
			ctx.Modes = modeChoiceNames(rp.sa, chosen, nil)
		}
		src := rp.obj
		if o.Ability != nil {
			src = o.Source
		}
		if rp.replacement && rp.replSource != 0 {
			src = rp.replSource
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
		e.repeatReported = nil
		// fx44: restore the replacement context the suspended body was
		// resolving under. applyReplacements reset e.applyingReplacement to
		// false when the body suspended, so without this the resumed body's
		// own completion move is re-intercepted by the same replacement it is
		// the product of — the re-asked discard loop. It is saved and restored
		// (not just set) so a resume that reaches here already inside a
		// replacement keeps the outer context intact, exactly the discipline
		// ensureLeftTheStack and applyReplacements already practise.
		savedReplacement := e.applyingReplacement
		e.applyingReplacement = rp.replacement
		e.replReplaced, e.replAction = rp.replaced, rp.action
		effects.Resolve(e, ctx, rp.sa)
		e.replReplaced, e.replAction = 0, ""
		e.applyingReplacement = savedReplacement
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
			if rp.loopBound {
				// Still inside the loop iteration this frame resumed: whatever
				// suspended at this level continues with its Remembered.
				e.bindLoopFrames(ctx.Remembered)
			}
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
	if rp.replacement && o.Zone != state.ZStack {
		// An Updated ETB replacement has already completed the spell's move.
		// Its answer resumes only the replacement body; there is no stack
		// object to finish or priority round to create here.
		return
	}
	if rp.outer != nil { // No nested ask this pass and the frame itself completed: continue
		// outward through the runner-up continuations this frame carried.
		if rp.loopBound {
			// Hand this frame's Remembered to the next frame. A loop frame
			// folds in what the finished iteration remembered. Any other next
			// frame runs at this frame's level -- the rest of the same
			// iteration, or (after a loop frame) the rest of the chain that
			// enclosed the loop -- and takes it as is, so what the loop
			// remembered is not lost to the stack object's stale Remembered.
			if next := rp.outer; next.kind == "repeat" && next.repeat != nil {
				next.repeat.last = append([]state.Target(nil), ctx.Remembered...)
				next.repeat.hasLast = true
			} else {
				next.loopBound = true
				next.loopRemembered = append([]state.Target(nil), ctx.Remembered...)
			}
		}
		e.resumeResolution(rp.outer, nil)
		return
	}
	e.finishResumption(rp.obj)
	// CR 117.3b / the counterpart of handlePriority's pass-branch grant: when
	// a SUSPENDED resolution completes (this is the outermost frame -- a
	// nested ask returns in the e.resume != nil branch above, and an outer
	// continuation recurses before reaching this line, so a resolution that
	// suspends more than once still reaches here exactly once), the pass
	// count resets and priority returns to the active player. The now-
	// suppressed unconditional emit in handlePriority's pass branch used to
	// log this at suspension time while the engine was parked on a
	// mid-resolution question, i.e. while nobody had priority; this emit puts
	// the reset and the "back to active" marker at the resolution's true end.
	// The answering Submit's own tail then grants the next priority round (its
	// grantPriority reads the passes this emit has just reset to zero), which
	// is the same two-event shape an unsuspended resolution already produces
	// (pass-branch grant + grantPriority), so the suspended path now matches.
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
}

// buildContinuationChain turns the enclosing-loop suspension points reported
// for one re-entry into a linked run of pure-continuation frames (kind ""),
// in report order — inner continuations first, outer last — and chains the
// given `tail` (the outer continuation of the frame being re-entered) onto
// the end. The resulting head is the what the new pending point must run
// after its own answer, or nil if there is nothing left to continue.
func (e *Engine) buildContinuationChain(frames []contFrame, obj state.ObjID, tail *resumePoint) *resumePoint {
	var head, prev *resumePoint
	for _, cf := range frames {
		sa := cf.sa
		// fx44: a continuation frame is the rest of the same resolution that
		// just suspended, so it carries the replacement context too — a body
		// that asks again and then continues must keep emitting under the
		// replacement guard, not re-interposed by the replacement it is the
		// product of, and must keep addressing the object it replaced (the
		// replaced/id carried by this frame comes from the engine's active
		// replacement context).
		f := &resumePoint{obj: obj, sa: sa.Sub, replacement: e.applyingReplacement,
			replaced: e.replReplaced, action: e.replAction, before: e.triggerBefore,
			loopBound: cf.bound, loopRemembered: cf.remembered}
		if cf.repeat != nil {
			f.kind, f.sa, f.repeat = "repeat", sa, cf.repeat
			f.choices, f.chosenValid = cf.choices, cf.chosenValid
		}
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

// chosenModeLabels returns the human-facing labels in answer order.
func chosenModeLabels(chosen []decision.Option) []string {
	labels := make([]string, 0, len(chosen))
	for _, o := range chosen {
		labels = append(labels, o.Label)
	}
	return labels
}

// modeDecision builds the shared KModes option vocabulary used by spell
// announcement and triggered-ability placement. min and max are resolved by
// effects.CharmModeBounds against the caller's complete effects context.
func modeDecision(p state.PlayerID, source state.ObjID, sa *cards.SA, svars map[string]string, min, max int) *decision.Decision {
	choices := strings.Split(sa.Params["Choices"], ",")
	for i := range choices {
		choices[i] = strings.TrimSpace(choices[i])
	}
	return modeDecisionForChoices(p, source, sa, svars, choices, min, max)
}

// modeDecisionForChoices is modeDecision over an explicit eligible subset.
// Casting uses it to omit modes whose mandatory targets cannot be chosen;
// ResumeModes preserves the SVar vocabulary server-side while Index stays
// dense for the wire.
func modeDecisionForChoices(p state.PlayerID, source state.ObjID, sa *cards.SA, svars map[string]string, choices []string, min, max int) *decision.Decision {
	if max > len(choices) {
		max = len(choices)
	}
	d := &decision.Decision{Player: p, Kind: decision.KModes, Min: min, Max: max,
		Source: source, ResumeKind: "modes", ResumeSA: sa,
		ResumeModes: append([]string(nil), choices...),
		Prompt:      "Choose " + strconv.Itoa(min) + " to " + strconv.Itoa(max) + " mode(s)"}
	for i, name := range choices {
		label := name
		if sub := cards.ResolveSVar(svars, name); sub != nil {
			if desc := strings.TrimSpace(sub.Params["SpellDescription"]); desc != "" {
				label = desc
			}
		}
		d.Options = append(d.Options, decision.Option{
			Index: i, Kind: "mode", Label: label, Obj: source, Player: p})
	}
	return d
}

// modeLabels maps SVar names to their human-facing option labels. abortCast
// uses it for the reverse ModeChosen marker that accompanies restoring the
// pre-proposal ChosenModes cache.
func modeLabels(sa *cards.SA, svars map[string]string, names []string) []string {
	labels := make([]string, 0, len(names))
	for _, name := range names {
		label := name
		if sub := cards.ResolveSVar(svars, name); sub != nil {
			if desc := strings.TrimSpace(sub.Params["SpellDescription"]); desc != "" {
				label = desc
			}
		}
		labels = append(labels, label)
	}
	return labels
}

// modeChoiceNames maps the chosen modal options back to the SVar names of
// the Choices$ sub-abilities they pick, in the order chosen — the answer
// effCharm's re-entry reads (Ctx.Modes). eligible carries a filtered cast
// decision's server-only vocabulary; nil falls back to the SA's full Choices$
// list for placement and mid-resolution decisions. Out-of-range indices drop.
func modeChoiceNames(sa *cards.SA, chosen []decision.Option, eligible []string) []string {
	if sa == nil {
		return nil
	}
	choices := eligible
	if choices == nil {
		choices = strings.Split(sa.Params["Choices"], ",")
		for i := range choices {
			choices[i] = strings.TrimSpace(choices[i])
		}
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

// bindLoopFrames binds the pending ask and every continuation frame recorded
// this pass that no deeper RepeatEach already bound to remembered.
func (e *Engine) bindLoopFrames(remembered []state.Target) {
	snap := append([]state.Target(nil), remembered...)
	if e.resume != nil && !e.resume.loopBound {
		e.resume.loopBound, e.resume.loopRemembered = true, snap
	}
	for i := range e.contChain {
		if !e.contChain[i].bound {
			e.contChain[i].bound, e.contChain[i].remembered = true, snap
		}
	}
}
