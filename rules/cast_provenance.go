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
// castProvenanceAdmits is the combined entry point every match site calls:
// it evaluates all three families in one pass, so a future carrier mixing
// the tokens in one spec is covered by construction (measured: alex_wilder
// and quandrix_the_proof carry wasCastByYou AND the bare token in one
// alternative).

package rules

import (
	"strings"

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
	return e.castFromHandAdmitsWindow(spec, objID, you, false)
}

// castFromHandAdmitsWindow is castFromHandAdmits with the pre-push OFFER
// window fallback: when pendingCast is set (the layer walk is evaluating an
// AffectedZone$ Stack grant against the object being cast, before CR
// 601.2a's push), the log carries no PutOnStack yet, so "cast from your hand
// by you" is inferred from the offer-time object: its controller is the
// caster and its zone is the hand. The log read still wins whenever it has
// an answer (a real on-stack spell), so no pushed cast changes behaviour.
func (e *Engine) castFromHandAdmitsWindow(spec string, objID state.ObjID, you state.PlayerID, pendingCast bool) (string, bool) {
	if !strings.Contains(spec, "wasCastFromYourHandByYou") {
		return spec, true
	}
	holds := false
	if o := e.G.Obj(objID); o != nil && !o.IsCopy {
		holds = e.WasCastFromHandByYou(objID, you)
		if !holds && pendingCast {
			holds = o.Controller == you && o.Zone == state.ZHand
		}
	}
	return admitProvenanceAlternatives(spec, "wasCastFromYourHandByYou", holds)
}

// castAtAllAdmits evaluates the bare wasCastByYou / !wasCastByYou qualifier
// (castprov2): "was cast at all, by you" — some PutOnStack event for this
// object names you as caster, any origin. Copies were never cast.
func (e *Engine) castAtAllAdmits(spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	return e.castAtAllAdmitsWindow(spec, objID, you, false)
}

// castAtAllAdmitsWindow is castAtAllAdmits with the pre-push OFFER window
// fallback (see castFromHandAdmitsWindow): an AffectedZone$ Stack grant whose
// Affected$ carries wasCastByYou (Zinnia's "Creature spells you cast have
// offspring {2}", Witherbloom's affinity, Prismari's storm, Quandrix's
// cascade, Silverquill's casualty, Mycosynth Golem's artifact affinity and
// the rest) evaluates against the object BEING CAST, which is still in hand
// at offer time and so has no PutOnStack in the log. "Cast by you" then
// holds iff the offer-time object's controller (the caster) is you. The log
// read wins whenever it has an answer, so a real on-stack spell is
// unchanged.
func (e *Engine) castAtAllAdmitsWindow(spec string, objID state.ObjID, you state.PlayerID, pendingCast bool) (string, bool) {
	if !strings.Contains(spec, "wasCastByYou") {
		return spec, true
	}
	holds := false
	if o := e.G.Obj(objID); o != nil && !o.IsCopy {
		holds = e.WasCastByYou(objID, you)
		if !holds && pendingCast {
			holds = o.Controller == you
		}
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
	return e.castFromHandAnyAdmitsWindow(spec, objID, false)
}

// castFromHandAnyAdmitsWindow is castFromHandAnyAdmits with the pre-push
// OFFER window fallback (see castFromHandAdmitsWindow): a bare
// wasCastFromYourHand grant evaluated against the object being cast infers
// "came from a hand" from the offer-time object's zone. The log read wins
// whenever it has an answer.
func (e *Engine) castFromHandAnyAdmitsWindow(spec string, objID state.ObjID, pendingCast bool) (string, bool) {
	if strings.Contains(spec, "wasCastFromYourHandByYou") || !strings.Contains(spec, "wasCastFromYourHand") {
		return spec, true
	}
	holds := false
	if o := e.G.Obj(objID); o != nil && !o.IsCopy {
		holds = e.WasCastFromHand(objID)
		if !holds && pendingCast {
			holds = o.Zone == state.ZHand
		}
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
	return e.castProvenanceAdmitsWindow(spec, objID, you, false)
}

// castProvenanceAdmitsWindow is castProvenanceAdmits with the pre-push OFFER
// window fallback threaded through all three families (see
// castFromHandAdmitsWindow): the layer walk's AffectedZone$ Stack read sets
// pendingCast, every other match site passes false and keeps the log-only
// semantics. The chain order is unchanged: ByYou before bare.
func (e *Engine) castProvenanceAdmitsWindow(spec string, objID state.ObjID, you state.PlayerID, pendingCast bool) (string, bool) {
	s, ok := e.castFromHandAdmitsWindow(spec, objID, you, pendingCast)
	if !ok {
		return "", false
	}
	s, ok = e.castAtAllAdmitsWindow(s, objID, you, pendingCast)
	if !ok {
		return "", false
	}
	return e.castFromHandAnyAdmitsWindow(s, objID, pendingCast)
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
	if !strings.Contains(spec, "wasCastFromYourHand") && !strings.Contains(spec, "wasCastByYou") {
		return spec, true
	}
	if o := e.G.Obj(objID); o == nil || o.Zone != state.ZStack {
		return "", false
	}
	return e.castProvenanceAdmits(spec, objID, you)
}
