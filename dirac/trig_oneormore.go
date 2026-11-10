package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "one or more" triggers on the kernel's
// simultaneous-event batches. Cold and out of line: every hot site reaches
// here behind genPrep.anyBatch or a Tf2Batch (+ TfNotToGrave) ability bit.
//
// gorge (rules/trigger_match.go) matches a ChangesZoneAll line per zone
// change, like ChangesZone, and inside an open zone batch queues it once,
// at its first matching move (zoneBatchIdx: the latch keys on the trigger
// line); outside a batch every move is its own batch. DamageDoneOnce
// latches per damaged referent inside a damage batch (a combat damage step
// is one). The kernel's batches are the stack frames that move or damage
// several objects as one event: an entry batch (fireEntries), leaveAll
// (DestroyAll, ChangeZoneAll off the battlefield), the creature
// casualties of one SBA pass (sba, trig_casualtybatch.go), one flush of an
// effect's sacrifice (flushSac), one multi-object target slot
// (execEachTarget: a multi-target Destroy / ChangeZone) and a combat
// damage pass (combatEventsGen). The latch needs no storage of its own:
// a line has fired in the open batch exactly when an instance of it was
// queued since the batch opened, so the batch records only its first
// pending index (genState.batchLatch, which was the unused pendOver byte).

const (
	batchGrave    uint8 = 1 << 7 // a leave to a graveyard is being checked (fireLeavesBatched)
	batchOpenMask uint8 = 0x1f   // the open batch's first pending index + 1 (0 = no batch)
)

// batchOpen opens a batch at the current pending queue (an outer open
// batch is kept: gorge's brackets nest, the outermost one counts) and
// returns the state batchClose restores.
func (s *State) batchOpen() uint8 {
	old := s.g.batchLatch
	if old&batchOpenMask == 0 {
		s.g.batchLatch = old&^batchOpenMask | (s.npend+1)&batchOpenMask
		s.g.batchTS = s.ts // CR 603.10a: the board this batch looks back to
	}
	return old
}

// batchClose closes the batch batchOpen opened (a cost-site sacrifice
// batch's batchSac bit is restored with it, act_proliferate.go).
func (s *State) batchClose(old uint8) {
	s.g.batchLatch = s.g.batchLatch&(batchGrave|batchHeld) | old&(batchOpenMask|batchSac)
}

// batchTrigOK is the batch gate of a Tf2Batch (+ TfNotToGrave) line: a
// "leave without dying" line is not checked for a leave to a graveyard,
// and a batch line already queued in the open batch is not queued again.
//
//go:noinline
func (s *State) batchTrigOK(l trigListener, a *kop.Ability, o Obj) bool {
	if a.Trig.Flags&kop.TfNotToGrave != 0 && s.g.batchLatch&batchGrave != 0 {
		return false
	}
	if DebugCzallNoLatch && a.Trig.Flags2&kop.Tf2Batch != 0 {
		return true
	}
	if a.Trig.Flags2&kop.Tf2Batch == 0 || s.g.batchLatch&batchOpenMask == 0 {
		return true
	}
	for i := int(s.g.batchLatch&batchOpenMask) - 1; i < int(s.npend); i++ {
		if p := &s.pend[i]; p.obj == l.obj && p.card == l.card && p.ab == l.ab {
			s.batchMemberAdd(i, a, o) // trig_batchmembers.go
			return false
		}
	}
	return true
}

// fireLeavesBatched is fireLeave's leaves check on a table with a batch line:
// the destination's graveyard bit is up while the listeners are checked.
//
//go:noinline
func (s *State) fireLeavesBatched(k *prep, sl Slot, z kop.Zone) {
	p := &s.perms[sl]
	if z == kop.ZoneGraveyard {
		s.g.batchLatch |= batchGrave
	}
	s.fireObj(k, kop.TrigLeaves, sl, p.obj, p.ctrl, czAmt(k, z))
	s.g.batchLatch &^= batchGrave
}

// leaveAllBatched is leaveAll's moves as one batch.
//
//go:noinline
func (s *State) leaveAllBatched(k *prep, order []Slot, z kop.Zone) {
	var zs [MaxPerm]kop.Zone
	for i, sl := range order {
		zs[i] = z
		if z == kop.ZoneGraveyard {
			zs[i] = s.dieZone(k, sl)
		}
	}
	old := s.batchOpen()
	for i, sl := range order {
		s.fireLeave(k, sl, zs[i])
	}
	s.batchClose(old)
	for i, sl := range order {
		s.removePerm(k, sl, zs[i], false)
	}
}

// replPlus applies the ReplDamagePlus replacements to damage n a source
// (srcSl, or a spell's card srcCard) controlled by srcCtrl would deal to
// the recipient (a permanent at sl controlled by rcp, or player rcp when
// sl is NoSlot): each matching one, in any battlefield, adds its Repl.A.
// gorge collects every damage replacement matching the event and, with two
// or more, asks the affected player for their order (CR 616.1; doubling
// and plus-N do not commute), which the kernel's order ask does not model:
// a plus-N replacement with another candidate stalls (StallReplPlus).
//
//go:noinline
func (s *State) replPlus(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, n int) int {
	g := k.gen
	add, c, raise := 0, 0, 0
	for m := s.statM; m.any(); m = m.rest() {
		b := m.first()
		if g.replBits[s.cardAt(b)]&rbPlusDmg == 0 || s.genLost(k, b) {
			continue
		}
		cd := &k.t.Cards[s.cardAt(b)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind != kop.AbReplacement || a.Repl.Kind != kop.ReplDamagePlus {
				continue
			}
			add2, raise2, ok := s.replPlusHit(k, b, a, srcCtrl, srcSl, srcCard, rcp, sl, combat, n)
			if !ok {
				continue
			}
			add += add2
			raise = max(raise, raise2)
			c++
		}
	}
	if c == 0 {
		return n
	}
	if c == 1 {
		// another damage replacement candidate (repl_gen.go's order ask)
		if g.anyRepl&rbDblDmg != 0 && s.dblDmgN2(k, srcCtrl, srcSl, srcCard, rcp, sl, combat) > 0 {
			c++
		}
		if g.anyRepl&rbPrevNC != 0 && !combat && sl != NoSlot && s.isCreature(k, sl) {
			for m := s.statM.and(s.ctrlM[SeatIx(rcp)]); m.any(); m = m.rest() {
				if b := m.first(); b != sl && g.replBits[s.cardAt(b)]&rbPrevNC != 0 && !s.genLost(k, b) && !s.noPrev(k, combat) {
					c++
					break
				}
			}
		}
		if combat && sl != NoSlot && s.prevCombatTo(k, sl) {
			c++
		}
	}
	if c >= 2 {
		// Two or more ReplDamagePlus additions in a table with no other
		// damage replacement kind all commute: every order gives the same
		// sum, so gorge's CR 616.1 ask is a no-op and the combat order walk
		// (replOrderCombat) poses it and applies the additions. Keep the
		// loud stall for any non-commuting competition.
		if !(raise == 0 && replPlusOnly(g) && (s.g.ro.on || s.g.plusBatchAsked)) {
			s.capFull(StallReplPlus)
		}
	}
	if raise > 0 {
		return raise
	}
	return n + add
}

// replPlusHit reports whether the ReplDamagePlus ability a of the permanent
// at b applies to n damage from (srcSl, or the spell's card srcCard when
// srcSl is NoSlot) controlled by srcCtrl to the recipient (a permanent at sl
// controlled by rcp, or player rcp when sl is NoSlot), and what it adds. A
// raise body (Ojer Axonil) is a candidate only while n is below the host's
// power; raise is then that power.
func (s *State) replPlusHit(k *prep, b Slot, a *kop.Ability, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, n int) (add, raise int, ok bool) {
	lc := s.perms[b].ctrl
	r := &a.Repl
	if r.Cond != 0 && !s.replHostLive(k, a, b) { // a granted max-speed gate (kw_maxspeed.go)
		return 0, 0, false
	}
	if r.C&kop.ReplDmgNonCombat != 0 && combat {
		return 0, 0, false
	}
	var to int16
	switch {
	case sl == NoSlot && rcp != lc:
		to = kop.ReplDmgPlayer | kop.ReplDmgOppPlayer
	case sl == NoSlot:
		to = kop.ReplDmgPlayer
	case rcp != lc:
		to = kop.ReplDmgPerm | kop.ReplDmgOppPerm
	default:
		to = kop.ReplDmgPerm
	}
	if r.C&to == 0 {
		return 0, 0, false
	}
	if srcSl != NoSlot {
		if !s.matchPermGen(k, r.Filter, srcSl, lc, b, layerFull) {
			return 0, 0, false
		}
	} else if !s.matchCardGen(k, r.Filter, srcCard, srcCtrl, lc, NoObj) {
		return 0, 0, false
	}
	if r.C&kop.ReplDmgRaise != 0 {
		// "less than <host>'s power ... deals damage equal to its power
		// instead" (Ojer Axonil): a candidate only while the amount is below
		// the host's current power (gorge DamageAmount$ LTX)
		if pw, _ := s.ptGen(k, b); n < pw {
			return 0, pw, true
		}
		return 0, 0, false
	}
	return s.dmgPlusAmt(k, r, b), 0, true
}
