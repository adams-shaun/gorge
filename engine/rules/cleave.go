package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Cleave (CR 702.148a): "Cleave [cost]" is an alternative cost -- "You may
// cast this spell by paying [cost] rather than paying its mana cost" -- plus
// a text change -- "If this spell's cleave cost was paid, change its text by
// removing all text found within square brackets". Forge scripts the cleaved
// spell as a SECOND SP ability carrying NonBasicSpell$ True (12 corpus files,
// every one PrecostDesc$ Cleave): its Cost$ is the cleave cost and its
// parameters are already the bracket-free text. So a cleaved cast is the
// plain cast with that ability substituted for f.SpellAbility() -- at the
// offer's target census, at the target ask (castStageSA), at resolution
// (resolveTop) and at a resumed resolution (resumeResolution) -- keyed on
// the "cleaved" pendingCast mode during the cast and on the pay-time
// FlagCleaved provenance once the spell is on the stack (the bestow
// precedent). A stack copy inherits FlagCleaved (not a CastProvenanceFlag):
// CR 707.10 copies the alternative-cost decision and the text change with it.

// castModeCleavedWord is the "cast" option Mode of a cleaved cast.
const castModeCleavedWord = "cleaved"

// cleaveSA is f's cleave spell ability: the first SP ability after the face's
// own SpellAbility whose NonBasicSpell$ is True, or nil when f has none.
func cleaveSA(f *cards.Face) *cards.SA {
	if f == nil {
		return nil
	}
	first := true
	for _, a := range f.Abilities {
		if a == nil || a.Kind != "SP" {
			continue
		}
		if first {
			first = false
			continue
		}
		if a.ParamStr(cards.PKNonBasicSpell) == "True" {
			return a
		}
	}
	return nil
}

// cleaveCost is the cleave ability's Cost$, the whole alternative cost (CR
// 118.9: paid instead of the mana cost), read through the compiled-cost cache
// so the offer walk and the charge share one parse. ok is false when f has no
// cleave ability or the cost carries a token ParseCost cannot model (the
// alternative-cost fail-closed convention: withheld, never charged degraded).
func (e *Engine) cleaveCost(f *cards.Face) (Cost, bool) {
	csa := cleaveSA(f)
	if csa == nil {
		return Cost{}, false
	}
	c := e.abilityCompiledCost(csa).Cost
	if len(c.Unknown) > 0 {
		return Cost{}, false
	}
	return c, true
}

// stackSpellSA is the spell ability a spell object o on the stack resolves
// with: the cleave ability when the cast paid the cleave cost, f's own
// SpellAbility otherwise.
func stackSpellSA(o *state.Object, f *cards.Face) *cards.SA {
	if f == nil {
		return nil
	}
	if o != nil && o.CastFlags&state.FlagCleaved != 0 {
		if csa := cleaveSA(f); csa != nil {
			return csa
		}
	}
	return f.SpellAbility()
}
