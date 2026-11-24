package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Ext PreventNext (CR 615.7; gorge effects/prevent_damage.go): "prevent the
// next N damage that would be dealt to <one recipient> this turn" (Healing
// Salve, Combat Medic, Battlefield Medic, Alabaster Potion and the rest of the
// corpus's single-recipient shields). gorge registers one this-turn DamageDone
// shield per recipient whose pool is N: each damage event to the recipient
// has min(pool, amount) prevented, the pool is spent by that, and a spent
// shield is dropped. They are records of genState.prevCol beside the colour and
// scope records, a record with the prevNextSc bit: obj is the effect's source
// (the option an order ask names), col the damage left, ctl the recipient's
// damage code (no other arm reads a record with no PrevScopePlayer /
// PrevScopeCreatures bit but prevColHit's colour loop, which skips it). A spent
// record keeps its slot (col 0) to the turn change, which clears them all
// with the others. Three records a turn of any kind; a fourth stalls
// StallPrevCol, a capacity limit.
//
// The recipient is keyed as gorge keys it, by object (a creature that left and
// came back keeps its shield in gorge: ObjID membership) or player. The shield
// applies after protection (CR 702.16e prevents first, gorge emit) and not at
// all while damage can't be prevented (CR 615.12: a skipped candidate, no
// depletion). Anything else that would be a candidate for the same event (a
// second shield on the recipient, a colour or scope prevention record, a
// SHIELD counter, a printed or granted prevention, a doubler, a plus-N, a
// prevent-and-mill or self-damage replacement on the battlefield, a damage
// mark) makes gorge ask the CR 616.1 order, and the order decides how much each
// prevents and spends: the kernel stalls StallPrevNext there rather than
// answer without that ask. Two or more shields on one recipient are the
// exception: only they compete, so the kernel poses gorge's ask (the options
// name each shield's source, in registration order), applies each pick as made
// and asks again while two or more are left and damage is (a noncombat event
// parked by damageTo, a combat event of an ordered pass; every other damage site
// stalls). The recipient's own bare printed "Prevent all damage that would be
// dealt to CARDNAME." is the other exception (parkNC): the ask's options name
// the shields first (gorge scans its effect records before the printed lines)
// and the printed line last, a pick of the printed line stops the event with
// the shields unspent (gorge handleDamageReplacementChoice's terminal arm)
// and a pick of a shield prevents and spends it, re-collecting the rest as
// gorge does (prevNextPick). The combat events of a pass are dealt in gorge's assignment order
// (replOrderOrder) so a shield depletes across them as gorge's does; a pass that
// asks about another event deals its lone-shield events at deal time
// (dealCombatEvent), never from the recomputed amounts.
//
// An unbounded shield (instruction flag C&2, col prevNextAll = 255: "prevent all
// damage that would be dealt to target creature this turn", Shielded Passage,
// Godtoucher) prevents everything of every event to its recipient and is never
// spent. It ends when the recipient leaves the battlefield (prevAllLeft: gorge's
// ExileOnMoved$ Battlefield, and a creature that comes back is a new object) or
// at the turn change. It never competes: beside a second spending shield or
// anything prevNextCompetes reports, the kernel stalls StallPrevNext (a bounded
// pool stalls at 255 too, so no real pool can look like the sentinel). Two or
// more unbounded shields with no rider alone on one recipient (Wellgabber
// Apothecary activated twice), or a bare unbounded shield beside another bare
// full prevention (a riderless colour or scope record, or the recipient's
// printed/granted "prevent all" fact, as Guard Gomazoa beside Indestructible
// Aura), are bare full preventions: whichever applies first prevents the event
// whole and none is spent, so gorge asks no order and the first applies
// (pnxBare, barePrevOnly, prevNextCompetes' bare flag). A matching colour or
// scope record with a rider body is not bare: gorge asks the order and the
// rider can gain life or mint, so the kernel stalls StallPrevNext instead of
// applying the shield (prevColRiderHit).
//
// Ria Ivor's form adds riders to the unbounded shield: kop.PrevNextPlayers
// matches a player recipient only, kop.PrevNextEndCombat ends the record with
// the combat (prevEndCombat, repl_prevx.go), and kop.PrevNextTokens makes the
// record's body mint the instruction's token (its pool index in B) as many as
// the damage the event dealt. The token rider applies only through
// prevNextTake — a candidate of an order ask stalls StallPrevRider — and
// beside a token doubler it stalls StallPrevRider too (repl_tokorder.go).

// StallPrevNext is a damage event a depleting prevent-the-next-N shield and
// another damage replacement both match (gorge asks the CR 616.1 order, and
// it changes the amount prevented and what is left), or an amount of 255 or more
// (a record's col byte holds the pool).
const StallPrevNext = 171

func init() { stallNames[StallPrevNext] = "prevnext" }

// prevNextSc marks a genState.prevCol record as a depleting shield: obj its
// source, col the damage left (0: spent), ctl the recipient's damage code.
const prevNextSc uint8 = 1 << 7

// prevNextAll is the col of an unbounded shield: all the damage, never spent.
const prevNextAll = 255

// prevNextCmb marks a depleting shield that matches combat damage only.
const prevNextCmb uint8 = 1 << 6

// DebugNoPrevNext (the prevent-the-next-N lockstep mutation): the shields are
// installed but prevent nothing.
var DebugNoPrevNext bool

// prevNextAdd is Ext PreventNext: it installs the shield on the recipient
// code Obj names, with the amount B read once. A shield whose instruction
// carries kop.PrevNextFrom (Awe Strike) is keyed on the damage source
// instead: Obj names the remembered target creature, the record's ctl is its
// code and the rider byte genState.prevX carries pxFrom (and pxRidGain,
// kop.PrevNextGain) beside the controller's seat.
func (s *State) prevNextAdd(k *prep, in *kop.Instr) {
	if in.C&kop.PrevNextChosen != 0 { // seat-ok: an instruction flag bit
		s.prevChosenAdd(k, in) // repl_prevchosen.go
		return
	}
	all := in.C&kop.PrevNextAll != 0 // seat-ok: an instruction flag bit
	from := in.C&kop.PrevNextFrom != 0
	if from && !all || in.C&(kop.PrevNextTokens|kop.PrevNextPlayers|kop.PrevNextEndCombat) != 0 && !from {
		s.capFull(StallLowering) // loud: a source-keyed shield has no amount to spend, and the riders are its shapes
		return
	}
	if in.C&kop.PrevNextTokens != 0 && !s.scopeInstrFindable() {
		// the rider mints the token the instruction names, found again from the
		// record's object (repl_prevscopefilter.go): a copy resolves from a card
		// the record can never read it from
		s.capFull(StallPrevScopeCopy)
		return
	}
	n := prevNextAll
	if !all {
		if n = s.value(k, in.B); n <= 0 {
			return // a shield that prevents nothing is no decision (gorge: total <= 0)
		}
	}
	rider := uint8(0)
	switch {
	case in.C&kop.PrevNextRiderCounter != 0:
		rider = pxRidCtr
	case in.C&kop.PrevNextRiderDelayedCounter != 0:
		rider = pxRidDelayCtr
	case in.C&kop.PrevNextRiderDamage != 0:
		rider = pxRidDmg
	}
	var rt TCode = NoTarget
	gate := rider == pxRidDelayCtr && in.W == kop.RefSelf
	if rider != 0 && in.W != 0 && !gate {
		rt = s.refCode(k, in.W)
	}
	switch {
	case in.C&kop.PrevNextDivided != 0:
		s.prevNextDivided(k, in, rider, rt, gate)
	case in.C&kop.PrevNextMultiObj != 0:
		s.prevNextEachObj(k, in, n, all, from, rider, rt, gate)
	case in.C&kop.PrevNextMultiPlayer != 0:
		s.prevNextEachPlayer(k, in, n, all, from, rider, rt, gate)
	case in.C&kop.PrevNextRadiance != 0:
		s.prevNextRadiance(k, in, n, all, from, rider, rt, gate)
	default:
		s.prevNextInstall(k, in, s.refCode(k, in.Obj), n, all, from, rider, rt, gate)
	}
}

// prevNextInstall stores one shield record for recipient code, or stalls
// StallPrevCol when the three records are taken. A player no longer in the
// game and an object no longer on the battlefield take no shield (gorge's
// recipient walk).
func (s *State) prevNextInstall(k *prep, in *kop.Instr, code TCode, n int, all, from bool, rider uint8, rt TCode, gate bool) {
	if code == NoTarget {
		return
	}
	if isPlayerOrNone(code) {
		if s.out(CodeSeat(code)) {
			return // gorge: a player no longer in the game takes no shield
		}
	} else if int(code) >= MaxObj || s.objs[code].zone != kop.ZoneBattlefield {
		return // gorge: an object no longer on the battlefield takes none
	}
	if n >= prevNextAll && !all {
		s.capFull(StallPrevNext) // loud: never truncate the pool
		return
	}
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.col == 0 && r.sc == 0 {
			r.obj, r.col, r.sc, r.ctl = s.res.e.obj, kop.ColorMask(n), prevNextSc, code
			s.g.prevX[i] = rider
			if rr := s.prevRidAt(i); rr != nil {
				rr.t, rr.n, rr.gate = rt, 0, gate
			}
			if in.C&kop.PrevNextCombat != 0 { // seat-ok: an instruction flag bit
				r.sc |= prevNextCmb
			}
			if from { // seat-ok: the resolving controller is the shield's controller
				s.g.prevX[i] = pxFrom | s.res.e.ctrl<<pxSeatShift
				if in.C&kop.PrevNextGain != 0 {
					s.g.prevX[i] |= pxRidGain
				}
				if in.C&kop.PrevNextTokens != 0 {
					s.g.prevX[i] |= pxRidTok // the rider mints the instruction's token
				}
				if in.C&kop.PrevNextEndCombat != 0 {
					s.g.prevX[i] |= pxEOC // repl_prevx.go: the record ends with the combat
				}
				if in.C&kop.PrevNextPlayers != 0 {
					s.g.prevX[i] |= pxPlayers // the shield matches a player recipient only
				}
			} else if rider == pxRidDmg {
				// the retribution damage's source is the shield's source, still
				// controlled by the resolving controller at damage time
				s.g.prevX[i] |= s.res.e.ctrl << pxSeatShift
			}
			return
		}
	}
	s.capFull(StallPrevCol) // loud: never drop an effect
}

// prevNextEachObj installs a shield on every battlefield permanent matching
// the instruction's filter F (Defined$ Valid Creature..., Kitsune Palliator's
// creature half, Samite Censer-Bearer).
func (s *State) prevNextEachObj(k *prep, in *kop.Instr, n int, all, from bool, rider uint8, rt TCode, gate bool) {
	var ord [MaxPerm]Slot
	cnt := s.sweepHits(k, in.F, &ord) // gorge's seat-0-first battlefield order
	for _, sl := range ord[:cnt] {
		s.prevNextInstall(k, in, TCode(s.perms[sl].obj), n, all, from, rider, rt, gate)
		if s.g.stalled != 0 {
			return
		}
	}
}

// prevNextEachPlayer installs a shield on every living player (Kitsune
// Palliator's player half): gorge's "each player" set from the controller.
func (s *State) prevNextEachPlayer(k *prep, in *kop.Instr, n int, all, from bool, rider uint8, rt TCode, gate bool) {
	q := s.aliveFrom(s.res.e.ctrl)
	for i := 0; i < int(q.n); i++ {
		s.prevNextInstall(k, in, PlayerCode(q.s[i]), n, all, from, rider, rt, gate)
		if s.g.stalled != 0 {
			return
		}
	}
}

// prevNextRadiance installs a shield on the target creature and on each other
// creature on the battlefield sharing a colour with it (Radiance, CR 702.6).
func (s *State) prevNextRadiance(k *prep, in *kop.Instr, n int, all, from bool, rider uint8, rt TCode, gate bool) {
	code := s.refCode(k, in.Obj)
	if code == NoTarget || isPlayerOrNone(code) || int(code) >= MaxObj || s.objs[code].zone != kop.ZoneBattlefield {
		return
	}
	sl := s.objs[code].slot
	col := s.colorsGen(k, sl)
	s.prevNextInstall(k, in, code, n, all, from, rider, rt, gate)
	if s.g.stalled != 0 {
		return
	}
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		b := m.first()
		if b != sl && s.isCreature(k, b) && s.colorsGen(k, b)&col != 0 {
			hit = hit.with(b)
		}
	}
	var ord [MaxPerm]Slot
	m := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		m = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), ord[:], m)
	}
	for _, b := range ord[:m] {
		s.prevNextInstall(k, in, TCode(s.perms[b].obj), n, all, from, rider, rt, gate)
		if s.g.stalled != 0 {
			return
		}
	}
}

// prevNextDivided installs one shield per still-legal target of the divided
// slot W, with the share announced for it (DividedAsYouChoose$), or all of
// the total on one target. A division the kernel has to ask at resolution
// stalls StallPrevNext.
func (s *State) prevNextDivided(k *prep, in *kop.Instr, rider uint8, rt TCode, gate bool) {
	e := &s.res.e
	slot := tgtSlotOf(in.W)
	if slot < 0 {
		s.capFull(StallLowering)
		return
	}
	var codes [maxXTgt + 1]TCode
	var amts [maxXTgt + 1]int
	m := 0
	x := s.xtOf(e)
	if e.xi != 0 && x.div[0] != 0 {
		// the announced division: each still-legal target takes its share
		// (CR 608.2b), an illegal one's is lost
		i := 0
		if e.tgt[slot] != NoTarget {
			if c := s.tgtCode(slot); c != NoTarget {
				codes[m], amts[m] = c, int(x.div[0])
				m++
			}
			i = 1
		}
		for j := 0; j < int(x.n); j++ {
			if x.slot[j] != uint8(slot) {
				continue
			}
			if s.xtLegal(k, e, j) {
				codes[m], amts[m] = x.code[j], int(x.div[i])
				m++
			}
			i++
		}
	} else {
		m = s.divTargets(k, e, uint8(slot), &codes)
		switch m {
		case 0:
			return
		case 1:
			amts[0] = s.divTotal(k, e, uint8(slot))
		default:
			s.capFull(StallPrevNext) // gorge divides at resolution and asks; the kernel has no ask here
			return
		}
	}
	for i := 0; i < m; i++ {
		if amts[i] <= 0 {
			continue
		}
		s.prevNextInstall(k, in, codes[i], amts[i], false, false, rider, rt, gate)
		if s.g.stalled != 0 {
			return
		}
	}
}

// pnxBare reports whether record i is an unbounded shield with no rider and no
// source key: gorge's bare "prevent all damage to it" Effect line (Wellgabber
// Apothecary, Shielded Passage). It prevents the whole event and is never
// spent, so beside another bare prevention the order is unobservable and gorge
// poses no CR 616.1 ask (barePrevOnly).
func (s *State) pnxBare(i int) bool {
	r := &s.g.prevCol[i]
	return r.sc&prevNextSc != 0 && r.col == prevNextAll && s.g.prevX[i]&(pxFrom|pxRider) == 0
}

// pnxBareMask is the bits of pnx (the candidates that are shield records, whose
// cand entry holds the record's index) that are bare unbounded shields.
func (s *State) pnxBareMask(cand *[4]Obj, pnx uint8) uint8 {
	var b uint8
	for j := range cand {
		if pnx&(1<<j) != 0 && s.pnxBare(int(cand[j])) {
			b |= 1 << j
		}
	}
	return b
}

// pnxLive reports whether any depleting shield has damage left.
func (s *State) pnxLive() bool {
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc&prevNextSc != 0 && r.col != 0 {
			return true
		}
	}
	return false
}

// pnxMatches reports whether the depleting shield at record i matches a
// damage event to the recipient code from the permanent at srcSl (NoSlot: a
// player or a source with no permanent). A shield without kop.PrevNextFrom
// matches by its recipient; one with it matches only by its source — the
// record's ctl is the source's code and the recipient is never read, so
// damage dealt TO the Awe-Struck creature is dealt. A shield with
// kop.PrevNextPlayers (Ria Ivor's ValidTarget$ Player) matches a player
// recipient only: damage to a blocker or a planeswalker is dealt and the
// record waits.
func (s *State) pnxMatches(i int, code TCode, srcSl Slot, combat bool) bool {
	r := &s.g.prevCol[i]
	if r.sc&prevNextSc == 0 || r.col == 0 || !combat && r.sc&prevNextCmb != 0 {
		return false
	}
	if pnxChosenForm(s.g.prevX[i]) {
		return s.pnxChosenMatches(i, code, srcSl) // repl_prevchosen.go
	}
	if s.g.prevX[i]&pxPlayers != 0 && !isPlayerTarget(code) {
		return false
	}
	if s.g.prevX[i]&pxFrom == 0 {
		return r.ctl == code
	}
	return srcSl != NoSlot && TCode(s.perms[srcSl].obj) == r.ctl
}

// prevNextTake applies the shield on recipient code (controlled by rcp; the
// permanent at sl, NoSlot for a player) to a damage event of n from the source
// at srcSl (card srcCard when it is no permanent) controlled by srcCtrl. A
// shield keyed on the source (kop.PrevNextFrom, Awe Strike) matches by srcSl
// instead (pnxMatches). It reports the damage left and whether a shield
// applied; false leaves the event to the other prevention and replacement
// arms.
func (s *State) prevNextTake(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot, combat bool, n int) (int, bool) {
	hit, bare, at, rider, tokRider := 0, 0, -1, false, false
	for i := range s.g.prevCol {
		if s.pnxMatches(i, code, srcSl, combat) {
			hit++
			at = i
			if s.pnxBare(i) {
				bare++
			}
			switch s.g.prevX[i] & pxRider {
			case pxRidGain:
				rider = s.g.prevX[i]&pxFrom != 0 // a gain rider: gorge runs the body (repl_prevx.go)
			case pxRidTok:
				rider, tokRider = true, true // a token rider: the record's body mints tokens
			case pxRidCtr, pxRidDelayCtr, pxRidDmg:
				rider = true // a counter, delayed-counter or retribution body
			}
		}
	}
	if hit == 0 || n <= 0 || DebugNoPrevNext {
		return n, false
	}
	g := k.gen
	if g.anyRedir && s.redirBeside(code, rcp) {
		return n, false // a shield and a redirect on one recipient: gorge asks the order (repl_dmgredirect.go)
	}
	if s.protBeforeRepl(k, srcCtrl, srcSl, srcCard, rcp, sl, combat) {
		return n, false // prevented before the replacements: nothing is spent
	}
	if s.noPrev(k, combat) {
		if rider {
			s.capFull(StallPrevRider) // loud: gorge runs the rider and keeps the damage
		}
		return n, false // CR 615.12: the shield is no candidate
	}
	if rider && (s.g.ro.on || s.parkedAny()) {
		s.capFull(StallPrevRider) // loud: the rider would be run where the kernel cannot
		return n, false
	}
	if tokRider && k.gen.anyRepl&rbDblTok != 0 {
		// the mint would run the token replacements mid-pass, and gorge's token
		// path there cannot be posed a question (repl_tokorder.go)
		s.capFull(StallPrevRider)
		return n, false
	}
	if hit > 1 && bare < hit || s.prevNextCompetes(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, hit == bare) {
		if rider {
			s.capFull(StallPrevRider) // a body is never a bare prevention: gorge asks the order
		} else {
			s.capFull(StallPrevNext) // two or more shields on one recipient: only damageTo and a combat pass pose the ask
		}
		return n, false
	}
	r := &s.g.prevCol[at]
	if s.g.prevX[at]&pxFrom != 0 {
		// a source-keyed shield: the first event from its source is prevented
		// whole and the record is spent (the Effect exiles itself)
		m := n
		r.col = 0
		x := s.g.prevX[at]
		if x&pxRidGain != 0 && !DebugNoPrevRider {
			s.gainLife(k, x>>pxSeatShift, m)
		} else if x&pxRidTok != 0 && !DebugNoPrevRider {
			in := s.prevTokInstr(k, at)
			if in == nil {
				// the record's object no longer holds the card whose program made
				// the record (a reused object): the token is unreadable
				s.capFull(StallPrevScopeCopy)
				return n, false
			}
			s.mintTokens(k, &kop.Instr{Op: kop.OpCreateToken, C: int16(in.B.N)}, x>>pxSeatShift, m)
		}
		return n - m, true
	}
	if r.col == prevNextAll {
		return 0, true // unbounded: everything, never spent
	}
	m := min(n, int(r.col))
	r.col -= kop.ColorMask(m)
	s.prevNextRiderApply(k, at, m)
	return n - m, true
}

// prevNextRiderApply runs the rider of the recipient-keyed shield record i
// on the amount m this application prevented (a source-keyed record's gain
// and token riders run in prevNextTake's own arm): +1/+1 counters, the
// delayed-counter accumulation, or the retribution damage. The record's
// target is the parent ability's target when the instruction named one
// (ShieldEffectTarget$ ParentTarget), else the shield's recipient.
func (s *State) prevNextRiderApply(k *prep, i, m int) {
	if m <= 0 || DebugNoPrevRider {
		return
	}
	switch s.g.prevX[i] & pxRider {
	case pxRidCtr:
		if sl, ok := s.prevRiderSlot(i); ok {
			s.putDividedShare(k, sl, kop.CtrP1P1, m)
		}
	case pxRidDelayCtr:
		if r := s.prevRidAt(i); r != nil {
			n := int(r.n) + m
			if n > 255 {
				n = 255
			}
			r.n = uint8(n)
		}
	case pxRidDmg:
		s.prevRiderDamage(k, i, m)
	}
}

// prevRiderSlot is the battlefield permanent the record's rider acts on:
// the stored parent target when there is one, else the shield's recipient.
func (s *State) prevRiderSlot(i int) (Slot, bool) {
	code := s.g.prevCol[i].ctl
	if r := s.prevRidAt(i); r != nil && r.t != NoTarget {
		code = r.t
	}
	if code == NoTarget {
		code = s.g.prevCol[i].ctl
	}
	if isPlayerOrNone(code) || int(code) >= MaxObj || s.objs[code].zone != kop.ZoneBattlefield {
		return NoSlot, false
	}
	return s.objs[code].slot, true
}

// prevRiderDamage deals the prevented amount as the shield's source to the
// rider's target (Acolyte's Reward, Vengeful Archon): the source is the
// effect's source, its controller the seat stored at installation. A nested
// damage replacement that has to ask stalls StallPrevRider rather than
// answering inside the outer event.
func (s *State) prevRiderDamage(k *prep, i, m int) {
	tgt := s.g.prevCol[i].ctl
	if r := s.prevRidAt(i); r != nil && r.t != NoTarget {
		tgt = r.t
	}
	if tgt == NoTarget {
		return
	}
	src := s.g.prevCol[i].obj
	saved := s.res.e
	if src != NoObj && int(src) < MaxObj {
		s.res.e.obj = src
		s.res.e.card = s.objs[src].card
		s.res.e.ctrl = s.g.prevX[i] >> pxSeatShift
		if s.objs[src].zone == kop.ZoneBattlefield {
			s.res.e.kind = seAbility
		} else {
			s.res.e.kind = seSpell
		}
	}
	nc := s.g.nc.n
	s.damageTo(k, tgt, m)
	s.res.e = saved
	if s.g.nc.n != nc {
		s.capFull(StallPrevRider)
	}
}

// prevTokInstr is the ExtPreventNext instruction with kop.PrevNextTokens in
// the token-rider record i's source card (as scopeInstr finds the filtered
// scope's, repl_prevscopefilter.go): its B.N is the token the rider mints.
// nil when the record's object no longer holds that card (a reused object),
// which prevNextTake stalls on.
func (s *State) prevTokInstr(k *prep, i int) *kop.Instr {
	r := &s.g.prevCol[i]
	if r.obj == NoObj || int(r.obj) >= MaxObj {
		return nil
	}
	ab := k.t.Cards[s.objs[r.obj].card].Abilities
	for j := range ab {
		for l := range ab[j].Prog {
			if in := &ab[j].Prog[l]; in.Ext() == kop.ExtPreventNext && in.C&kop.PrevNextTokens != 0 {
				return in
			}
		}
	}
	return nil
}

// combatEvLess takes d damage out of combat event ev of r (a partial
// prevention): the source's dealt totals and the recipient's assigned damage
// lose it, the event stays.
func (s *State) combatEvLess(r *dmgRec, ev *dmgEv, code TCode, d int) {
	ev.n = s.sat16(int(ev.n) - d)
	r.dealt[ev.src] = s.addSat16(r.dealt[ev.src], -d)
	if ev.dst == NoSlot {
		r.toPl[CodeSeat(code)] = s.addSat16(r.toPl[CodeSeat(code)], -d)
		r.dealtPl[ev.src] = s.addSat16(r.dealtPl[ev.src], -d)
	} else {
		r.toPerm[ev.dst] = s.addSat16(r.toPerm[ev.dst], -d)
	}
}

// prevNextCompetes reports whether another damage replacement would also be a
// candidate for this event (a conservative superset of gorge's collection:
// the battlefield scan does not read the other replacements' own filters; a
// doubler is read through dblDmgN2, as the SHIELD counter's shieldOther reads
// it: only the source controller's doublers that match the event compete, so
// an opponent's Fire Servant is no candidate for this spell). bare says every
// matching shield is a bare unbounded one: then the other bare full
// preventions (prevColHit, prevAnyTo, prevCombatTo, prevCombatFrom) do not
// compete, because every order prevents the event whole and none is spent
// (allBarePreventions); a matching colour or scope record with a rider body is
// not a bare prevention and still competes (prevColRiderHit), and a spending
// replacement still does.
func (s *State) prevNextCompetes(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, bare bool) bool {
	return !bare && s.prevColHit(k, srcSl, srcCard, prevTgt{rcp, sl}, combat) || // a colour or scope record
		bare && s.prevColRiderHit(k, srcSl, srcCard, prevTgt{rcp, sl}, combat) || // a rider body is never bare
		s.replOthersCompete(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, bare)
}

// replOthersCompete is prevNextCompetes without the colour and scope records:
// every other replacement that would also be a candidate for the event.
func (s *State) replOthersCompete(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool, bare bool) bool {
	g := k.gen
	if g.anyDmgMark && s.dmgMarkEv(rcp, srcCtrl) > 0 {
		return true
	}
	if k.embPrev && s.embPrevHit(k, &spEv{srcCtrl, srcSl, srcCard, rcp, sl, combat, 0}) {
		return true // a live emblem's static prevention (repl_staticprevent_emblem.go)
	}
	if sl != NoSlot {
		if g.anyRepl&rbShield != 0 && s.shieldN(sl) > 0 && !s.noPrev(k, combat) {
			return true
		}
		if !bare && (s.prevAnyTo(k, sl, combat) || combat && s.prevCombatTo(k, sl)) {
			return true
		}
	}
	if combat && !bare && g.anyPrevFrom && srcSl != NoSlot && s.prevCombatFrom(k, srcSl) {
		return true
	}
	if g.anyRepl&rbDblDmg != 0 && s.dblDmgN2(k, srcCtrl, srcSl, srcCard, rcp, sl, combat, nil) > 0 {
		return true
	}
	const others = rbPlusDmg | rbPrevNC | rbPrevMill | rbSelfDmg
	if g.anyRepl&others != 0 {
		for m := s.statM; m.any(); m = m.rest() {
			b := m.first()
			rb := g.replBits[s.cardAt(b)] & others
			if rb != 0 && !s.genLost(k, b) && !(rb == rbPlusDmg && s.plusHostIdle(k, b)) {
				return true
			}
		}
	}
	return false
}

// prevNextCands appends the depleting shields matching a damage event to the
// recipient code from the permanent at srcSl, in registration order, to the
// candidates after its first c (cand holds the record's index, the option
// names its source), marking them in *pm; the new count, or -1 (a loud stall)
// when another replacement is also a candidate or the candidates overflow.
// prevMask marks the c candidates collected so far that prevent the event
// whole: beside only those the order changes nothing but the shield's spend,
// which the walk applies as picked, so the ask is posed; beside any other
// candidate the amounts change and it stalls. With no matching shield, or
// under CR 615.12, it adds nothing.
func (s *State) prevNextCands(k *prep, code TCode, srcSl Slot, combat bool, cand *[4]Obj, c int, pm *uint8, prevMask uint8) int {
	if s.noPrev(k, combat) {
		return c
	}
	start := c
	rider := false
	for i := range s.g.prevCol {
		if s.pnxMatches(i, code, srcSl, combat) {
			if c == len(cand) {
				s.capFull(StallReplCands) // loud: never truncate the candidates
				return -1
			}
			cand[c] = Obj(i)
			*pm |= 1 << c
			c++
			if s.g.prevX[i]&pxRider != 0 {
				rider = true // a rider: the order's walk cannot run the body
			}
		}
	}
	if rider && c >= 2 {
		s.capFull(StallPrevRider) // loud: a body is never a bare prevention, gorge asks the order
		return -1
	}
	if c-start > 1 && s.pnxBareMask(cand, uint8((1<<c-1)&^(1<<start-1))) != uint8((1<<c-1)&^(1<<start-1)) {
		for j := start; j < c; j++ {
			if s.g.prevCol[cand[j]].col == prevNextAll {
				s.capFull(StallPrevNext) // an unbounded shield and a depleting one on one recipient: no order ask
				return -1
			}
		}
	}
	if c > start && start > 0 && uint8((1<<start)-1)&^prevMask != 0 {
		s.capFull(StallPrevNext) // beside a non-prevention candidate the order changes the amounts
		return -1
	}
	return c
}

// prevNextSpend applies shield candidate i of the ask in progress to the event's
// remaining damage (ro.an): it prevents min(left, remaining) and is spent by it.
func (s *State) prevNextSpend(i int) {
	ro := &s.g.ro
	r := &s.g.prevCol[ro.cand[i]]
	if s.g.prevX[ro.cand[i]]&pxFrom != 0 {
		r.col = 0 // a source-keyed shield: one event, spent
		ro.an = 0
		return
	}
	if r.col == prevNextAll {
		ro.an = 0 // unbounded: everything, never spent
		return
	}
	m := min(int(r.col), int(ro.an))
	r.col -= kop.ColorMask(m)
	ro.an -= int16(m)
}

// prevAllLeft ends the unbounded shields on o (spent, col 0) now that it has
// left the battlefield: gorge's Effect is exiled by ExileOnMoved$ Battlefield.
//
//go:noinline
func (s *State) prevAllLeft(o Obj) {
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc&prevNextSc != 0 && r.col == prevNextAll && r.ctl == TCode(o) {
			r.col = 0
		}
	}
}

// prevNextPick is replOrderPick for a shield candidate: it applies as picked
// (gorge applies each pick and re-collects the rest). The asks end when the
// damage is all prevented; one shield left applies on its own; two or more
// left are asked again. The recipient's bare printed prevention beside the
// shields (parkNC's modeled competition, and the combat walk's, whose
// collection reads the shields before it) is counted too, as gorge's
// re-collected set holds it: a lone remaining printed line applies
// automatically and stops the event with the shields unspent (gorge
// handleDamageReplacementChoice's case 1), so two or more left are asked
// again over it. In combat (ro.on) any remaining candidate is counted only
// while it still matches the event (replCandLive — the printed line, a
// granted record, the SHIELD counter the collection appends after the
// shields; the state-based actions at a pause can have removed one); the
// parked noncombat ask's set is modeled and never drops one, so only its
// printed prevention is counted there.
func (s *State) prevNextPick(i int) bool {
	ro := &s.g.ro
	s.prevNextSpend(i)
	if ro.an <= 0 {
		return false
	}
	left, last := 0, -1
	for j := 0; j < int(ro.n); j++ {
		if ro.used&(1<<j) != 0 {
			continue
		}
		if ro.pnx&(1<<j) != 0 {
			if s.g.prevCol[ro.cand[j]].col != 0 {
				left++
				last = j
			}
			continue
		}
		if ro.on {
			if s.replCandLive(s.k(), j) {
				left++ // the printed line, a record or the counter still matching
				last = j
			}
		} else if ro.prev&(1<<j) != 0 {
			left++ // the printed prevention: a candidate of the re-collected set (parkNC)
			last = j
		}
	}
	if left == 1 {
		ro.used |= 1 << last
		if ro.pnx&(1<<last) != 0 {
			s.prevNextSpend(last)
		} else if !ro.on && s.ncShieldSlot() != NoSlot && s.prevAnyTo(s.k(), s.ncShieldSlot(), false) {
			ro.an = 0 // the printed line applies: the event stops, the shields are unspent
		}
		// In combat the last remaining candidate applies at dealAsked (the
		// printed line in scan order, the counter's own spend logic): the
		// shields stay spent as picked and the event is prevented with ro.an's
		// remainder.
	}
	return left >= 2
}

// prevNextNC is the end of a parked noncombat event's ask: the shields were
// spent as picked, so the damage left is what the event deals (nothing left: the
// ask was posed and the event is prevented, ncDmg.n -1).
func (s *State) prevNextNC() {
	if ro := &s.g.ro; ro.pnx != 0 {
		s.g.nc.n = max(ro.an, -1)
		if s.g.nc.n == 0 {
			s.g.nc.n = -1
		}
	}
}
