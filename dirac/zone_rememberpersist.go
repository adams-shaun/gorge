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
	ver, ok := uint8(0), false
	for i := 0; i < int(s.g.nlinks); i++ {
		if l := s.g.links[i]; l.src == e.obj && (l.sver == e.srcVer || l.sver+1 == e.srcVer) {
			ver, ok = l.sver, true // the fire-time incarnation's own set (its battlefield ver is srcVer or srcVer-1)
			break
		}
	}
	if !ok && remReadsSet(k, e) {
		// The fire-time capture was empty: the enters trigger had not
		// resolved when the source left (the Cloudshift'd Mesmeric Fiend),
		// so gorge's pending-referent read answers the live list (effects
		// context.effectiveSourceRemembered) -- the newest kept set, written
		// by a later battlefield incarnation's enters trigger. A writer
		// resolution takes nothing: spending the set here would drop the
		// fire-time record a leaves trigger still reads.
		for i := 0; i < int(s.g.nlinks); i++ {
			if l := s.g.links[i]; l.src == e.obj && (!ok || l.sver > ver) {
				ver, ok = l.sver, true
			}
		}
	}
	if !ok {
		return
	}
	for i := 0; i < int(s.g.nlinks); i++ {
		l := s.g.links[i]
		if l.src != e.obj || l.sver != ver {
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
// incarnation (the links) while the source is still on the battlefield.
// gorge's write lands on the live object (effects eventRemember), so a
// source that left and re-entered before its enters trigger resolved — the
// Cloudshift'd Mesmeric Fiend — keeps on the new incarnation, the one a
// later leaves trigger reads; a source off the battlefield (the classic
// O-ring resolution, CR 610.3c) keeps nothing: no trigger of it reads the
// live list again.
//
//go:noinline
func (s *State) remPersistKeep(e *stackEntry) {
	if DebugRemPersistNoKeep {
		return
	}
	ob := &s.objs[e.obj]
	if ob.zone != kop.ZoneBattlefield {
		return
	}
	for i := 0; i < int(s.g.nrem); i++ {
		if s.g.nlinks >= uint8(len(s.g.links)) {
			s.capFull(StallLinks)
			return
		}
		l := &s.g.links[s.g.nlinks]
		l.src, l.sver, l.obj = e.obj, ob.ver, s.g.rem[i]
		s.g.nlinks++
	}
}

// remReadsSet reports whether the resolving entry's program reads the
// remembered set (a Return of RefRemembered): the pair shape's leaves
// trigger. A writer resolution (the enters trigger's Exile that remembers)
// takes nothing from the kept sets.
func remReadsSet(k *prep, e *stackEntry) bool {
	if e.kind != seTrigger || int(e.ab) >= len(k.t.Cards[e.card].Abilities) {
		return false
	}
	for _, in := range k.t.Cards[e.card].Abilities[e.ab].Prog {
		if in.Op == kop.OpReturn && in.Obj == kop.RefRemembered {
			return true
		}
	}
	return false
}
