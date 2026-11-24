package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// filtNetsSupply reports whether some hybrid-fee filter of f may raise the
// mana a payment can spend, which makes it a gaining census gap for the count
// proof. gorge's gapOnceNetSupply bounds a once-only {T} filter by what it
// nets beyond its own source's covered production: its fixed production
// minus the fee each activation spends minus the free {T} ability the same tap
// would have made instead (Sunken Ruins' "{U/B}, {T}: Add {U}{U}, {U}{B}, or
// {B}{B}" nets 2-1-1 = 0 beside its {T}: Add {C}), CR 106.1, 605.3b. A net of
// zero or less converts colour and never adds to the supply, so the census
// is no gap that gains (gapGains false) and costOutrunsSupply's count proof
// stands beside it. A dynamic amount is no fixed production and a
// non-hybrid filter keeps the unbounded read.
func (s *State) filtNetsSupply(k *prep, f slotMask) bool {
	for ; f.any(); f = f.rest() {
		sl := f.first()
		if _, _, hyb := s.filtHyb(k, sl); !hyb {
			return true
		}
		a := s.filtAb(k, sl)
		c := s.cardAt(sl)
		fi := firstManaAb(&k.t.Cards[c])
		if a.AFlags&kop.AfManaDynamic != 0 || fi < 0 || k.t.Cards[c].Abilities[fi].AFlags&kop.AfManaDynamic != 0 {
			return true
		}
		if int(a.ManaN)-s.filtFee(k, sl)-int(k.t.Cards[c].Abilities[fi].ManaN) > 0 {
			return true
		}
	}
	return false
}
