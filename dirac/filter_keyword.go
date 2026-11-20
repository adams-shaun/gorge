package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwKeyword: FwKeyword, the object has keyword bit Arg of its current
// KwMask (PredHasKeyword). A layer-walk view reads printed keywords only (no
// re-entry), so it fails closed.
//
//kmanifest:fword FwKeyword
func (s *State) fwKeyword(c *fctx, l *kop.FLit) bool {
	if uint(l.Arg) >= 32 {
		return false
	}
	switch {
	case c.walk:
		return false
	case c.perm():
		return s.kwGen(c.k, c.sl)&(1<<uint(l.Arg)) != 0
	case c.card():
		return c.k.cards[c.c].kw&(1<<uint(l.Arg)) != 0
	}
	return false
}
