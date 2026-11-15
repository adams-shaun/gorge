package dirac

import "math/bits"

// Entwine (CR 702.42, Tempest), copied from gorge (rules cast_altcost.go
// entwineCost, legal_walk_hand.go's "(entwined)" offer, cast_begin.go
// castModeEntwined, cast_asks.go castModeAsk):
//
//   - the entwine cost is an optional ADDITIONAL cost offered as its own cast
//     option beside the plain cast (gorge Mode "entwined", the Buyback
//     shape: base + entwine, never an ask). The kernel's kicked cast is that
//     option: CardDef.Entwine cards carry the entwine cost in CardDef.Kicker
//     (no KwKicker; a Sac<N/..> cost is zero mana plus KickerCost,
//     kw_kicker.go) and the paid flag is sfKicked on the entry.
//   - an entwined cast announces EVERY mode that has a legal target (gorge
//     min = max = the filtered mode count, CR 702.42b): a mode with no target
//     is left out, none at all aborts the cast (CR 733.1). entwineModes is
//     that count; the mode step reads it as the charm's pick count and
//     minimum (twModes, the Teamwork charm's seat), so the picks are the
//     multi-pick charm's.
//   - gorge flags nothing on an entwined spell: StackView.Kicks does not
//     count it (FeatStackKicked).

// DebugEntwinePickOne (mutation test only): an entwined cast picks one mode
// like a plain one.
var DebugEntwinePickOne bool

// FeatEntwineCast reports that object o's kicked cast is gorge's "(entwined)"
// cast: its card has Entwine.
func (s *State) FeatEntwineCast(o Obj) bool {
	return s.k().t.Cards[s.objs[o].card].Entwine
}

// entwineModes is the number of modes the entry being built is announced
// with when it is an entwined cast (ok false: it is not one). During the mode
// stage it is the count of modes with a legal target (0: the cast is
// reversed); afterwards the modes chosen.
func (s *State) entwineModes(k *prep) (n uint8, ok bool) {
	e := &s.bld().e
	if DebugEntwinePickOne || e.kind != seSpell || e.flags&sfKicked == 0 || !k.t.Cards[e.card].Entwine {
		return 0, false
	}
	if s.bld().stage != bsModes {
		return uint8(bits.OnesCount8(e.mode)), true
	}
	a := s.buildAbility(k)
	if a == nil {
		return 0, false
	}
	for m := 0; m < int(a.NModes); m++ {
		if s.modeFeasible(k, a, m) {
			n++
		}
	}
	return n, true
}
