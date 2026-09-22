// The rules-side split for the cast-provenance filter predicates (tasks
// castprov1 and castprov2). The predFn signature the effects filter runs on
// carries no Host, and a hand-origin cast deliberately carries no CastFlags
// bit (the flags mark alternative costs and origins only), so the provenance
// lives in the event log — Engine reads it through reverse PutOnStack scans,
// the same machinery the Count$ branch heads read. The tokens are therefore
// split OUT of the spec text at the rules-side match sites (where the
// Engine, and its log, is in scope) and evaluated there, with the remainder
// matched by the ordinary filter. The precedent is the spec-rewrite helpers
// the engine already keeps (spellCastPermanentSpec, permanentCardSpec).
//
// The two families, and their exact semantics:
//
//   - wasCastFromYourHandByYou (castprov1): the object's LATEST PutOnStack
//     event names the cast, and it was from hand, by you.
//   - wasCastByYou (castprov2, the "When CARDNAME enters, if you cast it"
//     ETB family — Zacama, Marina Vendrell's Grimoire, Nine-Lives
//     Familiar's etbCounter gate): SOME PutOnStack event for this object
//     names you as caster — the oracle's "if you cast it" does not care
//     where from. A copy was never cast (the same IsCopy guard both reads
//     take). Shared approximation of both scans: they read only the event
//     log's casts, so they cannot distinguish a card that was cast and then
//     re-entered play without a cast (reanimated and friends) — the
//     exists-scan still answers "you did cast it, earlier", which is the
//     oracle's own wording.
//
//   - wasCastFromYourHand (castprov3): the object's LATEST PutOnStack event
//     names the cast AND that cast came from a hand — ANY caster. The bare
//     spelling is the one the "from anywhere other than your hand" carriers
//     print (Vega the Watcher, Bilbo Thief in the Night, Mm'menon's
//     RestrictValid$); every carrier that needs player scoping supplies it
//     elsewhere (ValidActivatingPlayer$ You on the trigger lines, YouCtrl or
//     wasCastByYou in the same Affected$/Count spec), measured over the 46
//     raw carrier files. A copy was never cast (the same IsCopy guard both
//     existing families take); a card never put on the stack reads false.
//
//   - the origin-zone family (task wascastfrom): wasCastFromExile,
//     wasCastFromYourGraveyard, wasCastFromYourGraveyardByYou and
//     wasCastFromTheirHand — the object's LATEST PutOnStack cast came from
//     the named zone (and, for the YourGraveyard spellings, was made by the
//     evaluating you). The bare wasCastFromGraveyard spelling is NOT here:
//     it is the effects-side CastFlags predicate (FlagFlashback | Harmonize
//     | Escaped, effects/filter.go), which the rules-side strip must leave
//     in place for the effects-only call sites (the ConditionPresent gates,
//     the target ValidTgts$ matching) — and the log read and the flag read
//     agree on every shape the flag covers, so a rules-side site that strips
//     the other four never disagrees with an effects-side site that keeps it.
//     The same latest-cast-wins and copy-was-never-cast guards the hand
//     families take apply here.
//
// castProvenanceAdmits is the combined entry point every match site calls:
// it evaluates all the families in one pass, so a future carrier mixing the
// tokens in one spec is covered by construction (measured: alex_wilder
// and quandrix_the_proof carry wasCastByYou AND the bare token in one
// alternative).

package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/events"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// admitProvenanceAlternatives splits spec into its comma alternatives (the
// same split the filter draws), strips the provenance predicate — both the
// positive and the !-negated spelling — from every alternative, drops the
// alternatives whose requirement is not met by holds (positive requires
// hold, negated requires !hold), and rejoins the survivors. ok is false when
// no alternative survives: the spec matches nothing. A spec without the
// predicate is returned unchanged with ok true, so every unrelated spec is
// byte-identical.
func admitProvenanceAlternatives(spec, pred string, holds bool) (string, bool) {
	if !strings.Contains(spec, pred) {
		return spec, true
	}
	neg := "!" + pred
	var b strings.Builder
	first := true
	alive := false
	for alt := range effects.FilterAlternatives(spec) {
		s1, hadPos := effects.StripPredicateToken(alt, pred)
		s2, hadNeg := effects.StripPredicateToken(s1, neg)
		if (hadPos && !holds) || (hadNeg && holds) {
			continue
		}
		if !first {
			b.WriteByte(',')
		}
		b.WriteString(s2)
		first = false
		alive = true
	}
	if !alive {
		return "", false
	}
	return b.String(), true
}

// castFromHandAdmits evaluates the bare wasCastFromYourHandByYou /
// !wasCastFromYourHandByYou qualifier of a Forge filter spec against objID:
// every alternative carrying the qualifier but failing the provenance test —
// the object was NOT cast from you's hand by you, or the object is a copy
// (never cast, the same IsCopy guard the count head takes) — is dropped, and
// the surviving alternatives are rejoined. ok is false when no alternative
// survives: the spec matches nothing.
func (e *Engine) castFromHandAdmits(spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	if !strings.Contains(spec, "wasCastFromYourHandByYou") {
		return spec, true
	}
	holds := false
	if o := e.G.Obj(objID); o != nil && !o.IsCopy {
		holds = e.WasCastFromHandByYou(objID, you)
	}
	return admitProvenanceAlternatives(spec, "wasCastFromYourHandByYou", holds)
}

// castAtAllAdmits evaluates the bare wasCastByYou / !wasCastByYou qualifier
// (castprov2): "was cast at all, by you" — some PutOnStack event for this
// object names you as caster, any origin. Copies were never cast.
func (e *Engine) castAtAllAdmits(spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	if !strings.Contains(spec, "wasCastByYou") {
		return spec, true
	}
	holds := false
	if o := e.G.Obj(objID); o != nil && !o.IsCopy {
		holds = e.WasCastByYou(objID, you)
	}
	return admitProvenanceAlternatives(spec, "wasCastByYou", holds)
}

// castFromHandAnyAdmits evaluates the bare wasCastFromYourHand /
// !wasCastFromYourHand qualifier (castprov3): the object's LATEST PutOnStack
// event names the cast and that cast came from a hand, any caster — the
// WasCastFromHand read minus the ByYou families' player comparison. Copies
// were never cast; a card never put on the stack (cheated into play) reads
// false, so a negated alternative holds for it.
//
// ORDER INVARIANT: this helper MUST run after castFromHandAdmits in
// castProvenanceAdmits's chain. The bare token is a SUBSTRING of
// wasCastFromYourHandByYou, so a ByYou spec also contains the bare one;
// StripPredicateToken removes exact tokens (it would never partially mangle
// a ByYou token), but the polarity accounting would be wrong if the bare
// helper ran first — a ByYou spec would be evaluated under the bare,
// caster-less read. ByYou must be stripped (or found absent) first.
func (e *Engine) castFromHandAnyAdmits(spec string, objID state.ObjID) (string, bool) {
	if strings.Contains(spec, "wasCastFromYourHandByYou") || !strings.Contains(spec, "wasCastFromYourHand") {
		return spec, true
	}
	holds := false
	if o := e.G.Obj(objID); o != nil && !o.IsCopy {
		holds = e.WasCastFromHand(objID)
	}
	return admitProvenanceAlternatives(spec, "wasCastFromYourHand", holds)
}

// castProvenanceAdmits evaluates ALL THREE cast-provenance families of a
// Forge filter spec against objID — the hand-origin ByYou one
// (castFromHandAdmits), the any-origin one (castAtAllAdmits) and the bare
// player-less hand one (castFromHandAnyAdmits) — chaining the splits so a
// spec carrying any (or several) of the tokens evaluates fully. The chain
// order is load-bearing: ByYou before bare (see castFromHandAnyAdmits's
// order invariant). This is the one entry point every rules-side match site
// calls, so a future carrier mixing the tokens is covered by construction.
func (e *Engine) castProvenanceAdmits(spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	s, ok := e.castFromHandAdmits(spec, objID, you)
	if !ok {
		return "", false
	}
	s, ok = e.castAtAllAdmits(s, objID, you)
	if !ok {
		return "", false
	}
	s, ok = e.castFromHandAnyAdmits(s, objID)
	if !ok {
		return "", false
	}
	return e.castOriginAdmits(s, objID, you)
}

// castOriginTokens is the origin-zone cast-provenance family (task
// wascastfrom): each token reads the object's LATEST PutOnStack cast (the
// zone the cast came from, optionally scoped to the evaluating you). ByYou
// MUST be listed before the plain YourGraveyard spelling — it is a
// superstring of it and the strip is exact-token, but the polarity must be
// accounted under the ByYou read, the castFromHandAnyAdmits order
// invariant.
type castOriginToken struct {
	token string
	zone  state.Zone
	byYou bool
}

var castOriginTokens = []castOriginToken{
	// "you cast a spell from your graveyard" — the caster casts from their
	// own graveyard; both Forge spellings collapse onto the same read (the
	// ByYou suffix is the player scope the plain spelling takes implicitly:
	// in this build a cast's From is always a zone its caster owns).
	{token: "wasCastFromYourGraveyardByYou", zone: state.ZGraveyard, byYou: true},
	{token: "wasCastFromYourGraveyard", zone: state.ZGraveyard, byYou: true},
	// "whenever you cast a spell from exile" — any caster (the bare family's
	// caster-agnostic reading; every player-scoped carrier supplies
	// ValidActivatingPlayer$ or a You qualifier alongside).
	{token: "wasCastFromExile", zone: state.ZExile},
	// "whenever a player casts a spell from their hand" — "their" is the
	// CASTER's own hand, so the predicate is caster-agnostic too: a cast from
	// a hand IS from the caster's hand in this build (a player never casts
	// from another seat's hand). Aether Revolt's Knowledge Pool, Wash Away's
	// "wasn't cast from its owner's hand" (owner == caster for a spell this
	// build can put on the stack) and Aerial Extortionist's negated form all
	// read it.
	{token: "wasCastFromTheirHand", zone: state.ZHand},
}

// specCarriesCastOrigin reports whether spec carries ANY origin-zone token
// the castOriginAdmits chain strips — the pending guard's cheap gate.
func specCarriesCastOrigin(spec string) bool {
	for _, tok := range castOriginTokens {
		if strings.Contains(spec, tok.token) {
			return true
		}
	}
	return false
}

// latestCastOrigin reports obj's LATEST PutOnStack cast: the zone it came
// from and the player who cast it. ok=false for a card never put on the
// stack (cheated into play) — the same read every existing provenance
// family takes. Derived from the event log like WasCastFromHandByYou, so a
// replay derives the same answer.
func (e *Engine) latestCastOrigin(obj state.ObjID) (state.Zone, state.PlayerID, bool) {
	for i := len(e.L.Events) - 1; i >= 0; i-- {
		ev := e.L.Events[i]
		if ev.Kind == events.PutOnStack && ev.Obj == obj {
			return ev.From, ev.Player, true
		}
	}
	return 0, 0, false
}

// castOriginAdmits evaluates the origin-zone cast-provenance family
// (wasCastFromExile, wasCastFromYourGraveyard, wasCastFromYourGraveyardByYou,
// wasCastFromTheirHand) of a Forge filter spec against objID: every
// alternative carrying a token whose requirement is not met is dropped and
// the survivors rejoined, the admitProvenanceAlternatives split the hand
// families use. ok is false when no alternative survives. A spec without any
// of the tokens is returned unchanged, so every unrelated evaluation is
// byte-identical. The bare wasCastFromGraveyard spelling is deliberately NOT
// evaluated here (it stays the effects-side CastFlags predicate).
func (e *Engine) castOriginAdmits(spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	for _, tok := range castOriginTokens {
		if !strings.Contains(spec, tok.token) {
			continue
		}
		holds := false
		if o := e.G.Obj(objID); o != nil && !o.IsCopy {
			if from, by, ok := e.latestCastOrigin(objID); ok {
				holds = from == tok.zone && (!tok.byYou || by == you)
			}
		}
		var ok bool
		if spec, ok = admitProvenanceAlternatives(spec, tok.token, holds); !ok {
			return "", false
		}
	}
	return spec, true
}

// castOriginAdmitsAtZone evaluates the origin-zone family against the cast
// IN PROGRESS rather than the log: the cast a CantBeCast restriction gates
// has no PutOnStack yet, and the origin it will carry is the zone the
// restricted object is being cast FROM — its CURRENT zone at every cast
// evaluation site (the offer walks, beginCast's own re-check; the pending
// cast's from is always the object's zone at creation). A ByYou token holds
// unconditionally on the zone match: the caster attempting the cast IS the
// evaluating you. No log scan, so a prior completed cast's origin cannot
// leak into this cast's evaluation.
func (e *Engine) castOriginAdmitsAtZone(spec string, objID state.ObjID, from state.Zone) (string, bool) {
	for _, tok := range castOriginTokens {
		if !strings.Contains(spec, tok.token) {
			continue
		}
		var ok bool
		if spec, ok = admitProvenanceAlternatives(spec, tok.token, from == tok.zone); !ok {
			return "", false
		}
	}
	return spec, true
}

// castProvenanceAdmitsPending is castProvenanceAdmits with the pending-cast
// guard the two COST paths need (the ReduceCost/RaiseCost statics'
// ValidCard$, and the RestrictValid$ mana-restriction read): a
// provenance-keyed spec is UNRESOLVABLE while the priced object has no cast
// in the log yet. The offer-side walk (castable → costPayable) and the
// option-selection snapshot (costModifiers) both evaluate pre-push, where
// the log scan reads false and the NEGATED spellings would wrongly hold —
// a hand cast would be offered as payable on mana the payment then refuses,
// or priced at a reduction the payment must not take. Such a spec denies
// the whole match while the object is off the stack (the fail-closed
// direction both call sites document); continueCast re-prices the pending
// cast right after CR 601.2a's push, once the PutOnStack is in the log. A
// spec without a provenance token is returned unchanged, so every
// unrelated cost evaluation is byte-identical.
func (e *Engine) castProvenanceAdmitsPending(spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	if !strings.Contains(spec, "wasCastFromYourHand") && !strings.Contains(spec, "wasCastByYou") &&
		!specCarriesCastOrigin(spec) {
		return spec, true
	}
	o := e.G.Obj(objID)
	if o == nil {
		return "", false
	}
	if o.Zone != state.ZStack {
		// The origin-zone family is priceable PRE-push, unlike the hand
		// families: the origin a cast from here will carry is the object's
		// CURRENT zone (every cast evaluation site's pending origin — see
		// castOriginAdmitsAtZone), so the affordability walk can honestly
		// admit the restricted/reduced cost instead of the blanket deny that
		// made a RestrictValid$ Spell.wasCastFromYourGraveyard batch unable to
		// fund the very cast it names (Lord of the Forsaken's payment path).
		// The hand families keep the deny: their ByYou cast is not yet in the
		// log and the negated spellings would wrongly hold.
		if specCarriesCastOrigin(spec) {
			return e.castOriginAdmitsAtZone(spec, objID, o.Zone)
		}
		return "", false
	}
	return e.castProvenanceAdmits(spec, objID, you)
}
