package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the queue order of one sacrifice's triggers. gorge matches
// a sacrificed permanent's departure (one MoveZone, Sacrificed set) against
// the pre-departure board in ONE face walk (rules checkTriggers'
// look-back checkFaceTriggers): source by source in battlefield order, each
// source's trigger lines in card order, so a source's "whenever one or more
// creatures you control die" queues before its own "whenever you sacrifice
// a permanent" when that is the line order (Scavenger's Talent's L1 and L2).
// The kernel fires the Sacrificed triggers in fireSac, just before the
// departure, and the dies/leaves triggers in fireLeave: event by event.
// sacLeaveOrder re-sorts that one departure's block into gorge's walk order.

// DebugGyBfSacFirst is the mutation switch: the departure's
// block keeps the kernel's event-by-event order. TestGybfMutation requires
// the lockstep to diverge.
var DebugGyBfSacFirst bool

// DebugGyBfNoCounter is the second mutation switch: handGraveArm arms no
// entry counter for a plain Graveyard pick (Scavenger's Talent's return
// enters without its FINALITY counter).
var DebugGyBfNoCounter bool

// DebugGyBfOwnerEntry is the third mutation switch: a GainControl$ graveyard
// pick enters under the card's owner instead of the chooser (gorge's
// enteringControllerForAttack), so an opponent's card returns to them.
var DebugGyBfOwnerEntry bool

// gameGraveCands adds a plain Graveyard hidden pick's candidates when it is
// game-wide: with GainControl$ (SrUnderYourControl, an ownerless clause) or
// with SrGameGrave (no fetch player at all), gorge's gameWide pool is every
// player's graveyard in seat order, matched against the card's real owner
// with the resolving controller as the relative You. The ask stays with the
// chooser; the GainControl$ card enters under them (moveFound), a
// SrGameGrave one under its owner (its Exile destination's zone owner).
// Picks with neither flag keep the chooser's own graveyard.
//
//go:noinline
func (s *State) gameGraveCands(k *prep, in *kop.Instr, d *Decision) {
	ctrl := s.res.e.ctrl
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		pl := &s.pl[p]
		for _, o := range pl.grave[:pl.graveN] {
			if s.picksHas(o) {
				continue
			}
			if !s.tokenLeftBattlefield(o) && s.matchCardGen(k, in.F, s.objs[o].card, s.objs[o].owner(), ctrl, o) { // a token off the battlefield is no card (CR 111.8)
				d.add(MakeAction(AkChoose, TCode(o), 0, 0))
			}
		}
	}
}

// sacLeaveOrder sorts pend[start:npend] by gorge's walk position, where
// start is the first of the Sacrificed triggers fireSac queued for o just
// before n0 (npend as fireLeave began). Without such a block, or with
// nothing queued since, it leaves the queue alone.
//
//go:noinline
func (s *State) sacLeaveOrder(k *prep, o Obj, n0 uint8) {
	if DebugGyBfSacFirst || s.npend <= n0 {
		return
	}
	start := n0
	for start > 0 && s.sacTrigOf(k, &s.pend[start-1], o) {
		start--
	}
	if start == n0 {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	var key [MaxPending]uint16
	for i := start; i < s.npend; i++ {
		key[i] = s.sacWalkKey(&s.pend[i], ord[:n])
	}
	// a stable insertion sort: equal keys (a Panharmonicon echo) keep
	// their queue order
	for i := start + 1; i < s.npend; i++ {
		pt, kk := s.pend[i], key[i]
		j := i
		for j > start && key[j-1] > kk {
			s.pend[j], key[j] = s.pend[j-1], key[j-1]
			j--
		}
		s.pend[j], key[j] = pt, kk
	}
}

// sacEachDeparture is flushSac's fireSac / dieZone / fireLeave loops for a
// sacrifice of several permanents with a Sacrificed listener on the table:
// gorge walks each departure's triggers on its own, in departure order, so
// the queue holds each permanent's Sacrificed and dies/leaves triggers
// together (sacLeaveOrder sorts that block) instead of every Sacrificed
// trigger before every dies trigger (Season of Loss against Scavenger's
// Talent). The destinations are read before any trigger is queued, as the
// separate loops did.
//
//go:noinline
func (s *State) sacEachDeparture(k *prep, ord []Slot, zs *[MaxPerm]kop.Zone) {
	for i, sl := range ord {
		zs[i] = s.dieZone(k, sl)
	}
	for i, sl := range ord {
		s.fireSac(k, sl)
		s.fireLeave(k, sl, zs[i])
	}
}

// sacTrigOf reports a pending printed Sacrificed trigger whose event object
// is o.
func (s *State) sacTrigOf(k *prep, pt *pendingTrig, o Obj) bool {
	if pt.trigObj != o {
		return false
	}
	ab := k.t.Cards[pt.card].Abilities
	return int(pt.ab) < len(ab) && ab[pt.ab].Kind == kop.AbTriggered && ab[pt.ab].Trig.Event == kop.TrigSacrificed
}

// sacWalkKey is pt's place in gorge's face walk: its source's battlefield
// position, then the source's own lines (ability order) before the
// abilities granted to it. A source off the battlefield sorts last.
func (s *State) sacWalkKey(pt *pendingTrig, ord []Slot) uint16 {
	sl := s.objs[pt.obj].slot
	r := len(ord)
	for i, q := range ord {
		if q == sl {
			r = i
			break
		}
	}
	g := uint16(0)
	if sl == NoSlot || s.cardAt(sl) != pt.card {
		g = 1
	}
	return uint16(r)<<9 | g<<8 | uint16(pt.ab)
}
