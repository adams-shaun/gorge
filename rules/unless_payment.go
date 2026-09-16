package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// unlessPayment is the continuation between accepting an UnlessCost$ and
// completing its non-mana Sac/Discard components. Unlike ordinary activation
// costs, an unless cost is paid during a suspended resolution, so its choices
// must retain that resolution rather than silently taking the first card.
type unlessPayment struct {
	payer    state.PlayerID
	cost     Cost
	ctx      effects.Ctx
	stackObj state.ObjID
	// rp is non-nil for the ordinary stack-backed path. A nil rp belongs to
	// an activated mana ability, whose continuation remains in
	// manaUnlessActivation.
	rp       *resumePoint
	part     int
	sacs     []state.ObjID
	discards []state.ObjID
}

func cloneUnlessCtx(in effects.Ctx) effects.Ctx {
	out := in
	out.Targets = append([]state.Target(nil), in.Targets...)
	out.Remembered = append([]state.Target(nil), in.Remembered...)
	out.Captured = append([]state.Target(nil), in.Captured...)
	out.Chosen = append([]state.Target(nil), in.Chosen...)
	if in.SVars != nil {
		out.SVars = make(map[string]string, len(in.SVars))
		for k, v := range in.SVars {
			out.SVars[k] = v
		}
	}
	return out
}

// beginUnlessPayment begins a payer-selected payment. It owns all Sac and
// Discard components, including the exact-candidate no-ask cases, so neither
// this path nor a future sibling can fall back to a first-in-zone-order pick.
func (e *Engine) beginUnlessPayment(payer state.PlayerID, cost Cost, ctx *effects.Ctx, stackObj state.ObjID, rp *resumePoint) {
	e.unlessPayment = &unlessPayment{payer: payer, cost: cost, ctx: cloneUnlessCtx(*ctx), stackObj: stackObj, rp: rp}
	e.advanceUnlessPayment()
}

func (e *Engine) advanceUnlessPayment() {
	u := e.unlessPayment
	if u == nil {
		return
	}
	if int(u.payer) < 0 || int(u.payer) >= len(e.G.Players) ||
		!u.cost.payable(e.G.Players[u.payer].Pool, e.G.Players[u.payer].Snow, e.G.Players[u.payer].Life) ||
		!e.unlessCountersAffordable(u) {
		e.finishUnlessPayment(false)
		return
	}
	for u.part < len(u.cost.Sac)+len(u.cost.Discard) {
		zone, kind := state.ZBattlefield, "sacrifice"
		var part CostPart
		if u.part < len(u.cost.Sac) {
			part = u.cost.Sac[u.part]
		} else {
			zone, kind = state.ZHand, "discard"
			part = u.cost.Discard[u.part-len(u.cost.Sac)]
		}
		eligible := e.unlessPaymentCandidates(u, zone, part)
		if int32(len(eligible)) < part.N {
			e.finishUnlessPayment(false)
			return
		}
		if int32(len(eligible)) == part.N {
			e.recordUnlessPaymentPick(u, zone, eligible)
			u.part++
			continue
		}
		d := &decision.Decision{Player: u.payer, Kind: decision.KChoose,
			Min: int(part.N), Max: int(part.N), Source: u.ctx.Source,
			ResumeKind: "unless_cost",
			Prompt:     fmt.Sprintf("Choose %d card(s) to %s to pay the cost", part.N, kind)}
		for _, id := range eligible {
			label := "a card"
			if o := e.G.Obj(id); o != nil && o.Face() != nil {
				label = o.Face().Name
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: kind,
				Label: label, Obj: id, Player: u.payer})
		}
		e.choosing = chooseUnlessCost
		e.ask(d)
		return
	}
	// Resolve every drawer before charging any component. A Draw<N/Spec> may
	// name a role that this suspended resolution did not retain; that is an
	// unpayable cost, not a reason to spend mana/life and then silently omit
	// the draw. Keeping the resolved players also makes the selected-cost path
	// use the same binding rules as payUnlessCost's no-choice path.
	drawers := make([][]state.PlayerID, len(u.cost.Draw))
	for i, part := range u.cost.Draw {
		players, ok := unlessDrawPlayers(&u.ctx, u.payer, part.Spec)
		if !ok {
			e.finishUnlessPayment(false)
			return
		}
		for _, p := range players {
			if int(p) < 0 || int(p) >= len(e.G.Players) {
				e.finishUnlessPayment(false)
				return
			}
		}
		drawers[i] = players
	}
	if !e.payMana(u.payer, u.cost) { // guarded above; retain totality if state changes.
		e.finishUnlessPayment(false)
		return
	}
	for _, id := range u.sacs {
		e.emit(events.Sacrifice(id))
	}
	for _, id := range u.discards {
		e.emit(events.Discard(id, u.payer))
	}
	src := u.ctx.Source
	if o := e.G.Obj(u.stackObj); o != nil && o.Ability != nil {
		src = o.Source
	}
	for _, part := range u.cost.SubCounter {
		e.emit(events.Event{Kind: events.CounterChange, Obj: src, Counter: part.Spec, Amount: -part.N})
	}
	for i, part := range u.cost.Draw {
		for _, p := range drawers[i] {
			for n := int32(0); n < part.N; n++ {
				effects.DrawFor(e, p)
			}
		}
	}
	e.finishUnlessPayment(true)
}

func (e *Engine) unlessPaymentCandidates(u *unlessPayment, zone state.Zone, part CostPart) []state.ObjID {
	used := u.sacs
	if zone == state.ZHand {
		used = u.discards
	}
	seen := make(map[state.ObjID]bool, len(used))
	for _, id := range used {
		seen[id] = true
	}
	sc := u.ctx.SpecContext(u.payer)
	var out []state.ObjID
	for _, id := range e.G.Zone(zone, u.payer) {
		if !seen[id] && effects.MatchesSpecCtx(e.G, part.Spec, id, sc) {
			out = append(out, id)
		}
	}
	return out
}

func (e *Engine) recordUnlessPaymentPick(u *unlessPayment, zone state.Zone, ids []state.ObjID) {
	if zone == state.ZBattlefield {
		u.sacs = append(u.sacs, ids...)
	} else {
		u.discards = append(u.discards, ids...)
	}
}

func (e *Engine) unlessCountersAffordable(u *unlessPayment) bool {
	src := u.ctx.Source
	if o := e.G.Obj(u.stackObj); o != nil && o.Ability != nil {
		src = o.Source
	}
	o := e.G.Obj(src)
	if o == nil && len(u.cost.SubCounter) > 0 {
		return false
	}
	need := make(map[string]int32, len(u.cost.SubCounter))
	for _, part := range u.cost.SubCounter {
		need[part.Spec] += part.N
	}
	for kind, n := range need {
		have := int32(0)
		for _, c := range o.Counters {
			if c.Kind == kind {
				have += c.N
			}
		}
		if have < n {
			return false
		}
	}
	return true
}

func (e *Engine) answerUnlessPayment(chosen []decision.Option) {
	u := e.unlessPayment
	if u == nil || u.part >= len(u.cost.Sac)+len(u.cost.Discard) {
		return
	}
	zone := state.ZBattlefield
	if u.part >= len(u.cost.Sac) {
		zone = state.ZHand
	}
	ids := make([]state.ObjID, 0, len(chosen))
	for _, o := range chosen {
		if o.Obj != 0 {
			ids = append(ids, o.Obj)
		}
	}
	// Decision.Validate guaranteed the count and offered identity. Recheck the
	// zone/filter against the current game before any event is emitted so a
	// malformed resumed state declines rather than paying an illegal cost.
	var part CostPart
	if zone == state.ZBattlefield {
		part = u.cost.Sac[u.part]
	} else {
		part = u.cost.Discard[u.part-len(u.cost.Sac)]
	}
	eligible := e.unlessPaymentCandidates(u, zone, part)
	allowed := make(map[state.ObjID]bool, len(eligible))
	for _, id := range eligible {
		allowed[id] = true
	}
	if int32(len(ids)) != part.N {
		e.finishUnlessPayment(false)
		return
	}
	for _, id := range ids {
		if !allowed[id] {
			e.finishUnlessPayment(false)
			return
		}
	}
	e.recordUnlessPaymentPick(u, zone, ids)
	u.part++
	e.choosing = chooseNone
	e.advanceUnlessPayment()
}

func (e *Engine) finishUnlessPayment(paid bool) {
	u := e.unlessPayment
	if u == nil {
		return
	}
	e.unlessPayment = nil
	e.choosing = chooseNone
	if u.rp != nil {
		if paid {
			u.rp.unlessPay = "pay"
		} else {
			u.rp.unlessPay = "decline"
		}
		e.resumeResolution(u.rp, nil)
		return
	}
	e.finishManaUnlessPayment(paid)
}
