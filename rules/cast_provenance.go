// The rules-side split for the Card.wasCastFromYourHandByYou filter
// predicate (task castprov1). The predFn signature the effects filter runs
// on carries no Host, and a hand-origin cast deliberately carries no
// CastFlags bit (the flags mark alternative costs and origins only), so the
// provenance lives in the event log — Engine.WasCastFromHandByYou's reverse
// PutOnStack scan, the same machinery the Count$wasCastFromYourHandByYou
// branch head reads. The token is therefore split OUT of the spec text at
// the rules-side match sites (where the Engine, and its log, is in scope)
// and evaluated there, with the remainder matched by the ordinary filter.
// The precedent is the spec-rewrite helpers the engine already keeps
// (spellCastPermanentSpec, permanentCardSpec).

package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// castFromHandAdmits evaluates the bare wasCastFromYourHandByYou /
// !wasCastFromYourHandByYou qualifier of a Forge filter spec against objID:
// spec is split into its comma alternatives (the same split the filter
// draws), every alternative CARRYING the qualifier but failing the
// provenance test — the object was NOT cast from you's hand by you, or the
// object is a copy (never cast, the same IsCopy guard the count head takes)
// — is dropped, and the surviving alternatives are rejoined. ok is false
// when no alternative survives: the spec matches nothing. A spec without the
// token is returned unchanged with ok true, so every unrelated spec is
// byte-identical.
func (e *Engine) castFromHandAdmits(spec string, objID state.ObjID, you state.PlayerID) (string, bool) {
	if !strings.Contains(spec, "wasCastFromYourHandByYou") {
		return spec, true
	}
	holds := false
	if o := e.G.Obj(objID); o != nil && !o.IsCopy {
		holds = e.WasCastFromHandByYou(objID, you)
	}
	var b strings.Builder
	first := true
	alive := false
	for alt := range effects.FilterAlternatives(spec) {
		s1, hadPos := effects.StripPredicateToken(alt, "wasCastFromYourHandByYou")
		s2, hadNeg := effects.StripPredicateToken(s1, "!wasCastFromYourHandByYou")
		// The positive spelling requires the provenance to HOLD; the negated
		// spelling requires it to FAIL. An alternative whose requirement is
		// not met is dropped; the surviving alternatives are matched by the
		// ordinary filter.
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
