package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CopyPermanent's TokenAttacking$ True (gorge copyAttackRider): the token
// copy enters attacking. A copy has no planeswalker or battle slot, so the
// candidates are the players alone, and with two or more gorge asks which one
// (CR 508.4: chooseEnteringDefender) once per call, before its mint loop. The
// attack trigger's captured defending player is the fallback and the first
// option; at two seats it is the only opponent, so no ask is posed.

// TokenAttackingStats counts, over a lockstep game, the new TokenAttacking$
// shapes (TestDemoTokenattackingLockstep reads them so a green game is not one
// where no shape was reached). DebugTokenAttackingStats, when set, receives the
// counts; the kernel's hot path reads it behind a nil check.
type TokenAttackingStats struct {
	// Copies: token copies that entered attacking through CopyPermanent's
	// TokenAttacking$ True (CopyAttacking).
	Copies int64
	// EquippedTokens: attacking tokens from an Equipment's "whenever equipped
	// creature attacks" trigger (Sigiled Sword of Valeron).
	EquippedTokens int64
	// Asks: CR 508.4 pre-mint defender asks a copy posed (three or more
	// seats, two or more living opponents).
	Asks int64
	// PickedAsks: the Asks that ExtCopyPicked's copy posed (Calamity,
	// Galloping Inferno's two CopyPermanent calls each pose their own).
	PickedAsks int64
}

// DebugTokenAttackingStats, when set, receives the TokenAttackingStats counts.
var DebugTokenAttackingStats *TokenAttackingStats

// DebugNoCopyAttacking makes a CopyPermanent TokenAttacking$ copy enter
// without attacking (mutation test only).
var DebugNoCopyAttacking bool

// copyAttackingInstr reports whether in carries TokenAttacking$ True. Bit 14
// is CopyAttacking on ExtCopyPicked (which never carries a pump keyword) and
// CopyPumpEOT on OpCopyAsTokenEx beside a PumpKeywords$ keyword (the
// until-end-of-turn grant: God-Pharaoh's Gift, Encore, Gray and Adira). The
// two meanings share the bit, so on OpCopyAsTokenEx the attacking reading
// holds only with no pump keyword; kgen refuses a CopyPermanent that asks for
// both (lower_zone.go).
func copyAttackingInstr(in *kop.Instr) bool {
	cf := uint16(in.B.N)
	if cf&kop.CopyAttacking == 0 {
		return false
	}
	return cf&(0x3f<<kop.CopyPumpShift) == 0
}

// copyDefenderAsk poses CopyAttacking's CR 508.4 defender pick before the
// copy is minted (gorge copyAttackRider freezes one answer per call before its
// mint loop). res.chosen = NoSlot marks it as this pre-mint ask; the answer
// stores g.tokDef and re-runs the instruction (applyAskCzParams). true: the
// ask was posed.
//
//go:noinline
func (s *State) copyDefenderAsk(k *prep, in *kop.Instr, ctrl uint8) bool {
	if !copyAttackingInstr(in) || s.g.tokDefSet {
		return false
	}
	var buf [MaxOptions]TCode
	n := s.copyDefenderCands(ctrl, &buf)
	if !s.duel() && s.enterDefenderFallback(k, ctrl) == NoTarget {
		return false // no bound defender: the mint stalls (enterDefenderFallback)
	}
	if n < 2 {
		return false
	}
	s.res.ask, s.res.askP, s.res.chosen = raGenEnterDefender, ctrl, objWord(uint8(NoSlot))
	if DebugTokenAttackingStats != nil {
		DebugTokenAttackingStats.Asks++
		if copyPickedOp(&s.prog(k)[s.res.pc]) {
			DebugTokenAttackingStats.PickedAsks++
		}
	}
	return true
}

// copyDefenderCands lists a copy's CR 508.4 candidates: every living seat
// other than ctrl, in gorge enteringDefenders order. Players only: gorge's
// copyAttackRider passes objects=false, so a planeswalker or battle is never
// an option for a copy.
func (s *State) copyDefenderCands(ctrl uint8, buf *[MaxOptions]TCode) int {
	n := 0
	for opp := uint8(0); opp < s.nSeats(); opp++ {
		if opp != ctrl && !s.pl[opp].lost {
			buf[n] = PlayerCode(opp)
			if n++; n == MaxOptions {
				break
			}
		}
	}
	return n
}

// copyAskPending reports the pending raGenEnterDefender ask as
// copyDefenderAsk's: the instruction at res.pc carries CopyAttacking and
// res.chosen is the pre-mint NoSlot. The instruction is a CopyAsTokenEx, or
// the ExtCopyPicked op copyPicked turns into one.
func (s *State) copyAskPending(k *prep) bool {
	if s.res.ask != raGenEnterDefender || Slot(s.res.chosen) != NoSlot {
		return false
	}
	in := &s.prog(k)[s.res.pc]
	if !copyAttackingInstr(in) {
		return false
	}
	return in.Op == kop.OpCopyAsTokenEx || copyPickedOp(in)
}

// copyEnterAttacking is a copy token's TokenAttacking$ True: it enters
// attacking the defender gorge's copyAttackRider names. At two seats that is
// the controller's opponent; past two it is the firing attack trigger's
// captured defending player (enteringDefenderFallback), or the answer the
// pre-mint ask froze in g.tokDef. A departed defender means the copy enters
// but does not attack (CR 508.4, EnteringAttackLegal).
func (s *State) copyEnterAttacking(k *prep, sl Slot, ctrl uint8) {
	if DebugNoCopyAttacking {
		return
	}
	atk := PlayerCode(s.nextAlive(SeatIx(ctrl)))
	if s.g.tokDefSet {
		atk = s.g.tokDef
	} else if !s.duel() {
		if fb := s.enterDefenderFallback(k, ctrl); fb != NoTarget {
			atk = fb
		} else {
			return
		}
	}
	if s.out(CodeSeat(atk)) {
		return
	}
	s.setEnterAttacker(k, sl, atk)
	if DebugTokenAttackingStats != nil {
		DebugTokenAttackingStats.Copies++
	}
}

// clearCopyDefender consumes a copy op's pre-mint defender answer once the
// instruction is done, so a later Token/ChangeZone ask cannot read it.
func (s *State) clearCopyDefender(in *kop.Instr) {
	if copyAttackingInstr(in) && (in.Op == kop.OpCopyAsTokenEx || copyPickedOp(in)) {
		s.g.tokDefSet = false
	}
}

// copyPickedOp reports whether in is ExtCopyPicked, the choose-then-copy op
// whose mint is a CopyAsTokenEx (copyPicked).
func copyPickedOp(in *kop.Instr) bool {
	return in.Op == kop.OpExt && kop.ExtOp(in.A.N) == kop.ExtCopyPicked
}

// equippedAttackerTrigger reports whether the resolving trigger's source is
// an attached Equipment: the "whenever equipped creature attacks" shape whose
// TokenAttacking$ True is lowered (Sigiled Sword of Valeron).
func (s *State) equippedAttackerTrigger(k *prep) bool {
	if !s.res.active || s.res.e.kind != seTrigger {
		return false
	}
	o := s.res.e.obj
	if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield {
		return false
	}
	sl := s.objs[o].slot
	return s.typesGen(k, sl)&kop.TEquipment != 0 && s.perms[sl].attached != NoSlot
}
