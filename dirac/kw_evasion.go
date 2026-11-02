package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// landwalkEvades: attacker a's printed landwalk (CardDef.Landwalk) makes it
// unblockable while the defending player (blocker b's controller)
// controls a land matching the filter (CR 702.14; gorge rules/combat
// block.go LandwalkEvades, over its current keywords: none once it lost
// its abilities).
//
//go:noinline
func (s *State) landwalkEvades(k *prep, b, a Slot) bool {
	f := k.t.Cards[s.cardAt(a)].Landwalk
	if f == 0 || s.genLost(k, a) {
		return false
	}
	def := s.perms[b].ctrl
	for m := s.ctrlM[def]; m.any(); m = m.rest() {
		sl := m.first()
		if s.typesAt(k, sl)&kop.TLand != 0 && s.matchPerm(k, f, sl, s.perms[a].ctrl, a) {
			return true
		}
	}
	return false
}

// keyword arms (cold, out of line).

// evadesShadowFear reports whether the fear / shadow rules forbid blocker b from
// blocking an attacker with keywords akw (gorge combat.CanBlock):
// CR 702.28b-c a shadow creature blocks and is blocked only by shadow
// creatures; CR 702.36b a fear creature is blocked only by an artifact
// creature (the blocker's printed face, gorge bf.IsArtifact) or a black one
// (its derived colours).
func (s *State) evadesShadowFear(k *prep, b Slot, akw, bkw kop.KwMask) bool {
	if (akw^bkw)&kop.KwShadow != 0 {
		return true
	}
	if akw&kop.KwFear != 0 && k.t.Cards[s.cardAt(b)].Types&kop.TArtifact == 0 && s.colorsAt(k, b)&kop.ColB == 0 {
		return true
	}
	return false
}

// evasionAt is the permanent at sl's printed CardDef.Evasion bits, none
// once it has lost its abilities (gorge HasKW over the derived keywords).
func (s *State) evasionAt(k *prep, sl Slot) uint8 {
	if ev := k.t.Cards[s.cardAt(sl)].Evasion; ev != 0 && !s.genLost(k, sl) {
		return ev
	}
	return 0
}

// evasionEvades: attacker a's printed horsemanship or intimidate forbids
// blocker b (gorge rules/combat/block.go CanBlock): horsemanship unless b
// has horsemanship (CR 702.31b); intimidate unless b is an artifact
// creature (its printed face, gorge bf.IsArtifact, as Fear's) or shares a
// current colour with a (CR 702.13a).
//
//go:noinline
func (s *State) evasionEvades(k *prep, b, a Slot) bool {
	ev := s.evasionAt(k, a)
	if ev&kop.EvHorsemanship != 0 && s.evasionAt(k, b)&kop.EvHorsemanship == 0 {
		return true
	}
	if ev&kop.EvIntimidate != 0 && k.t.Cards[s.cardAt(b)].Types&kop.TArtifact == 0 && s.colorsAt(k, a)&s.colorsAt(k, b)&kop.ColAny == 0 {
		return true
	}
	if ev&kop.EvSkulk != 0 && s.skulkEvades(k, b, a) {
		return true
	}
	return false
}

// skulkEvades: attacker a's printed skulk forbids a blocker b with greater
// power (CR 702.118a; gorge rules/combat/block.go CanBlock: derived powers
// at declaration).
//
//go:noinline
func (s *State) skulkEvades(k *prep, b, a Slot) bool {
	bp, _ := s.ptGen(k, b)
	ap, _ := s.ptGen(k, a)
	return bp > ap
}
