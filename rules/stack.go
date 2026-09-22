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
	ok, _, _, _, _ := e.payManaForSpent(p, id, ability, cost, conv, rider)
	return ok
}

// payManaForSpent is payManaFor with the payment's actually-spent mana
// returned: the per-colour delta the negative ManaAdd events record (zero on
// a failed payment). FOUR deltas come back: `spentPlain` is the split the
// payment emits (restricted batches already carved off by
// emitRestrictedManaSpend, the delta RememberCostMana$ notes today), `spentAll`
// is the FULL pool delta copied before that split -- the true "all mana spent
// to pay this cost", which converge (CR 107.4f-family) counts from via
// payManaCastSpent -- `spentSnow` is the per-colour count of the spent
// units that were SNOW units (CR 107.4h), the parallel-tally delta, and
// `spentTyped` is the per-tag per-colour count of the spent units that were
// TYPED units (task castfilter2: Treasure/Cave/Desert, the parallel
// Player.TypedMana tally), which the filtered Count$CastTotalManaSpent
// Treasure/Cave/Desert heads read. The RememberCostMana$ payment site
// (Jeweled Amulet) keeps its existing split-based note; every other caller
// keeps the bool-only payManaFor wrapper, so no other payment site changes
// shape.
func (e *Engine) payManaForSpent(p state.PlayerID, id state.ObjID, ability bool, cost Cost, conv *manaConv, rider pipRider) (bool, state.Mana, state.Mana, state.Mana, [3]state.Mana) {
	av := e.manaAvailableFor(p, id, ability)
	// The payment's persistence attribution: the visible pool's persistent
	// share (perVis) and its ordinary complement (perFresh). resolveMana is
	// persistence-blind — the units are interchangeable — so attributing the
	// spent units ordinary-first (the exception mana spent last) is a
	// bookkeeping choice the emitted events carry: the split below emits the
	// persistent remainder as a MARKED negative ManaAdd whose " pm" suffix is
	// what moves Player.PersistentMana in events.Apply. Carrying the
	// attribution on the events, rather than letting the fold derive it from
	// the RAW pool (whose fresh share disagrees with this visible pool
	// whenever a restricted batch is hidden from the payment, or the carve
	// consumed the persistent batch first), is what keeps the tally on the
	// units that actually survived a boundary.
	perVis := e.visiblePersistentMana(p, id, ability)
	perFresh := state.Mana{}
	for i := range perFresh {
		perFresh[i] = av.pool[i] - perVis[i]
	}
	before := av.pool
	beforeSnow := e.G.Players[p].Snow
	beforeTyped := av.typed
	pay, ok := cost.resolveManaWith(before, beforeSnow, beforeTyped, e.G.Players[p].Life,
		e.payerGrantsPayLifeInsteadOfB(p), rider, conv)
	if !ok {
		return false, state.Mana{}, state.Mana{}, state.Mana{}, [3]state.Mana{}
	}
	after, afterSnow, afterTyped, lifeSpent := pay.pool, pay.snow, pay.typed, pay.lifeSpent
	spent := state.Mana{}
	spentSnow := state.Mana{}
	spentTyped := [3]state.Mana{}
	for i := range before {
		spent[i] = before[i] - after[i]
		// The parallel tallies' own deltas: how many of the units that left
		// slot i were snow / typed units. resolveManaWith consumes a plain
		// unit before a typed one and a typed one before snow wherever a
		// choice existed, so this is exactly what the payment search did and
		// each tally never exceeds spent[i].
		spentSnow[i] = beforeSnow[i] - afterSnow[i]
		for t := range spentTyped {
			spentTyped[t][i] = beforeTyped[t][i] - afterTyped[t][i]
		}
	}
	// Converge counts ALL mana spent, restricted batches included -- Boseiju's
	// {C} is not a colour, but a Tazri-restricted coloured unit IS the colour
	// it was paid as -- so copy the full delta before emitRestrictedManaSpend
	// carves the restricted batches out of `spent`.
	spentAll := spent
	// emitSnow/emitTyped are the emission split's copies of the tallies.
	// emitRestrictedManaSpend carves the restricted batches out of `spent`
	// and emits their tagged/snow form DIRECTLY, so the tallies driving the
	// remaining split must be carved alongside it -- otherwise a restricted
	// TAGGED unit would be emitted once by the carve and again by the loop
	// (double-decrementing TypedMana and the pool). The returned spentSnow /
	// spentTyped keep the FULL deltas the pay-time capture reads.
	emitSnow := spentSnow
	emitTyped := spentTyped
	e.emitRestrictedManaSpend(p, id, ability, &spent, &emitSnow, &emitTyped, &perVis, &perFresh)
	for i, letter := range manaLetters {
		if spent[i] == 0 {
			continue
		}
		// A slot whose snow / typed units were spent (all or part) emits the
		// "S<colour>" / "<Tag><colour>" Counter forms so the parallel tallies
		// move with the pool through the same events the adds used.
		// resolveMana consumes a plain unit before a typed one and a typed
		// one before snow wherever a choice existed, so the split here is
		// exactly what the payment search did. The emission order (plain,
		// Treasure, Cave, Desert, snow) is fixed and deterministic.
		snowSpent := emitSnow[i]
		typedSpent := int32(0)
		for t := range emitTyped {
			typedSpent += emitTyped[t][i]
		}
		if plain := spent[i] - snowSpent - typedSpent; plain > 0 {
			// The slot's ordinary units are attributed first (the exception
			// mana is spent last); the plain share past perFresh is the
			// persistent remainder and rides a MARKED event so the fold moves
			// the tally with the units that were actually consumed. The carve
			// above already charged its own consumption against perVis/perFresh
			// by the consumed batch's flag, so the marked remainder can never
			// exceed the visible persistent share.
			ord := plain
			if perFresh[i] < ord {
				ord = perFresh[i]
			}
			if ord > 0 {
				e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: letter, Amount: -ord})
			}
			if per := plain - ord; per > 0 {
				e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: letter,
					Amount: -per, Text: events.ManaPersistentText("")})
			}
		}
		for t, tag := range state.TypedManaTags {
			if emitTyped[t][i] > 0 {
				e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: tag + letter, Amount: -emitTyped[t][i]})
			}
		}
		if snowSpent > 0 {
			e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: "S" + letter, Amount: -snowSpent})
		}
	}
	// Fixed life costs and any Phyrexian pips paid with life are deducted
	// through the ordinary LifeChange event so a replay learns them.
	if lifeSpent != 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: p, Amount: -lifeSpent})
	}
	return true, spentAll, spent, spentSnow, spentTyped
}

// payManaCastSpent is the spell-cost payment (the shared payManaFor core
// with the cast's recorded may-play ignore-colour rider, CR 401.5's "spend
// mana as though it were mana of any color to cast it") returning the FULL
// spent delta: the pre-restriction-split per-colour pool delta converge
// counts from (task converge1), the snow-unit delta the filtered
// Count$CastTotalManaSpent Snow head reads (task castfilter1), and the
// per-tag typed deltas the filtered Treasure/Cave/Desert heads read (task
// castfilter2). The spell arm's only ask stages have all completed by
// payment, so the deltas ride pendingCast plain data to the pay-time
// CastInfo exactly like replicateTimes does. The rider was proved by the
// offer gate while the card still sat in the granted zone; the payment
// keeps it via pc.mayPlayIgnore because after the push (CR 601.2a) the card
// is on the stack and a zone re-derivation would wrongly drop the grant.
func (e *Engine) payManaCastSpent(pc *pendingCast, cost Cost) (bool, state.Mana, state.Mana, [3]state.Mana) {
	ok, spentAll, _, spentSnow, spentTyped := e.payManaForSpent(pc.player, pc.card, false, cost, e.paymentConv(pc.player, pc.card, false),
		pipRider{anyColor: pc.mayPlayIgnore, anyType: pc.mayPlayIgnoreType})
	return ok, spentAll, spentSnow, spentTyped
}

// payExtortPip charges the {W/B} hybrid pip (one mana of either W or B)
// from p's pool, emitting the ManaAdd events so a replay re-derives it. It
// returns false (and charges nothing) when the pool has neither colour, so
// an Extort payment a player genuinely cannot make is a decline rather than
// a free drain.
func (e *Engine) payExtortPip(p state.PlayerID) bool {
	// The pip is charged from the raw pool (the extort window carries no
	// payment id, so the restriction-aware view does not apply here — the
	// pre-existing, restriction-blind consumption is unchanged), but its
	// attribution follows the shared payment rule (ordinary units first): a
	// slot's persistent share is charged only past its non-persistent one,
	// and the persistent unit's event carries the " pm" marker so the fold
	// moves the tally with the unit actually consumed.
	pl := e.G.Players[p]
	fresh := pl.Pool
	per := pl.PersistentMana
	for i := range fresh {
		fresh[i] -= per[i]
	}
	for _, idx := range []int{state.MW, state.MB} {
		if fresh[idx] > 0 {
			e.emit(events.Event{Kind: events.ManaAdd, Player: p,
				Counter: manaLetters[idx], Amount: -1})
			return true
		}
	}
	for _, idx := range []int{state.MW, state.MB} {
		if per[idx] > 0 {
			e.emit(events.Event{Kind: events.ManaAdd, Player: p,
				Counter: manaLetters[idx], Amount: -1,
				Text: events.ManaPersistentText("")})
			return true
		}
	}
	return false
}

// availableMana is the restriction-aware view of a seat's floating mana:
// the pool minus every RestrictValid$ batch this payment cannot use, and the
// typed producer tallies with those same batches removed. A restricted TYPED
// unit (Echoing Cavern's Cave mana) must be invisible in BOTH: takeUnit
// partitions a slot by the typed tally, so a typed unit left visible under
// an unusable restriction could be consumed through that path although the
// filter already hid it from the pool.
type availableMana struct {
	pool  state.Mana
	typed [3]state.Mana
}

// manaAvailableFor removes every restricted batch from the visible pool, then
// restores exactly the batches valid for this payment. This means a cast or a
// nonmatching activation can never borrow Tazri-style mana merely because it
// shares a colour bucket with unrestricted mana. The typed tallies are
// filtered by the same rule, so a typed restricted unit can never be spent
// through the typed consumption path either.
func (e *Engine) manaAvailableFor(p state.PlayerID, id state.ObjID, ability bool) availableMana {
	pl := e.G.Players[p]
	available := availableMana{pool: pl.Pool, typed: pl.TypedMana}
	for _, r := range pl.RestrictedMana {
		idx := state.ManaSlot(r.Color)
		available.pool[idx] -= r.Amount
		// An empty Valid is an UNRESTRICTED batch that carries only its
		// AddsNoCounter$ provenance (Boseiju's plain {C}): it pays anything,
		// exactly like ordinary pool mana, so its units stay visible.
		if r.Valid == "" || e.restrictValidMatches(p, id, ability, r.Valid, r.Source) {
			available.pool[idx] += r.Amount
			continue
		}
		// The batch is unusable here: hide its typed provenance too.
		if tag, slot, ok := state.TypedManaCounter(r.Color); ok {
			available.typed[tag][slot] -= r.Amount
		}
	}
	return available
}

// visiblePersistentMana is the persistent share of p's VISIBLE pool for this
// payment: the PersistentMana tally minus every persistent restriction batch
// this payment cannot use. manaAvailableFor hides an unusable batch's units
// from the payment, so those units cannot be what a spend here consumed and
// must not be attributed to it; the same hiding rule keeps this view and the
// visible pool from disagreeing. (An empty Valid is an unrestricted
// AddsNoCounter batch — spendable anywhere — so its units stay attributed.)
// Measured corpus: every PersistentMana carrier produces plain mana, so the
// persistent share never carries a snow/typed tag in practice.
func (e *Engine) visiblePersistentMana(p state.PlayerID, id state.ObjID, ability bool) state.Mana {
	pl := e.G.Players[p]
	per := pl.PersistentMana
	for _, r := range pl.RestrictedMana {
		if !r.Persistent || (r.Valid != "" && e.restrictValidMatches(p, id, ability, r.Valid, r.Source)) {
			continue
		}
		idx := state.ManaSlot(r.Color)
		if per[idx] < r.Amount {
			per[idx] = 0
		} else {
			per[idx] -= r.Amount
		}
	}
	return per
}

// emitRestrictedManaSpend consumes matching restriction batches in insertion
// order before ordinary mana. Every matching unit is interchangeable for the
// current payment; using this fixed order keeps the log deterministic. When a
// consumed batch carries AddsNoCounter$ provenance and this is a SPELL cast
// payment (never an ability activation), the cast's id is captured in
// e.noCounterSpend for payCast to fold state.FlagNoCounter into the pay-time
// CastInfo — with the batch's own condition evaluated against the paying
// spell's face (Boseiju's !Permanent).
//
// The carve is capped by the units resolveMana's search ACTUALLY attributed to
// the batch's provenance, not merely by the slot's total delta: the search's
// takeUnit consumes a plain unit before a typed one and a typed one before
// snow, so a restricted TAGGED batch beside plain mana of the same colour can
// go entirely unspent even though the slot's delta exceeds its amount. Taking
// the full min(spent[idx], r.Amount) would decrement the tag's emission tally
// below what was spent, driving emitTyped negative; the split loop's
// `plain := spent - snow - typed` subtraction would then be inflated by the
// negative term and the pool would lose more units than the cost required.
// Capping at the tag's (or snow tally's, or the slot's remaining plain units')
// actual spend reconciles the carve's restricted-first attribution with the
// search's plain-first consumption and keeps every emission tally >= 0.
func (e *Engine) emitRestrictedManaSpend(p state.PlayerID, id state.ObjID, ability bool, spent *state.Mana, emitSnow *state.Mana, emitTyped *[3]state.Mana, perVis *state.Mana, perFresh *state.Mana) {
	e.noCounterSpend = 0
	e.manaSpentSources = nil
	// Emit mutates RestrictedMana through events.Apply, so range a snapshot:
	// otherwise removing the first of two matching batches would make the
	// live slice shift under this loop and could skip or double-spend one.
	batches := append([]state.ManaRestriction(nil), e.G.Players[p].RestrictedMana...)
	for _, r := range batches {
		if r.Amount <= 0 || (r.Valid != "" && !e.restrictValidMatches(p, id, ability, r.Valid, r.Source)) {
			continue
		}
		idx := state.ManaSlot(r.Color)
		used := spent[idx]
		if used > r.Amount {
			used = r.Amount
		}
		// Cap by the provenance the search actually spent in this slot. A
		// tagged or snow batch can only carve the units whose parallel tally
		// left the pool; a plain batch only the slot's remaining plain units
		// (spent minus every tally still attributed to this slot).
		if tag, slot, ok := state.TypedManaCounter(r.Color); ok {
			if emitTyped[tag][slot] < used {
				used = emitTyped[tag][slot]
			}
		} else if len(r.Color) == 2 && r.Color[0] == 'S' {
			if s := emitSnow[state.ManaIndex(r.Color[1])]; s < used {
				used = s
			}
		} else {
			plain := spent[idx]
			for t := range emitTyped {
				plain -= emitTyped[t][idx]
			}
			plain -= emitSnow[idx]
			if plain < used {
				used = plain
			}
		}
		if used <= 0 {
			continue
		}
		if r.NoCounter != "" && !ability && e.noCounterSpend == 0 && addsNoCounterHolds(e.G, id, r.NoCounter) {
			e.noCounterSpend = id
		}
		// A consumed batch's producing source is what the spell's
		// TriggersWhenSpent$ riders key on. Only a SPELL payment (the
		// ability=false arm -- payManaCastSpent is its only caller) records
		// it: an ability activation, the unless-pay arm and every other
		// payment fire nothing (the rider is a cast-spend gate). Dedup keeps
		// one entry per source when several batches from it pay one cast; the
		// insertion-order append keeps the queue deterministic.
		if !ability && r.Source != 0 && !containsObjID(e.manaSpentSources, r.Source) {
			e.manaSpentSources = append(e.manaSpentSources, r.Source)
		}
		e.emit(events.Event{Kind: events.ManaAdd, Player: p, Counter: r.Color, Amount: -used,
			Text: events.ManaRestrictionText(r.Valid, r.Source)})
		spent[idx] -= used
		// The consumed batch's own flag charges the persistence attribution:
		// a persistent batch's units were the slot's persistent share (the
		// fold moves Player.PersistentMana by the batch flags on this event),
		// an ordinary batch's were the ordinary share. events.Apply reduces
		// the SAME batches in the same insertion order, so the two views of
		// which units were consumed cannot disagree.
		if r.Persistent {
			if (*perVis)[idx] < used {
				(*perVis)[idx] = 0
			} else {
				(*perVis)[idx] -= used
			}
		} else {
			if (*perFresh)[idx] < used {
				(*perFresh)[idx] = 0
			} else {
				(*perFresh)[idx] -= used
			}
		}
		// Carve the consumed units out of the emission split's tallies too:
		// the carve emitted this batch's tagged/snow form directly, so the
		// split loop must not emit it a second time. The caps above guarantee
		// neither tally can go below zero.
		if tag, slot, ok := state.TypedManaCounter(r.Color); ok {
			emitTyped[tag][slot] -= used
		} else if len(r.Color) == 2 && r.Color[0] == 'S' {
			emitSnow[state.ManaIndex(r.Color[1])] -= used
		}
	}
}

// containsObjID reports whether id is already in ids (a small linear scan;
// the list holds at most a handful of mana-production sources per cast).
func containsObjID(ids []state.ObjID, id state.ObjID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
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
	// The bare wasCastFromYourHand qualifier (castprov3, Mm'menon's
	// RestrictValid$ Spell.!wasCastFromYourHand — "spend this mana only to
	// cast a spell from anywhere other than your hand"): split the
	// provenance out before the filter match, through the pending-cast
	// variant — the offer-side affordability walk (castable → costPayable)
	// evaluates this read PRE-push, where the object has no cast in the log
	// and the negated spelling would wrongly hold, offering a hand cast as
	// payable on mana the payment then refuses. Off the stack the spec
	// denies (this function's own fail-closed convention: restricted mana is
	// never spent illegally — here it is never even counted); at the payment
	// the spell is on the stack and the read is honest. The term's spec is
	// BASE-LESS here (the "Spell." class was already cut off) and the strip
	// helpers rejoin onto a base, so evaluate the Card.-prefixed form; a
	// surviving alternative whose only predicate was the provenance token
	// rejoins to bare "Card", which MatchesSpecFrom matches like any card.
	spec, ok = e.castProvenanceAdmitsPending("Card."+spec, id, p)
	if !ok {
		return false
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
	av := e.manaAvailableFor(p, id, ability)
	_, ok := cost.resolveManaWith(av.pool, e.G.Players[p].Snow, av.typed,
		e.G.Players[p].Life, e.payerGrantsPayLifeInsteadOfB(p), rider, e.paymentConv(p, id, ability))
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
func (e *Engine) costPayablePool(p state.PlayerID, id state.ObjID, ability bool, cost Cost, pool state.Mana, typed [3]state.Mana) bool {
	_, ok := cost.resolveManaWith(pool, e.G.Players[p].Snow, typed, e.G.Players[p].Life,
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
//
// This is the LITERAL reader only. The dynamic forms the corpus writes as
// TargetMax$ X / TargetMin$ X (with SVar:X:Count$...) resolve through
// resolvedTargetBounds below; a token that is not a literal is silently
// dropped here, which is today's (and the unresolvable-fallback's) semantics.
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

// isLiteralBound reports whether a raw TargetMin$/TargetMax$ token is a
// plain signed integer.
func isLiteralBound(v string) bool {
	_, err := strconv.Atoi(strings.TrimSpace(v))
	return err == nil
}

// targetBoundsDynamic reports whether either bound token is present and not
// a literal -- the only shape resolvedTargetBounds does extra work for, so a
// literal-only script never leaves the byte-identical fast path. (The two
// reads are spelled out rather than looped over the keys so the paramcensus
// rot guard sees two static Params keys.)
func targetBoundsDynamic(sa *cards.SA) bool {
	if v, ok := sa.Params["TargetMin"]; ok && !isLiteralBound(v) {
		return true
	}
	if v, ok := sa.Params["TargetMax"]; ok && !isLiteralBound(v) {
		return true
	}
	return false
}

// targetBoundCtx binds the effects numeric grammar to the asking player and
// the target declaration's source. The anchor follows what source IS: for a
// spell the stack object IS the card, so its face carries the SVar table
// (Kiora's Dismissal's SVar:X); for an ability or trigger wrapper
// (o.Card == nil, so Face() returns nil) the anchor is o.Source -- the
// source permanent every TriggerPush/AbilityPush stamps -- and ITS face's
// SVar table. No anchor (the object gone, or a sourceless wrapper) fails
// closed to the literal reader.
func (e *Engine) targetBoundCtx(p state.PlayerID, source state.ObjID) (*effects.Ctx, bool) {
	o := e.G.Obj(source)
	if o == nil {
		return nil, false
	}
	ctx := &effects.Ctx{Controller: p}
	// A trigger's dynamic target bound reading the causing event (Vitality
	// Hunter's `TargetMax$ MaxTgts` with `SVar:MaxTgts:TriggerCount$Amount`,
	// task agent-20260919T190014Z): the trigger context recorded for this
	// stack wrapper carries TriggerAmount, so the bound reads the mark/damage
	// magnitude instead of degrading to the clamp's 1. Measured corpus: the
	// ONLY two TargetMax$ TriggerCount$Amount shapes (one inline, one behind
	// the MaxTgts SVar name) are Vitality Hunter's; every other dynamic bound
	// names a Count$ body targetBoundCtx's SVar table already resolves.
	if tc, ok := e.triggerContexts[source]; ok {
		ctx.TriggerContext = tc
	}
	// The pending cast's own multikicker count (rules/cast.go's multikickAsk):
	// at the CR 601.2c announcement ask the pay-time CastInfo has not run
	// yet, so a TimesKicked bound (Comet Storm's TargetMin/Max$ TargetsNum)
	// would read 0 off the stack object. When the asking source IS the card
	// the pending cast is casting, seed the count the ask just settled --
	// exactly the `x` resolvedTargetBounds threads for a Count$xPaid bound.
	if pc := e.cast; pc != nil && pc.card == source && pc.multikickSet {
		ctx.TimesKicked = pc.multikickTimes
	}
	if f := o.Face(); f != nil {
		ctx.Source = source
		effects.SetSVars(ctx, f.SVars)
		return ctx, true
	}
	src := e.G.Obj(o.Source)
	if src == nil {
		return nil, false
	}
	if f := src.Face(); f == nil {
		return nil, false
	}
	ctx.Source = o.Source
	// A mutated pile's under-card triggered ability (CR 702.140d) reads its
	// OWN face's SVar table, not the pile's top card's: Archipelagoe and
	// Nethroi, Apex of Death both bound their targeting with TargetMax$ X,
	// and the pile's top card can be any creature (with no X at all). The
	// owning face comes from the compiled trigger pointer; an ordinary
	// trigger's owning face is the top face, so nothing else moves. A
	// HAS-ALL-ABILITIES-OF wrapper (r3) is covered inside the recovery
	// functions themselves, so every caller shares the one read.
	if _, mf, ok := e.findTriggerForAbilityFace(o.Source, o.Ability); ok && mf != nil {
		effects.SetSVars(ctx, mf.SVars)
	} else if mf, ok := e.pileFaceForSA(o.Source, o.Ability); ok && mf != nil {
		// An activated ability of a MUTATED pile (CR 702.140d): the ask's SVar
		// bounds (TargetMin$/TargetMax$ X) resolve against the under-card's own
		// table, the same owning-face rule resolveTop's ability branch applies.
		effects.SetSVars(ctx, mf.SVars)
	} else {
		effects.SetSVars(ctx, src.Face().SVars)
	}
	return ctx, true
}

// resolvedTargetBounds is targetBounds extended to the dynamic bounds the
// corpus writes as TargetMax$ X / TargetMin$ X with an SVar body (212 raw
// TargetMax$ X lines / 209 files, 102 TargetMin$ X lines / 100 files -- the
// dominant shape is TargetMin$ 0 + TargetMax$ X, "return any number up to
// X"). A bound token that is a plain literal keeps targetBounds' reading
// byte-for-byte; a token that is PRESENT and not a literal resolves through
// the effects numeric grammar (NumResolved: an SVar name, an inline
// Count$/... expression, or the bare X bound to ctx.X), bound to the asking
// player and the source anchor targetBoundCtx builds. A present token the
// grammar cannot resolve (TargetMax$ Y, the MaxTgts family, a named SVar the
// face does not define) keeps today's semantics -- the parameter is dropped
// to the default 1 -- because Num's degrade-to-zero contract is correct for
// an effect amount ("the card did nothing") but wrong for a mandatory
// target MINIMUM (a TargetMin$ X degrading to 0 would let a mandatory spell
// resolve untargeted). x is the cast's settled {X} (pc.x at the CR 601.2c
// announcement ask, where the announce has already run) so a
// SVar:X:Count$xPaid bound reads the paid value; at a placement ask no X
// applies (a trigger was never paid an X) and 0 is correct there -- an
// xPaid body still finds the cast's value on the source permanent via
// count.go's provenance fallback. The clamp contract is targetBounds',
// applied AFTER resolution: min >= 0, max >= 1, max >= min.
func (e *Engine) resolvedTargetBounds(p state.PlayerID, source state.ObjID, sa *cards.SA, x int32) (int, int) {
	min, max := targetBounds(sa)
	if !targetBoundsDynamic(sa) {
		return min, max
	}
	ctx, ok := e.targetBoundCtx(p, source)
	if !ok {
		return min, max
	}
	ctx.X = x
	if v, ok := sa.Params["TargetMin"]; ok && !isLiteralBound(v) {
		if n, resolved := effects.NumResolved(e, ctx, sa, "TargetMin", 1); resolved {
			min = int(n)
		}
	}
	if v, ok := sa.Params["TargetMax"]; ok && !isLiteralBound(v) {
		if n, resolved := effects.NumResolved(e, ctx, sa, "TargetMax", 1); resolved {
			max = int(n)
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

// resolvedTargetMin is the Min half of resolvedTargetBounds, for resolveTop's
// N2 gate -- an ability or spell that MAY target zero things (a resolved
// TargetMin$ 0) and has none recorded resolves untargeted rather than
// fizzling. An unresolvable dynamic Min keeps the literal reader's default 1,
// so the N2 exemption never opens for a bound this build cannot price.
func (e *Engine) resolvedTargetMin(p state.PlayerID, source state.ObjID, sa *cards.SA, x int32) int {
	min, _ := e.resolvedTargetBounds(p, source, sa, x)
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
// playerTargetSpecMatches judges one candidate seat q against a ValidTgts$
// player spec from the asker's (you) perspective. It is the ONE judge both
// target sites go through -- the offer (candidatesFor) and the resolution
// recheck (legalTargets) -- so the two cannot disagree (the one-definition
// rule). Every ordinary alternative is judged by the shared
// effects.MatchesPlayerSpecFrom grammar; an alternative whose qualifier names
// a trigger role the ask's own TriggerContext carries (pg2 event roles) is
// judged by triggerRolePlayerAlt below, because MatchesPlayerSpecFrom takes
// no trigger context and fails closed on the role names -- which turned The
// Lord of Pain's mandatory "choose another target player" trigger
// (ValidTgts$ Player.!TriggeredActivator) into an ask with no legal target
// and a silent fizzle. Forge's comma is OR: the spec matches when any one
// alternative matches.
func (e *Engine) playerTargetSpecMatches(sc effects.SpecContext, spec string, q, you state.PlayerID, source state.ObjID) bool {
	for _, alt := range strings.Split(spec, ",") {
		if matched, known := e.triggerRolePlayerAlt(sc, alt, q, you); known {
			if matched {
				return true
			}
			continue
		}
		if effects.MatchesPlayerSpecFrom(e.G, alt, q, you, source) {
			return true
		}
	}
	return false
}

// triggerRolePlayerAlt evaluates ONE comma-alternative of a ValidTgts$
// player spec whose qualifier names a trigger role the ask's own
// TriggerContext carries. The corpus writes exactly two such qualifiers on a
// player alternative, both negated: Player.!TriggeredActivator (The Lord of
// Pain) and Player.!TriggeredCardController (Lucy MacLean, Positively
// Armed); a positive form resolves the same way. The binding rides the ask's
// SpecContext (pg2); an absent binding matches NOBODY, including under !
// (the documented pg2 absent-binding contract), so the fail-closed direction
// is kept for every qualifier the context cannot answer. known=false when
// the alternative does not name a trigger role at all, leaving it to the
// shared grammar.
func (e *Engine) triggerRolePlayerAlt(sc effects.SpecContext, alt string, q, you state.PlayerID) (bool, bool) {
	base, qualifier, qualified := strings.Cut(strings.TrimSpace(alt), ".")
	if !qualified {
		return false, false
	}
	neg := strings.HasPrefix(qualifier, "!")
	var role state.PlayerID
	bound := false
	switch strings.TrimPrefix(qualifier, "!") {
	case "TriggeredActivator":
		role, bound = sc.TriggerContext.TriggerActivator.Player, sc.TriggerContext.TriggerActivator.IsPlayer
	case "TriggeredCardController":
		// effects.TriggeredCardController is the one resolver -- Defined$,
		// OptionalDecider$ and the targeting restriction all read it.
		if p, ok := effects.TriggeredCardController(e.G, sc.TriggerContext, sc.Remembered); ok {
			role, bound = p, true
		}
	default:
		return false, false
	}
	// The base still applies to the role alternative, exactly as
	// MatchesPlayerSpecFrom applies it to every other qualifier.
	switch base {
	case "Player", "Any":
	case "You":
		if q != you {
			return false, true
		}
	case "Opponent", "Other":
		if q == you {
			return false, true
		}
	default:
		// An object alternative (Creature.!TriggeredTarget, ...): not this
		// helper's business -- the object arm judges it.
		return false, false
	}
	return bound && ((q == role) != neg), true
}

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
	// The spec-relative source (Self/Other/CARDNAME/sameName predicates read
	// it) is the SOURCE PERMANENT when the ask belongs to a minted ability
	// object -- the same object resolution-time recheck (legalTargets) already
	// judges its specs against (rules/stack.go passes o.Source there), so the
	// offer and the recheck cannot disagree (Critical C2's one-definition
	// rule). The Face-less wrapper itself is never a creature/permanent, so
	// a spec like Flamerush Rider's `Creature.attacking+Other` judged the
	// wrapper id meant "every creature but nobody in particular" and offered
	// the ability's own source as its own copy target. excludeSelf stays the
	// object the CR 115.5 self-targeting rule keys on (the stack object, for
	// an ability -- an ability CAN legally target its own Source permanent),
	// and the trigger-context lookup stays keyed on the stack id.
	specSrc := source
	if o := e.G.Obj(source); o != nil && o.Face() == nil && o.Ability != nil && o.Source != 0 {
		specSrc = o.Source
	}
	sc := e.targetSpecContext(specSrc, excludeSelf, p)
	zones := targetZones(sa)
	var out []targetCandidate
	// Players are offered only alongside the default battlefield search and
	// only when the spec actually names a seat. A spec that routes elsewhere
	// (TgtZone$ Graveyard/Hand/Exile) targets objects only -- never a player.
	// Each candidate seat is judged by the SAME shared player filter the
	// resolution recheck (legalTargets) applies, from the asker's perspective,
	// so offer and recheck cannot disagree (the one-definition rule): a
	// ValidTgts$ Opponent ask no longer offers the controller, ValidTgts$ You
	// no longer offers opponents, and a spec whose qualifier the filter cannot
	// evaluate fails closed to no seat (the AGENTS.md MatchesPlayerSpec
	// convention). Note MatchesPlayerSpecFrom splits on ',' and skips
	// alternatives whose base is not a player base, so a mixed
	// `Creature,Opponent` spec keeps the object half and matches only the
	// player half's seats.
	if len(zones) == 1 && zones[0] == state.ZBattlefield {
		for _, q := range e.G.AliveFrom(0) {
			if e.playerTargetSpecMatches(sc, spec, q, p, specSrc) {
				out = append(out, targetCandidate{kind: "player", player: q})
			}
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
				// The cast-provenance split (castprov1/castprov3/wascastfrom):
				// a stack-target spec carrying a wasCast* token — Wash Away's
				// `Card.!wasCastFromTheirHand` — evaluates the token against the
				// candidate's cast log here, before the ordinary filter; the
				// effects-side filter never strips the token, so without this the
				// spec fails closed to no candidate.
				tspec, ok := e.castProvenanceAdmits(targetSpecForZone(spec, z), oid, p)
				if !ok {
					continue
				}
				if effects.MatchesSpecCtx(e.G, tspec, oid, sc) {
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
				// (Vines of Vastwood) withholds one from the spoke player;
				// CR 702.14 shroud withholds one from EVERY targeting spell
				// or ability, its controller's included.
				// All function only on the battlefield (CR 604.3), the same
				// gate as protection above. CR 115.5 excludes the source.
				if o != nil && o.Face() != nil && (excludeSelf == 0 || oid != excludeSelf) {
					// The cast-provenance split at the non-battlefield target
					// zones too (wascastfrom): the token evaluates against the
					// candidate's cast log before the ordinary filter.
					tspec, ok := e.castProvenanceAdmits(targetSpecForZone(spec, z), oid, p)
					if !ok {
						continue
					}
					if effects.MatchesSpecCtx(e.G, tspec, oid, sc) &&
						(!targeting || !(o.Zone == state.ZBattlefield && e.protectedFrom(oid, protSrc))) &&
						(!targeting || !(o.Zone == state.ZBattlefield && e.shroudBlocksTarget(oid))) &&
						(!targeting || !(o.Zone == state.ZBattlefield && e.hexproofBlocksTarget(oid, p, protSrc))) &&
						(!targeting || !(o.Zone == state.ZBattlefield && e.restrictionBlocksTarget(oid, p))) {
						out = append(out, targetCandidate{kind: "permanent", obj: oid, player: q})
					}
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

// candidateControllerSeat is the controller seat a TARGET CANDIDATE keys a
// per-controller selection constraint on: a player candidate is its own seat
// (player candidates carry an explicit seat), an object candidate is its
// controller (its owner for a card in a graveyard/hand/exile, which Move sets
// Controller to). Both oneEachTargetBounds' distinct-controller count and
// targetControllerGroup's group label read this one helper, so the bound and
// the wire exclusivity can never disagree about which candidates share a
// controller.
func (e *Engine) candidateControllerSeat(candidate targetCandidate) state.PlayerID {
	if candidate.kind == "player" {
		return candidate.player
	}
	if o := e.G.Obj(candidate.obj); o != nil {
		return o.Controller
	}
	return candidate.player
}

// oneEachTargetBounds applies Forge's per-controller target selection shapes:
// TargetsForEachPlayer$ (TargetRestrictions.setForEachPlayer) and
// TargetsWithDifferentControllers$ ("targets controlled by different
// players"). Both are the SAME set constraint -- no two chosen targets may
// share a controller -- and both are labelled on the wire by
// targetControllerGroup's Option.Group. askTarget (this file) and cast.go's
// targetAsk share it so the two ask sites cannot drift.
//
// It returns the (possibly rewritten) bounds, whether the constraint applies
// at all, and the DISTINCT-CONTROLLER count. The count is the real capacity of
// the constraint: a TargetMin$/TargetMax$ spelled OneEach asks for exactly
// that many, and BOTH shapes cap the effective maximum at it, because no legal
// answer can ever select more targets than there are distinct controllers.
// Without the cap (the pre-fix state) a TargetsWithDifferentControllers$ SA
// with a literal TargetMin$ 2 | TargetMax$ 2 asked for two picks while
// offering only one selectable group -- an unsatisfiable decision that no
// intent could answer (Run Away Together, Kitsune, Dragon's Daughter).
// Callers compare min against distinct (not the raw option count) to detect
// that no legal set exists.
// sameControllerTargetBounds applies the TargetsWithSameController$ set
// constraint. Unlike the one-per-controller family, this permits multiple
// picks from one group; its capacity is therefore the largest controller
// group, not the number of groups. The capacity is returned separately so
// callers can reject a mandatory ask without exposing an unsatisfiable
// decision.
func (e *Engine) sameControllerTargetBounds(sa *cards.SA, candidates []targetCandidate, min, max int) (int, int, int, bool) {
	if !strings.EqualFold(sa.Params["TargetsWithSameController"], "True") {
		return min, max, 0, false
	}
	counts := map[state.PlayerID]int{}
	for _, candidate := range candidates {
		counts[e.candidateControllerSeat(candidate)]++
	}
	capacity := 0
	for _, count := range counts {
		if count > capacity {
			capacity = count
		}
	}
	if capacity > 0 && max > capacity {
		max = capacity
	}
	return min, max, capacity, true
}

func (e *Engine) oneEachTargetBounds(sa *cards.SA, candidates []targetCandidate, min, max int) (int, int, bool, int) {
	if !targetControllerExclusive(sa) {
		return min, max, false, 0
	}
	// Option.Group makes the one-per-controller restriction part of the
	// generic decision contract, so every target API consumes the same
	// enforcement rather than each effect maintaining a picker.
	groups := map[state.PlayerID]bool{}
	for _, candidate := range candidates {
		groups[e.candidateControllerSeat(candidate)] = true
	}
	distinct := len(groups)
	// OneEach respells a bound as the distinct-controller count. It is the
	// TargetsForEachPlayer$ spelling in Forge's grammar, but the corpus also
	// writes it on a TargetsWithDifferentControllers$ SA (Mysterious
	// Stranger's "for each player" graveyard pick), where it means the same
	// thing -- so the respell is driven by the VALUE, not by which of the two
	// equivalent flags is present. Before this only the
	// TargetsForEachPlayer$ spelling was read and Mysterious Stranger asked
	// for ONE target (Min 1 / Max 1) instead of one per represented player.
	if strings.EqualFold(sa.Params["TargetMin"], "OneEach") {
		min = distinct
	}
	if strings.EqualFold(sa.Params["TargetMax"], "OneEach") {
		max = distinct
	}
	// Cap the maximum at the distinct-controller count for BOTH shapes: a
	// literal or dynamic TargetMax$ larger than the number of controllers
	// present could only invite an answer the exclusivity rule rejects, so
	// the offer must advertise the true capacity (Havoc Eater's TargetMax$ X,
	// Protector of the Wastes' TargetMax$ 2). distinct is 0 only when there
	// are no candidates at all, and the no-option paths handle that before a
	// decision is built, so the max >= 1 clamp contract is preserved.
	if distinct > 0 && max > distinct {
		max = distinct
	}
	return min, max, true, distinct
}

// targetControllerExclusive reports whether this targeting SA carries Forge's
// per-controller selection shape -- TargetsForEachPlayer$ True ("up to one
// target each player controls", the OneEach family) or
// TargetsWithDifferentControllers$ True ("targets controlled by different
// players", Protector of the Wastes and 7 more corpus carriers). Both are the
// SAME set constraint -- no two chosen targets may share a controller -- and
// both are expressed on the wire by Option.Group, so one predicate backs both
// and the resolution recheck reads it through the same helper.
func (e *Engine) narrowSameController(targets []state.Target) []state.Target {
	if len(targets) < 2 {
		return targets
	}
	controller := targets[0]
	seat, ok := e.targetControllerSeat(controller)
	if !ok {
		return targets
	}
	out := targets[:0]
	for _, target := range targets {
		if got, valid := e.targetControllerSeat(target); valid && got == seat {
			out = append(out, target)
		}
	}
	return out
}

func targetControllerExclusive(sa *cards.SA) bool {
	return strings.EqualFold(sa.Params["TargetsForEachPlayer"], "True") ||
		strings.EqualFold(sa.Params["TargetsWithDifferentControllers"], "True")
}

// targetControllerGroup is the Option.Group label binding one selection slot
// to its controller -- the same label both ask sites attach, so
// Decision.Validate's mutual-exclusion rule enforces one pick per controller
// on the wire. It applies whenever targetControllerExclusive holds, so a
// TargetsWithDifferentControllers$ ask restricts the SAME way a OneEach ask
// does: the exclusivity is the generic wire contract's job, not each effect's.
func (e *Engine) targetControllerGroup(sa *cards.SA, candidate targetCandidate) string {
	if !targetControllerExclusive(sa) {
		return ""
	}
	return "target-controller-" + strconv.Itoa(int(e.candidateControllerSeat(candidate)))
}

// targetControllerSeat is the controller seat a chosen target keys the
// per-controller constraint on, read at the resolution recheck: a player
// target is its own seat; an object target is its controller (its owner for a
// card in a graveyard/hand/exile, which apply.go's Move fold keeps equal to
// Controller). It is the recheck's twin of targetControllerGroup, which reads
// the same seat off a candidate at the offer (candidatesFor labels a
// non-battlefield candidate with the zone slice's owner q, and Move sets
// Controller = Owner there), so offer and recheck cannot disagree. ok is false
// for a target whose seat is unknown (a vanished object), which the caller
// keeps rather than duplicating a phantom controller.
func (e *Engine) targetControllerSeat(t state.Target) (state.PlayerID, bool) {
	if t.IsPlayer {
		if int(t.Player) < len(e.G.Players) && !e.G.Players[t.Player].Lost {
			return t.Player, true
		}
		return 0, false
	}
	if o := e.G.Obj(t.Obj); o != nil {
		return o.Controller, true
	}
	return 0, false
}

// narrowDifferentControllers is CR 608.2b's "does as much as possible" read of
// TargetsWithDifferentControllers$ at resolution: walk the still-legal targets
// in recorded order and keep the first of each controller, dropping a later
// target whose controller already appears. A controller change in response can
// make two targets share a controller, and a resolution must never act on a
// set the targeting requirement forbids; keeping the earliest chosen target is
// the deterministic, order-stable reading (the same "first in the recorded
// order" discipline legendCasualties uses for its scan-order survivor). It is
// applied only to the flag-bearing SA, after the ordinary per-target legality
// recheck, so it only ever REMOVES a target -- it can never widen a set the
// per-target filter already narrowed.
func (e *Engine) narrowDifferentControllers(targets []state.Target) []state.Target {
	seen := map[state.PlayerID]bool{}
	out := targets[:0:0]
	for _, t := range targets {
		seat, ok := e.targetControllerSeat(t)
		if ok && seen[seat] {
			continue
		}
		if ok {
			seen[seat] = true
		}
		out = append(out, t)
	}
	return out
}

// maxTotalTargetPower resolves a targeting subject's MaxTotalTargetPower$
// cap -- the running total-power bound over a multi-target selection
// ("Return any number of target creature cards with total power 10 or less",
// Reunion of the House and Nethroi, Apex of Death; 2 corpus files). A literal
// token reads directly (including a literal 0 or negative, both enforceable:
// the ask sites attach every present cap as the decision's budget with
// Decision.Budgeted set, because a MaxSum of 0 alone reads as NO budget on
// the wire); a dynamic token resolves through the
// effects numeric grammar, the same reader resolvedTargetBounds applies to a
// TargetMax$ X token, bound to the asking player and the source anchor. A
// token the grammar cannot resolve returns ok=false -- the cap is then
// simply not enforced (today's behaviour; measured, no corpus carrier
// reaches this arm unresolvable, both carriers are the literal 10).

func (e *Engine) maxTotalTargetPower(p state.PlayerID, source state.ObjID, sa *cards.SA, x int32) (int, bool) {
	v, ok := sa.Params["MaxTotalTargetPower"]
	if !ok {
		return 0, false
	}
	if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
		return n, true
	}
	ctx, okc := e.targetBoundCtx(p, source)
	if !okc {
		return 0, false
	}
	ctx.X = x
	if n, resolved := effects.NumResolved(e, ctx, sa, "MaxTotalTargetPower", 0); resolved {
		return int(n), true
	}
	return 0, false
}

// totalPowerCappedCandidates applies the MaxTotalTargetPower$ cap to a
// target census. The per-option half is a real filter, but ONLY where a
// candidate provably cannot join any legal selection: with every power
// non-negative, a candidate whose power ALONE exceeds the cap busts every
// set containing it and is pruned. A NEGATIVE-power candidate breaks that
// argument -- a CDA can be negative in a zone (Scourge of the Skyclaves's
// 20-minus-highest-life CDA is -1 at a 21-life opponent, and it applies in
// EVERY zone per CR 208.2), and 11 + (-1) = 10 is a legal compensated
// selection under a cap of 10 -- so when any candidate reads negative the
// over-cap candidate is pruned only when even the maximal offset cannot
// save it: no selection containing it can score under the cap unless it
// takes EVERY other negative candidate on offer, so the prune test is
// p + otherNeg > cap (otherNeg = the sum of the OTHER candidates' negative
// powers). The one rule covers every cap, zero and negative included: a
// cap of 0 keeps a 2-power candidate beside two -1s (2-1-1 = 0), and a
// negative cap no combination of negatives can reach prunes the whole
// census. The prune does NOT model TargetMax$ bounding how many negatives
// one selection may take (both corpus carriers' TargetMax$ is X, every
// candidate) -- a survivor it keeps may then be unselectable, never the
// reverse, and Decision.Validate still rejects any over-cap answer.
// The running half -- any combination whose summed power stays within the
// bound -- is NOT expressible as a per-candidate property, so it is not a
// filter here: the two ask sites (askTarget below and cast.go's targetAsk)
// attach it to the decision as the cumulative-budget wire contract --
// Decision.MaxSum over each object option's Value (the candidate's power,
// negatives included) -- which Decision.Validate enforces on every
// submitted answer and Clamp/decision.FitRequired mirror for the
// deterministic bot. That is the same mechanism a Dig's WithTotalCMC$
// budget uses, so what a client may submit and what the engine offered can
// never disagree. Player candidates carry Value 0 and are never pruned (a
// MaxTotalTargetPower$ ask names cards; a player's presence is free).
// Returns the pruned census, the cap and whether the parameter is present
// at all.
//
// The power read is the DERIVED power (Engine.Power), not the printed
// face: a characteristic-defining P/T applies in EVERY zone (CR 208.2 --
// Lord of Extinction counts the graveyards from its own graveyard, which
// derivedScalarFrom's CDA read covers), and the printed Face().Power()
// returns 0 for such a face -- the first cut of this read undercounted a
// CDA creature as a free target.
func (e *Engine) totalPowerCappedCandidates(candidates []targetCandidate, p state.PlayerID, source state.ObjID, sa *cards.SA, x int32) ([]targetCandidate, int, bool) {
	capPower, ok := e.maxTotalTargetPower(p, source, sa, x)
	if !ok {
		return candidates, 0, false
	}
	// First pass: every card candidate's DERIVED power, and the maximal
	// negative offset the census carries (the sum of the negative powers --
	// the most any selection containing an over-cap candidate can ever
	// claw back). Player candidates carry no power.
	power := make([]int32, len(candidates))
	negSum := int32(0)
	for i, c := range candidates {
		if c.kind == "player" {
			continue
		}
		o := e.G.Obj(c.obj)
		if o == nil || o.Face() == nil {
			continue
		}
		power[i] = e.Power(c.obj)
		if power[i] < 0 {
			negSum += power[i]
		}
	}
	out := make([]targetCandidate, 0, len(candidates))
	for i, c := range candidates {
		if c.kind == "player" {
			out = append(out, c)
			continue
		}
		o := e.G.Obj(c.obj)
		if o == nil || o.Face() == nil {
			continue
		}
		// Prune only a candidate no legal selection can contain: its own
		// power plus the maximal offset the OTHER candidates can supply (the
		// sum of their negative powers) still busts the cap. With no
		// negatives that is the plain p > cap prune; a candidate at or under
		// the cap is never pruned by it when cap >= 0. The same rule holds
		// for a cap of zero or less (powers 2,-1,-1 under a cap of 0 total
		// 0, so the 2 stays), and there it can prune the whole census --
		// when even every negative candidate together cannot reach a
		// negative cap, no selection is legal and the ask resolves as
		// targetless. Whatever survives, taking every negative survivor
		// alongside it fits, which is what decision.FitRequired's negative
		// top-up relies on to always reach a valid answer.
		p := power[i]
		others := negSum
		if p < 0 {
			others -= p
		}
		if p+others > int32(capPower) {
			continue
		}
		out = append(out, c)
	}
	return out, capPower, true
}

// AskCopyTargets offers CR 707.10c's new-target choice for the copy on top
// of the stack. It is driven entirely by the copy's own
// CopyMayChooseTarget flag -- set by the StackCopy fold from the CREATING
// CopySpellAbility's MayChooseTarget$ parameter, so an external copier
// (Mirari, Cloven Casting, a Storm or Replicate copy) grants the election
// even though its SA is not part of the copied spell's text.
//
// The ask preserves the copied spell's WHOLE target requirement, not just
// one slot: MayChooseTarget$ True is not restricted to one-target spells, so
// the decision's bounds come from the copy's own declaration through the
// SAME resolvedTargetBounds / oneEachTargetBounds pair askTarget and cast.go's
// targetAsk use (a two-target spell therefore accepts two picks, and a
// per-controller declaration keeps its Option.Group exclusivity). Every
// inherited target is offered first as a keep-current option -- ALWAYS, even
// when it is no longer legal, because choosing new targets is optional and a
// player who keeps an illegal target simply lets the copy fizzle per CR
// 608.2b (forcing a new target here would retarget a copy the player declined
// to change) -- so selecting the leading keep-current options reproduces
// "choose nothing new". The remaining options use the same legal-target
// census as casting. The election is one-shot: the answer records targets
// through recordChosenTargets, whose TargetsChosen fold clears the flag, so
// the resolveTop re-entry does not ask again.
func (e *Engine) AskCopyTargets() bool {
	n := len(e.G.Stack)
	if n == 0 {
		return false
	}
	o := e.G.Obj(e.G.Stack[n-1])
	if o == nil || !o.IsCopy || !o.CopyMayChooseTarget {
		return false
	}
	controller := o.Controller
	sa := o.Ability
	if o.Face() != nil {
		sa = o.Face().SpellAbility()
	}
	if sa == nil || strings.TrimSpace(sa.Params["ValidTgts"]) == "" {
		return false
	}
	candidates := e.legalTargetCandidates(controller, o.ID, o.ID, sa)
	ordered := make([]targetCandidate, 0, len(candidates)+len(o.Targets))
	for _, old := range o.Targets {
		matched := -1
		for i, candidate := range candidates {
			if targetCandidateEqual(old, candidate) {
				matched = i
				break
			}
		}
		if matched >= 0 {
			ordered = append(ordered, candidates[matched])
			candidates = append(candidates[:matched], candidates[matched+1:]...)
		} else {
			// Keep-current even though the target is no longer legal: the
			// player may decline new targets (CR 707.10c), and the copy
			// then fizzles at CR 608.2b.
			ordered = append(ordered, stateTargetCandidate(old))
		}
	}
	ordered = append(ordered, candidates...)
	if len(ordered) == 0 {
		return false
	}
	// The copy's OWN declaration supplies the required count (CR 707.10c
	// retargets a copy per the spell's target rules), through the same shared
	// readers the cast ask uses so the two sites cannot drift. oneEachTargetBounds
	// is fed the full selectable list -- keep-current slots included -- so the
	// per-controller capacity counts a kept target too.
	min, max := e.resolvedTargetBounds(controller, o.ID, sa, o.X)
	min, max, _, _ = e.oneEachTargetBounds(sa, ordered, min, max)
	// A mandatory minimum above the offered list would be an unanswerable
	// decision no seat could satisfy (a livelock). The inherited keep-current
	// entries are always offered even when illegal, so the list is non-empty;
	// clamping Min to it keeps totality whenever a dynamic bound outruns the
	// copy's inherited set (the copy then fizzles at CR 608.2b like any other
	// under-target resolution).
	if min > len(ordered) {
		min = len(ordered)
	}
	if max < min {
		max = min
	}
	d := &decision.Decision{Player: controller, Kind: decision.KTarget, Min: min, Max: max,
		Prompt: "Choose a new target for the copy", Source: o.ID,
		ResumeKind: "copy_targets", ResumeSA: sa, TargetEffect: describeTargetEffect(sa)}
	for _, candidate := range ordered {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: candidate.kind,
			Label: e.targetOptionLabel(candidate), Obj: candidate.obj, Player: candidate.player,
			Group: e.targetControllerGroup(sa, candidate)})
	}
	e.Ask(d)
	return true
}

// stateTargetCandidate converts a recorded state.Target into the option shape
// the target census uses. The kind is only the wire label; a player target
// keeps "player" and every object target is offered as "permanent" (the
// label reads the object's own name, so a target that has left the
// battlefield still renders correctly).
func stateTargetCandidate(t state.Target) targetCandidate {
	if t.IsPlayer {
		return targetCandidate{kind: "player", player: t.Player}
	}
	return targetCandidate{kind: "permanent", obj: t.Obj}
}

func targetCandidateEqual(t state.Target, c targetCandidate) bool {
	if t.IsPlayer {
		return c.kind == "player" && c.player == t.Player
	}
	return c.kind != "player" && c.obj == t.Obj
}

// askTarget offers every legal target for a spell or ability. It deliberately
// retains the post-push insufficient-target backstop: modal and dynamic target
// counts are not rejected by the earlier cast-offer census.
func (e *Engine) askTarget(p state.PlayerID, source state.ObjID, sa *cards.SA) {
	min, max := e.resolvedTargetBounds(p, source, sa, 0)
	candidates := e.legalTargetCandidates(p, source, source, sa)
	// MaxTotalTargetPower$ (Reunion of the House): prune the candidates that
	// can provably join no legal selection (individually over the cap unless
	// a negative-power candidate could offset them) and carry the running
	// cap as the decision's cumulative budget. Read BEFORE the per-controller
	// bounds so `distinct` counts only candidates a legal selection can
	// still take.
	candidates, powerCap, powerCapped := e.totalPowerCappedCandidates(candidates, p, source, sa, 0)
	min, max, exclusive, distinct := e.oneEachTargetBounds(sa, candidates, min, max)
	min, max, sameCapacity, sameController := e.sameControllerTargetBounds(sa, candidates, min, max)
	d := &decision.Decision{Player: p, Kind: decision.KTarget, Min: min, Max: max,
		Prompt: "Choose a target for " + e.targetName(source),
		Source: source, TargetEffect: describeTargetEffect(sa),
		TargetsWithSameController: sameController, ResumeSA: sa}
	for _, candidate := range candidates {
		// targetOptionLabel tolerates the Face-less ability object a
		// TargetType$ Activated/Triggered spec now offers: targetName falls
		// back to the source permanent's name.
		label := e.targetOptionLabel(candidate)
		o := decision.Option{Index: len(d.Options), Kind: candidate.kind,
			Label: label, Obj: candidate.obj, Player: candidate.player}
		o.Group = e.targetControllerGroup(sa, candidate)
		o.Controller = e.candidateControllerSeat(candidate)
		// Option.Value is omitempty and read only under a budget
		// (Decision.HasBudget), so a budget-less target ask keeps its wire
		// payload byte-identical. Every present cap -- zero and negative
		// included, via Decision.Budgeted -- rides the wire, so
		// Decision.Validate enforces the total on every submitted answer.
		if powerCapped && candidate.kind != "player" {
			if co := e.G.Obj(candidate.obj); co != nil && co.Face() != nil {
				o.Value = int(e.Power(candidate.obj))
			}
		}
		d.Options = append(d.Options, o)
	}
	if powerCapped {
		d.MaxSum, d.Budgeted = powerCap, true
	}
	if min == 0 {
		// Requirement N2 / totality: a target-hungry subject whose minimum
		// is zero resolves untargeted when NO legal target exists. When at
		// least one exists it is still offered (Min 0 lets the chooser take
		// none); a zero-option decision would strand the cast, never asked.
		if len(d.Options) == 0 {
			return
		}
	} else if len(d.Options) < min || (exclusive && min > distinct) || (sameController && min > sameCapacity) {
		// A target-hungry subject with fewer legal targets than Min -- or one
		// whose per-controller constraint admits fewer distinct controllers
		// than Min (exclusive && min > distinct: two mandatory targets, both
		// controlled by one player) -- uses CR
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
	if d.ResumeKind == "copy_targets" {
		if e.resume != nil {
			e.resumeResolution(e.resume, chosen)
		}
		return
	}
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
		// A Fuse cast may ask targets twice (front, then alternate). Append
		// rather than replace so the stack object's flat target list carries
		// both halves, and stageBase records whether an earlier half's
		// targets are already recorded (its first target must then APPEND).
		stageBase := len(pc.targets)
		pc.targets = append(pc.targets, targetOptions(chosen)...)
		// A Fuse cast records the stage's own slice (review MAJOR 1): the flat
		// list on the stack object cannot express which half chose which
		// target, and re-deriving the split through each half's ValidTgts spec
		// mis-assigns every target that half's spec merely overlaps (Turn //
		// Burn's Creature vs Any). Indexed by pc.targetStage, so a targetless
		// stage the ask loop skipped never misaligns the slices.
		if pc.mode == "fuse" {
			for len(pc.stageTargets) <= pc.targetStage {
				pc.stageTargets = append(pc.stageTargets, nil)
			}
			pc.stageTargets[pc.targetStage] = append(pc.stageTargets[pc.targetStage], targetOptions(chosen)...)
		}
		e.repriceForTargets(pc)
		if !pc.isAbility() {
			if pc.stackObj != 0 {
				e.recordChosenTargets(pc.stackObj, chosen, stageBase > 0)
			}
			// CR 702.101b: after the front half's targets, ask the alternate
			// half's before payment. targetAsk skips a targetless half and the
			// cast pays once every target stage is settled.
			if e.castHasNextTargetStage(pc, e.G.Obj(pc.card)) {
				pc.targetStage++
				if !e.targetAsk() {
					e.payCast()
				}
			} else {
				e.payCast()
			}
		} else {
			e.payCast()
			if pc.stackObj != 0 {
				e.recordChosenTargets(pc.stackObj, chosen, false)
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
	e.recordChosenTargets(d.Source, chosen, false)
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
// appendFirst makes even the first option an APPEND: a Fuse cast records the
// front half's targets first and must not have the alternate half's first
// target replace them on the stack object.
func (e *Engine) recordChosenTargets(targetObj state.ObjID, chosen []decision.Option, appendFirst bool) {
	for i, opt := range chosen {
		ev := events.Event{Kind: events.TargetsChosen, Obj: targetObj}
		appendThis := i > 0 || appendFirst
		if opt.Kind == "player" {
			// shape 1 replace / shape 3 append a single player target.
			ev.Amount = 1
			if appendThis {
				ev.Amount = 3
			}
			ev.Player = opt.Player
		} else {
			// shape 0 replace / shape 2 append one object target.
			if appendThis {
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
// offeredTargetSA is the SA whose ValidTgts$ targeting the placement or
// announcement ask covered for this stack object, derived exactly as the
// TargetsOffered marker's derivation in resolveTop: the ability SA itself
// for a non-modal trigger or activated ability (pushTrigger's askTarget),
// the first target-bearing CHOSEN MODE's sub for a modal one (handleModes'
// placement branch asks the mode sub and skips the outer ask entirely), and
// the spell's target declaration (targetAsk's targetSA -- the announced
// mode's for a modal spell) for a spell. nil when none of those declares
// targets. Shared by resolveTop's two branches and resumeResolution so the
// generic ValidTgts$ pre-ask (effects' chosenTargetsFor) skips exactly the
// covered SA on the first pass AND on every resume re-entry -- an optional
// trigger's yes re-enters through resumeResolution, where the first pass's
// bool marker alone is not carried (task mvts1).
func offeredTargetSA(o *state.Object, svars map[string]string) *cards.SA {
	if o.Ability != nil {
		if len(o.ChosenModes) > 0 && strings.TrimSpace(o.Ability.Params["Choices"]) != "" {
			for _, name := range o.ChosenModes {
				if sub := cards.ResolveSVar(svars, name); sub != nil &&
					strings.TrimSpace(sub.Params["ValidTgts"]) != "" {
					return sub
				}
			}
			return nil
		}
		if strings.TrimSpace(o.Ability.Params["ValidTgts"]) != "" {
			return o.Ability
		}
		return nil
	}
	f := o.Face()
	if f == nil {
		return nil
	}
	sa := f.SpellAbility()
	if sa == nil {
		return nil
	}
	targetSA := modalTargetSA(f, sa, o.ChosenModes)
	if targetSA != nil && strings.TrimSpace(targetSA.Params["ValidTgts"]) != "" {
		return targetSA
	}
	return nil
}

func (e *Engine) resolveTop() {
	id := e.G.Stack[len(e.G.Stack)-1]
	o := e.G.Obj(id)
	if o != nil && o.IsCopy && o.CopyMayChooseTarget {
		if e.AskCopyTargets() {
			return
		}
	}
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
			// NoResolvingCheck$ True (Ugin's Mastery, Werewolf Pack Leader,
			// Love on the Battlefield, ...): the condition was checked only
			// when the trigger fired, and the transient state it counted (a
			// bounced attacker, drained power) must not fizzle the ability
			// here (triggerResolvingCheckHolds in rules/trigger_condition.go).
			// The captured event roles travel with the ability; passing them
			// is what lets an event-relative clause (Condition$
			// AttackedPlayerWithMostLife) be re-checked with the defender the
			// trigger queued against, which no current state can re-derive.
			tc := e.triggerContexts[id]
			if !e.triggerResolvingCheckHolds(t, o.Source, &tc) {
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
		if spec := o.Ability.Params["ValidTgts"]; spec != "" && !(e.resolvedTargetMin(o.Controller, id, o.Ability, 0) == 0 && len(targets) == 0) {
			legal := e.legalTargets(targets, o.Ability, targetZones(o.Ability), o.Controller, o.Source, id)
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
		rt, triggered := e.findTriggerForAbility(o.Source, o.Ability)
		if triggered {
			if spec := rt.Params["OptionalDecider"]; spec != "" {
				who, askable := e.deciderFromSpec(spec, o.Controller, o.Remembered, e.triggerContexts[id])
				if !askable {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id,
						From: state.ZStack, To: state.ZExile, Text: "ceased to exist: its optional decider left the game"})
					e.ensureLeftTheStack(id, state.ZExile, "the optional decider of this ability left the game, so the "+
						"ability ceased to exist (CR 800.4a) and was parked in exile")
					return
				}
				e.askOptionalAtResolution(who, o, o.Ability, e.abilityLabel(o, rt))
				return
			}
		}
		// ResolvedLimit$ ("Do this only once each turn."): a MANDATORY
		// trigger that reaches this point is one whose effect is about to run
		// (the optional gate above returned for every OptionalDecider$ shape),
		// so consume its per-turn resolution count now -- before the
		// CumulativeUpkeep/Echo/Cost$ dispatch below, which may open a
		// pay/decline window but is still this ability resolving. A freshly
		// accepted optional trigger is counted at resumeResolution's
		// "optional" arm instead. The eligibility check is on the RESOLVED
		// line's own param (rt), not on the source's other lines: a mixed-line
		// carrier (Cosmic Crucible's mandatory Main1 mana trigger, no
		// ResolvedLimit$) resolving first must not spend its sibling's limit.
		if triggered {
			if _, limited := resolvedLimitValue(rt); limited {
				e.noteTriggerResolved(o.Source)
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
		//
		// A trigger effect whose Cost$ carries a Draw component joins them: the
		// corpus's Draw<X/Spec> family (Champion of Wits' "you may draw cards
		// equal to its power. If you do, discard two cards" -- Cost$ Draw<X/You>)
		// is the same "you may pay; when you do" idiom, and without the window
		// the body ran for free and no draw happened. The window's pay arm
		// settles the draw, the decline arm leaves the body unexecuted.
		//
		// trigcost1: the gate is no longer an API allowlist. ANY Cost$-bearing
		// trigger body enters the window (Kalastria Highborn's `Cost$ B`,
		// Elenda and Azor's `Cost$ PayLife<4>`, ...): Forge's Cost$ on a trigger
		// body is the "you may pay; if you do" idiom, so a body that never asks
		// executes for free. The window keeps the split -- Priceable cost offers
		// a real pay, everything else lands decline-only. Mandatory-prefixed
		// costs and Mana/CopySpellAbility bodies are carved out inside
		// triggerBodyNeedsCostWindow.
		if o.Ability.API == "CumulativeUpkeep" {
			e.startCumulativeUpkeep(id, o.Source, o.Ability)
			return
		}
		// Echo (kw:Echo, CR 702.35a) is the same keyword-expansion shape: an
		// ordinary Phase trigger whose DB$ Echo body needs rules' payment
		// window and the pay-or-sacrifice election (rules/echo.go). The
		// intervening-if was already applied at trigger time (triggerMatches's
		// Echo$ branch), so everything reaching here is owed.
		if o.Ability.API == "Echo" {
			e.startEcho(id, o.Source, o.Ability)
			return
		}
		if _, triggered := e.findTriggerForAbility(o.Source, o.Ability); triggered &&
			e.triggerBodyNeedsCostWindow(o.Ability) {
			e.startTriggeredEffectCost(&resumePoint{kind: "effect_cost", obj: id, sa: o.Ability}, o.Source)
			return
		}
		// The ability-cast copy family (abcopy1): an AB$ CopySpellAbility
		// execute carrying a real Cost$ (Rings of Brighthearth {2}, Kurkesh
		// {R}, Battlemages' Bracers {1}, Chandra's Regulator {1}) must never
		// copy for free. When the firing trigger's context carries an event
		// role -- the activation role TriggerAbility, or the spell arm's
		// TriggerCard (a SpellCast fires on PutOnStack, whose Obj IS the spell;
		// no ability wrapper is minted) -- route the same pay/decline window
		// the Untap/ImmediateTrigger shapes use: a decline leaves the trigger
		// unexecuted, a pay charges the cost and then runs the copy. An
		// unpriceable cost (Verrak's PayLife<X>, Mica's Sac<1/Artifact>) poses
		// the ask but offers no answerable "pay" -- the ParseUnlessCost
		// hard-decline convention. A context-less synthetic push (no role)
		// keeps the free-executor semantics.
		tc := e.triggerContexts[id]
		if _, triggered := e.findTriggerForAbility(o.Source, o.Ability); triggered &&
			o.Ability.API == "CopySpellAbility" &&
			o.Ability.Params["Cost"] != "" &&
			(tc.TriggerAbility != 0 || tc.TriggerCard != 0) {
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
		//
		// A mutated pile (CR 702.140d) makes "the source's current Face" the
		// wrong table for an UNDER-card's ability: Face() on a pile is always
		// its TOP card, whose SVar table the under-card's body never meant --
		// Huntmaster Liger mutated under a Grizzly Bears read the Bears' (
		// empty) table for its own "NumAtt$ +X | SVar:X:Count$TimesMutated"
		// and pumped by 0. The owning face is the one that carries the
		// resolving trigger, which findTriggerForAbilityFace recovers by the
		// compiled trigger pointer -- the whole reason MergedTriggerPush mints
		// f.Triggers[i].Effect rather than a freshly parsed SVar body. An
		// ordinary trigger finds its own (top) face there, so its table is
		// unchanged, and an activated ability finds no trigger at all and
		// falls through to Face() exactly as before.
		var svars map[string]string
		if src := e.G.Obj(o.Source); src != nil {
			if _, mf, ok := e.findTriggerForAbilityFace(o.Source, o.Ability); ok && mf != nil {
				svars = mf.SVars
			} else if mf, ok := e.pileFaceForSA(o.Source, o.Ability); ok && mf != nil {
				// An activated ability of a MUTATED pile (CR 702.140d): o.Ability
				// is the under-card's SA, so the table its body reads is the
				// under-card's own -- Porcuparrot's `NumDmg$ X` resolves X from
				// the pile's Count$TimesMutated on ITS face, not the top card's.
				svars = mf.SVars
			} else if sf := src.Face(); sf != nil {
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
			Targets: targets, Remembered: o.Remembered, Captured: o.Remembered, TriggerContext: e.triggerContexts[id],
			// The resolving stack-object wrapper: ValidStack's otherAbility
			// exclusion (Ulalek's sub-copy) anchors here, not on Source --
			// Source is the source permanent (Ruling T20-b), which is not on
			// the stack and would exclude nothing.
			ResolvingObj: id}
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
		offeredSA := offeredTargetSA(o, svars)
		if offeredSA != nil {
			ctx.TargetsOffered = true
			ctx.OfferedSA = offeredSA
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
	// Bestow (CR 702.114a): a cast paid for with the bestow cost resolves
	// as the synthesized Aura attach spell -- the creature face itself has
	// no SP -- so the whole ordinary Aura tail below runs unchanged:
	// effAttach emits events.Attach while the spell is still on the stack,
	// and moveResolvedOffStack enters the permanent attached (the entry
	// keeps an Attach set on the stack). The flag is the pay-time CastInfo
	// provenance modeFlags("bestowed") rode.
	if o.CastFlags&state.FlagBestowed != 0 {
		sa = bestowedAttachSA()
	}
	// Mutate (CR 702.140d): a spell cast for its mutate cost does not become
	// an independent permanent. It merges into its target, so resolution is
	// diverted BEFORE the ordinary spell-block tail (which would move it to
	// the battlefield): resolveMutate emits the Mutate fold, which parks this
	// object off the stack. A mutate card carries no SP, so the spell block
	// below would resolve nothing anyway.
	if o.CastFlags&state.FlagMutated != 0 {
		e.resolveMutate(o, o.Targets)
		return
	}
	// Fuse (CR 702.101b): a fused split spell is one spell whose BOTH halves'
	// spell abilities resolve in sequence. The card stays at its front face;
	// resolveFused owns the per-half target recheck, the Resolve event, the
	// Ascend blessing and the off-stack move.
	if o.CastFlags&state.FlagFused != 0 {
		// A half that suspended on a mid-resolution ask hands back the
		// continuation (the rest of that half plus a fuse-rest frame for any
		// unrun half); link it onto the ask's fresh resume point here, the
		// resolution machinery's own write (ruling T21-e).
		if cont, suspended := e.resolveFused(o); suspended && e.resume != nil {
			e.resume.outer = cont
		}
		return
	}
	targets := o.Targets
	// targetSA is the SA whose ValidTgts$ the cast-flow target ask offered
	// (the modal declaration for a Charm, the SpellAbility itself otherwise);
	// hoisted so the resolution ctx can carry the TargetsOffered marker and
	// the mvts1 pre-ask's OfferedSA skip.
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
		if spec := targetSA.Params["ValidTgts"]; spec != "" && !(e.resolvedTargetMin(o.Controller, id, targetSA, 0) == 0 && len(targets) == 0) {
			legal := e.legalTargets(targets, targetSA, targetZones(targetSA), o.Controller, id, id)
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
	// Ascend (CR 702.131a, the non-permanent case): an instant/sorcery with
	// K:Ascend grants its controller the blessing BEFORE the spell's own
	// body and condition checks read the latch (Forge's "do blessing there
	// before condition checks"; rules/ascend.go). Permanent faces are
	// excluded -- their grant is the emit-side continuous scan.
	e.grantSpellBlessing(o, f)
	if sa != nil {
		e.damaging = id
		ctx := &effects.Ctx{Source: id, Controller: o.Controller, Targets: targets,
			ResolvingObj: id}
		// Same marker as the ability branch: the cast-flow target ask
		// (targetAsk's targetSA) offered exactly this spell's targeting.
		if targetSA != nil && strings.TrimSpace(targetSA.Params["ValidTgts"]) != "" {
			ctx.TargetsOffered = true
			ctx.OfferedSA = targetSA
		}
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
	if o != nil && (state.ExilesLeavingStack(o.CastFlags) ||
		o.IsCopy || o.CastFlags&state.FlagAdventure != 0 || o.CastFlags&state.FlagReplaceGraveyard != 0) {
		return state.ZExile
	}
	if o != nil && o.CastFlags&state.FlagBuyback != 0 {
		return state.ZHand
	}
	return state.ZGraveyard
}

// spellFizzleZone is the resting place when a spell never resolved. Flashback,
// Harmonize, Aftermath, the ReplaceGraveyard$ Play rider and copies still use
// exile, but Buyback does not apply and the card reaches its owner's graveyard.
func spellFizzleZone(o *state.Object) state.Zone {
	if o != nil && (state.ExilesLeavingStack(o.CastFlags) ||
		o.IsCopy || o.CastFlags&state.FlagReplaceGraveyard != 0) {
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
// target is legal for as long as they are still in the game AND still match
// the spec's player filter (playerTargetSpecMatches -- the shared
// MatchesPlayerSpecFrom grammar plus the trigger-role alternatives the ask's
// own TriggerContext carries -- from the controller's perspective, against
// the same source the offer judged) -- the same judge the offer applies, so
// offer and recheck cannot disagree (the one-definition rule). A target
// whose qualifier the filter cannot evaluate was never offered and is
// rejected here too, fail closed.
func (e *Engine) legalTargets(targets []state.Target, sa *cards.SA, zones []state.Zone, you state.PlayerID, source state.ObjID, self state.ObjID) []state.Target {
	spec := ""
	if sa != nil {
		spec = sa.Params["ValidTgts"]
	}
	var legal []state.Target
	// The resolution recheck, unlike a target offer, has this stack object's
	// Targets available. Targeted* predicates may read precisely this binding;
	// setting it here keeps their self-reference unavailable at announcement.
	// The source rides in too, the same object askTarget's own offer filter
	// sees (candidatesFor's sc.Source): a source-reading predicate
	// (CanEnchantEquippedBy -- Mantle of the Ancients' recheck) judges the
	// chosen target at resolution exactly as the offer judged it at
	// placement, Critical C2's one-definition rule. Before this, the recheck
	// built its SpecContext with source 0 and every source-reading predicate
	// failed closed there -- a target the placement offer had just certified
	// fizzled at resolution.
	sc := e.targetSpecContext(source, self, you)
	sc.ResolutionTargets = targets
	sc.Resolving = true
	for _, t := range targets {
		if t.IsPlayer {
			if int(t.Player) < len(e.G.Players) && !e.G.Players[t.Player].Lost &&
				e.playerTargetSpecMatches(sc, spec, t.Player, you, source) {
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
		// CR 702.14 rides the same recheck: a target that GAINED shroud
		// between placement and resolution (Lightning Greaves equipping in
		// response) is dropped here, and a target with no other legal target
		// left fizzles the whole spell/ability through the existing fizzle
		// machinery upstream of this recheck.
		if o := e.G.Obj(t.Obj); o != nil && zoneIn(o.Zone, zones) {
			// The cast-provenance split at the resolution recheck too
			// (wascastfrom): the token evaluates against the target's cast
			// log before the ordinary filter, so offer and recheck cannot
			// disagree about a spec carrying one.
			tspec, ok := e.castProvenanceAdmits(targetSpecForZone(spec, o.Zone), t.Obj, you)
			if ok && effects.MatchesSpecCtx(e.G, tspec, t.Obj, sc) &&
				!(o.Zone == state.ZBattlefield && e.restrictionBlocksTarget(t.Obj, you)) &&
				!(o.Zone == state.ZBattlefield && e.shroudBlocksTarget(t.Obj)) &&
				!(o.Zone == state.ZBattlefield && e.hexproofBlocksTarget(t.Obj, you, e.protectionSource(source))) &&
				!e.protectedFrom(t.Obj, e.protectionSource(source)) {
				legal = append(legal, t)
			}
		}
	}
	// CR 608.2b / CR 601.2c: the per-controller targeting requirement is
	// rechecked on the surviving set too -- see narrowDifferentControllers.
	if sa != nil && strings.EqualFold(sa.Params["TargetsWithDifferentControllers"], "True") {
		legal = e.narrowDifferentControllers(legal)
	}
	if sa != nil && strings.EqualFold(sa.Params["TargetsWithSameController"], "True") {
		legal = e.narrowSameController(legal)
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
	// The caller supplies the chosen targets -- the announcement or placement
	// ask's answer -- so the generic ValidTgts$ pre-ask must not re-pose it
	// for an SA that declares targets (task mvts1).
	if sa != nil && strings.TrimSpace(sa.Params["ValidTgts"]) != "" {
		ctx.TargetsOffered = true
	}
	effects.SetSVars(ctx, svars)
	effects.Resolve(e, ctx, sa)
}

// Game, Emit, Rand and ShuffleLibrary satisfy effects.Host, which is how effects reach the
// engine without importing it. AddContinuous (layers.go), HasKeyword
// (layers.go) and Ask (resolution.go) round out the interface -- HasKeyword
// already existed for the layer system's own callers before effects.Host
// grew a method of the same name, and needed no change to satisfy it.
func (e *Engine) Game() *state.Game                       { return e.G }
func (e *Engine) ObjectColors(o *state.Object) string     { return e.objColors(o) }
func (e *Engine) Emit(ev events.Event)                    { e.emit(ev) }
func (e *Engine) EmitDamage(ev events.Event) events.Event { return e.emit(ev) }

// EmitLifeChange reports whether the exact proposed life change was applied.
// ExchangeLifeVariant uses this to avoid installing its characteristic half
// after a life replacement prevents, transforms, or parks the event.
func (e *Engine) EmitLifeChange(ev events.Event) bool {
	queued := len(e.replChoices)
	stored := e.emit(ev)
	// A parked CR 616.1 competition leaves the original event in hand while
	// putting a replacement choice on the engine queue.  It is not equivalent
	// to an unchanged event that was actually applied.
	if e.pending != nil || len(e.replChoices) != queued {
		return false
	}
	return stored.Kind == events.LifeChange && stored.Player == ev.Player && stored.Amount == ev.Amount
}
func (e *Engine) Rand(n int) int { return e.rng.IntN(n) }

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
//
// The census runs with the RESOLVING stack object as both source and
// excludeSelf whenever one exists -- exactly the placement ask's own call
// (pushTrigger -> askTarget passes the stack object id): CR 115.5 withholds
// the ability on the stack from targeting itself, never its source permanent,
// so a trigger whose source is a legal target may target it (Kor Outfitter's
// Attach sub attaches to Kor Outfitter). A direct, off-stack resolution (no
// resolving object) keeps the caller's source.
func (e *Engine) LegalTargets(chooser state.PlayerID, source state.ObjID, sa *cards.SA) []state.Target {
	if e.resolvingObj != 0 {
		source = e.resolvingObj
	}
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
	return len(e.spellsCastThisTurnMatching(you, spec, 0))
}

// SpellsCastThisTurnMatchingExcluding is SpellsCastThisTurnMatching with one
// object's own cast excluded -- the bare !CastSaSource qualifier's engine
// reading (the count's "other than the spell being cast" device; effects
// stripBareCastSaSource strips the token and routes here with the ctx
// source). Derived from the event log like CastThisTurn.
func (e *Engine) SpellsCastThisTurnMatchingExcluding(you state.PlayerID, spec string, exclude state.ObjID) int {
	return len(e.spellsCastThisTurnMatching(you, spec, exclude))
}

// EachSpellCastThisTurnMatching satisfies effects.Host's method of the same
// name: the matching casts' OBJECT IDS (the ARGUMENTED !CastSaSource$<Prop>
// aggregate forms' engine side; effects' aggregateCastProperty sums the
// property over them). Derived from the event log like the count forms.
func (e *Engine) EachSpellCastThisTurnMatching(you state.PlayerID, spec string, exclude state.ObjID) []state.ObjID {
	return e.spellsCastThisTurnMatching(you, spec, exclude)
}

func (e *Engine) spellsCastThisTurnMatching(you state.PlayerID, spec string, exclude state.ObjID) []state.ObjID {
	youScoped := strings.Contains(spec, "You")
	// The CastSa count specs (Rain of Riches' gate) are evaluated per cast
	// event against THAT cast's spend window, not the object's latest one —
	// a re-cast object's older cast must not inherit the newer cast's spend
	// — so the backward walk carries a per-caster spend bucket: a negative
	// ManaAdd (a spend event carries no Obj) belongs to the NEXT PutOnStack
	// the walk reaches for its player — the cast it sits above in the log —
	// exactly the window manaSpentForCast reads for the SA-level ValidSA$
	// family. Specs without a CastSa token take the unchanged per-event
	// chain call (their castProvenanceAdmits strip is event-local and
	// stateless).
	saTokens := castSaTokensIn(spec)
	// The in-flight cast's own grant walk (queueCascadeTriggers' scratch,
	// rules/cascade.go) counts PRIOR casts only: the Affected$ half of the
	// same static evaluates the current cast's own qualification, and the
	// gate's EQ0 is Forge's "the first" idiom — the twelve AffectedZone$
	// Stack SVarCompare$ gates in the corpus are all EQ0. Outside the walk
	// the count is inclusive (Vengevine's "the second creature spell" EQ2
	// gate is evaluated with the triggering cast in the log and must count
	// it).
	skipObj := e.stackGrantCast
	var buckets [8]castSpendFacts
	useAcc := len(saTokens) > 0
	var out []state.ObjID
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		switch ev.Kind {
		case events.ManaAdd:
			if useAcc && ev.Amount < 0 && int(ev.Player) < len(buckets) {
				buckets[ev.Player].spent += -ev.Amount
				if tag, _, ok := state.TypedManaCounter(ev.Counter); ok {
					buckets[ev.Player].tagged[tag] += -ev.Amount
				}
			}
			continue
		case events.PutOnStack:
		default:
			continue
		}
		// This push closes the spend window of the cast it announces: the
		// caster's bucket holds exactly the spends since the walk start,
		// which are this cast's own (plus the caster's own post-payment
		// floating — the manaSpentForCast convention). Take the facts and
		// reset, so an older cast of the same object (a hand cast before a
		// flashback) does not inherit them and the in-flight cast's window
		// belongs to no counted cast.
		var facts castSpendFacts
		if useAcc && int(ev.Player) < len(buckets) {
			facts = buckets[ev.Player]
			buckets[ev.Player] = castSpendFacts{}
		}
		// The push itself proves a cast exists: the window's ok read.
		facts.ok = true
		if skipObj != 0 && ev.Obj == skipObj {
			continue
		}
		if exclude != 0 && ev.Obj == exclude {
			continue
		}
		if youScoped && ev.Player != you {
			continue
		}
		matchSpec := spec
		alive := true
		for _, tok := range saTokens {
			var held bool
			if matchSpec, held = admitProvenanceAlternatives(matchSpec, tok.token, castSaTokenHolds(tok, facts)); !held {
				alive = false
				break
			}
		}
		if !alive {
			continue
		}
		// The bare wasCastFromYourHandByYou qualifier (the 5 end-step "if you
		// haven't cast a spell from your hand this turn" carriers'
		// Count$ThisTurnCast_Card.wasCastFromYourHandByYou bodies) is
		// evaluated per cast event against the log (task castprov1); the
		// wasCastByYou sibling (task castprov2) rides the same combined read.
		matchSpec, ok := e.castProvenanceAdmits(matchSpec, ev.Obj, you)
		if !ok {
			continue
		}
		if effects.MatchesSpecFrom(e.G, matchSpec, ev.Obj, you, ev.Obj) {
			out = append(out, ev.Obj)
		}
	}
	return out
}

// WasCastFromHandByYou satisfies effects.Host's WasCastFromHandByYou for the
// Count$wasCastFromYourHandByYou branch head (the Myojin cycle's etbCounter
// CheckSVar$ gate) and the Card.wasCastFromYourHandByYou filter predicate:
// obj's latest PutOnStack event names the cast that put it on the stack —
// From is the zone the cast came from, Player the caster. Provenance is
// game-long, so the scan is not bounded by the turn; if the card was later
// cast again from another zone, the latest cast wins. Derived from the event
// log like SpellsCastThisTurnMatching, so a replay derives the same answer.
func (e *Engine) WasCastFromHandByYou(obj state.ObjID, p state.PlayerID) bool {
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.PutOnStack && ev.Obj == obj {
			return ev.From == state.ZHand && ev.Player == p
		}
	}
	return false
}

// WasCastFromHand satisfies effects.Host's WasCastFromHand for the BARE
// wasCastFromYourHand filter family (task castprov3 — the "from anywhere
// other than your hand" carriers whose scripts spell the predicate without
// the ByYou suffix: Vega the Watcher, Bilbo Thief in the Night, Mm'menon's
// RestrictValid$): obj's latest PutOnStack event names the cast that put it
// on the stack, and From is the zone that cast came from — ANY caster. Every
// carrier that needs player scoping supplies it elsewhere (measured over the
// 46 raw carrier files: ValidActivatingPlayer$ You on the trigger lines,
// YouCtrl or wasCastByYou in the same Affected$/Count spec). A copy was
// never cast (the rules-side split, castProvenanceAdmits, applies the same
// IsCopy guard; this read answers the log question alone); a card never put
// on the stack (cheated into play) reads false. Derived from the event log
// like WasCastFromHandByYou, so a replay derives the same answer;
// latest-cast-wins.
func (e *Engine) WasCastFromHand(obj state.ObjID) bool {
	from, _, ok := e.latestCastOrigin(obj)
	return ok && from == state.ZHand
}

// WasCastFromExile satisfies effects.Host's WasCastFromExile for the
// Count$wasCastFromExile branch head (task wascastfrom): obj's LATEST
// PutOnStack cast came from EXILE (foretell, warp, may-play — no CastFlags
// bit carries an exile origin). Derived from the event log the way
// WasCastFromHand is, so a replay derives the same answer; a copy was never
// cast (the rules-side split applies that guard, this read answers the log
// question alone); a card never put on the stack reads false.
func (e *Engine) WasCastFromExile(obj state.ObjID) bool {
	from, _, ok := e.latestCastOrigin(obj)
	return ok && from == state.ZExile
}

// DiscardedInWindow satisfies effects.Host's DiscardedInWindow for the
// ConditionDefined$ Discarded group's cost-discard channel (task
// mordorparams1, Moria Scavenger's "If the discarded card was a creature
// card"): the events.DiscardCost records of obj's own activation, read off
// the log. The window walks BACKWARD from the log end and stops at the
// first event that proves a different resolution boundary — another
// wrapper's push (a different activation's AbilityPush/PutOnStack/trigger
// push), a step or turn change, a pool clear or a player loss — while
// crossing obj's OWN push events, because the two cost orderings share the
// one rule: an ability's cost parts are paid BEFORE its AbilityPush mints
// the wrapper (rules/cast.go's activation branch), a spell's AFTER its
// PutOnStack (the spell branch), and no other wrapper's push can sit
// between a cost discard and the resolution that follows it. Priority
// passes are deliberately NOT a boundary: an activated ability can sit on
// the stack across any number of passes before it resolves, and the
// discard it paid belongs to exactly that resolution. Derived from the log
// the way WasCastFromHandByYou is, so a replay derives the same answer.
func (e *Engine) DiscardedInWindow(obj state.ObjID) []state.ObjID {
	if obj == 0 {
		return nil
	}
	// An ability wrapper's AbilityPush carries the SOURCE permanent's id
	// (events.Apply mints the wrapper; Event.Obj names its source), so the
	// scan crosses its own push by wrapper id or source id alike.
	var src state.ObjID
	if o := e.G.Obj(obj); o != nil {
		src = o.Source
	}
	var out []state.ObjID
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		switch ev.Kind {
		case events.MoveZone:
			if events.IsDiscardCost(ev) {
				out = append(out, ev.Obj)
				continue
			}
			// An ordinary move inside the window is not a boundary — an
			// ability's payment can move several cards (exile parts, tapped
			// entries) between its discard and its push.
			continue
		case events.PutOnStack, events.AbilityPush, events.TriggerPush,
			events.DelayedPush, events.GrantTriggerPush:
			if ev.Obj == obj || (src != 0 && ev.Obj == src) {
				continue // the resolving object's own push: cross it
			}
			return reverseIDs(out)
		case events.StepChange, events.TurnChange, events.ManaClear, events.PlayerLost:
			return reverseIDs(out)
		}
	}
	return reverseIDs(out)
}

// reverseIDs restores log order to a backward scan's collection.
func reverseIDs(in []state.ObjID) []state.ObjID {
	for i, j := 0, len(in)-1; i < j; i, j = i+1, j-1 {
		in[i], in[j] = in[j], in[i]
	}
	return in
}

// WasCast satisfies effects.Host's WasCast (Forge Card.wasCast():
// castFrom != null), the Count$IfCastInOwnMainPhase third conjunct (task
// ifcastmain1). The pending CR 601.2c announcement ask is a cast in progress:
// pushCast runs AFTER targetAsk, so the log scan alone would misread Return
// to Dust's own TargetMax$ X bound as uncast; the live pending cast closes
// that window (Forge sets castFrom before setupTargets). !e.cast.isAbility()
// excludes an ACTIVATED-ABILITY activation (printed or granted, task
// grantcost1), which Forge never treats as a cast. A copy was never cast
// (IsCopy), and a card never put on the stack (cheated into play) reads
// false. Derived from the event log plus the live pending cast, so a replay
// derives the same answer.
func (e *Engine) WasCast(obj state.ObjID) bool {
	if e.cast != nil && e.cast.card == obj && !e.cast.isAbility() {
		return true
	}
	if o := e.G.Obj(obj); o == nil || o.IsCopy {
		return false
	}
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.PutOnStack && ev.Obj == obj {
			return true
		}
	}
	return false
}

// WasCastByYou reports whether card obj was CAST AT ALL by player p — the
// bare wasCastByYou qualifier's engine read (task castprov2: the "When
// CARDNAME enters, if you cast it" ETB family — Zacama, Marina Vendrell's
// Grimoire — and Nine-Lives Familiar's etbCounter gate field): the LATEST
// PutOnStack event for this object names you as caster, whatever zone the
// cast came from (a normal hand cast, a flashback, any origin — the oracle's
// "if you cast it" does not care where from). LATEST-cast, not exists-anywhere:
// the battlefield entry this gate answers for followed the latest cast, so
// that cast is the provenance the oracle means; the corner this leaves is
// you cast it, it left the battlefield again, and an OPPONENT later cast the
// same object — the gate then reads false even though you did cast it
// (measured: no corpus carrier exercises the corner; an exists-scan would
// instead answer true for a card whose latest cast was an opponent's, the
// wider wrong). Copies were never cast; the rules-side split
// (castProvenanceAdmits) applies that guard, this read answers the log
// question alone. Derived from the event log like WasCastFromHandByYou, so
// a replay derives the same answer; a card never put on the stack (cheated
// into play) reads false. Shared approximation with the hand read: the scan
// cannot distinguish a cast from a later un-cast re-entry's provenance.
func (e *Engine) WasCastByYou(obj state.ObjID, p state.PlayerID) bool {
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.PutOnStack && ev.Obj == obj {
			return ev.Player == p
		}
	}
	return false
}

// combatHit snapshots one landed combat-damage-to-player instance for the
// per-turn ledger. The dealing object is read through g.Obj at damage time
// (it is still on the battlefield then); its *cards.Card face pointer and
// controller are copied into the hit so a later reader can match the spec
// after the source has died, left the battlefield or been turned face down.
func (e *Engine) combatHit(player state.PlayerID, source state.ObjID, amount int32) effects.CombatDamageHit {
	hit := effects.CombatDamageHit{Player: player, Source: source, Amount: amount}
	if o := e.G.Obj(source); o != nil {
		hit.Card = o.Card
		hit.FaceIdx = o.FaceIdx
		hit.Controller = o.Controller
	}
	return hit
}

// CombatDamageToPlayersThisTurn satisfies effects.Host's
// CombatDamageToPlayersThisTurn: every combat-damage instance dealt to a
// player so far this turn, in assignment order, as captured at the combat
// damage site (runCombatAssignments). Engine-side, NO-EVENT state that every
// rebuild re-derives; emit clears it on TurnChange.
func (e *Engine) CombatDamageToPlayersThisTurn() []effects.CombatDamageHit {
	return e.combatHitsThisTurn
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

// CountersRemovedThisTurn satisfies effects.Host's CountersRemovedThisTurn
// for Count$CountersRemovedThisTurn <KIND> <player> (Blaster Hulk's per-{E}
// cast discount and Izzet Generatorium's paid-or-lost-four-or-more {E}
// activation gate): the TOTAL of player counters of kind p paid or lost this
// turn, summed from every negative-Amount PlayerCounterChange naming the kind
// (case-insensitively — the grants and the pays write the same kind text a
// card's script uses, e.g. "ENERGY") since the last TurnChange. Derived from
// the event log like LifeLostThisTurn, so a replay derives the same number.
// A payment and a loss are the same event shape — rules/mana.go's PayEnergy
// settle emits exactly this fold's input — and an object-counter removal
// (Kind CounterChange, a permanent losing counters) is deliberately NOT
// folded: the head's player form counts the PLAYER's pool only.
func (e *Engine) CountersRemovedThisTurn(p state.PlayerID, kind string) int32 {
	var n int32
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.PlayerCounterChange && ev.Player == p && ev.Amount < 0 &&
			strings.EqualFold(ev.Counter, kind) {
			n += -ev.Amount
		}
	}
	return n
}

// DamageTakenThisTurn satisfies effects.Host's DamageTakenThisTurn for the
// TargetedPlayer$DamageThisTurn count head (Knollspine Dragon's "draw cards
// equal to the damage dealt to target opponent this turn"): the total damage
// p was dealt this turn, summed from every player-targeted Damage event
// since the last TurnChange. A player hit is Kind Damage with Player set
// and Obj 0 — an object hit sets Obj and leaves Player 0 (seat 0 is a real
// player, so the discriminator is Obj == 0, never Player != 0); a
// replacement-rewritten Note never reaches this fold, and a redirect that
// moved a hit onto a permanent reads there instead. Derived from the event
// log like LifeLostThisTurn, so a replay derives the same number.
func (e *Engine) DamageTakenThisTurn(p state.PlayerID) int32 {
	var n int32
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.Damage && ev.Obj == 0 && ev.Player == p && ev.Amount > 0 {
			n += ev.Amount
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

// CardsDiscardedThisTurn satisfies effects.Host's CardsDiscardedThisTurn for
// PlayerCountPropertyYou$CardsDiscardedThisTurn (Ambergris Citadel Agent's
// "X = cards you discarded this turn"): every events.IsDiscard move since
// the last TurnChange naming p — the ordinary Discard form by its Player
// field, the cost form (events.DiscardCost, which carries no Player — every
// emitter constructs it without one, so the Player field is seat 0 regardless
// of who paid) by the discarded object's owner alone, since a cost discard is
// paid from the payer's own hand (CR 118.2a). Classifying by the marker and
// not by "Player == 0 as a fallback" is what keeps seat 0's tally from
// counting every other seat's cost discard. Derived from the event log like
// LifeLostThisTurn, so a replay derives the same number.
func (e *Engine) CardsDiscardedThisTurn(p state.PlayerID) int32 {
	var n int32
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if !events.IsDiscard(ev) {
			continue
		}
		if events.IsDiscardCost(ev) {
			if o := e.G.Obj(ev.Obj); o != nil && o.Owner == p {
				n++
			}
			continue
		}
		if ev.Player == p {
			n++
		}
	}
	return n
}

// CardsDrawnThisTurn satisfies effects.Host's CardsDrawnThisTurn for the
// PlayerCount<group>$Condition<N> CardsDrawn property (Smuggler's Share's
// "draw a card for each opponent who drew two or more cards this turn"):
// every events.Draw naming p since the last TurnChange. Derived from the
// event log like CardsDiscardedThisTurn, so a replay derives the same
// number. Every draw emitter — the draw step, an effect's Draw and the
// opening hand — emits the same event kind with Player set, so the fold
// counts them all, exactly as Forge's per-turn cardsDrawn list does.
func (e *Engine) CardsDrawnThisTurn(p state.PlayerID) int32 {
	var n int32
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind == events.Draw && ev.Player == p {
			n++
		}
	}
	return n
}

// SpellsCastThisTurnBy satisfies effects.Host's SpellsCastThisTurnBy for the
// PlayerCount<group>$Condition<N> SpellsCastThisTurn property (Ertai's
// Scorn / Mindbreak Trap / Whiplash Trap: "for each opponent who cast two
// or more spells this turn"): every PutOnStack naming p since the last
// TurnChange. Derived from the event log like CastThisTurn, so a replay
// derives the same number.
func (e *Engine) SpellsCastThisTurnBy(p state.PlayerID) int {
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

// StartingLife satisfies effects.Host's StartingLife: the opening life total
// genesis resolved (Config.StartingLife's 0-means-20 convention already
// applied in newWithRNG). Captured at construction and copied by Clone, so a
// replay derives the same value.
func (e *Engine) StartingLife() int32 { return e.startingLife }

// TurnsTaken satisfies effects.Host's TurnsTaken for Count$YourTurns (Serra
// Avenger's "your first, second, or third turns of the game"): the number of
// turns that have BEGUN with p as the active player, current turn included.
// Derived from the event log like LifeLostThisTurn — every turn p begins
// emits exactly one TurnChange naming p (events.Apply's TurnChange case),
// extra turns included, so a replay that rebuilds the log arrives at the
// same count. The whole-log walk (not a TurnChange-bounded scan) is the
// point: the count spans the game, not one turn.
// CommanderCastsFromCommandZone satisfies effects.Host's method of the
// same name for Count$TotalCommanderCastFromCommandZone: how many times
// player p has cast one of THEIR OWN commanders from the command zone this
// game. It walks the whole log for PutOnStack events whose caster is p,
// origin is the command zone and object is one of p's commanders — the
// exact criteria recordCmdCast (rules/cast.go) applies when it maintains
// the parallel CmdCasts slice, and commitCast's PutOnStack emit is the ONE
// site that can produce such an event, so this head and the CR 903.8 tax
// can never disagree. Whole-game scope like TurnsTaken (the whole-log walk
// is the point); derived from the event log, so a replay derives the same
// number. A non-Commander seat carries an empty Commanders list, so the
// count is 0 there by construction.
func (e *Engine) CommanderCastsFromCommandZone(p state.PlayerID) int32 {
	if p < 0 || int(p) >= len(e.G.Players) {
		return 0
	}
	var n int32
	for _, ev := range e.L.Events {
		if ev.Kind != events.PutOnStack || ev.Player != p || ev.From != state.ZCommand {
			continue
		}
		for _, cid := range e.G.Players[p].Commanders {
			if cid == ev.Obj {
				n++
				break
			}
		}
	}
	return n
}

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
// UnlessCost$ to payer p. Sacrifice, discard and reveal components are
// deliberately refused here: beginUnlessPayment owns every such component
// and gathers the payer's selected objects before it calls payMana. Keeping
// this guard makes a future caller unable to silently revive the old
// first-in-zone-order stand-in. Fixed mana/life, SubCounter and Draw
// components remain synchronous: a Draw<N/Spec> pays by drawing N cards for
// the player(s) the spec names (default the payer), resolved through the
// same Ctx roles the UnlessPayer$ grammar reads.
func (e *Engine) payUnlessCost(p state.PlayerID, cost Cost, ctx *effects.Ctx, stackObj state.ObjID) bool {
	if len(cost.Sac) != 0 || len(cost.Discard) != 0 || len(cost.Reveal) != 0 || len(cost.RevealChosen) != 0 {
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
