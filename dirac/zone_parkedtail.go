package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A search's or dig's tail parked at a queued as-enters ask.
// The found card enters mid-instruction, its replacement (a shock land's
// pay-life election, Wood Elves' fetched Banner's type election) queues an
// ask, and gorge suspends the enclosing effect right there: the search's
// mandatory shuffle (CR 701.24b) and a dig's rest ordering and placement run
// when the election is answered, not before. The kernel used to finish the
// instruction at once, so the library order and the RNG draw count were
// already past the ask when it was answered.
//
// The move phase (finishSearchNow, finishDigAt) parks the tail by setting
// res.left to a tail* marker; the raGenEnterAsk answer runs it
// (unparkTail) once the ask queue is drained. res.left is dead on a
// search/dig op (startSearch and startDig clear it, no resolveOptionsGen or
// applyResolveGen arm of raGenSearch/raGenDig/raGenDigArrange reads it), so
// no State byte is added. The marker is only ever read at a search/dig op
// (tailOp), so a stale res.left from an earlier op cannot run a tail.
const (
	tailSearch      uint8 = 1 + iota // the library shuffle and ThenTop of a search
	tailDig                          // a dig's rest ordering and placement
	tailDigArrange                   // a dig's raGenDigArrange pose
	tailDigUntil                     // a reveal-until's later found cards and the rest
	tailDigEnter                     // a dig's later taken cards, then its tail
	tailSearchEnter                  // a search's later found cards, then its shuffle
	tailDigEnterH                    // tailDigEnter with the entrants' enters checks held to the end
	tailDigH                         // tailDig with the entrants' enters checks held to the end
)

// parkTail parks the tail kind when the resolution will suspend at the
// as-enters ask the move phase queued: the very test runProg and continueRes
// make after the instruction (nask != 0 && midResAsk), so a parked tail is
// always served. false: no ask the resolution poses; the tail runs at once.
func (s *State) parkTail(k *prep, kind uint8) bool {
	if s.g.nask == 0 || !s.midResAsk(k) {
		return false
	}
	s.res.left = kind
	return true
}

// tailOp: the op at pc is one that parks a tail.
func (s *State) tailOp(k *prep) bool {
	switch s.prog(k)[s.res.pc].Op {
	case kop.OpSearchEx, kop.OpChooseMove, kop.OpDigEx, kop.OpDigUntil, kop.OpChangeZoneAll, kop.OpChangeZoneAllEx:
		return true
	}
	return false
}

// unparkTail runs the parked tail, if any, after the last queued ask is
// answered; true when the tail posed an ask (raGenDigArrange) and the
// program must wait for it.
func (s *State) unparkTail(k *prep) bool {
	t := s.res.left
	if t == 0 || !s.tailOp(k) {
		return false
	}
	s.res.left = 0
	switch t {
	case tailSearch:
		s.finishSearchTail(k)
	case tailDig:
		if kop.Zone(s.prog(k)[s.res.pc].C&7) == kop.ZoneBattlefield {
			s.digRestAside(int(s.g.nlook))
		}
		s.finishDigTail(k)
	case tailDigH:
		s.digRestAside(int(s.g.nlook))
		s.finishDigTail(k)
		s.holdClose(k, holdRec{on: true, ts: s.g.retTS})
	case tailDigEnterH:
		s.res.ask = raNone
		return s.finishDigAt(k, true, true)
	case tailDigArrange:
		s.res.ask, s.res.askP = raGenDigArrange, s.digArrangeAsker(k)
		return true
	case tailDigUntil, tailDigEnter, tailSearchEnter:
		s.res.ask = raNone
		return s.rerunMove(k, &s.prog(k)[s.res.pc])
	}
	return false
}

// digArranges: the dig's rest is ordered one card at a time ("the rest on
// the bottom in any order": gorge's dig_arrange; with DigRestTopAsk "the
// rest on top in any order", gorge digTopOrderAsk: zone_digtoporder.go).
func digArranges(n int, lookSel uint32, rz kop.Zone, fl int16) bool {
	nr := n - bits.OnesCount32(lookSel)
	order := fl&(kop.DigRandomOrder|kop.DigRestTop) == 0 || fl&kop.DigRestTopAsk == kop.DigRestTopAsk
	return nr >= 2 && (rz == kop.ZoneLibrary || rz == kop.ZoneNone) && order
}

// finishDigTail is a dig's tail after its taken cards moved: the rest
// (the looked-at cards not taken, in window order) in random order if asked,
// then back on top or to the rest zone.
func (s *State) finishDigTail(k *prep) {
	in := &s.prog(k)[s.res.pc]
	pl := &s.pl[s.g.pickP]
	n := int(s.g.nlook)
	fl := in.C >> 3
	rz := kop.Zone(fl & 7)
	if k.gen != nil && k.gen.anyBatch {
		defer s.batchClose(s.batchOpen()) // gorge effDig: one zone batch (trig_oneormore.go)
	}
	// The rest keeps the WINDOW order, not look's: digPick moves the picked
	// cards to the front of look in pick order, so a picked Aura that stays
	// behind (CR 303.4g) would jump ahead of the cards it was looked at
	// behind. lookOrig says where each look slot came from.
	var rest [24]Obj // len(genState.look)
	var at [24]uint8 // window index -> look slot
	for i := 0; i < n; i++ {
		at[s.resArm().lookOrig[i]] = uint8(i)
	}
	nr := 0
	for want := 0; want < n; want++ {
		i := at[want]
		if s.g.lookSel&(1<<i) == 0 {
			rest[nr] = s.resArm().look[i]
			nr++
		}
	}
	if fl&kop.DigRestTopAsk == kop.DigRandomOrder {
		for i := nr - 1; i > 0; i-- {
			j := s.rng.IntN(i + 1)
			rest[i], rest[j] = rest[j], rest[i]
		}
	}
	if rz == kop.ZoneLibrary && fl&kop.DigRestTop != 0 {
		for i := nr - 1; i >= 0; i-- {
			s.objs[rest[i]].zone = kop.ZoneLibrary
			s.objs[rest[i]].ver++
			pl.libTop(rest[i])
		}
		return
	}
	for i := 0; i < nr; i++ {
		o := rest[i]
		if rz == kop.ZoneLibrary || rz == kop.ZoneNone {
			s.objs[o].zone = kop.ZoneLibrary
			s.objs[o].ver++
			pl.libBottom(o)
		} else {
			s.placeFrom(o, rz, false, kop.ZoneLibrary, false)
		}
	}
}

// digPick selects looked-at card o. The picks move in the order they were
// made (gorge effDig's takeAnswered; CR 613.7m lets the controller order what
// enters at once), so they are kept at the front of look in that order, the
// rest keeping their window order behind them.
func (s *State) digPick(o Obj) {
	look := s.resArm().look[:s.g.nlook]
	orig := s.resArm().lookOrig[:s.g.nlook]
	at := bits.OnesCount32(s.g.lookSel)
	for i := at; i < len(look); i++ {
		if look[i] == o {
			oi := orig[i]
			copy(look[at+1:i+1], look[at:i])
			copy(orig[at+1:i+1], orig[at:i])
			look[at] = o
			orig[at] = oi
			s.g.lookSel |= 1 << at
			return
		}
	}
}
