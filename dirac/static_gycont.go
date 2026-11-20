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
// with no permanent slot (sl == NoSlot, emb == 0). Its grant timestamp is the
// layer stamp its card carried off the battlefield (grantTs;
// static_gystamp.go), as gorge's LayerTimestamp: a loss of abilities that began
// before the card entered the battlefield is older than the grant, one that
// began after is newer.

// gyContOf reports whether a is a graveyard-functioning Continuous static.
func gyContOf(a *kop.Ability) bool {
	return a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.StFlags&kop.StFunctionsInGraveyard != 0
}

// battlefieldStatic is pcard.hasStat after static ability a: a graveyard
// Continuous static is no battlefield static (it never joins statM).
func battlefieldStatic(had bool, a *kop.Ability) bool { return had || !gyContOf(a) }

// tableHasGyCont reports whether any card of t has a graveyard Continuous
// static (prep.gyCont).
func tableHasGyCont(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			if gyContOf(&t.Cards[i].Abilities[j]) {
				return true
			}
		}
	}
	return false
}

// anyGyStatic reports whether this table has a graveyard Continuous static.
func anyGyStatic(k *prep) bool { return k.gyCont }

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
		// the stamp decides which loss of abilities the grant outlasts
		h = hmix(h, uint64(s.gyStampOf(st.obj)))
		if st.a.Static.Cond2 != 0 { // FeatStaticKey's condition truth, for a static eachStatic does not yield
			c := uint64(1)
			if s.statCond(k, st) {
				c = 2
			}
			h = hmix(h, c)
		}
		return true
	})
	return h
}
