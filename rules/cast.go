// cast.go is the cast-flow state machine: beginCast starts it from a chosen
// "cast" priority option, continueCast runs its stages (X, Delve, each Sac
// part) in order, asking a KChoose (chooseCast) decision for any stage that
// needs one, and commitCast pays and puts the spell on the stack once every
// stage is settled. Kicker, Surge, Flashback and Delve are registered here
// as the primitives they are (rules/legal.go builds the options that choose
// among them; this file resolves whichever one was picked into a Cost and
// drives it to the stack).
package rules

import (
	"fmt"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// chooseCast, chooseETB and chooseMiracle extend chooseFor (rules/engine.go
// declares chooseNone = iota, the only value Task 8 needed). iota+1 here
// keeps every value distinct from chooseNone without redeclaring it --
// nothing outside this package compares chooseFor values, so the exact
// numbers only need to be pairwise different, not contiguous with the other
// file's block.
const (
	chooseCast chooseFor = iota + 1
	chooseETB
	chooseMiracle
)

// pendingCast is the cast flow's own state, live only between beginCast and
// commitCast (or an abort). ability is -1 for a spell; Task 10 (activated
// abilities) sets it to a real Face().Abilities index and reuses this same
// flow for a cost with X/Sac/Delve of its own.
type pendingCast struct {
	player  state.PlayerID
	card    state.ObjID
	from    state.Zone
	mode    string // "", "kicked", "surged", "flashback", "miracle"
	ability int    // -1 for a spell (Task 10 uses >= 0)

	cost Cost

	x     int32
	xDone bool

	delve     []state.ObjID
	delveDone bool

	sacs    []state.ObjID
	sacPart int
}

// kickerCost and surgeCost resolve a face's own parameterised keyword to a
// parsed Cost, reporting whether the keyword is printed at all.
func kickerCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Kicker")
	if !ok {
		return Cost{}, false
	}
	return ParseCost(s), true
}

func surgeCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Surge")
	if !ok {
		return Cost{}, false
	}
	return ParseCost(s), true
}

// flashbackCost is id's Flashback cost: the printed parameter if this face
// carries one, or -- Flashback granted by a continuous effect with no
// printed parameter of its own (Snapcaster Mage's shape) -- the card's own
// mana cost (CR 702.32a's "cast for its normal cost" fallback).
func (e *Engine) flashbackCost(id state.ObjID) Cost {
	o := e.G.Obj(id)
	if o == nil {
		return Cost{}
	}
	f := o.Face()
	if f == nil {
		return Cost{}
	}
	if s, ok := f.KeywordParam("Flashback"); ok {
		return ParseCost(s)
	}
	return ParseCost(f.ManaCost)
}

// delveCredit is the most generic mana id's Delve can cover right now for a
// cost whose generic requirement is generic: the smaller of that and p's
// graveyard size. Zero for a card without Delve.
func (e *Engine) delveCredit(p state.PlayerID, id state.ObjID, generic int32) int32 {
	if generic <= 0 || !e.HasKeyword(id, "Delve") {
		return 0
	}
	gy := int32(len(e.G.Zone(state.ZGraveyard, p)))
	if gy > generic {
		gy = generic
	}
	return gy
}

// castable reports whether cost is payable for id if cast by p right now:
// mana payable (Colored+Generic), crediting the generic requirement with
// delved graveyard cards when id has Delve; every Sac part has at least N
// matching permanents on p's battlefield; every SubCounter part's N does
// not exceed id's own current counters of that kind; and Tap requires id
// (an already-battlefield source -- Task 10 activates from there) to be
// untapped.
func (e *Engine) castable(p state.PlayerID, id state.ObjID, cost Cost) bool {
	mana := cost
	mana.Generic -= e.delveCredit(p, id, mana.Generic)
	if !mana.CanPay(e.G.Players[p].Pool) {
		return false
	}
	for _, part := range cost.Sac {
		var n int32
		for _, oid := range e.G.Zone(state.ZBattlefield, p) {
			if effects.MatchesSpec(e.G, part.Spec, oid, p) {
				n++
			}
		}
		if n < part.N {
			return false
		}
	}
	if o := e.G.Obj(id); o != nil {
		for _, part := range cost.SubCounter {
			if o.Counter(part.Spec) < part.N {
				return false
			}
		}
		if cost.Tap && o.Tapped {
			return false
		}
	} else if len(cost.SubCounter) > 0 || cost.Tap {
		return false
	}
	return true
}

// spellsCastThisTurn counts PutOnStack events for player p since the last
// TurnChange in the log (or since the start of the log, on turn 1).
func (e *Engine) spellsCastThisTurn(p state.PlayerID) int {
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.PutOnStack && ev.Player == p {
			n++
		}
	}
	return n
}

// beginCast starts the cast flow for opt (a "cast" priority option): resolve
// which cost opt pays (the base/alternative cost as before, or the
// kicked/surged/flashback cost opt.Mode names), build the pendingCast, and
// run its first stage.
func (e *Engine) beginCast(p state.PlayerID, opt decision.Option) {
	id := opt.Obj
	o := e.G.Obj(id)
	f := o.Face()
	from := o.Zone

	// Which cost this pays is opt.AltCostIndex, not always adjustedCost
	// (Ruling T19b-b): legalActions gates each "cast" option on that
	// specific option's own cost being payable, so beginCast must charge
	// that same cost. An out-of-range AltCostIndex (a stale option from a
	// board state that no longer holds the granting static) falls back to
	// the base cost rather than indexing out of bounds.
	cost := e.adjustedCost(p, id)
	if opt.AltCostIndex > 0 {
		if alts := e.alternativeCosts(p, id); opt.AltCostIndex-1 < len(alts) {
			cost = alts[opt.AltCostIndex-1]
		}
	}
	switch opt.Mode {
	case "kicked":
		if kc, ok := kickerCost(f); ok {
			cost = cost.Plus(kc)
		}
	case "surged":
		if sc, ok := surgeCost(f); ok {
			cost = sc
		}
	case "flashback":
		cost = e.flashbackCost(id)
	case "miracle":
		// Task 18: a Miracle cast pays the printed Miracle cost (CR 702.93d) in
		// place of the card's normal cost. KeywordParam is read off the face;
		// a missing keyword (offer routed here only from a Miracle offer, and
		// only while the card is in hand) falls back to the empty cost so a
		// stale cast cannot strand.
		if mc, ok := f.KeywordParam("Miracle"); ok {
			cost = ParseCost(mc)
		} else {
			cost = Cost{}
		}
	}

	e.cast = &pendingCast{player: p, card: id, from: from, mode: opt.Mode, ability: -1, cost: cost}
	e.continueCast()
}

// continueCast runs the cast flow's stages in order -- X, Delve, each Sac
// part -- stopping (and returning) the instant a stage asks a KChoose;
// commitCast runs once every stage has settled. A nil e.cast (a chooseCast
// answer arriving with no flow in progress, only reachable from a
// hand-built decision) is dropped rather than panicked on, mirroring
// castAnswer's own guard.
func (e *Engine) continueCast() {
	if e.cast == nil {
		return
	}
	if e.xAsk() {
		return
	}
	if e.delveAsk() {
		return
	}
	if e.sacAsk() {
		return
	}
	e.commitCast()
}

// xAsk asks a value for {X} if pc.cost carries one, offering 0..max where
// max is the largest value the mana pool (crediting the best possible
// Delve) can still pay. Runs at most once (xDone).
func (e *Engine) xAsk() bool {
	pc := e.cast
	if pc.xDone {
		return false
	}
	pc.xDone = true
	if pc.cost.X <= 0 {
		return false
	}
	pool := e.G.Players[pc.player].Pool
	gy := int32(len(e.G.Zone(state.ZGraveyard, pc.player)))
	// Bound: past this many mana no further X is ever payable, since a
	// bigger X strictly grows Generic (X > 0 here) while both the pool and
	// the best possible Delve credit are fixed at this instant.
	bound := pool.Total() + gy + 1
	var max int32
	for x := int32(0); x <= bound; x++ {
		wx := pc.cost.WithX(x)
		wx.Generic -= e.delveCredit(pc.player, pc.card, wx.Generic)
		if !wx.CanPay(pool) {
			break
		}
		max = x
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose a value for X", Source: pc.card}
	for x := int32(0); x <= max; x++ {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "x",
			Label: fmt.Sprintf("X = %d", x), Amount: int(x)})
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// delveAsk offers exiling graveyard cards to pay for id's Delve, when id has
// Delve, the caster's graveyard is non-empty and the resolved cost still
// carries a generic requirement to reduce. Runs at most once (delveDone).
// Max is the SHORTFALL -- generic minus what the pool can already pay --
// not the whole generic requirement, so a caster with enough mana is not
// offered (and a bot does not take) exiles the cost does not actually need.
func (e *Engine) delveAsk() bool {
	pc := e.cast
	if pc.delveDone {
		return false
	}
	pc.delveDone = true
	if !e.HasKeyword(pc.card, "Delve") {
		return false
	}
	gy := e.G.Zone(state.ZGraveyard, pc.player)
	cost := pc.cost.WithX(pc.x)
	generic := cost.Generic
	if len(gy) == 0 || generic <= 0 {
		return false
	}
	// Max is the SHORTFALL -- generic minus what the pool can already pay --
	// not the whole generic requirement, so a caster with enough mana is not
	// offered (and a bot does not take) exiles the cost does not actually
	// need. Delve only ever covers generic, so the colored requirement is
	// reserved out of the pool first; the rest of the pool can pay at most
	// its total as generic (Cost.Pay's WUBRG spending order never reduces
	// the total it can cover).
	rest := e.G.Players[pc.player].Pool
	for i, n := range cost.Colored {
		if rest[i] < n {
			// Colored unpayable: delve cannot help with it, so the whole
			// generic requirement is the shortfall (the commit stage's own
			// payMana will still fail honestly).
			rest = state.Mana{}
			break
		}
		rest[i] -= n
	}
	payable := generic
	if rest.Total() < payable {
		payable = rest.Total()
	}
	shortfall := generic - payable
	if shortfall <= 0 {
		return false
	}
	max := len(gy)
	if int32(max) > shortfall {
		max = int(shortfall)
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 0, Max: max,
		Prompt: "Delve: exile cards from your graveyard to help cast " + e.G.Obj(pc.card).Face().Name,
		Source: pc.card}
	for _, id := range gy {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "exile",
			Obj: id, Label: e.G.Obj(id).Face().Name})
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// sacAsk offers the next unsettled Sac cost part, walking pc.cost.Sac in
// order (pc.sacPart). castable already required at least N matching
// permanents before this option was ever offered, so a part with too few
// candidates is a board that changed under a hand-built intent -- and the
// totality rule is that the cast must not strand on an unanswerable
// decision, so such a part is skipped (the commit stage's own payMana/
// MoveZone will still fail honestly if the cost is genuinely unpayable)
// rather than asked with Min=Max=0 and no options. Already-chosen
// sacrifices are excluded from the candidates so one permanent can never
// be chosen for two Sac parts of the same cost.
func (e *Engine) sacAsk() bool {
	pc := e.cast
	for pc.sacPart < len(pc.cost.Sac) {
		part := pc.cost.Sac[pc.sacPart]
		var candidates []state.ObjID
		for _, oid := range e.G.Zone(state.ZBattlefield, pc.player) {
			if effects.MatchesSpec(e.G, part.Spec, oid, pc.player) {
				already := false
				for _, s := range pc.sacs {
					if s == oid {
						already = true
						break
					}
				}
				if !already {
					candidates = append(candidates, oid)
				}
			}
		}
		n := int(part.N)
		if n <= 0 || n > len(candidates) {
			pc.sacPart++
			continue
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: n, Max: n,
			Prompt: "Sacrifice a permanent to cast " + e.G.Obj(pc.card).Face().Name,
			Source: pc.card}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "sacrifice",
				Obj: id, Label: e.G.Obj(id).Face().Name})
		}
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	return false
}

// castAnswer records a chooseCast answer into the flow, keyed off which
// stage asked it (every option in one decision shares a Kind).
func (e *Engine) castAnswer(d *decision.Decision, chosen []decision.Option) {
	pc := e.cast
	if pc == nil || len(d.Options) == 0 {
		return
	}
	switch d.Options[0].Kind {
	case "x":
		if len(chosen) > 0 {
			// The value rides on Option.Amount, not Option.Index: xAsk is the
			// first stage and appends 0..max into an empty option list, so
			// Index happens to equal the value today, but a later task that
			// prepends an option (a "cancel", Task 10's ability variants)
			// would silently corrupt an Index-derived value.
			pc.x = int32(chosen[0].Amount)
		}
	case "exile":
		for _, o := range chosen {
			pc.delve = append(pc.delve, o.Obj)
		}
	case "sacrifice":
		for _, o := range chosen {
			pc.sacs = append(pc.sacs, o.Obj)
		}
		pc.sacPart++
	}
}

// modeFlags maps a pendingCast.mode to the CastInfo Counter string
// (events.FlagsString of the matching CastFlags bit), "" for a plain cast.
func modeFlags(mode string) string {
	switch mode {
	case "kicked":
		return events.FlagsString(state.FlagKicked)
	case "surged":
		return events.FlagsString(state.FlagSurged)
	case "flashback":
		return events.FlagsString(state.FlagFlashback)
	case "miracle":
		return events.FlagsString(state.FlagMiracle)
	}
	return ""
}

// commitCast pays, moves the delved and sacrificed cards, records how the
// spell was cast, and puts it on the stack. A payment that fails here (a
// pool that changed under a hand-built intent -- castable already gated the
// option the caster chose, so this is not reachable from an ordinary,
// well-formed client) aborts with a Note and leaves the card where it was;
// so does the card having moved out from under the flow entirely.
func (e *Engine) commitCast() {
	pc := e.cast
	e.cast, e.choosing = nil, chooseNone
	o := e.G.Obj(pc.card)
	if o == nil || o.Zone != pc.from {
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Text: "cast aborted: the card moved"})
		return
	}
	mana := pc.cost.WithX(pc.x)
	mana.Generic -= int32(len(pc.delve))
	if mana.Generic < 0 {
		mana.Generic = 0
	}
	if !e.payMana(pc.player, mana) {
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Text: "cast aborted: cost no longer payable"})
		return
	}
	for _, id := range pc.delve {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile, Text: "delved"})
	}
	for _, id := range pc.sacs {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
	if flags := modeFlags(pc.mode); pc.x != 0 || flags != "" {
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: pc.x, Counter: flags})
	}
	e.emit(events.Event{Kind: events.PutOnStack, Obj: pc.card, Player: pc.player, From: pc.from, To: state.ZStack, Text: o.Face().Name})
	if sa := o.Face().SpellAbility(); sa != nil && sa.Params["ValidTgts"] != "" {
		e.askTarget(pc.player, pc.card, sa)
	}
}

func init() {
	effects.RegisterNonAPI("kw:Kicker", "kw:Surge", "kw:Flashback", "kw:Delve")
}
