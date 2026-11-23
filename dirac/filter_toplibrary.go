package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwTopLibrary: FwTopLibrary, gorge wordTopLibrary
// (engine/effects/filter_word.go:887): the object is the top card of its
// library -- index 0 of the owner's library slice, the card the next draw
// takes. A card deeper in the library never matches, and a card that is not
// in a library at all never matches.
//
//kmanifest:fword FwTopLibrary
func (s *State) fwTopLibrary(c *fctx, l *kop.FLit) bool {
	switch {
	case c.perm():
		return false
	case c.card():
		if c.o == NoObj || c.zone != kop.ZoneLibrary {
			return false
		}
		p := &s.pl[c.owner]
		return p.libLen() > 0 && p.lib[p.libLo] == c.o
	}
	return false
}
