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
// stalls). The combat events of a pass are dealt in gorge's assignment order
// (replOrderOrder) so a shield depletes across them as gorge's does; a pass that
// asks about another event deals its lone-shield events at deal time
// (dealCombatEvent), never from the recomputed amounts.

// StallPrevNext is a damage event a depleting prevent-the-next-N shield and
// another damage replacement both match (gorge asks the CR 616.1 order, and
// it changes the amount prevented and what is left), or an amount past 255
// (a record's col byte holds the pool).
const StallPrevNext = 171

func init() { stallNames[StallPrevNext] = "prevnext" }

// prevNextSc marks a genState.prevCol record as a depleting shield: obj its
// source, col the damage left (0: spent), ctl the recipient's damage code.
const prevNextSc uint8 = 1 << 7

// prevNextCmb marks a depleting shield that matches combat damage only.
const prevNextCmb uint8 = 1 << 6

// DebugNoPrevNext (the prevent-the-next-N lockstep mutation): the shields are
// installed but prevent nothing.
var DebugNoPrevNext bool

// prevNextAdd is Ext PreventNext: it installs the shield on the recipient
// code Obj names, with the amount B read once.
func (s *State) prevNextAdd(k *prep, in *kop.Instr) {
	n := s.value(k, in.B)
	if n <= 0 {
		return // a shield that prevents nothing is no decision (gorge: total <= 0)
	}
	code := s.refCode(k, in.Obj)
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
	if n > 255 {
		s.capFull(StallPrevNext) // loud: never truncate the pool
		return
	}
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.col == 0 && r.sc == 0 {
			r.obj, r.col, r.sc, r.ctl = s.res.e.obj, kop.ColorMask(n), prevNextSc, code
			if in.C&1 != 0 { // seat-ok: an instruction flag bit
				r.sc |= prevNextCmb
			}
			return
		}
	}
	s.capFull(StallPrevCol) // loud: never drop an effect
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

// prevNextTake applies the shield on recipient code (controlled by rcp; the
// permanent at sl, NoSlot for a player) to a damage event of n from the source
// at srcSl (card srcCard when it is no permanent) controlled by srcCtrl. It
// reports the damage left and whether a shield applied; false leaves the
// event to the other prevention and replacement arms.
func (s *State) prevNextTake(k *prep, srcCtrl uint8, srcSl Slot, srcCard CardID, code TCode, rcp uint8, sl Slot, combat bool, n int) (int, bool) {
	hit, at := 0, -1
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc&prevNextSc != 0 && r.col != 0 && r.ctl == code && (combat || r.sc&prevNextCmb == 0) {
			hit++
			at = i
		}
	}
	if hit == 0 || n <= 0 || DebugNoPrevNext {
		return n, false
	}
	g := k.gen
	if sl != NoSlot && (g.anyRepl&rbProtAll != 0 && s.kw(k, sl)&kop.KwProtectionAll != 0 ||
		g.anyRepl&rbProtCol != 0 && s.protFrom(k, sl, srcSl, srcCard, s.srcColors(k, srcSl, srcCard), combat)) ||
		sl == NoSlot && g.anyRepl&rbPlProt != 0 && s.plProtected(k, srcCtrl, rcp, combat) {
		return n, false // prevented before the replacements: nothing is spent
	}
	if s.noPrev(k, combat) {
		return n, false // CR 615.12: the shield is no candidate
	}
	if hit > 1 || s.prevNextCompetes(k, srcSl, srcCard, rcp, sl, combat) {
		s.capFull(StallPrevNext) // two or more shields on one recipient: only damageTo and a combat pass pose the ask
		return n, false
	}
	r := &s.g.prevCol[at]
	m := min(n, int(r.col))
	r.col -= kop.ColorMask(m)
	return n - m, true
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
// the battlefield scan does not read each doubler's own filter).
func (s *State) prevNextCompetes(k *prep, srcSl Slot, srcCard CardID, rcp uint8, sl Slot, combat bool) bool {
	g := k.gen
	if s.prevColHit(k, srcSl, srcCard, prevTgt{rcp, sl}, combat) {
		return true // a colour or scope record
	}
	if g.anyDmgMark && s.dmgMarkN(rcp) > 0 {
		return true
	}
	if sl != NoSlot {
		if g.anyRepl&rbShield != 0 && s.shieldN(sl) > 0 && !s.noPrev(k, combat) {
			return true
		}
		if s.prevAnyTo(k, sl, combat) || combat && s.prevCombatTo(k, sl) {
			return true
		}
	}
	if combat && g.anyPrevFrom && srcSl != NoSlot && s.prevCombatFrom(k, srcSl) {
		return true
	}
	const others = rbDblDmg | rbPlusDmg | rbPrevNC | rbPrevMill | rbPrevCtrSelf | rbHealSelf
	if g.anyRepl&others != 0 {
		for m := s.statM; m.any(); m = m.rest() {
			b := m.first()
			if g.replBits[s.cardAt(b)]&others != 0 && !s.genLost(k, b) {
				return true
			}
		}
	}
	return false
}

// prevNextCands appends the depleting shields on the recipient code, in
// registration order, to the candidates of a damage event after its first c
// (cand holds the record's index, the option names its source), marking them
// in *pm; the new count, or -1 (a loud stall) when another replacement is also
// a candidate or the candidates overflow. With no shield on the recipient, or
// under CR 615.12, it adds nothing.
func (s *State) prevNextCands(k *prep, code TCode, combat bool, cand *[4]Obj, c int, pm *uint8) int {
	if s.noPrev(k, combat) {
		return c
	}
	start := c
	for i := range s.g.prevCol {
		if r := &s.g.prevCol[i]; r.sc&prevNextSc != 0 && r.col != 0 && r.ctl == code && (combat || r.sc&prevNextCmb == 0) {
			if c == len(cand) {
				s.capFull(StallReplCands) // loud: never truncate the candidates
				return -1
			}
			cand[c] = Obj(i)
			*pm |= 1 << c
			c++
		}
	}
	if c > start && start > 0 {
		s.capFull(StallPrevNext) // beside another candidate the order changes the amounts
		return -1
	}
	return c
}

// prevNextSpend applies shield candidate i of the ask in progress to the event's
// remaining damage (ro.an): it prevents min(left, remaining) and is spent by it.
func (s *State) prevNextSpend(i int) {
	ro := &s.g.ro
	r := &s.g.prevCol[ro.cand[i]]
	m := min(int(r.col), int(ro.an))
	r.col -= kop.ColorMask(m)
	ro.an -= int16(m)
}

// prevNextPick is replOrderPick for a shield candidate: it applies as picked
// (gorge applies each pick and re-collects the rest). The asks end when the
// damage is all prevented; one shield left applies on its own; two or more
// left are asked again.
func (s *State) prevNextPick(i int) bool {
	ro := &s.g.ro
	s.prevNextSpend(i)
	if ro.an <= 0 {
		return false
	}
	left, last := 0, -1
	for j := 0; j < int(ro.n); j++ {
		if ro.pnx&(1<<j) != 0 && ro.used&(1<<j) == 0 && s.g.prevCol[ro.cand[j]].col != 0 {
			left++
			last = j
		}
	}
	if left == 1 {
		ro.used |= 1 << last
		s.prevNextSpend(last)
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
