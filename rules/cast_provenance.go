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
// castProvenanceAdmits is the combined entry point every match site calls:
// it evaluates both families in one pass, so a future carrier mixing the
// tokens in one spec is covered by construction.

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

// castProvenanceAdmits evaluates BOTH cast-provenance families of a Forge
// filter spec against objID — the hand-origin one (castFromHandAdmits) and
// the any-origin one (castAtAllAdmits) — chaining the two splits so a spec
// carrying either (or both) token evaluates fully. This is the one entry
// point every rules-side match site calls, so a future carrier mixing the
// tokens is covered by construction.
func (e *Engine) castProvenanceAdmits(spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	s, ok := e.castFromHandAdmits(spec, objID, you)
	if !ok {
		return "", false
	}
	return e.castAtAllAdmits(s, objID, you)
}
