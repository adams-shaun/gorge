package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// castDivisionAPI names the primitives whose DividedAsYouChoose$ division is
// announced as the spell or ability is put on the stack (CR 601.2d) and read
// back at resolution (effects.AnnouncedDivision). A divided primitive outside
// it keeps its resolution-time division.
func castDivisionAPI(sa *cards.SA) bool {
	return sa != nil && sa.API == "DealDamage"
}

// divisionTotalFor resolves the total a targeted castDivisionAPI SA divides
// (DividedAsYouChoose$), for player's announcement of source's targets with
// the announced x. ok is false when sa announces no division: not a
// castDivisionAPI primitive, untargeted (CR 608.2d: an untargeted division is
// made at resolution), no DividedAsYouChoose$, or a total the numeric grammar
// cannot resolve yet. A negative total is 0.
func (e *Engine) divisionTotalFor(sa *cards.SA, player state.PlayerID, source state.ObjID, x int32) (int32, bool) {
	if !castDivisionAPI(sa) || !effects.TargetsOf(sa).Targeted() {
		return 0, false
	}
	p, ok := effects.DividedParamText(sa)
	if !ok {
		return 0, false
	}
	ctx, ok := e.targetBoundCtx(player, source)
	if !ok {
		return 0, false
	}
	ctx.X = x
	n, resolved := effects.NumTextResolvedStrict(e, ctx, p, 0)
	if !resolved {
		return 0, false
	}
	if n < 0 {
		n = 0
	}
	return n, true
}

// castDivisionTotal is divisionTotalFor for the pending cast's root target
// declaration sa. A Fuse half, a face-down cast and a modal mode announce no
// division here.
func (e *Engine) castDivisionTotal(pc *pendingCast, sa *cards.SA) (int32, bool) {
	if pc == nil || pc.faceDown || pc.mode == "fuse" {
		return 0, false
	}
	o := e.G.Obj(pc.card)
	if o == nil || (!pc.isAbility() && len(o.ChosenModes) > 0) {
		return 0, false
	}
	return e.divisionTotalFor(sa, pc.player, pc.card, pc.x)
}

// floorDivision is the division with every target at its CR 601.2d floor of
// 1 and the first target holding the rest: the only legal division when
// n == 1 or total == n, and the base the division ask's picks add to.
func floorDivision(n int, total int32) []int32 {
	shares := make([]int32, n)
	for i := range shares {
		shares[i] = 1
	}
	if n > 0 {
		shares[0] += total - int32(n)
	}
	return shares
}

// divisionDecision is the CR 601.2d division ask over targets: each target
// receives at least 1, so with n targets and a total T the only free choice
// is where the T-n remaining points go -- one KChoose option per target in
// announcement order (Kind "divide"), Min == Max == T-n, Repeatable, an
// option's multiplicity being its target's share above 1.
func (e *Engine) divisionDecision(player state.PlayerID, source state.ObjID, targets []state.Target, total int32) *decision.Decision {
	// The prompt carries no number: Min is the count left to place, and the
	// strings ratchet keeps strconv out of engine/rules.
	excess := int(total) - len(targets)
	d := &decision.Decision{Player: player, Kind: decision.KChoose, Min: excess, Max: excess,
		Repeatable: true, Source: source,
		Prompt: "Divide among the targets of " + e.targetName(source) +
			": each takes 1; choose where each remaining point goes"}
	for i, t := range targets {
		opt := decision.Option{Index: i, Kind: "divide"}
		if t.IsPlayer {
			opt.Player = t.Player
			opt.Label = seatFacingName(e.G, t.Player)
		} else {
			opt.Obj = t.Obj
			opt.Label = e.targetName(t.Obj)
		}
		d.Options = append(d.Options, opt)
	}
	return d
}

// divisionAnswer folds a divisionDecision answer over n targets.
func divisionAnswer(n int, chosen []decision.Option) []int32 {
	shares := floorDivision(n, int32(n))
	for _, o := range chosen {
		if o.Index >= 0 && o.Index < n {
			shares[o.Index]++
		}
	}
	return shares
}

// emitDivision records shares onto the stack object obj
// (events.Choose "divide"), after obj's root targets are recorded.
func (e *Engine) emitDivision(obj state.ObjID, player state.PlayerID, shares []int32) {
	e.emit(events.Event{Kind: events.Choose, Obj: obj, Player: player,
		Counter: events.DivisionCounter, Text: events.FormatDivision(shares)})
}

// divisionAsk announces the CR 601.2d division after the targets (601.2c,
// the chain links' included) and before any cost is paid. One target, or a
// total equal to the target count, has exactly one legal division and is
// filled without an ask. It returns true when it posed the ask.
func (e *Engine) divisionAsk(pc *pendingCast) bool {
	if pc.divisionDone {
		return false
	}
	if pc.division == nil {
		o := e.G.Obj(pc.card)
		if o == nil {
			pc.divisionDone = true
			return false
		}
		total, ok := e.castDivisionTotal(pc, e.castStageSA(pc, o, o.Face()))
		n := len(pc.targets)
		if !ok || n == 0 || total < int32(n) {
			// Nothing to divide, or (unreachable past targetAsk's CR 601.2d
			// clamp) fewer points than targets: no announcement, so the
			// resolution-time path applies.
			pc.divisionDone = true
			return false
		}
		if n > 1 && total > int32(n) {
			e.choosing = chooseCast
			e.ask(e.divisionDecision(pc.player, pc.card, pc.targets, total))
			return true
		}
		pc.division = floorDivision(n, total)
	}
	pc.divisionDone = true
	if !pc.isAbility() {
		e.recordCastDivision(pc)
	}
	return false
}

// answerCastDivision folds the divisionAsk answer and completes the cast
// exactly as a target answer does: the remaining post-target asks, then
// finishTargetedCast.
func (e *Engine) answerCastDivision(pc *pendingCast, chosen []decision.Option) {
	pc.division = divisionAnswer(len(pc.targets), chosen)
	if e.postTargetAsks(pc) {
		return
	}
	e.finishTargetedCast(pc, pc.player)
}

// recordCastDivision emits the settled division onto the stack object once:
// at the announcement for a spell (already on the stack), after payCast's
// AbilityPush for an activated ability.
func (e *Engine) recordCastDivision(pc *pendingCast) {
	if pc.divisionRecorded || pc.division == nil || pc.stackObj == 0 {
		return
	}
	pc.divisionRecorded = true
	e.emitDivision(pc.stackObj, pc.player, pc.division)
}
