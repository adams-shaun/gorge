package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Splice onto Arcane (CR 702.47), `K:Splice:<quality>:<cost>` on a card in the
// caster's hand.
//
//   - spliceAsk poses ONE KChoose after the mode announcement and before
//     targets (CR 601.2b): any subset of the eligible hand cards and its
//     resolution order. Each chosen card's splice cost is an additional cost
//     (702.47a, 601.2f-h) folded into pc.cost by answerSplice, and the card is
//     revealed (a public Note) and stays in hand (702.47a/e).
//   - Targets (601.2c, 702.47d): the spell announces its own text first
//     (target stage 0), then each spliced card's root ability in chosen order
//     (stage N asks pc.spliced[N-1]); castStageSA / castHasNextTargetStage
//     are the Fuse stage machinery's splice arm.
//   - Resolution (702.47b/c): resolveTop runs the spell's own text, then
//     runSplicedSegments runs each spliced text in chosen order against the
//     targets chosen for it (Engine.spliceFor / spliceTargets).
//
// Out of scope: splicing onto a fused spell, spliced modal text, a targeting
// SubAbility$ on spliced text (none of the 30 corpus carriers has one), other
// qualities than a type word list, an X in a splice cost.
func init() { effects.RegisterNonAPI("kw:Splice") }

// spliceQualityMatches reports whether the spell face has any type word of the
// comma-separated quality ("Arcane" or "Instant,Sorcery").
func spliceQualityMatches(spell *cards.Face, quality string) bool {
	if spell == nil {
		return false
	}
	for _, w := range strings.Split(quality, ",") {
		if w = strings.TrimSpace(w); w != "" && spell.HasTypeWord(w) {
			return true
		}
	}
	return false
}

// spliceOffered reports whether hand card id may be offered: its quality
// matches the spell, its splice cost is payable on top of the current total,
// and its text -- when it needs a target -- has a legal one (CR 702.47b: the
// spell would otherwise carry a mandatory target nothing can satisfy).
func (e *Engine) spliceOffered(pc *pendingCast, spell *cards.Face, id state.ObjID) (cost string, ok bool) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		return "", false
	}
	quality, cost, has := o.Face().Splice()
	if !has || !spliceQualityMatches(spell, quality) {
		return "", false
	}
	if !e.altAddPartPayable(pc, ParseCost(cost)) {
		return "", false
	}
	if sa := modalTargetSA(o.Face(), o.Face().SpellAbility(), nil); sa != nil {
		if tp := effects.TargetsOf(sa); tp.Targeted() && tp.BoundMin > 0 &&
			len(e.castTargetCandidates(pc, pc.card, sa)) == 0 {
			return "", false
		}
	}
	return cost, true
}

// spliceAsk poses the splice selection (CR 702.47a) and reports whether it
// parked the flow on a decision. One-shot (pc.spliceDone, set first).
func (e *Engine) spliceAsk() bool {
	pc := e.cast
	if pc == nil || pc.spliceDone || pc.isAbility() || pc.faceDown || pc.mode == "fuse" {
		return false
	}
	pc.spliceDone = true
	o := e.G.Obj(pc.card)
	if o == nil || o.Face() == nil {
		return false
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 0,
		AllowNone: true, Prompt: "Choose cards to splice onto " + e.targetName(pc.card), Source: pc.card}
	for _, id := range e.G.Zone(state.ZHand, pc.player) {
		if id == pc.card {
			continue
		}
		cost, ok := e.spliceOffered(pc, o.Face(), id)
		if !ok {
			continue
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "splice", Obj: id,
			Label: e.targetName(id) + " — splice " + costPhrase(ParseCost(cost))})
	}
	if len(d.Options) == 0 {
		return false
	}
	d.Max = len(d.Options)
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// answerSplice records the answer, in answer order: the cards join pc.spliced,
// each splice cost folds into pc.cost and the card is revealed. A card whose
// cost no longer fits on top of the running total (two options each payable
// alone but not together) is dropped, so the cast never charges an unpayable
// total.
func (e *Engine) answerSplice(pc *pendingCast, chosen []decision.Option) {
	for _, opt := range chosen {
		o := e.G.Obj(opt.Obj)
		if o == nil || o.Face() == nil || slicesContainsID(pc.spliced, opt.Obj) {
			continue
		}
		_, cost, has := o.Face().Splice()
		if !has {
			continue
		}
		part := ParseCost(cost)
		if !e.altAddPartPayable(pc, part) {
			continue
		}
		pc.spliced = append(pc.spliced, opt.Obj)
		pc.cost = pc.cost.Plus(part)
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Obj: opt.Obj, Text: "revealed for splice"})
	}
	pc.spliceTargets = make([][]state.Target, len(pc.spliced))
}

func slicesContainsID(ids []state.ObjID, id state.ObjID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// spliceStageSA is the root ability of the card spliced at target stage
// pc.targetStage (>= 1).
func (e *Engine) spliceStageSA(spliced []state.ObjID, stage int) *cards.SA {
	if stage < 1 || stage > len(spliced) {
		return nil
	}
	o := e.G.Obj(spliced[stage-1])
	if o == nil || o.Face() == nil {
		return nil
	}
	return modalTargetSA(o.Face(), o.Face().SpellAbility(), nil)
}

// castBaseSA is castStageSA at stage 0: the spell's own ability. The
// post-target asks (sub-ability chain, division, sub counters) work on the
// spell's own text even when a splice stage was the last one asked.
func (e *Engine) castBaseSA(pc *pendingCast, o *state.Object, f *cards.Face) *cards.SA {
	if pc.targetStage == 0 || len(pc.spliced) == 0 {
		return e.castStageSA(pc, o, f)
	}
	saved := pc.targetStage
	pc.targetStage = 0
	defer func() { pc.targetStage = saved }()
	return e.castStageSA(pc, o, f)
}

// targetAskThroughStages is targetAsk plus the stage walk: a stage that asks
// nothing (a zero-minimum target with no candidate) must not hide a later
// splice stage that does.
func (e *Engine) targetAskThroughStages() bool {
	if e.targetAsk() {
		return true
	}
	for pc := e.cast; pc != nil && len(pc.spliced) > 0 && !pc.passedTarget && !pc.faceDown; {
		if !e.castHasNextTargetStage(pc, e.G.Obj(pc.card)) {
			return false
		}
		pc.targetStage++
		if e.targetAsk() {
			return true
		}
	}
	return false
}

// publishSplice records the splice provenance for resolution (payCast).
func (e *Engine) publishSplice(pc *pendingCast) {
	if len(pc.spliced) == 0 {
		return
	}
	if e.spliceFor == nil {
		e.spliceFor = make(map[state.ObjID][]state.ObjID)
		e.spliceTargets = make(map[state.ObjID][][]state.Target)
	}
	e.spliceFor[pc.stackObj] = pc.spliced
	e.spliceTargets[pc.stackObj] = pc.spliceTargets
}

// spliceSegment is one spliced text at resolution: the card, its root ability
// and the targets still legal for it.
type spliceSegment struct {
	id    state.ObjID
	sa    *cards.SA
	legal []state.Target
}

// spliceLegal rechecks every spliced text's targets (CR 608.2b) and reports
// the segments in chosen order, whether any spliced target was chosen at all
// and whether any of them is still legal.
func (e *Engine) spliceLegal(o *state.Object) (segs []spliceSegment, chosen, alive bool) {
	ids := e.spliceFor[o.ID]
	for i, id := range ids {
		so := e.G.Obj(id)
		if so == nil || so.Face() == nil {
			continue
		}
		sa := modalTargetSA(so.Face(), so.Face().SpellAbility(), nil)
		seg := spliceSegment{id: id, sa: sa}
		var own []state.Target
		if i < len(e.spliceTargets[o.ID]) {
			own = e.spliceTargets[o.ID][i]
		}
		if sa != nil && effects.TargetsOf(sa).ValidTgts != "" {
			seg.legal = e.legalTargets(own, sa, targetZones(sa), o.Controller, o.ID, o.ID)
			if len(own) > 0 {
				chosen = true
			}
			if len(seg.legal) > 0 {
				alive = true
			}
		}
		segs = append(segs, seg)
	}
	return segs, chosen, alive
}

// runSplicedSegments resolves each spliced text after the spell's own, in the
// chosen order (CR 702.47c), each against its own targets, with the spell as
// the source (702.47d): the Ctx mirrors runFusedHalves'.
func (e *Engine) runSplicedSegments(o *state.Object, segs []spliceSegment) {
	for _, seg := range segs {
		if seg.sa == nil {
			continue
		}
		so := e.G.Obj(seg.id)
		if so == nil || so.Face() == nil {
			continue
		}
		e.damaging = o.ID
		ctx := effects.NewCtxPtr(o.ID, o.Controller, effects.CtxInit{Targets: seg.legal})
		ctx.ResolvingObj = o.ID
		if effects.TargetsOf(seg.sa).Targeted() {
			ctx.TargetsOffered = true
			ctx.OfferedSA = seg.sa
		}
		ctx.X = o.X
		ctx.Sacrificed = e.sacrificedLKI[o.ID]
		ctx.Paid.Exiled = e.castExiled[o.ID]
		ctx.Paid.Revealed = e.castRevealed[o.ID]
		ctx.Paid.Discarded = e.castDiscarded[o.ID]
		ctx.Paid.CounterPut = e.castCounterPut[o.ID]
		ctx.Paid.Tapped = e.castTapped[o.ID]
		ctx.Paid.Returned = e.castReturned[o.ID]
		effects.SetSVars(ctx, so.Face().SVars)
		effects.Resolve(e, ctx, seg.sa)
		e.damaging = 0
	}
}

// fizzleSplicedSpell is the CR 608.2b counter for a spell whose every target
// (all of them spliced text's) became illegal.
func (e *Engine) fizzleSplicedSpell(o *state.Object) {
	rest := spellFizzleZone(o)
	e.stackLeave = stackLeaveFizzled
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZStack, To: rest,
		Text: "fizzled: no legal targets remain"})
	e.stackLeave = stackLeaveNone
	e.ensureLeftTheStack(o.ID, rest, "all spliced targets became illegal")
}

// spliceMainTargets is the spell's own slice of its flat target list: the
// stages append in order (the spell's own first), so dropping the spliced
// stages' share off the end leaves the spell's own targets.
func (e *Engine) spliceMainTargets(o *state.Object, flat []state.Target) []state.Target {
	n := 0
	for _, ts := range e.spliceTargets[o.ID] {
		n += len(ts)
	}
	if n > len(flat) {
		return flat
	}
	return flat[:len(flat)-n]
}
