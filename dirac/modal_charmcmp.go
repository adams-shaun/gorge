package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A spell charm whose CharmNum$ is a Count$Compare with constant arms
// (Molten Collapse "if you descended this turn, choose both instead"; Wail
// of the Forgotten at eight permanent cards in your graveyard). gorge
// effects.CharmModeBounds evaluates CharmNum$ at each ask with the caster's
// ctx, floors it at 1 and clamps it to the number of modes; the offer gate
// reads only the minimum (MinCharmNum$, Ability.ModesPick-ModesFew), so
// castability never depends on the count. The count is CardDef.ModesPickV,
// evaluated on demand at each mode step -- nothing moves between the mode
// picks of one build, so it is never stored. Cold; reached behind
// genPrep.anyCharmV.

// DebugCharmCmpCap makes charmCap answer 1 always (mutation test only).
var DebugCharmCmpCap bool

// prepCharmCmp sets anyCharmV (and arms the multi-pick machinery) when some
// card carries ModesPickV.
func prepCharmCmp(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		if t.Cards[i].ModesPickV.K != 0 {
			g.anyCharmV, g.anyCharmNum = true, true
		}
	}
}

// charmCap is the pick count of the spell being built, clamped to
// 1..NModes (0: the card has no dynamic count).
//
//go:noinline
func (s *State) charmCap(k *prep) uint8 {
	e := &s.bld().e
	if e.kind != seSpell {
		return 0
	}
	cd := &k.t.Cards[e.card]
	if cd.ModesPickV.K == 0 {
		return 0
	}
	if DebugCharmCmpCap {
		return 1
	}
	a := s.buildAbility(k)
	if a == nil {
		return 0
	}
	v := s.evalV(k, cd.ModesPickV, s.ctxEntry(e))
	return uint8(max(1, min(v, int(a.NModes))))
}
