package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// countExileKw is ExStaticExileKw: the exiled cards matching x.F that print
// the keyword x.N names. The card's printed keywords are read (CR 400.7;
// gorge statSharedKeywords reads effects.PrintedChars off every non-battlefield
// card), never a derived layer result. N below 32 is a KwMask bit; N from 32 up is a
// protection colour (kop.ExileKwProtColour) in the card's printed ProtColors, which is not a keyword bit (protColourKw).
func (s *State) countExileKw(k *prep, x *kop.Expr, ctx evalCtx) int {
	n := 0
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		pl := &s.pl[pp]
		for j := 0; j < int(pl.exileN); j++ {
			o := pl.exile[j]
			if !s.matchCardGenAt(k, x.F, s.objs[o].card, pp, ctx.ctrl, o, &ctx) {
				continue
			}
			if printedHasKw(&k.t.Cards[s.objs[o].card], int(x.N)) {
				n++
			}
		}
	}
	return n
}

// printedHasKw reports whether cd prints the keyword code n (see countExileKw).
func printedHasKw(cd *kop.CardDef, n int) bool {
	if n >= 32 {
		c, ok := kop.ExileKwProtColour(n)
		return ok && cd.ProtColors&c != 0
	}
	return cd.Keywords&(kop.KwMask(1)<<uint(n)) != 0
}
