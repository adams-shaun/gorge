// Re-entry of a resolution body after a rules-side payment window. A
// resolution that runs on the kernel (engine/rules/resolve) never suspends: every
// ask inside it is answered in place from the intent tape. What remains of the
// old suspend/resume machinery is the body re-entry three flows need after
// they have settled their own window in line: a triggered ability's Cost$
// window (kind "effect_paid") and an optional trigger's yes (kind "optional",
// CR 603.5).

package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// finishCopyTargets records a copy's per-declaration answers. The ROOT
// declarations' answers are flattened in declaration order (option 0 replaces,
// the rest append) and published as the per-declaration split for a fused copy,
// so resolveFused hands each half exactly the targets chosen FOR it (the same
// split a cast publishes at payment). Each CHAINED declaration's answer is the
// copy's record for that link (castSubTargets, which resolution consumes line
// by line) and a sub-target TargetsChosen that folds into Object.SubTargets in
// place of the link's inherited entries, the shape a cast's
// installSubPreAsk/recordSubTargets leave.
func (e *Engine) finishCopyTargets(obj state.ObjID, answers [][]decision.Option) {
	rootDecls, chainDecls := e.copyDeclarationSplit(e.G.Obj(obj))
	flat := make([]decision.Option, 0, len(rootDecls))
	stages := make([][]state.Target, len(rootDecls))
	for i, sl := range answers[:len(rootDecls)] {
		flat = append(flat, sl...)
		stages[i] = targetOptions(sl)
	}
	if len(rootDecls) > 0 {
		e.recordChosenTargets(obj, flat, false)
		// An "up to" declaration answered with nothing: the copy keeps none of
		// its inherited targets.
		if o := e.G.Obj(obj); len(flat) == 0 && o != nil && len(o.Targets) > 0 {
			e.emit(events.Event{Kind: events.TargetsChosen, Obj: obj})
		}
	}
	if o := e.G.Obj(obj); o != nil && o.CastFlags&state.FlagFused != 0 {
		if ff, _ := fusedSplitFaces(o); ff != nil {
			if sa := ff.SpellAbility(); sa == nil || !effects.TargetsOf(sa).Targeted() {
				stages = append([][]state.Target{nil}, stages...)
			}
		}
		if e.fuseTargets == nil {
			e.fuseTargets = make(map[state.ObjID][][]state.Target)
		}
		e.fuseTargets[obj] = stages
	}
	if _, ok := e.charmTargets[obj]; ok {
		e.charmTargets[obj] = stages
	}
	for i, sa := range chainDecls {
		ts := append([]state.Target{}, targetOptions(answers[len(rootDecls)+i])...)
		for _, old := range e.castSubTargets[obj][sa.Line] {
			e.removeSubTarget(obj, old)
		}
		e.recordCastSub(obj, sa.Line, ts)
		e.recordSubTargets(obj, ts)
	}
}

// resumeResolution re-enters the body of the stack object rp names and then
// completes the resolution: it rebuilds the Ctx resolveTop built for the
// object on its first pass (Source/Controller/Targets/Remembered and the SVar
// table are all re-derivable from the stack object, which has not moved),
// re-runs rp's ability through effects.Resolve, and finally moves the
// fully-resolved object off the stack as resolveTop's own tail would have.
func (e *Engine) resumeResolution(rp *resumePoint, chosen []decision.Option) {
	savedResolving := e.resolvingObj
	e.resolvingObj = rp.obj
	defer func() { e.resolvingObj = savedResolving }()
	o := e.G.Obj(rp.obj)
	if o == nil || o.Zone != state.ZStack {
		// The object left the stack while its window was open: a no-op, the
		// same totality stance as every other resolution exit.
		return
	}
	var chainRoot *cards.SA
	if o.Ability != nil {
		chainRoot = o.Ability
		e.recheckCastSubTargets(rp.obj, o.Ability, o.Controller, o.Source)
	} else if f := o.Face(); f != nil {
		chainRoot = stackSpellSA(o, f)
		e.recheckCastSubTargets(rp.obj, chainRoot, o.Controller, rp.obj)
	}
	// The list resolveTop resolved this object against (the overload census,
	// the CR 608.2b-filtered set), not the raw recorded Targets.
	targets := e.resolutionTargets.flatFor(rp.obj, o.Targets)
	ctx := effects.NewCtxPtr(rp.obj, o.Controller, effects.CtxInit{Targets: targets})
	// Forge's Count$ResolvedThisTurn: the Resolve event was emitted once, so
	// the tally is unchanged, but a fresh Ctx defaults the field to zero.
	ctx.ResolvedThisTurn = e.resolvedAbilityTally(o)
	ctx.ActivationsThisTurn = e.activationsThisTurnFor(o.Source, o.Ability)
	// alltargeted1: the cast flow's pre-asked sub-ability target answers.
	ctx.SubPreAsk = e.castSubTargets[rp.obj]
	ctx.AllTargets = e.chainTargetUnion(rp.obj, chainRoot, targets)
	ctx.ModeTargets = e.resolutionTargets.modesFor(rp.obj, e.charmTargets[rp.obj])
	// The resolving stack-object wrapper, same anchor resolveTop's branches
	// set (the ValidStack otherAbility exclusion).
	ctx.ResolvingObj = rp.obj
	// Publish this rebuilt Ctx for the whole of the resumed resolution (the
	// same restore-on-return bracket effects.Resolve uses).
	prevCtx := e.SetResolutionCtx(ctx)
	defer e.SetResolutionCtx(prevCtx)
	// Cost-sacrificed objects are engine-only LKI keyed by the stack object.
	ctx.Sacrificed = e.sacrificedLKI[rp.obj]
	ctx.Paid.Exiled = e.castExiled[rp.obj]
	ctx.Paid.Revealed = e.castRevealed[rp.obj]
	ctx.Paid.Discarded = e.castDiscarded[rp.obj]
	ctx.Paid.CounterPut = e.castCounterPut[rp.obj]
	ctx.Paid.Tapped = e.castTapped[rp.obj]
	ctx.Paid.Returned = e.castReturned[rp.obj]
	// CR 107.3i: X is the value paid for the object's {X}, preserved on the
	// stack object by CastInfo; CR 107.3m binds a trigger's X to the causing
	// event's card.
	ctx.X = o.X
	ctx.XAnnounced = stackXAnnounced(o)
	if ctx.X == 0 {
		ctx.X = e.triggerPaidX(rp.obj, o)
	}
	// The triggered-cost window's dynamic tapXType<X/Spec> payment (the
	// Battlesphere/yotia shape): the election's tap count is the cost's
	// announced X. It wins over both reads above.
	if rp.tapPaidX != 0 {
		ctx.X = rp.tapPaidX
		ctx.XAnnounced = true
	}
	// The trigger-cost window's X fold (the {X}/{PayLife<X>} announcement):
	// the announced or fixed value binds exactly like the dyn-tap count.
	if rp.winPaidX != 0 || rp.winXBound {
		ctx.X = rp.winPaidX
		ctx.XAnnounced = true
	}
	var svars map[string]string
	if o.Ability != nil {
		// A triggered or activated ability: mirror resolveTop's ability
		// branch -- Source is the source permanent, Remembered carries what
		// the trigger captured, and the SVar table comes from that
		// permanent's face.
		ctx.Source = o.Source
		ctx.TriggerContext = e.triggerContexts[rp.obj]
		ctx.Remembered = e.resolvingRemembered(o)
		ctx.Captured = e.resolvingCaptured(o)
		ctx.Grantor = o.GrantedBy
		bindNinjutsuDefender(ctx, o)
		e.bindStationAmount(ctx, o)
		if abilityXAnnounced(o) {
			ctx.XAnnounced = true
		}
		reflexiveCaptured(ctx, e.reflexiveRememberedPT[rp.obj])
		if rp.kind == "optional" || rp.kind == "effect_paid" {
			// CR 400.7: the CR 603.5 yes and a triggered ability's Cost$
			// window both resume before any of the body ran, so this is the
			// resolution-start list resolveTop computes (Emiel the Blessed's
			// paid counter trigger for a creature blinked in response).
			ctx.TriggerRememberedMoved = e.rememberedMovedSinceFire(ctx.TriggerRememberedStamps)
		}
		lki, lkiHeld := e.triggerLKI[rp.obj]
		bindTriggerObjectLKI(ctx, lki, lkiHeld)
		if link, ok := e.sourceLifelinkLKI[rp.obj]; ok {
			ctx.Snap.SourceLifelink = link
			ctx.Snap.SourceLifelinkValid = true
		}
		if controller, ok := e.sourceControllerLKI[rp.obj]; ok {
			ctx.Snap.SourceController = controller
			ctx.Snap.SourceControllerValid = true
		}
		if lki := e.damageSourceLKI[rp.obj]; lki != nil {
			ctx.Snap.DamageSource = cloneDamageSourceLKI(lki)
		}
		// CR 603.3c: keep the placement-announced mode choice.
		ctx.Modes = resumeChosenModes(rp, o)
		// The same table resolveTop's first pass read: the owning face of a
		// pile's under-card ability, and above all a granted or delayed
		// trigger's recorded line table.
		svars = e.abilityResolutionSVars(rp.obj, o)
	} else if f := o.Face(); f != nil {
		svars = f.SVars
	}
	effects.SetSVars(ctx, svars)
	// Carry the SA whose targeting the placement/announcement ask
	// covered, exactly as resolveTop's first pass does.
	if offeredSA := offeredTargetSA(o, svars); offeredSA != nil {
		ctx.OfferedSA = offeredSA
	}
	// An accepted optional trigger may itself carry Cost$ (Mana Vault's "you
	// may pay {4}; if you do" untap). The optional answer chooses to attempt
	// the effect; payment is a separate resolution-time window.
	tc := e.triggerContexts[rp.obj]
	armed := rp.kind == "optional" && rp.sa != nil &&
		(e.triggerBodyNeedsCostWindow(rp.sa) ||
			// An OptionalDecider$ copy trigger's AB$ CopySpellAbility
			// with a real Cost$ pays through the same window whenever the
			// trigger context carries an event role.
			(rp.sa.API == "CopySpellAbility" && rp.sa.ParamStr(cards.PKCost) != "" &&
				(tc.TriggerAbility != 0 || tc.TriggerCard != 0)))
	if armed {
		e.startTriggeredEffectCost(rp, ctx.Source)
		return
	}
	if rp.sa != nil {
		// Cross-mode TargetUnique attribution: a frame whose SA is one of the
		// chosen target-bearing modes' chains re-enters with that mode's own
		// target.
		if one := e.charmModeTarget(rp.obj, rp.sa); one != nil {
			ctx.Targets = one
		}
		src := rp.obj
		if o.Ability != nil {
			src = o.Source
		}
		e.damaging = src
		effects.Resolve(e, ctx, rp.sa)
		e.damaging = 0
		if e.restart != nil {
			e.finishRestart()
			return
		}
		// A resumed EndTurn has already exiled the stack, including the
		// resolving ability: enter cleanup just as resolveTop does.
		if e.endTurnRequested {
			e.finishEndTurn()
			e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
			return
		}
		if e.endCombatRequested {
			e.finishEndCombatPhase()
			e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
			return
		}
	}
	e.finishResumption(rp.obj)
	// CR 117.3b / the counterpart of handlePriority's pass-branch grant: when
	// a resolution that ran a window completes, the pass count resets and
	// priority returns to the active player.
	e.emit(events.Event{Kind: events.Priority, Player: e.G.Active})
}
