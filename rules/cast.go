// cast.go is the cast-flow state machine: beginCast starts it from a chosen
// "cast" priority option, continueCast runs its stages (X, Delve, each Sac
// and Discard part) in order, asking a KChoose (chooseCast) decision for any stage that
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
	"strings"

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

	discards    []state.ObjID
	discardPart int

	// payIdx / payColor / payLife / payGeneric carry the flexible-pip payment
	// announcement (CR 601.2b/107.4e-f). manaAsk walks the cost's combined
	// announcement-pip list one decision at a time; payIdx is the next
	// unsettled pip, payColor accumulates the coloured spend the announced
	// pips chose, payLife the life a Phyrexian face paid with two life costs,
	// and payGeneric the generic a monocolour hybrid pip paid with its
	// generic face. Plain data, so Clone copies it like x/delve/sacs/discards.
	payIdx     int
	payColor   state.Mana
	payLife    int32
	payGeneric int32

	// mods / taxGeneric carry the CR 601.2f cost composition: the evaluated
	// RaiseCost/ReduceCost modifiers (computed in beginCast for a spell,
	// beginActivation for an ability — Color$ reductions, MinMana$ floors and
	// the SetCost floor included) and the CR 903.8 commander tax. They are
	// applied to the mana cost only AFTER {X} is folded into Generic
	// (manaToPay), so an {X} reduction is not lost and the tax (an additional
	// cost) is never reduced -- increases before reductions, per 601.2f.
	mods       costMods
	taxGeneric int32

	// windowDone is set when the 601.2g mana window was answered "done", so
	// payCast proceeds straight to payment instead of re-offering it.
	windowDone bool
	// modesDone is set once a modal spell's CR 601.2b mode question has been
	// posed. modeChosen says its answer was recorded during this proposal;
	// preModes is the object's value immediately before that answer, so
	// abortCast can restore it under CR 733.1.
	modesDone  bool
	modeChosen bool
	preModes   []string

	// passedTarget is set once the flow has moved past the 601.2c target
	// choice into payCast, so a resume through continueCast (the mana-window
	// re-entry) does not re-ask for targets.
	passedTarget bool

	// targets are the chosen cast-time targets while this proposal is live.
	// They are copied from the target decision before payment so a ValidTarget$
	// cost modifier can be recomputed after CR 601.2c and before 601.2h, even
	// for an activated ability whose stack object is not minted until payment.
	targets []state.Target

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

	// preAborts is the castAborts no-progress count map (engine.go) as it was
	// just before pushCast's PutOnStack, captured and restored for exactly the
	// reason preSuppress is: the push is a state-changing event that emit
	// treats as progress and clears the count, but an aborted cast is net no
	// progress, so the count must come back across the push (F05-2). Nil when
	// no cast push is in flight.
	preAborts map[state.ObjID]int32

	// Task 12: the card's "as this enters" choices (one per ETBReplacement
	// Repl whose ReplaceWith$ is NameCard/ChooseType/ChooseNumber). Each is
	// asked in order while choosing == chooseETB; etbIdx is the next
	// unsettled one, so the continuation survives the chooseAnswer round trip
	// (answer -> etbAnswer increments etbIdx -> continueCast re-enters
	// etbAsk). Plain data, so a Clone copies it like the fields above.
	etbs   []etbChoice
	etbIdx int

	// etbChosen records whether THIS proposal actually emitted an "as this
	// enters" Choose event, and the object's choice fields as they were
	// immediately before the first one (captured in etbAnswer, before it
	// records the answered choice). An aborted proposal (CR 733.1) must
	// return the object to the moment before it was proposed, so abortCast
	// emits reverse Choose events restoring these captured values -- but ONLY
	// when a choice was recorded during this proposal (a spell that never
	// chose anything emits nothing, so no chain head moves for it). Capturing
	// all three fields at the first answer means a card with several etb
	// choices restores the true pre-proposal state, not the state after the
	// first answer.
	etbChosen bool
	etbName   string
	etbType   string
	etbNumber int32
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
// matching permanents on p's battlefield; every Discard part is payable from
// p's hand; every SubCounter part's N does
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
func (e *Engine) castable(p state.PlayerID, id state.ObjID, cost Cost, ability bool) bool {
	mana := cost
	mana.Generic -= e.delveCredit(p, id, mana.Generic)
	if !mana.payable(e.G.Players[p].Pool, e.G.Players[p].Snow, e.G.Players[p].Life) {
		return false
	}
	return e.nonManaCastable(p, id, cost, ability)
}

// nonManaCastable is castable's payment-independent tail. Cost-modifier
// offer checks use it after their flexible-pip walk has established a payable
// resolved mana face: applying Color$ before that walk would otherwise see a
// hybrid pip as neither of its colours and withhold a cast that the eventual
// announced face can legally make free. Keeping all non-mana checks in this
// one helper means that specialized offer logic cannot bypass Sac/Discard/
// counter/tap legality.
func (e *Engine) nonManaCastable(p state.PlayerID, id state.ObjID, cost Cost, ability bool) bool {
	reserved := map[state.ObjID]bool{}
	for _, part := range cost.Sac {
		var avail []state.ObjID
		matchSpec := sacrificeMatchSpec(part.Spec)
		for _, oid := range e.G.Zone(state.ZBattlefield, p) {
			if reserved[oid] { // an earlier Sac part already claimed this one
				continue
			}
			if effects.MatchesSpecFrom(e.G, matchSpec, oid, p, id) {
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
	if !e.discardCostPayable(p, id, cost.Discard, !ability) {
		return false
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

// sacrificeMatchSpec normalizes Forge's NICKNAME spelling to CARDNAME before
// the source-aware filter is applied. The filter owns CARDNAME's object-ID
// semantics; costs use this helper at both offer and payment time so the two
// stages cannot disagree about whether a self-reference is payable.
func sacrificeMatchSpec(spec string) string {
	if strings.EqualFold(spec, "NICKNAME") {
		return "CARDNAME"
	}
	return spec
}

// discardCandidates returns the still-available cards that can pay one
// Discard cost part. Random names a selection method rather than a card
// characteristic, and a Hand spec is Forge's "discard your hand" shape
// (the corpus spells its ignored count as both 0 and 1).
// A spell being announced is excluded because it will be on the stack when
// costs are paid; an activated ability's source may remain in hand and can
// therefore pay CARDNAME/NICKNAME costs such as channel and bloodrush.
func (e *Engine) discardCandidates(p state.PlayerID, source state.ObjID, part CostPart, casting bool, reserved map[state.ObjID]bool) []state.ObjID {
	all := strings.EqualFold(part.Spec, "Random") || strings.EqualFold(part.Spec, "Hand")
	matchSpec := part.Spec
	if strings.EqualFold(matchSpec, "NICKNAME") {
		// Forge uses NICKNAME as the same self-reference as CARDNAME in the
		// four discard-cost lines that carry it.
		matchSpec = "CARDNAME"
	}
	var out []state.ObjID
	for _, id := range e.G.Zone(state.ZHand, p) {
		if reserved[id] || (casting && id == source) {
			continue
		}
		if all || effects.MatchesSpecFrom(e.G, matchSpec, id, p, source) {
			out = append(out, id)
		}
	}
	return out
}

// discardCostPayable is the offer-side totality gate for Discard costs. It
// mirrors discardAsk's deterministic reservation walk without consuming RNG.
func (e *Engine) discardCostPayable(p state.PlayerID, source state.ObjID, parts []CostPart, casting bool) bool {
	reserved := map[state.ObjID]bool{}
	for _, part := range parts {
		candidates := e.discardCandidates(p, source, part, casting, reserved)
		if strings.EqualFold(part.Spec, "Hand") {
			for _, id := range candidates {
				reserved[id] = true
			}
			continue
		}
		if part.N <= 0 || int32(len(candidates)) < part.N {
			return false
		}
		for i := int32(0); i < part.N; i++ {
			reserved[candidates[i]] = true
		}
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

// withSpellAbilityExtras folds a spell's own SpellAbility Cost$ ADDITIONAL
// (non-mana) parts into cost. It exists so the OFFER and the CHARGE cannot
// disagree about what a plain cast costs.
//
// They did disagree, and it wedged a live game. legal.go offered a plain cast
// after gating castable on adjustedCost alone -- the printed mana -- while
// beginCast folded the SpellAbility's Cost$ Sac part in afterwards. Village
// Rites ({B}, "As an additional cost, sacrifice a creature") was therefore
// offered to a player with no creature: sacAsk found zero candidates and
// aborted the cast, the abort consumed nothing, priority returned to a board
// identical to the one that produced the offer, and the same option was
// offered again -- an unbounded livelock (measured: a 5-event cycle repeating
// until the match was killed). The abort in sacAsk is correct and stays; what
// was wrong is that the option existed at all, which is the standing rule that
// an option that cannot be paid must never be offered.
//
// The alternative-cost path was given this same gate in an earlier round (see
// the ruling comment in legal.go's alternativeCosts loop). The base cast path
// has the identical hole and was missed, so both now go through this one
// definition rather than each repeating the fold.
//
// The mana part of a Cost$ is deliberately NOT folded: it RESTATES the printed
// mana cost rather than adding to it, so re-adding it would double charge.
// Only Life/Sac/Discard/SubCounter/Tap are additional.
func withSpellAbilityExtras(f *cards.Face, cost Cost) Cost {
	sa := f.SpellAbility()
	if sa == nil {
		return cost
	}
	sc := sa.Params["Cost"]
	if sc == "" {
		return cost
	}
	extra := ParseCost(sc)
	cost.Life = addClampedGeneric(cost.Life, int64(extra.Life))
	if len(extra.Sac) > 0 {
		cost.Sac = append(append([]CostPart(nil), cost.Sac...), extra.Sac...)
	}
	if len(extra.Discard) > 0 {
		cost.Discard = append(append([]CostPart(nil), cost.Discard...), extra.Discard...)
	}
	if len(extra.SubCounter) > 0 {
		cost.SubCounter = append(append([]CostPart(nil), cost.SubCounter...), extra.SubCounter...)
	}
	cost.Tap = cost.Tap || extra.Tap
	return cost
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
	//
	// The cost stored here is the RAW selected cost, with no cost modifiers
	// (CR 601.2f): RaiseCost/ReduceCost are applied later, in manaToPay,
	// once {X} is folded into Generic. adjustedCost's modifier-into-generic
	// form must not be stored here, or a spell with an {X} in it would have
	// its reduction applied before X is known (and lost), and a
	// flashback/alternative recast would drop the modifiers entirely.
	cost := e.rawBaseCost(p, id)
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
	// (Sac/Discard/SubCounter/Tap) are additional and fold into the total cost here; a
	// re-added mana part would double charge. Only a plain cast reaches this
	// (pc.ability < 0 and no alternative/flashback recast), and a spell with
	// no SP Cost$ contributes nothing.
	if opt.AltCostIndex == 0 && opt.Mode == "" {
		cost = withSpellAbilityExtras(f, cost)
	}
	// CR 903.8: the commander tax, applied to whatever cost this cast pays
	// (the base/alternative/kicked/flashback/surged/miracle cost resolved
	// above) -- the exact same commanderTaxFor the command-zone offer in
	// legal.go gated castable on, over the same board, so this charge and
	// that offer can never disagree. For a command-zone commander this is the
	// plain base + the tax; for every other card/zone it passes cost through
	// unchanged (commanderTaxFor is a no-op outside the Commander format and
	// off the command zone). It lands AFTER cost modifiers and any keyword
	// recast, so an additional cost is never reduced by them, in line with how
	// Kicker's own additional cost composes.
	//
	// The tax is captured as a separate generic amount (taxGeneric) rather
	// than folded into cost, so manaToPay adds it AFTER the 601.2f modifiers
	// and never lets those spill onto it.
	tax := e.commanderTaxAmount(p, id)
	mods := e.costModifiers(p, id, spellScope(opt.Mode))
	e.cast = &pendingCast{player: p, card: id, from: from, mode: opt.Mode, ability: -1,
		cost: cost, mods: mods, taxGeneric: tax}
	e.collectETBChoices(p)
	e.continueCast()
}

// continueCast runs the cast flow's stages in order -- X, Delve, each Sac
// and Discard part -- stopping (and returning) the instant a stage asks a KChoose;
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
	if e.discardAsk() {
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
	// CR 601.2b: a modal spell announces its modes after reaching the stack
	// and before targets are chosen or costs are paid. The answer is cached on
	// the proposed spell so targetAsk can inspect the selected mode and
	// resolution can execute it without asking again.
	if e.castModeAsk() {
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

// castModeAsk poses CR 601.2b's mode announcement for a modal spell. It uses
// KModes like the placement and resolution paths, but ResumeKind distinguishes
// this cast-transaction continuation from both: handleModes records the answer
// on the proposed spell and re-enters continueCast rather than resuming an
// effect or the trigger drain. Activated abilities retain their existing
// resolution-time behaviour; CR 603.3c triggered abilities remain owned by
// askTriggerModes.
func (e *Engine) castModeAsk() bool {
	pc := e.cast
	if pc == nil || pc.ability >= 0 || pc.modesDone {
		return false
	}
	pc.modesDone = true
	o := e.G.Obj(pc.card)
	if o == nil || o.Face() == nil {
		return false
	}
	f := o.Face()
	sa := f.SpellAbility()
	if sa == nil || sa.API != "Charm" || strings.TrimSpace(sa.Params["Choices"]) == "" {
		return false
	}
	ctx := &effects.Ctx{Source: pc.card, Controller: pc.player}
	effects.SetSVars(ctx, f.SVars)
	charmNum := effects.Num(e, ctx, sa, "CharmNum", 1)
	if charmNum < 1 {
		charmNum = 1
	}
	choices := strings.Split(sa.Params["Choices"], ",")
	legal := make([]string, 0, len(choices))
	for _, name := range choices {
		name = strings.TrimSpace(name)
		sub := cards.ResolveSVar(f.SVars, name)
		if sub == nil || sub.Params["ValidTgts"] == "" {
			legal = append(legal, name)
			continue
		}
		min, _ := targetBounds(sub)
		if len(e.legalTargetCandidates(pc.player, pc.card, pc.card, sub)) >= min {
			legal = append(legal, name)
		}
	}
	if int(charmNum) > len(legal) {
		// No legal set of modes can complete its mandatory target choices. This
		// is the modal counterpart of targetAsk's no-legal-target reversal; use
		// the no-progress suppression so an automated seat cannot propose the
		// same impossible cast forever.
		e.abortCast(pc, "cast aborted: no legal modal choice", true)
		return true
	}
	d := modeDecisionForChoices(pc.player, pc.card, sa, f.SVars, legal, int(charmNum))
	d.ResumeKind = "cast_modes"
	e.ask(d)
	return true
}

// modalTargetSA returns the target declaration selected by a modal spell.
// Forge puts a Charm's ValidTgts$ on each Choices$ SVar rather than on the
// outer Charm SA. This engine has one target list per stack object, so when
// several chosen modes target independently it can currently carry only the
// first target-bearing mode; the ordinary one-mode Charm shape is exact.
func modalTargetSA(f *cards.Face, sa *cards.SA, modes []string) *cards.SA {
	if sa == nil || sa.Params["ValidTgts"] != "" || sa.API != "Charm" || f == nil {
		return sa
	}
	for _, name := range modes {
		if sub := cards.ResolveSVar(f.SVars, name); sub != nil && sub.Params["ValidTgts"] != "" {
			return sub
		}
	}
	return sa
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
		wx := e.manaToPayX(pc, x)
		wx.Generic -= e.delveCredit(pc.player, pc.card, wx.Generic)
		if !wx.payable(pool, e.G.Players[pc.player].Snow, e.G.Players[pc.player].Life) {
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
	cost := e.manaToPay(pc)
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
		matchSpec := sacrificeMatchSpec(part.Spec)
		var candidates []state.ObjID
		for _, oid := range e.G.Zone(state.ZBattlefield, pc.player) {
			if effects.MatchesSpecFrom(e.G, matchSpec, oid, pc.player, pc.card) {
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
			// paid (fix round 1, reviewer Important 1). Abort the whole
			// thing; nothing has moved yet.
			//
			// This site used to hand-roll the teardown (clear e.cast, emit the
			// Note) on the reasoning that it was "unreachable from a
			// well-formed offer after the castable gate". It was reachable,
			// and hand-rolling it is what made that reachability unbounded
			// rather than merely wasteful: abortCast is where a no-progress
			// abort holds the option out of the rest of the priority window
			// (suppress=true), and skipping it meant the identical board
			// re-offered the identical doomed cast forever. A live 4-player
			// game sat on turn 3 doing that until it was killed.
			//
			// Route through abortCast like every other unpayable-cost abort,
			// so this path gets the same liveness guarantee the Delve decline
			// has (see cast_liveness_test.go): the suppression lifts on the
			// first state-changing event, which is exactly when a retry could
			// succeed.
			e.abortCast(pc, "sacrifice cost no longer payable; cast/activation aborted", true)
			return true
		}
		// CARDNAME and NICKNAME are bare source-object references in Forge
		// sacrifice costs. When the source is their sole candidate, this exact
		// one-object payment has no player choice: record it and settle the next
		// cost part instead of posing a KChoose the player can only answer one
		// way. The candidate check above deliberately stays first, so a source
		// that has left the battlefield still takes the ordinary unpayable-cost
		// abort path.
		if part.N == 1 && len(candidates) == 1 && candidates[0] == pc.card &&
			(strings.EqualFold(part.Spec, "CARDNAME") || strings.EqualFold(part.Spec, "NICKNAME")) {
			pc.sacs = append(pc.sacs, pc.card)
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

// discardAsk settles Discard cost parts from the payer's hand. Ordinary
// specs pose the same exact-N KChoose used by sacrifice costs. Random parts
// consume the engine's seeded RNG and never ask the player; a Hand spec records
// every remaining hand card without asking. Nothing moves until commitCast,
// so an abort cannot leave a partially paid cost on the board.
func (e *Engine) discardAsk() bool {
	pc := e.cast
	for pc.discardPart < len(pc.cost.Discard) {
		part := pc.cost.Discard[pc.discardPart]
		reserved := make(map[state.ObjID]bool, len(pc.discards))
		for _, id := range pc.discards {
			reserved[id] = true
		}
		candidates := e.discardCandidates(pc.player, pc.card, part, pc.ability < 0, reserved)

		if strings.EqualFold(part.Spec, "Hand") {
			pc.discards = append(pc.discards, candidates...)
			pc.discardPart++
			continue
		}
		n := int(part.N)
		if n <= 0 || n > len(candidates) {
			e.abortCast(pc, "discard cost no longer payable; cast/activation aborted", true)
			return true
		}
		if strings.EqualFold(part.Spec, "Random") {
			for i := 0; i < n; i++ {
				pick := e.Rand(len(candidates))
				pc.discards = append(pc.discards, candidates[pick])
				candidates = append(candidates[:pick], candidates[pick+1:]...)
			}
			pc.discardPart++
			continue
		}

		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: n, Max: n,
			Prompt: "Discard a card to pay the cost of " + e.G.Obj(pc.card).Face().Name,
			Source: pc.card}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "discard",
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

// announcePip resolves the i-th announcement pip of a cost's hybrid →
// monocolour-hybrid → Phyrexian → hybrid-Phyrexian list into its alternative
// payments, in the order manaAsk offers them (each colour, then a generic
// face, then life). It is the single source both manaAsk (the ask's
// valid-option set) and castAnswer (recording the choice) consult, so the
// option offered and the recorded choice always agree. Snow pips are not
// announcement pips: a {S} pip has no alternative payment to announce.
func (c Cost) announcePip(i int) []pipAlt {
	if i < len(c.Hybrid) {
		p := c.Hybrid[i]
		return []pipAlt{{color: p.A}, {color: p.B}}
	}
	i -= len(c.Hybrid)
	if i < len(c.Twobrid) {
		t := c.Twobrid[i]
		alts := []pipAlt{{color: t.Col}}
		if t.Generic > 0 {
			alts = append(alts, pipAlt{generic: t.Generic})
		}
		return alts
	}
	i -= len(c.Twobrid)
	if i < len(c.Phyrexian) {
		letter := c.Phyrexian[i]
		return []pipAlt{{color: letter}, {life: 2}}
	}
	i -= len(c.Phyrexian)
	hp := c.HybridPhyrexian[i]
	return []pipAlt{{color: hp.A}, {color: hp.B}, {life: 2}}
}

// annPipCount is how many announcement pips a cost carries: the two-colour
// hybrids, the monocolour hybrids, the Phyrexian pips and the
// hybrid-Phyrexian pips (snow pips have nothing to announce).
func (c Cost) annPipCount() int {
	return len(c.Hybrid) + len(c.Twobrid) + len(c.Phyrexian) + len(c.HybridPhyrexian)
}

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
	m.Twobrid = nil
	m.HybridPhyrexian = nil
	for i := range pc.payColor {
		m.Colored[i] += pc.payColor[i]
	}
	m.Generic += pc.payGeneric
	return m
}

// resolvedMana is the X-folded, pip-resolved cost (see above). manaToPay is
// the full CR 601.2f composition on top of it.
func (pc *pendingCast) resolvedManaX(x int32) Cost {
	m := pc.cost.WithX(x)
	m.Hybrid = nil
	m.Phyrexian = nil
	m.Twobrid = nil
	m.HybridPhyrexian = nil
	for i := range pc.payColor {
		m.Colored[i] += pc.payColor[i]
	}
	m.Generic += pc.payGeneric
	return m
}

// repriceForTargets refreshes the modifier snapshot after CR 601.2c chooses
// targets and before CR 601.2h pays. ValidTarget$ is necessarily unavailable
// at the initial offer, but it is a cost requirement rather than a
// resolution-time condition, so this is the one point every spell and
// activation can apply it. It deliberately preserves taxGeneric: commander
// tax is independent of the chosen target and is captured at proposal start.
func (e *Engine) repriceForTargets(pc *pendingCast) {
	o := e.G.Obj(pc.card)
	if o == nil || o.Face() == nil {
		return
	}
	scope := spellScope(pc.mode)
	if pc.ability >= 0 {
		if pc.ability >= len(o.Face().Abilities) {
			return
		}
		scope = abilityScope(o.Face().Abilities[pc.ability])
	}
	pc.mods = e.costModifiersForTargets(pc.player, pc.card, scope, pc.targets)
}

// targetDependentCostMayPay is targetAsk's pre-payment exception: before a
// target is selected, the ordinary modifier snapshot intentionally excludes
// ValidTarget$ statics. Do not abort the proposal merely because that base
// snapshot is unaffordable when some legal target can make a reduction apply;
// repriceForTargets will replace the potential snapshot with the actual one
// as soon as the target answer arrives.
func (e *Engine) targetDependentCostMayPay(pc *pendingCast) bool {
	scope, ok := e.pendingCastScope(pc)
	if !ok {
		return false
	}
	mods := e.costModifiersForPotentialTargets(pc.player, pc.card, scope, e.costPotentialTargets(pc.player, pc.card, scope))
	m := mods.apply(pc.resolvedMana())
	m.Generic = addClampedGeneric(m.Generic, int64(pc.taxGeneric))
	if pc.ability < 0 {
		m.Generic -= int32(len(pc.delve))
		if m.Generic < 0 {
			m.Generic = 0
		}
	}
	pl := e.G.Players[pc.player]
	return m.payable(pl.Pool, pl.Snow, pl.Life)
}

// pendingCastScope returns the exact spell or ability scope whose modifiers
// price pc. Keeping this derivation shared by the potential-target gate and
// the final target menu makes a new ValidSpell$/Type$ rule reach both sides
// of the cast transaction rather than admitting a target the payment phase
// will price under different modifiers.
func (e *Engine) pendingCastScope(pc *pendingCast) (costScope, bool) {
	o := e.G.Obj(pc.card)
	if o == nil || o.Face() == nil {
		return costScope{}, false
	}
	if pc.ability < 0 {
		return spellScope(pc.mode), true
	}
	if pc.ability >= len(o.Face().Abilities) {
		return costScope{}, false
	}
	return abilityScope(o.Face().Abilities[pc.ability]), true
}

// affordableTargetCandidates filters legal CR 115 targets to the choices
// whose final target-dependent cost can complete this transaction. A target
// is tested as the sole selection: that is exact for the normal one-target
// shape and conservatively safe for multi-target declarations (where the
// decision API cannot express that one option requires another option).
func (e *Engine) affordableTargetCandidates(pc *pendingCast, candidates []targetCandidate) []targetCandidate {
	scope, ok := e.pendingCastScope(pc)
	if !ok {
		return nil
	}
	out := make([]targetCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		target := state.Target{Obj: candidate.obj}
		if candidate.kind == "player" {
			target = state.Target{Player: candidate.player, IsPlayer: true}
		}
		mods := e.costModifiersForTargets(pc.player, pc.card, scope, []state.Target{target})
		cost := mods.apply(pc.resolvedMana())
		cost.Generic = addClampedGeneric(cost.Generic, int64(pc.taxGeneric))
		if pc.ability < 0 {
			cost.Generic -= int32(len(pc.delve))
			if cost.Generic < 0 {
				cost.Generic = 0
			}
		}
		// Mana abilities cannot make a non-mana payment or a life shortage
		// disappear, so preserve a candidate for the mana window only after
		// those independent requirements pass.
		if !e.nonManaCastable(pc.player, pc.card, cost, pc.ability >= 0) {
			continue
		}
		pl := e.G.Players[pc.player]
		if cost.Life > pl.Life {
			continue
		}
		if cost.payable(pl.Pool, pl.Snow, pl.Life) ||
			(cost.hasManaPayment() && e.hasUntappedManaSource(pc.player)) {
			out = append(out, candidate)
		}
	}
	return out
}

// manaToPay is the CR 601.2f total-cost composition for pc: resolvedMana
// ({X} folded and flexible-pip announcement recorded), then cost increases
// and reductions, then the CR 903.8 commander tax. Delve credit is the
// caller's concern (targetAsk/payCast subtract pc.delve from Generic).
func (e *Engine) manaToPay(pc *pendingCast) Cost {
	m := pc.mods.apply(pc.resolvedMana())
	m.Generic += pc.taxGeneric
	return m
}

// manaToPayX is manaToPay with {X} folded to an explicit value.
func (e *Engine) manaToPayX(pc *pendingCast, x int32) Cost {
	m := pc.mods.apply(pc.resolvedManaX(x))
	m.Generic += pc.taxGeneric
	return m
}

// hasManaPayment reports whether a cost's mana component is non-empty, per CR
// 601.2g's "if the total cost includes a mana payment".
func (c Cost) hasManaPayment() bool {
	return c.Colored.Total() > 0 || c.Generic > 0
}

// dropAnnouncePrefix removes the first n announcement pips (in announcePip
// order: two-colour hybrids, then monocolour hybrids, then Phyrexian, then
// hybrid-Phyrexian) from the cost, leaving the rest as the cost's live
// choices. It is how announceCost keeps the pips a manaAsk has not yet
// settled, so a feasibility check never sees a pip the flow already decided.
func (c Cost) dropAnnouncePrefix(n int) Cost {
	drop := n
	if drop < len(c.Hybrid) {
		c.Hybrid = c.Hybrid[drop:]
		drop = 0
	} else {
		drop -= len(c.Hybrid)
		c.Hybrid = nil
	}
	if drop > 0 {
		if drop < len(c.Twobrid) {
			c.Twobrid = c.Twobrid[drop:]
			drop = 0
		} else {
			drop -= len(c.Twobrid)
			c.Twobrid = nil
		}
	}
	if drop > 0 {
		if drop < len(c.Phyrexian) {
			c.Phyrexian = c.Phyrexian[drop:]
			drop = 0
		} else {
			drop -= len(c.Phyrexian)
			c.Phyrexian = nil
		}
	}
	if drop > 0 {
		if drop < len(c.HybridPhyrexian) {
			c.HybridPhyrexian = c.HybridPhyrexian[drop:]
		} else {
			c.HybridPhyrexian = nil
		}
	}
	return c
}

// announceCost reconstructs the cost for checking feasibility of offering
// alternative alt at the payIdx-th announcement pip: X folded, the pips
// already committed (payColor/payLife/payGeneric, the aggregate of pips 0..
// payIdx-1) and the candidate alt baked into Colored/Generic/Life, the pips
// after payIdx kept live, and (for a spell) the Delve credit the payment
// subtracts taken off the generic. It is exactly the cost manaAsk's decision
// would commit to if it offered alt, with the still-unsettled pips free.
func (e *Engine) announceCost(pc *pendingCast, payIdx int, alt pipAlt, payColor state.Mana, payLife, payGeneric int32) Cost {
	c := pc.cost.WithX(pc.x)
	for i := range c.Colored {
		c.Colored[i] += payColor[i]
	}
	c.Generic = addClampedGeneric(c.Generic, int64(payGeneric))
	c.Life = addClampedGeneric(c.Life, int64(payLife))
	switch {
	case alt.color != 0:
		c.Colored[state.ManaIndex(alt.color)]++
	case alt.generic > 0:
		c.Generic = addClampedGeneric(c.Generic, int64(alt.generic))
	case alt.life > 0:
		c.Life = addClampedGeneric(c.Life, int64(alt.life))
	}
	if pc.ability < 0 && len(pc.delve) > 0 {
		// A Delve spell pays its generic from the graveyard (CR 702.65), so
		// the eligibility same as the offer/payment gates (castable, payCast)
		// subtract the credit before measuring the generic shortfall.
		if c.Generic > int32(len(pc.delve)) {
			c.Generic -= int32(len(pc.delve))
		} else {
			c.Generic = 0
		}
	}
	// Use the same total-cost composition that manaToPay charges after every
	// flexible pip is announced. A RaiseCost (or SetCost) can make a twobrid
	// generic face infeasible even if the base cost was payable; commander tax
	// is an additional cost applied after reductions.
	c = pc.mods.apply(c.dropAnnouncePrefix(payIdx + 1))
	c.Generic = addClampedGeneric(c.Generic, int64(pc.taxGeneric))
	return c
}

// announceFeasible reports whether offering alternative alt at the payIdx-th
// announcement pip still leaves the whole cost payable from pool/snow/life,
// given the pips already committed (payColor/payLife/payGeneric) and the
// payer's current resources. It is the CR 601.2b legality question: an
// announced payment is offered only if SOME legal assignment of the still-
// unsettled pips makes the cost payable, so a player is never offered a
// payment that can only strand the cast in an unpayable remainder (and an
// abort at targetAsk). It uses the same resolveMana the payment stage
// charges, so the offered set and the charged cost can never disagree.
func (e *Engine) announceFeasible(pc *pendingCast, payIdx int, alt pipAlt, payColor state.Mana, payLife, payGeneric int32, pool state.Mana, snow state.Mana, life int32) bool {
	c := e.announceCost(pc, payIdx, alt, payColor, payLife, payGeneric)
	return c.payable(pool, snow, life)
}

// manaAsk offers the player's payment choice for the next unsettled hybrid or
// Phyrexian pip of the cost (CR 601.2b), one decision per pip. Only payment
// alternatives that are legal right now -- a hybrid half with pool mana of
// that colour left, or a Phyrexian pip's colour or two life if the payer has
// both -- are offered, and each ONLY if some legal assignment of the still-
// unsettled pips makes the whole cost payable (announceFeasible), so a
// payment a player can actually complete is the only thing on the menu: a
// twobrid {2/W} generic face with not enough mana (or too little left for the
// pips after it) is not offered, because choosing it can only abandon the
// cast. The valid one comes first, so the deterministic bot fallback (index
// 0) always picks a legal payment and a no-answer host never wedges. The
// offer gate (castable) already proved at least one alternative is feasible,
// so the decision is never empty. It returns true once it has asked (and
// therefore suspended); payCast applies the accumulated payColor / payLife
// when every pip is settled.
func (e *Engine) manaAsk() bool {
	pc := e.cast
	if pc == nil || pc.payIdx >= pc.cost.annPipCount() {
		return false
	}
	alts := pc.cost.announcePip(pc.payIdx)
	// announceFeasible receives the full pool and life total because the
	// commitments already made (and this candidate face) are baked into the
	// reconstructed cost it evaluates. Do not pre-filter a colour face merely
	// because the current pool lacks that colour: a Color$ reduction can make
	// the announced face free (for example {W/U} under Color$ W).
	pool, snow := e.G.Players[pc.player].Pool, e.G.Players[pc.player].Snow
	fullLife := e.G.Players[pc.player].Life
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose how to pay a mana symbol of " + e.G.Obj(pc.card).Face().Name,
		Source: pc.card}
	addPip := func(alt pipAlt) {
		switch {
		case alt.color != 0:
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "pay_" + string(alt.color), Label: "Pay " + string(alt.color), Amount: 1})
		case alt.generic > 0:
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "pay_generic", Label: fmt.Sprintf("Pay %d generic", alt.generic), Amount: int(alt.generic)})
		case alt.life > 0:
			d.Options = append(d.Options, decision.Option{Index: len(d.Options),
				Kind: "pay_life", Label: "Pay 2 life", Amount: 2})
		}
	}
	// One option per DISTINCT colour alternative (A then B; a single-colour
	// Phyrexian pip carries the same colour twice, so the seen set keeps one
	// option for it), then a monocolour hybrid's generic face, then life. The
	// shared full-cost feasibility check is the only resource gate, so it also
	// covers reductions that make a face free.
	seen := map[byte]bool{}
	seenGeneric := false
	for _, alt := range alts {
		switch {
		case alt.color != 0:
			if seen[alt.color] {
				continue
			}
			seen[alt.color] = true
			if e.announceFeasible(pc, pc.payIdx, alt, pc.payColor, pc.payLife, pc.payGeneric, pool, snow, fullLife) {
				addPip(alt)
			}
		case alt.generic > 0:
			if seenGeneric {
				continue
			}
			seenGeneric = true
			if e.announceFeasible(pc, pc.payIdx, alt, pc.payColor, pc.payLife, pc.payGeneric, pool, snow, fullLife) {
				addPip(alt)
			}
		case alt.life > 0:
			if e.announceFeasible(pc, pc.payIdx, alt, pc.payColor, pc.payLife, pc.payGeneric, pool, snow, fullLife) {
				addPip(alt)
			}
		}
	}
	if len(d.Options) == 0 {
		// Defensive: the offer gate proved at least one pip alternative
		// completes the cost, so a feasible option is always present for a
		// gated cast; this arm only guards a logic bug. Rather than offer an
		// infeasible payment, offer the first alternative (index 0, the
		// deterministic best) so the decision is never empty -- the abort path
		// that alternatives-only filtering can otherwise leave unreachable is
		// never chosen by a gated cast that somehow reached an empty menu.
		addPip(alts[0])
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
	// CR 733.1: an aborted proposal must undo the as-enters choice. Capture
	// the object's choice fields as they were IMMEDIATELY BEFORE this answer
	// records one (the first answer of a multi-choice card sees the true
	// pre-proposal state), so abortCast can emit reverse Choose events
	// restoring them. Mark etbChosen once, so the capture is not overwritten
	// by a later answer and so a spell that never chose asks for no restore.
	if !pc.etbChosen {
		if o := e.G.Obj(pc.card); o != nil {
			pc.etbName, pc.etbType, pc.etbNumber = o.ChosenName, o.ChosenType, o.ChosenNumber
		}
		pc.etbChosen = true
	}
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
// stage asked it (every option in one decision shares a Kind). A mana-window
// decision (CR 601.2g) is the exception: it offers both "activate" and
// "done" options, so the CHOSEN option's kind, not the stage's, identifies
// the answer.
func (e *Engine) castAnswer(d *decision.Decision, chosen []decision.Option) {
	pc := e.cast
	if pc == nil || len(d.Options) == 0 {
		return
	}
	// The decision's CHOSEN option identifies the answer, never the first
	// offered option. Most chooseCast decisions are single-kind (an {X} value,
	// a Delve exile, a sacrifice) so Options[0].Kind would coincidentally be
	// right, but a hybrid/Phyrexian/twobrid pip decision offers MIXED kinds
	// (pay_W, pay_generic, pay_life) and the mana window offers activate/done,
	// so dispatching on Options[0].Kind would mis-route a non-first choice
	// (picking a twobrid generic face from a decision whose first option is
	// pay_W fell into the pay_W branch and minted a colourless pip).
	kind := d.Options[0].Kind
	if len(chosen) > 0 {
		kind = chosen[0].Kind
	}
	switch kind {
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
	case "discard":
		for _, o := range chosen {
			pc.discards = append(pc.discards, o.Obj)
		}
		pc.discardPart++
	case "pay_W", "pay_U", "pay_B", "pay_R", "pay_G", "pay_C":
		// A hybrid or Phyrexian pip paid with pool mana: record which colour.
		if len(chosen) > 0 {
			pc.payColor[state.ManaIndex(chosen[0].Kind[4])]++
		}
		pc.payIdx++
	case "pay_life":
		// A Phyrexian face (plain or hybrid) paid with two life.
		pc.payLife += 2
		pc.payIdx++
	case "pay_generic":
		// A monocolour hybrid pip paid with its generic face.
		if len(chosen) > 0 {
			pc.payGeneric += int32(chosen[0].Amount)
		}
		pc.payIdx++
	case "activate":
		// CR 601.2g: a source's mana abilities are distinct activations that
		// share its tap cost. activateMana resolves a singleton immediately or
		// asks the caster to choose one before re-entering this payment window.
		if len(chosen) > 0 {
			e.activateMana(pc.player, chosen[0].Obj, true)
		}
	case "done":
		// CR 601.2g: the player declines further mana abilities; pay the cost.
		pc.windowDone = true
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
	if pc == nil || pc.passedTarget {
		// passedTarget: the flow has already moved through the 601.2c target
		// choice into payCast (a spell or ability whose target was chosen, or
		// one with no target); a mana-window resume re-enters continueCast and
		// must not re-ask for a target already settled.
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
	sa = modalTargetSA(f, sa, o.ChosenModes)
	if sa == nil || sa.Params["ValidTgts"] == "" {
		return false
	}
	// A proposal whose resolved mana cost can no longer be paid, and with no
	// untapped mana-ability source the 601.2g window could activate to enable
	// it, can never complete. Reverse it (CR 733.1), which undoes the pushCast
	// stack move (E2: no progress was made, so hold this card's option out of
	// the window rather than re-offering the same unpayable cast). When such a
	// source exists, the window (asked later, in payCast) may still supply the
	// mana, so the proposal is not yet dead -- it proceeds to the target ask
	// and then the window. Resolved means X is fixed, the CR 601.2f modifiers
	// are applied and Delve credit is subtracted, all settled by the stages
	// above.
	mana := e.manaToPay(pc)
	if pc.ability < 0 {
		mana.Generic -= int32(len(pc.delve))
		if mana.Generic < 0 {
			mana.Generic = 0
		}
	}
	if !mana.payable(e.G.Players[pc.player].Pool, e.G.Players[pc.player].Snow, e.G.Players[pc.player].Life) &&
		!e.hasUntappedManaSource(pc.player) && !e.targetDependentCostMayPay(pc) {
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
	// A ValidTarget$ cost modifier can make this proposal offerable only for
	// particular targets. Once mana faces are announced, do not put a target
	// on the menu unless repricing that target can still complete the cast:
	// selecting an unaffordable target and then reversing the proposal is not
	// a legal CR 601.2c choice. An untapped mana source keeps a candidate on
	// the menu because the 601.2g window may make its final cost payable.
	//
	// For a multi-target declaration this is deliberately conservative: a
	// target that needs another selected target to satisfy ValidTarget$ is
	// withheld rather than exposing a selection subset that would abort. The
	// engine's decision type cannot express cross-option dependencies, and
	// withholding is safer than offering an illegal transaction.
	candidates = e.affordableTargetCandidates(pc, candidates)
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
		// Shared with stack.go's askTarget so a Face-less ability object (a
		// TargetType$ Activated/Triggered census) can never nil-deref here.
		label := e.targetOptionLabel(candidate)
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
	if pc.pushed {
		// The object is already on the stack (a mana-window resume re-enters
		// continueCast); do not push it a second time.
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
	pc.preAborts = e.castAborts
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

// recheckIllegal implements CR 601.2e: once every announcement choice (the
// {X} value) is made but before the cost is paid, the game rechecks that the
// proposed spell can legally be cast, considering the characteristics the
// choices changed -- most importantly the mana value with {X} counted at its
// chosen value (CR 202.3e). A CantBeCast restriction that the chosen {X} now
// makes applicable forbids the spell, so the proposal is reversed (CR 733.1)
// and nothing is paid. Only a spell is rechecked: an activated ability's
// legality was fully gated before it was offered, and 202.3e's X-count is a
// spell-mana-value rule. Returns true (and has reversed the proposal) when
// the spell has become illegal.
func (e *Engine) recheckIllegal(pc *pendingCast) bool {
	if pc.ability >= 0 {
		return false
	}
	o := e.G.Obj(pc.card)
	if o == nil || o.Face() == nil {
		return false
	}
	// CR 202.3e: the spell's mana value counts {X} at the chosen value, and
	// is a property of the card's printed mana cost -- never the alternative
	// cost (flashback) it may be paid with.
	printed := ParseCost(o.Face().ManaCost)
	mv := printed.CMC()
	if printed.X > 0 {
		mv = printed.WithX(pc.x).CMC()
	}
	for _, sv := range e.activeStatics("CantBeCast") {
		if !e.actorMatches(sv, "Caster", pc.player) {
			continue
		}
		sc := e.specCtx(sv.Source, sv.Controller)
		sc.HasManaValue = true
		sc.ManaValue = mv
		if effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], pc.card, sc) {
			e.abortCast(pc, "cast aborted: proposed spell is illegal (CR 601.2e)", false)
			return true
		}
	}
	return false
}

// manaWindowAsk implements CR 601.2g: if the total cost includes a mana
// payment, the player gets a chance to activate mana abilities before paying
// (601.2h). The engine poses a mid-cast KChoose window -- one "activate"
// option per untapped mana-ability source the player controls, then a "done"
// option -- only when the pool alone cannot pay the resolved total cost and
// at least one such source is untapped; a caster who already has the mana, or
// has no untapped source, has nothing a window could enable, so payCast
// proceeds straight to payment. Answering routes through castAnswer
// (chooseCast): "activate" taps the source and resolves its mana abilities
// (a tap consumes it, so it is not re-offered) and continueCast re-enters
// payCast to re-price the window; "done" sets windowDone so payCast pays.
func (e *Engine) manaWindowAsk() bool {
	pc := e.cast
	if pc == nil || pc.windowDone {
		return false
	}
	mana := e.manaToPay(pc)
	if pc.ability < 0 {
		mana.Generic -= int32(len(pc.delve))
		if mana.Generic < 0 {
			mana.Generic = 0
		}
	}
	if !mana.hasManaPayment() {
		return false
	}
	// A pool that already pays the total cost needs no window (nothing to
	// gain by activating more mana abilities here).
	if mana.payable(e.G.Players[pc.player].Pool, e.G.Players[pc.player].Snow, e.G.Players[pc.player].Life) {
		return false
	}
	var sources []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, pc.player) {
		if !e.untappedManaSource(pc.player, id) {
			continue
		}
		sources = append(sources, id)
	}
	if len(sources) == 0 {
		return false
	}
	name := e.G.Obj(pc.card).Face().Name
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Activate mana abilities to pay for " + name, Source: pc.card}
	for _, id := range sources {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "activate",
			Obj: id, Label: "Tap " + e.G.Obj(id).Face().Name + " for mana"})
	}
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "done", Label: "Done"})
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// untappedManaSource reports whether id is an untapped permanent under the
// player p's control with at least one unrestricted mana ability.
func (e *Engine) untappedManaSource(p state.PlayerID, id state.ObjID) bool {
	return len(e.availableManaAbilities(p, id)) > 0
}

// hasUntappedManaSource reports whether p controls ANY untapped permanent
// with a usable mana ability -- the condition under which the 601.2g window
// could supply the mana a pool alone cannot.
func (e *Engine) hasUntappedManaSource(p state.PlayerID) bool {
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if e.untappedManaSource(p, id) {
			return true
		}
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
	// The flow is now past the 601.2c target choice (either it was asked and
	// answered, or the SA has no target), so a mana-window resume through
	// continueCast must not re-ask for one.
	pc.passedTarget = true
	// CR 601.2e: the game checks that the proposed spell can legally be cast,
	// once every announcement choice (the {X} value) is known. An illegal
	// proposal is reversed (CR 733.1) -- see recheckIllegal.
	if e.recheckIllegal(pc) {
		return
	}
	// CR 601.2g: if the total cost includes a mana payment, the player gets a
	// chance to activate mana abilities before paying. manaWindowAsk poses
	// that window (returning true to suspend) only when the pool alone cannot
	// pay and an untapped mana source exists.
	if e.manaWindowAsk() {
		return
	}
	if pc.ability >= 0 {
		// CR 608.2h: snapshot the source's derived lifelink before any cost can
		// remove it from the battlefield. AbilityPush is deliberately emitted
		// only after costs settle, so emit's generic departure capture cannot
		// see a self-sacrificing ability on the stack yet. Resolution consults
		// this only if the source is gone; a source that remains in play uses
		// its live derived state instead.
		sourceLifelinkLKI := e.HasKeyword(pc.card, "Lifelink")
		// Task 10: an activated ability. The shared stages above (X, Delve --
		// never present on an ability --, Sac) have already run and been
		// recorded; what differs from a spell here is the cost's remaining
		// non-mana parts. Pay mana, then each Tap (a Tap event), each
		// SubCounter part (a CounterChange of -N), and every chosen sacrifice.
		// The ability object was already minted by pushCast; targets are
		// recorded onto it by handleTarget.
		mana := e.manaToPay(pc)
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
		for _, id := range pc.discards {
			e.emit(events.DiscardCost(id))
		}
		if pc.cost.Tap {
			// The {T} cost's payer taps the permanent (Forge CostTap).
			e.emitTap(pc.card, pc.player, false)
		}
		for _, part := range pc.cost.SubCounter {
			e.emit(events.Event{Kind: events.CounterChange, Obj: pc.card, Counter: part.Spec, Amount: -part.N})
		}
		// CR 606.3: a [+N] loyalty cost adds N loyalty counters to the walker
		// as part of the activation's payment, settled beside the SubCounter
		// removals and before the AbilityPush (the ability object the effect
		// resolves through). AddCounter is a free cost component -- no mana,
		// no gate -- so this is the only thing the activation does with it.
		// A 0-count part (the [0] abilities) emits nothing: a CounterChange of
		// 0 would be a no-op folded into state but a spurious log entry.
		for _, part := range pc.cost.AddCounter {
			if part.N != 0 {
				e.emit(events.Event{Kind: events.CounterChange, Obj: pc.card, Counter: part.Spec, Amount: part.N})
			}
		}
		// Capture the sacrifice LKI (Task sac1) BEFORE the MoveZone events
		// drain the permanents: each chosen object is still on the battlefield
		// here, so SacrificedInfoOf reads its live face and +1/+1 counters (the
		// layer-7d portion of its P/T, which Move will reset). The resulting
		// stack object carries these to resolution, where the ability's
		// Sacrificed$<Property> SVar heads answer "the sacrificed creature's
		// power/toughness/mana value" (CR 608.2g) against them.
		var sacrificedLKI []state.SacrificedInfo
		for _, id := range pc.sacs {
			sacrificedLKI = append(sacrificedLKI, state.SacrificedInfoOf(e.G, id))
		}
		for _, id := range pc.sacs {
			e.emit(events.Sacrifice(id))
		}
		// AbilityPush mints the ability object onto the stack AFTER the cost
		// settles, so an aborted activation leaves no stack object behind
		// (CR 733.1). handleTarget records the chosen targets onto it.
		e.emit(events.Event{Kind: events.AbilityPush, Obj: pc.card, Player: pc.player, Amount: int32(pc.ability)})
		if len(e.G.Stack) > 0 {
			pc.stackObj = e.G.Stack[len(e.G.Stack)-1]
		}
		// CR 107.3i: record the chosen {X} on the ability stack object, the
		// same way the spell arm records it on the spell below. The shared
		// xAsk stage asked and paid it (pc.cost.WithX(pc.x)), but AbilityPush's
		// Amount is the ability index, not the X value, so without this the
		// paid X never reaches resolution and every Cost$-X parameter
		// (CounterNum$ X, NumDmg$ X, NumCards$ X, SVar:X:Count$xPaid) reads 0.
		// Obj is pc.stackObj (the minted ability object), never pc.card (the
		// source permanent), so the permanent's own X (e.g. a Walking
		// Ballista's ETB value) is not clobbered. Emitted AFTER the push so
		// the object exists for events.Apply to write it on. Zero means no
		// X was paid: no event, matching the spell arm's guard.
		if pc.x != 0 {
			e.emit(events.Event{Kind: events.CastInfo, Obj: pc.stackObj, Amount: pc.x})
		}
		if e.sacrificedLKI == nil {
			e.sacrificedLKI = make(map[state.ObjID][]state.SacrificedInfo)
		}
		e.sacrificedLKI[pc.stackObj] = sacrificedLKI
		for _, id := range pc.sacs {
			if id != pc.card {
				continue
			}
			if e.sourceLifelinkLKI == nil {
				e.sourceLifelinkLKI = make(map[state.ObjID]bool)
			}
			e.sourceLifelinkLKI[pc.stackObj] = sourceLifelinkLKI
			break
		}
		e.cast, e.choosing = nil, chooseNone
		return
	}
	mana := e.manaToPay(pc)
	mana.Generic -= int32(len(pc.delve))
	if mana.Generic < 0 {
		mana.Generic = 0
	}
	if !e.payMana(pc.player, mana) {
		// E2 (round 2) / F05-2. This is the reachable no-progress arm: a Delve
		// exile ask (Min:0, Max the shortfall) was answered with fewer cards
		// than the shortfall needs, so the cast aborts with no state change
		// and priority re-offers it. Declining is a legal, conforming answer --
		// CR 601.2h rewinds the cast -- but the engine must not re-offer the
		// SAME unpayable cast forever. CR 733.2 lets a reversed illegal action
		// be redone legally, so the FIRST no-progress abort leaves THIS card's
		// option offered; only the SECOND identical abort in the same window
		// holds it out of the remaining priority window (the suppression clears
		// on the first state-changing event, so the option returns as soon as
		// the window ends or the mana/board changes).
		e.abortCast(pc, "cast aborted: cost no longer payable", true)
		return
	}
	if pc.payLife != 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: pc.player, Amount: -pc.payLife})
	}
	for _, id := range pc.delve {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile, Text: "delved"})
	}
	for _, id := range pc.discards {
		e.emit(events.DiscardCost(id))
	}
	// Capture the sacrifice LKI before the MoveZones (see the ability branch's
	// comment): the sacrificed permanents are still on the battlefield here.
	var sacrificedLKI []state.SacrificedInfo
	for _, id := range pc.sacs {
		sacrificedLKI = append(sacrificedLKI, state.SacrificedInfoOf(e.G, id))
	}
	for _, id := range pc.sacs {
		e.emit(events.Sacrifice(id))
	}
	if e.sacrificedLKI == nil {
		e.sacrificedLKI = make(map[state.ObjID][]state.SacrificedInfo)
	}
	e.sacrificedLKI[pc.stackObj] = sacrificedLKI
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
	// held-out no-progress state must survive. For a pushed spell that state
	// is pc.preSuppress / pc.preAborts (captured before the push, which
	// cleared it); for an ability (never pushed) it is the current maps. Both
	// the held-out set and the per-card no-progress count are restored, so a
	// no-progress decline of THIS object can be counted again across the
	// push (F05-2).
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
	var savedAborts map[state.ObjID]int32
	if pc.pushed && pc.preAborts != nil {
		savedAborts = make(map[state.ObjID]int32, len(pc.preAborts))
		for id, n := range pc.preAborts {
			savedAborts[id] = n
		}
	} else if e.castAborts != nil {
		savedAborts = make(map[state.ObjID]int32, len(e.castAborts))
		for id, n := range e.castAborts {
			savedAborts[id] = n
		}
	}
	if suppress {
		if savedAborts == nil {
			savedAborts = map[state.ObjID]int32{}
		}
		savedAborts[pc.card]++
		// F05-2 (CR 733.2): the FIRST no-progress abort of a card leaves its
		// option offered, so a merely-reversed illegal action may be redone
		// legally; the SECOND identical abort holds the option out. Undo the
		// restore-only path for a count below two by not adding to `saved`.
		if savedAborts[pc.card] >= 2 {
			if saved == nil {
				saved = map[state.ObjID]bool{}
			}
			saved[pc.card] = true
		}
	}
	if pc.pushed && pc.stackObj != 0 {
		if pc.ability >= 0 {
			e.emit(events.Event{Kind: events.MoveZone, Obj: pc.stackObj, From: state.ZStack, To: state.ZExile, Text: "reversed"})
		} else {
			e.emit(events.Event{Kind: events.MoveZone, Obj: pc.stackObj, From: state.ZStack, To: pc.from, Text: "reversed"})
		}
	}
	// CR 733.1: the game returns to the moment before the spell or ability was
	// proposed, so an as-enters choice recorded during the proposal must be
	// undone too (Sanctum Prelate's ChosenNumber otherwise survives a mana
	// abort while every other resource is restored). Emit reverse Choose
	// events restoring the captured pre-proposal values, but ONLY for the
	// fields a choice actually changed and ONLY when a choice was recorded
	// (etbChosen): a spell that never chose emits nothing here, so no chain
	// head moves for the choiceless abort sites.
	if pc.etbChosen {
		if o := e.G.Obj(pc.card); o != nil {
			if o.ChosenName != pc.etbName {
				e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "name", Text: pc.etbName})
			}
			if o.ChosenType != pc.etbType {
				e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "type", Text: pc.etbType})
			}
			if o.ChosenNumber != pc.etbNumber {
				e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "number", Amount: pc.etbNumber})
			}
		}
	}
	// CR 733.1 applies identically to a mode announced during the proposal.
	// ModeChosen is a marker event, so the cache is restored beside the reverse
	// marker just as handleModes maintains it beside the forward marker.
	if pc.modeChosen {
		if o := e.G.Obj(pc.card); o != nil {
			if sa := o.Face().SpellAbility(); sa != nil {
				e.emit(events.Event{Kind: events.ModeChosen, Obj: pc.card, Player: pc.player,
					Text: strings.Join(modeLabels(sa, o.Face().SVars, pc.preModes), ",")})
			}
			o.ChosenModes = append([]string(nil), pc.preModes...)
		}
	}
	e.deferredPush = nil
	e.deferredPushLKI = nil
	e.cast, e.choosing = nil, chooseNone
	e.emit(events.Event{Kind: events.Note, Player: pc.player, Text: text})
	e.suppressedCast = saved
	e.castAborts = savedAborts
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

// castSuppressed reports whether id's cast option is currently held out of
// p's priority offers (see suppressedCast, engine.go). The id names the one
// seat holding it, so p is not consulted beyond matching that id's zone in
// the walk that called it.
func (e *Engine) castSuppressed(p state.PlayerID, id state.ObjID) bool {
	return e.suppressedCast != nil && e.suppressedCast[id]
}

func init() {
	effects.RegisterNonAPI("kw:Kicker", "kw:Surge", "kw:Flashback", "kw:Delve")
}
