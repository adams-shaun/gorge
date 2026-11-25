package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// returnDefenderAsk poses a battlefield return's CR 508.4 defender pick
// (Attacking$ True on a ChangeZone: gorge classifyAttackingEntryText runs
// once per call, before its mover loop) with the card still in its zone: a
// graveyard card is asked about while it is in the graveyard. With two or more
// defenders (enteringDefenderCands) it suspends on raGenEnterDefender with
// res.chosen = NoSlot, whose answer stores the defender (tokDef) and re-runs
// this instruction; the re-run enters the card attacking that defender
// (enterAttacking). true: the ask was posed. A hand, library or search mover
// asks after its card enters instead (gorge deferAttackingEntry), through
// enterAttacking alone.
//
//go:noinline
func (s *State) returnDefenderAsk(k *prep, in *kop.Instr, host Slot) bool {
	if host != NoSlot || s.g.retAt != 0 || s.g.tokDefSet || kop.Zone(in.C&7) != kop.ZoneBattlefield ||
		in.C>>3&kop.MvAttacking == 0 {
		return false
	}
	o := s.returningObj(in)
	if o == NoObj {
		return false
	}
	switch s.objs[o].zone {
	case kop.ZoneBattlefield, kop.ZoneStack, kop.ZoneNone:
		return false // the move finds nothing
	}
	ctrl := s.objs[o].owner()
	if in.C>>3&kop.MvUnderYourControl != 0 {
		ctrl = s.res.e.ctrl
	}
	if in.B.N&kop.RetUnderOpponent != 0 {
		ctrl = s.retUnderOppSeat(s.res.e.ctrl) // GainControl$ ChosenPlayer (kop.RetUnderOpponent)
	}
	if in.W != kop.RefNone {
		if p := s.playerRef(k, in.W); p != 255 {
			ctrl = p // GainControl$ Player.IsRemembered (kgen's W ref)
		}
	}
	if ctrl != s.active || s.step < kop.StepAttackers || s.step > kop.StepEndCombat {
		return false
	}
	var buf [MaxOptions]TCode
	if s.enteringDefenderCands(k, ctrl, &buf) < 2 {
		return false
	}
	s.res.ask, s.res.askP, s.res.chosen = raGenEnterDefender, ctrl, objWord(uint8(NoSlot))
	return true
}

// retUnderOppSeat is the controller a kop.RetUnderOpponent return enters
// under (GainControl$ ChosenPlayer: Zuko, Conflicted; Sol'Kanar the
// Tainted): the opponent the resolution's mid-resolution ChoosePlayer chose
// (res.chPl, repl_chooseplayer.go), which gorge's chosen player is at any
// seat count; with no such choice, the one opponent after ctrl (a two-seat
// lowering, kgen's one-opponent ChoosePlayer).
func (s *State) retUnderOppSeat(ctrl uint8) uint8 {
	if p := s.resChosenPlayer(); p != 255 && !DebugChosenNextSeat {
		return p
	}
	return s.nextAlive(ctrl)
}

// DebugChosenNextSeat is the lockstep mutation of the pod ChoosePlayer
// readers: a GainControl's new controller and a RetUnderOpponent return read
// the next live seat instead of the chosen one, the two-seat reading
// (TestFourSeatChoosePlayerControlLockstep must then diverge).
var DebugChosenNextSeat bool
