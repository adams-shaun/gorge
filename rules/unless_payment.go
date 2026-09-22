package rules

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// unlessPayment is the continuation between accepting an UnlessCost$ and
// completing its non-mana Sac/Discard/Reveal components. Unlike ordinary
// activation costs, an unless cost is paid during a suspended resolution, so
// its choices must retain that resolution rather than silently taking the
// first card.
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
	// reveals holds the Reveal<N/Spec> picks (the hideaway-family ETB
	// lands, Xyru Specter's Challenge): unlike a discard the revealed cards
	// STAY in hand, so the dedup must be explicit — one card must not pay
	// two parts — and the settled picks are announced with one public Note
	// (the same event the cast flow's emitChoiceCosts emits).
	reveals []state.ObjID
	// manaWindow keeps the resolution suspended while the payer activates
	// mana abilities for this unless cost.
	manaWindow bool
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

// beginUnlessPayment begins a payer-selected payment. It owns all Sac,
// Discard and Reveal components, including the exact-candidate no-ask cases,
// so neither this path nor a future sibling can fall back to a
// first-in-zone-order pick.
// UnlessCostPayable is the rules-side offer gate for the generic unless
// election. It includes the CR 601.2g payment reach: floating mana or a
// reachable combination of currently usable mana sources. Merely having one
// source is not enough: a {3} tax with one Island, or a {R} tax with only an
// Island, must not offer a pay branch.
func (e *Engine) UnlessCostPayable(p state.PlayerID, raw string) bool {
	cost, ok := ParseUnlessCost(raw)
	if !ok {
		return true
	}
	player := e.G.Players[p]
	if cost.payable(player.Pool, player.Snow, player.TypedMana, player.Life) {
		return true
	}
	if !cost.hasManaPayment() {
		return false
	}
	// AvailableMana is deliberately conservative: it includes only free,
	// fixed-producing, singleton mana abilities, but it is exact for the
	// ordinary land sources this payment window can activate. This keeps the
	// offer gate from promising a colour or amount that the window cannot
	// actually produce.
	available := e.AvailableMana(p)
	pool := player.Pool
	for i, n := range available {
		pool[i] += n
	}
	return cost.payable(pool, player.Snow, player.TypedMana, player.Life)
}

func (e *Engine) beginUnlessPayment(payer state.PlayerID, cost Cost, ctx *effects.Ctx, stackObj state.ObjID, rp *resumePoint) {
	e.unlessPayment = &unlessPayment{payer: payer, cost: cost, ctx: cloneUnlessCtx(*ctx), stackObj: stackObj, rp: rp}
	e.advanceUnlessPayment()
}

// unlessPartAt returns the cost component at flat index i across the
// unlessPayment's Sac, Discard and Reveal lists, together with the zone its
// candidates come from and the option kind the wire carries. The option kind
// "revealcost" is the cast flow's own reveal-cost string (rules/cast.go), so
// a client sees the same vocabulary for both paths.
func unlessPartAt(cost Cost, i int) (CostPart, state.Zone, string) {
	nSac, nDisc := len(cost.Sac), len(cost.Discard)
	switch {
	case i < nSac:
		return cost.Sac[i], state.ZBattlefield, "sacrifice"
	case i < nSac+nDisc:
		return cost.Discard[i-nSac], state.ZHand, "discard"
	default:
		return cost.Reveal[i-nSac-nDisc], state.ZHand, "revealcost"
	}
}

// paymentPartCount is the flat count of the choice-bearing components
// (Sac, Discard, Reveal) the continuation walks before it settles the
// synchronous ones.
func (u *unlessPayment) paymentPartCount() int {
	return len(u.cost.Sac) + len(u.cost.Discard) + len(u.cost.Reveal)
}

func (e *Engine) advanceUnlessPayment() {
	u := e.unlessPayment
	if u == nil {
		return
	}
	if int(u.payer) < 0 || int(u.payer) >= len(e.G.Players) ||
		(!u.manaWindow && !u.cost.payable(e.G.Players[u.payer].Pool, e.G.Players[u.payer].Snow, e.G.Players[u.payer].TypedMana, e.G.Players[u.payer].Life) &&
			!(u.cost.hasManaPayment() && e.hasUntappedManaSource(u.payer))) ||
		!e.unlessCountersAffordable(u) ||
		!u.revealChosenDesignated(e) {
		e.finishUnlessPayment(false)
		return
	}
	for u.part < u.paymentPartCount() {
		part, zone, kind := unlessPartAt(u.cost, u.part)
		eligible := e.unlessPaymentCandidates(u, zone, kind, part)
		if int32(len(eligible)) < part.N {
			e.finishUnlessPayment(false)
			return
		}
		if int32(len(eligible)) == part.N {
			e.recordUnlessPaymentPick(u, kind, eligible)
			u.part++
			continue
		}
		prompt := fmt.Sprintf("Choose %d card(s) to %s to pay the cost", part.N, kind)
		if kind == "revealcost" {
			prompt = fmt.Sprintf("Choose %d card(s) to reveal to pay the cost", part.N)
		}
		d := &decision.Decision{Player: u.payer, Kind: decision.KChoose,
			Min: int(part.N), Max: int(part.N), Source: u.ctx.Source,
			ResumeKind: "unless_cost",
			Prompt:     prompt}
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
	if !e.payMana(u.payer, u.cost) {
		// Assemble floating mana one source at a time, as in Ward and the
		// cast payment window, rather than treating an empty pool as a decline.
		if u.cost.hasManaPayment() && e.hasUntappedManaSource(u.payer) {
			u.manaWindow = true
			e.askUnlessMana()
			return
		}
		e.finishUnlessPayment(false)
		return
	}
	// The settled reveal picks are announced exactly like the cast flow's
	// emitChoiceCosts announces them: one public Note carrying the revealed
	// ids (the cards STAY in hand), emitted only on the paid path. The
	// RevealChosen parts have no ids -- they make the source's secret
	// designation public with one Note too, the same call emitChoiceCosts
	// makes.
	if len(u.reveals) > 0 {
		names := make([]string, 0, len(u.reveals))
		for _, id := range u.reveals {
			names = append(names, e.targetName(id))
		}
		e.emit(events.Event{Kind: events.Note, Player: u.payer, Obj: u.ctx.Source,
			IDs:  append([]state.ObjID(nil), u.reveals...),
			Text: "revealed " + strings.Join(names, ", ") + " as a cost"})
	}
	for _, part := range u.cost.RevealChosen {
		if text, ok := revealChosenText(e.G, e.G.Obj(u.ctx.Source), part.Spec); ok {
			e.emit(events.Event{Kind: events.Note, Player: u.payer, Obj: u.ctx.Source, Text: text})
		}
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

func (e *Engine) askUnlessMana() {
	u := e.unlessPayment
	if u == nil {
		return
	}
	d := &decision.Decision{Player: u.payer, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Activate mana abilities to pay the unless cost", ResumeKind: "unless_mana"}
	for _, id := range e.G.Zone(state.ZBattlefield, u.payer) {
		if e.untappedManaSource(u.payer, id) {
			o := e.G.Obj(id)
			label := "Tap a mana source"
			if o != nil && o.Face() != nil {
				label = "Tap " + o.Face().Name + " for mana"
			}
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "activate", Obj: id, Label: label})
		}
	}
	// Done is intentionally always legal: it lets the payer settle the cost
	// after the last activation, and is the R-9 no-host-compatible decline when
	// no payment can actually be completed.
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "done", Label: "Done"})
	e.choosing = chooseUnlessMana
	e.ask(d)
}

func (e *Engine) answerUnlessMana(chosen []decision.Option) {
	u := e.unlessPayment
	e.choosing = chooseNone
	if u == nil || len(chosen) != 1 {
		return
	}
	if chosen[0].Kind == "done" {
		u.manaWindow = false
		e.advanceUnlessPayment()
		return
	}
	if chosen[0].Kind != "activate" || !e.untappedManaSource(u.payer, chosen[0].Obj) {
		e.finishUnlessPayment(false)
		return
	}
	e.activateManaPayment(u.payer, chosen[0].Obj, false)
	if e.Pending() == nil {
		e.advanceUnlessPayment()
	}
}

func (e *Engine) unlessPaymentCandidates(u *unlessPayment, zone state.Zone, kind string, part CostPart) []state.ObjID {
	// The dedup is over the union of every component's picks, not just this
	// one's list: one card must not pay two parts, and a card already
	// sacrificed has left its zone anyway, so the wider union only ever
	// removes an already-impossible candidate. Revealed cards are NOT
	// removed from the hand, which is exactly why they need the explicit
	// exclusion.
	used := make([]state.ObjID, 0, len(u.sacs)+len(u.discards)+len(u.reveals))
	used = append(used, u.sacs...)
	used = append(used, u.discards...)
	used = append(used, u.reveals...)
	seen := make(map[state.ObjID]bool, len(used))
	for _, id := range used {
		seen[id] = true
	}
	sc := u.ctx.SpecContext(u.payer)
	var out []state.ObjID
	for _, id := range e.G.Zone(zone, u.payer) {
		if kind == "sacrifice" && e.SacrificeBlocked(id, true) {
			// A CantSacrifice restriction (Call for Aid) or face static: the
			// permanent cannot pay a sacrifice component.
			continue
		}
		if !seen[id] && effects.MatchesSpecCtx(e.G, part.Spec, id, sc) {
			out = append(out, id)
		}
	}
	return out
}

func (e *Engine) recordUnlessPaymentPick(u *unlessPayment, kind string, ids []state.ObjID) {
	switch kind {
	case "sacrifice":
		u.sacs = append(u.sacs, ids...)
	case "revealcost":
		u.reveals = append(u.reveals, ids...)
	default:
		u.discards = append(u.discards, ids...)
	}
}

// revealChosenDesignated reports whether every RevealChosen<Spec> part of the
// pending payment still has its secret designation on the source. A part with
// no designation is unpayable, so the whole payment declines.
func (u *unlessPayment) revealChosenDesignated(e *Engine) bool {
	if len(u.cost.RevealChosen) == 0 {
		return true
	}
	src := e.G.Obj(u.ctx.Source)
	for _, part := range u.cost.RevealChosen {
		if !hasRevealChosenDesignation(src, part.Spec) {
			return false
		}
	}
	return true
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
	if u == nil || u.part >= u.paymentPartCount() {
		return
	}
	part, zone, kind := unlessPartAt(u.cost, u.part)
	ids := make([]state.ObjID, 0, len(chosen))
	for _, o := range chosen {
		if o.Obj != 0 {
			ids = append(ids, o.Obj)
		}
	}
	// Decision.Validate guaranteed the count and offered identity. Recheck the
	// zone/filter against the current game before any event is emitted so a
	// malformed resumed state declines rather than paying an illegal cost.
	eligible := e.unlessPaymentCandidates(u, zone, kind, part)
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
	e.recordUnlessPaymentPick(u, kind, ids)
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
		// The settled Discard component's picks ride the resume point (task
		// mordorparams1): the unless_pay arm hands them to the continuing
		// walk as Ctx.UnlessDiscarded, the ConditionDefined$ Discarded group's
		// mid-resolution channel. A paid payment without a Discard component
		// sets nothing (the channel stays absent).
		if paid && len(u.discards) > 0 {
			u.rp.unlessDiscards = append([]state.ObjID(nil), u.discards...)
		}
		e.resumeResolution(u.rp, nil)
		return
	}
	e.finishManaUnlessPayment(paid)
}
