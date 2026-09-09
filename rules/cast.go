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
	"sort"
	"strconv"

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

	// payIdx / payColor / payLife carry the hybrid and Phyrexian payment
	// announcement (CR 601.2b/107.4e-f). manaAsk walks the cost's combined
	// hybrid-then-Phyrexian pip list one decision at a time; payIdx is the
	// next unsettled pip, payColor accumulates the coloured spend the
	// announced pips chose, and payLife the life a Phyrexian pip paid with
	// two life costs. Plain data, so Clone copies it like x/delve/sacs.
	payIdx   int
	payColor state.Mana
	payLife  int32

	// stackObj is the id of the object pushCast placed on the stack (the
	// spell card itself, or an activated ability's AbilityPush-minted
	// object). Zero until pushCast runs; handleTarget records the chosen
	// targets onto it, because a zone change clears an object's Targets.
	stackObj state.ObjID

	// pushed is true once the object has reached the stack (post-pushCast).
	// An aborted proposal reverses the push when it is set.
	pushed bool

	// preSuppress is the suppressedCast set as it was just before pushCast's
	// PutOnStack, captured so an aborted (reversed) cast can restore it:
	// the push is a state-changing event that emit treats as progress and so
	// clears the held-out no-progress set, but an aborted cast is net no
	// progress, so that set must come back. Nil when no cast push is in
	// flight (an ability, or a spell aborted before the push).
	preSuppress map[state.ObjID]bool

	// Task 12: the card's "as this enters" choices (one per ETBReplacement
	// Repl whose ReplaceWith$ is NameCard/ChooseType/ChooseNumber). Each is
	// asked in order while choosing == chooseETB; etbIdx is the next
	// unsettled one, so the continuation survives the chooseAnswer round trip
	// (answer -> etbAnswer increments etbIdx -> continueCast re-enters
	// etbAsk). Plain data, so a Clone copies it like the fields above.
	etbs   []etbChoice
	etbIdx int
}

// etbChoice is one "as this enters" choice, pre-computed: its kind
// ("name"/"/type"/"number", matching the Choose event's Counter) and the
// option list that will be offered, captured once at the start of the cast
// flow so the decision and the recorded choice always agree.
type etbChoice struct {
	kind    string
	options []decision.Option
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
//
// The Sac check is a distinct-candidate feasibility check, not N independent
// head-counts against the same board (fix round 1, reviewer Important 1):
// the Sac parts of ONE cost are paid one after another, each consuming its
// chosen permanents, so a cost with TWO Sac parts cannot be paid by the same
// permanent twice. A `Sac<1/Creature> Sac<1/Creature>` cost must therefore
// not be offered with a single creature on the battlefield, and a
// `Sac<1/Creature.Red> Sac<1/Creature.Green> Sac<1/Creature.White>` cost
// cannot count one red-and-green creature towards both the red and the green
// part. Each part's N candidates are reserved (distinct, in zone-walk order)
// as the parts are walked, mirroring exactly what sacAsk offers; a part with
// fewer than N un-reserved candidates makes the whole cost unpayable, so the
// option is never offered (the totality rule an option that cannot be paid
// should never be offered). Reserving the first N matches in zone order is a
// sound test -- it never reports payable when no distinct assignment exists --
// and never illegal: a truly-payable cost where the FIRST N happen to collide
// with a scarcer later part is conservatively withheld (the engine's standing
// rule is that wrongly withholding a legal option is safe, while wrongly
// offering an unpayable one is an illegal game action).
func (e *Engine) castable(p state.PlayerID, id state.ObjID, cost Cost) bool {
	mana := cost
	mana.Generic -= e.delveCredit(p, id, mana.Generic)
	if !mana.payable(e.G.Players[p].Pool, e.G.Players[p].Life) {
		return false
	}
	reserved := map[state.ObjID]bool{}
	for _, part := range cost.Sac {
		var avail []state.ObjID
		for _, oid := range e.G.Zone(state.ZBattlefield, p) {
			if reserved[oid] { // an earlier Sac part already claimed this one
				continue
			}
			if effects.MatchesSpecFrom(e.G, part.Spec, oid, p, id) {
				avail = append(avail, oid)
			}
		}
		if int32(len(avail)) < part.N {
			return false
		}
		for i := int32(0); i < part.N; i++ {
			reserved[avail[i]] = true
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
	// CR 601.2b/f/h: a spell's own SpellAbility may carry an explicit Cost$
	// (Forge's SP Cost) naming an additional cost -- most commonly a
	// sacrifice (Altar's Reap's "1 B Sac<1/Creature>", the CR 601.2h example).
	// The mana part of that Cost$ REPLACES the printed mana (it is the same
	// cost the card already charges), so only its non-mana parts
	// (Sac/SubCounter/Tap) are additional and fold into the total cost here; a
	// re-added mana part would double charge. Only a plain cast reaches this
	// (pc.ability < 0 and no alternative/flashback recast), and a spell with
	// no SP Cost$ contributes nothing.
	if opt.AltCostIndex == 0 && opt.Mode == "" {
		if sa := f.SpellAbility(); sa != nil {
			if sc := sa.Params["Cost"]; sc != "" {
				extra := ParseCost(sc)
				if len(extra.Sac) > 0 {
					cost.Sac = append(append([]CostPart(nil), cost.Sac...), extra.Sac...)
				}
				if len(extra.SubCounter) > 0 {
					cost.SubCounter = append(append([]CostPart(nil), cost.SubCounter...), extra.SubCounter...)
				}
				cost.Tap = cost.Tap || extra.Tap
			}
		}
	}
	// CR 903.8: the commander tax, applied to whatever cost this cast pays
	// (the base/alternative/kicked/flashback/surged/miracle cost resolved
	// above) -- the exact same commanderTaxFor the command-zone offer in
	// legal.go gated castable on, over the same board, so this charge and
	// that offer can never disagree. For a command-zone commander this is the
	// plain base + the tax; for every other card/zone it passes cost through
	// unchanged (commanderTaxFor is a no-op outside the Commander format and
	// off the command zone). It lands AFTER cost reduction and any keyword
	// recast, so an additional cost is never reduced by them, in line with how
	// Kicker's own additional cost composes.
	cost = e.commanderTaxFor(p, id, cost)

	e.cast = &pendingCast{player: p, card: id, from: from, mode: opt.Mode, ability: -1, cost: cost}
	e.collectETBChoices(p)
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
	if e.etbAsk() {
		return
	}
	// CR 601.2a: the object reaches the stack before the target choice
	// (601.2c) and payment (601.2h). For a spell the cast trigger (601.2i)
	// is held back until payCast; an ability's AbilityPush fires no trigger.
	if e.pushCast() {
		return
	}
	// CR 601.2b: announce how each hybrid and Phyrexian pip is paid -- which
	// half of a hybrid, whether a Phyrexian pip is paid with life -- before
	// targets (601.2c) and payment (601.2h). Runs as one decision per pip.
	if e.manaAsk() {
		return
	}
	// CR 601.2c: choose targets, now that the object is on the stack. An SA
	// with no target (or a zero-minimum one with no legal candidate) asks
	// nothing and payCast runs directly.
	if e.targetAsk() {
		return
	}
	e.payCast()
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
		if !wx.payable(pool, e.G.Players[pc.player].Life) {
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
// order (pc.sacPart). The chosen sacrifices are excluded from each later
// part's candidates so one permanent can never pay two Sac parts of the
// same cost. castable already required a distinct-candidate assignment
// before this option was ever offered (the fix-round-1 gate), so a part
// with too few candidates here is a board that changed under the flow --
// most directly, an earlier part of the SAME cost consumed the remaining
// matching permanents (an `Sac` part earlier in the same cost, or a board
// that changed under a hand-built intent) that the later part now needs.
// The totality rule is that a cost that cannot be fully paid must not be
// committed with only part of it paid, so rather than skip the part and
// let commitCast validate mana only, such a part aborts the whole cast/
// activation cleanly, exactly as if it was never offered. No sacrifice has
// actually moved yet -- sacAsk only records the choices into pc.sacs; the
// MoveZone events are emitted by commitCast -- so clearing e.cast restores
// the pre-offer board and the Note leaves nothing behind.
func (e *Engine) sacAsk() bool {
	pc := e.cast
	for pc.sacPart < len(pc.cost.Sac) {
		part := pc.cost.Sac[pc.sacPart]
		var candidates []state.ObjID
		for _, oid := range e.G.Zone(state.ZBattlefield, pc.player) {
			if effects.MatchesSpecFrom(e.G, part.Spec, oid, pc.player, pc.card) {
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
			// A cost that can no longer be fully paid must not commit half
			// paid (fix round 1, reviewer Important 1; see the doc above for
			// why this is unreachable from a well-formed offer after the
			// castable gate). Abort the whole thing; nothing has moved yet.
			e.cast, e.choosing = nil, chooseNone
			e.emit(events.Event{Kind: events.Note, Player: pc.player, Text: "sacrifice cost no longer payable; cast/activation aborted"})
			return true
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

// collectETBChoices walks pc.card's printed replacement lines and, for every
// ETBReplacement Repl whose ReplaceWith$ resolves to a NameCard/ChooseType/
// ChooseNumber ability, adds one etbChoice with its pre-built option list.
// The list (not just the kind) is captured up front so the offered option and
// the recorded choice always agree, and so the choice is the same whether it
// is asked here (cast flow) or once the object has moved (a land's
// play_land). Nothing is asked and no choice is recorded for an etbCounter
// replacement (its ReplaceWith$ is PutCounter) -- those need only Ctx.X, not
// a player decision.
func (e *Engine) collectETBChoices(you state.PlayerID) {
	pc := e.cast
	if pc == nil {
		return
	}
	o := e.G.Obj(pc.card)
	if o == nil {
		return
	}
	f := o.Face()
	if f == nil {
		return
	}
	for i := range f.Repls {
		r := &f.Repls[i]
		if r.Params["Keyword"] != "ETBReplacement" || r.With == nil {
			continue
		}
		kind := etbChoiceKind(r.With.API)
		if kind == "" {
			continue
		}
		pc.etbs = append(pc.etbs, etbChoice{
			kind:    kind,
			options: e.etbOptions(you, pc.card, kind, r.With.Params["ValidCards"]),
		})
	}
}

func etbChoiceKind(api string) string {
	switch api {
	case "NameCard":
		return "name"
	case "ChooseType":
		return "type"
	case "ChooseNumber":
		return "number"
	}
	return ""
}

// etbOptions builds the option list for one "as this enters" choice. It is a
// total list-pick -- every collectETBChoices borrower guaranteed at least one
// legal option (a name is anything on the board/hand/yard, a type falls back
// to "Human", a number is always 0..12) -- so no etb decision can ever be
// handed out with zero options, and nothing asks an empty choice (R-9's
// totality rule; see the Options here and the Min/Max 1 in etbAsk).
//
// Option list order is deterministic: names and types are sorted strings
// (never from a map), numbers are ascending.
func (e *Engine) etbOptions(you state.PlayerID, card state.ObjID, kind, validCards string) []decision.Option {
	switch kind {
	case "name":
		if validCards == "" {
			validCards = "Card.nonLand"
		}
		seen := map[string]bool{}
		names := []string{}
		add := func(z state.Zone, players []state.PlayerID) {
			for _, p := range players {
				for _, id := range e.G.Zone(z, p) {
					o := e.G.Obj(id)
					if o == nil || o.Face() == nil {
						continue
					}
					if !effects.MatchesSpecFrom(e.G, validCards, id, you, card) {
						continue
					}
					if seen[o.Face().Name] {
						continue
					}
					seen[o.Face().Name] = true
					names = append(names, o.Face().Name)
				}
			}
		}
		add(state.ZHand, []state.PlayerID{you})
		add(state.ZBattlefield, e.G.AliveFrom(0))
		add(state.ZGraveyard, e.G.AliveFrom(0))
		sort.Strings(names)
		out := make([]decision.Option, 0, len(names))
		for _, n := range names {
			out = append(out, decision.Option{Index: len(out), Kind: "name", Label: n})
		}
		return out
	case "type":
		seen := map[string]bool{}
		types := []string{}
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if o.Owner != you {
				continue
			}
			f := o.Face()
			if f == nil || !isCreatureFace(f) {
				continue
			}
			for _, t := range f.Types {
				if !effects.CreatureTypeWords(t) || seen[t] {
					continue
				}
				seen[t] = true
				types = append(types, t)
			}
		}
		if len(types) == 0 {
			types = []string{"Human"}
		}
		sort.Strings(types)
		out := make([]decision.Option, 0, len(types))
		for _, t := range types {
			out = append(out, decision.Option{Index: len(out), Kind: "type", Label: t})
		}
		return out
	default: // "number"
		out := make([]decision.Option, 0, 13)
		for i := 0; i <= 12; i++ {
			out = append(out, decision.Option{Index: len(out), Kind: "number", Label: strconv.Itoa(i), Amount: i})
		}
		return out
	}
}

// isCreatureFace is a local creature test (effects.hasType is unexported);
// reads the printed Types, which is all any creature-subtype enumeration
// needs.
func isCreatureFace(f *cards.Face) bool {
	for _, t := range f.Types {
		if t == "Creature" {
			return true
		}
	}
	return false
}

// etbAsk asks the next unsettled "as this enters" choice (pc.etbs[pc.etbIdx]),
// one at a time. Runs until every choice is settled; once none remain it
// returns false and continueCast falls through to commitCast. Every choice is
// a single-pick of its full option list, so Min==Max==1; a real option is
// always present, so the cast cannot strand on an unanswerable decision.
func (e *Engine) etbAsk() bool {
	pc := e.cast
	if pc == nil || pc.etbIdx >= len(pc.etbs) {
		return false
	}
	ch := pc.etbs[pc.etbIdx]
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose" + etbChoicePrompt(ch.kind), Source: pc.card}
	d.Options = append(d.Options, ch.options...)
	e.choosing = chooseETB
	e.ask(d)
	return true
}

// etbChoicePrompt names the kind of an "as this enters" choice for a client
// prompt; a cosmetic suffix on the shared "Choose" heading.
func etbChoicePrompt(kind string) string {
	switch kind {
	case "name":
		return " a card name"
	case "type":
		return " a creature type"
	}
	return " a number"
}

// announcePip resolves the i-th announcement pip of a cost's hybrid-then-
// Phyrexian list into its acceptable colours and whether it may be paid with
// two life (a Phyrexian pip). It is the single source both manaAsk (the ask's
// valid-option set) and castAnswer (recording the choice) consult, so the
// option offered and the recorded choice always agree.
func (c Cost) announcePip(i int) (colors [2]byte, lifeOK bool) {
	if i < len(c.Hybrid) {
		p := c.Hybrid[i]
		return [2]byte{p.A, p.B}, false
	}
	letter := c.Phyrexian[i-len(c.Hybrid)]
	return [2]byte{letter, letter}, true
}

// annPipCount is how many hybrid + Phyrexian pips a cost carries.
func (c Cost) annPipCount() int { return len(c.Hybrid) + len(c.Phyrexian) }

// resolvedMana returns the cost the announced payment actually commits: X
// folded, every hybrid and Phyrexian pip removed (each was announced by
// manaAsk into payColor/payLife), and the announced coloured spend folded
// into Colored so payMana charges it from the pool. payLife is applied
// separately by payCast. For a cost with no hybrid or Phyrexian pip this is
// just the X-folded cost, so ordinary casting is unchanged.
func (pc *pendingCast) resolvedMana() Cost {
	m := pc.cost.WithX(pc.x)
	m.Hybrid = nil
	m.Phyrexian = nil
	for i := range pc.payColor {
		m.Colored[i] += pc.payColor[i]
	}
	return m
}

// manaAsk offers the player's payment choice for the next unsettled hybrid or
// Phyrexian pip of the cost (CR 601.2b), one decision per pip. Only payment
// alternatives that are legal right now -- a hybrid half with pool mana of
// that colour left, or a Phyrexian pip's colour or two life if the payer has
// both -- are offered, with the valid one first, so the deterministic bot
// fallback (index 0) always picks a legal payment and a no-answer host never
// wedges. The offer gate (castable) already proved at least one alternative
// is available, so the decision is never empty. It returns true once it has
// asked (and therefore suspended); payCast applies the accumulated payColor /
// payLife when every pip is settled.
func (e *Engine) manaAsk() bool {
	pc := e.cast
	if pc == nil || pc.payIdx >= pc.cost.annPipCount() {
		return false
	}
	colors, lifeOK := pc.cost.announcePip(pc.payIdx)
	// remaining pool = the payer's pool minus what earlier announced pips
	// (payColor) have already reserved, and the life already committed.
	rem := e.G.Players[pc.player].Pool
	for i := range rem {
		rem[i] -= pc.payColor[i]
	}
	life := e.G.Players[pc.player].Life - pc.payLife
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose how to pay a mana symbol of " + e.G.Obj(pc.card).Face().Name,
		Source: pc.card}
	// Hybrid (and a Phyrexian pip's colour half): one option per DISTINCT
	// colour that has pool mana left, A then B. A single-colour Phyrexian pip
	// carries the same colour twice, so the seen set keeps one option for it.
	seen := map[byte]bool{}
	for _, col := range colors {
		if col == 0 || seen[col] {
			continue
		}
		seen[col] = true
		if rem[state.ManaIndex(col)] > 0 {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "pay_" + string(col), Label: "Pay " + string(col), Amount: 1})
		}
	}
	// Phyrexian: its colour (already offered above if in pool) or two life.
	if lifeOK && life >= 2 {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "pay_life", Label: "Pay 2 life", Amount: 2})
	}
	if len(d.Options) == 0 {
		// Defensive: castable already proved at least one alternative, but a
		// colourless Phyrexian pip with a colourless-only pool is offered its
		// life payment so the decision can never be empty.
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "pay_life", Label: "Pay 2 life", Amount: 2})
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// etbAnswer records one answered "as this enters" choice onto the card as a
// Choose event, before the object is put on the stack (or, for a land, before
// it moves to the battlefield), so the recorded value survives replay exactly
// as the player chose it. The value rides on Option.Label (name/type) or
// Option.Amount (number), not the choice index.
func (e *Engine) etbAnswer(d *decision.Decision, chosen []decision.Option) {
	pc := e.cast
	if pc == nil || len(chosen) != 1 {
		return
	}
	opt := chosen[0]
	switch opt.Kind {
	case "name":
		e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "name", Text: opt.Label})
	case "type":
		e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "type", Text: opt.Label})
	case "number":
		e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "number", Amount: int32(opt.Amount)})
	}
	pc.etbIdx++
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
	case "pay_W", "pay_U", "pay_B", "pay_R", "pay_G":
		// A hybrid or Phyrexian pip paid with pool mana: record which colour.
		if len(chosen) > 0 {
			pc.payColor[state.ManaIndex(chosen[0].Kind[4])]++
		}
		pc.payIdx++
	case "pay_life":
		// A Phyrexian pip paid with two life.
		pc.payLife += 2
		pc.payIdx++
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

// targetAsk is the last stage of continueCast before commitCast: it asks the
// spell or activated ability's target selection (CR 601.2c / 602.2b) while the
// proposal is still provisional -- BEFORE any cost is paid, any sacrificial
// permanent moves, or the object is put on the stack. That ordering is what
// makes the cast a transaction: the target answer (601.2c) precedes payment
// (601.2h), and the cast trigger (601.2i, fired by PutOnStack) waits until the
// proposal is complete. handleTarget (stack.go) completes the transaction by
// calling commitCast and then records the chosen targets onto the object that
// actually reached the stack.
//
// It returns true when it either asked a target decision or ABORTED the
// proposal. A proposal that can never complete is reversed here, before
// anything has been paid or moved (CR 733.1): the card left the zone, the
// resolved mana cost is no longer payable, or a mandatory target (min >= 1)
// has zero legal candidates. Clearing e.cast with nothing committed restores
// the pre-proposal board. An SA with no ValidTgts (or a zero-minimum target
// with no legal candidate, Requirement N2) returns false so commitCast runs
// directly.
func (e *Engine) targetAsk() bool {
	pc := e.cast
	if pc == nil {
		return false
	}
	o := e.G.Obj(pc.card)
	if o == nil {
		return false
	}
	f := o.Face()
	var sa *cards.SA
	if pc.ability >= 0 {
		if f == nil || pc.ability >= len(f.Abilities) {
			return false
		}
		sa = f.Abilities[pc.ability]
	} else if f != nil {
		sa = f.SpellAbility()
	}
	if sa == nil || sa.Params["ValidTgts"] == "" {
		return false
	}
	// A proposal whose resolved mana cost can no longer be paid can never
	// complete. Reverse it (CR 733.1), which undoes the pushCast stack move
	// (E2: no progress was made, so hold this card's option out of the window
	// rather than re-offering the same unpayable cast). Resolved means X is
	// fixed and Delve credit is applied, both settled by the stages above.
	mana := pc.resolvedMana()
	if pc.ability < 0 {
		mana.Generic -= int32(len(pc.delve))
		if mana.Generic < 0 {
			mana.Generic = 0
		}
	}
	if !mana.payable(e.G.Players[pc.player].Pool, e.G.Players[pc.player].Life) {
		e.abortCast(pc, "cast aborted: cost no longer payable", true)
		return true
	}
	min, max := targetBounds(sa)
	// CR 115.5: a spell may not target itself (excludeSelf == the card); an
	// activated ability CAN target its own Source permanent (Mother of Runes
	// targeting itself). The Face-less ability stack object on the stack is
	// never offered (legalTargetCandidates drops Face()-less stack objects),
	// so the source permanent is still a legal target of its own ability.
	var excludeSelf state.ObjID
	if pc.ability < 0 {
		excludeSelf = pc.card
	}
	candidates := e.legalTargetCandidates(pc.player, pc.card, excludeSelf, sa)
	if min > 0 && len(candidates) < min {
		// CR 601.2c: a proposal with fewer legal targets than its mandatory
		// minimum cannot be announced. Reverse the whole proposal (CR 733.1):
		// the pushed object returns to where it was, nothing is paid and no
		// cast trigger fires. No library was shuffled during the proposal, so
		// the 733.1 library exception does not apply.
		e.abortCast(pc, "cast aborted: no legal target", false)
		return true
	}
	if min == 0 && len(candidates) == 0 {
		// Requirement N2: a subject that MAY target zero things resolves
		// untargeted when no legal target exists; proceed straight to payCast
		// with no target decision.
		return false
	}
	// The decision's Source is the object that must not be offered as its own
	// target (CR 115.5). For a spell that is the card (excluded via
	// excludeSelf). For an activated ability the object that may not target
	// itself is the ability stack object, which is not minted yet (the push
	// is a no-op for an ability; payCast's AbilityPush creates it), so Source
	// is 0 and the source permanent remains a legal target of its own ability
	// (Mother of Runes) via excludeSelf == 0. The prompt keeps the source
	// permanent's name for readability.
	var src state.ObjID
	if pc.ability < 0 {
		src = pc.card
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KTarget, Min: min, Max: max,
		Prompt: "Choose a target for " + e.targetName(pc.card),
		Source: src, TargetEffect: describeTargetEffect(sa)}
	for _, candidate := range candidates {
		label := e.G.Players[candidate.player].Name
		if candidate.obj != 0 {
			label = e.G.Obj(candidate.obj).Face().Name + " (" + label + ")"
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: candidate.kind,
			Label: label, Obj: candidate.obj, Player: candidate.player})
	}
	e.ask(d)
	return true
}

// pushCast implements CR 601.2a: the card reaches the stack BEFORE the
// target choice (601.2c) and payment (601.2h), which is what makes the
// transaction match the CR's ordered list. The cast trigger (601.2i) is held
// back by emit (deferCastTrigger) because it fires only once the spell is
// actually cast -- after payment; payCast's fireDeferredCastTrigger re-walks
// the held PutOnStack. CastInfo (the X / mode-flag recording) is deferred to
// payCast too, so an aborted proposal leaves no cast-time trace on the card.
// An activated ability is a no-op here: CR 602.2b imports 601.2 but its
// stack object is minted by payCast's AbilityPush AFTER the target and cost
// settle, and an aborted activation must reverse with no stack object left
// behind (CR 733.1) -- pushing it first would strand a Face-less object in
// exile. A land play never goes on the stack.
//
// It returns true when the cast cannot proceed at all (the card left its
// zone before the push) and was aborted; in that case nothing was pushed, so
// no reversal is owed.
func (e *Engine) pushCast() bool {
	pc := e.cast
	if pc == nil || pc.mode == "land" || pc.ability >= 0 {
		return false
	}
	o := e.G.Obj(pc.card)
	if o == nil || o.Zone != pc.from {
		e.cast, e.choosing = nil, chooseNone
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Text: "cast aborted: the card moved"})
		return true
	}
	// Capture the held-out suppression set before the push, so an aborted
	// (reversed) cast can restore it -- the push below is a state-changing
	// event that emit treats as progress and clears the set, yet an aborted
	// cast is net no progress.
	pc.preSuppress = e.suppressedCast
	// CR 601.2a: the spell reaches the stack. The cast trigger is held back
	// (deferCastTrigger) so it cannot fire before the spell is paid for.
	e.deferCastTrigger = true
	e.emit(events.Event{Kind: events.PutOnStack, Obj: pc.card, Player: pc.player, From: pc.from, To: state.ZStack, Text: o.Face().Name})
	e.deferCastTrigger = false
	pc.stackObj = pc.card
	pc.pushed = true
	// CR 903.8: the cast counter increments the INSTANT the spell is put on
	// the stack, never when it resolves -- so a commander spell that is later
	// countered still raises the next cast's tax. Only a cast FROM the
	// command zone counts, and recordCmdCast itself carries the Commander
	// format gate.
	if pc.from == state.ZCommand {
		e.recordCmdCast(pc.player, pc.card)
	}
	return false
}

// payCast implements CR 601.2h (pay all costs) and, for a spell, CR 601.2i
// (the "when you cast" trigger). It runs after the target choice (601.2c);
// the object is already on the stack (pushCast). A payment that fails here
// (a pool that changed under a hand-built intent -- castable already gated
// the option the caster chose, so this is not reachable from an ordinary,
// well-formed client) aborts and REVERSES the push (CR 733.1): the object
// returns to the zone it came from, nothing remains paid and no cast trigger
// fires. The land play is also handled here (it never goes on the stack).
func (e *Engine) payCast() {
	pc := e.cast
	if pc == nil {
		return
	}
	if pc.mode == "land" {
		// Task 12: a land played through the one-stage flow (an "as this
		// enters" choice, e.g. Cavern of Souls). The choice was already
		// recorded by etbAnswer; now move it onto the battlefield and log the
		// land play, exactly the two events handlePriority's no-choice
		// play_land path emits. Its own MoveZone routes through
		// applyReplacements, so an ETBReplacement on the land itself (or its
		// choice already recorded) resolves on entry.
		e.cast, e.choosing = nil, chooseNone
		e.emit(events.Event{Kind: events.MoveZone, Obj: pc.card, From: pc.from, To: state.ZBattlefield})
		e.emit(events.Event{Kind: events.LandPlayed, Player: pc.player})
		return
	}
	if pc.ability >= 0 {
		// Task 10: an activated ability. The shared stages above (X, Delve --
		// never present on an ability --, Sac) have already run and been
		// recorded; what differs from a spell here is the cost's remaining
		// non-mana parts. Pay mana, then each Tap (a Tap event), each
		// SubCounter part (a CounterChange of -N), and every chosen sacrifice.
		// The ability object was already minted by pushCast; targets are
		// recorded onto it by handleTarget.
		mana := pc.resolvedMana()
		if !e.payMana(pc.player, mana) {
			e.abortCast(pc, "activation aborted: cost no longer payable", true)
			return
		}
		if pc.payLife != 0 {
			e.emit(events.Event{Kind: events.LifeChange, Player: pc.player, Amount: -pc.payLife})
		}
		for _, id := range pc.delve {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile, Text: "delved"})
		}
		if pc.cost.Tap {
			e.emit(events.Event{Kind: events.Tap, Obj: pc.card})
		}
		for _, part := range pc.cost.SubCounter {
			e.emit(events.Event{Kind: events.CounterChange, Obj: pc.card, Counter: part.Spec, Amount: -part.N})
		}
		for _, id := range pc.sacs {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
		}
		// AbilityPush mints the ability object onto the stack AFTER the cost
		// settles, so an aborted activation leaves no stack object behind
		// (CR 733.1). handleTarget records the chosen targets onto it.
		e.emit(events.Event{Kind: events.AbilityPush, Obj: pc.card, Player: pc.player, Amount: int32(pc.ability)})
		if len(e.G.Stack) > 0 {
			pc.stackObj = e.G.Stack[len(e.G.Stack)-1]
		}
		e.cast, e.choosing = nil, chooseNone
		return
	}
	mana := pc.resolvedMana()
	mana.Generic -= int32(len(pc.delve))
	if mana.Generic < 0 {
		mana.Generic = 0
	}
	if !e.payMana(pc.player, mana) {
		// E2 (round 2). This is the reachable no-progress arm: a Delve exile
		// ask (Min:0, Max the shortfall) was answered with fewer cards than
		// the shortfall needs, so the cast aborts with no state change and
		// priority re-offers it. Declining is a legal, conforming answer --
		// CR 601.2h rewinds the cast -- but the engine must not re-offer the
		// SAME unpayable cast forever. Hold THIS card's cast option out of the
		// remaining priority window (suppressCast); the suppression clears on
		// the first state-changing event, so the option returns as soon as the
		// window ends or the mana/board changes.
		e.abortCast(pc, "cast aborted: cost no longer payable", true)
		return
	}
	if pc.payLife != 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: pc.player, Amount: -pc.payLife})
	}
	for _, id := range pc.delve {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile, Text: "delved"})
	}
	for _, id := range pc.sacs {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard, Text: "sacrificed"})
	}
	// CR 601.2b: record how the spell was cast (the X value and mode flags).
	// Deferred to payment rather than the up-front push so an aborted
	// proposal leaves no cast-time trace on the card. A cast trigger that
	// reads the mode (e.g. "cast a kicked spell") sees it, because the flag
	// is applied before the trigger fires next.
	if flags := modeFlags(pc.mode); pc.x != 0 || flags != "" {
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: pc.x, Counter: flags})
	}
	// CR 601.2i: the "when you cast" trigger, held back from the up-front
	// push, fires now -- only after the spell is paid for.
	e.fireDeferredCastTrigger()
	e.cast, e.choosing = nil, chooseNone
}

// abortCast reverses a cast or activation proposal that cannot complete, per
// CR 733.1. If the object was already pushed (CR 601.2a / 602.2a), the
// reversal undoes the push: a spell returns to the zone it came from, and an
// ability's stack object leaves the stack (it ceases to exist, moved to exile
// as the existing ability-fizzle resting place does). Nothing is paid and
// the held cast trigger is dropped. e.cast and e.choosing are cleared.
func (e *Engine) abortCast(pc *pendingCast, text string, suppress bool) {
	// The reversal below and the push that preceded it are state-changing
	// events to emit's suppression-clearing rule, but their NET effect is no
	// progress -- the object returns to the zone it came from -- so the
	// held-out no-progress set must survive. For a pushed spell that set is
	// pc.preSuppress (captured before the push, which cleared it); for an
	// ability (never pushed) it is the current set. When suppress is true, the
	// no-progress decline of THIS object is added back.
	var saved map[state.ObjID]bool
	if pc.pushed && pc.preSuppress != nil {
		saved = make(map[state.ObjID]bool, len(pc.preSuppress))
		for id := range pc.preSuppress {
			saved[id] = true
		}
	} else if e.suppressedCast != nil {
		saved = make(map[state.ObjID]bool, len(e.suppressedCast))
		for id := range e.suppressedCast {
			saved[id] = true
		}
	}
	if suppress {
		if saved == nil {
			saved = map[state.ObjID]bool{}
		}
		saved[pc.card] = true
	}
	if pc.pushed && pc.stackObj != 0 {
		if pc.ability >= 0 {
			e.emit(events.Event{Kind: events.MoveZone, Obj: pc.stackObj, From: state.ZStack, To: state.ZExile, Text: "reversed"})
		} else {
			e.emit(events.Event{Kind: events.MoveZone, Obj: pc.stackObj, From: state.ZStack, To: pc.from, Text: "reversed"})
		}
	}
	e.deferredPush = nil
	e.deferredPushLKI = nil
	e.cast, e.choosing = nil, chooseNone
	e.emit(events.Event{Kind: events.Note, Player: pc.player, Text: text})
	e.suppressedCast = saved
}

// fireDeferredCastTrigger re-walks the up-front PutOnStack event that
// pushCast held back (deferredPush) so the CR 601.2i "when you cast" triggers
// fire, which is only after the spell is paid for. It is called from payCast
// for a spell; a no-op when nothing was deferred (an ability, a land, or an
// aborted proposal).
func (e *Engine) fireDeferredCastTrigger() {
	if e.deferredPush == nil {
		return
	}
	ev := e.deferredPush
	e.deferredPush = nil
	lki := e.deferredPushLKI
	e.deferredPushLKI = nil
	e.checkTriggers(*ev, lki)
}

// recordCmdCast increments the CmdCasts[k] bookkeeping parallel to
// Commanders[k] for a commander cast from the command zone. It is called by
// commitCast only when a command-zone cast's PutOnStack was just appended, so
// a cast from any other zone (hand, graveyard-flashback, ...) is never
// counted here.
//
// The count it maintains is DERIVED state, and the brief's "derived from
// events on replay" is the right of its two options for exactly this reason:
// CmdCasts[k] is a deterministic pure function of the already-logged
// PutOnStack events (From == ZCommand, per commander id). The generic,
// format-agnostic events.Apply handler is the wrong home for it -- this is a
// Commander-format rule, not a universally-applicable state transition -- so
// it is maintained as a projection at the exact point its authoritative event
// is appended, which introduces no new degree of freedom: a faithful replay,
// which re-runs this same beginCast -> commitCast path against the recorded
// Intents, appends the identical PutOnStack events and so lands on the
// identical count. Clone deep-copies the slice (state goes through
// Game.Clone) so the O(1) tax read (commanderTaxFor) survives a clone; the
// number itself comes from the event stream alone. No event of its own is
// needed, and events/ is outside this task's boundary.
func (e *Engine) recordCmdCast(p state.PlayerID, id state.ObjID) {
	if e.format != FormatCommander {
		return
	}
	for k, cid := range e.G.Players[p].Commanders {
		if cid == id {
			e.G.Players[p].CmdCasts[k]++
			return
		}
	}
}

// suppressCast holds id's cast option out of the current priority window
// (suppressedCast, engine.go) because its last cast/activation attempt
// aborted unpayable with no state change. The offer walks (rules/legal.go)
// skip it via castSuppressed, so the no-progress re-offer loop commitCast
// describes cannot repeat -- a legal decline never kills the match, and the
// seat may still do anything else; the suppressed card's option comes back
// on the first state-changing event (engine.go's emit clears the whole set),
// which is when the window ends or the mana/board changes.
func (e *Engine) suppressCast(id state.ObjID) {
	if e.suppressedCast == nil {
		e.suppressedCast = map[state.ObjID]bool{}
	}
	e.suppressedCast[id] = true
}

// castSuppressed reports whether id's cast option is currently held out of
// p's priority offers (see suppressCast). The id names the one seat holding
// it, so p is not consulted beyond matching that id's zone in the walk that
// called it.
func (e *Engine) castSuppressed(p state.PlayerID, id state.ObjID) bool {
	return e.suppressedCast != nil && e.suppressedCast[id]
}

func init() {
	effects.RegisterNonAPI("kw:Kicker", "kw:Surge", "kw:Flashback", "kw:Delve")
}
