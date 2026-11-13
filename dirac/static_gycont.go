package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A Continuous static with kop.StFunctionsInGraveyard works while its source
// card is in its owner's graveyard instead of on the battlefield (the Mirage
// Incarnations: Anger "As long as Anger is in your graveyard and you control a
// Mountain, creatures you control have haste"; CR 113.6, 604.2; gorge
// staticEffectsWalk's graveyard walk). The card's own face is not a
// battlefield static, so pc.hasStat stays false for it (prep.go) and it never
// joins statM; the source is the graveyard object, controlled by its owner,
// with no permanent slot (sl == NoSlot, emb == 0). Its grant timestamp is 0
// (grantTs), older than any loss of abilities.

// gyContOf reports whether a is a graveyard-functioning Continuous static.
func gyContOf(a *kop.Ability) bool {
	return a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.StFlags&kop.StFunctionsInGraveyard != 0
}

// anyGyStatic reports whether this table has a graveyard Continuous static.
func anyGyStatic(k *prep) bool { return k.gen != nil && k.gen.anyGyCont }

// eachGyStatic calls f for every graveyard Continuous static of a class in
// cls (0 = all) of a card in the graveyard of a seat still in the game, in
// seat then graveyard order; f returns false to stop it (eachGyStatic then
// reports false).
func (s *State) eachGyStatic(k *prep, cls uint16, f func(st statSrc) bool) bool {
	if !anyGyStatic(k) {
		return true
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		pl := &s.pl[SeatIx(pp)]
		for i := 0; i < int(pl.graveN); i++ {
			o := pl.grave[i]
			c := s.objs[o].card
			if k.gen.sCls[c]&sclsGyCont == 0 || cls != 0 && k.gen.sCls[c]&cls == 0 {
				continue
			}
			cd := &k.t.Cards[c]
			for j := range cd.Abilities {
				if a := &cd.Abilities[j]; gyContOf(a) {
					if !f(statSrc{a: a, sl: NoSlot, ctrl: pp, obj: o}) {
						return false
					}
				}
			}
		}
	}
	return true
}

// gyStaticKey fingerprints which graveyard objects carry such a static, for
// FeatStaticKey: a card entering or leaving the graveyard moves what the
// statics in play grant even when no condition of theirs changes.
func (s *State) gyStaticKey(k *prep, h uint64) uint64 {
	s.eachGyStatic(k, 0, func(st statSrc) bool {
		h = hmix(h, uint64(st.obj)<<8|uint64(st.ctrl))
		return true
	})
	return h
}
