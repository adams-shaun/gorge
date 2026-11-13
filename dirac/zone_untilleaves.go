package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// dropLinkHand removes link i's bit from genState.linkHand (the links
// above it shift down one) and reports whether it was set.
//
//go:noinline
func (s *State) dropLinkHand(i int) bool {
	m := s.g.linkHand
	set := m>>uint(i)&1 != 0
	lo := m & (1<<uint(i) - 1)
	s.g.linkHand = lo | (m>>uint(i+1))<<uint(i)
	h := s.g.linkHide // the Hideaway mask shifts with it (kw_hideaway.go)
	s.g.linkHide = h&(1<<uint(i)-1) | (h>>uint(i+1))<<uint(i)
	a := s.g.linkAeg // and the Assimilation Aegis mask (zone_exilethenattach.go)
	s.g.linkAeg = a&(1<<uint(i)-1) | (a>>uint(i+1))<<uint(i)
	return set
}

// handExileLink records a hand card exiled until the resolving source
// leaves the battlefield (Deep-Cavern Bat; gorge recordExileReturnFor): a
// link that returns it to its owner's hand (sweepExileReturn's From zone).
//
//go:noinline
func (s *State) handExileLink(o Obj) {
	e := &s.res.e
	if s.g.nlinks >= uint8(len(s.g.links)) {
		s.capFull(StallLinks)
		return
	}
	l := &s.g.links[s.g.nlinks]
	l.src, l.sver, l.obj = e.obj, s.objs[e.obj].ver, o
	s.g.linkHand |= 1 << s.g.nlinks
	s.g.nlinks++
}

// handExileHostGone reports whether a hand exile until the source leaves
// moves nothing because its source has already left the battlefield (gorge
// changeZonePrelude: exileHostGoneFor stops the ChangeZone before any ask).
//
//go:noinline
func (s *State) handExileHostGone() bool {
	e := &s.res.e
	ob := &s.objs[e.obj]
	return ob.zone != kop.ZoneBattlefield || ob.ver != e.srcVer
}

// An until-leaves exile with a return rider (Lagrella, the Magpie):
// "When an exiled card enters under your control this way, put two +1/+1
// counters on it." gorge (engine/rules/exile_return_ability.go,
// runExileReturnAbility, CR 610.3) runs the ReturnAbility$ at the exile-return
// sweep, inline right after the card's MoveZone: a returned card that
// ReturnValid$ Card.IsTriggerRemembered+YouCtrl matches, i.e. one that comes
// back under the control of the exiler's last controller (its owner), gets the
// counters; an opponent's creature returns with none. The count is the
// OpExileUntilLeaves A operand (kgen lower_exilereturncounters.go; unset = no rider).
//
// The rider is read off the source's CARD, not the link (no State byte): a
// card with two until-leaves exiles of which only one has a rider would
// misread, and the corpus has none.

// tableLinkCtr reports whether some until-leaves exile carries a return rider.
func tableLinkCtr(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExileUntilLeaves && in.A.K == kop.VConst && in.A.N > 0 {
					return true
				}
			}
		}
	}
	return false
}

// linkRetCtrs is the counters the departed source object o's card gives the
// returning card ret when the source's last controller was srcCtrl (0 =
// none): ReturnValid$ ...+YouCtrl, the returned card under its owner's control.
//
//go:noinline
func (s *State) linkRetCtrs(k *prep, srcCtrl uint8, o Obj, ret Obj) int {
	if s.objs[ret].owner() != srcCtrl {
		return 0
	}
	cd := &k.t.Cards[s.objs[o].card]
	for j := range cd.Abilities {
		for _, in := range cd.Abilities[j].Prog {
			if in.Op == kop.OpExileUntilLeaves && in.A.K == kop.VConst && in.A.N > 0 {
				return int(in.A.N)
			}
		}
	}
	return 0
}

// linkRetPut puts the return rider's n counters on the card that just entered
// at nsl (as OpPutCounter: replaceable, and a counter-added listener fires).
// n is read BEFORE the card enters: it enters into the very slot the source
// just left.
//
//go:noinline
func (s *State) linkRetPut(k *prep, n int, nsl Slot) {
	if nsl == NoSlot || !s.occ.has(nsl) {
		return
	}
	n = s.placeCounters(k, nsl, int(kop.CtrP1P1), n, s.perms[nsl].ctrl, ctrPutEffect)
	if n > 0 && k.anyCtrAdded {
		s.ctrNote(k, nsl, int(kop.CtrP1P1))
	}
	if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, nsl, s.perms[nsl].obj, s.perms[nsl].ctrl, int16(n))
	}
}

// linkRetStall is loud where the card does not enter inline (a returning
// Aura asked its host, an as-enters choice asked first): the rider's counters
// would follow the answer, which the kernel does not carry.
//
//go:noinline
func (s *State) linkRetStall(k *prep, srcCtrl uint8, o Obj, ret Obj) {
	if s.linkRetCtrs(k, srcCtrl, o, ret) > 0 {
		s.capFull(StallLinkCtr)
	}
}

// diffCtrlCap bounds a TargetsWithDifferentControllers$ slot by the number of
// distinct controllers among its candidates at the slot's start (gorge
// target_modes.go oneEachTargetBounds: no legal answer picks more targets
// than there are controllers, so "any number" over two players closes after
// two picks, and after one when only one controller has a candidate, with no
// done-only ask; Lagrella, the Magpie). It returns that count of controllers.
//
//go:noinline
func (s *State) diffCtrlCap(k *prep, t *kop.TargetSpec, ti uint8, mt *multiTgt) uint8 {
	var buf [MaxOptions]uint8
	n := s.multiCands(k, t, ti, &buf)
	var seen [MaxSeats]bool
	for i := 0; i < n; i++ {
		if isObjTarget(TCode(buf[i])) {
			seen[SeatIx(s.ctrlOfObj(Obj(buf[i])))] = true
		}
	}
	d := seatCount(&seen)
	if d > 0 && mt.max > d {
		mt.max = max8(d, mt.min)
	}
	return d
}

// payReturnPark poses, once a cost's sacrifice has been paid, the as-enters
// choice of a card the sacrificed permanent held in exile until it left (In
// the Pale Moonlight's sacrificed Malfunctioning Holodeck returning Kai Winn,
// or a returning Aura choosing its host among several): gorge asks it with
// the card still in exile, mid-payment, so the rest of the resolution waits
// for the answer (CR 614.12, 610.3). It reports whether the payment is
// suspended on that ask.
//
//go:noinline
func (s *State) payReturnPark() bool {
	p := &s.g.pre
	if s.k().gen == nil || p.state != preReturn {
		return false
	}
	p.state = preReturnPay
	s.res.ask, s.res.askP = raGenRetChoice, s.objs[p.obj].owner()
	s.ctl = ctlResolve
	return true
}
