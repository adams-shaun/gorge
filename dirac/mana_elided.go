package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// coldElideArm is the mirror's recognition mark: the lead gorge side's
// elided mana activations (and the plan's sub-cost spends) were applied out
// of band before this answer's processing, so the kernel's Apply-time
// re-derivation (applyPriority's prefix funding) must not fund the play
// again -- the sources are tapped, the floats are in the pool, and the plan
// paid its sub-costs. It is set by the harness's mirror (dirac/oracle
// elided.go) before the kernel side's Step or Apply of the same answer, and
// cleared once that processing is done. One byte, every table.
var coldElideArm = newCold[bool]("elideArm", func(g *genPrep) int { return 1 })

// coldElideFifo is the mirror's queue of trigger groups: gorge's engine drains
// its trigger queue after each plan activation, so the Taps triggers one
// activation's tap queued are on the stack (APNAP among themselves) before the
// next source is tapped. The kernel's placeTriggers would group the whole
// span's triggers by controller; it places the mirror-queued head of the
// pending queue one group at a time instead (cast.go placeTriggers). Byte 0 is
// the group count, the bytes after it the group sizes in queue order (the last
// takes the overflow). Eight bytes, every table.
var coldElideFifo = newCold[uint8]("elideFifo", func(g *genPrep) int { return 8 })

// elideNoteQueued records the triggers one mirrored tap queued (npend moved
// from before to now) as a group of the head block, when every trigger pending
// before it is a mirror-queued one.
func (s *State) elideNoteQueued(before uint8) {
	b := coldElideFifo.of(s)
	if len(b) < 8 || s.npend <= before {
		return
	}
	sum := 0
	for _, g := range b[1 : 1+int(b[0])] {
		sum += int(g)
	}
	if sum != int(before) {
		return // something else is pending ahead of or among them: APNAP as before
	}
	n := s.npend - before
	if b[0] < 7 {
		b[0]++
		b[b[0]] = n
	} else {
		b[7] += n
	}
}

// elideHeadLim is the length of the pending queue's head group when the mirror
// queued it, and the whole queue otherwise.
func (s *State) elideHeadLim() int {
	b := coldElideFifo.of(s)
	if len(b) < 8 || b[0] == 0 {
		return int(s.npend)
	}
	if b[1] > s.npend {
		b[0] = 0 // the queue shrank under the mark (a trigger left it another way)
		return int(s.npend)
	}
	return int(b[1])
}

// elideTaken spends the head group's entry for pending trigger i, as it goes
// to the stack.
func (s *State) elideTaken(i int) {
	b := coldElideFifo.of(s)
	if len(b) < 8 || b[0] == 0 || i >= int(b[1]) {
		return
	}
	if b[1]--; b[1] == 0 {
		copy(b[1:7], b[2:8])
		b[7] = 0
		b[0]--
	}
}

// elideFifoClear drops the marks: nothing mirror-queued is left pending.
func (s *State) elideFifoClear() {
	b := coldElideFifo.of(s)
	for i := range b {
		b[i] = 0
	}
}

// flushPrefixMirrored is runPrefixes' flush of the triggers its prefix
// searches queued. With the plan applied out of band (the recognition mark)
// the prefixes tap nothing: what waits in the pending queue is the mirror's,
// and only the head groups gorge drained per plan activation go to the stack
// here. The triggers a late (post-push) activation queued -- a payment's
// sacrifice, a tap -- wait for the play's APNAP placement, below which
// nothing of them may resolve.
func (s *State) flushPrefixMirrored(k *prep) {
	if s.elidePrefixPend() == 0 {
		return
	}
	s.flushPrefixTriggers(k)
}

// elidePrefixPend is the count of pending triggers the play's prefix places
// before the play: the whole queue, except under the recognition mark, where
// only the mirror-queued head groups are the prefix's (gorge drained them per
// plan activation). What else waits -- a sacrifice's or a tap's triggers a
// post-push activation queued -- is the play's own APNAP placement, above the
// play on gorge's stack, and must not be placed or resolved ahead of it.
func (s *State) elidePrefixPend() uint8 {
	if !s.elideArmed() {
		return s.npend
	}
	b := coldElideFifo.of(s)
	if len(b) < 8 {
		return 0
	}
	n := 0
	for _, g := range b[1 : 1+int(b[0])] {
		n += int(g)
	}
	if n > int(s.npend) {
		return s.npend
	}
	return uint8(n)
}

// ElideArmSet marks (or clears) the recognition mark.
func (s *State) ElideArmSet(v bool) {
	if b := coldElideArm.of(s); len(b) > 0 {
		b[0] = v
	}
}

// elideArmed reads the recognition mark.
func (s *State) elideArmed() bool {
	b := coldElideArm.of(s)
	return len(b) > 0 && b[0]
}

// ManaUse is one elided mana activation or spend the harness mirrors into the
// kernel's state out of band. Gorge applies a play's payment plan's mana
// activations at the play's submit (engine/rules/cast_begin.go
// beginCastWithPayment and the cost payments a submit runs inline); the
// kernel re-derives the same payment at the play's Apply (applyPriority's
// runPrefixes and the cost payment's planner). Between the two, the canonical
// menus are priced from different positions: gorge's, post-activation, and
// the kernel's, from the raw position -- so the kernel over-offers casts
// gorge withholds. The lockstep carries the lead gorge side's uses across
// (dirac/oracle/elided.go) and the kernel applies them here, before its own
// processing of the same answer.
//
// An activation's footprint is its Tap and the mana it produced (N positive,
// gorge's ManaAdd events). The plan's machinery also spends between
// activations -- a face-down land's flip cost, a filter's conversion -- and
// those spends (N negative) are mirrored too: gorge named the exact unit (the
// ManaAdd's counter), so the kernel subtracts that unit directly. The
// kernel's re-derivation, which skips the plan's sub-cost steps once the
// mirrored position prices the play, would otherwise leave the spend unpaid.
// The spend of the final cost payment stays the kernel's own
// (dirac/oracle/elided.go mirrors nothing after the play's push): payFromPool
// prices from the pool the mirrored floats fill.
//
// The activation's riders run through manaTapped's own machinery on the
// kernel side, exactly as the kernel's own activation would run them: the
// counter riders (mana_rider.go) go through the counter replacements, and the
// Taps triggers (TrigTaps watchers, CR 605.3b) queue where gorge's queued
// theirs. A life rider or a life cost of the activation itself (Myr
// Convert's "{T}, Pay 2 life: Add one mana of any color") is the activation's
// LifeChange event, carried in Life and folded with foldLifeChange's
// semantics (life, LifeLostThisTurn/LifeGainedThisTurn), and MirrorManaUse
// fires the TrigLifeLost/TrigLifeGained watchers and the speed loss of a
// payment's life lost, as the kernel's own activation would.
type ManaUse struct {
	Obj  Obj   // the kernel's object id of the source permanent (the canon Ref)
	Seat uint8 // the player whose pool moves
	Col  uint8 // the pool column (W U B R G C)
	N    int32 // the units: positive a float, negative a spend, 0 a tap alone
	// Trimmed is a spend of the final cost payment the harness left to the
	// kernel's own payment (N is 0): the units gorge spent that the kernel's
	// pool still holds until its payment runs. Only the reconcile reads it.
	Trimmed int32
	Tap     bool // tap the source (the activation's cost tap)
	// Ab is 1 plus the index of the mana ability the tap activated on its source
	// (0: unknown or granted): the activation's AbilityCast listeners queue as
	// the kernel's own activation queues them, ahead of the tap.
	Ab uint8
	// Rider marks a life change on a seat other than the activation's own
	// (Grove of the Burnwillows' "each opponent gains 1 life").
	Rider bool
	Life  int32 // the activation's life change: a cost's payment negative, a rider's gain positive
	// Late marks an activation after the play's push (the cost payment's own):
	// gorge drains its trigger queue per activation only for the plan's, so
	// the triggers a late tap queues wait for the play's APNAP placement.
	Late bool
	// Sac sacrifices the source after its float: the activation's own cost
	// (a Treasure's {T}, Sacrifice, a Lotus Petal's {T}, Sacrifice), which
	// gorge's engine pays in the same activation.
	Sac bool
	// Sba marks a sacrifice whose activation asked its colour: gorge's check ran
	// at the answer's boundary, before the next decision.
	Sba bool
	// Cease marks a sacrificed token gorge's state-based check has already
	// ceased by the time the use is drained; one it has not is left for the
	// kernel's own check.
	Cease bool
	// Restr floats the mana restricted, as the source's own restriction made
	// it (Carnelian Orb's Dragon-only {R}, a Powerstone's {C}).
	Restr bool
	// Ctr removes the counters the source's own mana ability costs (a
	// depletion land's "{T}, Remove a depletion counter"), before its tap.
	Ctr bool
	// CtrN is the counters' net change on the source (negative: removed): the
	// amount of a variable cost ("Remove X storage counters"), which the
	// ability's printed cost does not name.
	CtrN int32
	// Others are the other permanents the activation's cost tapped (a
	// Springleaf Drum's "Tap an untapped creature you control"), before the
	// source's own tap: one tapping action, as tapXDo taps them.
	Others  [2]Obj
	NOthers uint8
	// Pain is the damage the source's own mana ability dealt its controller (a
	// painland's rider, mana_pain.go painDamage): dealt as the kernel's own
	// payment of the unit deals it.
	Pain int32
}

// MirrorManaUse applies one elided mana activation or spend to the kernel's
// state out of band. An activation whose source is no longer a battlefield
// permanent (it left between the resolution and the mirror, say) still
// floats: only the tap is dropped, and the float lands plain (no snow or
// typed provenance survives its source).
func (s *State) MirrorManaUse(u ManaUse) {
	if u.Ctr {
		if int(u.Obj) < len(s.objs) {
			if ob := &s.objs[u.Obj]; ob.zone == kop.ZoneBattlefield {
				s.mirrorCtrCost(ob.slot, u.CtrN)
			}
		}
		return
	}
	if u.Pain > 0 {
		if int(u.Obj) < len(s.objs) {
			if ob := &s.objs[u.Obj]; ob.zone == kop.ZoneBattlefield {
				s.painDamage(s.k(), ob.slot, u.Seat, int(u.Pain))
			}
		}
		return
	}
	if u.Sac {
		if int(u.Obj) < len(s.objs) {
			if ob := &s.objs[u.Obj]; ob.zone == kop.ZoneBattlefield {
				sl := ob.slot
				before := s.npend
				s.fireSac(s.k(), sl)
				s.leaveForMana(s.k(), sl)
				if !u.Late {
					s.elideNoteQueued(before) // the plan's activation: its sacrifice triggers drain with it
				}
				if u.Cease && s.g.on && ob.of&ofToken != 0 && ob.zone == kop.ZoneGraveyard {
					// a token gorge has already ceased (its state-based check
					// ran after the sacrifice, as for a cast's payment): the
					// kernel's own payment ceases it at once too (mana.go)
					s.ceaseManaToken(u.Obj)
				}
				if u.Sba {
					// gorge's colour ask for the activation: its answer's boundary ran the
					// state-based check, so the host's attachments let go (mana.go
					// sbaAfterManaSacrifice); a sacrifice without one waits for the next
					s.sba(s.k())
				}
			}
		}
		return
	}
	if u.N < 0 {
		// A plan sub-cost's spend: the unit gorge named, straight off the
		// pool. No snow or typed tally rides it (a spend event consumes the
		// restriction batch on gorge's side; the kernel's floats are plain).
		if u.Restr {
			// a restricted unit: out of the batches, one unit at a time
			for n := -u.N; n > 0 && s.rbSpendCol(u.Seat, 0xff, int(u.Col)); n-- {
			}
			return
		}
		pl := &s.pl[SeatIx(u.Seat)]
		if int(u.Col) < len(pl.pool) {
			n := -u.N
			if int(pl.pool[u.Col]) < int(n) {
				n = int32(pl.pool[u.Col]) // never below zero
			}
			pl.pool[u.Col] -= uint8(n)
		}
		return
	}
	src := false
	var sl Slot
	if int(u.Obj) < len(s.objs) {
		if ob := &s.objs[u.Obj]; ob.zone == kop.ZoneBattlefield {
			if s.hasteRiderSource(ob.slot) {
				// Carnelian Orb of Dragonkind's "if that mana is spent on a
				// Dragon creature spell, it gains haste": the kernel records
				// the rider as the cast's payment taps the source (noteHasteSrc),
				// so the activation is left to the kernel's own payment.
				return
			}
			sl, src = ob.slot, true
			before := s.npend
			if u.Ab != 0 {
				s.fireManaAbilityCast(s.k(), s.cardAt(sl), int8(u.Ab-1), u.Seat)
				s.markOncePerTurn(sl, int(u.Ab-1))
			}
			s.mirrorTapOthers(u)
			if u.Tap && s.perms[sl].flags&pfTapped == 0 {
				// The tap the activation paid. A source already tapped (a
				// merged continuation record of one activation) is not tapped
				// again.
				s.perms[sl].flags |= pfTapped
				s.touchSl(sl)
				if s.buildLive() && s.bld().manaNP == 0 && s.bld().bfl&bflPushed != 0 && s.bld().e.kind == seSpell {
					// the cast's mana step began out of band: what its mana
					// abilities queue from here a reversal keeps (unpushSpell)
					s.bld().manaNP = 1 + before
				}
				s.manaTapped(s.k(), sl)
			}
			if !u.Late {
				s.elideNoteQueued(before) // the activation's cost taps and its own tap: one trigger group
			}
		}
	}
	if u.N != 0 && int(u.Col) < kop.NumManaKinds {
		if ab := s.restrAbFor(u, sl, src); ab != nil {
			// the one colour this use names: an all-at-once restricted ability
			// (floatRestrAb's ManaEach) would float every colour it makes, and
			// its other colours are uses of their own
			k := s.k()
			s.floatRestrV(k, u.Seat, sl, ab.ManaRestrict, k.gen.restrVarOf(ab), int(u.Col), uint8(u.N))
		} else if src {
			// The mana the activation produced, with the source's snow and
			// typed provenance (poolAddFrom), as the kernel's own production
			// would land.
			s.poolAddFrom(s.k(), u.Seat, s.cardAt(sl), int(u.Col), int(u.N))
		} else {
			s.poolAdd(u.Seat, int(u.Col), int(u.N))
		}
	}
	if u.Life != 0 {
		// The activation's own life change (a cost's payment, a rider's
		// gain), folded as gorge's LifeChange event folds it -- the amount is
		// final, no replacement reads it again: life, the lost/gained-this-turn
		// tallies its expressions read, and the listeners loseLife and gainLife
		// queue (the "whenever you lose life" trigger of the paid life).
		k := s.k()
		p := SeatIx(u.Seat)
		pl := &s.pl[p]
		pl.life += int16(u.Life)
		if u.Life > 0 {
			pl.lifeGained += int16(u.Life)
			if s.subs[kop.TrigLifeGained].any() || k.gen != nil && k.gen.embCombatEv&(1<<kop.TrigLifeGained) != 0 {
				s.firePlayer(k, kop.TrigLifeGained, p, int16(u.Life))
			}
		} else {
			pl.lifeLost -= int16(u.Life)
			if s.subs[kop.TrigLifeLost].any() {
				s.firePlayer(k, kop.TrigLifeLost, p, int16(-u.Life))
			}
			if k.speed {
				s.speedLoss(p)
			}
		}
	}
}

// mirrorTapOthers taps the other permanents the activation's cost tapped, as
// tapXDo taps its candidates: one tapping action, a Taps trigger per tap that
// happens, the tap marked.
func (s *State) mirrorTapOthers(u ManaUse) {
	if u.NOthers == 0 {
		return
	}
	k := s.k()
	tb := s.tapBatchOpen(k)
	for _, o := range u.Others[:u.NOthers] {
		if int(o) >= len(s.objs) || s.objs[o].zone != kop.ZoneBattlefield {
			continue
		}
		cd := s.objs[o].slot
		cp := &s.perms[cd]
		was := cp.flags & pfTapped
		cp.flags |= pfTapped
		s.touchSl(cd)
		if was == 0 {
			if s.subs[kop.TrigTaps].any() {
				s.fireTaps(k, cd)
			}
			s.markTapped(k, cd)
		}
	}
	s.tapBatchClose(tb)
}

// restrAbFor is the mana ability of the source whose restriction the use's
// float carries, when the use floats restricted and the source is still a
// battlefield permanent: the table's restricted-source record names it, and
// a table with no such records (the planner prices no restricted source) has
// it as the source's mana ability that names a restriction.
func (s *State) restrAbFor(u ManaUse, sl Slot, src bool) *kop.Ability {
	k := s.k()
	if !u.Restr || !src || k.gen == nil {
		return nil
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	if k.gen.rs != nil {
		if rec := k.gen.restrRecOf(s.cardAt(sl)); rec != nil && rec.ab >= 0 && int(rec.ab) < len(cd.Abilities) {
			return &cd.Abilities[rec.ab]
		}
		return nil
	}
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbMana && a.ManaRestrict != 0 {
			return a
		}
	}
	return nil
}

// hasteRiderSource reports whether the permanent at sl is a mana source whose
// mana carries a spent-on-a-spell rider (mana_spentrider.go's haste), or an
// observed source of a spend trigger that makes a choice of colour (Path of
// Ancestry's "any color in your commander's color identity"): the kernel
// records both as the cast's payment taps the source (noteTapSrc). A
// single-colour observed source (Gilanra, Caller of Wirewood) is mirrored: the
// kernel's payment taps no source for a float gorge left unspent.
func (s *State) hasteRiderSource(sl Slot) bool {
	cd := &s.k().t.Cards[s.cardAt(sl)]
	obs := s.k().cards[s.cardAt(sl)].castSrc&csObserved != 0
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind != kop.AbMana {
			continue
		}
		if a.ManaHasteValid != 0 || obs && a.ManaColors&(a.ManaColors-1) != 0 {
			return true
		}
	}
	return false
}

// mirrorCtrCost pays the counter cost of the mana ability of the source at sl
// (the activation path's, as ctrGapDo and riderGap pay it): the counters a
// depletion land removes, or the counter a Wall of Roots puts on itself.
func (s *State) mirrorCtrCost(sl Slot, delta int32) {
	k := s.k()
	a, ok := s.riderAbilityOf(k, sl)
	if !ok || a.Cost.Flags&kop.CostRemoveCtr == 0 {
		a = k.ctrAbOf(s.cardAt(sl))
	}
	if a == nil {
		return
	}
	if kop.StoreManaAb(a) {
		// a storage battery's announced X (storeGapX): the counters gorge
		// removed, which the printed cost does not name; its tap and mana
		// are mirrored as their own uses
		if delta < 0 {
			s.perms[sl].seeCtr(s)
			s.subCtrFire(k, sl, int(a.Cost.Ctr), int(-delta))
		}
		return
	}
	if a.Cost.Flags&kop.CostRemoveCtr != 0 {
		s.perms[sl].seeCtr(s)
		s.subCtrFire(k, sl, int(a.Cost.Ctr), int(a.Cost.N))
		return
	}
	if a.Cost.Flags&kop.CostOncePerTurn != 0 {
		s.perms[sl].actUsed |= 1 << uint(k.ctrAb[s.cardAt(sl)]) // once each turn (ctrGapDo)
	}
	if a.Cost.Ctr != 0 && a.Cost.N > 0 && kop.CtrManaAb(a) {
		p := s.perms[sl].ctrl
		n := s.placeCounters(k, sl, int(a.Cost.Ctr), int(a.Cost.N), p, ctrPutCost) // a cost: only the non-EffectOnly$ replacements
		if n > 0 && k.anyCtrAdded {
			s.ctrNoteBy(k, sl, int(a.Cost.Ctr), p)
		}
	}
}

// markOncePerTurn marks the activation of the once-each-turn ability ab of the
// permanent at sl (perm.actUsed), as the kernel's own activation marks it.
func (s *State) markOncePerTurn(sl Slot, ab int) {
	cd := &s.k().t.Cards[s.cardAt(sl)]
	if ab < 8 && ab < len(cd.Abilities) && cd.Abilities[ab].Cost.Flags&kop.CostOncePerTurn != 0 {
		s.perms[sl].actUsed |= 1 << uint(ab)
	}
}
