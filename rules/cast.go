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
			out = append(out, decision.Option{Index: i, Kind: "number", Label: strconv.Itoa(i), Amount: i})
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
	if pc.mode == "land" {
		// Task 12: a land played through the one-stage flow (an "as this
		// enters" choice, e.g. Cavern of Souls). The choice was already
		// recorded by etbAnswer; now move it onto the battlefield and log the
		// land play, exactly the two events handlePriority's no-choice
		// play_land path emits. Its own MoveZone routes through
		// applyReplacements, so an ETBReplacement on the land itself (or its
		// choice already recorded) resolves on entry.
		e.emit(events.Event{Kind: events.MoveZone, Obj: pc.card, From: pc.from, To: state.ZBattlefield})
		e.emit(events.Event{Kind: events.LandPlayed, Player: pc.player})
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
