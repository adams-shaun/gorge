package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// dugBottom is OpDiscover C=DiscoverDugBottom (Jodah, the Unifier;
// kgen lower_digimprint.go): the cards the chain's DuDugMark DigUntil exiled
// from the resolving controller's library (found first, then the rest, the
// exile order both engines keep) that are still in exile and not queued to
// be cast go to the library bottom in a random order. gorge's ChangeZoneAll
// RandomOrder$ (zone_changeall.go) collects them in exile scan order,
// Fisher-Yates them backward through the engine rng (Rand(i+1) for i from
// the last index down to 1) and moves them in that order, each onto the
// bottom.
//
//go:noinline
func (s *State) dugBottom(k *prep) {
	p := s.res.e.ctrl
	pl := &s.pl[p]
	var list [MaxDeck]Obj
	n := 0
	for i := int(s.g.digLo); i < int(pl.exileN); i++ {
		if o := pl.exile[i]; !s.freeQueued(o) {
			list[n] = o
			n++
		}
	}
	for i := n - 1; i > 0 && !DebugNoDugShuffle; i-- {
		j := s.rng.IntN(i + 1)
		list[i], list[j] = list[j], list[i]
	}
	for _, o := range list[:n] {
		from := s.detach(o)
		s.placeFrom(o, kop.ZoneLibrary, true, from, false)
	}
}

// DebugNoDugShuffle keeps the dug cards in exile order (the jodah
// mutation test).
var DebugNoDugShuffle bool
