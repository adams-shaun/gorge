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
	"regexp"
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
	// chooseRiot is deliberately outside the independently extended
	// chooseCleanup/chooseMana ranges in combat.go and mana_activation.go.
	// It is 20 because the merged package occupies 1 through 18 (cast/etb/
	// miracle 1-3, cleanup 4, damageDivision 5, mana 6-9, opening 10,
	// suspendCast 12, station 13, unlock 14, cumulative 15, triggeredCost
	// 16, manaUnless/unlessCost 17-18): all pairwise-distinct consts in one
	// switch table, so the exact numbers do matter inside the package.
	chooseRiot chooseFor = 20
)

// pendingCast is the cast flow's own state, live only between beginCast and
// commitCast (or an abort). ability is -1 for a spell; Task 10 (activated
// abilities) sets it to a real Face().Abilities index and reuses this same
// flow for a cost with X/Sac/Delve of its own.
type pendingCast struct {
	player  state.PlayerID
	card    state.ObjID
	from    state.Zone
	mode    string // "", "kicked", "surged", "flashback", "miracle", and the alternative-cost modes this file offers
	ability int    // -1 for a spell (Task 10 uses >= 0)

	cost Cost

	// mayPlayIgnore is the may-play grant's MayPlayIgnoreColor$ rider,
	// recorded at beginCast from the offer gate that proved it (the card was
	// still in the granted zone); the mana window and the payment keep the
	// grant through it, since after the push (CR 601.2a) the card is on the
	// stack and a zone re-derivation would wrongly drop the grant.
	mayPlayIgnore bool
	// mayPlayIgnoreType is the grant's MayPlayIgnoreType$ rider (Rakdos, the
	// Muscle): the same recorded-at-beginCast discipline as mayPlayIgnore,
	// threading "mana of any type" through the same window and payment.
	mayPlayIgnoreType bool

	x     int32
	xDone bool
	// announceX is the alternative cost's Announce$ variable (the Shoal
	// cycle's "X"): the X this cast announces is NOT a mana X — it is bound
	// by the exile settlement's cmcEQX filter (xAsk's announce arm offers
	// exactly the mana values some exilable card matches at; exAsk binds the
	// announced value into that filter). Empty on every ordinary cast.
	announceX string

	// sameCtrlTargets is the TargetsWithSameController$ True rider (Lodestone
	// Bauble): every target this cast's announcement chooses must share one
	// controller — in a graveyard, its owner. The offered option list spans
	// every player's graveyard, so the pairwise constraint is enforced at
	// Submit (validateCastContributions' preserve-and-reject shape), not by
	// an option-list shape the wire cannot express.
	sameCtrlTargets bool
	// suspendTimeX makes the chosen cast X also set the number of TIME
	// counters; suspendMinX is Forge's XMin<N> lower bound.
	suspendTimeX bool
	suspendMinX  int32

	delve     []state.ObjID
	delveDone bool

	// replicateParam is the raw Replicate keyword parameter (CR 702.55a) a
	// "replicated" cast re-parses at ask and answer time; replicateTimes is
	// the answered payment count (0 = declined: no flag, a plain cast) and
	// replicateDone marks the one ask already posed. Plain data, so Clone
	// copies it like x/delve/sacs.
	replicateParam string
	replicateSet   bool
	replicateTimes int32
	replicateDone  bool

	// multikickParam is the raw Multikicker keyword parameter (CR 702.43) a
	// "multikicked" cast re-parses at ask and answer time; multikickTimes is
	// the answered payment count (0 = declined: no flag, a plain cast) and
	// multikickDone marks the one ask already posed. Same shape as the
	// replicate fields above; plain data, so Clone copies it.
	multikickParam string
	multikickSet   bool
	multikickTimes int32
	multikickDone  bool

	// converge (task converge1) is CR 107.4f-family's count of distinct
	// colours (WUBRG) of mana actually spent to cast this spell, captured at
	// payment from the full spent delta payManaCastSpent returns. convergeOn
	// is the heads-safety two-arm gate (faceWantsConverge OR a battlefield
	// reader naming TriggeredCard$Converge): the pay-time CastInfo is emitted
	// ONLY for a face carrying a Count$Converge SVar, or when some alive
	// player's battlefield permanent's trigger reads another spell's cast
	// colours, so no game that casts neither changes an event. Plain data, so
	// Clone copies it like replicateTimes.
	convergeOn bool
	converge   int32

	// manaSpentOn/manaSpent (task castprov1) capture the TOTAL mana the
	// cast's payment actually spent, from the same full spent delta
	// payManaCastSpent returns (the pips summed over every slot).
	// manaSpentOn is the heads-safety gate (faceWantsCastSpend): the pay-time
	// CastInfo is emitted ONLY for a face whose SVar table reads the
	// Count$CastTotalManaSpent head, so no game that casts no such card
	// changes an event. Plain data, so Clone copies it like converge.
	manaSpentOn bool
	manaSpent   int32

	sacs    []state.ObjID
	sacPart int

	discards    []state.ObjID
	discardPart int

	// convoke is the announced set of creatures paying Convoke or Harmonize.
	// It is chosen after the complete mana cost exists and before the mana
	// ability window; a committed creature is therefore unavailable to make
	// mana as well as being tapped when payment is settled.
	convoke          []convokePayment
	convokeDone      bool
	suspendCastClear bool

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

	// ownReduce is the amount the ability's own ReduceCost$ parameter folded
	// into pc.cost at beginActivation (nil targets there: CR 601.2c has not
	// run). repriceForTargets recomputes it target-aware and net-adjusts
	// cost.Generic by the delta, so the folded amount is never applied twice
	// and the net form is idempotent across a mana-window resume.
	ownReduce int32

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

	// provenanceRepriced is true once the post-push provenance re-price has
	// run for this proposal (castprov3: a provenance-keyed cost static is
	// unresolvable pre-push, so continueCast re-prices pc.mods right after
	// the push; the flag keeps the re-entries — a mana-window resume re-enters
	// continueCast with pushed already true — from gathering the statics
	// again). Plain data, so Clone copies it.
	provenanceRepriced bool

	// preSuppress is the suppressedCast set as it was just before pushCast's
	// PutOnStack, captured so an aborted (reversed) cast can restore it:
	// the push is a state-changing event that emit treats as progress and so
	// clears the held-out no-progress set, but an aborted cast is net no
	// progress, so that set must come back. Nil when no cast push is in
	// flight (an ability, or a spell aborted before the push).
	preSuppress map[state.ObjID]bool

	// faceBefore is non-nil only for a CR 309.4b alternate Room cast or a CR
	// 714 Adventure-face cast (adventure_alt / adventure_recast). The
	// proposal begins with an event-sourced FlipFace so all ordinary cast
	// stages read the chosen door; an aborted proposal flips it back.
	faceBefore *uint8

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
	etbColor  string

	// altAddParts are the alternative parts of the card's
	// AlternateAdditionalCost keyword ("As an additional cost to cast this
	// spell, sacrifice a creature or pay {3}{B}"): one KChoose over them at
	// cast-announcement time (altAddAsk), and the chosen part's cost folded
	// into cost for the ordinary cost stages to settle. Empty for a card
	// without the keyword; altAddDone marks the one ask already posed.
	altAddParts []string
	altAddDone  bool

	// exiles / exilePart carry the Exile cost parts (ExileFromHand /
	// ExileFromGrave tokens: the evoke alternative cast's Fury/Grief shape,
	// encore's "exile this card from your graveyard") through the same ask
	// stage / commit shape sacAsk and sacs use. Nothing moves until payCast,
	// so an abort cannot leave a partially paid exile on the board.
	exiles    []state.ObjID
	exilePart int

	// returns / returnPart carry the Return cost parts (Return<N/Spec>
	// tokens: a permanent matching Spec returned to its OWNER's hand) through
	// the same ask stage / commit shape the exile parts use.
	returns    []state.ObjID
	returnPart int

	// putToLibs / putToLibPart carry the PutToLib cost parts
	// (PutCardToLibFrom<Zone><N/Pos/Spec> tokens: cards matching Spec moved
	// from the payer's Hand/Graveyard/Battlefield to the top or bottom of
	// their owner's library) through the same ask stage / commit shape the
	// Return parts use. Nothing moves until payCast, so an abort cannot leave
	// a partially paid library placement behind.
	putToLibs    []state.ObjID
	putToLibPart int

	reveals, beholds, taps, blights             []state.ObjID
	revealPart, beholdPart, tapPart, blightPart int
	forageDone                                  bool
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
	if _, _, two := twoPartKickerCosts(f); two {
		// The and/or two-part Kicker is its own option family (legal.go's
		// kicked1/kicked2/kickedboth offers, one per independently payable
		// part): the single "kicked" option must not also exist for such a
		// face -- its whole-string parse would degrade the colon separator
		// into a generic pip and charge the both-parts price for a
		// single-part choice.
		return Cost{}, false
	}
	s, ok := f.KeywordParam("Kicker")
	if !ok {
		return Cost{}, false
	}
	return ParseCost(s), true
}

// twoPartKickerCosts resolves the and/or Kicker ("Kicker {G} and/or {1}{U}",
// Forge's colon-separated two-part Kicker:<a>:<b> keyword line -- 18 corpus
// files at the pin, the Volver/Battlemage/Involver cycle family, Wastescape
// Battlemage the repo-deck carrier) to its two independently payable parts:
// each may be paid alone or both together (CR 601.2b's optional additional
// costs, each declared separately). ok=false for the single-cost Kicker
// (every colon-free param) and for a colon form whose parts do not parse
// clean -- a part with an unmodelled token stays a labelled census gap
// rather than being silently charged.
func twoPartKickerCosts(f *cards.Face) (Cost, Cost, bool) {
	s, ok := f.KeywordParam("Kicker")
	if !ok {
		return Cost{}, Cost{}, false
	}
	a, b, is := strings.Cut(s, ":")
	if !is || strings.TrimSpace(a) == "" || strings.TrimSpace(b) == "" {
		return Cost{}, Cost{}, false
	}
	ca, cb := ParseCost(a), ParseCost(b)
	if len(ca.Unknown) > 0 || len(cb.Unknown) > 0 {
		return Cost{}, Cost{}, false
	}
	return ca, cb, true
}

func surgeCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Surge")
	if !ok {
		return Cost{}, false
	}
	return ParseCost(s), true
}

// replicateCost resolves the Replicate keyword's payment cost (CR 702.55a,
// Forge's K:Replicate:<cost>), the surgeCost shape. A cost carrying a token
// ParseCost cannot model is withheld (the fail-closed direction
// twoPartKickerCosts takes) rather than charged as degraded generic mana.
func replicateCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Replicate")
	if !ok {
		return Cost{}, false
	}
	c := ParseCost(s)
	if len(c.Unknown) > 0 {
		return Cost{}, false
	}
	return c, true
}

// multikickerCost resolves the Multikicker keyword's PER-PAYMENT cost
// (CR 702.43, Forge's K:Multikicker:<cost>), the replicateCost shape: the
// same payment may be made any number of times as the spell is cast, so the
// offer gates on ONE payment being payable and the count ask
// (multikickAsk) settles how many. A cost carrying a token ParseCost cannot
// model is withheld (the replicateCost fail-closed direction).
func multikickerCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Multikicker")
	if !ok {
		return Cost{}, false
	}
	c := ParseCost(s)
	if len(c.Unknown) > 0 {
		return Cost{}, false
	}
	return c, true
}

// keywordAltCost resolves any of the alternative-cast keyword family
// (Evoke, Dash, Overload, Warp, Madness) to a parsed Cost, reporting whether
// the keyword is printed at all. All five are "you may cast this for [cost]
// instead of its mana cost" shapes whose post-cast behaviour lives elsewhere
// (the ETB machinery for evoke, modeFlags for the rest), so they share one
// parameter read.
func keywordAltCost(f *cards.Face, head string) (Cost, bool) {
	s, ok := f.KeywordParam(head)
	if !ok {
		return Cost{}, false
	}
	return ParseCost(s), true
}

func buybackCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Buyback")
	if !ok {
		return Cost{}, false
	}
	return ParseCost(s), true
}

// altAddCostParts splits a face's AlternateAdditionalCost keyword into its
// alternative parts: the parameter is the parts joined by ":" (e.g. Bone
// Shards' "Sac<1/Creature>:Discard<1/Card>", Redirect Lightning's
// "PayLife<5>:2"). "As an additional cost to cast this spell, [A] or [B]"
// is a MANDATORY either-or (CR 601.2h), so the cast flow asks which one and
// folds the chosen part into the total cost. An empty or missing keyword
// yields nil; a parameter with no ":" yields nil (a single-part form would
// be an ordinary additional cost, which no corpus line uses -- the keyword's
// whole point is the either-or).
func altAddCostParts(f *cards.Face) []string {
	param, ok := f.KeywordParam("AlternateAdditionalCost")
	if !ok {
		return nil
	}
	parts := strings.Split(param, ":")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func harmonizeCost(f *cards.Face) (Cost, bool) {
	s, ok := f.KeywordParam("Harmonize")
	if !ok {
		return Cost{}, false
	}
	return ParseCost(s), true
}

// harmonizePayment applies the keyword's creature-power reduction in stable
// battlefield order and returns the creatures that pay it by becoming tapped.
func (e *Engine) harmonizePayment(p state.PlayerID, id state.ObjID, c Cost) (Cost, []state.ObjID) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || !o.Face().HasKeyword("Harmonize") {
		return c, nil
	}
	var tapped []state.ObjID
	for _, cid := range e.G.Zone(state.ZBattlefield, p) {
		if c.Generic == 0 {
			break
		}
		co := e.G.Obj(cid)
		if co == nil || co.Tapped || co.Face() == nil || !co.EffectiveIsCreature() || co.BestowedAttached() {
			continue
		}
		// The reduction is the creature's ACTUAL power (CR 702.46a: "reduce
		// that spell's generic cost by its power"): layer-7 effects and
		// P1P1/M1M1 counters apply, not the printed face. harmonizePayment
		// and convokeAsk's offer must read the same number or the offer
		// gate and the payment disagree on what the creature funds.
		reduce := e.Derived(cid).Power
		if reduce <= 0 {
			continue
		}
		if reduce > c.Generic {
			reduce = c.Generic
		}
		c.Generic -= reduce
		tapped = append(tapped, cid)
	}
	return c, tapped
}

type suspendInfo struct {
	time    int32
	timeX   bool
	minTime int32
	cost    Cost
}

// convokePayment is one announced non-mana payment. color is zero for a
// generic Convoke contribution or Harmonize; power is zero for Convoke and
// is the amount a Harmonize creature reduces the generic total by.
type convokePayment struct {
	id    state.ObjID
	color byte
	power int32
}

// hasCastConvoke reports whether the spell being cast carries Convoke once
// it is on the stack: the printed keyword, or a layer-6 grant (Chief
// Engineer's "Artifact spells you cast have convoke") whose AffectedZone$
// scope reaches the cast spell. The announcement (CR 601.2b) runs while the
// announced spell is still in hand, so the evaluation pretends the zone is
// the stack (derivedWith's override); a wasCast Affected$ predicate already
// matches because it keys on the object being a cast spell, which it is.
func (e *Engine) hasCastConvoke(id state.ObjID) bool {
	for _, k := range e.derivedWith(id, state.ZStack).Keywords {
		if strings.EqualFold(cardsKeywordHead(k), "Convoke") {
			return true
		}
	}
	return false
}

// suspendCost parses Forge's Suspend:<time>:<cost> keyword form. X-time
// scripts put their lower bound in the leading XMin<N> cost token; the same
// announced X pays the cost and becomes the number of TIME counters.
// convokeCost commits untapped creatures in battlefield order, consuming a
// needed colour when that creature has one and otherwise one generic mana.
// It returns the reduced cost and exactly the creatures that must be tapped.
func (e *Engine) convokeCost(p state.PlayerID, id state.ObjID, c Cost) (Cost, []state.ObjID) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil || !e.hasCastConvoke(id) {
		return c, nil
	}
	var tapped []state.ObjID
	for _, cid := range e.G.Zone(state.ZBattlefield, p) {
		co := e.G.Obj(cid)
		if co == nil || co.Tapped || co.Face() == nil || !co.EffectiveIsCreature() || co.BestowedAttached() {
			continue
		}
		used := false
		for _, col := range []byte{'W', 'U', 'B', 'R', 'G'} {
			i := state.ManaIndex(col)
			if c.Colored[i] > 0 && strings.Contains(e.objColors(co), string(col)) {
				c.Colored[i]--
				used = true
				break
			}
		}
		if !used && c.Generic > 0 {
			c.Generic--
			used = true
		}
		if used {
			tapped = append(tapped, cid)
		}
	}
	return c, tapped
}

func suspendCost(f *cards.Face) (suspendInfo, bool) {
	raw, ok := f.KeywordParam("Suspend")
	if !ok {
		return suspendInfo{}, false
	}
	n, rest, ok := strings.Cut(raw, ":")
	if !ok {
		return suspendInfo{}, false
	}
	if strings.TrimSpace(n) == "X" {
		fields := strings.Fields(rest)
		info := suspendInfo{timeX: true}
		if len(fields) > 0 && strings.HasPrefix(fields[0], "XMin") {
			min, err := strconv.ParseInt(strings.TrimPrefix(fields[0], "XMin"), 10, 32)
			if err != nil || min < 0 {
				return suspendInfo{}, false
			}
			info.minTime = int32(min)
			fields = fields[1:]
		}
		info.cost = ParseCost(strings.Join(fields, " "))
		// The corpus's X-time Suspend form charges that same X. Without a
		// cost X there is no finite legal option range to announce, so do not
		// offer a made-up bound.
		if info.cost.X == 0 {
			return suspendInfo{}, false
		}
		return info, true
	}
	time, err := strconv.ParseInt(strings.TrimSpace(n), 10, 32)
	if err != nil || time < 0 {
		return suspendInfo{}, false
	}
	return suspendInfo{time: int32(time), cost: ParseCost(rest)}, true
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
// castable is the ordinary offer gate's price check. The mana half resolves
// through costPayable -- the seat's RESTRICTION-ADJUSTED floating pool
// (manaAvailableFor), exactly the pool payManaFor will charge -- because the
// engine must never offer a cast whose payment would later fail: restricted
// mana (a RestrictValid$ batch, e.g. Eldrazi Temple's "colorless mana that
// can be used only to pay Eldrazi costs") that does not match this payment
// is invisible here, the same way it is invisible to the payment. The
// potential-action walk does NOT go through castable: it prices against an
// explicitly hypothetical pool (castablePriced below), where the over-bound
// direction is deliberate. Every non-mana part -- Sac candidates, Discard
// candidates, SubCounter counts, Tap untappedness -- is checked against the
// REAL state: floating mana never satisfies a sacrifice.
func (e *Engine) castable(p state.PlayerID, id state.ObjID, cost Cost, ability bool) bool {
	mana := cost
	mana.Generic -= e.delveCredit(p, id, mana.Generic)
	if !e.costPayable(p, id, ability, mana) {
		return false
	}
	return e.nonManaCastable(p, id, cost, ability)
}

// castablePriced is castable priced against an EXPLICIT pool instead of the
// seat's restriction-adjusted floating one. Its only caller is the
// potential-action walk (rules/legal.go legalActionsPriced's affordable, and
// its own hyp==nil arm routes back to castable): pool is the hypothetical
// bound the seat would hold after floating every untapped source. The pool is
// a pure mana bound -- the walk may price against raw units a RestrictValid$
// provenance would refuse at payment time, because a wrongly WITHHELD pass
// costs one idle stop while a wrongly eaten window loses the player's action
// -- but every non-mana part (Sac candidates, Discard candidates, SubCounter
// counts, Tap untappedness) is still checked against the REAL state: floating
// or hypothetical mana never satisfies a sacrifice.
func (e *Engine) castablePriced(p state.PlayerID, id state.ObjID, cost Cost, ability bool, pool state.Mana) bool {
	mana := cost
	mana.Generic -= e.delveCredit(p, id, mana.Generic)
	if !e.costPayablePool(p, id, ability, mana, pool) {
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
			if reserved[oid] || e.SacrificeBlocked(oid) { // an earlier Sac part already claimed this one; a CantSacrifice-blocked one can never pay
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
	// Exile cost parts (ExileFromHand/ExileFromGrave): each needs N matching
	// cards still available in the part's zone, reserved against the earlier
	// parts the same way the Sac parts above reserve against each other. For a
	// CAST (ability == false) the card being cast can never pay its own exile
	// cost: at offer time it still sits in the part's zone, so without the
	// self-skip below a Kotis with exactly Kotis+2 other cards would be
	// offered and then abort at payment (only 2 "other" cards remain once the
	// card is on the stack). An ability activation (ability == true) is
	// untouched -- encore's Cost$ ExileFromGrave<1/CARDNAME> really does exile
	// its own source. This also closes the same latent over-offer for an
	// escape cast from the graveyard.
	castObj := e.G.Obj(id)
	for _, part := range cost.Exile {
		zone := part.Zone
		if zone == 0 {
			zone = state.ZHand
		}
		selfInZone := !ability && castObj != nil && castObj.Zone == zone
		var avail []state.ObjID
		for _, oid := range e.G.Zone(zone, p) {
			if reserved[oid] || (selfInZone && oid == id) {
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
	for _, part := range cost.Reveal {
		if len(e.costCandidates(p, id, state.ZHand, part.Spec, true, false)) < int(part.N) {
			return false
		}
	}
	for _, part := range cost.Behold {
		n := len(e.costCandidates(p, id, state.ZHand, part.Spec, true, false)) +
			len(e.costCandidates(p, id, state.ZBattlefield, part.Spec, false, false))
		if n < int(part.N) {
			return false
		}
	}
	// Tap cost parts reserve their candidates against the Sac/Exile/Return
	// reservations above (one permanent cannot pay both) AND against each
	// other: a composed cost carrying the same tap part several times -- a
	// replicated cast re-pays its tapXType cost once per payment -- must not
	// count one untapped permanent for every part. Without the reservation
	// an affordability walk over the composed cost offered a bound far above
	// what the board could actually pay, and answering it aborted the cast
	// at the payment stage (CR 733's clean reversal, but a needless one).
	for _, part := range cost.TapPermanent {
		if part.Dyn != "" {
			// The dynamic tapXType heads (tapXType<X/Spec>, tapXType<Any/Spec>):
			// the tap election resolves the count at payment. An X-form part is
			// payable with zero candidates (X = 0 is a legal announcement); an
			// Any-form part must tap at least one matching permanent the earlier
			// parts have not already claimed, so a spec no unreserved candidate
			// satisfies (Mossbridge Troll's withTotalPowerGE10 group predicate
			// fails closed) leaves the cost unpayable rather than offering a
			// zero-tap payment of an effect that does not scale with the taps.
			if part.Dyn == "Any" {
				avail := 0
				for _, oid := range e.costCandidates(p, id, state.ZBattlefield, part.Spec, false, true) {
					if reserved[oid] || (cost.Tap && oid == id) {
						continue
					}
					avail++
				}
				if avail == 0 {
					return false
				}
			}
			continue
		}
		var avail []state.ObjID
		for _, oid := range e.costCandidates(p, id, state.ZBattlefield, part.Spec, false, true) {
			if reserved[oid] || (cost.Tap && oid == id) {
				continue
			}
			avail = append(avail, oid)
		}
		if len(avail) < int(part.N) {
			return false
		}
		for i := 0; i < int(part.N); i++ {
			reserved[avail[i]] = true
		}
	}
	for range cost.Blight {
		if len(e.costCandidates(p, id, state.ZBattlefield, "Creature.YouCtrl", false, false)) == 0 {
			return false
		}
	}
	if cost.Forage && len(e.G.Zone(state.ZGraveyard, p)) < 3 &&
		len(e.costCandidates(p, id, state.ZBattlefield, "Food.YouCtrl", false, false)) == 0 {
		return false
	}
	// Energy cost parts (PayEnergy<N>): the payer's energy counter total
	// covers the SUM of the fixed parts -- Forge CostPayEnergy.canPay reads
	// the same total, and a composed cost carrying the part several times (a
	// replicated cast re-pays its PayEnergy cost once per payment) draws the
	// pool down once per part, so the parts cannot each spend the whole
	// counter total independently. The dynamic X form is bounded by that
	// total at the X ask, so the offer gate needs no assumption about the
	// not-yet-chosen value.
	energyTotal := int32(0)
	for _, part := range cost.Energy {
		if part.Spec == "X" {
			continue
		}
		energyTotal += part.N
	}
	if energyTotal > 0 && e.G.Players[p].Counter("ENERGY") < energyTotal {
		return false
	}
	// Return cost parts (Return<N/Spec>): the source itself (Spec CARDNAME,
	// Forge's payCostFromSource) must be in play; otherwise the payer controls
	// at least N distinct matching permanents, reserved against the Sac and
	// Exile reservations above so two parts cannot claim one permanent.
	for _, part := range cost.Return {
		spec := sacrificeMatchSpec(part.Spec)
		if strings.EqualFold(spec, "CARDNAME") {
			if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
				return false
			}
			continue
		}
		var avail []state.ObjID
		for _, oid := range e.G.Zone(state.ZBattlefield, p) {
			if reserved[oid] {
				continue
			}
			if effects.MatchesSpecFrom(e.G, spec, oid, p, id) {
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
	// PutToLib cost parts (PutCardToLibFrom<Zone><N/Pos/Spec>): the payer needs
	// N matching cards in the part's zone. A Battlefield CARDNAME part is the
	// source itself (Forge's payCostFromSource), which must still be on the
	// battlefield. Cards are reserved against the earlier Sac/Exile/Return
	// parts so two components cannot claim the same permanent. The library
	// POSITION never affects payability.
	for _, part := range cost.PutToLib {
		spec := sacrificeMatchSpec(part.Spec)
		if part.N == 1 && part.Zone == state.ZBattlefield && strings.EqualFold(spec, "CARDNAME") {
			// The singleton self-reference fast path only covers N=1; a larger
			// N needs the general candidate walk below (it would otherwise be
			// offered on the source alone and abort at payment time). The
			// controller check matches putToLibAsk's candidates branch: a
			// control-changed source is not a cost the payer can pay.
			if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != p {
				return false
			}
			reserved[id] = true
			continue
		}
		var avail []state.ObjID
		for _, oid := range e.G.Zone(part.Zone, p) {
			if reserved[oid] {
				continue
			}
			if effects.MatchesSpecFrom(e.G, spec, oid, p, id) {
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
	for _, part := range cost.Draw {
		// Draw cost parts (Draw<N/Spec>): the cast flow draws the PAYER; a
		// spec naming a trigger-only role (Player.TriggeredPlayer and friends)
		// has no binding here and is unpayable -- never offered -- rather than
		// silently drawing nobody. A dynamic part (Draw<X/Spec>) additionally
		// needs its SVar-resolved count to evaluate at payment; an
		// unresolvable body withholds the whole cost (fail closed, the
		// fixLifeXCost direction), never an unpayable offer with a zero draw.
		if _, ok := castFlowDrawPlayer(part.Spec, p); !ok {
			return false
		}
		if part.Dyn != "" {
			if _, ok := e.drawCostCount(id, p, part); !ok {
				return false
			}
		}
	}
	if o := e.G.Obj(id); o != nil {
		for _, part := range cost.SubCounter {
			// An announced SubCounter<X/Kind> part's count is the cast's X,
			// bounded by the source's counter count at the X ask; the offer
			// gate makes no assumption about the not-yet-chosen value.
			if part.Announced {
				continue
			}
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

// castFlowDrawPlayer resolves a Draw cost part's spec inside the
// cast/activation flow: the payer draws (spec "", You, Player, Self), and
// Player.Activator too, because within an activation the activator IS the
// payer. A trigger-only role has no binding here; nonManaCastable blocks
// such a part so it is never offered.
func castFlowDrawPlayer(spec string, payer state.PlayerID) (state.PlayerID, bool) {
	switch spec {
	case "", "You", "Player", "Self", "Player.Activator":
		return payer, true
	}
	return 0, false
}

// drawCostCard emits the ordinary Draw event one card of a cost payment
// draws: the library's top card moves to the payer's hand, and a draw from
// an empty library is the loss the SBA checks (the same shape DrawFor's
// no-replacement draw and resumeOrdinaryDraw emit).
func (e *Engine) drawCostCard(p state.PlayerID) {
	lib := e.G.Zone(state.ZLibrary, p)
	if len(lib) == 0 {
		e.emit(events.Event{Kind: events.PlayerLost, Player: p, Text: "drew from an empty library"})
		return
	}
	e.emit(events.Event{Kind: events.Draw, Player: p, Obj: lib[0],
		From: state.ZLibrary, To: state.ZHand, Secret: true})
}

// payDrawCostParts settles every Draw cost component of a cast or activation
// payment: one ordinary draw per card of the part's count, for the drawer the
// part's spec names. The count is the literal N, or -- for the dynamic
// Draw<X/Spec> form -- the source's SVar bound by part.Dyn, resolved here at
// payment time (Champion of Wits' "draw cards equal to its power"). The
// offer gate (nonManaCastable) already proved each part's drawer and dynamic
// count resolvable, so a part that is somehow unresolvable at payment -- a
// stale stored cost -- pays nothing rather than guessing a count; the whole
// cost is never offered, so this is a belt-and-braces no-op, not a live path.
func (e *Engine) payDrawCostParts(pc *pendingCast) {
	for _, part := range pc.cost.Draw {
		drawer, ok := castFlowDrawPlayer(part.Spec, pc.player)
		if !ok {
			continue
		}
		n, ok := e.drawCostCount(pc.card, pc.player, part)
		if !ok {
			continue
		}
		for k := int32(0); k < n; k++ {
			e.drawCostCard(drawer)
		}
	}
}

// settlePutToLibCost settles every PutToLib cost component of a cast or
// activation payment (PutCardToLibFrom<Zone><N/Pos/Spec>): the chosen cards
// move to their OWNER's library. MoveZone appends to the destination zone, so
// a plain move lands at the bottom (Forge's Pos -1); a top placement (Pos 0)
// follows the move with one LibraryOrder per owner putting the moved cards
// back on top in the order they were chosen -- exactly the shape effects'
// libraryOrderPlacement emits (the same private flag), re-derived here rather
// than imported because effects must never be reached for a cost settle.
func (e *Engine) settlePutToLibCost(pc *pendingCast) {
	idx := 0
	for _, part := range pc.cost.PutToLib {
		n := int(part.N)
		end := idx + n
		if end > len(pc.putToLibs) {
			end = len(pc.putToLibs)
		}
		picks := pc.putToLibs[idx:end]
		idx = end
		for _, id := range picks {
			if o := e.G.Obj(id); o != nil {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZLibrary,
					Text: "put on the library as a cost"})
			}
		}
		if part.LibraryPos == 0 && len(picks) > 0 {
			e.putLibPicksOnTop(picks)
		}
	}
}

// putLibPicksOnTop emits the LibraryOrder that lifts the just-moved picks to
// the top of each owner's library, preserving pick order. MoveZone already
// appended them to the bottom; the order carries the complete new library and
// is Secret (a hidden zone must not leak). Grouped per owner in first-pick
// order -- deterministic because the picks slice is -- so a pick owned by
// another player lands in the right library (the zone owner Move already used).
func (e *Engine) putLibPicksOnTop(picks []state.ObjID) {
	byOwner := map[state.PlayerID][]state.ObjID{}
	var owners []state.PlayerID
	for _, id := range picks {
		o := e.G.Obj(id)
		if o == nil || o.Zone != state.ZLibrary {
			continue
		}
		if _, ok := byOwner[o.Owner]; !ok {
			owners = append(owners, o.Owner)
		}
		byOwner[o.Owner] = append(byOwner[o.Owner], id)
	}
	for _, owner := range owners {
		sel := byOwner[owner]
		selected := make(map[state.ObjID]bool, len(sel))
		for _, id := range sel {
			selected[id] = true
		}
		lib := e.G.Zone(state.ZLibrary, owner)
		order := make([]state.ObjID, 0, len(lib))
		order = append(order, sel...)
		for _, id := range lib {
			if !selected[id] {
				order = append(order, id)
			}
		}
		e.emit(events.Event{Kind: events.LibraryOrder, Player: owner, IDs: order, Secret: true})
	}
}

// payDamageCost makes the payer take n damage from the source -- the
// DamageYou<N> cost payment (Forge CostDamage). The event shape is the one
// payUnlessDamageCost emits: the Damage event names the payer, the engine's
// damage-source context names the source, and the same-source lifelink
// gains the controller the damage (CR 702.16d).
func (e *Engine) payDamageCost(payer state.PlayerID, n int32, source state.ObjID) {
	if n <= 0 {
		return
	}
	prev := e.SetDamageSource(source)
	ev := e.emit(events.Event{Kind: events.Damage, Player: payer, Amount: n})
	e.SetDamageSource(prev)
	if ev.Kind != events.Damage || !e.HasKeyword(source, "Lifelink") {
		return
	}
	controller := payer
	if o := e.G.Obj(source); o != nil && o.Zone == state.ZBattlefield {
		controller = o.Controller
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: controller, Amount: n})
}

func (e *Engine) costCandidates(p state.PlayerID, source state.ObjID, zone state.Zone, spec string, excludeSource, untapped bool) []state.ObjID {
	var out []state.ObjID
	for _, id := range e.G.Zone(zone, p) {
		o := e.G.Obj(id)
		if o == nil || (excludeSource && id == source) || (untapped && o.Tapped) {
			continue
		}
		if effects.MatchesSpecFrom(e.G, spec, id, p, source) {
			out = append(out, id)
		}
	}
	return out
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
	if len(extra.Exile) > 0 {
		cost.Exile = append(append([]CostPart(nil), cost.Exile...), extra.Exile...)
	}
	if len(extra.Reveal) > 0 {
		cost.Reveal = append(append([]CostPart(nil), cost.Reveal...), extra.Reveal...)
	}
	if len(extra.Behold) > 0 {
		cost.Behold = append(append([]CostPart(nil), cost.Behold...), extra.Behold...)
	}
	if len(extra.TapPermanent) > 0 {
		cost.TapPermanent = append(append([]CostPart(nil), cost.TapPermanent...), extra.TapPermanent...)
	}
	if len(extra.Blight) > 0 {
		cost.Blight = append(append([]CostPart(nil), cost.Blight...), extra.Blight...)
	}
	if len(extra.Energy) > 0 {
		cost.Energy = append(append([]CostPart(nil), cost.Energy...), extra.Energy...)
	}
	if len(extra.Return) > 0 {
		cost.Return = append(append([]CostPart(nil), cost.Return...), extra.Return...)
	}
	if len(extra.Draw) > 0 {
		cost.Draw = append(append([]CostPart(nil), cost.Draw...), extra.Draw...)
	}
	if len(extra.LifeX) > 0 {
		cost.LifeX = append(append([]CostPart(nil), cost.LifeX...), extra.LifeX...)
	}
	if len(extra.DamageYou) > 0 {
		cost.DamageYou = append(append([]CostPart(nil), cost.DamageYou...), extra.DamageYou...)
	}
	cost.Forage = cost.Forage || extra.Forage
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
	if o == nil {
		return
	}
	from := o.Zone
	var faceBefore *uint8
	if opt.Mode == "room_alt" {
		if roomAlternateCastFace(o) == nil {
			return
		}
		before := o.FaceIdx
		faceBefore = &before
		e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: int32(1 - int(before))})
	}
	// CR 714: the same flip mechanism serves the Adventure faces. From the
	// hand the cast flips to the Adventure spell face (adventure_alt); from
	// the adventure zone it flips back to the main face (adventure_recast).
	// Everything downstream -- rawBaseCost, targets, timing, resolution --
	// then reads the flipped face, because o.Face() is Faces[FaceIdx]. An
	// aborted proposal restores the pre-flip face via pc.faceBefore (CR
	// 733.1), the same reversal a Room cast takes.
	if opt.Mode == "adventure_alt" {
		if adventureSpellFace(o) == nil {
			return
		}
		before := o.FaceIdx
		faceBefore = &before
		e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: int32(1 - int(before))})
	}
	if opt.Mode == "adventure_recast" {
		if o.Zone != state.ZExile || adventureSpellFace(o) == nil || o.Face() != o.Card.Faces[1] {
			return
		}
		before := o.FaceIdx
		faceBefore = &before
		e.emit(events.Event{Kind: events.FlipFace, Obj: id, Amount: int32(1 - int(before))})
	}
	f := o.Face()
	if f == nil {
		return
	}

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
	var announceAlt *altCostView
	if opt.AltCostIndex > 0 {
		if alts := e.alternativeCosts(p, id); opt.AltCostIndex-1 < len(alts) {
			cost = alts[opt.AltCostIndex-1].cost
			announceAlt = &alts[opt.AltCostIndex-1]
		}
	}
	switch opt.Mode {
	case "kicked":
		if kc, ok := kickerCost(f); ok {
			cost = cost.Plus(kc)
		}
	case "kicked1", "kicked2", "kickedboth":
		// The and/or Kicker's per-part modes (legal.go offers one option per
		// independently payable part): each mode adds exactly the parts its
		// name promises. An out-of-range or unparseable form (a stale option
		// or a face whose Kicker changed) falls back to the base cost --
		// the same no-crash read the AltCostIndex fallback below takes.
		if c1, c2, ok := twoPartKickerCosts(f); ok {
			switch opt.Mode {
			case "kicked1":
				cost = cost.Plus(c1)
			case "kicked2":
				cost = cost.Plus(c2)
			default:
				cost = cost.Plus(c1).Plus(c2)
			}
		}
	case "surged":
		if sc, ok := surgeCost(f); ok {
			cost = sc
		}
	case "buyback":
		if bc, ok := buybackCost(f); ok {
			cost = cost.Plus(bc)
		}
	case "replicated":
		// Replicate (CR 702.55a): the mode marks the intent to pay the
		// optional replicate cost. The PAYMENT COUNT is a cast announcement
		// of its own (CR 601.2b), settled by replicateAsk before the
		// Convoke/X stages and folded into cost there, so a declined count
		// leaves an exactly plain cast and the offer gate's own base+1
		// composition is never silently charged for a different count. The
		// parameter itself is captured onto the pendingCast after its
		// construction (the suspend block below), keeping this switch's
		// cost-folding contract intact.
	case "multikicked":
		// Multikicker (CR 702.43): the same shape as "replicated" above --
		// the mode marks the intent to pay the optional multikicker cost;
		// the PAYMENT COUNT is settled by multikickAsk before the Convoke/X
		// stages and folded into cost there, so this case folds NOTHING. A
		// declined count leaves an exactly plain cast (modeFlags maps this
		// mode to ""), and the pay-time CastInfo rides the trailing
		// FlagMultikicked event.
	case "harmonize":
		if hc, ok := harmonizeCost(f); ok {
			cost = hc
		}
	case "suspend":
		if sc, ok := suspendCost(f); ok {
			cost = sc.cost
		}
	case "suspend_cast":
		cost = Cost{}
	case "foretell":
		// CR 702.126a: the Foretell ACTION pays {2} and exiles the card face
		// down -- never the keyword's own colon parameter, which prices the
		// LATER cast (the foretell_cast case below).
		cost = Cost{Generic: 2}
	case "foretell_cast":
		// CR 702.126a: the later cast pays the foretell cost -- the K: line's
		// colon parameter (Starnheim Unleashed's "X X W" rides the ordinary
		// X machinery here), read off the face; a missing parameter falls
		// back to the rule's action default {2} (no corpus carrier -- every
		// K:Foretell line carries a colon cost, measured 55/55). Stored RAW:
		// cost modifiers apply later in manaToPay, exactly like every other
		// alternative-cost mode's cost.
		if fc, ok := f.KeywordParam("Foretell"); ok && strings.TrimSpace(fc) != "" {
			cost = ParseCost(fc)
		} else {
			cost = Cost{Generic: 2}
		}
	case "flashback":
		cost = e.flashbackCost(id)
	case "mayplay":
		// rules/mayplay.go granted this play from a non-hand zone. The
		// printed cost is paid (the default below) unless the granting
		// static said MayPlayWithoutManaCost$ True, in which case the mana
		// part is free while non-mana additional costs still apply
		// (CR 118.9) -- so this mode folds into the withSpellAbilityExtras
		// condition below, exactly like a plain cast. CR 118.3a: the
		// granting static's RaiseCost$ surcharge is then added on top
		// through the SAME helper the offer walk (legal.go's may-play spell
		// walk) used, so the offered cost and the charged cost structurally
		// cannot disagree. A raise the helper could not price leaves
		// mayPlayGrant withholding the card; the cast never reaches here.
		if free, ok := e.mayPlayGrant(p, id); ok && free {
			cost = Cost{}
		}
		if raise, hasRaise, priced := e.mayPlayRaiseCost(p, id); hasRaise && priced {
			cost = cost.Plus(raise)
		}
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
	case "escape":
		if ec, ok := e.escapeCost(id); ok {
			cost = ec
		} else {
			cost = Cost{}
		}
	case "evoked", "dashed", "overloaded", "warped", "madness", "bestowed":
		// The alternative-cost keyword family (altcosts): each mode's cost is
		// the printed keyword parameter in place of the mana cost, exactly the
		// Miracle shape. Evoke and Madness casts come from hand and exile
		// respectively via the pending-trigger/cast-offer machinery; a stale
		// option whose keyword is gone (the face cannot change, so in practice
		// only a hand-built option) falls back to the empty cost rather than
		// charging the printed mana cost.
		head := map[string]string{"evoked": "Evoke", "dashed": "Dash",
			"overloaded": "Overload", "warped": "Warp", "madness": "Madness",
			"bestowed": "Bestow"}[opt.Mode]
		if mc, ok := f.KeywordParam(head); ok {
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
	if opt.AltCostIndex == 0 && (opt.Mode == "" || opt.Mode == "mayplay" || opt.Mode == "room_alt" ||
		opt.Mode == "adventure_alt") {
		cost = withSpellAbilityExtras(f, cost)
	}
	// Convoke and Harmonize are announced only after X/mode/pip choices have
	// formed the total cost (convokeAsk). Do not preselect creatures here:
	// doing so let one of those creatures activate a mana ability before its
	// delayed Tap payment.
	// The either-or additional cost (AlternateAdditionalCost) is a CHOICE,
	// not a fixed component, so the parts are only captured here and the ask
	// (altAddAsk) folds the chosen part into cost before any other cost stage
	// runs. Only a plain cast carries them: a kicked/surged/etc. cast of the
	// same card pays that mode's cost without recomposing this choice (no
	// corpus card pairs both shapes). The OFFER gate already proved at least
	// one part is payable (legal.go); the ask narrows it to exactly one.
	tax := e.commanderTaxAmount(p, id)
	mods := e.costModifiers(p, id, spellScope(opt.Mode))
	// The SVar-fixed PayLife<X> conversion (fixLifeXCost) -- the same helper
	// offerCastable shaped the offered cost with, so the stored cost and the
	// gated charge agree. A fixed face's value folds into Life here; the
	// withheld (unresolvable-body) shape cannot reach this line through any
	// offer gate, and a stale option that does degrades to a no-op before
	// anything is pushed or charged.
	converted, ok := e.fixLifeXCost(p, id, cost)
	if !ok {
		return
	}
	cost = converted
	if opt.AltCostIndex == 0 && opt.Mode == "" {
		pcAlt := altAddCostParts(f)
		e.cast = &pendingCast{player: p, card: id, from: from, mode: opt.Mode, ability: -1,
			cost: cost, faceBefore: faceBefore, mods: mods, taxGeneric: tax, altAddParts: pcAlt}
	} else {
		e.cast = &pendingCast{player: p, card: id, from: from, mode: opt.Mode, ability: -1,
			cost: cost, faceBefore: faceBefore, mods: mods, taxGeneric: tax}
	}
	// The announce-bearing alternative (the Shoal cycle) and the
	// TargetsWithSameController rider (Lodestone Bauble) ride the selected
	// cast SA into the transaction: xAsk's announce arm and exAsk's binding
	// read the first, handleTarget's Submit-time validator the second.
	if announceAlt != nil && announceAlt.announce != "" {
		e.cast.announceX = announceAlt.announce
	}
	if sa := f.SpellAbility(); sa != nil && strings.EqualFold(strings.TrimSpace(sa.Params["TargetsWithSameController"]), "True") {
		e.cast.sameCtrlTargets = true
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
	if opt.Mode == "suspend" {
		if sc, ok := suspendCost(f); ok && sc.timeX {
			e.cast.suspendTimeX, e.cast.suspendMinX = true, sc.minTime
		}
	}
	// Replicate (CR 702.55a): the cost is carried as its raw keyword
	// parameter and re-parsed by replicateAsk and the answer handler -- a
	// string survives the intent boundary's pendingCast Clone without
	// deep-copying cost slices, and ParseCost is deterministic.
	if opt.Mode == "replicated" {
		if _, ok := replicateCost(f); ok {
			e.cast.replicateParam, e.cast.replicateSet = f.KeywordParam("Replicate")
		}
	}
	// Multikicker (CR 702.43): the cost is carried as its raw keyword
	// parameter and re-parsed by multikickAsk and the answer handler -- the
	// same string-survives-Clone convention the replicate capture above
	// documents.
	if opt.Mode == "multikicked" {
		if _, ok := multikickerCost(f); ok {
			e.cast.multikickParam, e.cast.multikickSet = f.KeywordParam("Multikicker")
		}
	}
	// CR 401.5's MayPlayIgnoreColor$ rider: "you may spend mana as though it
	// were mana of any color to cast it". Recorded from the grant the offer
	// gate consulted while the card was still in the granted zone.
	if opt.Mode == "mayplay" {
		e.cast.mayPlayIgnore = e.payerGrantsIgnoreColor(p, id)
		e.cast.mayPlayIgnoreType = e.payerGrantsIgnoreType(p, id)
	}
	e.collectETBChoices(p)
	e.continueCast()
}

// convertedManaCostToken matches Forge's ConvertedManaCost placeholder inside
// a PlayCost$ token, case-insensitively (the corpus spells it exactly this
// way; the case fold costs nothing).
var convertedManaCostToken = regexp.MustCompile(`(?i)convertedmanacost`)

// pricePlayCost prices a Play effect's PlayCost$ token for one chosen card:
// the ConvertedManaCost placeholder is substituted with the card face's mana
// value (Amped Raptor's "an amount of {E} equal to its mana value") and the
// result is parsed with the ordinary cost grammar -- PayEnergy<N>, PayLife<N>,
// a fixed generic, and Discard<N/Spec> all land in the Cost fields the cast
// flow already asks and charges. A token the grammar reports as unmodelled
// (Cost.Unknown -- the corpus's one PlayCost$ SuspendCost, The Face of Boe)
// is NOT degraded the way a printed cost's malformed token would be:
// PlayCost$ is an ALTERNATIVE to the mana cost (CR 118.9 "rather than paying
// its mana cost"), so degrading it to one generic would still charge the
// player full price -- the caller hard-declines instead, ParseUnlessCost-style.
func pricePlayCost(f *cards.Face, token string) (Cost, bool) {
	s := convertedManaCostToken.ReplaceAllString(token, strconv.FormatInt(int64(f.ManaValue()), 10))
	c := ParseCost(s)
	if len(c.Unknown) > 0 {
		return Cost{}, false
	}
	return c, true
}

// beginPlay is the rules' hand-off for an answered Play effect. It starts a
// cast from the card's current zone and uses its printed cost unless that
// specific Play SA said WithoutManaCost$ True (Spinerock Knoll grants a free
// cast, while Conduit of Worlds requires payment) or carries a PlayCost$
// alternative (Amped Raptor's energy cast), which REPLACES the mana cost
// only -- additional costs and cost modifiers ride exactly as an ordinary
// cast's do (CR 118.9 / 601.2f). The card must still be on the stack of the
// suspended Play resolution when this runs; a malformed answer degrades to a
// logged no-op rather than panic.
func (e *Engine) beginPlay(p state.PlayerID, id state.ObjID, withoutManaCost bool, playCost string) {
	o := e.G.Obj(id)
	if o == nil || o.Face() == nil {
		e.emit(events.Event{Kind: events.Note, Player: p, Text: "Play found no card to play"})
		return
	}
	if o.Face().IsLand() {
		// A "play" permission can play a land, but it does not grant an
		// additional land drop. Lands never become spells or enter the stack.
		if int(p) >= len(e.G.Players) || e.G.Players[p].LandsPlayed >= 1 {
			e.emit(events.Event{Kind: events.Note, Player: p, Text: "Play cannot use an additional land drop"})
			return
		}
		e.cast = &pendingCast{player: p, card: id, from: o.Zone, mode: "land", ability: -1}
		e.collectETBChoices(p)
		e.continueCast()
		return
	}
	cost := e.rawBaseCost(p, id)
	if withoutManaCost {
		cost = Cost{}
	} else if playCost != "" {
		// A PlayCost$ alternative replaces the mana cost; an unpriceable
		// token is a hard DECLINE, never a mana fallback -- the player is
		// never charged full mana for a "rather than" alternative.
		alt, ok := pricePlayCost(o.Face(), playCost)
		if !ok {
			e.emit(events.Event{Kind: events.Note, Player: p,
				Text: "Play cannot price its alternative cost (" + playCost + "); the play is declined"})
			return
		}
		// The alternative's non-mana parts must be payable the way an
		// offered cast's would be (the energy total, the discard
		// candidates): a YES answer the payment cannot settle is declined
		// with a Note, not begun and short-changed at the settle.
		if !e.nonManaCastable(p, id, alt, false) {
			e.emit(events.Event{Kind: events.Note, Player: p,
				Text: "The alternative cost cannot be paid (" + playCost + "); the play is declined"})
			return
		}
		if alt.Life > e.G.Players[p].Life {
			e.emit(events.Event{Kind: events.Note, Player: p,
				Text: "The alternative cost cannot be paid (" + playCost + "); the play is declined"})
			return
		}
		cost = alt
	}
	// A normal Play cast pays its printed mana cost; a free Play cast does
	// not; a PlayCost$ cast pays the alternative. All three still pay
	// non-mana additional costs, exactly as an ordinary cast does (CR
	// 118.9 / 601.2f). The cost is stored RAW (no cost modifiers folded):
	// RaiseCost/ReduceCost ride pc.mods and manaToPay applies them after {X}
	// is folded, the same shape beginCast stores.
	cost = withSpellAbilityExtras(o.Face(), cost)
	converted, ok := e.fixLifeXCost(p, id, cost)
	if !ok {
		// The offer gate withheld this cost; a stale Play degrades to a no-op.
		return
	}
	cost = converted
	mods := e.costModifiers(p, id, spellScope(""))
	e.cast = &pendingCast{player: p, card: id, from: o.Zone, mode: "play", ability: -1,
		cost: cost, mods: mods}
	e.collectETBChoices(p)
	e.continueCast()
}

// applyDredge performs a dredged replacement of a draw: mill N cards (N = the
// dredge card's Dredge number) from p's library into the graveyard, then move
// the dredge card from p's graveyard to their hand. The ordinary draw was
// skipped by choosing option 0 in DrawFor's dredge ask.
func (e *Engine) applyDredge(p state.PlayerID, dredgeID state.ObjID) {
	o := e.G.Obj(dredgeID)
	if o == nil || o.Face() == nil {
		return
	}
	n := int32(0)
	if v, ok := o.Face().KeywordParam("Dredge"); ok {
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			n = int32(parsed)
		}
	}
	lib := e.G.Zone(state.ZLibrary, p)
	// A stale or malformed answer must not turn an illegal insufficient-library
	// dredge into a partial mill: CR 702.55 requires all N cards.
	if n <= 0 || int(n) > len(lib) {
		return
	}
	for _, id := range lib[:n] {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZGraveyard})
	}
	if dredgeID != 0 && o.Zone != state.ZHand {
		e.emit(events.Event{Kind: events.MoveZone, Obj: dredgeID,
			From: o.Zone, To: state.ZHand})
	}
}

// resumeOrdinaryDraw re-emits the ordinary draw a declined dredge skipped.
// DrawFor posed the dredge ask and suspended; a "no" (option 1) means the
// player draws as normal, which is exactly the Draw event DrawFor would have
// emitted had no dredger been in the graveyard. Only the library's top card
// moves; a decline with an empty library is a loss, checked by the SBA.
func (e *Engine) resumeOrdinaryDraw(p state.PlayerID) {
	lib := e.G.Zone(state.ZLibrary, p)
	if len(lib) == 0 {
		e.emit(events.Event{Kind: events.PlayerLost, Player: p, Text: "drew from an empty library"})
		return
	}
	e.emit(events.Event{Kind: events.Draw, Player: p, Obj: lib[0],
		From: state.ZLibrary, To: state.ZHand, Secret: true})
}

// continueCast runs the cast flow's stages in order -- announced
// Convoke/Harmonize contributions, X, Delve, each Sac and Discard part --
// stopping (and returning) the instant a stage asks a KChoose;
// commitCast runs once every stage has settled. A nil e.cast (a chooseCast
// answer arriving with no flow in progress, only reachable from a
// hand-built decision) is dropped rather than panicked on, mirroring
// castAnswer's own guard.
func (e *Engine) continueCast() {
	if e.cast == nil {
		return
	}
	if e.altAddAsk() {
		return
	}
	if e.forageAsk() || e.revealCostAsk() || e.beholdCostAsk() || e.tapPermanentCostAsk() || e.blightCostAsk() {
		return
	}
	// CR 601.2b: the replicate count (CR 702.55a's optional additional cost,
	// paid any number of times) is announced before Convoke/Harmonize and X,
	// whose asks must see and bound against the composed total.
	if e.replicateAsk() {
		return
	}
	// CR 601.2b: the multikicker count (CR 702.43's optional additional cost,
	// paid any number of times) is announced the same way -- no Kicker
	// carrier pairs both keywords (measured over the corpus), so the two
	// asks never coexist on one cast.
	if e.multikickAsk() {
		return
	}
	// CR 601.2b announces Convoke/Harmonize before X: an announced creature
	// contribution is part of the available payment for X, and cannot be used
	// as a mana source in the later mana window.
	if e.convokeAsk() {
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
	if e.exAsk() {
		return
	}
	if e.returnAsk() {
		return
	}
	if e.putToLibAsk() {
		return
	}
	if e.etbAsk() {
		return
	}
	// Suspend does not put a spell on the stack: its alternate action pays
	// the keyword cost and exiles the card with time counters. Targets are
	// chosen only when its later free cast is announced.
	if e.cast.mode == "suspend" {
		e.payCast()
		return
	}
	// Foretell (CR 702.126a) is the same shape: the {2} special action is not
	// a cast -- no stack push, no targets; the card is exiled face down and
	// its later foretell-cost cast announces its own targets.
	if e.cast.mode == "foretell" {
		e.payCast()
		return
	}
	// CR 601.2a: the object reaches the stack before the target choice
	// (601.2c) and payment (601.2h). For a spell the cast trigger (601.2i)
	// is held back until payCast; an ability's AbilityPush fires no trigger.
	if e.pushCast() {
		return
	}
	// castprov3: a provenance-keyed cost modifier (Bilbo's
	// "!wasCastFromYourHand" ReduceCost) is unresolvable before CR 601.2a's
	// push — the offer and option-selection snapshots both denied it (full
	// price, the fail-closed direction) because the priced card had no cast
	// in the log yet. Now the PutOnStack is in the log: when the selection
	// pass evaluated such a static (e.costProvenanceSeen, the
	// noCounterSpend-style transient capture), re-price the pending cast so
	// the payment takes the honest reduction. For every other cast the
	// recompute is byte-identical to the offer snapshot (both are the
	// nil-target base snapshot), so no existing price — and no chain head —
	// moves.
	if pc := e.cast; pc != nil && pc.pushed && !pc.provenanceRepriced && e.costProvenanceSeen {
		pc.provenanceRepriced = true
		pc.mods = e.costModifiers(pc.player, pc.card, spellScope(pc.mode))
	}
	// FlagSuspend is exile provenance, not cast-time state. Clear it when the
	// mandatory free cast starts so a later unrelated exile move cannot revive
	// an old suspension.
	if e.cast.mode == "suspend_cast" && !e.cast.suspendCastClear {
		e.emit(events.Event{Kind: events.CastInfo, Obj: e.cast.card})
		e.cast.suspendCastClear = true
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

// altAddAsk poses the AlternateAdditionalCost either-or choice: which ONE
// alternative additional cost this cast pays (CR 601.2h). It runs FIRST in
// continueCast -- before the {X} ask -- because the chosen part folds into
// cost and every later stage (Delve, sac, discard, the mana window and the
// final payment) must see the total it will actually charge. ALL parts are
// offered, the payable ones FIRST (deterministically: script order within
// each group), so an automated seat taking the first option always takes one
// it can pay; a seat that picks an unpayable part walks into a later stage's
// abort (CR 733.1 reversal). When NO part is payable the flow aborts (the
// offer gate already proved one was payable at offer time, so this is a
// board that changed under the flow).
func (e *Engine) altAddAsk() bool {
	pc := e.cast
	if pc == nil || pc.altAddDone || len(pc.altAddParts) == 0 {
		return false
	}
	pc.altAddDone = true
	if len(pc.altAddParts) == 1 {
		part := ParseCost(pc.altAddParts[0])
		if !e.castable(pc.player, pc.card, pc.cost.Plus(part), pc.ability >= 0) {
			e.abortCast(pc, "additional cost no longer payable; cast aborted", true)
			return true
		}
		pc.cost = pc.cost.Plus(part)
		return false
	}
	payable := make([]int, 0, len(pc.altAddParts))
	unpayable := make([]int, 0, len(pc.altAddParts))
	for i, part := range pc.altAddParts {
		if e.castable(pc.player, pc.card, pc.cost.Plus(ParseCost(part)), pc.ability >= 0) {
			payable = append(payable, i)
		} else {
			unpayable = append(unpayable, i)
		}
	}
	order := append(append([]int(nil), payable...), unpayable...)
	if len(payable) == 0 {
		e.abortCast(pc, "additional cost no longer payable; cast aborted", true)
		return true
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose an additional cost to cast " + e.targetName(pc.card), Source: pc.card}
	for _, i := range order {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "altaddcost",
			Label: "Pay " + formatCost(ParseCost(pc.altAddParts[i])), Amount: i})
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
}

func (e *Engine) forageAsk() bool {
	pc := e.cast
	if pc == nil || !pc.cost.Forage || pc.forageDone {
		return false
	}
	pc.forageDone = true
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose how to forage", Source: pc.card}
	if len(e.G.Zone(state.ZGraveyard, pc.player)) >= 3 {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "forage_exile", Label: "Exile three cards from your graveyard"})
	}
	for _, id := range e.costCandidates(pc.player, pc.card, state.ZBattlefield, "Food.YouCtrl", false, false) {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "forage_food", Obj: id,
			Label: "Sacrifice " + e.targetName(id)})
	}
	if len(d.Options) == 0 {
		e.abortCast(pc, "forage no longer payable; cast aborted", true)
		return true
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
}

func (e *Engine) revealCostAsk() bool {
	pc := e.cast
	for pc.revealPart < len(pc.cost.Reveal) {
		part := pc.cost.Reveal[pc.revealPart]
		candidates := e.costCandidates(pc.player, pc.card, state.ZHand, part.Spec, true, false)
		if len(candidates) < int(part.N) {
			e.abortCast(pc, "reveal cost no longer payable; cast aborted", true)
			return true
		}
		if len(candidates) == int(part.N) {
			pc.reveals = append(pc.reveals, candidates...)
			pc.revealPart++
			continue
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: int(part.N), Max: int(part.N),
			Prompt: "Choose cards to reveal", Source: pc.card}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "revealcost", Obj: id, Label: e.targetName(id)})
		}
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	return false
}

func (e *Engine) beholdCostAsk() bool {
	pc := e.cast
	for pc.beholdPart < len(pc.cost.Behold) {
		part := pc.cost.Behold[pc.beholdPart]
		candidates := append(e.costCandidates(pc.player, pc.card, state.ZBattlefield, part.Spec, false, false),
			e.costCandidates(pc.player, pc.card, state.ZHand, part.Spec, true, false)...)
		if len(candidates) < int(part.N) {
			e.abortCast(pc, "behold cost no longer payable; cast aborted", true)
			return true
		}
		if len(candidates) == int(part.N) {
			pc.beholds = append(pc.beholds, candidates...)
			pc.beholdPart++
			continue
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: int(part.N), Max: int(part.N),
			Prompt: "Choose permanents or cards to behold", Source: pc.card}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "beholdcost", Obj: id, Label: e.targetName(id)})
		}
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	return false
}

func (e *Engine) tapPermanentCostAsk() bool {
	pc := e.cast
	for pc.tapPart < len(pc.cost.TapPermanent) {
		part := pc.cost.TapPermanent[pc.tapPart]
		candidates := e.costCandidates(pc.player, pc.card, state.ZBattlefield, part.Spec, false, true)
		// One permanent can never pay two parts of the same cost, so the
		// candidate filter claims everything an earlier stage already recorded:
		// an earlier tap part's choice (taps settle together at payCast, so the
		// state does not yet show it), a Convoke/Harmonize creature announced
		// for a spell whose dyn tap part deferred to xAsk (the settle runs
		// after the announcement), and -- for every part, literal and dynamic
		// alike -- the source itself when a {T} in the same cost will tap it at
		// payCast (Forge CostTap): the {T} and the tapXType can never spend one
		// permanent twice.
		if len(pc.taps) > 0 || len(pc.convoke) > 0 || pc.cost.Tap {
			taken := make(map[state.ObjID]bool, len(pc.taps)+len(pc.convoke)+1)
			for _, id := range pc.taps {
				taken[id] = true
			}
			for _, pay := range pc.convoke {
				taken[pay.id] = true
			}
			if pc.cost.Tap {
				taken[pc.card] = true
			}
			kept := make([]state.ObjID, 0, len(candidates))
			for _, cid := range candidates {
				if !taken[cid] {
					kept = append(kept, cid)
				}
			}
			candidates = kept
		}
		if part.Dyn != "" {
			// The dynamic tapXType heads (dynTapCost's doc). An X-form part whose
			// cost carries another announce-bearing part defers: xAsk (later in
			// continueCast's stage order) announces the X the part settles
			// exactly, so the ask must not run before the announcement exists
			// (Necron Overlord's "{X}, tap X untapped artifacts"). xAsk's own
			// bound caps the announced value by these candidates.
			if part.Dyn == "X" && !pc.xDone && (costAnnouncesCastX(pc.cost) || pc.announceX != "") {
				return false
			}
			// An Any-form part pays only by tapping at least one: no eligible
			// permanent (the affordability gate agreed, so this is a board that
			// changed under the offer) aborts the whole cast/activation.
			if part.Dyn == "Any" && len(candidates) == 0 {
				e.abortCast(pc, "tap cost no longer payable; cast aborted", true)
				return true
			}
			// An X-form election with no eligible permanent can only announce
			// X = 0 (CR 601.2b; the affordability gate agrees, so this is a
			// board that changed under the offer). A decision nobody could
			// answer differently is never emitted -- posting the Min 0/Max 0
			// empty ask panics rules/engine.go's ask -- so resolve it silently
			// with X = 0 and no taps, mirroring triggeredTapAsk's decline.
			if part.Dyn == "X" && !pc.xDone && len(candidates) == 0 {
				pc.x = 0
				pc.tapPart++
				continue
			}
			if part.Dyn == "X" && pc.xDone {
				// The announced X settles exactly: no choice beyond which
				// permanents, so a shortfall is the same unpayable abort.
				n := int(pc.x)
				if n > len(candidates) {
					e.abortCast(pc, "tap cost no longer payable; cast aborted", true)
					return true
				}
				if n > 0 {
					d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: n, Max: n,
						Prompt: "Choose permanents to tap", Source: pc.card}
					for _, id := range candidates {
						d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "tapcost", Obj: id, Label: e.targetName(id)})
					}
					e.choosing = chooseCast
					e.ask(d)
					return true
				}
				pc.tapPart++
				continue
			}
			// The election announces the count: Min 0 for the X form (X = 0 is
			// a legal announcement) and Min 1 for Any (a cost is not paid by
			// tapping nothing). The answer records the taps and, for the X form,
			// binds the cast's X to the chosen count (the "tapcost" answer arm).
			min := int32(1)
			if part.Dyn == "X" {
				min = 0
			}
			d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: int(min), Max: len(candidates),
				Prompt: "Choose permanents to tap", Source: pc.card}
			for _, id := range candidates {
				d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "tapcost", Obj: id, Label: e.targetName(id)})
			}
			e.choosing = chooseCast
			e.ask(d)
			return true
		}
		if len(candidates) < int(part.N) {
			e.abortCast(pc, "tap cost no longer payable; cast aborted", true)
			return true
		}
		if len(candidates) == int(part.N) {
			pc.taps = append(pc.taps, candidates...)
			pc.tapPart++
			continue
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: int(part.N), Max: int(part.N),
			Prompt: "Choose permanents to tap", Source: pc.card}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "tapcost", Obj: id, Label: e.targetName(id)})
		}
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	return false
}

func (e *Engine) blightCostAsk() bool {
	pc := e.cast
	for pc.blightPart < len(pc.cost.Blight) {
		candidates := e.costCandidates(pc.player, pc.card, state.ZBattlefield, "Creature.YouCtrl", false, false)
		if len(candidates) == 0 {
			e.abortCast(pc, "blight cost no longer payable; cast aborted", true)
			return true
		}
		if len(candidates) == 1 {
			pc.blights = append(pc.blights, candidates[0])
			pc.blightPart++
			continue
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt: "Choose a creature to blight", Source: pc.card}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "blightcost", Obj: id, Label: e.targetName(id)})
		}
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	return false
}

// exAsk offers the next unsettled Exile cost part (ExileFromHand /
// ExileFromGrave), walking pc.cost.Exile in order (pc.exilePart) the way
// sacAsk walks pc.cost.Sac. Candidates come from the part's zone (the payer's
// hand, or their graveyard for the encore self-exile shape) and are filtered
// through MatchesSpecFrom so CARDNAME self-references resolve to the source
// object exactly as they do for sacrifice costs. A part with too few
// candidates aborts the whole cast (nothing has moved yet); a part whose sole
// candidate IS the source records it without a decision, mirroring
// sacAsk's CARDNAME singleton rule.
func (e *Engine) exAsk() bool {
	pc := e.cast
	for pc.exilePart < len(pc.cost.Exile) {
		part := pc.cost.Exile[pc.exilePart]
		zone := part.Zone
		if zone == 0 {
			zone = state.ZHand
		}
		// The announce-bound filter (the Shoal cycle's cmcEQX): the announced
		// X binds the spec's non-literal RHS through SpecContext.Resolve — the
		// same closure mechanism a Chosen* predicate resolves through — so
		// the part's candidates are exactly the cards at the announced mana
		// value. pc.x is already settled (xAsk's announce arm ran first in
		// continueCast).
		var sc *effects.SpecContext
		if pc.announceX != "" {
			name := pc.announceX
			sc = &effects.SpecContext{You: pc.player, Source: pc.card, Resolve: func(n string) (int32, bool) {
				if n == name {
					return pc.x, true
				}
				return 0, false
			}}
		}
		var candidates []state.ObjID
		for _, oid := range e.G.Zone(zone, pc.player) {
			// A CAST (pc.ability < 0) can never exile the card it is casting:
			// the card sits in this zone until pushCast runs (CR 601.2a pushes
			// AFTER the cost asks), so without this skip a `Card` spec would
			// offer the cast card as its own ExileFromGrave fodder -- the
			// payment side of the same self-exclusion nonManaCastable applies
			// to the affordability walk. An ability activation (pc.ability >=
			// 0) is untouched: encore's Cost$ ExileFromGrave<1/CARDNAME>
			// really does exile its own source.
			if pc.ability < 0 && oid == pc.card {
				continue
			}
			match := effects.MatchesSpecFrom(e.G, part.Spec, oid, pc.player, pc.card)
			if sc != nil {
				match = effects.MatchesSpecCtx(e.G, part.Spec, oid, *sc)
			}
			if match {
				already := false
				for _, s := range pc.exiles {
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
			e.abortCast(pc, "exile cost no longer payable; cast/activation aborted", true)
			return true
		}
		// A singleton self-reference (encore's ExileFromGrave<1/CARDNAME>, the
		// sole candidate being the resolving card itself) has no player choice.
		if part.N == 1 && len(candidates) == 1 && candidates[0] == pc.card &&
			strings.EqualFold(part.Spec, "CARDNAME") {
			pc.exiles = append(pc.exiles, pc.card)
			pc.exilePart++
			continue
		}
		zoneName := "hand"
		switch zone {
		case state.ZGraveyard:
			zoneName = "graveyard"
		case state.ZBattlefield:
			zoneName = "battlefield"
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: n, Max: n,
			Prompt: "Exile " + strconv.Itoa(n) + " card(s) from your " + zoneName +
				" to cast " + e.targetName(pc.card), Source: pc.card}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "exilecost",
				Obj: id, Label: e.G.Obj(id).Face().Name})
		}
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	return false
}

// returnAsk offers the next unsettled Return cost part (Return<N/Spec>:
// a permanent matching Spec returned to its OWNER's hand -- Forge
// CostReturn.doPayment's moveToHand), walking pc.cost.Return in order
// (pc.returnPart) the way exAsk walks pc.cost.Exile. A Spec of CARDNAME is
// Forge's payCostFromSource: the resolving permanent itself is the sole
// candidate (Chthonian Nightmare's "Return Chthonian Nightmare to its
// owner's hand"), so no decision is posed. Any other Spec walks the payer's
// battlefield. The settle is payCast's: the chosen objects move to their
// owner's hand beside the other cost payments, so an abort cannot leave a
// partially paid return on the board.
func (e *Engine) returnAsk() bool {
	pc := e.cast
	for pc.returnPart < len(pc.cost.Return) {
		part := pc.cost.Return[pc.returnPart]
		spec := sacrificeMatchSpec(part.Spec)
		var candidates []state.ObjID
		if strings.EqualFold(spec, "CARDNAME") {
			if o := e.G.Obj(pc.card); o != nil && o.Zone == state.ZBattlefield {
				candidates = append(candidates, pc.card)
			}
		} else {
			candidates = e.costCandidates(pc.player, pc.card, state.ZBattlefield, spec, false, false)
		}
		n := int(part.N)
		if n <= 0 || n > len(candidates) {
			e.abortCast(pc, "return cost no longer payable; cast/activation aborted", true)
			return true
		}
		// A singleton self-reference has no player choice, mirroring
		// sacAsk/exAsk's CARDNAME singleton rule.
		if part.N == 1 && len(candidates) == 1 && candidates[0] == pc.card &&
			strings.EqualFold(spec, "CARDNAME") {
			pc.returns = append(pc.returns, pc.card)
			pc.returnPart++
			continue
		}
		verb := "cast " + e.targetName(pc.card)
		if pc.ability >= 0 {
			verb = "activate"
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: n, Max: n,
			Prompt: "Return " + strconv.Itoa(n) + " permanent(s) to their owner's hand to " + verb,
			Source: pc.card}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "returncost",
				Obj: id, Label: e.targetName(id)})
		}
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	return false
}

// putToLibAsk offers the next unsettled PutToLib cost part
// (PutCardToLibFrom<Zone><N/Pos/Spec>: cards matching Spec moved from the
// payer's Hand, Graveyard or Battlefield to the top or bottom of their OWNER's
// library -- Forge CostPutCardToLib.doPayment), walking pc.cost.PutToLib in
// order (pc.putToLibPart) the way returnAsk walks pc.cost.Return. A Spec of
// CARDNAME from the BATTLEFIELD is Forge's payCostFromSource: the resolving
// permanent itself is the sole candidate (Timestream Navigator's "{2}{U}{U},
// {T}, Put Timestream Navigator on the bottom of its owner's library"), so no
// decision is posed. Any other zone/spec walks the payer's own zone (a hand
// or graveyard cost may still name the source itself when the ability is
// activated from there -- no corpus carrier does, but costCandidates resolves
// it). The settle is payCast's: the chosen objects move to their owner's
// library beside the other cost payments, so an abort cannot leave a
// partially paid placement on the board.
func (e *Engine) putToLibAsk() bool {
	pc := e.cast
	for pc.putToLibPart < len(pc.cost.PutToLib) {
		part := pc.cost.PutToLib[pc.putToLibPart]
		spec := sacrificeMatchSpec(part.Spec)
		var candidates []state.ObjID
		if part.Zone == state.ZBattlefield && strings.EqualFold(spec, "CARDNAME") {
			// The source itself is the sole candidate (Forge's
			// payCostFromSource) -- but only while the payer still controls it:
			// a control-changed source is not a cost the payer can pay, so
			// leaving it out sends the ability to the no-candidates abort
			// below instead of paying with a permanent the payer does not own
			// the choice over (0 corpus carriers; fail-closed).
			if o := e.G.Obj(pc.card); o != nil && o.Zone == state.ZBattlefield && o.Controller == pc.player {
				candidates = append(candidates, pc.card)
			}
		} else {
			candidates = e.costCandidates(pc.player, pc.card, part.Zone, spec, false, false)
		}
		n := int(part.N)
		if n <= 0 || n > len(candidates) {
			e.abortCast(pc, "put-to-library cost no longer payable; cast/activation aborted", true)
			return true
		}
		// A singleton self-reference (CARDNAME from the battlefield) has no
		// player choice, mirroring sacAsk/exAsk/returnAsk's CARDNAME rule.
		if part.N == 1 && len(candidates) == 1 && candidates[0] == pc.card &&
			part.Zone == state.ZBattlefield && strings.EqualFold(spec, "CARDNAME") {
			pc.putToLibs = append(pc.putToLibs, pc.card)
			pc.putToLibPart++
			continue
		}
		verb := "cast " + e.targetName(pc.card)
		if pc.ability >= 0 {
			verb = "activate"
		}
		dest := "the top of their owner's library"
		if part.LibraryPos == -1 {
			dest = "the bottom of their owner's library"
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: n, Max: n,
			Prompt: "Put " + strconv.Itoa(n) + " card(s) from your " + putToLibZoneName(part.Zone) +
				" on " + dest + " to " + verb,
			Source: pc.card}
		for _, id := range candidates {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "puttolibcost",
				Obj: id, Label: e.targetName(id)})
		}
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	return false
}

// putToLibZoneName names a PutToLib part's origin zone in a human-readable
// prompt. It is the cost-flow vocabulary, deliberately separate from
// handDestPhrase/destinationPhrase in effects.
func putToLibZoneName(z state.Zone) string {
	switch z {
	case state.ZHand:
		return "hand"
	case state.ZGraveyard:
		return "graveyard"
	default:
		return "battlefield"
	}
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
	choices := strings.Split(sa.Params["Choices"], ",")
	legal := make([]string, 0, len(choices))
	for _, name := range choices {
		name = strings.TrimSpace(name)
		sub := cards.ResolveSVar(f.SVars, name)
		if sub == nil || sub.Params["ValidTgts"] == "" {
			legal = append(legal, name)
			continue
		}
		min, _ := e.resolvedTargetBounds(pc.player, pc.card, sub, pc.x)
		if len(e.legalTargetCandidates(pc.player, pc.card, pc.card, sub)) >= min {
			legal = append(legal, name)
		}
	}
	min, max, repeat := effects.CharmModeBounds(e, ctx, sa, len(legal))
	if min > len(legal) && !repeat {
		// No legal set of modes can complete its required target choices. This
		// is the modal counterpart of targetAsk's no-legal-target reversal; use
		// the no-progress suppression so an automated seat cannot propose the
		// same impossible cast forever. A repeatable Charm can fill its slots by
		// repeating an eligible mode, so it never aborts here.
		e.abortCast(pc, "cast aborted: no legal modal choice", true)
		return true
	}
	d := modeDecisionForChoices(pc.player, pc.card, sa, f.SVars, legal, min, max, repeat)
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

// replicateAsk poses CR 702.55a's replicate count question -- "you may pay
// [the replicate cost] any number of times as you cast this spell" -- once,
// before the Convoke/Harmonize and X stages, whose asks must see and bound
// against the composed total. The max is the largest N the current board can
// still pay, checked with the SAME affordability checker the payment window's
// composed total faces (castable: the conversion-aware mana gate plus every
// non-mana part), pool-only at ask time -- the 601.2g window afterwards may
// still produce mana for the composed total, exactly like a kicked cast. The
// answered count folds that many payments into cost (castAnswer); 0 declines:
// no flag, an exactly plain cast.
func (e *Engine) replicateAsk() bool {
	pc := e.cast
	if pc.replicateDone || !pc.replicateSet {
		return false
	}
	pc.replicateDone = true
	rc := ParseCost(pc.replicateParam)
	max := int32(0)
	cand := pc.cost
	for i := int32(0); i < 64; i++ {
		// The hard cap only exists so a degenerate future cost whose every
		// part prices against a non-reserving candidate count cannot loop;
		// every real replicate resource (mana, energy, life, tap/sac
		// candidates) is finite and breaks the loop naturally.
		next := cand.Plus(rc)
		if !e.castable(pc.player, pc.card, next, false) {
			break
		}
		cand = next
		max++
	}
	if max == 0 {
		// The offer gate proved one payment payable; a board that changed
		// under the proposal (or a cost modifier that priced the OFFER but
		// not this loop's bare, unmodified cost -- the bound here is
		// deliberately conservative, never over-offering) degrades the
		// explicitly chosen "(replicated)" mode to the count-0 plain cast
		// (the conservative CR 733 direction) rather than wedging or
		// aborting. The degrade is loud: a silent downgrade would leave the
		// player's choice unrecorded.
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Obj: pc.card,
			Text: "replicate no longer payable; casting without replicate"})
		return false
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Pay the replicate cost how many times?", Source: pc.card}
	for n := int32(0); n <= max; n++ {
		label := "No replicate"
		if n == 1 {
			label = "Pay replicate once"
		} else if n > 1 {
			label = fmt.Sprintf("Pay replicate %d times", n)
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "replicate",
			Label: label, Amount: int(n)})
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// multikickAsk poses CR 702.43's multikicker count question -- "you may pay
// [the multikicker cost] any number of times as you cast this spell" -- once,
// the replicateAsk shape (the same cast announcement, CR 601.2b): the max is
// the largest N the current board can still pay, walked with the SAME
// affordability checker the payment window's composed total faces (castable),
// pool-only at ask time. The answered count folds that many payments into
// cost (castAnswer); 0 declines: no flag, an exactly plain cast.
func (e *Engine) multikickAsk() bool {
	pc := e.cast
	if pc.multikickDone || !pc.multikickSet {
		return false
	}
	pc.multikickDone = true
	mk := ParseCost(pc.multikickParam)
	max := int32(0)
	cand := pc.cost
	for i := int32(0); i < 64; i++ {
		// The hard cap only exists so a degenerate future cost whose every
		// part prices against a non-reserving candidate count cannot loop;
		// every real multikicker resource is finite and breaks the loop
		// naturally (the replicateAsk comment).
		next := cand.Plus(mk)
		if !e.castable(pc.player, pc.card, next, false) {
			break
		}
		cand = next
		max++
	}
	if max == 0 {
		// The offer gate proved one payment payable; a board that changed
		// under the proposal degrades the explicitly chosen
		// "(multikicked)" mode to the count-0 plain cast (the replicateAsk
		// max==0 arm's conservative direction), never a wedge or abort.
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Obj: pc.card,
			Text: "multikicker no longer payable; casting without multikick"})
		return false
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Pay the multikicker cost how many times?", Source: pc.card}
	for n := int32(0); n <= max; n++ {
		label := "No multikick"
		if n == 1 {
			label = "Pay multikicker once"
		} else if n > 1 {
			label = fmt.Sprintf("Pay multikicker %d times", n)
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "multikick",
			Label: label, Amount: int(n)})
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
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
	// The announce-bearing alternative cost (the Shoal cycle's Announce$ X):
	// the announced X is bound by the exile settlement, not by mana — each
	// candidate value is a distinct mana value some exilable card still
	// matches at (altCostXCandidates walks the payer's hand, binding X to
	// each card's own mana value and keeping the matches). No pool bound
	// applies (the alt cost pays no mana X), and the pool-based payable walk
	// below is meaningless for it, so the arm returns straight from the
	// candidate set. An empty set — the hand changed under the offer gate —
	// aborts the cast (CR 733.1, nothing has moved).
	if pc.announceX != "" && len(pc.cost.Exile) > 0 {
		vals := e.altCostXCandidates(pc.player, pc.card, altCostView{
			cost: pc.cost, announce: pc.announceX, src: pc.card})
		if len(vals) == 0 {
			e.abortCast(pc, "announce cost no longer payable; cast aborted", true)
			return true
		}
		d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
			Prompt: "Choose a value for X", Source: pc.card}
		for _, x := range vals {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "x",
				Label: fmt.Sprintf("X = %d", x), Amount: int(x)})
		}
		e.choosing = chooseCast
		e.ask(d)
		return true
	}
	// A PayEnergy<X> part announces the same X the cast pays with (CR
	// 107.3i's ability X), so its presence triggers this ask exactly like a
	// printed {X} mana symbol does. A Sac<X/Spec> part announces the count of
	// permanents to sacrifice the same way; the announced PayLife<X> and
	// SubCounter<X/Kind> parts announce the same X too (the life and the
	// counter removal settle at exactly that value).
	energyX := false
	for _, part := range pc.cost.Energy {
		if part.Spec == "X" {
			energyX = true
		}
	}
	sacX := false
	for _, part := range pc.cost.Sac {
		if part.Announced {
			sacX = true
		}
	}
	subCounterX := false
	for _, part := range pc.cost.SubCounter {
		if part.Announced {
			subCounterX = true
		}
	}
	lifeXCount := len(pc.cost.LifeX)
	if pc.cost.X <= 0 && !energyX && !sacX && !subCounterX && lifeXCount == 0 {
		return false
	}
	min := int32(0)
	if pc.suspendTimeX {
		min = pc.suspendMinX
	}
	pool := e.G.Players[pc.player].Pool
	gy := int32(len(e.G.Zone(state.ZGraveyard, pc.player)))
	// Bound: past this many mana no further X is ever payable. In addition to
	// the pool and possible Delve, the already-announced Convoke/Harmonize
	// payments can cover X's generic requirement. Their exact application
	// below handles coloured costs before generic reductions; this bound need
	// only be a safe finite ceiling.
	credit := int32(0)
	for _, pay := range pc.convoke {
		if pay.power > 0 {
			credit += pay.power
		} else {
			credit++
		}
	}
	bound := pool.Total() + gy + credit + 1
	// A PayEnergy<X> cost part pays the SAME announced X in energy counters
	// (Forge CostPayEnergy.getMaxAmountX bounds a dynamic PayEnergy by the
	// payer's energy total). When the energy part is the ONLY X the cost
	// carries, the mana bound is irrelevant and the bound is exactly that
	// total; when a printed {X} also exists, the energy total still caps it
	// from above -- an X beyond it could be announced but never paid, and
	// CR 601.2b's announcement must be one the payment can settle.
	for _, part := range pc.cost.Energy {
		if part.Spec == "X" {
			energy := e.G.Players[pc.player].Counter("ENERGY")
			if pc.cost.X == 0 {
				bound = energy
			} else if energy < bound {
				bound = energy
			}
		}
	}
	// A Sac<X/Spec> part's bound is the number of matching permanents the
	// payer could sacrifice -- announcing a count beyond it could never be
	// settled (CR 601.2b's announcement must be one the payment can settle).
	// When the sac count is the ONLY announced X it IS the bound; when a
	// mana/energy X also exists the candidate count caps it from above.
	for _, part := range pc.cost.Sac {
		if part.Announced {
			matchSpec := sacrificeMatchSpec(part.Spec)
			avail := int32(0)
			for _, oid := range e.G.Zone(state.ZBattlefield, pc.player) {
				if e.SacrificeBlocked(oid) {
					continue
				}
				if effects.MatchesSpecFrom(e.G, matchSpec, oid, pc.player, pc.card) {
					avail++
				}
			}
			if pc.cost.X == 0 && !energyX && bound > avail {
				bound = avail
			} else if avail < bound {
				bound = avail
			}
		}
	}
	// An X-form tapXType part settles exactly the announced X the same way a
	// Sac<X/Spec> part's count does, so the announcement is bounded by the
	// untapped permanents matching its spec (Necron Overlord's "{X}, tap X
	// untapped artifacts": X beyond the artifact count could be announced but
	// never settled). When the tap part is the ONLY announced X it IS the
	// bound -- but that shape never reaches xAsk at all (the tap election
	// announces it at the tap stage, before xAsk's guard sees no other reason
	// and returns); this cap governs the composed shapes.
	for _, part := range pc.cost.TapPermanent {
		if part.Dyn != "X" {
			continue
		}
		avail := int32(0)
		for _, oid := range e.costCandidates(pc.player, pc.card, state.ZBattlefield, part.Spec, false, true) {
			// The {T} in the same cost claims the source (see tapPermanentCostAsk).
			if pc.cost.Tap && oid == pc.card {
				continue
			}
			avail++
		}
		if pc.cost.X == 0 && !energyX && bound > avail {
			bound = avail
		} else if avail < bound {
			bound = avail
		}
	}
	// An announced SubCounter<X/Kind> part's bound is the number of counters
	// of that kind the source actually has (Chandra, Awakened Inferno's
	// SubCounter<X/LOYALTY>: the loyalty the walker has to remove), and an
	// announced PayLife<X> part's bound is the payer's life total divided
	// across the parts (the payer cannot pay more life than they have;
	// paying exactly all of it is legal -- the SBA owns the zero-life
	// consequence). When an announced part is the ONLY X the cost carries it
	// IS the bound -- the pool-based mana ceiling is meaningless without a
	// mana X -- and when another announced X also exists each cap min-clamps
	// the shared X (CR 601.2b's announcement must be one the payment can
	// settle).
	announcedOnly := pc.cost.X <= 0 && !energyX && !sacX
	boundSet := false
	applyCap := func(cap int32) {
		if announcedOnly && !boundSet {
			bound, boundSet = cap, true
		} else if cap < bound {
			bound = cap
		}
	}
	for _, part := range pc.cost.SubCounter {
		if !part.Announced {
			continue
		}
		have := int32(0)
		if o := e.G.Obj(pc.card); o != nil {
			have = o.Counter(part.Spec)
		}
		applyCap(have)
	}
	if lifeXCount > 0 {
		life := e.G.Players[pc.player].Life
		if life < 0 {
			life = 0
		}
		applyCap(life / int32(lifeXCount))
	}
	var legal []int32
	maxOld := int32(0)
	for x := min; x <= bound; x++ {
		wx := e.paymentManaX(pc, x)
		wx.Generic -= e.delveCredit(pc.player, pc.card, wx.Generic)
		if !e.costPayableGrant(pc.player, pc.card, pc.ability >= 0, wx, pipRider{anyColor: pc.mayPlayIgnore, anyType: pc.mayPlayIgnoreType}) {
			break
		}
		maxOld = x
		// Every announced contribution must actually reduce this X's cost
		// (convokeAbsorbs against the PRE-contribution total manaToPayX --
		// paymentManaX has already applied them): an announcement
		// over-selected for the generic total cannot be made legal by
		// choosing a small X, so that X is not offered and the larger X
		// that absorbs every creature is. The payable check breaks at the
		// first unpayable X -- generic only grows with x, so everything
		// past it is unpayable too -- while a no-op X is skipped without
		// breaking: absorption improves monotonically with x. Without
		// announced contributions the absorb check is vacuously true, so
		// the offer is exactly the old payable range.
		if !e.convokeAbsorbs(pc, e.manaToPayX(pc, x), pc.convoke, false) {
			continue
		}
		legal = append(legal, x)
	}
	vals := legal
	if len(vals) == 0 {
		// Either the whole range was unpayable (a proposal the offer gate
		// would have priced differently, or one made directly -- the CR 733
		// audit does exactly that), or every payable X left an announced
		// contribution a no-op. In both, the OLD offer stands and payCast's
		// own payable check aborts as it always did (CR 733.2), rather
		// than this ask wedging or moving the abort site.
		for x := min; x <= maxOld; x++ {
			vals = append(vals, x)
		}
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Choose a value for X", Source: pc.card}
	for _, x := range vals {
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
			if e.SacrificeBlocked(oid) {
				continue
			}
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
		if part.Announced {
			// Sac<X/Spec>: the announced count (0 = sacrifice nothing -- Dargo's
			// "you MAY sacrifice any number"), already bounded by xAsk to the
			// candidates available then; no priority passes mid-flow, so the
			// board cannot shrink between announcement and this settle.
			n = int(pc.x)
			if n == 0 {
				pc.sacPart++
				continue
			}
		}
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
// ChooseNumber/ChooseColor ability, adds one etbChoice with its pre-built
// option list.
// The list (not just the kind) is captured up front so the offered option and
// the recorded choice always agree, and so the choice is the same whether it
// is asked here (cast flow) or once the object has moved (a land's
// play_land). Nothing is asked and no choice is recorded for an etbCounter
// replacement (its ReplaceWith$ is PutCounter) -- those need only Ctx.X, not
// a player decision.
// collectETBChoices is the cast-path fast path. Entries that do not pass
// through a pending cast (reanimation, blink, or a direct ChangeZone) are
// caught by applyRiotReplacement in replacement.go before their MoveZone is
// logged, so Riot is never limited to spells cast normally.
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
	if f.HasKeyword("Riot") {
		pc.etbs = append(pc.etbs, etbChoice{kind: "riot", options: []decision.Option{
			{Index: 0, Kind: "riot", Label: "Enter with a +1/+1 counter"},
			{Index: 1, Kind: "riot", Label: "Gain haste"},
		}})
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
			kind: kind,
			options: e.etbOptions(you, pc.card, kind,
				r.With.Params["ValidCards"],
				// Type$ (Herald's Horn, Urza's Incubator, Roaming Throne, Three
				// Tree City) names the category the choice ranges over. The
				// option list below builds it; a category this build cannot
				// enumerate is recorded loudly at resolution time by
				// effects.effChooseType, never silently.
				r.With.Params["Type"],
				// Exclude$ (Black Dragon Gate, the five Thriving lands) names
				// colours the choice must NOT offer, comma-separated.
				r.With.Params["Exclude"]),
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
	case "ChooseColor":
		return "color"
	}
	return ""
}

// etbColourLabels pairs the WUBRG letter the Choose event records with the
// option label the client shows, in fixed WUBRG order -- the same order every
// colour choice in this build offers (askManaColor, triggeredManaColourChoice,
// commanderIdentityColours). etbOptions and etbAnswer both read it, so the
// option offered and the letter recorded always agree.
var etbColourLabels = []struct{ letter, name string }{
	{"W", "White"}, {"U", "Blue"}, {"B", "Black"}, {"R", "Red"}, {"G", "Green"},
}

// etbColourLetter maps an option label (or already-a-letter) back to the
// WUBRG letter the event records; "" when the label is neither (an etbAnswer
// caller only sees options etbOptions built, so the guard is defensive).
func etbColourLetter(name string) string {
	for _, cl := range etbColourLabels {
		if strings.EqualFold(name, cl.name) || strings.EqualFold(name, cl.letter) {
			return cl.letter
		}
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
func (e *Engine) etbOptions(you state.PlayerID, card state.ObjID, kind, validCards, typeCategory, exclude string) []decision.Option {
	switch kind {
	case "color":
		// Exclude$ tokens (comma-separated, e.g. "black" on Black Dragon
		// Gate) remove the matching WUBRG label. Fail OPEN: a token
		// etbColourLetter cannot resolve is ignored, never emptied into an
		// ask with zero options (the totality rule in this doc comment).
		excluded := map[string]bool{}
		for _, tok := range strings.Split(exclude, ",") {
			if letter := etbColourLetter(strings.TrimSpace(tok)); letter != "" {
				excluded[letter] = true
			}
		}
		out := make([]decision.Option, 0, len(etbColourLabels))
		for _, cl := range etbColourLabels {
			if excluded[cl.letter] {
				continue
			}
			out = append(out, decision.Option{Index: len(out), Kind: "color", Label: cl.name})
		}
		if len(out) == 0 {
			// Totality guard: an exclusion naming every colour must never
			// empty the ask (corpus carriers exclude exactly one; this is
			// defensive against a future carrier).
			out = make([]decision.Option, 0, len(etbColourLabels))
			for _, cl := range etbColourLabels {
				out = append(out, decision.Option{Index: len(out), Kind: "color", Label: cl.name})
			}
		}
		return out
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
		// The shared creature-type enumeration (creatureTypeOptions); the
		// comment there is the read.
		return e.creatureTypeOptions(you)
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

// creatureTypeOptions enumerates the creature-type option list the cast-time
// "as this enters" ask (etbOptions' "type" arm) and the mid-resolution
// ChooseType ask (Engine.TypeChoices, task ct1) BOTH offer, so the two asks
// and the no-ask fallback can never disagree about what a creature-type
// choice ranges over. The list is the distinct creature subtypes of every
// object you OWN (all zones, object order), sorted alphabetically; the
// "Human" tail keeps the list non-empty when you own no creature subtype,
// the same totality rule the colour list carries.
func (e *Engine) creatureTypeOptions(you state.PlayerID) []decision.Option {
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
}

// TypeChoices implements effects.Host.TypeChoices (task ct1): the option list
// a mid-resolution ChooseType ask offers its chooser — the SAME enumeration
// the cast-time "type" arm builds, so the two lists can never disagree. A
// category this build cannot enumerate yields nil; the asking effect never
// asks for one (it records the loud Note and the deterministic fallback), so
// nil is unreachable through the ask path.
func (e *Engine) TypeChoices(chooser state.PlayerID, category string) []decision.Option {
	if category != "" && !strings.EqualFold(category, "Creature") {
		return nil
	}
	return e.creatureTypeOptions(chooser)
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
	case "color":
		return " a color"
	case "riot":
		return " how this creature enters (counter or haste)"
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
		ab := o.Face().Abilities[pc.ability]
		scope = abilityScope(ab)
		// The ability's own target-dependent ReduceCost$ (Raft Security
		// Officer's AllTargeted$Valid Creature.powerLE3): beginActivation
		// folded pc.ownReduce with nil targets (full price at offer time,
		// fail closed); now the CR 601.2c answer exists, so re-evaluate and
		// net-adjust the generic by the delta. The net form is idempotent --
		// a second pass computes delta 0 -- which matters because
		// repriceForTargets can run again on a mana-window resume.
		if n := e.ownReduceCost(pc.player, pc.card, ab, pc.targets); n != pc.ownReduce {
			pc.cost.Generic = addClampedGeneric(pc.cost.Generic, int64(pc.ownReduce-n))
			pc.ownReduce = n
		}
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
	delve := int32(0)
	if pc.ability < 0 {
		delve = int32(len(pc.delve))
	}
	return e.manaFeasibleGrant(pc.player, pc.card, pc.ability >= 0, pc.resolvedMana(), mods, pc.taxGeneric, delve, pipRider{anyColor: pc.mayPlayIgnore, anyType: pc.mayPlayIgnoreType})
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
	delve := int32(0)
	if pc.ability < 0 {
		delve = int32(len(pc.delve))
	}
	pl := e.G.Players[pc.player]
	out := make([]targetCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		target := state.Target{Obj: candidate.obj}
		if candidate.kind == "player" {
			target = state.Target{Player: candidate.player, IsPlayer: true}
		}
		mods := e.costModifiersForTargets(pc.player, pc.card, scope, []state.Target{target})
		cost := mods.apply(pc.resolvedMana())
		// An announce-bound Exile part (the Shoal cycle's cmcEQX) is priced
		// by the ANNOUNCED X, not by the candidate: nonManaCastable below
		// evaluates a non-literal cmc comparison fail-closed (it has no X
		// binding in scope), so an alt-cost cast whose only payment is such
		// an exile would drop every candidate and reverse the proposal at
		// the target ask. The offer gate (altCostXCandidates' existential
		// over X) and the X ask settled the part's payability independent of
		// any target, so the probe re-runs the exact exAsk binding for it
		// rather than dropping it silently. Parts are collected and filtered
		// AFTER the loop — mutating cost.Exile mid-range would splice wrong
		// indices once a second announce-bound part exists.
		exileSpent := make([]bool, len(cost.Exile))
		for i, part := range cost.Exile {
			if pc.announceX == "" || !strings.Contains(part.Spec, "cmcEQ"+pc.announceX) {
				continue
			}
			zone := part.Zone
			if zone == 0 {
				zone = state.ZHand
			}
			sc := effects.SpecContext{You: pc.player, Source: pc.card, Resolve: func(n string) (int32, bool) {
				if n == pc.announceX {
					return pc.x, true
				}
				return 0, false
			}}
			n := 0
			for _, oid := range e.G.Zone(zone, pc.player) {
				if effects.MatchesSpecCtx(e.G, part.Spec, oid, sc) {
					n++
				}
			}
			if n >= int(part.N) {
				exileSpent[i] = true
			}
		}
		for _, spent := range exileSpent {
			if !spent {
				continue
			}
			kept := make([]CostPart, 0, len(cost.Exile))
			for i, part := range cost.Exile {
				if !exileSpent[i] {
					kept = append(kept, part)
				}
			}
			cost.Exile = kept
			break
		}
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
		if cost.Life > pl.Life {
			continue
		}
		// resolvedMana carries no live pip, so manaFeasible (the shared
		// primitive) here degenerates to the composed payable check — the same
		// composition payCast will charge for this candidate's repricing.
		if e.manaFeasibleGrant(pc.player, pc.card, pc.ability >= 0, pc.resolvedMana(), mods, pc.taxGeneric, delve, pipRider{anyColor: pc.mayPlayIgnore, anyType: pc.mayPlayIgnoreType}) ||
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
	if costAnnouncesPaidX(pc.cost) {
		// The announced sacrifice count re-prices the ReduceCost statics that
		// read the paid X (Dargo's {2}-less-per-sacrifice): the offer-time
		// pc.mods snapshot was bound to X=0.
		if scope, ok := e.pendingCastScope(pc); ok {
			m = e.costModifiersForTargetsX(pc.player, pc.card, scope, pc.targets, pc.x).apply(pc.resolvedMana())
		}
	}
	m.Generic += pc.taxGeneric
	return m
}

// costAnnouncesSacX reports whether the cost carries a Sac<X/Spec> part whose
// count the cast announces (the Dargo shape).
func costAnnouncesSacX(c Cost) bool {
	for _, part := range c.Sac {
		if part.Announced {
			return true
		}
	}
	return false
}

// costAnnouncesPaidX reports whether the cost carries ANY announced-count
// part whose count the cast announces as X: the Sac<X/Spec> shape
// (costAnnouncesSacX), the announced SubCounter<X/Kind> removal and the
// announced PayLife<X> payment. The offer-time costModifiers snapshot was
// bound to X=0, so a static reading the paid X must be re-priced once the
// announcement is known -- the same reason Dargo's Sac<X> needed it.
func costAnnouncesPaidX(c Cost) bool {
	if costAnnouncesSacX(c) {
		return true
	}
	if len(c.LifeX) > 0 {
		return true
	}
	for _, part := range c.SubCounter {
		if part.Announced {
			return true
		}
	}
	// An X-form tapXType part binds the same X (the election announced it or
	// the tap settle paid the announced value), so a ReduceCost static
	// reading Count$xPaid is re-priced on it the same way.
	for _, part := range c.TapPermanent {
		if part.Dyn == "X" {
			return true
		}
	}
	return false
}

// manaToPayX is manaToPay with {X} folded to an explicit value.
// paymentMana applies announced Convoke/Harmonize contributions to the
// already-formed total. A stale answer can never make a requirement negative.
// faceWantsConverge is the heads-safety gate for the pay-time converge
// CastInfo: it reports whether the face carries a Count$Converge SVar body.
// Without it a count>0-only gate would stamp a CastInfo onto EVERY
// multicolour cast and move the chain heads; with it, no game that casts no
// converge card changes an event (measured: no repo-deck card carries
// Count$Converge, so TestHeads stays put). K:Sunburst's keyword expansion
// (its own ledger entry) is the planned second consumer of this seam.
func faceWantsConverge(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for _, v := range f.SVars {
		if body, ok := strings.CutPrefix(v, "Count$"); ok && strings.EqualFold(strings.TrimSpace(body), "Converge") {
			return true
		}
	}
	return false
}

// faceWantsCastSpend is the heads-safety gate for the pay-time cast-spend
// CastInfo (the converge gate's shape): it reports whether the face's SVar
// table reads the TOTAL mana actually spent to cast the spell -- a body
// naming the Count$CastTotalManaSpent head (Freestrider Commando's
// SVar:X:Count$CastTotalManaSpent feeding its etbCounter CheckSVar$ gate).
// The ref-property readers of OTHER casts (TriggeredCard$
// CastTotalManaSpent and its family) do not read this object field and do
// not gate the emission -- they stay on the rv2b exotic-heads ledger.
func faceWantsCastSpend(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for _, body := range f.SVars {
		if strings.Contains(body, "Count$CastTotalManaSpent") {
			return true
		}
	}
	return false
}

// faceWantsTimesKicked is the heads-safety gate for the pay-time multikick
// CastInfo on a PLAIN-Kicker cast mode (the converge gate's shape): it
// reports whether the face's SVar table reads the times-kicked count
// anywhere (Count$TimesKicked, op suffix included). The 11 legacy
// plain-Kicker carriers (Stronghold Arena, Urborg Lhurgoyf, ...) carry the
// SVar and get a real count stamped; a kicker card without the SVar (Into
// the Roil, Wastescape Battlemage) casts byte-identically to before. A
// multikicked-mode cast needs no gate -- its count>0 emission is the
// primitive itself.
func faceWantsTimesKicked(f *cards.Face) bool {
	if f == nil {
		return false
	}
	for _, v := range f.SVars {
		if strings.Contains(v, "Count$TimesKicked") {
			return true
		}
	}
	return false
}

// triggeredConvergeReaderOut is the capture gate's second arm: it reports
// whether any alive player's battlefield holds a permanent whose face SVars
// name TriggeredCard$Converge -- a trigger that reads ANOTHER spell's cast
// colours (Magmablood Archaic's "for each color of mana spent to cast that
// spell"), which the Count$Converge face gate cannot see because the cast
// face itself is an ordinary non-converge instant/sorcery. Only then does the
// pay-time CastInfo need stamping on a plain cast; a TriggerZones$ Battlefield
// SpellCast trigger can only exist for casts made while the reader is out, so
// this scan-at-pay-time gate stamps exactly when the value can be needed and
// no game without a reader out changes an event (heads stay put: neither
// Archaic is in any repo deck). Pure read -- the boolean OR over the
// deterministic seat/zone walk cannot reach an event; replay re-runs payCast
// and derives the same scan.
func (e *Engine) triggeredConvergeReaderOut() bool {
	g := e.G
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			o := g.Obj(id)
			if o == nil {
				continue
			}
			f := o.Face()
			if f == nil {
				continue
			}
			for _, v := range f.SVars {
				if strings.Contains(strings.ToLower(v), "triggeredcard$converge") {
					return true
				}
			}
		}
	}
	return false
}

// convergeColours is CR 107.4f-family's converge count: the number of
// DISTINCT colours among W,U,B,R,G actually spent to cast the spell.
// Colourless/generic ({C}, generic pips) is not a colour and does not count;
// snow mana spent as a colour lives in the colour buckets here, so the plain
// per-colour delta is already right; mana conversion and the may-play
// ignore-colour rider changed what was actually paid, which is exactly what
// converge asks about.
func convergeColours(spent state.Mana) int32 {
	n := int32(0)
	for i := state.MW; i <= state.MG; i++ {
		if spent[i] > 0 {
			n++
		}
	}
	return n
}

// manaSpentTotal is the total mana a cast's payment actually spent: the
// spent delta's pips summed over every slot (coloured and colourless).
// Contribute the FULL delta -- a generic pip spent from a coloured unit is
// one mana spent -- so the sum is CR 601.2h's "mana spent to cast it".
func manaSpentTotal(spent state.Mana) int32 {
	var n int32
	for i := range spent {
		n += spent[i]
	}
	return n
}

func (e *Engine) paymentMana(pc *pendingCast) Cost {
	return e.applyConvoke(pc, e.manaToPay(pc))
}

// paymentManaX applies the same announced creature contributions after X is
// folded into the total. xAsk uses it so an X value funded by Convoke or
// Harmonize is actually offered, not rejected before the payment is known.
func (e *Engine) paymentManaX(pc *pendingCast, x int32) Cost {
	return e.applyConvoke(pc, e.manaToPayX(pc, x))
}

func (e *Engine) applyConvoke(pc *pendingCast, m Cost) Cost {
	for _, pay := range pc.convoke {
		if pay.color != 0 {
			i := state.ManaIndex(pay.color)
			if m.Colored[i] > 0 {
				m.Colored[i]--
			}
			continue
		}
		if pay.power > 0 {
			m.Generic -= pay.power
		} else {
			m.Generic--
		}
		if m.Generic < 0 {
			m.Generic = 0
		}
	}
	return m
}

// convokeAbsorbs reports whether every announced Convoke/Harmonize payment
// actually reduces the outstanding mana requirement m when applied in
// announcement order. A colour contribution whose pip is already covered,
// or a generic/power contribution against a generic total already at {0},
// pays nothing -- CR 601.2b/702.51a let a creature be tapped only for a
// reduction the total cost still needs -- and payCast taps every announced
// creature, so an announcement containing such a no-op is an illegal
// over-payment that must be rejected, not silently tapped.
//
// With an unfixed {X} the generic requirement is not yet known when the
// announcement is answered (CR 601.2b announces Convoke before X), so a
// generic/power contribution against {0} generic is tentatively allowed
// there (xOpen) and xAsk prices the announcement against every candidate X
// with xOpen false, m already X-folded.
func (e *Engine) convokeAbsorbs(pc *pendingCast, m Cost, pays []convokePayment, xOpen bool) bool {
	for _, pay := range pays {
		if pay.color != 0 {
			i := state.ManaIndex(pay.color)
			if m.Colored[i] <= 0 {
				return false
			}
			m.Colored[i]--
			continue
		}
		if m.Generic <= 0 {
			if xOpen {
				continue
			}
			return false
		}
		reduce := pay.power
		if reduce <= 0 {
			reduce = 1
		}
		m.Generic -= reduce
		if m.Generic < 0 {
			m.Generic = 0
		}
	}
	return true
}

// validateSearch is the Submit-time gate for a hidden-library search
// decision (ResumeKind "search") whose SA carries ShareLandType$ True
// (Myriad Landscape's "up to two basic land cards that share a land type").
// The decision's static Validate sees only the offered option list -- no
// option carries the shared-type constraint -- so an answer naming two lands
// of disjoint types would pass it; this gate rejects such an answer before
// the intent is recorded and the pending decision is consumed, exactly like
// validateAttackers/validateCastContributions. Single-card answers are
// trivially legal (one card always shares with itself). Any other search --
// no ResumeSA, no ShareLandType$ -- is passed through untouched. The effect
// side (applyLibrarySearch's trim) keeps the same constraint for a host
// that bypassed the wire, through the one shared classifier
// effects.SharedLandTypes.
func (e *Engine) validateSearch(d *decision.Decision, in decision.Intent) error {
	if d.ResumeKind != "search" || d.ResumeSA == nil ||
		!strings.EqualFold(strings.TrimSpace(d.ResumeSA.Params["ShareLandType"]), "True") {
		return nil
	}
	ids := make([]state.ObjID, 0, len(in.Choices))
	for _, c := range in.Choices {
		if c < 0 || c >= len(d.Options) {
			continue // Validate's own out-of-range error already fired
		}
		if o := d.Options[c]; o.Obj != 0 {
			ids = append(ids, o.Obj)
		}
	}
	if !effects.SharedLandTypes(e.G, ids) {
		return fmt.Errorf("chosen cards do not share a land type")
	}
	return nil
}

// validateSameControllerTargets is the Submit-time gate for a cast-flow
// target announcement whose SA carries TargetsWithSameController$ True
// (Lodestone Bauble): every chosen object must share one owner — in a
// graveyard, the owner the card there has. Any other KTarget decision, a
// single-object answer, and an out-of-range choice (Validate's own error)
// pass through untouched.
func (e *Engine) validateSameControllerTargets(d *decision.Decision, in decision.Intent) error {
	pc := e.cast
	if pc == nil || !pc.sameCtrlTargets || d.Kind != decision.KTarget || len(in.Choices) <= 1 {
		return nil
	}
	var owner state.PlayerID
	haveOwner := false
	for _, c := range in.Choices {
		if c < 0 || c >= len(d.Options) {
			continue // Validate's own out-of-range error already fired
		}
		o := d.Options[c]
		if o.Obj == 0 {
			continue
		}
		obj := e.G.Obj(o.Obj)
		if obj == nil {
			return nil // the resolution-time recheck owns a vanished object
		}
		if !haveOwner {
			owner, haveOwner = obj.Owner, true
			continue
		}
		if obj.Owner != owner {
			return fmt.Errorf("chosen targets do not share one controller")
		}
	}
	return nil
}

// validateCastContributions is the Submit-time gate for the cast flow's
// Convoke/Harmonize announcement decision (convokeAsk). The decision's
// static Validate sees only the offered option list -- two white creatures
// each carry a convoke_W option for a {W} spell, in distinct groups -- so
// an over-selection passes it; this gate rejects any answer containing a
// contribution that reduces nothing (convokeAbsorbs), before the intent is
// recorded and the pending decision is consumed, exactly like
// validateAttackers. The client then resubmits a legal subset. Any other
// KChoose decision, and an out-of-range choice (Validate's own error), is
// passed through untouched.
func (e *Engine) validateCastContributions(d *decision.Decision, in decision.Intent) error {
	pc := e.cast
	if pc == nil || len(in.Choices) == 0 {
		return nil
	}
	var pays []convokePayment
	for _, c := range in.Choices {
		if c < 0 || c >= len(d.Options) {
			return nil
		}
		o := d.Options[c]
		switch {
		case o.Kind == "harmonize":
			pays = append(pays, convokePayment{id: o.Obj, power: int32(o.Amount)})
		case strings.HasPrefix(o.Kind, "convoke_"):
			color := byte(0)
			if o.Kind != "convoke_generic" {
				color = o.Kind[len("convoke_")]
			}
			pays = append(pays, convokePayment{id: o.Obj, color: color})
		default:
			return nil // a different cast-flow ask, not the convoke announcement
		}
	}
	all := append(append([]convokePayment(nil), pc.convoke...), pays...)
	if !e.convokeAbsorbs(pc, e.manaToPay(pc), all, pc.cost.X > 0) {
		return fmt.Errorf("announcement reduces nothing: the outstanding cost cannot absorb every chosen contribution")
	}
	return nil
}

// convokeAsk announces every creature used for Convoke or Harmonize. It is
// deliberately before manaWindowAsk: tapping is part of paying, so a chosen
// creature cannot first be used as a mana source.
func (e *Engine) convokeAsk() bool {
	pc := e.cast
	if pc == nil || pc.convokeDone || pc.ability >= 0 {
		return false
	}
	pc.convokeDone = true
	isConvoke := e.hasCastConvoke(pc.card)
	isHarmonize := pc.mode == "harmonize"
	if !isConvoke && !isHarmonize {
		return false
	}
	mana := e.manaToPay(pc)
	// Before X is announced, its generic requirement is not folded into
	// mana. It nevertheless makes every creature a possible generic payment;
	// the subsequent xAsk prices the selected contributions against the real
	// X total.
	hasX := pc.cost.X > 0
	if !mana.hasManaPayment() && !hasX {
		return false
	}
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 0,
		Prompt: "Choose creatures to help pay for " + e.G.Obj(pc.card).Face().Name, Source: pc.card}
	for _, id := range e.G.Zone(state.ZBattlefield, pc.player) {
		o := e.G.Obj(id)
		if o == nil || o.Tapped || o.Face() == nil || !o.EffectiveIsCreature() || o.BestowedAttached() {
			continue
		}
		group := fmt.Sprintf("payment:%d", id)
		if isHarmonize && (mana.Generic > 0 || hasX) {
			// The reduction offered is the creature's ACTUAL power (CR
			// 702.46a), the same number harmonizePayment credits: a printed
			// 1/1 currently boosted to 4 funds four generic, and a printed
			// 4/4 reduced to 1 funds only one.
			if p := e.Derived(id).Power; p > 0 {
				d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "harmonize", Obj: id,
					Group: group, Amount: int(p), Label: "Tap " + o.Face().Name + " (reduce by " + strconv.Itoa(int(p)) + ")"})
			}
		}
		if !isConvoke {
			continue
		}
		for _, color := range []byte{'W', 'U', 'B', 'R', 'G'} {
			if mana.Colored[state.ManaIndex(color)] > 0 && strings.Contains(e.objColors(o), string(color)) {
				d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "convoke_" + string(color), Obj: id,
					Group: group, Label: "Tap " + o.Face().Name + " for " + string(color)})
			}
		}
		if mana.Generic > 0 || hasX {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "convoke_generic", Obj: id,
				Group: group, Label: "Tap " + o.Face().Name + " for 1"})
		}
	}
	if len(d.Options) == 0 {
		return false
	}
	// The announcement cannot tap more creatures than the cost can absorb:
	// each chosen contribution reduces exactly one outstanding slot (a
	// colour pip or one generic), so Max is the outstanding slot count.
	// With an unfixed {X} the generic requirement is not yet known (CR
	// 601.2b announces Convoke before X), so the bound is left open and
	// xAsk prices the announcement against every candidate X instead;
	// convokeAbsorbs's answer gate plus that pricing close the rest.
	d.Max = len(d.Options)
	if !hasX {
		if slots := int(mana.Colored.Total() + mana.Generic); slots < d.Max {
			d.Max = slots
		}
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
}

func (e *Engine) manaToPayX(pc *pendingCast, x int32) Cost {
	m := pc.mods.apply(pc.resolvedManaX(x))
	if costAnnouncesPaidX(pc.cost) {
		// The announced sacrifice count re-prices the ReduceCost statics that
		// read the paid X (Dargo's {2}-less-per-sacrifice): the offer-time
		// pc.mods snapshot was bound to X=0.
		if scope, ok := e.pendingCastScope(pc); ok {
			m = e.costModifiersForTargetsX(pc.player, pc.card, scope, pc.targets, x).apply(pc.resolvedManaX(x))
		}
	}
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
// choices. It is how feasibleAny's per-level walk consumes one announcement
// pip at a time after folding that pip's resolved face into the cost, so a
// leaf never sees a pip slot twice.
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

// announceCost is gone: its per-pip composition (mods applied to a cost that
// still carried the unannounced pips, so a Color$ reduction saw no W pip it
// could legally take and a floor priced an unresolved pip at its generic
// face) answered a different feasibility question than the offer gate and
// the charge, and could reject the only legal announcement (a reduction that
// is legally assigned to a LATER pip). announceFeasible below folds the
// already-announced pips into the cost as their final resolved faces and
// hands the remainder to the one shared primitive, costMods.feasibleAny.
// announceFeasible reports whether offering alternative alt for the pip the
// flow is announcing still leaves the whole cost payable: the pips already
// committed (payColor/payLife/payGeneric on pc) and the candidate alt are
// folded into the cost as their FINAL resolved faces, and the still-
// unannounced pips are enumerated by the shared primitive with the CR 601.2f
// modifiers composed onto each fully-resolved assignment, the CR 903.8
// commander tax added after and (for a spell) the Delve credit taken off the
// generic — exactly the composition manaToPay/payCast will charge once every
// pip is settled. It is the CR 601.2b legality question: an announced payment
// is offered only if SOME legal assignment of the remaining pips makes the
// total cost payable, so a player is never offered a payment that can only
// strand the cast in an unpayable remainder (and an abort at payCast).
func (e *Engine) announceFeasible(pc *pendingCast, alt pipAlt, pool, snow state.Mana, life int32) bool {
	c := pc.cost.WithX(pc.x)
	for i := range c.Colored {
		c.Colored[i] += pc.payColor[i]
	}
	c.Generic = addClampedGeneric(c.Generic, int64(pc.payGeneric))
	c.Life = addClampedGeneric(c.Life, int64(pc.payLife))
	switch {
	case alt.color != 0:
		c.Colored[state.ManaIndex(alt.color)]++
	case alt.generic > 0:
		c.Generic = addClampedGeneric(c.Generic, int64(alt.generic))
	case alt.life > 0:
		c.Life = addClampedGeneric(c.Life, int64(alt.life))
	}
	delve := int32(0)
	if pc.ability < 0 {
		delve = int32(len(pc.delve))
	}
	// The pips 0..payIdx have been announced (their faces are folded in
	// above), so their slots leave the cost; the pips after payIdx stay live
	// for the shared primitive to enumerate.
	c = c.dropAnnouncePrefix(pc.payIdx + 1)
	return e.manaFeasibleGrant(pc.player, pc.card, pc.ability >= 0, c, pc.mods, pc.taxGeneric, delve, pipRider{anyColor: pc.mayPlayIgnore, anyType: pc.mayPlayIgnoreType})
}

// manaAsk offers the player's payment choice for the next unsettled hybrid or
// Phyrexian pip of the cost (CR 601.2b), one decision per pip. EVERY face is
// gated on the ONE shared feasibility primitive (announceFeasible →
// costMods.feasibleAny): with the pips already announced and the candidate
// folded in as its final resolved face, some legal assignment of the
// remaining pips must make the composed total (modifiers on final faces,
// commander tax, Delve credit) payable. Any composed modifier can make a
// locally affordable face strand the final payment — a generic Thalia raise
// (Dismember's black faces), a Color$ reduction that is only legally
// assignable to a later pip ({W/U}{W/U} under Color$ W), a SetCost floor on
// an unresolved twobrid — and only the whole-cost search sees that, so a
// player is never offered a payment a complete assignment cannot pay. The
// valid options keep their announcePip order, so the deterministic bot
// fallback (index 0) always picks a legal payment and a no-answer host never
// wedges. The offer gate (offerCastable) proved at least one full assignment
// feasible over the same primitive, so the decision is never empty for a
// state the gate measured; an empty menu is still possible after the offer
// (the {X} choice or a repricing changed the composition) and the defensive
// arm below preserves the flow's behaviour for it. It returns true once it
// has asked (and therefore suspended); payCast applies the accumulated
// payColor / payLife / payGeneric when every pip is settled.
func (e *Engine) manaAsk() bool {
	pc := e.cast
	if pc == nil || pc.payIdx >= pc.cost.annPipCount() {
		return false
	}
	alts := pc.cost.announcePip(pc.payIdx)
	// announceFeasible receives the full pool and life total because the
	// commitments already made (and this candidate face) are folded into the
	// cost it evaluates; nothing has been paid yet. Do not pre-filter a colour
	// face merely because the current pool lacks that colour: a Color$
	// reduction can make the announced face free (for example {W/U} under
	// Color$ W).
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
	seen := map[byte]bool{}
	seenGeneric := false
	for _, alt := range alts {
		switch {
		case alt.color != 0:
			if seen[alt.color] {
				continue
			}
			seen[alt.color] = true
			if e.announceFeasible(pc, alt, pool, snow, fullLife) {
				addPip(alt)
			}
		case alt.generic > 0:
			if seenGeneric {
				continue
			}
			seenGeneric = true
			if e.announceFeasible(pc, alt, pool, snow, fullLife) {
				addPip(alt)
			}
		case alt.life > 0:
			if e.announceFeasible(pc, alt, pool, snow, fullLife) {
				addPip(alt)
			}
		}
	}
	if len(d.Options) == 0 {
		// Defensive: the offer gate proved at least one pip alternative
		// completes the cost, so a feasible option is always present for a
		// gated cast measured at the gate; this arm only guards the state
		// having shifted since (the {X} choice, a repricing, a shorter pool).
		// Rather than offer an infeasible payment, offer the first alternative
		// (index 0, the deterministic best) so the decision is never empty.
		addPip(alts[0])
	}
	e.choosing = chooseCast
	e.ask(d)
	return true
}

// etbAnswer records one answered "as this enters" choice onto the card as a
// Choose event, before the object is put on the stack (or, for a land, before
// it moves to the battlefield), so the recorded value survives replay exactly
// as the player chose it. The value rides on Option.Label (name/type/colour)
// or Option.Amount (number), not the choice index.
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
			pc.etbName, pc.etbType, pc.etbNumber, pc.etbColor = o.ChosenName, o.ChosenType, o.ChosenNumber, o.ChosenColor
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
	case "color":
		if letter := etbColourLetter(opt.Label); letter != "" {
			e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "color", Text: letter})
		}
	case "riot":
		choice := "haste"
		if opt.Index == 0 {
			choice = "counter"
		}
		e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "riot", Text: choice})
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
	case "replicate":
		// CR 702.55a: the answered payment count folds that many replicate
		// payments into cost, so every later stage (Convoke, X, the payment
		// window, payCast) charges the composed total. The count rides on
		// Option.Amount, not Index, for the same reason xAsk's value does.
		if len(chosen) > 0 && pc.replicateSet {
			n := int32(chosen[0].Amount)
			rc := ParseCost(pc.replicateParam)
			for i := int32(0); i < n; i++ {
				pc.cost = pc.cost.Plus(rc)
			}
			pc.replicateTimes = n
		}
	case "multikick":
		// CR 702.43: the answered payment count folds that many multikicker
		// payments into cost -- the replicate arm's exact shape.
		if len(chosen) > 0 && pc.multikickSet {
			n := int32(chosen[0].Amount)
			mk := ParseCost(pc.multikickParam)
			for i := int32(0); i < n; i++ {
				pc.cost = pc.cost.Plus(mk)
			}
			pc.multikickTimes = n
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
	case "altaddcost":
		// The either-or additional cost (AlternateAdditionalCost): the chosen
		// part's cost folds into pc.cost (Plus), so the ordinary stages settle
		// it and payCast charges it. The chosen part rides on Option.Amount
		// (the index into pc.altAddParts), not the option Index, for the same
		// reason xAsk's value does.
		if len(chosen) > 0 && chosen[0].Amount >= 0 && int(chosen[0].Amount) < len(pc.altAddParts) {
			pc.cost = pc.cost.Plus(ParseCost(pc.altAddParts[chosen[0].Amount]))
		}
	case "exilecost":
		for _, o := range chosen {
			pc.exiles = append(pc.exiles, o.Obj)
		}
		pc.exilePart++
	case "revealcost":
		for _, o := range chosen {
			pc.reveals = append(pc.reveals, o.Obj)
		}
		pc.revealPart++
	case "beholdcost":
		for _, o := range chosen {
			pc.beholds = append(pc.beholds, o.Obj)
		}
		pc.beholdPart++
	case "tapcost":
		part := CostPart{}
		if pc.tapPart < len(pc.cost.TapPermanent) {
			part = pc.cost.TapPermanent[pc.tapPart]
		}
		for _, o := range chosen {
			pc.taps = append(pc.taps, o.Obj)
		}
		pc.tapPart++
		// A dynamic X-form part whose tap election ANNOUNCED the count (no other
		// announce-bearing part ran xAsk first -- see tapPermanentCostAsk): the
		// chosen count is the cast's {X} (CR 601.2b), which the pay-time
		// CastInfo then carries to resolution and Count$xPaid reads. A part
		// whose cost pre-announced the X (pc.xDone) settles exactly that value
		// and must not overwrite it.
		if part.Dyn == "X" && !pc.xDone {
			pc.x = int32(len(chosen))
		}
	case "blightcost":
		for _, o := range chosen {
			pc.blights = append(pc.blights, o.Obj)
		}
		pc.blightPart++
	case "returncost":
		for _, o := range chosen {
			pc.returns = append(pc.returns, o.Obj)
		}
		pc.returnPart++
	case "puttolibcost":
		for _, o := range chosen {
			pc.putToLibs = append(pc.putToLibs, o.Obj)
		}
		pc.putToLibPart++
	case "forage_exile":
		pc.cost.Exile = append(pc.cost.Exile, CostPart{N: 3, Spec: "Card", Zone: state.ZGraveyard})
	case "forage_food":
		if len(chosen) > 0 {
			pc.sacs = append(pc.sacs, chosen[0].Obj)
		}
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
	case "convoke_W", "convoke_U", "convoke_B", "convoke_R", "convoke_G", "convoke_generic":
		for _, choice := range chosen {
			color := byte(0)
			if choice.Kind != "convoke_generic" {
				color = choice.Kind[len("convoke_")]
			}
			pc.convoke = append(pc.convoke, convokePayment{id: choice.Obj, color: color})
		}
	case "harmonize":
		for _, choice := range chosen {
			pc.convoke = append(pc.convoke, convokePayment{id: choice.Obj, power: int32(choice.Amount)})
		}
	case "activate":
		// CR 601.2g: a source's mana abilities are distinct activations that
		// share its tap cost. activateManaPayment resolves a singleton
		// immediately or asks the caster to choose one before re-entering
		// this payment window -- the payment-window form, so an
		// InstantSpeed$ True mana ability (Lion's Eye Diamond, "Activate
		// only as an instant") is withheld: paying a cost is no priority
		// moment.
		if len(chosen) > 0 {
			e.activateManaPayment(pc.player, chosen[0].Obj, true)
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
	// The and/or Kicker's per-part modes: each index flag rides with the
	// bare FlagKicked (every part paid IS a kicked cast -- the bare
	// predicate and the Condition$ Kicked gate keep matching), so the
	// CastInfo wire carries the part's identity and the generic read.
	case "kicked1":
		return events.FlagsString(state.FlagKicked | state.FlagKicked1)
	case "kicked2":
		return events.FlagsString(state.FlagKicked | state.FlagKicked2)
	case "kickedboth":
		return events.FlagsString(state.FlagKicked | state.FlagKicked1 | state.FlagKicked2)
	case "surged":
		return events.FlagsString(state.FlagSurged)
	case "flashback":
		return events.FlagsString(state.FlagFlashback)
	case "miracle":
		return events.FlagsString(state.FlagMiracle)
	// The alternative-cost keyword family: the flag is what the ETB machinery
	// (evoke's sacrifice trigger, dash's haste + delayed return, warp's
	// delayed exile) and the warp recast offer read.
	case "escape":
		return events.FlagsString(state.FlagEscaped)
	case "evoked":
		return events.FlagsString(state.FlagEvoked)
	case "dashed":
		return events.FlagsString(state.FlagDashed)
	case "overloaded":
		return events.FlagsString(state.FlagOverloaded)
	case "warped":
		return events.FlagsString(state.FlagWarped)
	// The Adventure spell face's cast (CR 714.3a): the flag is what the
	// resolution reader (spellRestZone) uses to exile the spell into the
	// adventure zone instead of the graveyard. adventure_recast deliberately
	// has NO case here -- casting the main face from the adventure zone is an
	// ordinary cast, exactly like warp_recast.
	case "adventure_alt":
		return events.FlagsString(state.FlagAdventure)
	case "buyback":
		return events.FlagsString(state.FlagBuyback)
	case "mayplay":
		return events.FlagsString(state.FlagMayPlay)
	case "harmonize":
		return events.FlagsString(state.FlagHarmonize)
	case "suspend":
		return events.FlagsString(state.FlagSuspend)
	// Foretell's later cast (CR 702.126a): the flag is the provenance an ETB
	// reader (Lupine Harbingers' CheckSVar$ WasForetold) and Count$Foretold
	// read off the permanent the spell becomes -- the stack->battlefield
	// persistence the Suspend flag rides too. The {2} ACTION's CastInfo is
	// emitted directly by payCast's foretell branch (which then returns, so
	// the ordinary flags path below is never reached for that mode); the
	// action's flag has no modeFlags case for the same reason suspend's
	// branch does not share this switch.
	case "foretell_cast":
		return events.FlagsString(state.FlagForetold)
	// Bestow (CR 702.114a): the flag is the provenance the resolution
	// reader (resolveTop) uses to substitute the synthesized Aura attach
	// spell, and what keeps a bestowed cast distinguishable on the wire.
	case "bestowed":
		return events.FlagsString(state.FlagBestowed)
	// Multikicker (CR 702.43): the mode marks the INTENT to pay the
	// optional multikicker cost, and the count ask (multikickAsk) can still
	// answer 0 -- a DECLINED multikick must stay the byte-identical plain
	// cast, no flag and no event, exactly the "replicated" contract above.
	// When a payment WAS made, payCast ORs bare FlagKicked (a multikicked
	// cast IS a kicked cast) and FlagMultikicked onto the trailing CastInfo.
	case "multikicked":
		return ""
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
		if sa == nil && pc.mode == "bestowed" {
			// Bestow (CR 702.114a): the bestowed cast targets through the
			// synthesized Aura attach SA -- the creature face has no SP of its
			// own, so the plain cast's no-SP shape says nothing about the
			// bestowed one.
			sa = bestowedAttachSA()
		}
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
	mana := e.paymentMana(pc)
	if pc.ability < 0 {
		mana.Generic -= int32(len(pc.delve))
		if mana.Generic < 0 {
			mana.Generic = 0
		}
	}
	// resolvedMana carries no live pip at this stage (the pip announcements
	// are already settled, manaAsk runs before targetAsk), so the composed
	// payable check here is the same composition manaToPay charges;
	// paymentMana additionally folds the announced Convoke/Harmonize
	// contributions in (zero when none were announced). costPayable is the
	// conversion-aware equivalent: the SAME resolveMana payManaConvFor will
	// run, including RestrictValid$ provenance. The
	// targetDependentCostMayPay arm keeps the ValidTarget$ reducer exception.
	if !e.costPayableGrant(pc.player, pc.card, pc.ability >= 0, mana, pipRider{anyColor: pc.mayPlayIgnore, anyType: pc.mayPlayIgnoreType}) &&
		!e.hasUntappedManaSource(pc.player) && !e.targetDependentCostMayPay(pc) {
		e.abortCast(pc, "cast aborted: cost no longer payable", true)
		return true
	}
	min, max := e.resolvedTargetBounds(pc.player, pc.card, sa, pc.x)
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
	// Forge's TargetsForEachPlayer$ selection shape (one per player): the
	// same bounds/group read the trigger-path askTarget uses, so a OneEach
	// CAST ask (Unexplained Absence's "up to one target nonland permanent
	// each player controls") offers the whole table's slots and the wire's
	// mutual-exclusion rule enforces one pick per controller. Before this the
	// cast-time ask ignored the shape and capped the ask at the plain Max.
	min, max, _ = e.oneEachTargetBounds(sa, candidates, min, max)
	// Overload changes the word "target" to "each". It makes no selection at
	// announcement time: the current matching set is derived at resolution,
	// so permanents entering or changing controller in response are handled.
	// No target decision/event is emitted and zero objects is legal.
	if pc.mode == "overloaded" {
		return false
	}
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
		//
		// suppress=true engages the F05-2 (CR 733.2) no-progress discipline
		// like every other abort site: the FIRST identical abort of this card
		// in the window leaves the option offered (a player may still make a
		// play that creates a legal target -- cast a creature, then Shelter),
		// the SECOND holds it out of the window. Without it a seat whose
		// policy keeps re-picking the same castable-but-targetless spell
		// livelocks inside one priority window forever (measured: the bot
		// bench replayed "cast Shelter -> abort" 20000 times, engine note
		// "cast aborted: no legal target", zero state change). Any genuine
		// state change clears the count and the held-out set, so a target
		// created later re-offers the cast normally.
		e.abortCast(pc, "cast aborted: no legal target", true)
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
		o := decision.Option{Index: len(d.Options), Kind: candidate.kind,
			Label: label, Obj: candidate.obj, Player: candidate.player}
		o.Group = e.oneEachTargetGroup(sa, candidate)
		d.Options = append(d.Options, o)
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
	if pc == nil || pc.mode == "land" || pc.mode == "suspend" || pc.ability >= 0 {
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
	// CR 601.2a: the player who cast the spell is its controller. A card
	// another seat controlled (Rashmi and Ragavan's exiled OPPONENT card,
	// Gonti's stolen card, Intellect Devourer's may-play exile) comes under
	// the caster's control the moment it is cast, and the resulting permanent
	// enters the battlefield under the caster's control; an ordinary cast's
	// card already answers to the caster, so no event rides those.
	if o := e.G.Obj(pc.card); o != nil && o.Controller != pc.player {
		e.emit(events.Event{Kind: events.ControlChange, Obj: pc.card, Player: pc.player})
	}
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
	for _, sv := range e.castRestrictionSources(e.activeStatics("CantBeCast"), pc.card) {
		if !e.actorMatches(sv, "Caster", pc.player) {
			continue
		}
		if !e.restrictionGateHolds(sv, pc.card) || !e.checkSVarHolds(sv) {
			continue
		}
		sc := e.specCtx(sv.Source, sv.Controller)
		sc.HasManaValue = true
		sc.ManaValue = mv
		if effects.MatchesSpecCtx(e.G, sv.Params["ValidCard"], pc.card, sc) {
			// suppress=true, not false: an illegal-proposal abort is a
			// no-progress reversal (CR 733.1) exactly like every other abort
			// site, so it rides the same F05-2 (CR 733.2) discipline -- first
			// identical abort retryable, second holds the option out of the
			// window. With false, a seat that re-picks the same X (the only
			// value it knows) re-announces the same illegal spell forever.
			e.abortCast(pc, "cast aborted: proposed spell is illegal (CR 601.2e)", true)
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
	mana := e.paymentMana(pc)
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
	if e.costPayableGrant(pc.player, pc.card, pc.ability >= 0, mana, pipRider{anyColor: pc.mayPlayIgnore, anyType: pc.mayPlayIgnoreType}) {
		return false
	}
	var sources []state.ObjID
	for _, z := range []state.Zone{state.ZBattlefield, state.ZHand, state.ZGraveyard} {
		for _, id := range e.G.Zone(z, pc.player) {
			if !e.convokeCommitted(pc, id) && e.untappedManaSource(pc.player, id) {
				sources = append(sources, id)
			}
		}
	}
	if len(sources) == 0 {
		return false
	}
	name := e.G.Obj(pc.card).Face().Name
	d := &decision.Decision{Player: pc.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: "Activate mana abilities to pay for " + name, Source: pc.card}
	for _, id := range sources {
		opt := decision.Option{Index: len(d.Options), Kind: "activate",
			Obj: id, Label: "Activate " + e.G.Obj(id).Face().Name + " for mana"}
		// The same beyond-tap cost marker legal.go's priority-window offer
		// carries, so the one "activate" option shape stays consistent across
		// both ask sites (fb-led1); this ask sits on a choose decision, which
		// every auto path refuses, so the marker changes no classification.
		if marker := manaActivationCostMarker(e.availableManaAbilities(pc.player, id)); marker != "" {
			opt.Cost = marker
		}
		d.Options = append(d.Options, opt)
	}
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "done", Label: "Done"})
	e.choosing = chooseCast
	e.ask(d)
	return true
}

func (e *Engine) convokeCommitted(pc *pendingCast, id state.ObjID) bool {
	for _, pay := range pc.convoke {
		if pay.id == id {
			return true
		}
	}
	return false
}

// untappedManaSource reports whether id is an untapped permanent under
// the player p's control with at least one unrestricted mana ability. It
// is the PAYMENT-WINDOW gate only -- every call site (the CR 601.2g
// cast window, a ward payment, a cumulative-upkeep payment) runs while
// the payer holds no priority -- so an InstantSpeed$ True mana ability
// (Lion's Eye Diamond, "Activate only as an instant") does not make its
// source eligible: the ability is activatable at priority, never inside
// a payment window (CR 605.4 defers to the ability's own timing
// restriction).
func (e *Engine) untappedManaSource(p state.PlayerID, id state.ObjID) bool {
	for _, ma := range e.availableManaAbilities(p, id) {
		if e.instantSpeedOnly(ma) {
			continue
		}
		return true
	}
	return false
}

// hasUntappedManaSource reports whether p controls ANY untapped permanent
// with a usable mana ability -- the condition under which the 601.2g window
// could supply the mana a pool alone cannot.
func (e *Engine) hasUntappedManaSource(p state.PlayerID) bool {
	for _, z := range []state.Zone{state.ZBattlefield, state.ZHand, state.ZGraveyard} {
		for _, id := range e.G.Zone(z, p) {
			if e.untappedManaSource(p, id) {
				return true
			}
		}
	}
	return false
}

func (e *Engine) emitChoiceCosts(pc *pendingCast) {
	names := func(ids []state.ObjID) string {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			out = append(out, e.targetName(id))
		}
		return strings.Join(out, ", ")
	}
	if len(pc.reveals) > 0 {
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Obj: pc.card,
			IDs: append([]state.ObjID(nil), pc.reveals...), Text: "revealed " + names(pc.reveals) + " as a cost"})
	}
	if len(pc.beholds) > 0 {
		e.emit(events.Event{Kind: events.Note, Player: pc.player, Obj: pc.card,
			IDs: append([]state.ObjID(nil), pc.beholds...), Text: "beheld " + names(pc.beholds) + " as a cost"})
	}
	for _, id := range pc.taps {
		e.emit(events.Event{Kind: events.Tap, Obj: id, Text: "tapped as a cost"})
	}
	for i, id := range pc.blights {
		if i < len(pc.cost.Blight) {
			e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "M1M1",
				Amount: pc.cost.Blight[i].N})
		}
	}
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
		sourceControllerLKI := e.G.Obj(pc.card).Controller
		// Task 10: an activated ability. The shared stages above (X, Delve --
		// never present on an ability --, Sac) have already run and been
		// recorded; what differs from a spell here is the cost's remaining
		// non-mana parts. Pay mana, then each Tap (a Tap event), each
		// SubCounter part (a CounterChange of -N), and every chosen sacrifice.
		// The ability object was already minted by pushCast; targets are
		// recorded onto it by handleTarget.
		mana := e.manaToPay(pc)
		ok, _, spentMana := e.payManaForSpent(pc.player, pc.card, true, mana, e.paymentConv(pc.player, pc.card, true), pipRider{})
		if !ok {
			e.abortCast(pc, "activation aborted: cost no longer payable", true)
			return
		}
		// RememberCostMana$ (Jeweled Amulet: "Note the type of mana spent to
		// pay this activation cost"): the colours the payment actually spent
		// (the same per-colour delta the negative ManaAdd events above
		// record, in WUBRG order) fold onto the source object through the
		// "noted-mana" Choose marker, where the card's mana ability (Produced$
		// Special LastNotedType) reads them back. A cost with no mana part
		// notes nothing — Forge's CostRememberSpentMana records only mana
		// costs too.
		remembered := false
		if f := e.G.Obj(pc.card).Face(); f != nil && pc.ability >= 0 && pc.ability < len(f.Abilities) {
			ab := f.Abilities[pc.ability]
			remembered = strings.EqualFold(strings.TrimSpace(ab.Params["RememberCostMana"]), "True")
		}
		if remembered {
			noted := ""
			for i, letter := range manaLetters {
				if spentMana[i] > 0 {
					noted += letter
				}
			}
			e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "noted-mana", Text: noted})
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
		// Exile cost parts (ExileFromHand/ExileFromGrave): each chosen card
		// leaves its zone (hand, or the graveyard for a self-reference) for
		// exile. Read the zone live: the settled card is still where exAsk
		// found it, but a From read from the object keeps a graveyard
		// self-exile honest about where it moved from.
		for _, id := range pc.exiles {
			if o := e.G.Obj(id); o != nil {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZExile, Text: "exiled as a cost"})
			}
		}
		// Energy cost parts (PayEnergy<N>/<X>): the announced amount leaves
		// the payer's energy pool as one PlayerCounterChange (a player
		// counter, not an object's -- CR 118.2d). The X form spends exactly
		// the announced value (xAsk bounded it by this same total).
		for _, part := range pc.cost.Energy {
			amt := part.N
			if part.Spec == "X" {
				amt = pc.x
			}
			if amt > 0 {
				e.emit(events.Event{Kind: events.PlayerCounterChange, Player: pc.player,
					Counter: "ENERGY", Amount: -amt})
			}
		}
		// Announced PayLife<X> parts (Toxic Deluge's "pay X life"): each pays
		// the announced X as one LifeChange beside the fixed life payMana
		// charged above (payLife). xAsk bounded the announcement by the payer's
		// life, so the payment cannot drive the total below zero here.
		for range pc.cost.LifeX {
			if pc.x > 0 {
				e.emit(events.Event{Kind: events.LifeChange, Player: pc.player, Amount: -pc.x})
			}
		}
		// DamageYou<N> cost parts: the payer takes N damage from the source
		// (Forge CostDamage; the same event shape payUnlessDamageCost emits).
		for _, part := range pc.cost.DamageYou {
			e.payDamageCost(pc.player, part.N, pc.card)
		}
		// Draw cost parts (Draw<N/Spec>): the payer draws N, as one ordinary
		// Draw event per card (an empty library's loss is the SBA's). The
		// dredge replacement is NOT posed here -- the cast-flow payment stage
		// cannot re-enter mid-payment -- and no corpus card reaches a Draw
		// cost payment with a dredger in the graveyard.
		e.payDrawCostParts(pc)
		// Return cost parts: each chosen object moves to its OWNER's hand
		// (Forge CostReturn.doPayment's moveToHand) beside the other payments.
		for _, id := range pc.returns {
			if o := e.G.Obj(id); o != nil {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZHand, Text: "returned to hand as a cost"})
			}
		}
		e.emitChoiceCosts(pc)
		if pc.cost.Tap {
			// The {T} cost's payer taps the permanent (Forge CostTap). This
			// MUST come before settlePutToLibCost: a self-placement cost that
			// also carries {T} (Timestream Navigator's
			// "{2}{U}{U}, {T}, Put Timestream Navigator on the bottom of its
			// owner's library") moves the source off the battlefield, and
			// tapping a library card is not a state that exists (CR 110.5) --
			// a library tap would also survive a direct library→battlefield
			// re-entry, whose Move entry arm does not clear Tapped. Emitted
			// in this order the tap lands on the still-battlefield permanent
			// and the move's leave-battlefield arm resets it.
			e.emitTap(pc.card, pc.player, false)
		}
		// Settled after the {T} tap for the same reason (see above).
		e.settlePutToLibCost(pc)
		for _, part := range pc.cost.SubCounter {
			amt := part.N
			if part.Announced {
				amt = pc.x
			}
			if amt != 0 {
				e.emit(events.Event{Kind: events.CounterChange, Obj: pc.card, Counter: part.Spec, Amount: -amt})
			}
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
			if e.sourceControllerLKI == nil {
				e.sourceControllerLKI = make(map[state.ObjID]state.PlayerID)
			}
			e.sourceLifelinkLKI[pc.stackObj] = sourceLifelinkLKI
			e.sourceControllerLKI[pc.stackObj] = sourceControllerLKI
			break
		}
		e.cast, e.choosing = nil, chooseNone
		return
	}
	mana := e.paymentMana(pc)
	mana.Generic -= int32(len(pc.delve))
	if mana.Generic < 0 {
		mana.Generic = 0
	}
	paid, spentMana := e.payManaCastSpent(pc, mana)
	if !paid {
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
	if f := e.G.Obj(pc.card).Face(); faceWantsConverge(f) || e.triggeredConvergeReaderOut() {
		pc.convergeOn = true
		pc.converge = convergeColours(spentMana)
	}
	if f := e.G.Obj(pc.card).Face(); faceWantsCastSpend(f) {
		pc.manaSpentOn = true
		pc.manaSpent = manaSpentTotal(spentMana)
	}
	if pc.payLife != 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: pc.player, Amount: -pc.payLife})
	}
	for _, pay := range pc.convoke {
		e.emit(events.Event{Kind: events.Tap, Obj: pay.id})
	}
	for _, id := range pc.delve {
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZExile, Text: "delved"})
	}
	for _, id := range pc.discards {
		e.emit(events.DiscardCost(id))
	}
	// Exile cost parts (see the ability branch above for the why).
	for _, id := range pc.exiles {
		if o := e.G.Obj(id); o != nil {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZExile, Text: "exiled as a cost"})
		}
	}
	// Energy cost parts (see the ability branch above for the why).
	for _, part := range pc.cost.Energy {
		amt := part.N
		if part.Spec == "X" {
			amt = pc.x
		}
		if amt > 0 {
			e.emit(events.Event{Kind: events.PlayerCounterChange, Player: pc.player,
				Counter: "ENERGY", Amount: -amt})
		}
	}
	// Announced PayLife<X>, DamageYou<N> and Draw<N/Spec> cost parts (see the
	// ability branch above for the why).
	for range pc.cost.LifeX {
		if pc.x > 0 {
			e.emit(events.Event{Kind: events.LifeChange, Player: pc.player, Amount: -pc.x})
		}
	}
	for _, part := range pc.cost.DamageYou {
		e.payDamageCost(pc.player, part.N, pc.card)
	}
	e.payDrawCostParts(pc)
	// Return cost parts (see the ability branch above for the why).
	for _, id := range pc.returns {
		if o := e.G.Obj(id); o != nil {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZHand, Text: "returned to hand as a cost"})
		}
	}
	e.settlePutToLibCost(pc)
	e.emitChoiceCosts(pc)
	// Capture the sacrifice LKI before the MoveZones (see the ability branch's
	// comment): the sacrificed permanents are still on the battlefield here.
	var sacrificedLKI []state.SacrificedInfo
	for _, id := range pc.sacs {
		sacrificedLKI = append(sacrificedLKI, state.SacrificedInfoOf(e.G, id))
	}
	for _, id := range pc.sacs {
		e.emit(events.Sacrifice(id))
	}
	if pc.mode == "suspend" {
		info, _ := suspendCost(e.G.Obj(pc.card).Face())
		time := info.time
		if info.timeX {
			time = pc.x
		}
		// CastInfo is the replayable provenance marker: only this action sets
		// FlagSuspend, so an arbitrary exiled Suspend card is never treated as
		// having been suspended. Its Amount retains X while the card is exiled:
		// counter-removal triggers on X-time Suspend cards read xPaid there.
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: pc.x, Counter: events.FlagsString(state.FlagSuspend)})
		e.emit(events.Event{Kind: events.MoveZone, Obj: pc.card, From: pc.from, To: state.ZExile, Text: "suspended"})
		if time > 0 {
			e.emit(events.Event{Kind: events.CounterChange, Obj: pc.card, Counter: "TIME", Amount: time})
		}
		e.cast, e.choosing = nil, chooseNone
		return
	}
	if pc.mode == "foretell" {
		// CR 702.126a: the Foretell ACTION is not a cast. CastInfo is the
		// replayable provenance marker -- only this action sets FlagForetold
		// on a hand->exile move, so an arbitrary exiled card is never treated
		// as foretold -- and the MoveZone carries the face-down exile
		// encoding (events.Apply's decode sets FaceDown and clears ExiledWith:
		// no exiling source permanent exists for Foretell, so Amount 0). Do
		// NOT route through effects' applyExileFaceDown: it is unexported and
		// binds an ability source that does not exist here -- the raw event
		// encoding is emitted directly, the suspend branch's own pattern. The
		// view redacts the face-down exile to everyone but the exiler (the
		// owner, for Foretell), and any move NOT to exile clears FaceDown, so
		// the later foretell-cost cast reveals automatically.
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Counter: events.FlagsString(state.FlagForetold)})
		e.emit(events.Event{Kind: events.MoveZone, Obj: pc.card, From: pc.from, To: state.ZExile,
			Counter: "exiled_with_face_down", Amount: 0})
		e.cast, e.choosing = nil, chooseNone
		return
	}
	if e.sacrificedLKI == nil {
		e.sacrificedLKI = make(map[state.ObjID][]state.SacrificedInfo)
	}
	e.sacrificedLKI[pc.stackObj] = sacrificedLKI
	// AddsNoCounter$ mana (Cavern of Souls): if the payment just consumed a
	// batch carrying the can't-be-countered provenance FOR THIS CAST, fold
	// state.FlagNoCounter into the same pay-time CastInfo so a replay marks
	// the spell exactly like every other cast flag. The capture is read once
	// here and cleared — nothing can suspend between emitRestrictedManaSpend's
	// set and this read (it emits, never asks).
	noCounter := e.noCounterSpend == pc.stackObj
	e.noCounterSpend = 0
	// CR 601.2b: record how the spell was cast (the X value and mode flags).
	// Deferred to payment rather than the up-front push so an aborted
	// proposal leaves no cast-time trace on the card. A cast trigger that
	// reads the mode (e.g. "cast a kicked spell") sees it, because the flag
	// is applied before the trigger fires next.
	flags := modeFlags(pc.mode)
	if noCounter {
		flags = events.FlagsString(events.FlagsFrom(flags) | state.FlagNoCounter)
	}
	// Replicate (CR 702.55a): the payment count rides the same pay-time
	// CastInfo. modeFlags deliberately maps "replicated" to "" -- a DECLINED
	// replicate (count 0) must stay the byte-identical plain cast, no flag
	// and no event -- so the flag is ORed here only when a payment was made.
	// The count and a paid {X} never share one Amount: measured at the corpus
	// pin, no K:Replicate carrier's mana value carries {X}, so the
	// single-event shape below is the live path; the defensive two-event
	// split keeps the two provenances distinct should one ever pair.
	repCount := int32(0)
	if pc.mode == "replicated" {
		repCount = pc.replicateTimes
	}
	if repCount > 0 {
		flags = events.FlagsString(events.FlagsFrom(flags) | state.FlagReplicated)
	}
	if repCount > 0 && pc.x != 0 {
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: pc.x,
			Counter: events.FlagsString(events.FlagsFrom(flags) &^ state.FlagReplicated)})
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: repCount, Counter: flags})
	} else if pc.x != 0 || flags != "" {
		amt := pc.x
		if repCount > 0 {
			amt = repCount
		}
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: amt, Counter: flags})
	}
	// Converge (CR 107.4f-family, task converge1): the distinct-colour spend
	// count rides its own TRAILING pay-time CastInfo -- the flag routes the
	// Amount into Object.ConvergeColours (events.Apply's CastInfo case), so
	// this event never clobbers the X or replicate count an earlier event in
	// this block set, and its Counter (flags + FlagConverged) leaves
	// CastFlags carrying every earlier flag too. Emitted whenever the face
	// carries a Count$Converge SVar -- or when a battlefield permanent's
	// trigger names TriggeredCard$Converge and so reads THIS cast's colours --
	// count 0 included (a colourless-only converge cast is a real zero, not an
	// absent one); the two-arm gate is heads-safety, so no game that casts no
	// converge card with no reader out changes an event.
	if pc.convergeOn {
		flags = events.FlagsString(events.FlagsFrom(flags) | state.FlagConverged)
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: pc.converge, Counter: flags})
	}
	// Multikicker (CR 702.43, task multikicker1): the times-kicked count
	// rides its own TRAILING pay-time CastInfo -- the flag routes the Amount
	// into Object.TimesKicked (events.Apply's CastInfo case), so this event
	// never clobbers the X a main CastInfo carried (Comet Storm pairs {X}
	// with Multikicker; the two-event split falls out of the trailing shape
	// itself), and its Counter leaves CastFlags carrying every earlier flag
	// too. A multikicked cast IS a kicked cast, so the flag rides with the
	// bare FlagKicked (the bare predicate and the Condition$ Kicked gate
	// keep matching). The emission gate keeps unrelated casts
	// byte-identical: ALWAYS on a multikicked-mode cast with count > 0 (a
	// declined kick -- count 0, the modeFlags("replicated") contract --
	// emits nothing), and on a plain-Kicker cast mode ONLY when the face
	// carries a Count$TimesKicked SVar (faceWantsTimesKicked): the 11 legacy
	// plain-Kicker carriers' scripts still read the count, and no other
	// kicked cast gains an event.
	mkCount := int32(0)
	switch pc.mode {
	case "multikicked":
		mkCount = pc.multikickTimes
	case "kicked", "kicked1", "kicked2":
		mkCount = 1
	case "kickedboth":
		mkCount = 2
	}
	if mkCount > 0 && (pc.mode == "multikicked" || faceWantsTimesKicked(e.G.Obj(pc.card).Face())) {
		mkFlags := events.FlagsString(events.FlagsFrom(flags) | state.FlagKicked | state.FlagMultikicked)
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: mkCount, Counter: mkFlags})
	}
	// Cast-spend (task castprov1): the TOTAL mana actually spent to cast the
	// spell rides its own TRAILING pay-time CastInfo -- the flag routes the
	// Amount into Object.ManaSpent (events.Apply's CastInfo case), so this
	// event never clobbers the X an earlier event in this block carried, and
	// its Counter leaves CastFlags carrying every earlier flag too. A
	// convoke-only cast (tapped creatures, no mana) is a real zero, not an
	// absent one -- the same "count 0 included" contract the converge
	// emission keeps. The emission gate keeps unrelated casts byte-identical:
	// only a face whose SVar table reads the count (faceWantsCastSpend)
	// stamps the event.
	if pc.manaSpentOn {
		flags = events.FlagsString(events.FlagsFrom(flags) | state.FlagManaSpent)
		e.emit(events.Event{Kind: events.CastInfo, Obj: pc.card, Amount: pc.manaSpent, Counter: flags})
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
	if pc.faceBefore != nil {
		if o := e.G.Obj(pc.card); o != nil && o.FaceIdx != *pc.faceBefore {
			e.emit(events.Event{Kind: events.FlipFace, Obj: pc.card, Amount: int32(*pc.faceBefore)})
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
			if o.ChosenColor != pc.etbColor {
				e.emit(events.Event{Kind: events.Choose, Obj: pc.card, Counter: "color", Text: pc.etbColor})
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
	e.checkTriggers(*ev, lki, 0, 0, false)
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
	effects.RegisterNonAPI("kw:Kicker", "kw:Surge", "kw:Flashback", "kw:Delve",
		// The alternative-cost keyword family (altcosts): each is implemented
		// to its CR shape with a named proof test in altcast_test.go --
		// kw:Evoke (alternative cast + ETB unconditional sacrifice), kw:Dash
		// (alternative cast + haste + delayed return), kw:Overload
		// (alternative cast + target-reads-each), kw:Warp (alternative cast
		// from hand/graveyard/exile + delayed exile), kw:Madness (exile on
		// discard + immediate cast offer), kw:Encore (graveyard activation
		// minting attacking token copies), and kw:AlternateAdditionalCost
		// (the mandatory either-or additional cost choice).
		"kw:Evoke", "kw:Dash", "kw:Overload", "kw:Warp", "kw:Madness",
		"kw:Encore", "kw:AlternateAdditionalCost",
		"kw:Buyback", "kw:Transmute", "kw:Suspend", "kw:Convoke", "kw:Harmonize", "kw:Cycling",
		// kw:Level up: CR 702.87, expanded by cards/keywords.go into an
		// ordinary sorcery-speed PutCounter activation (CounterType$ LEVEL);
		// the level-band statics read the counter through the existing
		// counters_<CMP><n>_LEVEL predicate, so no separate path of its own.
		"kw:Level up",
		// kw:Replicate: CR 702.55, expanded by cards/keywords.go into the
		// Storm-shaped copy trigger whose Amount$ Count$ReplicatePaid reads
		// the pay-time CastInfo's count; the cast flow's replicateAsk poses
		// the CR 601.2b count announcement.
		"kw:Replicate",
		// kw:Multikicker: CR 702.43, the count-ask cast shape -- the cast flow
		// (rules/legal.go's multikicked offer, rules/cast.go's multikickAsk)
		// poses the CR 601.2b count announcement and the trailing
		// FlagMultikicked CastInfo carries the count into Object.TimesKicked
		// for the Count$TimesKicked head (effects/count.go). No keyword
		// expansion: the K:Multikicker line is read directly.
		"kw:Multikicker",
		// kw:Affinity: CR 702.41, expanded by cards/keywords.go into the
		// ordinary ReduceCost cost-static machinery (rules/statics.go's
		// collectCostStatics) -- no separate cast path of its own.
		"kw:Affinity",
		// kw:Embalm / kw:Eternalize: CR 702.128 / 702.129, expanded by
		// cards/keywords.go into one graveyard-zone CopyPermanent activation
		// whose cost exiles the card itself (ExileFromGrave<1/CARDNAME>) and
		// whose token copy carries the keyword's modified characteristics --
		// the Encore graveyard-activation shape with a different effect.
		"kw:Embalm", "kw:Eternalize")
}
