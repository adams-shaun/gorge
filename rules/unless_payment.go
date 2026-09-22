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

// manaPipCount is an upper bound on the mana units a cost can consume. Any
// minimal covering selection of window sources uses at most this many
// sources (every window source contributes at least one unit), so the
// reachability search below never has to consider picking more.
func (c Cost) manaPipCount() int {
	n := int(c.Generic) + int(c.Colored.Total()) + int(c.Snow)
	n += len(c.Hybrid) + len(c.Phyrexian) + len(c.Twobrid) + len(c.HybridPhyrexian)
	return n
}

// unlessManaReachable reports whether pool plus one production alternative
// per window unit (or none of a unit) can satisfy cost's mana/life component.
// It is the exact affordability question the offer gate and the window's
// safety ordering both ask: a unit with several free abilities offers its
// alternatives as a CHOICE (the payer taps the permanent for one of them),
// never as their sum, so a dual land contributes {U} or {R} -- and the
// search proves a {U} tax payable from it. The search is bounded by
// manaPipCount (a minimal cover never needs more sources than the mana it
// supplies) and by a hard node budget; beyond that it fails closed, which is
// the conservative direction (never offer a Pay the window cannot complete).
func (e *Engine) unlessManaReachable(cost Cost, pool, snow state.Mana, typed [3]state.Mana, life int32, units []windowManaUnit) bool {
	if cost.payable(pool, snow, typed, life) {
		return true
	}
	budget := cost.manaPipCount()
	if budget <= 0 {
		return false
	}
	nodes := 0
	var rec func(start, remaining int, acc state.Mana) bool
	rec = func(start, remaining int, acc state.Mana) bool {
		if cost.payable(manaAdd(pool, acc), snow, typed, life) {
			return true
		}
		if remaining <= 0 {
			return false
		}
		nodes++
		if nodes > 1<<18 {
			return false
		}
		for i := start; i < len(units); i++ {
			for _, a := range units[i].alts {
				if rec(i+1, remaining-1, manaAdd(acc, a.mana())) {
					return true
				}
			}
		}
		return false
	}
	return rec(0, budget, state.Mana{})
}

// UnlessCostPayable is the rules-side offer gate for the generic unless
// election (the ctx-less form used by tests and any mana-only caller).
func (e *Engine) UnlessCostPayable(p state.PlayerID, raw string) bool {
	return e.unlessCostPayable(p, raw, nil, 0)
}

// UnlessCostPayableFromCtx is the full-context offer gate the effects ask
// uses (poseUnlessAsk): it hands the gate the very resolution context, so
// source- and role-dependent components (RevealChosen's secret designation,
// a SubCounter drain, and a Draw<.../Player.targetedBy> or TriggeredPlayer
// drawer) are evaluated against the same bindings the pay path will use. It
// is the same gate the resolution arm runs, so the option list and the pay
// decision cannot disagree.
func (e *Engine) UnlessCostPayableFromCtx(p state.PlayerID, raw string, ctx *effects.Ctx) bool {
	var stackObj state.ObjID
	if ctx != nil {
		stackObj = ctx.Source
	}
	return e.unlessCostPayable(p, raw, ctx, stackObj)
}

// unlessCostPayable is the ctx-aware offer gate. A cost is offered when every
// supported component is satisfiable RIGHT NOW: the choice-bearing Sac/
// Discard/Reveal parts must have enough distinct candidates, SubCounter parts
// enough counters on the resolving source, RevealChosen parts their secret
// designation, Draw parts a resolvable role, and the mana/life component must
// be reachable from the floating pool plus the window's alternatives. An
// unpriceable cost is a hard decline: it receives only the decline option, so
// an answer can never select a Pay that the settlement path must reject. The
// Sacrifice arm's recognised DamageYou<N> payment is separate and never calls
// this generic gate. The pre-fix gate skipped the non-mana components and used
// a single-ability-per-
// source aggregate, so it offered Pay for a `Sac<1/Creature>` with no
// creatures and suppressed a {U} tax on an untapped dual land.
func (e *Engine) unlessCostPayable(p state.PlayerID, raw string, ctx *effects.Ctx, stackObj state.ObjID) bool {
	cost, ok := ParseUnlessCost(raw)
	if !ok {
		return false
	}
	if ctx == nil {
		ctx = &effects.Ctx{Controller: p}
	}
	if !e.unlessComponentsPayable(p, cost, ctx, stackObj) {
		return false
	}
	player := e.G.Players[p]
	if cost.payable(player.Pool, player.Snow, player.TypedMana, player.Life) {
		return true
	}
	if !cost.hasManaPayment() {
		// No mana to window. A life/snow component is what the pool check
		// just rejected and no source can supply it, so do not offer.
		if cost.Life > 0 || cost.Snow > 0 {
			return false
		}
		return true
	}
	return e.unlessManaReachable(cost, player.Pool, player.Snow, player.TypedMana, player.Life, e.windowManaUnits(p))
}

// unlessComponentsPayable checks every non-mana part of an unless cost for
// satisfiability, sharing the pay path's own candidate enumeration
// (unlessCandidatesFor) so the gate can never disagree with
// beginUnlessPayment about what exists. The choice-bearing Sac/Discard/Reveal
// parts must admit a system of distinct representatives (one card cannot pay
// two parts); the synchronous parts (SubCounter/RevealChosen/Draw) are
// checked directly.
func (e *Engine) unlessComponentsPayable(p state.PlayerID, cost Cost, ctx *effects.Ctx, stackObj state.ObjID) bool {
	return e.unlessChoiceComponentsPayable(p, cost, ctx) &&
		e.unlessCountersAffordableFor(cost, *ctx, stackObj) &&
		unlessRevealChosenDesignated(e.G, cost, *ctx) &&
		e.unlessDrawsResolvable(p, cost, ctx)
}

// unlessChoiceComponentsPayable builds the per-sub-slot candidate lists for
// the choice-bearing parts and runs a bipartite matching: feasible iff a
// system of distinct representatives of the required size exists.
func (e *Engine) unlessChoiceComponentsPayable(p state.PlayerID, cost Cost, ctx *effects.Ctx) bool {
	type slot struct {
		zone state.Zone
		kind string
		part CostPart
	}
	var subs []slot
	add := func(zone state.Zone, kind string, parts []CostPart) {
		for _, part := range parts {
			for i := int32(0); i < part.N; i++ {
				subs = append(subs, slot{zone: zone, kind: kind, part: part})
			}
		}
	}
	add(state.ZBattlefield, "sacrifice", cost.Sac)
	add(state.ZHand, "discard", cost.Discard)
	add(state.ZHand, "revealcost", cost.Reveal)
	if len(subs) == 0 {
		return true
	}
	cands := make([][]state.ObjID, len(subs))
	for i, s := range subs {
		cands[i] = e.unlessCandidatesFor(p, *ctx, s.zone, s.kind, s.part, nil)
	}
	return maxBipartiteMatch(cands) == len(subs)
}

// maxBipartiteMatch returns the maximum matching size between left slots and
// the distinct right-side candidates, in deterministic slot/candidate order.
func maxBipartiteMatch(cands [][]state.ObjID) int {
	matchOf := map[state.ObjID]int{} // candidate -> slot
	var try func(slot int, seen map[state.ObjID]bool) bool
	try = func(slot int, seen map[state.ObjID]bool) bool {
		for _, c := range cands[slot] {
			if seen[c] {
				continue
			}
			seen[c] = true
			if prev, ok := matchOf[c]; !ok || try(prev, seen) {
				matchOf[c] = slot
				return true
			}
		}
		return false
	}
	n := 0
	for i := range cands {
		if try(i, map[state.ObjID]bool{}) {
			n++
		}
	}
	return n
}

func (e *Engine) unlessDrawsResolvable(p state.PlayerID, cost Cost, ctx *effects.Ctx) bool {
	for _, part := range cost.Draw {
		if _, ok := unlessDrawPlayers(ctx, p, part.Spec); !ok {
			return false
		}
	}
	return true
}

// beginUnlessPayment begins a payer-selected payment. It owns all Sac,
// Discard and Reveal components, including the exact-candidate no-ask cases,
// so neither this path nor a future sibling can fall back to a
// first-in-zone-order pick.
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
		!e.unlessCountersAffordable(u) ||
		!u.revealChosenDesignated(e) {
		e.finishUnlessPayment(false)
		return
	}
	// A mana component the pool cannot yet cover opens the one-source-at-a-
	// time CR 601.2g window (the ward/attack-prop discipline) rather than
	// declining: the payer taps until the pool covers the charge, then the
	// ordinary path below pays it. The window is only opened while a source
	// remains that the budget counted.
	if !u.cost.payable(e.G.Players[u.payer].Pool, e.G.Players[u.payer].Snow, e.G.Players[u.payer].TypedMana, e.G.Players[u.payer].Life) {
		if u.cost.hasManaPayment() && len(e.windowManaUnits(u.payer)) > 0 {
			e.askUnlessMana()
			return
		}
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
	if !e.payMana(u.payer, u.cost) { // guarded above; retain totality if state changes.
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

// askUnlessMana opens one tap ask over the payer's remaining window-eligible
// sources. Every production alternative of a source is its own option (a dual
// land offers "Tap Volcanic Island for {U}" and "... for {R}"), so the tap
// resolves the chosen ability with no nested colour ask. Options that keep
// the charge reachable come first, and the bot's first-activate pick is then
// always a non-stranding tap; options that would strand remain offered (a
// player may make a losing tap) but after every safe one. "Done" is always
// legal: it settles the payment (the ordinary path declines if the pool still
// cannot cover the charge) and is the R-9 no-host-compatible completion.
func (e *Engine) askUnlessMana() {
	u := e.unlessPayment
	if u == nil {
		return
	}
	units := e.windowManaUnits(u.payer)
	pool := e.G.Players[u.payer].Pool
	snow := e.G.Players[u.payer].Snow
	typed := e.G.Players[u.payer].TypedMana
	life := e.G.Players[u.payer].Life
	d := &decision.Decision{Player: u.payer, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Activate mana abilities to pay the unless cost", ResumeKind: "unless_mana"}
	// Safe alternatives first: tapping this one leaves the charge reachable
	// from the remaining sources, so the first-activate bot arm cannot strand.
	for _, safe := range []bool{true, false} {
		for si, src := range units {
			rest := make([]windowManaUnit, 0, len(units)-1)
			rest = append(rest, units[:si]...)
			rest = append(rest, units[si+1:]...)
			for ai, a := range src.alts {
				if safe != (e.unlessManaReachable(u.cost, manaAdd(pool, a.mana()), snow, typed, life, rest)) {
					continue
				}
				name := "a mana source"
				if o := e.G.Obj(src.id); o != nil && o.Face() != nil {
					name = o.Face().Name
				}
				d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "activate",
					Obj: src.id, Ability: ai, Label: "Tap " + name + manaAltLabel(a)})
			}
		}
	}
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "done", Label: "Done"})
	e.choosing = chooseUnlessMana
	e.ask(d)
}

// manaAltLabel renders an alt's production as a short " for {U}{R}" suffix,
// so a dual land's two abilities are distinguishable on the wire.
func manaAltLabel(a windowManaAlt) string {
	syms := "WUBRGC"
	var b strings.Builder
	for i, n := range a.counts {
		for k := int32(0); k < n*a.amt; k++ {
			b.WriteByte(syms[i])
		}
	}
	if b.Len() == 0 {
		return ""
	}
	return " for {" + strings.Join(strings.Split(b.String(), ""), "}{") + "}"
}

// answerUnlessMana applies one window answer. "Done" re-enters the ordinary
// payment path (which declines if the pool is still short); an activation
// resolves the recorded alternative's exact *cards.SA -- the membership walk
// captured it, so no chooseMana sub-ask is posed -- and then either re-enters
// advanceUnlessPayment (a sub-askless activation) or waits for
// handleChoose's continuation.
func (e *Engine) answerUnlessMana(chosen []decision.Option) {
	u := e.unlessPayment
	e.choosing = chooseNone
	if u == nil || len(chosen) != 1 {
		return
	}
	if chosen[0].Kind == "done" {
		e.advanceUnlessPayment()
		return
	}
	if chosen[0].Kind != "activate" {
		e.finishUnlessPayment(false)
		return
	}
	for _, src := range e.windowManaUnits(u.payer) {
		if src.id != chosen[0].Obj {
			continue
		}
		ai := chosen[0].Ability
		if ai < 0 || ai >= len(src.alts) {
			break
		}
		e.resolveManaAbility(u.payer, src.id, src.alts[ai].ma, false)
		if e.Pending() == nil {
			e.advanceUnlessPayment()
		}
		return
	}
	e.finishUnlessPayment(false)
}

// unlessCandidatesFor is the shared candidate enumeration the offer gate
// (unlessChoiceComponentsPayable) and the payment continuation
// (unlessPaymentCandidates) both use, so the two can never disagree about
// what exists. used lists already-committed picks that must not be offered
// twice; the gate passes the empty list.
func (e *Engine) unlessCandidatesFor(payer state.PlayerID, ctx effects.Ctx, zone state.Zone, kind string, part CostPart, used []state.ObjID) []state.ObjID {
	seen := make(map[state.ObjID]bool, len(used))
	for _, id := range used {
		seen[id] = true
	}
	sc := ctx.SpecContext(payer)
	var out []state.ObjID
	for _, id := range e.G.Zone(zone, payer) {
		if kind == "sacrifice" && e.SacrificeBlocked(id, true) {
			// A CantSacrifice restriction (Call for Aid) or face static: the
			// permanent cannot pay a sacrifice component.
			continue
		}
		if !seen[id] && e.matchesSpec(part.Spec, id, sc) {
			out = append(out, id)
		}
	}
	return out
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
	return e.unlessCandidatesFor(u.payer, u.ctx, zone, kind, part, used)
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
	return unlessRevealChosenDesignated(e.G, u.cost, u.ctx)
}

// unlessRevealChosenDesignated is the shared RevealChosen check: the offer
// gate (unlessComponentsPayable) and the payment continuation
// (revealChosenDesignated) both require every part's secret designation to be
// present on the resolving source.
func unlessRevealChosenDesignated(g *state.Game, cost Cost, ctx effects.Ctx) bool {
	if len(cost.RevealChosen) == 0 {
		return true
	}
	src := g.Obj(ctx.Source)
	for _, part := range cost.RevealChosen {
		if !hasRevealChosenDesignation(src, part.Spec) {
			return false
		}
	}
	return true
}

func (e *Engine) unlessCountersAffordable(u *unlessPayment) bool {
	return e.unlessCountersAffordableFor(u.cost, u.ctx, u.stackObj)
}

// unlessCountersAffordableFor is the shared SubCounter feasibility check: the
// offer gate and the payment continuation both resolve the draining source the
// same way (an activated ability's host, else the resolving source -- exactly
// payUnlessCost's rule) and require enough counters of each named kind.
func (e *Engine) unlessCountersAffordableFor(cost Cost, ctx effects.Ctx, stackObj state.ObjID) bool {
	if len(cost.SubCounter) == 0 {
		return true
	}
	src := ctx.Source
	if o := e.G.Obj(stackObj); o != nil && o.Ability != nil {
		src = o.Source
	}
	o := e.G.Obj(src)
	if o == nil {
		return false
	}
	need := make(map[string]int32, len(cost.SubCounter))
	for _, part := range cost.SubCounter {
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
