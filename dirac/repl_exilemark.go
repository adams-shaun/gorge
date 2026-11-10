package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExtExileExMark: a Pump's or Fight's
// ReplaceDyingDefined$ Targeted. gorge registerReplaceDying (effects/
// damage.go) registers one until-end-of-turn Moved replacement over the
// targeted objects: Battlefield to Graveyard becomes exile (CR 614.1a,
// 614.9). The kernel's per-slot mechExileEx mark and its exMarkRec are that
// replacement.

// DebugExileExMarkOff is the mutation switch of TestExileexmarkMutation:
// the op does nothing, so the marked creature dies to the graveyard.
var DebugExileExMarkOff bool

// exileExMark is ExtExileExMark for the one object in Obj (the interpreter
// runs it once per object of a multi-object target slot).
func (s *State) exileExMark(k *prep, in *kop.Instr) {
	if DebugExileExMarkOff {
		return
	}
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot {
		return
	}
	if in.C != 0 {
		// a Fight: gorge fightPairs keeps the pair only while both members
		// are creatures on the battlefield, else registerReplaceDying gets
		// nothing (a fighter that left, a target that stopped being one)
		f := s.objRef(k, kop.Ref(in.C))
		if f == NoSlot || !s.isCreature(k, f) || !s.isCreature(k, sl) {
			return
		}
	}
	s.markExileEx(k, sl)
}

// exMarkRec is one exile-instead-of-dying effect on slot sl, made by src.
type exMarkRec struct {
	sl   Slot
	src  Obj
	live bool
}

// maxExMarks is how many such effects one turn can hold; more stall by name.
const maxExMarks = 6

func (s *State) exMarkCount(sl Slot) int {
	n := 0
	for i := range s.cold.exMark {
		if r := &s.cold.exMark[i]; r.live && r.sl == sl {
			n++
		}
	}
	return n
}

// A move to a graveyard that two or more exile-instead replacements compete
// for is parked (CR 616.1): gorge leaves the object where it is, finishes what
// it was doing and asks its controller which applies. All of them exile, so the
// answer only settles the ask; the move then runs with nothing left to order.
// A replacement sending the card elsewhere (graveLib) changes what the answer
// means, and stays a stall in dieZone and exileAnyDest.
type parkKind uint8

const (
	parkNone     parkKind = iota
	parkSBA               // a state-based pass's casualty
	parkDiscard           // discardObj
	parkCycle             // cycleDiscard
	parkLeave             // leave
	parkFinish            // finishResolve; x is its fizzled argument
	parkAnswered          // asked: its move no longer competes
	parkMass              // a mass move in a resolution, one at a time: what an earlier move took away no longer competes for it
	parkMill              // a milled card, off the library's top when asked; x bit 0 is RememberMilled$, bit 1 Imprint$
	parkRest              // a surveil or look rest card already off the library, moved one at a time; x is 1 for RememberMoved$
	parkDread             // a manifest dread window's other card, still on the library list; its marker fired before the ask
)

// parkedMove is one parked move, oldest first in s.cold.parked; x is a cycled
// ability's paid X, and src the sources competing when it parked: the ask offers
// them even if one has left since, as a casualty of the same state-based pass
// has. A resolution waits with its moves, and goes on after the last answer.
type parkedMove struct {
	obj  Obj
	kind parkKind
	x    uint8
	src  [maxParkSrc]Obj
	// fl holds the count of src in its low nibble and the flags parkLater and
	// parkNext above it, one byte to a record
	fl uint8
}

// nsrc is the number of sources src holds.
func (r *parkedMove) nsrc() int { return int(r.fl & parkNsrcMask) }

// setNsrc records n (at most maxParkSrc) sources, keeping the flags.
func (r *parkedMove) setNsrc(n int) { r.fl = r.fl&^parkNsrcMask | uint8(n) }

const (
	parkNsrcMask uint8 = 15 // fl's source count

	// parkLater: a state-based batch's answer came before this move's ask
	parkLater uint8 = 16
	// parkNext: a state-based pass parked this move while an earlier pass's asks
	// were still to come, so gorge's check unattaches before it asks for this one
	parkNext uint8 = 32
	// parkSacd: a parkMass move is a sacrifice, emitted with its move (repl_sacbatch.go)
	parkSacd uint8 = 64
	// parkBatchEnd: the last parkMass move of a sacrifice batch (with parkSacd),
	// which closes the batch kept open across the asks to the latch held in x
	parkBatchEnd uint8 = 128
	// parkRemember: a destroy-all with RememberDestroyed$ parked this victim on a
	// CR 616.1 ask; it joins the remembered set when its move reaches the
	// graveyard (act_remember.go, runParkedMass). A destroyed victim is never
	// parkSacd, so it shares parkBatchEnd's bit: fl's byte is full.
	parkRemember uint8 = 128
)

// manaSacParks reports whether a source sacrificed for mana now can wait for its
// order ask, with the payment going on after the answer: the build's mana step, or
// a resolution's planned payment of a mana cost.
func (s *State) manaSacParks() bool {
	if s.res.active {
		return s.res.ask == raGenPay && s.g.pickZ == kop.PayMana && s.g.pickF&(pfWard|pfModes) == 0
	}
	return s.ctl == ctlBuild && s.bld().stage == bsMana
}

// raParkFinish is resCtx.ask while a resolution outside the program loop has its
// parked moves to wait for, and ends on the last answer. It is never posed.
const raParkFinish uint8 = 99

// maxParked is how many moves one cost, pass, mass move or mill can park; more stall
// by name. 58 is the most that keeps every pool under the State bar.
const maxParked = 48

// maxParkSrc is how many sources one parked move can offer; more stall by name.
const maxParkSrc = 8

// graveMoveSources fills out with the exile-instead sources competing for
// object o's move to a graveyard and returns how many. Marks on a creature are
// counted apart (exMarkCount).
func (s *State) graveMoveSources(k *prep, o Obj, out *[MaxPerm]Obj) int {
	g := k.gen
	ob := &s.objs[o]
	n := 0
	for m := s.statM; m.any(); m = m.rest() {
		b := m.first()
		sc := s.cardAt(b)
		if g.replBits[sc]&rbExileOnLeave == 0 || s.genLost(k, b) {
			continue
		}
		var hit bool
		if ob.zone == kop.ZoneBattlefield {
			hit = s.matchPerm(k, g.exileOnLeave[sc], ob.slot, s.perms[b].ctrl, b)
		} else {
			hit = g.anyExileAny && g.exileAnyC[sc] && s.matchCardGen(k, g.exileOnLeave[sc], ob.card, ob.owner(), s.perms[b].ctrl, o)
		}
		if hit {
			out[n] = s.perms[b].obj
			n++
		}
	}
	return n
}

// graveMoveCompetes reports whether object o's move to a graveyard is one the
// ask settles: two or more exile-instead effects and nothing that sends it
// elsewhere, and not asked already.
func (s *State) graveMoveCompetes(k *prep, o Obj) bool {
	g := k.gen
	if g == nil || g.anyRepl&rbExileOnLeave == 0 && g.ix[mechExileEx] == arenaNone {
		return false
	}
	if g.anyRepl&rbGraveLib != 0 && s.graveRedirect(k, s.objs[o].card, kop.ZoneGraveyard) == kop.ZoneLibrary {
		return false
	}
	if ob := &s.objs[o]; g.anyFinality && ob.zone == kop.ZoneBattlefield && s.perms[ob.slot].ctrN(s, int(kop.CtrFinality)) > 0 {
		// a finality counter exiles the permanent at the move boundary,
		// before the replacements are collected (dieZone; gorge
		// finalityReplacementApplies): nothing is left to order
		return false
	}
	n := 0
	if g.anyRepl&rbExileOnLeave != 0 {
		var src [MaxPerm]Obj
		n = s.graveMoveSources(k, o, &src)
	}
	if ob := &s.objs[o]; ob.zone == kop.ZoneBattlefield && s.mSl(k, mechExileEx, ob.slot) {
		n += s.exMarkCount(ob.slot)
	}
	return n > 1 && !s.moveAnswered(o)
}

// moveAnswered reports whether o's parked move has had its answer.
func (s *State) moveAnswered(o Obj) bool {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkAnswered && r.obj == o {
			return true
		}
	}
	return false
}

// takeAnswered is moveAnswered, and closes the record: the move is happening.
func (s *State) takeAnswered(o Obj) bool {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkAnswered && r.obj == o {
			s.unpark(i)
			return true
		}
	}
	return false
}

func (s *State) unpark(i int) {
	p := s.cold.parked
	copy(p[i:], p[i+1:])
	p[len(p)-1] = parkedMove{}
}

// park queues o's move; false when it does not compete or the queue is full,
// and the caller moves it now (a full queue stalls there).
func (s *State) park(k *prep, o Obj, kind parkKind, x uint8) bool {
	if s.parkedWaitingOn(o) {
		return true // a pass re-run under its own ask
	}
	if !s.graveMoveCompetes(k, o) {
		return false
	}
	var src [MaxPerm]Obj
	n := 0
	if k.gen.anyRepl&rbExileOnLeave != 0 {
		n = s.graveMoveSources(k, o, &src)
	}
	if n > maxParkSrc {
		return false
	}
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkNone {
			*r = parkedMove{obj: o, kind: kind, x: x}
			r.setNsrc(n)
			copy(r.src[:], src[:n])
			return true
		}
	}
	return false
}

// parkedWaitingOn reports whether o's move is in the queue with its ask to come.
func (s *State) parkedWaitingOn(o Obj) bool {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind != parkNone && r.kind != parkAnswered && r.obj == o {
			return true
		}
	}
	return false
}

// parked reports whether o's move is in the queue.
func (s *State) parked(o Obj) bool {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind != parkNone && r.obj == o {
			return true
		}
	}
	return false
}

// parkedWaiting reports a parked move whose ask is still to come.
func (s *State) parkedWaiting() bool {
	return len(s.cold.parked) != 0 && s.cold.parked[0].kind != parkNone && s.cold.parked[0].kind != parkAnswered
}

// exileOrderAsk parks the casualties of a state-based pass with several
// exile-instead effects before any of the pass moves: a source dying beside
// them still applies (CR 704.3), and they do not depart with it. It poses gorge's
// CR 616.1 order ask for the first once nothing else is left to die.
func (s *State) exileOrderAsk(k *prep, die, gy, pwz, aur *slotMask, askAfter *bool) bool {
	var order [MaxPerm]Slot
	n := 0
	for p := uint8(0); p < 2; p++ {
		n = s.battlefieldOrder(p, die.or(*gy).and(s.ctrlM[p]), order[:], n)
	}
	fresh := false
	busy := s.parkedWaiting()
	for _, sl := range order[:n] {
		o := s.perms[sl].obj
		was := s.parkedWaitingOn(o)
		if s.park(k, o, parkSBA, 0) {
			fresh = fresh || !was
			if !was && busy {
				for i := range s.cold.parked {
					if r := &s.cold.parked[i]; r.obj == o && r.kind == parkSBA {
						r.fl |= parkNext
					}
				}
			}
			*die, *gy, *pwz, *aur = die.without(sl), gy.without(sl), pwz.without(sl), aur.without(sl)
		}
	}
	if die.or(*gy).any() {
		// The rest of the batch applies around the parked casualties, and
		// gorge's pass ends at the ask (checkStateBased): no further pass runs
		// before it, so an Aura whose host left in the batch is swept, and a
		// token it moved ceases, only at the check after the answer.
		*askAfter = fresh
		return false
	}
	if !s.parkedWaiting() {
		return false
	}
	// The batch's later asks follow the first with no state-based check between them
	// (CR 704.3): a token an earlier answer exiled is still there. So is one that
	// died beside the parked casualties and moved in an earlier pass of this check:
	// it ceases at the next check. Only a pass that parks them now first ceases the
	// tokens already outside the battlefield, as gorge's pass does before its ask.
	if fresh && s.cold.parked[0].fl&parkLater == 0 {
		s.ceaseTokensGen()
	}
	s.ctl = ctlExileOrder
	return true
}

// exileOrderOptions asks the moving object's controller, or its owner off the
// battlefield. The mode tells a mark, by its index, from a source.
func (s *State) exileOrderOptions(k *prep, d *Decision) {
	o := s.cold.parked[0].obj
	ob := &s.objs[o]
	d.Kind, d.Sub, d.Player = DkChoose, ChGenReplOrder, ob.owner()
	if ob.zone == kop.ZoneBattlefield {
		d.Player = s.perms[ob.slot].ctrl
		for i := range s.cold.exMark {
			if r := &s.cold.exMark[i]; r.live && r.sl == ob.slot {
				d.add(MakeAction(AkChoose, uint8(r.src), 0, uint8(i)))
			}
		}
	}
	r := &s.cold.parked[0]
	for i, src := range r.src[:r.nsrc()] {
		d.add(MakeAction(AkChoose, uint8(src), 0, uint8(maxExMarks+i)))
	}
}

// applyExileOrder records the answer and runs the parked move, leaving the
// creature only the mark answered. After a state-based move the next pass moves
// what has come to die meanwhile, then asks for the next.
func (s *State) applyExileOrder(k *prep, a Action) {
	r := s.cold.parked[0]
	if ob := &s.objs[r.obj]; ob.zone == kop.ZoneBattlefield {
		for i := range s.cold.exMark {
			if m := &s.cold.exMark[i]; m.live && m.sl == ob.slot {
				m.live = i == int(a.Mode())
			}
		}
	}
	s.cold.parked[0].kind = parkAnswered
	s.ctl = ctlPriority
	resumePay := false
	if r.kind == parkLeave && r.x != 0 {
		s.pl[s.perms[s.objs[r.obj].slot].ctrl].pool[r.x>>1&7] += r.x >> 4
		s.ctl = ctlBuild
		if s.res.active {
			s.ctl = ctlResolve
			resumePay = true
		}
	}
	s.runParked(k, r)
	s.takeAnswered(r.obj)
	if r.kind == parkDread {
		// the manifest dread window goes on after the answer: the next window
		// (when Amount$ is >1) is asked with the other card now moved
		s.res.left--
		if !s.dreadAsk(k) {
			s.ctl = ctlResolve // face_manifest.go's ask
			return
		}
	}
	if r.kind == parkSBA {
		for i := range s.cold.parked {
			s.cold.parked[i].fl |= parkLater
		}
		if h := &s.cold.parked[0]; h.kind == parkSBA && h.fl&parkNext != 0 && s.g.lingerM.any() {
			// the answered pass is over and the next one's check unattaches first
			h.fl &^= parkNext
			s.lingerDrain(k)
		}
		return
	}
	s.massMovesGoOn(k)
	if s.parkedWaiting() {
		s.ctl = ctlExileOrder
	} else if resumePay {
		s.payManaResume(k)
	} else if r.kind == parkDiscard && r.x == parkCleanup {
		s.finishCleanup(k)
	} else if s.res.ask == raParkFinish {
		s.res.ask = raNone
		s.finishResolve(k, false)
	} else if s.res.active {
		s.continueRes(k, 0)
	}
}

// parkCleanup is a parkDiscard's x for a discard to hand size: the last answer
// finishes the cleanup step.
const parkCleanup = 1

// parkManaHold is a parkLeave's x for a source sacrificed for mana while a cost is
// being tapped for: the answer adds its n mana of colour c and returns to the build,
// whose payment waits on the ask.
func parkManaHold(c int, n uint8) uint8 {
	return n<<4 | uint8(c)<<1 | 1
}

// massMovesGoOn runs the mass moves at the head of the queue that an earlier move
// has left with one exile-instead effect or none, and gives the next one its ask
// over the sources still there. Gorge makes these moves one at a time.
func (s *State) massMovesGoOn(k *prep) {
	for len(s.cold.parked) != 0 && (s.cold.parked[0].kind == parkMass || s.cold.parked[0].kind == parkMill || s.cold.parked[0].kind == parkRest || s.cold.parked[0].kind == parkDiscard) {
		r := &s.cold.parked[0]
		var src [MaxPerm]Obj
		n := s.graveMoveSources(k, r.obj, &src)
		if n > 1 && n <= maxParkSrc {
			r.setNsrc(n)
			copy(r.src[:], src[:n])
			return
		}
		m := *r
		s.unpark(0)
		s.runParked(k, m)
	}
}

// parkMills queues p's next n milled cards when the first's move competes (gorge
// asks and moves them one at a time, each on top until its turn); false: nothing is
// queued and the mill goes on as usual.
func (s *State) parkMills(k *prep, p uint8, n int, c int) bool {
	pl := &s.pl[p]
	if pl.libLo >= pl.libHi || !s.graveMoveCompetes(k, pl.lib[pl.libLo]) {
		return false
	}
	x := uint8(0)
	if c&kop.MvRemember != 0 {
		x = 1
	}
	if c&kop.MvImprint != 0 {
		x |= 2
	}
	for i := 0; i < n && int(pl.libLo)+i < int(pl.libHi); i++ {
		o := pl.lib[int(pl.libLo)+i]
		if s.park(k, o, parkMill, x) {
			continue
		}
		if !s.parkMillQuiet(o, x) {
			s.capFull(StallReplCands)
			break
		}
	}
	return true
}

// parkMassQuiet queues a batch move with one exile-instead effect or none,
// whose turn comes in the queue all the same (gorge emits a sacrifice or
// destroy batch's later moves only after an earlier one's answer); nil when
// the queue is full.
func (s *State) parkMassQuiet(o Obj) *parkedMove {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkNone {
			*r = parkedMove{obj: o, kind: parkMass}
			return r
		}
	}
	return nil
}

// parkDiscardQuiet queues a discarded card with one exile-instead effect or
// none, whose turn comes in the queue all the same; false when the queue is full.
func (s *State) parkDiscardQuiet(o Obj, x uint8) bool {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkNone {
			*r = parkedMove{obj: o, kind: parkDiscard, x: x}
			return true
		}
	}
	return false
}

// parkMillQuiet queues a milled card with one exile-instead effect or none, whose
// turn comes in the queue all the same; false when the queue is full.
func (s *State) parkMillQuiet(o Obj, x uint8) bool {
	for j := range s.cold.parked {
		if r := &s.cold.parked[j]; r.kind == parkNone {
			*r = parkedMove{obj: o, kind: parkMill, x: x}
			return true
		}
	}
	return false
}

// parkRests parks the rest of a surveil or look from the first card whose
// graveyard move competes (CR 616.1): gorge asks the order one card at a time
// and moves them in that order, so that card and every later one wait in the
// queue, competing or not. It returns the first parked index, or -1 when the
// caller moves them all now.
func (s *State) parkRests(k *prep, rest []Obj, remember bool) int {
	first := -1
	for i, o := range rest {
		if s.graveMoveCompetes(k, o) {
			first = i
			break
		}
	}
	if first < 0 {
		return -1
	}
	x := uint8(0)
	if remember {
		x = 1
	}
	for _, o := range rest[first:] {
		if s.park(k, o, parkRest, x) {
			continue
		}
		if !s.parkRestQuiet(o, x) {
			s.capFull(StallReplCands)
			break
		}
	}
	return first
}

// parkRestQuiet queues a rest card with one exile-instead effect or none, whose
// turn comes in the queue all the same; false when the queue is full.
func (s *State) parkRestQuiet(o Obj, x uint8) bool {
	for i := range s.cold.parked {
		if r := &s.cold.parked[i]; r.kind == parkNone {
			*r = parkedMove{obj: o, kind: parkRest, x: x}
			return true
		}
	}
	return false
}

// runParkedRest takes a rest card off its owner's library and sends it to the
// graveyard zone, whose exile-instead effects its answer has settled.
func (s *State) runParkedRest(k *prep, o Obj, remember bool) {
	if s.objs[o].zone != kop.ZoneLibrary {
		return // the card left the look while it waited
	}
	s.touch(o)
	s.objs[o].zone = kop.ZoneNone
	s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
	if remember {
		s.surveilRemember(o) // RememberMoved$ (zone_rememberwrites.go)
	}
}

// runParkedMill takes o off its owner's library top and sends it to the graveyard
// zone, whose exile-instead effects its answer has settled.
func (s *State) runParkedMill(k *prep, o Obj, x uint8) {
	pl := &s.pl[s.objs[o].owner()]
	if pl.libLo >= pl.libHi || pl.lib[pl.libLo] != o {
		return
	}
	pl.libLo++
	s.touch(o)
	s.objs[o].zone = kop.ZoneNone
	s.placeFromK(k, o, kop.ZoneGraveyard, false, kop.ZoneLibrary, false)
	c := 0
	if x&1 != 0 {
		c |= kop.MvRemember
	}
	if x&2 != 0 {
		c |= kop.MvImprint
	}
	s.millMark(o, c)
}

// leaveParked is leave to the graveyard inside a resolution, parked when its
// move competes.
func (s *State) leaveParked(k *prep, sl Slot) {
	if !s.park(k, s.perms[sl].obj, parkLeave, 0) {
		s.leave(k, sl, kop.ZoneGraveyard, false)
	}
}

// parkCost is park for a cost's move; a free cast's cost inside a resolution
// has no way back to it from the ask, so it stays a stall.
func (s *State) parkCost(k *prep, o Obj, kind parkKind, x uint8) bool {
	return !s.res.active && s.park(k, o, kind, x)
}

// parkSpellExit parks a resolved or fizzled spell's move to a graveyard, the
// spell still on the stack. A card leaving as a face or a paradigm goes
// elsewhere first and is not parked.
func (s *State) parkSpellExit(k *prep, fizzled bool) bool {
	e := &s.res.e
	if e.kind != seSpell || s.objs[e.obj].zone != kop.ZoneStack || e.exilesOnLeave() || s.g.nfq > 0 && !fizzled {
		return false
	}
	if cd := &k.t.Cards[s.objs[e.obj].card]; cd.Face != 0 || cd.Paradigm != 0 {
		return false
	}
	x := uint8(0)
	if fizzled {
		x = 1
	}
	if !s.park(k, e.obj, parkFinish, x) {
		return false
	}
	s.ctl = ctlExileOrder
	return true
}

func (s *State) runParked(k *prep, r parkedMove) {
	switch r.kind {
	case parkDiscard:
		s.discardObj(k, r.obj)
	case parkCycle:
		s.cycleDiscard(k, r.obj, r.x)
	case parkMass:
		s.runParkedMass(k, r)
	case parkLeave, parkSBA:
		s.leave(k, s.objs[r.obj].slot, kop.ZoneGraveyard, false)
	case parkFinish:
		s.finishResolve(k, r.x != 0)
	case parkMill:
		s.runParkedMill(k, r.obj, r.x)
	case parkRest:
		s.runParkedRest(k, r.obj, r.x != 0)
	case parkDread:
		s.runParkedDread(k, r.obj)
	}
}

// runParkedDread takes an answered manifest-dread window's other card out of
// its owner's library and sends it to the graveyard zone, whose exile-instead
// effects its answer has settled. The window's marker already fired before the
// ask; the caller goes on with the window (face_manifest.go).
func (s *State) runParkedDread(k *prep, o Obj) {
	if s.objs[o].zone != kop.ZoneLibrary {
		return // the card left the library while it waited
	}
	from := s.detach(o)
	s.placeFromK(k, o, kop.ZoneGraveyard, false, from, false)
}
