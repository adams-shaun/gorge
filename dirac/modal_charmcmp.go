package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A spell or triggered charm whose CharmNum$ is a Count$Compare or a branch
// head with constant arms, or a general count beside MinCharmNum$ 0 ("choose
// up to V"; V = 0 chooses no mode and removes a trigger, CR 603.3c) (Molten Collapse "if you descended this turn,
// choose both instead"; Wail of the Forgotten at eight permanent cards in
// your graveyard; Captain James T. Kirk with no cards in hand). gorge
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

// charmCap is the pick count of the spell or trigger being built, clamped
// to 0..NModes; ok is false when the card has no dynamic count. Only a
// "choose up to V" charm (MinCharmNum$ 0) reaches 0 -- gorge floors every
// other count at 1 (effects.CharmModeBounds); the constant arms of a
// two-arm count are all 1 or more. A trigger's modes are
// chosen as it is put on the stack (CR 603.3c), with its controller's ctx;
// kgen sets ModesPickV on a card only when every triggered charm of it
// shares that count (Captain James T. Kirk's Count$Hellbent.3.1).
//
//go:noinline
func (s *State) charmCap(k *prep) (uint8, bool) {
	e := &s.bld().e
	if e.kind != seSpell && e.kind != seTrigger {
		return 0, false
	}
	cd := &k.t.Cards[e.card]
	if cd.ModesPickV.K == 0 {
		return 0, false
	}
	if DebugCharmCmpCap {
		return 1, true
	}
	a := s.buildAbility(k)
	if a == nil {
		return 0, false
	}
	v := s.evalV(k, cd.ModesPickV, s.ctxEntry(e))
	return uint8(max(0, min(v, int(a.NModes)))), true
}
