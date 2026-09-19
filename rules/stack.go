package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// manaLetters is state.Mana's index order (MW, MU, MB, MR, MG, MC) spelled
// out as the WUBRGC symbols events.ManaAdd's Counter field expects.
var manaLetters = [...]string{"W", "U", "B", "R", "G", "C"}

// payMana spends cost from p's pool and reports whether it could. Every
// state mutation goes through events, so payment cannot be a direct field
// write to Players[p].Pool: it emits one ManaAdd event per colour bucket
// actually spent, with a negative Amount. That reuses the existing ManaAdd
// kind rather than adding a new one -- Apply's ManaAdd case is a plain "+=",
// so a negative Amount already subtracts correctly, the same trick Ruling F4
// uses for clearing damage.
//
// Ruling T19b-b: this used to be silent on failure (a no-op the caller could
// not observe), on the theory that legalActions always gates "cast" on
// CanPay first so failure here was unreachable. That theory held for the
// base cost but not for an alternative one: legalActions gates an alt-cost
// option on the ALTERNATIVE cost being payable, so a caller that (as
// castSpell used to) paid the base cost regardless of which option was
// chosen could genuinely hit this path with real, well-formed card data --
// and a silent no-op there is what let the spell go on the stack anyway,
// having paid nothing. Reporting failure explicitly is what lets castSpell
// abort the cast instead.
func (e *Engine) payMana(p state.PlayerID, cost Cost) bool {
	return e.payManaConvFor(p, 0, false, cost, nil)
}

// payManaConv is payMana under a stat:ManaConvert conversion set (or nil,
// the plain exact-colour payment payMana always was). The conversion widens
// (and the <-C restriction narrows) what the pool's mana may pay, never what
// the cost demands.
func (e *Engine) payManaConv(p state.PlayerID, cost Cost, conv *manaConv) bool {
	return e.payManaConvFor(p, 0, false, cost, conv)
}

// payManaConvFor pays a specific spell or activated ability. RestrictValid$
// mana remains distinct from ordinary floating mana until this point: it is
// included only when its restriction admits this payment, then spent first
// and marked on the negative ManaAdd event so events.Apply can reconstruct
// the same provenance during replay.
func (e *Engine) payManaConvFor(p state.PlayerID, id state.ObjID, ability bool, cost Cost, conv *manaConv) bool {
	return e.payManaFor(p, id, ability, cost, conv, pipRider{})
}

// payManaFor is payManaConvFor with the may-play ignore-colour rider passed
// explicitly, so the payment sites that know the cast's recorded rider (a
// pendingCast's mayPlayIgnore, kept from the offer gate that proved it) keep
// the grant after the card has moved to the stack -- at payment time the
// card is no longer in the granted zone, so re-deriving from the zone would
// wrongly drop it.
func (e *Engine) payManaFor(p state.PlayerID, id state.ObjID, ability bool, cost Cost, conv *manaConv, rider pipRider) bool {
	ok, _ := e.payManaForSpent(p, id, ability, cost, conv, rider)
	return ok
}

// payManaForSpent is payManaFor with the payment's actually-spent mana
// returned: the per-colour delta the negative ManaAdd events record (zero on
// a failed payment). The RememberCostMana$ payment site (Jeweled Amulet) is
// the caller that needs it — every existing caller keeps the bool-only
// wrapper, so no other payment site changes shape.
func (e *Engine) payManaForSpent(p state.PlayerID, id state.ObjID, ability bool, cost Cost, conv *manaConv, rider pipRider) (bool, state.Mana) {
	before := e.manaAvailableFor(p, id, ability)
	beforeSnow := e.G.Players[p].Snow
	pay, ok := cost.resolveManaWith(before, beforeSnow, e.G.Players[p].Life,
		e.payerGrantsPayLifeInsteadOfB(p), rider, conv)
	if !ok {
		return false, state.Mana{}
	}
	after, afterSnow, lifeSpent := pay.pool, pay.snow, pay.lifeSpent
	spent := state.Mana{}
	for i := range before {
		spent[i] = before[i] - after[i]
	}
	e.emitRestrictedManaSpend(p, id, ability, &spent)
	for i, letter := range manaLetters {
		if spent[i] == 0 {
			continue
		}
		// A slot whose snow units were spent (all or part) emits the
		// "S<colour>" Counter form so the parallel snow tally moves with the
		// pool through the same events the adds used. resolveMana consumes a
		// non-snow unit before a snow one wherever a choice existed, so the
		// snow split here is exactly what the payment search did.
		snowSpent := beforeSnow[i] - afterSnow[i]
		if snowSpent > 0 {
			if plain := spent[i] - snowSpent; plain > 0 {
				e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: letter, Amount: -plain})
			}
			e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: "S" + letter, Amount: -snowSpent})
			continue
		}
		e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: letter, Amount: -spent[i]})
	}
	// Fixed life costs and any Phyrexian pips paid with life are deducted
	// through the ordinary LifeChange event so a replay learns them.
	if lifeSpent != 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: p, Amount: -lifeSpent})
	}
	return true, spent
}

// payExtortPip charges the {W/B} hybrid pip (one mana of either W or B)
// from p's pool, emitting the ManaAdd events so a replay re-derives it. It
// returns false (and charges nothing) when the pool has neither colour, so
// an Extort payment a player genuinely cannot make is a decline rather than
// a free drain.
func (e *Engine) payExtortPip(p state.PlayerID) bool {
	pool := e.G.Players[p].Pool
	for _, idx := range []int{state.MW, state.MB} {
		if pool[idx] > 0 {
			e.emit(events.Event{Kind: events.ManaAdd, Player: p,
				Counter: manaLetters[idx], Amount: -1})
			return true
		}
	}
	return false
}

// payManaCast is the spell-cost payment: the shared payManaFor core with the
// cast's recorded may-play ignore-colour rider (CR 401.5's "spend mana as
// though it were mana of any color to cast it"). The rider was proved by the
// offer gate while the card still sat in the granted zone; the payment keeps
// it via pc.mayPlayIgnore because after the push (CR 601.2a) the card is on
// the stack and a zone re-derivation would wrongly drop the grant.
func (e *Engine) payManaCast(pc *pendingCast, cost Cost) bool {
	return e.payManaFor(pc.player, pc.card, false, cost, e.paymentConv(pc.player, pc.card, false),
		pipRider{anyColor: pc.mayPlayIgnore, anyType: pc.mayPlayIgnoreType})
}

// manaAvailableFor removes every restricted batch from the visible pool, then
// restores exactly the batches valid for this payment. This means a cast or a
// nonmatching activation can never borrow Tazri-style mana merely because it
// shares a colour bucket with unrestricted mana.
func (e *Engine) manaAvailableFor(p state.PlayerID, id state.ObjID, ability bool) state.Mana {
	available := e.G.Players[p].Pool
	for _, r := range e.G.Players[p].RestrictedMana {
		idx := state.ManaIndex(r.Color[0])
		available[idx] -= r.Amount
		// An empty Valid is an UNRESTRICTED batch that carries only its
		// AddsNoCounter$ provenance (Boseiju's plain {C}): it pays anything,
		// exactly like ordinary pool mana.
		if r.Valid == "" || e.restrictValidMatches(p, id, ability, r.Valid, r.Source) {
			available[idx] += r.Amount
		}
	}
	return available
}

// emitRestrictedManaSpend consumes matching restriction batches in insertion
// order before ordinary mana. Every matching unit is interchangeable for the
// current payment; using this fixed order keeps the log deterministic. When a
// consumed batch carries AddsNoCounter$ provenance and this is a SPELL cast
// payment (never an ability activation), the cast's id is captured in
// e.noCounterSpend for payCast to fold state.FlagNoCounter into the pay-time
// CastInfo — with the batch's own condition evaluated against the paying
// spell's face (Boseiju's !Permanent).
func (e *Engine) emitRestrictedManaSpend(p state.PlayerID, id state.ObjID, ability bool, spent *state.Mana) {
	e.noCounterSpend = 0
	// Emit mutates RestrictedMana through events.Apply, so range a snapshot:
	// otherwise removing the first of two matching batches would make the
	// live slice shift under this loop and could skip or double-spend one.
	batches := append([]state.ManaRestriction(nil), e.G.Players[p].RestrictedMana...)
	for _, r := range batches {
		if r.Amount <= 0 || (r.Valid != "" && !e.restrictValidMatches(p, id, ability, r.Valid, r.Source)) {
			continue
		}
		idx := state.ManaIndex(r.Color[0])
		used := spent[idx]
		if used > r.Amount {
			used = r.Amount
		}
		if used == 0 {
			continue
		}
		if r.NoCounter != "" && !ability && e.noCounterSpend == 0 && addsNoCounterHolds(e.G, id, r.NoCounter) {
			e.noCounterSpend = id
		}
		e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: r.Color, Amount: -used,
			Text: events.ManaRestrictionText(r.Valid, r.Source)})
		spent[idx] -= used
	}
}

// addsNoCounterHolds evaluates a consumed batch's AddsNoCounter$ condition
// against the spell being paid for: "True" (or the empty default) always
// holds; "NotPermanent" (Forge's AddsNoCounter$ !Permanent, Boseiju's
// instant-or-sorcery mana) holds when the paying spell is not a permanent
// spell. An unrecognised condition fails closed — no protection.
func addsNoCounterHolds(g *state.Game, id state.ObjID, cond string) bool {
	switch cond {
	case "", "True":
		return true
	case "NotPermanent":
		o := g.Obj(id)
		return o != nil && o.Face() != nil && !o.Face().IsPermanent()
	default:
		return false
	}
}

// restrictValidMatches evaluates RestrictValid$'s payment class. Forge spells
// each term as <SA-kind>.<object filter> and a Valid$ value may name several
// comma-separated terms with OR semantics (Eldrazi Temple's
// "Spell.Eldrazi+Colorless,Activated.Eldrazi+Colorless+inZoneBattlefield",
// Master of Dark Rites' "Spell.Demon,Spell.Cleric,Spell.Vampire"). The zone
// predicate is checked here because it describes the ability's source, not
// the mana source that made the restriction. Unknown classes fail closed so
// restricted mana is never spent illegally. src is the producing permanent's
// id when the batch's event recorded one, so source-relative filter
// predicates (Cavern of Souls' ChosenType) resolve against the mana source;
// source-less batches keep the historical paid-card reading.
func (e *Engine) restrictValidMatches(p state.PlayerID, id state.ObjID, ability bool, valid string, src state.ObjID) bool {
	for _, term := range strings.Split(strings.TrimSpace(valid), ",") {
		if e.restrictValidTermMatches(p, id, ability, strings.TrimSpace(term), src) {
			return true
		}
	}
	return false
}

func (e *Engine) restrictValidTermMatches(p state.PlayerID, id state.ObjID, ability bool, term string, src state.ObjID) bool {
	kind, spec, ok := strings.Cut(term, ".")
	if !ok {
		return false
	}
	switch kind {
	case "Activated":
		if !ability {
			return false
		}
	case "Spell":
		if ability {
			return false
		}
	default:
		return false
	}
	needsBattlefield := strings.Contains(spec, "inZoneBattlefield")
	spec = strings.Trim(strings.ReplaceAll(spec, "+inZoneBattlefield", ""), "+")
	o := e.G.Obj(id)
	if o == nil || (needsBattlefield && o.Zone != state.ZBattlefield) {
		return false
	}
	if spec == "" {
		return true
	}
	srcID := id
	if src != 0 {
		srcID = src
	}
	if effects.MatchesSpecFrom(e.G, spec, id, p, srcID) {
		return true
	}
	// Forge's object-filter grammar defaults the base to Card, so a bare
	// predicate-only spec -- "Spell.Colorless" (Shrine of the Forsaken
	// Gods), "Spell.MultiColor", the compound "Spell.Eldrazi+Colorless"
	// (Eldrazi Temple) -- means Card.<spec>: a bare type word ("Creature",
	// "Artifact") already evaluates as its own base, but a word the matcher
	// only knows as a QUALIFIER fails closed with no base. Retry with the
	// explicit base before denying the batch: the retry can only turn a
	// "never spendable" batch into the correct evaluation, never widen a
	// spec that already evaluated (the first attempt ran unchanged).
	return effects.MatchesSpecFrom(e.G, "Card."+spec, id, p, srcID)
}

// paymentConv is the conversion set for p paying id (ability selects the
// ValidSA$ Spell/Activated scoping), or nil when no ManaConvert static would
// change any pip match. Returning nil -- not a zero conv -- keeps the pure
// resolveMana path (and every game without a converter on the board)
// byte-identical.
func (e *Engine) paymentConv(p state.PlayerID, id state.ObjID, ability bool) *manaConv {
	conv := e.manaConversion(p, id, ability)
	if conv.empty() {
		return nil
	}
	return &conv
}

// costPayableGrant is costPayable with the may-play ignore-colour rider
// passed explicitly, for the payment sites that know the cast's recorded
// rider and cannot re-derive it from the card's zone.
func (e *Engine) costPayableGrant(p state.PlayerID, id state.ObjID, ability bool, cost Cost, rider pipRider) bool {
	_, ok := cost.resolveManaWith(e.manaAvailableFor(p, id, ability), e.G.Players[p].Snow, e.G.Players[p].Life,
		e.payerGrantsPayLifeInsteadOfB(p), rider, e.paymentConv(p, id, ability))
	return ok
}

// costPayable is the conversion-aware equivalent of Cost.payable at the
// offering and window gates: the SAME resolveMana payMana will run, so an
// offered cost and the cost actually charged can never disagree about what
// the payer's converted mana may satisfy. The payer-side grants (a
// PayLifeInsteadOf:B static under its controller; a may-play grant's
// MayPlayIgnoreColor$ rider, derived from the card's current zone) are
// applied here too, so an offered cost and the charged cost agree about a
// K'rrik-shaped or may-play-shaped payment as well.
func (e *Engine) costPayable(p state.PlayerID, id state.ObjID, ability bool, cost Cost) bool {
	return e.costPayableGrant(p, id, ability, cost,
		pipRider{anyColor: e.payerGrantsIgnoreColor(p, id), anyType: e.payerGrantsIgnoreType(p, id)})
}

// costPayablePool is costPayable priced against an EXPLICIT pool instead of
// the seat's restriction-adjusted floating one: pool is the mana the cost
// must resolve against, whatever the seat is actually holding right now. The
// ordinary gates (costPayable here, manaFeasible in statics.go) are exactly
// this with the real manaAvailableFor pool and never call it directly with
// the RAW pool -- offering a cast on mana its RestrictValid$ provenance would
// refuse at payment is the illegal direction (rv2c review: an earlier shape
// of castable did, and was reverted). The only caller is the potential-action
// walk (rules/legal.go legalActionsPriced via castablePriced), which passes
// the hypothetical bound the seat would hold after floating every untapped
// source. The payer grants and conversion shaping are the same reads in both
// modes, so a potential action and the payment it promises can never disagree
// about what the pool may satisfy.
func (e *Engine) costPayablePool(p state.PlayerID, id state.ObjID, ability bool, cost Cost, pool state.Mana) bool {
	_, ok := cost.resolveManaWith(pool, e.G.Players[p].Snow, e.G.Players[p].Life,
		e.payerGrantsPayLifeInsteadOfB(p),
		pipRider{anyColor: e.payerGrantsIgnoreColor(p, id), anyType: e.payerGrantsIgnoreType(p, id)},
		e.paymentConv(p, id, ability))
	return ok
}

// targetBounds resolves a targeting subject's TargetMin$/TargetMax$ to the
// decision's Min/Max. Missing bounds default to 1 (the M1 single-target
// contract: a spell or ability that targets at all targets one thing).
// TargetMin$ 0 is legal -- requirement N2: a spell that MAY target zero
// things resolves untargeted when no legal target exists -- while a negative
// Min or a Max below Min is clamped back to a sane bound (never below 1, so
// a truncated 0 max still asks for at least one). The rule is clamp, not
// ignore: TargetMax$ is parsed whatever it reads (TargetMax$ 0 and even a
// negative TargetMax$ are both taken seriously) and then clamped so
// max >= 1 and max >= min. A discarded parameter and a clamped one both
// resolve to the same number when used alone, but they differ the moment
// TargetMin$ is also present -- this says which one the engine means.
func targetBounds(sa *cards.SA) (int, int) {
	min, max := 1, 1
	if v, ok := sa.Params["TargetMin"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			min = n
		}
	}
	if v, ok := sa.Params["TargetMax"]; ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			max = n
		}
	}
	if min < 0 {
		min = 1
	}
	if max < 1 {
		max = 1
	}
	if max < min {
		max = min
	}
	return min, max
}

// targetMin is the Min half of targetBounds, inlined for resolveTop's N2 gate.
func targetMin(sa *cards.SA) int {
	min, _ := targetBounds(sa)
	return min
}

// targetZones resolves TgtZone$ (comma-separated) and TargetType$ into the
// zones to search for target options. TgtZone$ is the explicit zone
// declaration; TargetType$ (Forge) names the KIND of thing targeted, and
// when it names a stack object -- Spell, Instant, Sorcery, Activated,
// Triggered, SpellAbility -- the target lives on the stack. Mana Leak is
// scripted `TargetType$ Spell | ValidTgts$ Card` with NO TgtZone$ at all, so
// without reading TargetType$ the search defaulted to the battlefield and
// offered every permanent of every seat for a counterspell -- the bug this
// fixes (every counterspell in the corpus was inert, targeting a permanent
// so effCounter found o.Zone != state.ZStack and did nothing). The default
// remains the battlefield. An unknown TgtZone$ token is dropped (reviewer
// minor 4), but the battlefield default applies only when NEITHER source
// named a zone -- a typo'd TgtZone$ on a TargetType$ Spell card must not
// silently widen a stack target back to the battlefield.
//
// fb-20260916T024739Z-b89aea46: the remaining default is wrong for one more
// shape -- the Wrenn and Six ability (`AB$ ChangeZone | Origin$ Graveyard |
// Destination$ Hand | TargetMin$ 0 | TargetMax$ 1 | ValidTgts$ Land.YouOwn`,
// no TgtZone$). Its census searched the battlefield, offered a land already
// in play, and omitted the eligible graveyard card the prompt names -- and
// the offered battlefield land could never pass effChangeZone's own Origin$
// Graveyard resolution guard, so the ability could not do what it promises.
// For that one unambiguous shape the Origin$ implies the target zone; see
// originImpliedTargetZone for the four gates that admit it.
func targetZones(sa *cards.SA) []state.Zone {
	var zones []state.Zone
	for _, z := range strings.Split(sa.Params["TgtZone"], ",") {
		switch strings.TrimSpace(z) {
		case "Battlefield":
			zones = appendUniqueZone(zones, state.ZBattlefield)
		case "Graveyard":
			zones = appendUniqueZone(zones, state.ZGraveyard)
		case "Hand":
			zones = appendUniqueZone(zones, state.ZHand)
		case "Exile":
			zones = appendUniqueZone(zones, state.ZExile)
		case "Stack":
			zones = appendUniqueZone(zones, state.ZStack)
		}
	}
	// A stack-targeting TargetType$ adds the stack even when no TgtZone$ is
	// present (the counterspell shape) and even alongside a TgtZone$
	// Battlefield for a spell-or-permanent effect (TgtZone$ Stack,Battlefield).
	if targetsStackObjects(sa.Params["TargetType"]) {
		zones = appendUniqueZone(zones, state.ZStack)
	}
	if len(zones) == 0 {
		if z, ok := originImpliedTargetZone(sa); ok {
			zones = []state.Zone{z}
		} else {
			zones = []state.Zone{state.ZBattlefield}
		}
	}
	return zones
}

// originImpliedTargetZone reports the implicit target zone for a ChangeZone
// whose Origin$ names exactly one concrete zone. Deliberately narrow -- this
// is established ONLY for the unambiguous public-graveyard object-targeted
// shape and must not grow into a general origin grammar (Origin$ Hand/
// Library/Exile carry hidden-information, chooser and mixed-zone semantics
// this does not establish; Origin$ Hand's mixed multi-zone handling lives in
// effects/zone.go). It admits an SA when ALL of these hold:
//
//  1. it is API$ ChangeZone;
//  2. it has no explicit TgtZone$ (explicit TgtZone$ stays authoritative;
//     this helper only runs from targetZones' empty fallback, but a TgtZone$
//     whose tokens were all unknown must not silently fall through to Origin$
//     either) and no stack-targeting TargetType$;
//  3. effects.ParseZones parses its Origin$ as exactly the one concrete
//     state.ZGraveyard -- not Any/All, not an unknown token, not a multi-zone
//     origin (ParseZones' ok=false on an unknown token fails closed);
//  4. its ValidTgts$ is object-only under the existing targetsPlayers
//     classifier, so a player-targeted ChangeZone keeps its existing
//     player-target route untouched.
//
// The zone feeds both legalTargetCandidates (offer time) and legalTargets
// (the CR 608.2b resolution recheck) through their shared targetZones calls,
// and effChangeZone's own Origin$ guard -- unchanged -- then accepts the
// chosen graveyard object at resolution.
func originImpliedTargetZone(sa *cards.SA) (state.Zone, bool) {
	if sa.API != "ChangeZone" {
		return 0, false
	}
	if sa.Params["TgtZone"] != "" || targetsStackObjects(sa.Params["TargetType"]) {
		return 0, false
	}
	if targetsPlayers(sa.Params["ValidTgts"]) {
		return 0, false
	}
	zones, all, ok := effects.ParseZones(sa.Params["Origin"])
	if !ok || all || len(zones) != 1 || zones[0] != state.ZGraveyard {
		return 0, false
	}
	return state.ZGraveyard, true
}

// appendUniqueZone appends z to zones when it is not already present,
// preserving the deterministic TgtZone$/TargetType$ order both askTarget and
// legalTargets share (never a map, so no map iteration order reaches a wire
// decision).
func appendUniqueZone(zones []state.Zone, z state.Zone) []state.Zone {
	for _, existing := range zones {
		if existing == z {
			return zones
		}
	}
	return append(zones, z)
}

// targetsStackObjects reports whether a Forge TargetType$ value names a
// target that lives on the stack: a spell (Spell/Instant/Sorcery), or an
// activated/triggered/spell-ability object. The base token precedes any "."
// qualifier (Spell.singleTarget, Instant.singleTarget, ...).
func targetsStackObjects(tt string) bool {
	for _, t := range strings.Split(tt, ",") {
		base, _, _ := strings.Cut(strings.TrimSpace(t), ".")
		switch base {
		case "Spell", "Instant", "Sorcery", "Activated", "Triggered", "SpellAbility":
			return true
		}
	}
	return false
}

// stackObjKind classifies one stack object for TargetType$ legality. The
// classifier moved to state.StackKindOf -- effects' Defined$ ValidStack arm
// must admit exactly what this census admits and cannot import rules, so one
// shared classifier serves both (state.StackKindOf's doc). The type alias
// and the three kind constants keep every existing rules-side name valid.
type stackObjKind = state.StackObjKind

const (
	stackSpell     = state.StackKindSpell     // a card object (a Face) on the stack
	stackActivated = state.StackKindActivated // an ability object minted by AbilityPush
	stackTriggered = state.StackKindTriggered // an ability object minted by TriggerPush/DelayedPush
)

func (e *Engine) stackObjKind(o *state.Object) stackObjKind { return state.StackKindOf(e.G, o) }

// targetTypeToken is state.StackKindToken: one comma-separated TargetType$
// token -- which stack object kinds its base admits, the controller qualifier
// read off the qualifiers after the base ("YouCtrl" -- controller must be the
// chooser; "OppCtrl" -- controller must not be; e.g. Weaver of Harmony's
// `Activated.YouCtrl,Triggered.YouCtrl`, Kang Dynasty's `Spell.OppCtrl`), and
// the card-type restriction the base or a qualifier can impose -- the bases
// "Instant" and "Sorcery" (Spider Sense's `Instant,Sorcery,Triggered`) and the
// Spell qualifiers "Instant"/"Sorcery" (Sister of Silence's
// `Spell.Instant,Spell.Sorcery,Activated,Triggered`) restrict the Spell kind
// to instant/sorcery CARD objects -- a creature spell is never admitted.
// Any OTHER qualifier (singleTarget, numTargets GE1, ...) is NOT read -- the
// token admits its full kind set with no restriction, the same widening the
// AGENTS.md TargetType$ row records.
type targetTypeToken = state.StackKindToken

// stackTargetKindTokens parses a TargetType$ value into its kind tokens.
// A parameter that is absent -- or whose tokens name no stack kind at all --
// defaults to Spell-only (today's behaviour, deliberately narrow: a spec
// that never said it wants abilities does not get them).
func stackTargetKindTokens(tt string) []targetTypeToken { return state.StackKindTokens(tt) }

// stackKindAdmits reports whether any TargetType$ token admits the stack
// object o (of kind k) controlled by controller, from chooser you's
// perspective. Token semantics are OR, matching ValidTgts$ alternatives:
// the object is offered when SOME token whose kind set contains k admits it
// under that token's own controller and card-type restriction. An
// instantOnly/sorceryOnly token checks the card object's Face, so a creature
// spell is never admitted by Spider Sense's `Instant,Sorcery,Triggered` or
// Sister of Silence's `Spell.Instant,Spell.Sorcery,...`; a Face-less object
// (never reachable for stackSpell, since StackObjKind only classifies a
// Face-bearing object as a spell) fails closed.
func stackKindAdmits(toks []targetTypeToken, k stackObjKind, o *state.Object, controller, you state.PlayerID) bool {
	return state.StackKindAdmits(toks, k, o, controller, you)
}

// targetName is the object's name for a target prompt, tolerating the ability
// stack object (no Face) a triggered ability's own target ask produces by
// falling back to its source permanent's name.
func (e *Engine) targetName(source state.ObjID) string {
	if o := e.G.Obj(source); o != nil {
		if f := o.Face(); f != nil && f.Name != "" {
			return f.Name
		}
		if s := e.G.Obj(o.Source); s != nil {
			if sf := s.Face(); sf != nil && sf.Name != "" {
				return sf.Name
			}
		}
	}
	// Falling back to the literal word "target" (reviewer minor 7) is
	// deliberate: every caller has already given the decision a usable
	// prompt, so this is only reached for an object with no name at all --
	// an unreadable name there is better than a fabricated one.
	return "target"
}

// targetOptionLabel renders one target option's label: the target's name
// followed by its controller's. The Face-less ability object a
// TargetType$ Activated/Triggered census now offers has no name of its own,
// so targetName's fallback (the source permanent's name) serves -- the two
// census consumers (cast.go's targetAsk and stack.go's askTarget) share this
// one helper so an ability object can never reach a nil-Face dereference in
// either.
func (e *Engine) targetOptionLabel(candidate targetCandidate) string {
	// The controller's name is seat-facing (the seat that answers sees it),
	// so it prefers the table's display name over the deck-identity slug.
	label := seatFacingName(e.G, candidate.player)
	if candidate.obj != 0 {
		label = e.targetName(candidate.obj) + " (" + label + ")"
	}
	return label
}

// protectionSource resolves the object whose characteristics decide whether
// "protection from X" filters out a candidate target or a Damage recipient
// (Task 15 fix round 1, Critical C2): for an ability stack object -- the
// top-of-stack wrapper events.Apply mints via AbilityPush/TriggerPush with no
// Face -- that is the Source permanent that carries the ability, because
// effects.ColorsOf and the Face-type reads in sourceHasQuality both return
// nothing for a Face-less object (CR 606.3: targeting checks the spell or
// ability's source, and for an activated or triggered ability that source is
// the permanent that granted it). For everything else -- a spell's own stack
// object, which IS the card and so carries a Face, or a plain permanent -- it
// is the object itself. This is the single definition both askTarget (which
// in the same round discovered it was passing the Face-less ability object)
// and legalTargets consult, so the two sites always agree on what "the
// source" is.
func (e *Engine) protectionSource(source state.ObjID) state.ObjID {
	if o := e.G.Obj(source); o != nil && o.Ability != nil && o.Source != 0 {
		return o.Source
	}
	return source
}

// describeTargetEffect is intentionally independent of game state and host.
// Only literal damage is known: evaluating Num here would collapse unresolved
// SVars to zero and would mistake a current X/count for a resolution forecast.
func describeTargetEffect(sa *cards.SA) *decision.TargetEffect {
	if sa == nil {
		return nil
	}
	out := &decision.TargetEffect{API: sa.API}
	switch sa.API {
	case "DealDamage", "DamageAll":
		out.Damage = &decision.DamageEffect{}
		// Fixed-width parsing is architecture-independent and safely representable
		// by the wire's JavaScript number. Negative/overflow/missing stay unknown.
		if n, err := strconv.ParseInt(sa.Params["NumDmg"], 10, 32); err == nil && n >= 0 {
			amount := int(n)
			out.Damage.Amount = &amount
		}
	}
	return out
}

type targetCandidate struct {
	kind   string
	obj    state.ObjID
	player state.PlayerID
}

// legalTargetCandidates is the pure target census shared by cast-option
// enumeration and the post-announcement target ask. It reads state in the
// same deterministic order as the old askTarget loops and emits no events.
//
// source is the object the spec's Self/Other predicates and the protection
// test are resolved against (for an ability, the Source permanent).
// excludeSelf is the object a prospective target may not equal -- the CR
// 115.5 self-targeting rule. It is separate from source because during a
// cast/activation proposal the two diverge: a spell on the stack may not
// target itself (excludeSelf == the card), while an activated ability CAN
// target its own Source permanent (excludeSelf == 0, since the Face-less
// ability object is not on the stack yet). Callers set excludeSelf == 0 to
// disable the rule.
func (e *Engine) legalTargetCandidates(p state.PlayerID, source, excludeSelf state.ObjID, sa *cards.SA) []targetCandidate {
	return e.candidatesFor(p, source, excludeSelf, sa, true)
}

// affectedCandidates is the Overload counterpart of legalTargetCandidates.
// It applies the script's object/player filter and zone/type restrictions but
// deliberately omits every rule that exists only because something is a
// target: protection, hexproof/CantTarget, and becomes-target bookkeeping.
func (e *Engine) affectedCandidates(p state.PlayerID, source, excludeSelf state.ObjID, sa *cards.SA) []targetCandidate {
	return e.candidatesFor(p, source, excludeSelf, sa, false)
}

func (e *Engine) candidatesFor(p state.PlayerID, source, excludeSelf state.ObjID, sa *cards.SA, targeting bool) []targetCandidate {
	spec := sa.Params["ValidTgts"]
	sc := e.targetSpecContext(source, excludeSelf, p)
	zones := targetZones(sa)
	var out []targetCandidate
	// Players are offered only alongside the default battlefield search and
	// only when the spec can name one. A spec that routes elsewhere
	// (TgtZone$ Graveyard/Hand/Exile) targets objects only -- never a player.
	if len(zones) == 1 && zones[0] == state.ZBattlefield && targetsPlayers(spec) {
		for _, q := range e.G.AliveFrom(0) {
			out = append(out, targetCandidate{kind: "player", player: q})
		}
	}
	// Resolve the source ONCE for the whole census -- for an ability this is
	// the Source permanent, not the Face-less stack object. Every protection
	// test below is guarded on the candidate's zone, because a permanent's
	// static ability functions only on the battlefield (CR 604.3), so a
	// printed protection does not withhold a target sitting in the
	// Graveyard/Hand/Exile that a TgtZone$ spec is asking about.
	protSrc := e.protectionSource(source)
	for _, z := range zones {
		if z == state.ZStack {
			// The stack is a single, shared sequence, not a per-seat zone, so
			// its objects are enumerated ONCE each -- labelled with the
			// object's own controller -- rather than once per alive seat,
			// which would offer the same spell N times in an N-seat game and
			// drift the option list. Which stack objects are targetable is
			// TargetType$'s job (CR 115.5 aside): Spell admits card objects,
			// Instant/Sorcery admit instant/sorcery card objects only (Spider
			// Sense, Sister of Silence), Activated/Triggered admit the ability
			// objects AbilityPush/TriggerPush mint, SpellAbility admits all
			// three, and a TargetType$ naming no stack kind falls back to
			// Spell -- the pre-fix behaviour, kept for every spec that never
			// said otherwise (the default stays the narrow one, never
			// widened).
			toks := stackTargetKindTokens(sa.Params["TargetType"])
			for _, oid := range e.G.Zone(state.ZStack, 0) {
				o := e.G.Obj(oid)
				// CR 115.5: a spell or ability on the stack is an illegal
				// target for itself. This census runs after PutOnStack or
				// AbilityPush, so source is already that object atop the
				// stack -- its own id must never be offered, or a
				// counterspell would counter itself. Only the source OBJECT
				// is excluded, never a different copy of the same card.
				if o == nil || (excludeSelf != 0 && oid == excludeSelf) {
					continue
				}
				if !stackKindAdmits(toks, e.stackObjKind(o), o, o.Controller, p) {
					continue
				}
				if effects.MatchesSpecCtx(e.G, targetSpecForZone(spec, z), oid, sc) {
					out = append(out, targetCandidate{kind: "permanent", obj: oid, player: o.Controller})
				}
			}
			continue
		}
		// Hand is the chooser's own hand only (CR 701.15a); the other
		// non-battlefield zones are public, so every seat's slice is offered.
		players := e.G.AliveFrom(0)
		if z == state.ZHand {
			players = []state.PlayerID{p}
		}
		for _, q := range players {
			for _, oid := range e.G.Zone(z, q) {
				o := e.G.Obj(oid)
				// CR 702.16c withholds a permanent protected from the
				// targeting source's qualities; a CantTarget restriction
				// (Vines of Vastwood) withholds one from the spoke player.
				// Both function only on the battlefield (CR 604.3), the same
				// gate as protection above. CR 115.5 excludes the source.
				if o != nil && o.Face() != nil && (excludeSelf == 0 || oid != excludeSelf) &&
					effects.MatchesSpecCtx(e.G, targetSpecForZone(spec, z), oid, sc) &&
					(!targeting || !(o.Zone == state.ZBattlefield && e.protectedFrom(oid, protSrc))) &&
					(!targeting || !(o.Zone == state.ZBattlefield && e.restrictionBlocksTarget(oid, p))) {
					out = append(out, targetCandidate{kind: "permanent", obj: oid, player: q})
				}
			}
		}
	}
	out = e.filterTargetsWithDefinedController(out, sa, sc)
	return e.filterTargetValidTargeting(out, sa, sc)
}

// filterTargetValidTargeting implements TargetValidTargeting$ (Not of This
// World: "Counter target spell or ability that targets a permanent you
// control", TargetValidTargeting$ Permanent.YouCtrl+inRealZoneBattlefield):
// the candidate stack object QUALIFIES only when its own chosen targets
// include an object matching the spec, evaluated from the targeting
// ability's controller (the counter's "you" is its controller, not the
// countered spell's). Only permanent-kind candidates carry targets to
// check; a player candidate or a stack object without recorded targets --
// the ability-object shapes whose per-stack target bindings live in the
// trigger/activation roles this filter cannot see -- fails the filter, the
// narrower direction (Not of This World can still counter every spell that
// visibly targeted a matching permanent; an ability it cannot verify is
// never offered, never wrongly offered).
func (e *Engine) filterTargetValidTargeting(in []targetCandidate, sa *cards.SA, sc effects.SpecContext) []targetCandidate {
	spec := strings.TrimSpace(sa.Params["TargetValidTargeting"])
	if spec == "" {
		return in
	}
	out := make([]targetCandidate, 0, len(in))
	for _, cand := range in {
		if cand.kind != "permanent" {
			continue
		}
		o := e.G.Obj(cand.obj)
		if o == nil {
			continue
		}
		for _, t := range o.Targets {
			if t.IsPlayer || t.Obj == 0 {
				continue
			}
			if effects.MatchesSpecCtx(e.G, spec, t.Obj, sc) {
				out = append(out, cand)
				break
			}
		}
	}
	return out
}

// filterTargetsWithDefinedController implements the common target restriction
// that says the chosen object must be controlled by an event-role player. It
// is deliberately applied once after every zone's candidates are collected,
// so battlefield, graveyard and stack target offers cannot drift apart.
//
// Only a selector this build binds narrows the offer. An unsupported selector
// (ParentTarget, ParentTargetedController, TriggeredCauser, ...) and a
// supported role the current trigger did not bind leave the candidates
// unchanged -- the offer every such ability had before this restriction was
// read -- so no existing ability silently loses its targets. The two roles
// only an attack-declaration trigger binds (TriggeredAttackingPlayer,
// TriggeredAttackedTarget: Karazikar, Firkraag, Seifer, Gornog, Whirlwind
// Killer) fail closed when unbound instead, since offering every creature
// would widen "target creature that player controls" to any player's.
func (e *Engine) filterTargetsWithDefinedController(in []targetCandidate, sa *cards.SA, sc effects.SpecContext) []targetCandidate {
	ref := strings.TrimSpace(sa.Params["TargetsWithDefinedController"])
	if ref == "" {
		return in
	}
	var player state.PlayerID
	var ok bool
	failClosed := false
	switch ref {
	case "TriggeredTarget":
		if sc.TriggerTarget.IsPlayer {
			player, ok = sc.TriggerTarget.Player, true
		} else if o := e.G.Obj(sc.TriggerTarget.Obj); o != nil {
			player, ok = o.Controller, true
		}
	case "TriggeredDefendingPlayer":
		if sc.DefendingPlayer.IsPlayer {
			player, ok = sc.DefendingPlayer.Player, true
		}
	case "TriggeredPlayer":
		if sc.TriggerPlayer.IsPlayer {
			player, ok = sc.TriggerPlayer.Player, true
		}
	case "TriggeredAttackingPlayer":
		failClosed = true
		if sc.AttackingPlayer.IsPlayer {
			player, ok = sc.AttackingPlayer.Player, true
		}
	case "TriggeredAttackedTarget":
		failClosed = true
		if sc.AttackedTarget.IsPlayer {
			player, ok = sc.AttackedTarget.Player, true
		}
	case "TriggeredCardController":
		player, ok = effects.TriggeredCardController(e.G, sc.TriggerContext, nil)
	}
	if !ok {
		if failClosed {
			return nil
		}
		return in
	}
	out := in[:0]
	for _, candidate := range in {
		if candidate.kind != "permanent" {
			continue
		}
		if o := e.G.Obj(candidate.obj); o != nil && o.Controller == player {
			out = append(out, candidate)
		}
	}
	return out
}

// askCrossModeCharmTargets poses the cross-mode TargetUnique family's ONE
// combined target ask: Min == Max == the number of chosen target-bearing
// modes, over the shared candidate pool of the modes' common ValidTgts$ spec,
// with every option's Group naming its player — Decision.Validate's
// mutual-exclusion rule (and botpolicy clamp's group discipline) enforce
// "each mode must target a different player" on the wire, so an intent that
// reuses a player is not merely wrong but impossible to submit. The answer
// records onto the stack object in choice order, which is the chosen-mode
// order, so the per-mode attribution is positional and replay-safe without
// any new event kind or field. Returns false (nothing asked) when the legal
// candidates are fewer than the modes that need them — the caller keeps the
// historical first-mode narrowing, whose own insufficiency handling governs.
func (e *Engine) askCrossModeCharmTargets(p state.PlayerID, source state.ObjID, tbms []*cards.SA) bool {
	k := len(tbms)
	sub := tbms[0]
	candidates := e.legalTargetCandidates(p, source, source, sub)
	if len(candidates) < k {
		return false
	}
	d := &decision.Decision{Player: p, Kind: decision.KTarget, Min: k, Max: k,
		Prompt: fmt.Sprintf("Choose %d targets: one for each mode, each a different player", k),
		Source: source, TargetEffect: describeTargetEffect(sub)}
	for _, candidate := range candidates {
		o := decision.Option{Index: len(d.Options), Kind: candidate.kind,
			Label: e.targetOptionLabel(candidate), Obj: candidate.obj, Player: candidate.player}
		if candidate.kind == "player" {
			o.Group = "charm-mode-player-" + strconv.Itoa(int(candidate.player))
		}
		d.Options = append(d.Options, o)
	}
	e.drainAwaitsTarget = true
	e.ask(d)
	return true
}

// askTarget offers every legal target for a spell or ability. It deliberately
// retains the post-push insufficient-target backstop: modal and dynamic target
// counts are not rejected by the earlier cast-offer census.
func (e *Engine) askTarget(p state.PlayerID, source state.ObjID, sa *cards.SA) {
	min, max := targetBounds(sa)
	candidates := e.legalTargetCandidates(p, source, source, sa)
	oneEach := strings.EqualFold(sa.Params["TargetsForEachPlayer"], "True")
	groups := map[state.PlayerID]bool{}
	if oneEach {
		// Forge TargetRestrictions.setForEachPlayer limits the selected targets
		// to one controlled by each player. Option.Group makes that restriction
		// part of the generic decision contract, so every target API consumes
		// the same enforcement rather than each effect maintaining a picker.
		for _, candidate := range candidates {
			owner := candidate.player
			if candidate.kind != "player" {
				if o := e.G.Obj(candidate.obj); o != nil {
					owner = o.Controller
				}
			}
			groups[owner] = true
		}
		if strings.EqualFold(sa.Params["TargetMin"], "OneEach") {
			min = len(groups)
		}
		if strings.EqualFold(sa.Params["TargetMax"], "OneEach") {
			max = len(groups)
		}
	}
	d := &decision.Decision{Player: p, Kind: decision.KTarget, Min: min, Max: max,
		Prompt: "Choose a target for " + e.targetName(source),
		Source: source, TargetEffect: describeTargetEffect(sa)}
	for _, candidate := range candidates {
		// targetOptionLabel tolerates the Face-less ability object a
		// TargetType$ Activated/Triggered spec now offers: targetName falls
		// back to the source permanent's name.
		label := e.targetOptionLabel(candidate)
		o := decision.Option{Index: len(d.Options), Kind: candidate.kind,
			Label: label, Obj: candidate.obj, Player: candidate.player}
		if oneEach {
			owner := candidate.player
			if candidate.kind != "player" {
				if obj := e.G.Obj(candidate.obj); obj != nil {
					owner = obj.Controller
				}
			}
			o.Group = "target-controller-" + strconv.Itoa(int(owner))
		}
		d.Options = append(d.Options, o)
	}
	if min == 0 {
		// Requirement N2 / totality: a target-hungry subject whose minimum
		// is zero resolves untargeted when NO legal target exists. When at
		// least one exists it is still offered (Min 0 lets the chooser take
		// none); a zero-option decision would strand the cast, never asked.
		if len(d.Options) == 0 {
			return
		}
	} else if len(d.Options) < min {
		// A target-hungry subject with fewer legal targets than Min uses CR
		// 608.2b's existing counter/fizzle exit: an immediate move to its
		// normal resting place (exile instead of the graveyard for a
		// Flashback cast, and for a triggered ability object -- which has no
		// graveyard -- exile per CR 608.2m, same as every ability fizzle in
		// resolveTop).
		rest := spellFizzleZone(e.G.Obj(source))
		if o := e.G.Obj(source); o != nil && o.Ability != nil {
			rest = state.ZExile
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: source,
			From: state.ZStack, To: rest, Text: "countered: no legal targets"})
		e.ensureLeftTheStack(source, rest, "a replacement fully discarded this "+
			"spell's 'countered: no legal targets' move without relocating it anywhere; sent "+
			"to its resting zone instead of re-resolving forever")
		// Ruling T14-e: p, the casting player, not e.G.Active -- CR 117.3c,
		// the caster keeps priority even when it fizzles. Only a SPELL keeps
		// priority this way: an ability fizzling is parked in exile (ceases
		// to exist) mid-drain, and handing out priority from inside the
		// trigger drain would double-emit against the drain's own tail.
		if o := e.G.Obj(source); o == nil || o.Ability == nil {
			e.emit(events.Event{Kind: events.Priority, Player: p, Amount: 0})
		}
		return
	}
	e.ask(d)
}

// handleTarget records the chosen target(s) via TargetsChosen events (Ruling
// T14-b) rather than writing state.Object.Targets directly: apply.go clears
// Targets on a zone change but nothing else ever set it, so a direct write
// here would leave a replayed game with no targets while the live game had
// them. N targets record as TargetMin$ asks for: the first option replaces
// with targets (TargetsChosen shape 0 for an object, 1 for a player), each
// further option appends one (shape 2 object, 3 player) -- see apply.go's
// TargetsChosen case and its test TestTargetsChosenAppendShapes.
func (e *Engine) handleTarget(d *decision.Decision, in decision.Intent) {
	chosen := d.Chosen(in)
	// A cast-flow target decision (CR 601.2c, asked by targetAsk after the
	// object was pushed by pushCast but BEFORE any cost is paid): completing
	// it means recording the chosen targets onto the stack object and then
	// committing the transaction -- paying the costs and firing the cast
	// trigger -- via payCast. For a spell the stack object is the card
	// itself, already on the stack (pushCast), so targets are recorded before
	// payment (601.2c before 601.2h); for an activated ability the stack
	// object is minted by payCast's AbilityPush, so targets are recorded
	// AFTER it. Targets are never written directly (they go through
	// TargetsChosen events) and always after the push, because a zone change
	// clears them.
	if e.cast != nil {
		pc := e.cast
		pc.targets = targetOptions(chosen)
		e.repriceForTargets(pc)
		if pc.ability < 0 {
			if pc.stackObj != 0 {
				e.recordChosenTargets(pc.stackObj, chosen)
			}
			e.payCast()
		} else {
			e.payCast()
			if pc.stackObj != 0 {
				e.recordChosenTargets(pc.stackObj, chosen)
			}
		}
		if e.drainAwaitsTarget {
			e.drainAwaitsTarget = false
			e.resumeTriggerDrain()
		} else if e.pending == nil {
			// CR 117.3c: the caster keeps priority after a completed cast.
			// payCast can pose a MID-CAST ask (CR 601.2g's mana window, a
			// cost choice) which leaves the engine parked on that question;
			// the casting player does not keep priority until the cast has
			// actually paid every cost and fired its cast trigger, so the
			// "caster keeps priority" marker is emitted only once no further
			// announcement decision is outstanding. Emitting it while parked
			// is the same class of log lie as the pass-branch emit this task
			// removed: the log would assert the caster held priority at a
			// moment the engine is waiting on an unanswered question.
			e.emit(events.Event{Kind: events.Priority, Player: in.Player, Amount: 0})
		}
		return
	}
	e.recordChosenTargets(d.Source, chosen)
	// A target decision asked by a trigger drain (putTriggersOnStack's
	// pushTrigger, immediately after the trigger's TriggerPush -- Task 20's
	// checkTriggers never asked targets, so only a spell's cast-time ask
	// reached here before) does not hand out priority: the drain's own tail
	// does, through the SAME continuation handleTriggerOrder uses
	// (resumeTriggerDrain, turn.go), so a second, later trigger is still
	// placed before any player acts. A spell's cast-time target decision
	// keeps its caster's priority (CR 117.3c) via the emit below.
	if e.drainAwaitsTarget {
		e.drainAwaitsTarget = false
		e.resumeTriggerDrain()
		return
	}
	// Ruling T14-e: the submitting player, not e.G.Active -- CR 117.3c, the
	// player who chose the target (the caster) keeps priority.
	e.emit(events.Event{Kind: events.Priority, Player: in.Player, Amount: 0})
}

// targetOptions converts a target decision's selected options to the
// proposal-local target representation used while its cost is still being
// assembled. Events remain the source of truth once the stack object exists;
// this short-lived copy is only what lets an activated ability evaluate a
// ValidTarget$ cost modifier before its AbilityPush object is minted.
func targetOptions(chosen []decision.Option) []state.Target {
	out := make([]state.Target, 0, len(chosen))
	for _, opt := range chosen {
		if opt.Kind == "player" {
			out = append(out, state.Target{Player: opt.Player, IsPlayer: true})
		} else {
			out = append(out, state.Target{Obj: opt.Obj})
		}
	}
	return out
}

// recordChosenTargets emits the TargetsChosen events for a set of chosen
// target options onto the given object. Amount discriminates the target shape
// (Ruling T14-b, extended by Task 4): 0 replace-with-object, 1
// replace-with-player, 2 append-object, 3 append-player. It is the shared
// recording path for both a cast-flow target decision (onto the stack object,
// after the push) and a triggered ability's own post-TriggerPush ask.
func (e *Engine) recordChosenTargets(targetObj state.ObjID, chosen []decision.Option) {
	for i, opt := range chosen {
		ev := events.Event{Kind: events.TargetsChosen, Obj: targetObj}
		if opt.Kind == "player" {
			// shape 1 replace / shape 3 append a single player target.
			ev.Amount = 1
			if i > 0 {
				ev.Amount = 3
			}
			ev.Player = opt.Player
		} else {
			// shape 0 replace / shape 2 append one object target.
			if i > 0 {
				ev.Amount = 2
			}
			ev.IDs = []state.ObjID{opt.Obj}
		}
		e.emit(ev)
	}
}

// resolveTop resolves the object on top of the stack and moves it to
// wherever it goes next.
//
// CR 608.2b: before anything else runs, every target recorded when this
// spell or ability was put on the stack is rechecked against legalTargets
// below -- a target legal when chosen can stop being legal by the time its
// spell reaches the top of the stack, most commonly a creature that died to
// something else in the meantime. With no legal target left, this does not
// resolve at all: it leaves the stack (a spell to its owner's graveyard, an
// ability to exile per CR 608.2m just below) with no effect -- not even a
// SubAbility chained onto it that names no target of its own (Defined$ You
// and the like). That distinguishes this from the per-target nil-checks
// primitives like effDealDamage already had (Task 18, effects/damage.go):
// those already skip a target that individually vanished, but nothing
// before this stopped the OTHER, untargeted parts of the same spell's
// script from running anyway once every target it had was gone. With only
// some targets still legal, resolution proceeds against exactly that
// narrowed set -- CR 608.2b's "resolves, doing as much as possible".
func (e *Engine) resolveTop() {
	id := e.G.Stack[len(e.G.Stack)-1]
	o := e.G.Obj(id)
	savedResolving := e.resolvingObj
	e.resolvingObj = id
	defer func() { e.resolvingObj = savedResolving }()

	if o.Ability != nil {
		// A keyword trigger that refers to one particular permanent incarnation
		// (Evoke's "sacrifice it") loses track when that permanent changes
		// zones. The ability still resolves and leaves the stack, but does
		// nothing to the new object now sharing its stable ObjID (CR 400.7).
		if o.SourceIncarnation != 0 {
			src := e.G.Obj(o.Source)
			if src == nil || src.Incarnation != o.SourceIncarnation {
				e.emit(events.Event{Kind: events.Resolve, Obj: id})
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZExile})
				return
			}
		}
		// A triggered or activated ability with no printed card: Ruling
		// T14-c / F3 -- Face() returns nil for these, so this branch must
		// run before anything below touches it. Task 20 is what actually
		// puts objects like this on the stack.
		//
		// CR 603.4 (intervening-if): a triggered ability's condition is
		// checked both when it would trigger AND again as it resolves; if
		// it no longer holds, the ability is removed from the stack and does
		// nothing. The trigger was queued because triggerConditionHolds was
		// true at trigger time (triggerMatches), but Scute Mob's "whenever
		// you control five or more lands" can be false by the time the
		// ability resolves -- here a real instant response destroyed one of
		// those lands. findTriggerForAbility identifies the T: line from the
		// SA pointer: it is false for an activated ability (whose SA comes
		// from Abilities, not a Triggers entry) so this recheck never
		// applies to one, and false for a source whose face has changed or
		// gone so a trigger-only rule never fires on unknown provenance.
		// The fizzle move is the same exile rest the ability branch uses for
		// CR 608.2b's no-legal-targets case: an ability "ceases to exist"
		// (608.2m) rather than moving to a card zone, and this build parks
		// such objects in exile. Ordered first because it decides whether
		// the ability does anything at all.
		if t, ok := e.findTriggerForAbility(o.Source, o.Ability); ok {
			if !e.triggerConditionHolds(t, o.Source) {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZStack, To: state.ZExile, Text: "fizzled: intervening-if no longer holds"})
				e.ensureLeftTheStack(id, state.ZExile, "a replacement fully discarded this "+
					"ability's 'intervening-if' move without relocating it anywhere; sent to exile "+
					"instead of re-resolving forever")
				return
			}
		}
		// No triggered or activated ability this build produces ever
		// actually populates Targets (only Remembered): Task 20's
		// checkTriggers never calls askTarget, which is the only place
		// TargetsChosen is ever emitted from. So this is unreachable in
		// practice today, but a stack object is a stack object, and CR
		// 608.2b's "spell or ability" covers this shape too if a later
		// task ever gives a triggered ability a player-chosen target.
		targets := o.Targets
		// Fix round 2 (re-review N1): the gate is `spec != ""` -- "this
		// ability declares a targeting requirement" -- not `len(targets) > 0`
		// -- "this ability happens to have targets right now". The old form
		// used the latter as a proxy for the former, so an ability that NEEDS
		// a target but has none recorded skipped CR 608.2b's recheck entirely
		// and resolved. Zero recorded targets is zero LEGAL targets, which is
		// exactly what 608.2b counters.
		// Requirement N2: an ability that MAY target zero things (TargetMin$ 0)
		// and has none recorded resolves untargeted rather than fizzling --
		// targetMin(o.Ability)==0 && len(targets)==0 is the exemption.
		if spec := o.Ability.Params["ValidTgts"]; spec != "" && !(targetMin(o.Ability) == 0 && len(targets) == 0) {
			legal := e.legalTargets(targets, spec, targetZones(o.Ability), o.Controller, o.Source, id)
			if len(legal) == 0 {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZStack, To: state.ZExile, Text: "fizzled: no legal targets remain"})
				e.ensureLeftTheStack(id, state.ZExile, "a replacement fully discarded this "+
					"ability's 'fizzled: no legal targets' move without relocating it anywhere; "+
					"sent to exile instead of re-resolving forever")
				return
			}
			targets = legal
		}
		// CR 608.2m: a resolved ability just ceases to exist rather than
		// moving to a card zone. This build has no "ceases to exist" zone,
		// so it is parked in exile as the closest existing approximation.
		e.emit(events.Event{Kind: events.Resolve, Obj: id})
		// CR 702.35b: the mandatory, respondable madness trigger makes its
		// cast-or-graveyard choice only as it resolves. Stifle reaches this
		// object before this branch; if the exiled card has moved meanwhile,
		// the ability simply finishes with no choice.
		if o.Ability.API == "MadnessCast" {
			if e.askMadnessCast(o) {
				return
			}
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZExile})
			return
		}
		// CR 603.5: an optional triggered ability goes on the stack regardless
		// (putTriggersOnStack pushes it unconditionally), and its controller
		// -- or whatever seat its OptionalDecider$ names -- chooses whether to
		// apply the effect as the ability resolves. With the Resolve event
		// already logged, pose that yes/no now and SPEND the resolution: a
		// yes re-enters it (resumeResolution runs the effect, exactly as the
		// tail below would have), a no lets the ability leave the stack
		// having done nothing. A decider who has left the game is nobody to
		// apply an effect to, so the ability ceases to exist (CR 800.4a) and
		// is parked in exile like the other ceased-to-exist rests. Activated
		// abilities and mandatory triggers (findTriggerForAbility returns
		// false for the former, or an OptionalDecider-less trigger for the
		// latter) fall straight through to their effect below.
		if t, ok := e.findTriggerForAbility(o.Source, o.Ability); ok {
			if spec := t.Params["OptionalDecider"]; spec != "" {
				who, askable := e.deciderFromSpec(spec, o.Controller, o.Remembered, e.triggerContexts[id])
				if !askable {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZStack, To: state.ZExile, Text: "ceased to exist: its optional decider left the game"})
					e.ensureLeftTheStack(id, state.ZExile, "the optional decider of this ability left the game, so the "+
						"ability ceased to exist (CR 800.4a) and was parked in exile")
					return
				}
				e.askOptionalAtResolution(who, o, o.Ability, e.abilityLabel(o, t))
				return
			}
		}
		// Cumulative upkeep is an ordinary trigger through placement, but its
		// age/payment resolution needs rules' cost machinery. Mana Vault's
		// triggered Untap is the one ordinary effect shape authorized to use
		// that window; unrelated Cost$-bearing trigger effects retain their
		// established executor semantics. ImmediateTrigger joins Untap: its AB
		// shape is Forge's "you may pay <Cost$>. When you do, ..." idiom (Speed,
		// Young Avenger's TrigImmediateTrig -- the only repo-deck carrier), so
		// the ordinary triggered-cost window poses the pay/decline ask before
		// the body runs; a decline leaves the body unexecuted exactly as CR
		// 603.5's "when you do" promises. The DB shape's optional payment stays
		// the UnlessCost$ gate's (effects.unlessProceed); a plain Cost$ on a DB
		// ImmediateTrigger remains the established free-executor semantics.
		if o.Ability.API == "CumulativeUpkeep" {
			e.startCumulativeUpkeep(id, o.Source, o.Ability)
			return
		}
		if _, triggered := e.findTriggerForAbility(o.Source, o.Ability); triggered &&
			(o.Ability.API == "Untap" || o.Ability.API == "ImmediateTrigger") &&
			o.Ability.Params["Cost"] != "" {
			e.startTriggeredEffectCost(&resumePoint{kind: "effect_cost", obj: id, sa: o.Ability}, o.Source)
			return
		}
		// The ability object itself has no Face, so its SVar table (needed
		// for Num's SVar indirection, e.g. Goblin Piledriver's "NumAtt$ +X")
		// comes from the permanent that granted it (o.Source) instead.
		// SVars are static card-script text that never changes after
		// parsing, so reading them live from the source's current Face at
		// resolution time is equivalent to a snapshot taken when the
		// trigger was queued, with no need for a new field to carry one
		// through the stack. A source that has since left the battlefield
		// (or ceased to exist) has nothing to read here and degrades to a
		// nil SVar table, same as before this ability object existed at
		// all, rather than panicking.
		var svars map[string]string
		if src := e.G.Obj(o.Source); src != nil {
			if sf := src.Face(); sf != nil {
				svars = sf.SVars
			}
		}
		// Ruling T20-b: Source must be o.Source (the permanent that has this
		// ability), not id (the transient stack-object wrapper) -- Defined$
		// Self, the most common Defined$ value in real trigger scripts,
		// resolves to Ctx.Source, and a wrapper ID means "Self" refers to a
		// stack object with no Face() that leaves play the instant this
		// resolves, so the effect would silently apply to nothing. The SVar
		// lookup two lines above already gets this right by reading from
		// o.Source; this was a one-line inconsistency, not a second design.
		ctx := &effects.Ctx{Source: o.Source, Controller: o.Controller,
			Targets: targets, Remembered: o.Remembered, Captured: o.Remembered, TriggerContext: e.triggerContexts[id]}
		// The SA whose targeting the placement ask actually offered, not
		// blindly the resolving SA: for a non-modal ability that is the outer
		// SA's own ValidTgts$ (pushTrigger's askTarget), for a modal one it is
		// the first target-bearing CHOSEN MODE's sub -- handleModes' placement
		// branch asks the mode sub and skips the outer ask entirely (a Charm's
		// ValidTgts$ lives inside its modes, Kami of Restless Shadows'
		// RaiseScoundrel). Deriving the marker from the outer SA alone left
		// the modal shape unmarked, so a Min-0 mode target the chooser elected
		// ZERO of was re-posed by effChangeZone's mid-resolution ask at
		// resolution -- the exact duplicate-ask defect the marker exists to
		// stop.
		offeredSA := (*cards.SA)(nil)
		if o.Ability != nil {
			if len(o.ChosenModes) > 0 && strings.TrimSpace(o.Ability.Params["Choices"]) != "" {
				if src := e.G.Obj(o.Source); src != nil && src.Face() != nil {
					for _, name := range o.ChosenModes {
						if sub := cards.ResolveSVar(src.Face().SVars, name); sub != nil &&
							strings.TrimSpace(sub.Params["ValidTgts"]) != "" {
							offeredSA = sub
							break
						}
					}
				}
			} else {
				offeredSA = o.Ability
			}
		}
		if offeredSA != nil {
			ctx.TargetsOffered = strings.TrimSpace(offeredSA.Params["ValidTgts"]) != ""
		}
		if lki, ok := e.triggerLKI[id]; ok {
			ctx.LKI = lki.object
			ctx.LKIPower, ctx.LKIToughness, ctx.LKIPTValid =
				lki.power, lki.toughness, lki.ptValid
		}
		// CR 107.3i: X is the value the activator chose for a Cost$ carrying
		// {X} (recorded on the ability stack object by commitCast's CastInfo,
		// emitted right after the AbilityPush). Zero for a trigger, which was
		// never paid an X -- and for a trigger CR 107.3m rebinds X to the
		// spell that became the permanent (an ETB trigger) or the spell the
		// trigger fired on (a cast/magecraft trigger), which triggerPaidX
		// reads off the causing event's card.
		ctx.X = o.X
		if ctx.X == 0 {
			ctx.X = e.triggerPaidX(id, o)
		}
		// A cost-paid sacrifice carried its objects' LKI snapshot on the
		// engine (rules/cast.go commitCast), keyed by this stack object id;
		// load it so the ability's Sacrificed$<Property> heads resolve against
		// what it sacrificed. Mirror of triggerContexts: engine-only.
		ctx.Sacrificed = e.sacrificedLKI[id]
		if link, ok := e.sourceLifelinkLKI[id]; ok {
			ctx.SourceLifelinkLKI = link
			ctx.SourceLifelinkLKIValid = true
		}
		if controller, ok := e.sourceControllerLKI[id]; ok {
			ctx.SourceControllerLKI = controller
			ctx.SourceControllerLKIValid = true
		}
		if lki := e.damageSourceLKI[id]; lki != nil {
			ctx.DamageSourceLKI = cloneDamageSourceLKI(lki)
		}
		effects.SetSVars(ctx, svars)
		// CR 603.3c: the mode choice was announced at placement (pushTrigger
		// asked KModes and handleModes recorded the answer into ChosenModes).
		// Pre-seeding Ctx.Modes makes effCharm take its re-entry branch and
		// run exactly the chosen modes rather than asking again at
		// resolution. Nil for a non-modal trigger, for an activated ability,
		// and for any trigger the placement ask never reached.
		ctx.Modes = o.ChosenModes
		e.damaging = o.Source
		e.contChain = e.contChain[:0]
		e.repeatReported = nil
		effects.Resolve(e, ctx, o.Ability)
		e.damaging = 0
		if e.resume != nil {
			// A placement-announced modal ability can reach a nested ask during
			// this initial pass. Preserve every enclosing continuation exactly as
			// resumeResolution does for a nested ask reached on re-entry.
			e.resume.outer = e.buildContinuationChain(e.contChain, id, nil)
			// A mid-resolution ask (M2d-2): the effect that asked has set a
			// decision pending and recorded a resume point. The object stays
			// on the stack waiting for the answer -- entering the exile exit
			// below would discard it mid-resolution. The answered decision
			// re-enters the suspended effect through resumeResolution
			// (rules/resolution.go), which runs the rest of this same tail.
			return
		}
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: state.ZExile})
		e.ensureLeftTheStack(id, state.ZExile, "a replacement fully discarded this resolved "+
			"ability's own move off the stack without relocating it anywhere; sent to exile "+
			"instead of re-resolving forever")
		return
	}

	f := o.Face()
	sa := f.SpellAbility()
	targets := o.Targets
	// targetSA is the SA whose ValidTgts$ the cast-flow target ask offered
	// (the modal declaration for a Charm, the SpellAbility itself otherwise);
	// hoisted so the resolution ctx can carry the TargetsOffered marker.
	targetSA := modalTargetSA(f, sa, o.ChosenModes)
	// An overloaded spell affects the matching set as it resolves, never as
	// targets chosen during announcement. This fresh non-target census means
	// protection/hexproof do not apply and objects entering or changing
	// controller in response are included correctly. Effect primitives keep
	// their generic Ctx.Targets recipient API; only the source of that list is
	// different.
	overloaded := o.CastFlags&state.FlagOverloaded != 0
	if overloaded && sa != nil {
		if targetSA != nil {
			for _, cand := range e.affectedCandidates(o.Controller, id, id, targetSA) {
				if cand.kind == "player" {
					targets = append(targets, state.Target{Player: cand.player, IsPlayer: true})
				} else {
					targets = append(targets, state.Target{Obj: cand.obj})
				}
			}
		}
	}
	if sa != nil && !overloaded {
		// A modal spell's target declaration lives on its announced mode SVar,
		// not the outer Charm SA. Use the same selected declaration targetAsk
		// used during CR 601.2c, so its targets receive the ordinary CR 608.2b
		// legality recheck at resolution. (targetSA is hoisted above.)
		// Fix round 2 (re-review N1), the same correction as the ability
		// branch above, and the one that was actually reachable. Widening the
		// departed-player release hook in fix round 1 turned a stall into a
		// spell that RESOLVES with no targets at all: the caster is
		// eliminated while their target decision is outstanding, the hook
		// clears it so the match can continue, and the spell is left on the
		// stack with Targets nil. Under the old `len(targets) > 0` gate that
		// skipped the recheck and ran the whole script -- untargeted riders
		// included -- gaining the ELIMINATED caster 7 life in the re-review's
		// own reproduction. CR 608.2b counters a spell with no legal targets;
		// CR 800.4a says a departed player's spell ceases to exist. Neither
		// permits it to resolve.
		// Requirement N2, the same exemption as the ability branch: an
		// untargeted-with-Min-0 spell resolves rather than fizzling.
		if spec := targetSA.Params["ValidTgts"]; spec != "" && !(targetMin(targetSA) == 0 && len(targets) == 0) {
			legal := e.legalTargets(targets, spec, targetZones(targetSA), o.Controller, id, id)
			if len(legal) == 0 {
				// CR 608.2b: every target became illegal. This spell does
				// not resolve -- no Resolve event, no script runs -- it goes
				// straight to its normal resting place, the same zone it
				// would reach after an ordinary resolution (an Aura instead
				// reaching the battlefield would be wrong here: CR 704.5m's
				// "nothing legal to attach to" is exactly this case for
				// that spell shape, and the resting zone is where it
				// belongs) -- exile instead of the graveyard for one cast
				// via Flashback (CR 702.32b).
				rest := spellFizzleZone(o)
				e.emit(events.Event{Kind: events.MoveZone, Obj: id,
					From: state.ZStack, To: rest, Text: "fizzled: no legal targets remain"})
				e.ensureLeftTheStack(id, rest, "a replacement fully discarded this "+
					"spell's 'fizzled: no legal targets' move without relocating it anywhere; "+
					"sent to its resting zone instead of re-resolving forever")
				return
			}
			targets = legal
		}
	}
	e.emit(events.Event{Kind: events.Resolve, Obj: id, Text: f.Name})
	if sa != nil {
		e.damaging = id
		ctx := &effects.Ctx{Source: id, Controller: o.Controller, Targets: targets}
		// Same marker as the ability branch: the cast-flow target ask
		// (targetAsk's targetSA) offered exactly this spell's targeting.
		ctx.TargetsOffered = targetSA != nil && strings.TrimSpace(targetSA.Params["ValidTgts"]) != ""
		// CR 107.3i: X is the value the caster chose for the mana cost's {X},
		// recorded on the stack object by commitCast's CastInfo (the same
		// value the ETB/replacement path already reads as o.X). Without this
		// every numeric parameter that reads the paid X (TokenAmount$ X,
		// Amount$ X, SVar:X:Count$xPaid, ...) resolves to 0 and the spell's
		// body does nothing -- Entreat the Angels resolved to the graveyard
		// having created zero Angels.
		ctx.X = o.X
		// Same as the ability branch: carry the sacrifice LKI (engine-keyed)
		// onto resolution so Sacrificed$<Property> heads resolve against what
		// this spell sacrificed.
		ctx.Sacrificed = e.sacrificedLKI[id]
		effects.SetSVars(ctx, f.SVars)
		// CR 601.2b: a modal spell's choice was recorded on its proposal before
		// targets and payment. Pre-seeding Modes makes effCharm execute exactly
		// that announcement instead of posing its old resolution-time ask.
		ctx.Modes = o.ChosenModes
		e.contChain = e.contChain[:0]
		e.repeatReported = nil
		effects.Resolve(e, ctx, sa)
		e.damaging = 0
		if e.resume != nil {
			// The cast-announced outer mode may itself contain an asking effect.
			// This is an initial resolution pass rather than a resume re-entry,
			// but its enclosing SubAbility continuations have the same lifetime.
			e.resume.outer = e.buildContinuationChain(e.contChain, id, nil)
			// A mid-resolution ask (M2d-2): same as the ability branch above
			// — the resolution is suspended with the object still on the
			// stack, and the answered decision re-enters it through
			// resumeResolution instead of this tail.
			return
		}
	}
	e.moveResolvedOffStack(o)
}

// spellRestZone is where a spell goes AFTER IT RESOLVES. Buyback is a
// resolution replacement (CR 702.27a), not a replacement for being
// countered, so only this resolved-spell helper may return it to hand.
func spellRestZone(o *state.Object) state.Zone {
	if o != nil && (o.CastFlags&state.FlagFlashback != 0 || o.CastFlags&state.FlagHarmonize != 0 ||
		o.IsCopy || o.CastFlags&state.FlagAdventure != 0) {
		return state.ZExile
	}
	if o != nil && o.CastFlags&state.FlagBuyback != 0 {
		return state.ZHand
	}
	return state.ZGraveyard
}

// spellFizzleZone is the resting place when a spell never resolved. Flashback,
// Harmonize and copies still use exile, but Buyback does not apply and the
// card reaches its owner's graveyard.
func spellFizzleZone(o *state.Object) state.Zone {
	if o != nil && (o.CastFlags&state.FlagFlashback != 0 || o.CastFlags&state.FlagHarmonize != 0 || o.IsCopy) {
		return state.ZExile
	}
	return state.ZGraveyard
}

// ensureLeftTheStack is CR 608.2m housekeeping, not a further game action:
// every one of resolveTop's five exits (the permanent-ETB Move above, the
// instant/sorcery Move above, and the three fizzle/ability-resolution Moves
// earlier in this function) emits a MoveZone meant to take id off the stack
// for good. If a ValidCard$-matching, ReplacementResult$-absent (this
// build's "Replaced") R:Event$ Moved replacement on some OTHER object
// intercepts that specific MoveZone and its own ReplaceWith$ does not itself
// relocate the card (e.g. it only gains life), nothing else ever removes id
// from e.G.Stack: the next priority round finds the same object on top and
// resolves it again, forever. Note this is NOT the shipped corpus's own
// Rest in Peace / Dryad Militant / Leyline of the Void shape: that shape's
// ReplaceWith$ names Defined$ ReplacedCard, which effects/context.go's
// Defined DOES model, and with ChangeZone's Origin$ All parsed as a real
// wildcard (effects/zone.go's ParseZones) it genuinely relocates the card
// to exile -- so a working Rest-in-Peace-shaped replacement takes the object
// off the stack itself and the guard never engages. The guard is for the
// replacements whose ReplaceWith$ truly leaves the card where it was.
//
// Originally added by Task 29 (Ruling T26-a) for the permanent-ETB exit
// alone; the final whole-branch review (Critical C1) measured the identical
// hazard, independently reachable, on the other four exits -- a two-card
// deck (an instant or an ability-granting permanent, plus a Rest-in-Peace-
// shaped enchantment already on the battlefield) reliably hung a real match
// at 100 000+ intents, never terminating, on turn 1. 24 corpus cards this
// build's own coverage gate already calls fully playable carry the shape
// (Rest in Peace, Dryad Militant, Leyline of the Void, ...), so this is
// reachable from ordinary deck-building, not adversarial card text. This
// helper is what makes every exit safe with one implementation instead of
// five copies of the same six lines.
//
// Held under applyingReplacement (saved and restored, not just set, in case
// a future caller ever reaches here already inside one) for the same reason
// the original ETB guard was (Task 29 review finding I-1): "ceases to
// exist" is engine bookkeeping, not a game event a card's own replacement
// gets to intercept a second time. why becomes the Note's Text, tailored by
// each call site to name which of the five moves was actually being
// guarded.
func (e *Engine) ensureLeftTheStack(id state.ObjID, to state.Zone, why string) {
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZStack {
		return
	}
	// fx44: a SUSPENDED replacement is legitimately still on the stack,
	// waiting for its answer — the ask it posed decides where the object goes
	// (Mox Diamond's ETB replacement parks the card exactly here). The guard
	// must distinguish "the replacement finished and moved nothing" (the
	// ordinary re-resolve hazard below, where e.resume is nil) from "the
	// replacement is waiting for an answer" (where the object must stay put
	// so resumeResolution can act on it). Deferring the park while suspended
	// leaves the object on the stack; the answered resume performs the real
	// move, and finishResumption's own o.Zone != state.ZStack check skips
	// this guard entirely once that move has happened.
	if e.Suspended() {
		return
	}
	saved := e.applyingReplacement
	e.applyingReplacement = true
	e.emit(events.Event{Kind: events.Note, Obj: id, Text: why})
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZStack, To: to})
	e.applyingReplacement = saved
}

// legalTargets is CR 608.2b's recheck, applied at resolution: the subset of
// targets that are still legal right now. An object target must still be in
// a zone the spec legitimately targets -- zones, computed by the same
// targetZones the offering askTarget used, is what no-longer-hardcodes the
// battlefield -- because matchesBase's own bare-type predicates (effects/
// filter.go), e.g. "Creature", read printed types straight off the Face with
// no zone check of their own, so a creature that died and is sitting in a
// graveyard would otherwise still look like a match. The old form required
// o.Zone == state.ZBattlefield, which made every non-battlefield legal
// target offered by askTarget (a stack spell for a counterspell; a
// Graveyard card for the Snapcaster shape) fizzle at resolution. It still
// satisfies spec, via the same effects.MatchesSpec call askTarget used to
// offer it as an option in the first place: no self-relative source,
// matching askTarget's own simplification, so a spec that would filter on
// Self/Other is exactly as (im)precise here as it was at cast time. A player
// target is legal for as long as they are still in the game; askTarget never
// applies MatchesPlayerSpec's finer You/Opponent distinction when it first
// offers every living player as an option (targetsPlayers below it), so this
// does not either -- rechecking against a filter the engine never enforced
// when the target was chosen would reject targets this build always
// considered fine.
func (e *Engine) legalTargets(targets []state.Target, spec string, zones []state.Zone, you state.PlayerID, source state.ObjID, self state.ObjID) []state.Target {
	var legal []state.Target
	// The resolution recheck, unlike a target offer, has this stack object's
	// Targets available. Targeted* predicates may read precisely this binding;
	// setting it here keeps their self-reference unavailable at announcement.
	sc := e.targetSpecContext(0, self, you)
	sc.ResolutionTargets = targets
	sc.Resolving = true
	for _, t := range targets {
		if t.IsPlayer {
			if int(t.Player) < len(e.G.Players) && !e.G.Players[t.Player].Lost {
				legal = append(legal, t)
			}
			continue
		}
		// CR 115.5: the resolving spell or ability is an illegal target for
		// itself. self is the stack object being resolved (not source, which
		// for an ability is the source PERMANENT and so is a legal target of
		// its own ability -- e.g. a creature's "target creature" ability on
		// itself). A target chosen at cast time for a different spell -- a
		// different copy of the same card, or a permanent -- is unaffected.
		// This is why the exclusion is keyed on the resolving object id, and
		// mirrors askTarget's own withholding so the two sites always agree.
		if t.Obj == self {
			continue
		}
		// CR 702.16c: a permanent that became protected from the resolving
		// source's qualities since the target was chosen is no longer a legal
		// target, exactly as askTarget withheld it at cast time -- so a
		// previously-offered target that gained matching protection mid-race
		// is dropped from resolution too (CR 608.2b). The source is resolved
		// through protectionSource so an ability fizzling here judges "the
		// source" as its Source permanent, the same object askTarget's own
		// filter has now been made to see (Critical C2 -- one definition).
		if o := e.G.Obj(t.Obj); o != nil && zoneIn(o.Zone, zones) &&
			effects.MatchesSpecCtx(e.G, targetSpecForZone(spec, o.Zone), t.Obj, sc) &&
			!(o.Zone == state.ZBattlefield && e.restrictionBlocksTarget(t.Obj, you)) &&
			!e.protectedFrom(t.Obj, e.protectionSource(source)) {
			legal = append(legal, t)
		}
	}
	return legal
}

// targetSpecForZone preserves Forge's distinction between a battlefield
// permanent and a permanent card in another zone. Forge spells both with a
// `Permanent` base (Conduit of Worlds is a real `TgtZone$ Graveyard` example),
// while the general matcher correctly treats a bare Permanent as a battlefield
// object. Rewrite only the leading base token, retaining every qualifier, so
// all target offer and target-legality callers share this rule.
func targetSpecForZone(spec string, z state.Zone) string {
	if z == state.ZBattlefield || z == state.ZStack {
		return spec
	}
	if spec == "Permanent" {
		return "PermanentCard"
	}
	if len(spec) > len("Permanent") && spec[:len("Permanent")] == "Permanent" {
		next := spec[len("Permanent")]
		if next == '.' || next == '+' || next == ',' {
			return "PermanentCard" + spec[len("Permanent"):]
		}
	}
	return spec
}

// zoneIn reports whether z is one of the zones in the set.
func zoneIn(z state.Zone, zones []state.Zone) bool {
	for _, candidate := range zones {
		if candidate == z {
			return true
		}
	}
	return false
}

// resolveAbility walks an SA chain, running each API's implementation. svars
// is the resolving face's SVar table (nil for an ability-only stack object),
// which is what lets Num's SVar indirection and primitives like Charm and
// Repeat -- which run a sub-ability named by SVar rather than the
// auto-linked "SubAbility$" -- actually resolve something outside a test.
func (e *Engine) resolveAbility(source state.ObjID, controller state.PlayerID,
	targets []state.Target, sa *cards.SA, svars map[string]string) {
	ctx := &effects.Ctx{Source: source, Controller: controller, Targets: targets}
	effects.SetSVars(ctx, svars)
	effects.Resolve(e, ctx, sa)
}

// Game, Emit, Rand and ShuffleLibrary satisfy effects.Host, which is how effects reach the
// engine without importing it. AddContinuous (layers.go), HasKeyword
// (layers.go) and Ask (resolution.go) round out the interface -- HasKeyword
// already existed for the layer system's own callers before effects.Host
// grew a method of the same name, and needed no change to satisfy it.
func (e *Engine) Game() *state.Game                       { return e.G }
func (e *Engine) Emit(ev events.Event)                    { e.emit(ev) }
func (e *Engine) EmitDamage(ev events.Event) events.Event { return e.emit(ev) }
func (e *Engine) Rand(n int) int                          { return e.rng.IntN(n) }

// ShuffleLibrary is the single library-shuffle path used by rules and effects.
// State changes only when the caller emits the resulting Shuffle event.
func (e *Engine) ShuffleLibrary(player state.PlayerID, order []state.ObjID) []state.ObjID {
	out := append([]state.ObjID(nil), order...)
	s := e.rng.chance
	if s == nil || s.planner == nil {
		e.rng.Shuffle(out)
		return out
	}
	ordinal := s.shuffleOrdinals[player]
	s.shuffleOrdinals[player] = ordinal + 1
	ctx := ShuffleContext{Player: player, Ordinal: ordinal, Library: e.shuffleCards(out), Hand: e.shuffleCards(e.G.Zone(state.ZHand, player))}
	desired, err := s.planner(ctx)
	if err != nil {
		s.fail(fmt.Errorf("hypothetical shuffle planner: %w", err))
	}
	if desired == nil {
		e.rng.Shuffle(out)
		return out
	}
	if err := e.rng.forcePermutation(out, desired); err != nil {
		s.fail(err)
	}
	return out
}

// EmitTap satisfies effects.Host's EmitTap: see emitTap.
func (e *Engine) EmitTap(obj state.ObjID, tapper state.PlayerID, entering bool) {
	e.emitTap(obj, tapper, entering)
}

// emitTap emits the plain Tap event for obj while its provenance -- who tapped
// it, and whether it is only being given its entry state -- is visible to the
// Taps/TapsForMana matcher and to trigger referents. The event payload is the
// same one every Tap producer emitted before, so no chain head moves for a
// game without such a trigger. The previous context is restored rather than
// zeroed, so a Tap emitted from inside another Tap's trigger matching cannot
// clobber the outer one.
func (e *Engine) emitTap(obj state.ObjID, tapper state.PlayerID, entering bool) {
	savedObj, savedPlayer, savedEntering := e.tapObj, e.tapPlayer, e.tapEntering
	e.tapObj, e.tapPlayer, e.tapEntering = obj, tapper, entering
	e.emit(events.Event{Kind: events.Tap, Obj: obj})
	e.tapObj, e.tapPlayer, e.tapEntering = savedObj, savedPlayer, savedEntering
}

// LegalTargets satisfies effects.Host for target-changing effects. It exposes
// the same census used by cast and trigger target decisions, so a redirect
// cannot bypass protection, CantTarget, stack-kind, zone, or filter legality.
func (e *Engine) LegalTargets(chooser state.PlayerID, source state.ObjID, sa *cards.SA) []state.Target {
	cs := e.legalTargetCandidates(chooser, source, source, sa)
	out := make([]state.Target, 0, len(cs))
	for _, c := range cs {
		if c.kind == "player" {
			out = append(out, state.Target{Player: c.player, IsPlayer: true})
		} else {
			out = append(out, state.Target{Obj: c.obj})
		}
	}
	return out
}

// CastThisTurn satisfies effects.Host's CastThisTurn for Count$ThisTurnCast
// (Task 17/Storm): the spells cast this turn by ANY player, counted from
// the same event log spellsCastThisTurn reads, so a replay that rebuilds the
// game derives the identical number -- never a live-only engine counter.
// Storm subtracts one (Count$ThisTurnCast/Minus1) because the resolving
// spell's own PutOnStack is already in the log by the time its trigger
// effect runs, which would otherwise overcount by exactly one.
func (e *Engine) CastThisTurn() int {
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.PutOnStack {
			n++
		}
	}
	return n
}

// SpellsCastThisTurnMatching satisfies effects.Host's
// SpellsCastThisTurnMatching for Count$ThisTurnCast_<spec> (the
// "first/second spell you cast" cost modifiers and triggers): spells put on
// the stack this turn whose object matches the Forge spec. When the spec
// carries a You* qualifier the count scopes to YOU's casts; otherwise it
// counts everyone's. Derived from the event log like CastThisTurn.
func (e *Engine) SpellsCastThisTurnMatching(you state.PlayerID, spec string) int {
	youScoped := strings.Contains(spec, "You")
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind != events.PutOnStack {
			continue
		}
		if youScoped && ev.Player != you {
			continue
		}
		if effects.MatchesSpecFrom(e.G, spec, ev.Obj, you, ev.Obj) {
			n++
		}
	}
	return n
}

// LifeLostThisTurn satisfies effects.Host's LifeLostThisTurn for
// Count$LifeOppsLostThisTurn (Rakdos, Lord of Riots' cost reduction): the
// total life p lost this turn, summed from every LifeChange below zero since
// the last TurnChange. Derived from the event log like CastThisTurn, so a
// replay that rebuilds the game arrives at the same number. Life GAINED is
// not folded in — "lost life" is a loss even if the player ended the turn
// higher than they started (CR 118.3's distinction, and the reading Forge's
// own head takes).
func (e *Engine) LifeLostThisTurn(p state.PlayerID) int32 {
	var n int32
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.LifeChange && ev.Player == p && ev.Amount < 0 {
			n += -ev.Amount
		}
	}
	return n
}

// LifeGainedThisTurn satisfies effects.Host's LifeGainedThisTurn for
// Count$LifeYouGainedThisTurn (the "At the beginning of each end step, if you
// gained 4 or more life this turn" family's CheckSVar$ gate — Angelic Accord,
// Resplendent Angel, Valkyrie Harbinger): the total life p gained this turn,
// summed from every LifeChange above zero since the last TurnChange. Derived
// from the event log like LifeLostThisTurn, so a replay that rebuilds the
// game arrives at the same number. Life LOST is not folded in — "gained
// life" counts only positive LifeChanges (CR 118.3's distinction, the same
// one-sided read LifeLostThisTurn takes in the other direction).
func (e *Engine) LifeGainedThisTurn(p state.PlayerID) int32 {
	var n int32
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.LifeChange && ev.Player == p && ev.Amount > 0 {
			n += ev.Amount
		}
	}
	return n
}

// TurnsTaken satisfies effects.Host's TurnsTaken for Count$YourTurns (Serra
// Avenger's "your first, second, or third turns of the game"): the number of
// turns that have BEGUN with p as the active player, current turn included.
// Derived from the event log like LifeLostThisTurn — every turn p begins
// emits exactly one TurnChange naming p (events.Apply's TurnChange case),
// extra turns included, so a replay that rebuilds the log arrives at the
// same count. The whole-log walk (not a TurnChange-bounded scan) is the
// point: the count spans the game, not one turn.
func (e *Engine) TurnsTaken(p state.PlayerID) int32 {
	if int(p) >= len(e.G.Players) {
		return 0
	}
	if len(e.turnsTaken) != len(e.G.Players) || e.turnsTakenEpoch != len(e.L.Events) {
		e.turnsTaken = make([]int32, len(e.G.Players))
		for _, ev := range e.L.Events {
			if ev.Kind == events.TurnChange && int(ev.Player) < len(e.turnsTaken) {
				e.turnsTaken[ev.Player]++
			}
		}
		e.turnsTakenEpoch = len(e.L.Events)
	}
	return e.turnsTaken[p]
}

// AttackersThisTurn satisfies effects.Host's AttackersThisTurn for
// Count$AttackersDeclared (the Raid family's "attacked this turn" read): the
// number of attackers declared this turn, summed from every DeclareAttackers
// event's attacker list since the last TurnChange. Derived from the event log
// like CastThisTurn, so a replay that rebuilds the game arrives at the same
// number. A DeclareAttackers event carries its declared attackers in IDs (one
// event per defender); an event with no IDs contributes nothing.
func (e *Engine) AttackersThisTurn() int {
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.DeclareAttackers {
			n += len(ev.IDs)
		}
	}
	return n
}

// targetsPlayers and targetsPermanents read the coarse shape of a ValidTgts
// spec. The per-object predicate work is effects.MatchesSpec.

// targetsPlayers reports whether a spec can name a player as a target, by
// looking at each alternative's BASE type (the part before any "."), never
// a substring scan: "Any" targets either an object or a player, "Player"/
// "Opponent"/"You" a player, while a predicate like "Creature.YouCtrl" IS an
// object filter (its ".YouCtrl" clause scopes the creature's controller,
// not the target being a player). The old substring form matched the "You"
// inside "YouCtrl" and wrongly offered players as Equip targets (Task 14
// found it: an Equip onto Creature.YouCtrl offered both players as options,
// which effAttach then had to refuse).
func targetsPlayers(spec string) bool {
	for _, alt := range strings.Split(spec, ",") {
		switch base, _, _ := strings.Cut(strings.TrimSpace(alt), "."); base {
		case "Player", "Any", "Opponent", "You":
			return true
		}
	}
	return false
}

func targetsPermanents(spec string) bool {
	for _, t := range [...]string{"Creature", "Any", "Permanent", "Artifact",
		"Enchantment", "Land", "Planeswalker", "Card"} {
		if strings.Contains(spec, t) {
			return true
		}
	}
	return false
}

// payUnlessCost charges the non-choice subset of a mid-resolution
// UnlessCost$ to payer p. Sacrifice and discard components are deliberately
// refused here: beginUnlessPayment owns every such component and gathers the
// payer's selected objects before it calls payMana. Keeping this guard makes
// a future caller unable to silently revive the old first-in-zone-order
// stand-in. Fixed mana/life, SubCounter and Draw components remain
// synchronous: a Draw<N/Spec> pays by drawing N cards for the player(s) the
// spec names (default the payer), resolved through the same Ctx roles the
// UnlessPayer$ grammar reads.
func (e *Engine) payUnlessCost(p state.PlayerID, cost Cost, ctx *effects.Ctx, stackObj state.ObjID) bool {
	if len(cost.Sac) != 0 || len(cost.Discard) != 0 {
		return false
	}
	if int(p) < 0 || int(p) >= len(e.G.Players) {
		return false
	}
	g := e.G
	// The source the SubCounter parts drain is the activated ability's host
	// when this is an ability object, otherwise the resolving source.
	src := ctx.Source
	if o := g.Obj(stackObj); o != nil && o.Ability != nil {
		src = o.Source
	}
	type counterDrain struct {
		obj  state.ObjID
		kind string
		n    int32
	}
	var drains []counterDrain
	for _, part := range cost.SubCounter {
		o := g.Obj(src)
		if o == nil {
			return false
		}
		have := int32(0)
		for _, ct := range o.Counters {
			if ct.Kind == part.Spec {
				have += ct.N
			}
		}
		if have < part.N {
			return false
		}
		drains = append(drains, counterDrain{obj: o.ID, kind: part.Spec, n: part.N})
	}
	// Resolve every drawer before charging mana/life. A Draw component whose
	// role is unavailable makes the entire cost unpayable; validating first
	// avoids a partial payment followed by a silent omitted draw.
	drawers := make([][]state.PlayerID, len(cost.Draw))
	for i, part := range cost.Draw {
		players, ok := unlessDrawPlayers(ctx, p, part.Spec)
		if !ok {
			return false
		}
		for _, dp := range players {
			if int(dp) < 0 || int(dp) >= len(g.Players) {
				return false
			}
		}
		drawers[i] = players
	}
	// Everything is affordable: charge mana/life through ordinary events,
	// then apply the synchronous counter components, then the draws.
	if !e.payMana(p, cost) {
		return false
	}
	for _, d := range drains {
		e.emit(events.Event{Kind: events.CounterChange, Obj: d.obj, Counter: d.kind, Amount: -d.n})
	}
	for i, part := range cost.Draw {
		for _, dp := range drawers[i] {
			for n := int32(0); n < part.N; n++ {
				effects.DrawFor(e, dp)
			}
		}
	}
	return true
}

// unlessDrawPlayers resolves a Draw<N/Spec> cost component's drawer(s). The
// empty spec and "You" are the payer; every other spelling is one of the
// player roles the unless-payment context carries, and an unresolvable or
// unknown spec fails closed (the cost was not paid).
func unlessDrawPlayers(ctx *effects.Ctx, payer state.PlayerID, spec string) ([]state.PlayerID, bool) {
	one := func(t state.Target) ([]state.PlayerID, bool) {
		if t.IsPlayer {
			return []state.PlayerID{t.Player}, true
		}
		return nil, false
	}
	switch spec {
	case "", "You", "Player", "Self":
		return []state.PlayerID{payer}, true
	case "Player.targetedBy", "Targeted", "TargetedPlayer":
		if len(ctx.Targets) == 0 {
			return nil, false
		}
		return one(ctx.Targets[0])
	case "Player.Activator", "TriggeredActivator":
		return one(ctx.TriggerActivator)
	case "Player.TriggeredPlayer", "TriggeredPlayer":
		return one(ctx.TriggerPlayer)
	case "Player.TriggeredTarget", "TriggeredTarget":
		return one(ctx.TriggerTarget)
	}
	return nil, false
}
