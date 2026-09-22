// Cast, copy and ability-cast trigger modes.
//
// Mode$ SpellCast, SpellCastOrCopy, SpellCopy, AbilityCast and SpellAbilityCast,
// with the cast evaluation and the mana/alternative-cost gates they share.
//
// Split out of trigger_match.go so tickets touching different modes stop
// colliding on one file. Registration is at the bottom; a duplicate mode
// panics (registerTrigMatcher).

package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// spellCastMatches implements Mode$ SpellCast: ValidCard$ and
// ValidActivatingPlayer$ against a PutOnStack event, plus the two cast-
// condition clauses the measured cards carry -- ActivatorThisTurnCast$
// (The Lord of Pain, Vial Smasher the Fierce: <OP><N> over the spells the
// ACTIVATOR has cast this turn, the current cast included -- its PutOnStack
// is already in the log when the deferred trigger fires) and ValidSA$
// (Roiling Vortex: the Spell.ManaSpent <OP><value> comparison family).
func (e *Engine) spellCastMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.PutOnStack {
		return false
	}
	return e.spellCastEval(t, source, ev)
}

// spellCopyMatches is the copy half of the spell-cast family: Mode$
// SpellCastOrCopy delegates a StackCopy event here, and Mode$ SpellCopy
// ("Whenever you copy a spell, ...", the_parnesse_the_subtle_brush shape)
// is its only mode. Copies do not re-enter the stack as a PutOnStack --
// effects/copy.go emits events.StackCopy naming the ORIGINAL spell (ev.Obj,
// still on the stack; events.Apply's StackCopy case rejects anything else)
// with ev.Player the copy's controller -- and the copy object shares the
// original's card, face and CastFlags, so the shared evaluation below reads
// the copied spell identically. A plain SpellCast trigger stays silent here
// (a copy is not a cast) and a SpellCastOrCopy/SpellCopy trigger does not
// fire on the cast of the spell itself -- that half of the division is
// spellCastMatches'.
func (e *Engine) spellCopyMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.StackCopy {
		return false
	}
	return e.spellCastEval(t, source, ev)
}

// spellCastEval is the spell evaluation spellCastMatches and
// spellCopyMatches share, minus the entering-the-stack guard each mode owns:
// ValidCard$ (through the trigger-side cast alternatives and the
// cast-provenance qualifiers), ValidActivatingPlayer$, the
// ActivatorThisTurnCast[Each] counts, ValidSA$ and HasXManaCost$ -- read off
// ev.Obj (the spell being cast, or the original being copied) and ev.Player
// (the cast's or the copy's controller) exactly alike.
func (e *Engine) spellCastEval(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	// Casting a spell means an actual card entering the stack (a copy
	// evaluates the ORIGINAL, which Apply's StackCopy case guarantees is on
	// the stack). This build also uses PutOnStack-shaped Move()s for nothing else today (triggered
	// abilities go on the stack via a dedicated TriggerPush event --
	// putTriggersOnStack, above, and events.Apply's TriggerPush case), but a
	// Face()-less object could otherwise satisfy a bare "Any"/"Spell"
	// ValidCard$ regardless (matchesBase's Spell/Any cases don't consult
	// Face()), so this guard holds regardless of how a future ability-object
	// path might reach here. Ruling F3.
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	ctrl := e.controllerOf(source)
	// The ValidCard alternatives (post trigger-side self-cast exclusion) are
	// computed ONCE: the cast's own match below and the
	// ActivatorThisTurnCastEach$ tally must read the same surviving set, so
	// the two can never disagree (the Each arm below reads only the alts this
	// block computed).
	var castAlts []triggerCastAlt
	if v, ok := t.Params["ValidCard"]; ok {
		alts, ok2 := e.triggerCastAlternatives(v, source, ev.Obj)
		if !ok2 {
			return false
		}
		spec := ""
		for i, alt := range alts {
			if i > 0 {
				spec += ","
			}
			spec += alt.spec
		}
		// The bare wasCastFromYourHandByYou / wasCastByYou qualifiers (the
		// cast-provenance families, tasks castprov1/castprov2) are split out
		// and evaluated against the log here; the remainder matches as before.
		spec, ok3 := e.castProvenanceAdmits(spec, ev.Obj, ctrl)
		if !ok3 || !effects.MatchesSpecCtx(e.G, spec, ev.Obj, e.specCtx(source, ctrl)) {
			return false
		}
		castAlts = alts
	}
	if v, ok := t.Params["ValidActivatingPlayer"]; ok {
		if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	if v, ok := t.Params["ActivatorThisTurnCast"]; ok {
		if !compareIntCount(int32(e.spellsCastThisTurn(ev.Player)), v) {
			return false
		}
	}
	if v, ok := t.Params["ActivatorThisTurnCastEach"]; ok {
		// The PER-ALTERNATIVE first-cast read (task castprov2, Alania,
		// Divergent Storm — the corpus's one carrier): the trigger fires when
		// the activator's cast is the FIRST this turn of at least ONE
		// ValidCard$ alternative — a disjunction of firsts, so casting an
		// instant and then a sorcery fires on the sorcery. The tally is the
		// activator's casts this turn matching that alternative, the current
		// cast INCLUDED (it is already in the log when the deferred trigger
		// fires; EQ1 means this cast is the first).
		//
		// Only alternatives the CURRENT CAST MATCHES are evaluated (round-2
		// review): the oracle reads "if it's the first instant ... you've cast
		// this turn" — the firstness must attach to the spell being cast, so
		// a second instant after instant→sorcery does NOT fire on the
		// sorcery alternative's earlier tally (that sorcery was the first
		// sorcery, but this cast is not one). An alternative whose tally is
		// met while the current cast matches a DIFFERENT alternative is
		// skipped; without the guard the trigger false-fires on any
		// two-cast-then-repeat turn. castAlts comes from the same surviving
		// set the cast's own match above used, so the two cannot disagree.
		//
		// A trigger carrying the Each param with NO ValidCard$ has no
		// alternatives to attach the firstness to: UNSUPPORTED, fail closed
		// (castAlts stays nil; the corpus's one carrier, Alania, always has
		// a ValidCard$).
		fired := false
		for _, alt := range castAlts {
			if !effects.MatchesSpecCtx(e.G, alt.spec, ev.Obj, e.specCtx(source, ctrl)) {
				continue
			}
			if compareIntCount(int32(e.spellsCastThisTurnByMatching(ev.Player, alt.spec, alt.exclSelf, source)), v) {
				fired = true
				break
			}
		}
		if !fired {
			return false
		}
	}
	if v, ok := t.Params["ValidSA"]; ok {
		if !e.validSAMatches(source, ev, ctrl, v) {
			return false
		}
	}
	if !hasXManaCostGate(t.Params, obj.Face().ManaCost, nil) {
		return false
	}
	return true
}

// abilityCastMatches implements Mode$ AbilityCast and Mode$ SpellAbilityCast
// against an AbilityPush event -- the moment an activated ability is put on
// the stack (rules/cast.go's commitCast). This is the COMPLETED boundary: an
// AbilityPush is emitted only once the activation's cost is fully paid and
// the ability object is minted, so a trigger firing here is never observing a
// provisional or abandoned activation. (F15: there was no such arm at all, so
// Rings of Brighthearth's "Whenever you activate an ability, if it isn't a
// mana ability..." trigger never fired.)
//
// ValidActivatingPlayer$ and ValidSA$ narrow the activation the trigger
// observes, in the same two params the corpus spells them with. A source
// permanent whose face has no Abilities at the recorded index (stale data) is
// a no-op, never a panic.
func (e *Engine) abilityCastMatches(t cards.Trigger, source state.ObjID, ev events.Event) bool {
	if ev.Kind != events.AbilityPush {
		return false
	}
	obj := e.G.Obj(ev.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	ctrl := e.controllerOf(source)
	if v, ok := t.Params["ValidActivatingPlayer"]; ok {
		// ev.Player is the player who activated the ability;
		// MatchesPlayerSpec resolves "You" as the trigger's controller.
		if !effects.MatchesPlayerSpec(e.G, v, ev.Player, ctrl) {
			return false
		}
	}
	// CR 702.140d: an AbilityPush's Amount is the FLAT pile-ability index
	// events.Apply decoded the mint from, so an under-card activation sits
	// past the top face's list. Resolve through the same pile view the
	// emitter, the offer loop and events.Apply share -- reading
	// obj.Face().Abilities here would make a ValidSA$ narrowing fail closed
	// on every under-card activation and degrade a HasXManaCost$ gate to the
	// pile TOP's printed cost (a wrong-wide pass whenever the top card's cost
	// carried {X}). A plain permanent's flat index is unchanged.
	pa, havePa := obj.PileAbilityAt(int(ev.Amount))
	if v, ok := t.Params["ValidSA"]; ok {
		if !havePa {
			return false
		}
		if !abilityCastValidSA(pa.SA, v) {
			return false
		}
	}
	// HasXManaCost$ True: the activation cost must contain {X}. The ability
	// at the recorded index (the same bounds check ValidSA$ uses); a stale
	// index fails closed through the nil ab below.
	var ab *cards.SA
	if havePa {
		ab = pa.SA
	}
	if !hasXManaCostGate(t.Params, "", ab) {
		return false
	}
	return true
}

// abilityCastValidSA reports whether an activated ability matches a ValidSA$
// narrowing on an AbilityCast/SpellAbilityCast trigger. The grammar is Forge's
// comma-separated OR list of "<kind>.<constraint>" values; a value whose kind
// names a spell (Spell/Instant/Sorcery) describes a cast, not an activation,
// so it never matches an activated ability and is simply skipped. An absent
// or unqualified value matches every activated ability.
func abilityCastValidSA(ab *cards.SA, validSA string) bool {
	v := strings.TrimSpace(validSA)
	if v == "" {
		return true
	}
	for _, alt := range strings.Split(v, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		kind, constraint := alt, ""
		if i := strings.IndexByte(alt, '.'); i >= 0 {
			kind, constraint = alt[:i], alt[i+1:]
		}
		switch kind {
		case "SpellAbility", "Activated", "":
			switch constraint {
			case "":
				return true
			case "!ManaAbility":
				if !isManaAbilityAPI(ab.API) {
					return true
				}
			case "ManaAbility":
				if isManaAbilityAPI(ab.API) {
					return true
				}
			}
		}
	}
	return false
}

// hasXManaCostGate implements HasXManaCost$ True on cast/activation trigger
// modes (Mode$ SpellCast and Mode$ AbilityCast/SpellAbilityCast): "whenever
// you cast a permanent spell with a mana cost that contains {X}" / "...or
// activate an ability ... if that ability's activation cost contains {X}"
// (Unbound Flourishing, Glava Five-Advents Mage, Brass Infiniscope, Magus
// Lucea Kane). The gate reads the PRINTED cost -- the spell's ManaCost face
// field, or the ability's Cost$ param -- through ParseCost, which counts only
// the mana {X} tokens (c.X); a non-mana component token such as
// SubCounter<X/CHARGE> or PayEnergy<X> is matched by its own grammar and does
// NOT count, which is exactly the card text's "mana cost/activation cost
// contains {X}". For an activation the printed Cost$ still carries the X at
// AbilityPush time (the announced value folds into the provisional payment
// cost, rules/cast.go's pc.cost.WithX, never into the SA params), so the
// gate is about the printed shape, never the announced value.
//
// A param present with a value other than True FAILS CLOSED (the trigger
// stays silent), per the repo's unreadable-condition convention; a param
// absent leaves the trigger's behaviour unchanged. ab is the activated
// ability on the AbilityCast arm (nil on the SpellCast arm, where faceCost is
// the spell's printed face cost); a nil ab with an empty faceCost fails
// closed rather than firing wide.
func hasXManaCostGate(params map[string]string, faceCost string, ab *cards.SA) bool {
	v, ok := params["HasXManaCost"]
	if !ok {
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(v), "True") {
		return false
	}
	if ab != nil {
		return ParseCost(ab.Params["Cost"]).X > 0
	}
	return ParseCost(faceCost).X > 0
}

// manaSpentForCast sums the mana the player spent casting the spell ev put
// on the stack: every negative ManaAdd for that player since the spell's
// own PutOnStack. Between CR 601.2a's push and the deferred cast trigger
// the only mana leaving a pool is this cast's payment (a mana window adds
// mana, it never spends), and the scan reads the log, so a replay derives
// the identical number.
func (e *Engine) manaSpentForCast(p state.PlayerID, id state.ObjID) int32 {
	var spent int32
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.PutOnStack && ev.Obj == id && ev.Player == p {
			return spent
		}
		if ev.Kind == events.ManaAdd && ev.Player == p && ev.Amount < 0 {
			spent += -ev.Amount
		}
	}
	return spent
}

// validSAMatches evaluates a SpellCast trigger's ValidSA$ clause. The
// measured grammar is the mana comparison family, "Spell.ManaSpent <OP><N>"
// (Roiling Vortex's EQ0 -- no mana was spent to cast that spell; Raggadragga
// and the emperor's GE7/EQ0 shapes read the same head): the value is the
// mana the ACTIVATOR paid for the cast, so GTX/other dynamic values fail
// closed. A clause naming an unmodelled property (ManaSpentBy, MayPlaySource,
// Self, YouCtrl) or carrying no comparison is evaluated as a plain spec
// filter over the cast spell when it is a single field, and fails closed
// otherwise.
func (e *Engine) validSAMatches(source state.ObjID, ev events.Event, ctrl state.PlayerID, clause string) bool {
	fields := strings.Fields(strings.TrimSpace(clause))
	switch len(fields) {
	case 1:
		return effects.MatchesSpecCtx(e.G, fields[0], ev.Obj, e.specCtx(source, ctrl))
	case 2:
		if fields[0] == "Spell.ManaSpent" {
			return compareIntCount(e.manaSpentForCast(ev.Player, ev.Obj), fields[1])
		}
		return false
	}
	return false
}

// spellCastPermanentSpec rewrites the leading `Permanent` base token of
// every comma-alternative in a SpellCast trigger's ValidCard$ spec to
// `PermanentCard`, so the "whenever you cast a permanent spell" family
// (Unbound Flourishing, the Defiler cycle, Archmage of Echoes) reads the
// base as a permanent card at PutOnStack -- the evaluated object there is
// always the cast spell, and CR 109.2 makes an artifact/creature/
// enchantment/planeswalker/battle spell a permanent spell. matchesBase's
// bare `Permanent` keeps the on-the-battlefield reading every other filter
// depends on and is deliberately untouched. rules/stack.go's
// targetSpecForZone and effects/cardflow.go's permanentCardSpec carry the
// same leading-token rule with a zone gate (both leave ZStack unchanged);
// this third copy applies it WITHOUT a zone gate and ACROSS alternatives,
// because a comma-separated spec names each alternative's own base
// (Archmage of Echoes' `Permanent.Faerie,Permanent.Wizard`). effects must
// not import rules and vice versa, so the copies cannot share code. The
// rewritten text is not the original spec, so the compiled sidecar's
// byText lookup misses and the textual oracle answers it -- which after the
// matchesBase/matchesCompiledBase relaxation is correct.
func spellCastPermanentSpec(spec string) string {
	// Comma-split at angle-bracket depth 0: a named<X, Y> name argument's
	// printed comma is not an alternative boundary (the same distinction
	// effects.filterAlternatives draws).
	depth, start := 0, 0
	var b strings.Builder
	for i := 0; i < len(spec); i++ {
		switch spec[i] {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				if start > 0 {
					b.WriteByte(',')
				}
				b.WriteString(spellCastLeadingPermanentToCard(spec[start:i]))
				start = i + 1
			}
		}
	}
	if start == 0 {
		return spellCastLeadingPermanentToCard(spec)
	}
	b.WriteByte(',')
	b.WriteString(spellCastLeadingPermanentToCard(spec[start:]))
	return b.String()
}

// spellCastLeadingPermanentToCard is the leading-token rule targetSpecForZone
// and permanentCardSpec share: the whole spec is `Permanent`, or its leading
// token is `Permanent` followed by '.', '+' or ',' -- only that token is
// rewritten, every qualifier rides along.
func spellCastLeadingPermanentToCard(alt string) string {
	if alt == "Permanent" {
		return "PermanentCard"
	}
	if len(alt) > len("Permanent") && alt[:len("Permanent")] == "Permanent" {
		switch alt[len("Permanent")] {
		case '.', '+':
			return "PermanentCard" + alt[len("Permanent"):]
		}
	}
	return alt
}

// triggerSnapshot is immutable look-back state. Parked replacement choices
// may retain it across intent/Clone boundaries; each matching walk constructs
// its own Engine scratch caches, never mutating or sharing the snapshot's.
type triggerSnapshot struct {
	game       *state.Game
	continuous []ContinuousEffect
}

// triggerCastAlternatives splits a Mode$ SpellCast trigger's ValidCard$ into
// its comma alternatives and applies the trigger-side bare !CastSaSource
// reading (task castprov2, Alania, Divergent Storm — measured: exactly 1
// trigger-side line in the corpus): an alternative carrying the token names
// "a spell whose PRINTED NAME is not the trigger SOURCE's printed name" (the
// oracle's "other than NICKNAME"; Forge has no trigger-side CastSaSource
// grammar precedent in this corpus, so this reading is a documented
// decision, not a discovered fact). The cast card's printed name equal to
// the source's drops the alternative (a second Alania cast fails the Otter
// alternative and does not trigger); the token is otherwise stripped and the
// alternative kept for the ordinary filter. ok is false when no alternative
// survives.
func (e *Engine) triggerCastAlternatives(rawSpec string, source, castObj state.ObjID) ([]triggerCastAlt, bool) {
	spec := spellCastPermanentSpec(rawSpec)
	sourceName, castName := "", ""
	if o := e.G.Obj(source); o != nil && o.Face() != nil {
		sourceName = o.Face().Name
	}
	if o := e.G.Obj(castObj); o != nil && o.Face() != nil {
		castName = o.Face().Name
	}
	isSelf := sourceName != "" && castName == sourceName
	var alts []triggerCastAlt
	for alt := range effects.FilterAlternatives(spec) {
		s, had := effects.StripPredicateToken(alt, "!CastSaSource")
		if had && isSelf {
			continue
		}
		alts = append(alts, triggerCastAlt{spec: s, exclSelf: had})
	}
	if len(alts) == 0 {
		return nil, false
	}
	return alts, true
}

// spellsCastThisTurnByMatching counts player p's spells cast this turn whose
// object matches ONE alternative spec, with the trigger-side NICKNAME
// exclusion applied to the tally when that alternative carried the
// !CastSaSource token (Alania's "the first Otter spell other than Alania":
// the source's own casts do not count toward that alternative's first).
// The current cast is INCLUDED — it is already in the log when the deferred
// trigger fires, and the oracle's "first ... you've cast this turn" counts
// it (EQ1 = this cast is the first).
func (e *Engine) spellsCastThisTurnByMatching(p state.PlayerID, spec string, exclSelf bool, source state.ObjID) int {
	selfName := ""
	if exclSelf {
		if o := e.G.Obj(source); o != nil && o.Face() != nil {
			selfName = o.Face().Name
		}
	}
	n := 0
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.TurnChange {
			break
		}
		if ev.Kind != events.PutOnStack || ev.Player != p {
			continue
		}
		o := e.G.Obj(ev.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		if selfName != "" && o.Face().Name == selfName {
			continue
		}
		if effects.MatchesSpecFrom(e.G, spec, ev.Obj, p, ev.Obj) {
			n++
		}
	}
	return n
}

func init() {
	registerTrigMatcher(func(e *Engine, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return e.spellCastMatches(t, source, ev)
	}, "SpellCast")
	// The magecraft family: the cast half is the ordinary SpellCast evaluation
	// on the PutOnStack event; the copy half delegates a StackCopy event to
	// spellCopyMatches. A copy never re-enters the stack as a PutOnStack --
	// effects/copy.go emits events.StackCopy naming the original -- so
	// spellCastMatches' entering-the-stack guard would keep the copy half dead
	// if the whole mode fell through to it.
	registerTrigMatcher(func(e *Engine, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		if ev.Kind == events.StackCopy {
			return e.spellCopyMatches(t, source, ev)
		}
		return e.spellCastMatches(t, source, ev)
	}, "SpellCastOrCopy")
	// The copy-only mode ("Whenever you copy a spell, ..."): plain casts are not
	// copies, so a PutOnStack event must not fire it.
	registerTrigMatcher(func(e *Engine, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return e.spellCopyMatches(t, source, ev)
	}, "SpellCopy")
	registerTrigMatcher(func(e *Engine, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		return e.abilityCastMatches(t, source, ev)
	}, "AbilityCast")
	// SpellAbilityCast is the spell-OR-ability mode (Feather, Radiant
	// Arbiter, Unbound Flourishing, Sunken Palace): the AbilityPush half is
	// abilityCastMatches; the PutOnStack half is the ordinary SpellCast
	// evaluation, whose ValidSA$ reads the Spell.* spell-kind alternatives
	// abilityCastValidSA deliberately skips for abilities. Before this
	// matcher existed the cast half was dead: the mode only ever saw
	// AbilityPush events, so Feather's "Whenever you cast a noncreature
	// spell that targets only CARDNAME ..." never fired.
	registerTrigMatcher(func(e *Engine, t cards.Trigger, source state.ObjID, ev events.Event, _ *state.Object) bool {
		if ev.Kind == events.AbilityPush {
			return e.abilityCastMatches(t, source, ev)
		}
		return e.spellCastMatches(t, source, ev)
	}, "SpellAbilityCast")
}
