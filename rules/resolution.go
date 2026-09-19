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
	// replacementTarget/replacementSource/replacementAmount are the in-flight
	// Damage event's own target/source/amount (e.replacingEvent), captured so
	// a DB$ ReplaceEffect body that asks mid-resolution can rebuild the same
	// ReplacementTarget/ReplacementSource/ReplacementAmount Ctx fields on
	// resume that Defined$ ReplacedTarget/ReplacedSource and friends read.
	// Zero/empty outside a Damage replacement's body.
	replacementTarget state.Target
	replacementSource state.ObjID
	replacementAmount int32
	// action is the replaced event's action marker (Engine.replAction),
	// captured with replaced so a body that suspends before its move still
	// labels that move a sacrifice or discard on the resume.
	action string
	before *triggerSnapshot // immutable look-back if a batch replacement suspends
	// target is Dig's index into its deterministic Defined$ target list. It
	// keeps a resumed answer attached to the library that actually asked.
	target int
	// player is the decision's owner. Dredge uses it to apply the answered
	// replacement to the player drawing even when the enclosing effect's
	// controller is someone else.
	player state.PlayerID
	// direct identifies an effect invoked outside stack resolution (currently
	// an enters-the-battlefield replacement such as Hideaway). It resumes its
	// source directly rather than requiring a stack object.
	direct bool
	// moved is the object list a ShuffleNonMandatory$ search's first pass
	// moved before its may-shuffle confirm suspended, ridden on the ask via
	// Decision.ResumeMoved: the re-entry's LibraryPosition$ placement needs
	// the list the suspension lost. Nil for every other ask.
	moved       []state.ObjID
	choices     []state.Target
	chosenValid bool
	remembered  []state.Target
	// unlessPay is set only after a nested non-mana unless-cost payment has
	// completed. It prevents the resumed `unless_pay` arm from charging that
	// payment a second time.
	unlessPay string
	// unlessResolved is the unless-cost outcome the suspended pass recorded
	// through Host.SuspendUnless (effects.Resolve: the gate had resolved
	// when the SA's own body posed the pending ask). "resolved-pay" and
	// "resolved-decline" re-enter the gate as an already-resolved answer —
	// the re-entry pass consumes the marker instead of re-posing the pay
	// ask, the asking-body-under-UnlessCost$ livelock fix (Rhystic Study).
	unlessResolved string
	// rolls is the per-die results of the RollDice ask whose answer this
	// point resumes (effects/dice.go's ChosenSVar$/OtherSVar$ choose-one-
	// result shape, the Endeavor cycle): the asking first pass carried them
	// on the decision (decision.Decision.Rolls), Ask copies them here, and
	// the "roll" arm hands them to the re-entered effect, which publishes
	// the chosen/other sums WITHOUT re-rolling -- a re-roll would both
	// re-draw the seeded generator and answer a different question. Plain
	// value data, cloned with the point; a replay re-derives the same rolls
	// from the same seeded draws. Nil for every other ask.
	rolls []int32
	// replSource is the host of the replacement whose body asked (the
	// ReplaceWith$ body's own Ctx.Source); zero outside a replacement.
	replSource state.ObjID
	// replacedPlayer is the draw-er of the replaced Draw event the frame
	// resumes inside (Ctx.ReplacedPlayer); zero outside a Draw replacement.
	replacedPlayer state.Target
	// loopBound frames resume inside a RepeatEach iteration (or at the
	// RepeatEach itself, kind "repeat"): Ctx.Remembered is rebuilt from
	// loopRemembered rather than from the stack object, because the loop
	// binds its current subject there and the stack object never saw it.
	loopBound      bool
	loopRemembered []state.Target
	// repeatSubject is the RepeatEach subject of the loop whose iteration
	// this frame resumes inside (the Imprinted binding). It rides the frame
	// so a resumed unless/dig/etc. ask re-enters with Ctx.RepeatSubject
	// set; zero on frames outside any iteration.
	repeatSubject state.Target
	// repeat is a kind "repeat" frame's loop cursor.
	repeat *repeatCursor
	// lifeDraws parks a GainLife→Draw replacement body's remaining draws
	// (replacement.go's lifeReplacementDraw): the loop's DrawFor suspended
	// on a Dredge ask (CR 702.55) mid-replacement, and the answered dredge
	// re-drives the rest from this cursor. Zero for every other frame.
	lifeDraws int32
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
	sa            *cards.SA
	repeat        *repeatCursor
	bound         bool
	remembered    []state.Target
	repeatSubject state.Target
	choices       []state.Target
	chosenValid   bool
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
	direct := false
	if n := len(e.G.Stack); n > 0 {
		obj = e.G.Stack[n-1]
	} else {
		obj = d.Source
		direct = true
	}
	kind := d.ResumeKind
	if kind == "" {
		kind = "modes"
	}
	e.ask(d)
	var replacementTarget state.Target
	var replacementAmount int32
	if e.replacingEvent != nil && e.replacingEvent.Kind == events.Damage {
		replacementTarget = state.Target{Obj: e.replacingEvent.Obj}
		if e.replacingEvent.Obj == 0 {
			replacementTarget = state.Target{Player: e.replacingEvent.Player, IsPlayer: true}
		}
		replacementAmount = e.replacingEvent.Amount
	}
	// Capture whether the ask is being posed from inside a replacement
	// effect's ReplaceWith$ body (fx44). e.applyingReplacement is true for
	// the whole of that body's resolution, so an ask posed from within it
	// must resume still under the flag — see the resumePoint field's
	// comment and resumeResolution's restore of it.
	//
	// A replacement body resumes from the object whose resolution it
	// interrupted -- normally still on top of the stack (a sorcery that
	// reanimates Mox Diamond), so the rest of that spell's chain and its
	// completion still run after the answer. The one exception is a permanent
	// spell whose own Updated entry replacement asks (Sower of Discord): the
	// move has already taken the resolving object off the stack, so the top
	// of the stack is some unrelated object and the resume must rebuild from
	// the entering permanent itself. replSource keeps the replacement's host
	// for the resumed body's own Source either way. The one further
	// exception this build adds is an effect invoked outside stack
	// resolution (direct), whose resume must find its own source.
	var replSource state.ObjID
	if e.applyingReplacement && d.Source != 0 {
		replSource = d.Source
		// The resume must rebuild from the replacement's host in exactly two
		// shapes: a permanent spell whose own Updated entry replacement asks
		// (Sower of Discord — d.Source IS e.resolvingObj but the move has
		// already taken it off the stack, so the top of the stack is some
		// unrelated object), and a permanent that entered by a replacement with
		// NO stack resolution in flight at all (a land drop: Hallowed Fountain's
		// "you may pay 2 life. If you don't, it enters tapped" — e.resolvingObj
		// is 0, so the original d.Source == e.resolvingObj gate never fired and
		// the answer degraded to a no-op). Every other shape — a replacement
		// asking while a DIFFERENT spell is resolving (Mox Diamond reanimated by
		// a sorcery: d.Source is the mox, already on the battlefield, but the
		// spell whose chain the replacement interrupted is still the top of the
		// stack and MUST own the resume) — keeps the ordinary top-of-stack
		// resume. resolveTop keeps e.resolvingObj == the stack top for the whole
		// of a spell's resolution, so "top of stack is the interrupted spell" is
		// exactly "e.resolvingObj != 0 && d.Source != e.resolvingObj".
		if so := e.G.Obj(d.Source); so != nil && so.Zone != state.ZStack &&
			(e.resolvingObj == 0 || d.Source == e.resolvingObj) {
			obj = d.Source
		}
	}
	e.resume = &resumePoint{kind: kind, obj: obj, sa: d.ResumeSA, replSource: replSource,
		replacement: e.applyingReplacement, replaced: e.replReplaced, action: e.replAction,
		replacedPlayer:    e.replReplacedPlayer,
		replacementTarget: replacementTarget, replacementSource: e.protectionSource(e.damaging),
		replacementAmount: replacementAmount,
		before:            e.triggerBefore, target: d.ResumeTarget, player: d.Player,
		direct: direct, rolls: d.Rolls,
		choices:     append([]state.Target(nil), d.ResumeChoices...),
		chosenValid: d.ResumeChosenValid, remembered: append([]state.Target(nil), d.ResumeRemembered...),
		moved: append([]state.ObjID(nil), d.ResumeMoved...)}
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
// SuspendUnless implements effects.Host.SuspendUnless: the unless-cost
// outcome of an SA whose BODY posed the pending ask (the gate had resolved
// before the body suspended). The pending ask's resume point re-enters that
// SA (resumeResolution's effects.Resolve(e, ctx, rp.sa)), so the recorded
// marker rides exactly that resume point; a deeper frame's continuation
// re-enters sa.Sub (buildContinuationChain), which bypasses the gate, so
// those frames carry nothing.
func (e *Engine) SuspendUnless(sa *cards.SA, paid bool) {
	marker := "resolved-decline"
	if paid {
		marker = "resolved-pay"
	}
	if e.resume != nil && e.resume.sa == sa {
		e.resume.unlessResolved = marker
	}
}

func (e *Engine) Suspended() bool {
	return e.resume != nil || e.unlessPayment != nil || e.cumulative != nil || e.triggerCost != nil
}

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
		e.resume.loopBound, e.resume.loopRemembered, e.resume.repeatSubject = true, body, s.Subject
	}
	for i := range e.contChain {
		if !e.contChain[i].bound {
			e.contChain[i].bound, e.contChain[i].remembered, e.contChain[i].repeatSubject = true, body, s.Subject
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
	// An activated mana ability resolves outside the stack. Its UnlessCost$
	// answer is therefore owned by the mana activation flow rather than an
	// effects resume point, but is still recorded like every KModes answer.
	if d.ResumeKind == "mana_unless" {
		chosen := d.Chosen(in)
		labels := chosenModeLabels(chosen)
		e.emit(events.Event{Kind: events.ModeChosen, Obj: d.Source, Player: in.Player,
			Text: strings.Join(labels, ",")})
		e.choosing = chooseNone
		e.answerManaUnless(chosen)
		return
	}

	// CR 601.2b cast branch: the spell is already provisionally on the stack,
	// but no targets have been selected and no cost has been paid. Record the
	// answer on that spell, then resume the cast transaction at target choice.
	// CR 702.55 dredge: a draw-step draw replaced by a graveyard dredge is
	// NOT a stack object, so the ordinary mid-resolution resume path (which
	// re-enters a suspended stack-object resolution) does not apply. Handle
	// the answered dredge here: option 0 mills the dredge card's N and returns
	// it to hand (the ordinary draw is already skipped by the ask's
	// suspension), option 1 (or an empty answer) lets the draw happen, which
	// the suspended DrawFor re-runs as the ordinary draw.
	if d.ResumeKind == "dredge" && (e.resume == nil || e.resume.direct) {
		// A turn-based draw has no enclosing stack resolution to re-enter.
		// A Draw API on a resolving spell/ability instead falls through to
		// resumeResolution below, which restores its cursor and finishes every
		// remaining draw and SubAbility$ exactly once.
		ch := d.Chosen(in)
		if len(ch) > 0 && ch[0].Kind == "dredge" {
			e.applyDredge(in.Player, ch[0].Obj)
		} else {
			e.resumeOrdinaryDraw(in.Player)
		}
		// A GainLife→Draw replacement body parked its remaining draws on this
		// ask (replacement.go's lifeReplacementDraw): the answer resolved the
		// draw that asked, so re-drive the rest -- which may park again on
		// the next dredge ask -- and then drain any replacement-order queue
		// the interrupted pass left behind.
		if rp := e.resume; rp != nil && rp.lifeDraws > 0 {
			rest := rp.lifeDraws
			e.resume = nil
			e.lifeReplacementDraw(in.Player, rest)
			e.askNextReplacementChoice()
			return
		}
		e.resume = nil
		return
	}
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
			var so *state.Object
			if o := e.G.Obj(id); o != nil {
				so = o
				o.ChosenModes = names
			}
			e.emit(events.Event{Kind: events.ModeChosen, Obj: id, Player: in.Player,
				Text: strings.Join(labels, ",")})
			// A trigger Charm's targeting lives INSIDE its modes (the
			// corpus pairs ValidTgts$ on the chosen mode's SVar body,
			// never on the ability — Charming Scoundrel's DBToken), so the
			// mode the player just chose asks its targets now, at the same
			// placement moment CR 603.3c puts the mode choice. The first
			// selected target-bearing mode wins, the same one-undivided
			// target-list narrowing the spell-side Charm carries. The
			// answer lands on the stack object through handleTarget's
			// ordinary record, and the resolution's effToken
			// (AttachedTo$ Targeted) and friends read it as c.Targets.
			if so != nil && so.Ability != nil {
				if src := e.G.Obj(so.Source); src != nil && src.Face() != nil {
					for _, name := range names {
						if sub := cards.ResolveSVar(src.Face().SVars, name); sub != nil &&
							strings.TrimSpace(sub.Params["ValidTgts"]) != "" {
							e.drainAwaitsTarget = true
							e.askTarget(in.Player, id, sub)
							return
						}
					}
				}
			}
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
	// A GainLife→Draw replacement body parked its remaining draws on this
	// ask (replacement.go's lifeReplacementDraw). The body is not a stack
	// resolution: there is no sub-ability to re-enter (rp.sa is nil -- the
	// loop called DrawFor directly, which poses its own dredge ask). The
	// signature is exact: DrawFor is the only sa==nil dredge asker (effDraw's
	// frames carry ResumeSA, and a turn-based draw's direct frame never
	// reaches here -- handleModes routes those to its own arm). The answer
	// resolved the draw that asked, whether or not draws remain parked
	// (lifeDraws == 0 is the FINAL draw of the body -- findings-sol5: the old
	// lifeDraws > 0 gate dropped that frame's answer into the no-sub-ability
	// Note below); apply it and re-drive the rest (which may park again on
	// the next dredge ask). The frame's rp.outer continuation and the
	// completion tail below still run after it -- the same order the body ran
	// in before it suspended -- and the drain at this cascade's true end
	// picks up any replacement-order queue the interrupted pass left behind.
	parkedDraws := false
	if rp.kind == "dredge" && rp.sa == nil {
		parkedDraws = true
		if !e.G.Players[rp.player].Lost {
			// CR 800.4f: a departed player makes no choice and draws
			// nothing; the outer continuation below still runs.
			if len(chosen) > 0 && chosen[0].Kind == "dredge" {
				e.applyDredge(rp.player, chosen[0].Obj)
			} else {
				e.resumeOrdinaryDraw(rp.player)
			}
			e.lifeReplacementDraw(rp.player, rp.lifeDraws)
			if e.Suspended() || e.pending != nil {
				// The re-drive parked on the next dredge ask: that frame's
				// own resume arms carry the rest. The new pending point is
				// fresh (outer nil -- Ask builds it bare, and nothing in this
				// re-drive reports continuations), so link the interrupted
				// frame's own continuation onto it -- the same fx32 linking
				// discipline the e.resume != nil branch below practises --
				// or the cascade's last frame would complete the object
				// without ever running what this interrupted resolution was
				// still carrying (findings-sol5: the sub-ability after the
				// GainLife never ran). Every later park re-links the same
				// chain, so rp.outer survives until the cascade truly ends.
				if e.resume != nil {
					e.resume.outer = rp.outer
				}
				return
			}
		}
	}
	before := e.triggerBefore
	e.triggerBefore = rp.before
	defer func() { e.triggerBefore = before }()
	savedResolving := e.resolvingObj
	e.resolvingObj = rp.obj
	defer func() { e.resolvingObj = savedResolving }()
	o := e.G.Obj(rp.obj)
	if o == nil || (o.Zone != state.ZStack && !rp.replacement && !rp.direct) {
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
	if rp.replacement {
		// fx44: this suspended frame is a ReplaceWith$ body, so restore the
		// replacement context applyReplacements seeded for it. Ctx.Replaced is
		// the object the replaced event was about (ev.Obj, threaded via
		// rp.replaced), so a Defined$ ReplacedCard resolution still finds its
		// subject after the suspension. Without it the completed move (Mox
		// Diamond's MoveToBattlefield) targets nothing and the object never
		// leaves the stack. Ctx.Remembered is NOT re-seeded with the replaced
		// object: Remembered$Amount inside a replacement body counts the
		// body's own rider-remembered cards (Mox's discarded land, Scorched
		// Ruins' sacrificed two), never the replaced card itself — the same
		// empty-start rule replCtx (rules/replacement.go) documents. The
		// body's own Remembered from before the ask rides rp.remembered. The replacement arm has no other X to restore, so
		// triggerPaidX's fallback below is skipped for it.
		ctx.Replaced = rp.replaced
		ctx.ReplacementTarget = rp.replacementTarget
		ctx.ReplacementSource = rp.replacementSource
		ctx.ReplacementAmount = rp.replacementAmount
		// Remembered is NOT seeded with the replaced/damaged object: no corpus
		// replacement body (Damage ones included — measured, zero use
		// Remembered in a Damage body) reads it, and seeding it made every
		// Remembered$Amount gate or count inside a replacement one too high
		// (Mox Diamond's EQ0/EQ1 discard split broke). The body's own
		// rider-remembered cards ride rp.remembered below.
	} else if ctx.X == 0 {
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
		// rp.replaced), so a Defined$ ReplacedCard resolution and an
		// SVar:X Remembered$Amount gate find their subject after the
		// suspension. Without these the completed move (Mox Diamond's
		// MoveToBattlefield) targets nothing and the object never leaves the
		// stack. (Captured, not Remembered, carries the replaced object on
		// the non-draw path — the same empty-Remembered rule replCtx
		// (rules/replacement.go) documents.)
		ctx.Replaced = rp.replaced
		ctx.ReplacedPlayer = rp.replacedPlayer
		if rp.replacedPlayer.IsPlayer {
			// A Draw replacement body: the draw-er's binding is the whole
			// seed, and the body's own RememberDrawn$ records what it draws —
			// a MoveZone-shaped Remembered seed would pollute the reveal and
			// the discard condition with a stale would-be-drawn entry.
			ctx.Remembered, ctx.Captured = nil, nil
		} else {
			// Ctx.Remembered is NOT re-seeded with the replaced object: a
			// Remembered$Amount gate inside a replacement body counts the
			// body's own rider-remembered cards, never the replaced card
			// itself (seeding it made every such gate one too high).
			ctx.Captured = []state.Target{{Obj: rp.replaced}}
		}
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
	// A mid-resolution ask that rode the walk's Remembered (the hidden-library
	// search sets ResumeRemembered -- a cast spell's Remembered lives only in
	// the resolving Ctx frame, so without the ride the resume rebuilds an
	// empty set and the re-entered primitive's eligibility recheck and the
	// chain's later sub-abilities see nothing, and Card.IsRemembered /
	// Defined$ Remembered in a chained hidden-origin ChangeZone would see an
	// empty list on re-entry too). The loop and replacement branches above
	// are authoritative when they fire; this applies only to the ordinary
	// frames, which never carry rp.remembered otherwise.
	if rp.remembered != nil && !rp.replacement && !rp.loopBound {
		ctx.Remembered = append([]state.Target(nil), rp.remembered...)
	}
	effects.SetSVars(ctx, svars)
	// An accepted optional trigger may itself carry Cost$ (Mana Vault's
	// "you may pay {4}; if you do" untap). The optional answer chooses to
	// attempt the effect; payment is a separate resolution-time window with
	// mana-ability opportunities. Direct mandatory triggers enter the same
	// window from resolveTop.
	if rp.kind == "optional" && rp.sa != nil && rp.sa.API == "Untap" && rp.sa.Params["Cost"] != "" {
		e.startTriggeredEffectCost(rp, ctx.Source)
		return
	}
	if rp.sa != nil {
		switch rp.kind {
		case "dredge":
			// CR 702.55 replaces exactly the one draw that asked. The enclosing
			// Draw cursor advances only after the replacement (or declined
			// ordinary draw) completes; effDraw then re-enters at that cursor
			// and performs all remaining draws before its SubAbility$.
			if len(chosen) > 0 && chosen[0].Kind == "dredge" {
				e.applyDredge(rp.player, chosen[0].Obj)
			} else {
				e.resumeOrdinaryDraw(rp.player)
			}
			ctx.DrawDone = int32(rp.target + 1)
		case "repeat":
			// A RepeatEach loop re-entered after one of its iterations
			// suspended: no answer, just the cursor (CR 608.2c).
			if cur := rp.repeat; cur != nil {
				ctx.Repeat = &effects.RepeatCursor{SA: rp.sa, Subjects: cur.subjects, Next: cur.next,
					Last: cur.last, HasLast: cur.hasLast}
			}
		case "unless_pay":
			if rp.unlessPay != "" {
				ctx.UnlessPay = rp.unlessPay
				ctx.UnlessNext = rp.target
				break
			}
			// The payer agreed to pay (option 0 is "Pay …") or not. Payment
			// happens HERE, in rules, because payMana owns the cost grammar and
			// emits the ManaAdd events — so a replay re-derives the identical
			// payment. An answer to pay from a payer that cannot cover the cost
			// is a decline: the effect's body runs (or not) per its orientation,
			// Ward has non-mana payment forms (sacrifice, discard, tap and
			// several keyword-specific costs). Its payment handler owns those
			// choices; ordinary unless-pay effects retain the shared mana path.
			if rp.sa.API == "Ward" {
				if len(chosen) > 0 && chosen[0].Index == 0 {
					paid, asked := e.beginWardPayment(rp, ctx)
					if asked {
						return
					}
					if paid {
						ctx.UnlessPay = "pay"
					} else {
						ctx.UnlessPay = "decline"
					}
				} else {
					ctx.UnlessPay = "decline"
				}
				break
			}
			// Sacrifice's damage-payment offer (Vexing Devil's UnlessCost$
			// DamageYou<4>, UnlessPayer$ Opponent, UnlessSwitched$ True):
			// "paying" is TAKING THE DAMAGE, which the mana path below cannot
			// express — ParseCost silently substitutes an unknown spelling for
			// a flat {1} and would charge one floating mana for four damage.
			// Payment happens HERE, in rules (the same split that owns
			// payMana's events): the accepting opponent's Damage event is
			// emitted from the offering permanent, and the answered
			// UnlessPay re-enters effSacrifice, which then sacrifices (the
			// switched orientation: paying CAUSES the sacrifice). The resume
			// point's target cursor travels with the answer so the effect can
			// offer the next opponent after a decline.
			if rp.sa.API == "Sacrifice" {
				if n, dmg := effects.ParseDamageUnlessCost(rp.sa.Params["UnlessCost"]); dmg {
					if len(chosen) > 0 && chosen[0].Index == 0 {
						e.payUnlessDamageCost(ctx, chosen[0].Player, n)
						ctx.UnlessPay = "pay"
					} else {
						ctx.UnlessPay = "decline"
					}
					// The answering payer's cursor travels in UnlessNext (the
					// same field every other unless-pay answer uses): a decline
					// re-entry resumes the offer at payers[idx+1], and the last
					// decline ends the ask. (UnlessPayTarget was a vestigial
					// second cursor nothing read — its one write is this line —
					// so a second opponent's decline re-offered payers[1]
					// forever.)
					ctx.UnlessNext = rp.target
					break
				}
				// A plain-mana UnlessCost$ (the echo / cumulative-upkeep
				// family) falls through to the shared mana path below, exactly
				// like a Counter's: paid spares the permanent, decline
				// sacrifices it. The unimplemented non-mana shapes never
				// reach the ask, so they never reach this arm.
			}
			// deterministically.
			paid, ok := ParseUnlessCost(rp.sa.Params["UnlessCost"])
			if !ok {
				// I-5: an unless-cost the payment API cannot price is a hard
				// DECLINE. ParseCost("X") is {Generic:0, X:1}; payMana never
				// charges the unfolded X, so an empty pool "pays" it for free
				// and the counterspell stays inert. An unpriceable cost must
				// decline, never resolve at zero. This is the conservative
				// correct behaviour: a cleared counter is closer to the card
				// than a no-op. The real fix (M4) is cost-grammar work — a
				// value for X from CastInfo/ModeChosen or an SVar folded into
				// Generic via WithX before payment. ParseUnlessCost is the
				// strict parser: every token must be a mana symbol, a fixed
				// PayLife<N>, or a Sac/Discard/SubCounter component; X, Y,
				// DamageYou<N>, PayEnergy<N>, Return<...>, ExileFromGrave<...>,
				// Reveal<...>, LifeTotalHalfUp, DefinedCost_* and every other
				// dynamic or unmodelled token declines here rather than
				// ParseCost's flat {1} substitution buying it for one generic.
				// The ask is still posed to the payer (the answer is recorded by
				// ModeChosen) but cannot succeed. Declining here (rather than
				// suppressing the ask in effects, which cannot import rules'
				// cost type) keeps the decision on the wire for hosts to observe
				// while never letting an empty pool satisfy it.
				ctx.UnlessPay = "decline"
			} else if len(chosen) > 0 && chosen[0].Index == 0 {
				if len(paid.Sac) > 0 || len(paid.Discard) > 0 {
					// Sacrifice and discard are choice-bearing costs. Park this
					// resume before any mutation and let the payer select every
					// component; finishUnlessPayment re-enters with unlessPay
					// set, so this arm never charges it twice.
					e.beginUnlessPayment(chosen[0].Player, paid, ctx, rp.obj, rp)
					return
				}
				if e.payUnlessCost(chosen[0].Player, paid, ctx, rp.obj) {
					ctx.UnlessPay = "pay"
				} else {
					ctx.UnlessPay = "decline"
				}
			} else {
				ctx.UnlessPay = "decline"
			}
			// The payer whose answer this is (the unlessProceed gate moves a
			// decline on to the next UnlessPayer$ payer, and a pay ends the
			// ask), threaded through the decision's ResumeTarget via the
			// resume point — the same channel the "choice" and "dig" arms use.
			ctx.UnlessNext = rp.target
		case "sacrifice_optional":
			// Optional$ + StrictAmount$ is a disjoint choice (decline, or
			// exactly Amount) that KChoose cannot represent. Its first KModes
			// answer records only the election; effSacrifice then asks an exact
			// KChoose if several complete batches are available.
			ctx.SacOptional = "decline"
			if len(chosen) > 0 && chosen[0].Index == 0 {
				ctx.SacOptional = "sacrifice"
			}
			ctx.SacOptionalTarget = rp.target
		case "sacrifice":
			// A player-targeted Sacrifice's KChoose (CR 701.21a: the
			// sacrificing player chooses which of their permanents) was
			// answered. The chosen options carry the object in Obj (the same
			// shape the "discard" and "dig" arms read), so the id list goes
			// straight to Ctx.SacPicks in the player's answer order; SacDone
			// distinguishes "answered, possibly with nothing" (an Optional$
			// decline) from the first pass, and SacTarget keeps the answer
			// attached to the exact Defined$ target that asked. effSacrifice
			// consumes and clears all three at the top of its own walk, so a
			// nested sacrifice cannot inherit the outer answer.
			ctx.SacPicks = make([]state.ObjID, 0, len(chosen))
			for _, o := range chosen {
				if o.Obj != 0 {
					ctx.SacPicks = append(ctx.SacPicks, o.Obj)
				}
			}
			ctx.SacDone = true
			ctx.SacTarget = rp.target
		case "ward_mana":
			if e.answerWardMana(rp, chosen, ctx) {
				return
			}
		case "ward_alt":
			// The Discard<...>:<mana> Ward alternative can choose its mana
			// half even when it is not already floating; it receives the same
			// CR 702.21a activation window as an ordinary numeric Ward.
			if len(chosen) == 1 && chosen[0].Kind == "ward_mana" {
				_, manaRaw, _ := strings.Cut(rp.sa.Params["UnlessCost"], ">:")
				cost := e.parseCost(manaRaw)
				if e.payMana(chosen[0].Player, cost) {
					ctx.UnlessPay = "pay"
				} else if cost.hasManaPayment() && e.hasUntappedManaSource(chosen[0].Player) {
					e.askWardMana(rp, chosen[0].Player, cost)
					return
				} else {
					ctx.UnlessPay = "decline"
				}
				break
			}
			fallthrough
		case "ward_blight", "ward_evidence", "ward_waterbend", "ward_tap", "ward_sac", "ward_discard":
			if e.settleWardPayment(rp.kind, rp.sa, ctx, chosen) {
				ctx.UnlessPay = "pay"
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
		case "search_mayshuffle":
			// A ChangeZone search carrying ShuffleNonMandatory$ True (Path to
			// Exile, Stoneforge Mystic, Boggart Harbinger) asked its searcher
			// "Shuffle your library?" after the search's moves landed. The
			// answer is a bare yes/no, recorded here as a marker the
			// re-entered effect consumes and clears (fx42 scoping): "yes"
			// emits the same Secret events.Shuffle every library shuffle
			// emits, "no" -- the information-mercy Forge's flag names -- keeps
			// the library order. The moved list rides the ask
			// (Decision.ResumeMoved -> rp.moved) so the re-entry can finish
			// with the LibraryPosition$ placement after the answered shuffle.
			// A malformed or empty answer keeps the order, the conservative
			// read of an ambiguous one.
			ctx.SearchShuffle = "no"
			if len(chosen) > 0 && chosen[0].Kind == "yes" {
				ctx.SearchShuffle = "yes"
			}
			ctx.SearchShuffleMoved = append([]state.ObjID(nil), rp.moved...)
		case "imprint":
			// An Imprint$ True public-zone choice. The effect consumes this
			// answer on re-entry and emits the persistent Imprint event.
			ctx.Imprint = make([]state.ObjID, 0, len(chosen))
			for _, o := range chosen {
				if o.Obj != 0 {
					ctx.Imprint = append(ctx.Imprint, o.Obj)
				}
			}
			ctx.ImprintDone = true
		case "untap":
			ctx.Untap = make([]state.ObjID, 0, len(chosen))
			for _, o := range chosen {
				if o.Obj != 0 {
					ctx.Untap = append(ctx.Untap, o.Obj)
				}
			}
			ctx.UntapDone = true
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
		case "counter_dist":
			// A DividedAsYouChoose$ PutCounter distribution pick was answered
			// (Vastwood Hydra): the chooser picked which of the Choices$
			// eligible battlefield creatures receive the CounterNum$ total, in
			// answer order. CounterDistDone distinguishes "answered, possibly
			// with no creatures" (a MinChoiceAmount$ 0 decline) from the first
			// pass. effPutCounter consumes and clears both at the top of its
			// own walk (the fx42 scoping discipline), so a nested PutCounter
			// cannot inherit the outer answer.
			ctx.CounterDist = make([]state.ObjID, 0, len(chosen))
			for _, o := range chosen {
				if o.Obj != 0 {
					ctx.CounterDist = append(ctx.CounterDist, o.Obj)
				}
			}
			ctx.CounterDistDone = true
		case "roll":
			// A RollDice choose-one-result answer (effects/dice.go's
			// ChosenSVar$/OtherSVar$ shape, the Endeavor cycle): the chosen
			// options' Index values name the dice (into the ask's own per-die
			// results, rp.rolls) the player picked. The re-entered effRollDice
			// publishes ChosenSVar$ = the sum of the picked dice's results,
			// OtherSVar$ = the sum of the rest, and consumes and clears all
			// three Ctx fields at its top (the fx42 scoping discipline).
			ctx.RollResults = rp.rolls
			pick := make([]int, 0, len(chosen))
			for _, o := range chosen {
				pick = append(pick, o.Index)
			}
			ctx.RollPick = pick
			ctx.RollDone = true
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
			// The owner-selected shape (rv2b r2) chains one ask per hand owner:
			// the answer belongs to the exact owner that asked, and the
			// re-entered walk skips owners before the cursor and continues with
			// the owners after it (the same continuation DigTarget carries).
			ctx.HandMoveTarget = rp.target
		case "hidden_pick":
			// A Hidden$ True public-origin pick was answered (hiddenpick1): the
			// chooser picked which of the ChangeType$-eligible cards in the
			// origin zone(s) move to Destination$. The chosen options carry the
			// object in Obj (the same shape the "search", "dig" and
			// "hand_move" arms read), so the id list is read straight off them,
			// in the player's answer order. HiddenPickDone distinguishes
			// "answered, possibly with no cards" from the first pass, and the
			// cursor keeps the answer attached to the exact fetch player that
			// asked -- the same continuation HandMoveTarget carries.
			// effHiddenPick consumes and clears all three at its top (the fx42
			// scoping discipline), so a nested pick cannot inherit the outer
			// answer.
			ctx.HiddenPick = make([]state.ObjID, 0, len(chosen))
			for _, o := range chosen {
				if o.Obj != 0 {
					ctx.HiddenPick = append(ctx.HiddenPick, o.Obj)
				}
			}
			ctx.HiddenPickDone = true
			ctx.HiddenPickTarget = rp.target
		case "arrange":
			// Ruling J0: rules' handleArrange already applied the answered
			// arrangement and emitted the LibraryOrder event before calling
			// resumeResolution, so the re-entered effect needs only to know
			// not to re-ask -- the arrangement lives on the LibraryOrder
			// event, not on Ctx, so this is a done-marker rather than an
			// answer the effect re-reads.
			ctx.Arrange = true
		case "arrange_mayshuffle":
			// A RearrangeTopOfLibrary carrying MayShuffle$ True (Ponder) asked
			// "you may shuffle?" on its arrange re-entry pass. The effect
			// re-enters here third: the arrange itself was applied two passes
			// ago, so ctx.Arrange stays the done-marker; ctx.MayShuffle carries
			// the answer as the ask-again marker the effect consumes and
			// clears. The shuffle itself is emitted HERE, before the
			// re-entered walk continues to the chained SubAbility$ (Ponder's
			// draw comes after the shuffle) -- the same Fisher-Yates over the
			// engine rng and the same Secret events.Shuffle the genesis deal
			// and every effects shuffle emit, with rp.player the library's
			// owner (the arranging player the ask was posed to).
			ctx.Arrange = true
			ctx.MayShuffle = "no"
			if len(chosen) > 0 && chosen[0].Kind == "yes" {
				ctx.MayShuffle = "yes"
				order := append([]state.ObjID(nil), e.G.Zone(state.ZLibrary, rp.player)...)
				e.rng.Shuffle(order)
				e.emit(events.Event{Kind: events.Shuffle, Player: rp.player, IDs: order, Secret: true})
			}
		case "discard_unless":
			// A Discard carrying UnlessType$ (Thirst for Knowledge) asked its
			// election: "discard one card of the type instead" vs "discard
			// NumCards$". The re-entered effDiscard reads the election off
			// ctx.UnlessElected (consumed and cleared there, fx42 scoping);
			// an empty or malformed answer elects the ordinary discard, the
			// conservative read of an ambiguous one.
			ctx.UnlessElected = "ordinary"
			if len(chosen) > 0 && chosen[0].Kind == "unless" {
				ctx.UnlessElected = "unless"
			}
		case "hideaway_pick":
			// Hideaway's first ask chooses exactly one of the looked-at cards.
			// The effect validates it remains in the library before moving it,
			// then asks the separate ordered-bottom question for the remainder.
			ctx.HideawayPicked = true
			if len(chosen) == 1 {
				ctx.Hideaway = chosen[0].Obj
			}
		case "hideaway_arrange":
			ctx.HideawayArranged = true
		case "soulbond":
			// Soulbond is a may choice: no option is a legitimate decline.
			ctx.SoulbondDone = true
			if len(chosen) == 1 {
				ctx.SoulbondPartner = chosen[0].Obj
			}
		case "myriad":
			// CR 702.109 makes a separate may choice for each eligible opponent.
			// ResumeTarget is that opponent's stable index in effMyriad's
			// deterministic list; only its explicit yes option creates the copy.
			ctx.MyriadDone = true
			ctx.MyriadTarget = rp.target
			ctx.MyriadCreate = len(chosen) == 1 && chosen[0].Kind == "yes"
		case "defined_library_optional":
			// An Optional$ object-valued Defined$ library fetch list (Kenessos's
			// DBBottom): option zero accepts the whole direct move; every other
			// answer declines it. The effect consumes this marker before any
			// nested optional fetch can see it.
			if len(chosen) > 0 && chosen[0].Kind == "yes" {
				ctx.DefinedLibraryMove = "yes"
			} else {
				ctx.DefinedLibraryMove = "no"
			}
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
		case "look_ack":
			// The bare private look's pacing ack (lookack, task
			// fb-20260917T232325Z-35cfca4b, Mishra's Bauble / Gitaxian Probe):
			// the looker clicked Continue on the "You look at ..." modal. There
			// is NO decline — the ask gates only the pacing, and CR 701.20e
			// requires the look itself to happen — so ANY answer (the single
			// Continue option; a malformed empty one included) acknowledges.
			// The answer is addressed by the per-target cursor (the DigTarget
			// pattern): LookAckTarget carries rp.target, the index of the
			// Defined$ target whose ack was answered, so the re-entered
			// effReveal emits exactly that target's note, skips the targets
			// already processed on the pass that suspended, and every later
			// bare look in the walk poses its own ack — without the cursor a
			// multi-target bare look (Case the Joint's Defined$ Player)
			// re-posed the last target's ack forever.
			ctx.LookAck = true
			ctx.LookAckTarget = rp.target
		case "draw_optional":
			// OptionalDecider$ Draw (Mystic Remora, Rhystic Study): the
			// decider's yes/no was answered. Option 0 is "yes" (draw the
			// NumCards the unless arm did not price); anything else — option
			// 1, an empty or malformed answer — is a decline, the same
			// conservative read the reveal_optional arm takes. The re-entered
			// effDraw consumes the answer before its draw loop (fx42), so a
			// chained sub-Draw poses its own ask.
			if len(chosen) > 0 && chosen[0].Kind == "yes" {
				ctx.DrawOpt = "yes"
			} else {
				ctx.DrawOpt = "no"
			}
		case "play":
			// A Play effect (Conduit of Worlds, Spinerock Knoll) was answered:
			// each chosen option's Obj is a card to play from its current zone,
			// in answer order. An empty answer is a DECLINE of an Optional$
			// Play (effPlay now offers Min 0) -- the answer is consumed with
			// nothing begun. Only the effect's own WithoutManaCost$ grants a
			// free cast: Conduit has no such parameter, while Spinerock Knoll
			// does. An Amount$ All / N answer may name several cards; each is
			// begun in turn, and the loop stops at the first cast that cannot
			// commit synchronously (an ask inside the cast transaction -- an
			// ETB choice, a target, a mana window -- parks the resolution on
			// that cast's question, and the not-yet-begun cards are dropped
			// with a Note rather than wedging; the corpus Amount$ All shapes
			// are without-mana-cost creature/spell plays, which commit
			// synchronously). ctx.Play/PlayDone are set so the re-entered
			// effPlay sees the answer as consumed either way.
			free := strings.EqualFold(rp.sa.Params["WithoutManaCost"], "True")
			var toPlay []state.ObjID
			for _, ch := range chosen {
				if ch.Obj != 0 {
					toPlay = append(toPlay, ch.Obj)
				}
			}
			ctx.PlayDone = true
			for i, id := range toPlay {
				if i == 0 {
					ctx.Play = id
				}
				e.beginPlay(ctx.Controller, id, free)
				if e.Suspended() || e.cast != nil {
					if rest := toPlay[i+1:]; len(rest) > 0 {
						e.emit(events.Event{Kind: events.Note, Obj: rp.obj,
							Text: "Play stopped after a suspended cast; the remaining cards stay unplayed"})
					}
					break
				}
			}
		case "extort":
			// Extort's optional {W/B} payment was answered. Option 0 is "pay";
			// anything else is a decline. The hybrid pip is charged from the
			// caster's pool as one W or B when available; a pool lacking both
			// colours deterministically declines (the drain never runs without
			// the mana being genuinely paid). The re-entered effExtort reads
			// Ctx.Extort and runs the drain only on "pay".
			if len(chosen) > 0 && chosen[0].Index == 0 {
				if e.payExtortPip(chosen[0].Player) {
					ctx.Extort = "pay"
				} else {
					ctx.Extort = "decline"
				}
			} else {
				ctx.Extort = "decline"
			}
		case "effect_paid":
			// The trigger's Cost$ was paid by triggeredCostAnswer; run the
			// parked effect without opening the payment window a second time.
		case "optional":
			// CR 603.5: the decider answered yes to applying this optional
			// triggered ability's effect. The answer is a yes/no, not a mode
			// choice, so nothing is written to Ctx.Modes -- it was already
			// seeded from the stack object's ChosenModes by the o.Ability
			// branch above, exactly as resolveTop's own first pass would
			// have. The re-entry below just runs the ability's effect.
		default: // "modes", and "" (a pure outer continuation with no answer)
			// A KWChoice$ pump's modes are keyword labels, not SVar names:
			// when the asking SA carries no Choices$ but a KWChoice$, the
			// chosen indexes map against THAT list (effects' effPump re-entry
			// consumes them as the granted keywords).
			eligible := []string(nil)
			if strings.TrimSpace(rp.sa.Params["Choices"]) == "" {
				if kw := strings.TrimSpace(rp.sa.Params["KWChoice"]); kw != "" {
					eligible = strings.Split(kw, ",")
					for i := range eligible {
						eligible[i] = strings.TrimSpace(eligible[i])
					}
				}
			}
			ctx.Modes = modeChoiceNames(rp.sa, chosen, eligible)
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
		e.replReplaced, e.replAction, e.replReplacedPlayer = rp.replaced, rp.action, rp.replacedPlayer
		// The body's own re-entry (Host.SuspendUnless): the gate of THIS SA
		// had already resolved when the body posed the pending ask, so the
		// recorded marker re-enters it as an already-resolved answer — the
		// unless gate consumes it and never re-poses the pay ask (the
		// asking-body-under-UnlessCost$ livelock fix). A unless_pay resume
		// point never carries the marker: that ask was posed by the gate
		// itself, before any body ran. The arm's own authoritative answer
		// (the Ward arm's beginWardPayment outcome, the generic arm's
		// payUnlessCost outcome) wins over the marker — the gate-posed ask's
		// own suspension also records a marker (the SA carries UnlessCost$),
		// and letting it clobber the arm's answer counted a PAID ward as a
		// decline (the Kitesail Larcenist regression).
		if rp.unlessResolved != "" && ctx.UnlessPay == "" {
			ctx.UnlessPay = rp.unlessResolved
		}
		effects.Resolve(e, ctx, rp.sa)
		e.replReplaced, e.replAction, e.replReplacedPlayer = 0, "", state.Target{}
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
	} else if !parkedDraws && rp.kind != "replacement" {
		// A resume with no sub-ability recorded: normally reachable only from
		// a hand-built Ask (every real asking primitive sets ResumeSA). Two
		// deliberate exceptions need no Note either: a parked GainLife→Draw
		// frame whose answer was applied above, and a replacement-order
		// decision — the intercepted event has already completed, and the
		// continuation begins at rp.outer rather than re-running the effect
		// that proposed it. The resolution still finishes — the object leaves
		// the stack with no effect, the same degrade-to-nothing stance as an
		// unrecognised choice, rather than stalling the match forever.
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
		if parkedDraws {
			e.askNextReplacementChoice()
		}
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
	if parkedDraws {
		// The cascade's true end: any replacement-order choice the
		// interrupted pass left queued is asked now, after the resolution's
		// completion marker, never before it.
		e.askNextReplacementChoice()
	}
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
			replaced: e.replReplaced, action: e.replAction, replacedPlayer: e.replReplacedPlayer,
			before:    e.triggerBefore,
			loopBound: cf.bound, loopRemembered: cf.remembered, repeatSubject: cf.repeatSubject}
		if e.replacingEvent != nil && e.replacingEvent.Kind == events.Damage {
			f.replacementTarget = state.Target{Obj: e.replacingEvent.Obj}
			if e.replacingEvent.Obj == 0 {
				f.replacementTarget = state.Target{Player: e.replacingEvent.Player, IsPlayer: true}
			}
			f.replacementAmount = e.replacingEvent.Amount
			f.replacementSource = e.protectionSource(e.damaging)
		}
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

// payUnlessDamageCost lands the damage an accepting opponent chose to take
// from Sacrifice's damage-payment offer (Vexing Devil's "any opponent may
// have it deal 4 damage to them"). rules owns payment events, so the Damage
// event is emitted here — never in effects — exactly the split payMana's
// ManaAdd events already follow. The source is the offering permanent (the
// resolving ability object unwrapped to its source, the same rule
// effects.resolveSourceObject applies), published through SetDamageSource so
// the emit-side protection check (CR 702.16d) and DamageDone trigger
// matching see the real source, and the lifelink rider (CR 702.15a) is paid
// for its controller when the hit actually landed (a prevention or other
// replacement that substituted the event pays no life, the same gate
// rules/combat.go's rider uses).
func (e *Engine) payUnlessDamageCost(ctx *effects.Ctx, payer state.PlayerID, n int) {
	if n <= 0 {
		return
	}
	source := ctx.Source
	if o := e.G.Obj(source); o != nil && o.Ability != nil && o.Source != 0 {
		source = o.Source
	}
	prev := e.SetDamageSource(source)
	ev := e.emit(events.Event{Kind: events.Damage, Player: payer, Amount: int32(n)})
	e.SetDamageSource(prev)
	if ev.Kind != events.Damage || !e.HasKeyword(source, "Lifelink") {
		return
	}
	controller := ctx.Controller
	if o := e.G.Obj(source); o != nil && o.Zone == state.ZBattlefield {
		controller = o.Controller
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: controller, Amount: int32(n)})
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
