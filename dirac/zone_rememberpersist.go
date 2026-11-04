package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Remembered persisting on the source, cold: reached only
// when the table has a CardDef.RemPersist card (genPrep.anyRemPersist).
//
// gorge's Remembered also lives on the source object (effects eventRemember,
// sourceRemembered), so Journey to Nowhere's leaves trigger returns the
// creature its enters trigger exiled and Mesmeric Fiend's the card its
// enters trigger took (kgen lower_rempersist.go accepts exactly that pair).
// The kernel's remembered set is per resolution, so the pair rides the exile
// links (gen.links, 0 B): when a RemPersist card's trigger finishes resolving
// with its source still the incarnation it triggered from (on the
// battlefield, ver == srcVer), each remembered object becomes a link {src,
// sver: that incarnation, obj}; when a later trigger of the card resolves
// from that incarnation (its leaves trigger: srcVer is the battlefield ver,
// mana_window.go/fireLeave), its remembered set starts as those objects and the
// links are spent. A leaves trigger's srcVer is the battlefield ver or,
// queued after the departure bumped it, that ver + 1: the next battlefield
// incarnation is ver + 2 (enterNF bumps again), so neither can name another
// incarnation's links. leaveGen leaves such links alone (no immediate return:
// the return is the leaves trigger, on the stack). A source that left before
// its enters trigger resolved is a new incarnation: nothing is kept, so the
// exiled card stays exiled (the CR 610.3c "O-ring" ordering gorge follows).

// DebugRemPersistNoKeep makes remPersistKeep keep nothing (the oracle's
// mutation: the leaves trigger then finds an empty remembered set).
var DebugRemPersistNoKeep bool

// remPersistSeeds / remPersistKeeps report whether entry e is a RemPersist
// card's trigger whose resolution starts with the kept set (CardDef.RemSeed)
// or whose finished set is kept (CardDef.RemKeep). A Saga's chapters split
// the two (kgen lower_rempersist.go remPersistSaga): only its reading chapter
// takes the kept set, so a chapter between them leaves it alone.
func remPersistSeeds(k *prep, e *stackEntry) bool {
	c := &k.t.Cards[e.card]
	return e.kind == seTrigger && c.RemPersist && e.ab < 32 && c.RemSeed>>e.ab&1 != 0
}

func remPersistKeeps(k *prep, e *stackEntry) bool {
	c := &k.t.Cards[e.card]
	return e.kind == seTrigger && c.RemPersist && e.ab < 32 && c.RemKeep>>e.ab&1 != 0
}

// remPersistSeed starts the resolving entry's remembered set with the
// objects its source's incarnation kept.
//
//go:noinline
func (s *State) remPersistSeed(k *prep) {
	e := &s.res.e
	for i := 0; i < int(s.g.nlinks); i++ {
		l := s.g.links[i]
		if l.src != e.obj || l.sver != e.srcVer && l.sver+1 != e.srcVer || !k.t.Cards[s.objs[l.src].card].RemPersist {
			continue
		}
		if s.g.linkHand|s.g.linkHide|s.g.linkAeg != 0 {
			s.dropLinkHand(i) // act_mayask.go
		}
		copy(s.g.links[i:], s.g.links[i+1:s.g.nlinks])
		s.g.nlinks--
		i--
		if s.g.nrem >= maxRem {
			s.capFull(StallRemember)
			return
		}
		s.g.rem[s.g.nrem] = l.obj
		s.g.nrem++
	}
}

// remPersistKeep keeps the finished entry's remembered set on its source's
// incarnation (the links) while the source is still that incarnation.
//
//go:noinline
func (s *State) remPersistKeep(e *stackEntry) {
	if DebugRemPersistNoKeep {
		return
	}
	if ob := &s.objs[e.obj]; ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer {
		return // gorge records it on a new incarnation no leaves trigger reads
	}
	for i := 0; i < int(s.g.nrem); i++ {
		if s.g.nlinks >= uint8(len(s.g.links)) {
			s.capFull(StallLinks)
			return
		}
		l := &s.g.links[s.g.nlinks]
		l.src, l.sver, l.obj = e.obj, e.srcVer, s.g.rem[i]
		s.g.nlinks++
	}
}
