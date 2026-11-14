package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// the small unread parameters of the MageZero sample decks
// (CARD-CENSUS.md, unread-params). Cold and out of line: every op here is
// one only a table naming it reaches, and every ask is reached only from
// such an op.

// Resolution asks of this file (resCtx.ask), far above the modal
// activation's raGenModalMode/raGenModalTarget so a sibling family's
// appended asks cannot collide.
const (
	raGenMayAsk         = raGenManifestDread + 40 // OpMayAsk: do the next instruction? (ChGenConfirm yes/no)
	raGenDiscUnless     = raGenManifestDread + 41 // OpDiscardUnless: discard one matching card instead? (ChGenDiscUnless)
	raGenDiscUnlessPick = raGenManifestDread + 42 // OpDiscardUnless: which matching card (ChDiscard, one pick)
	raGenUntapPick      = raGenManifestDread + 43 // OpUntapChoose: a permanent to untap, or Done (ChGenUntapPick)
)

// mayAsk is OpMayAsk: gorge's "you may" election on one effect (effMill's
// mill_optional, effAttach's attach_optional), a KChoose of yes / no the
// resolving controller answers, or the OptionalDecider$ player a set Obj
// names. The guard mirrors when gorge asks; with it
// failing the next op runs unasked (and does nothing).
func (s *State) mayAsk(k *prep, in *kop.Instr) bool {
	prog := s.prog(k)
	pc := int(s.res.pc) + 1
	if pc >= len(prog) {
		return true
	}
	next := &prog[pc]
	switch in.C {
	case kop.MayGuardMill:
		if s.value(k, next.A) <= 0 {
			return true
		}
	case kop.MayGuardAttach:
		// gorge destCandidatesFor: the Defined$ destination is a battlefield
		// object other than the attaching source (Attachable admits any), and
		// an attaching object that left the battlefield (or returned as a new
		// one) is not attached at all (effAttach's attachObjectGone: Celestial
		// Armor, Goldvein Pick), so no ask is posed -- Cori-Steel Cutter's
		// Flurry destroys the Equipment in response to its own trigger.
		src := s.objRef(k, kop.RefSelf)
		dst := s.objRef(k, next.Obj)
		if src == NoSlot || dst == NoSlot || dst == src {
			return true
		}
	case kop.MayGuardMove:
		if !s.optMoveAsks(k, in, next) { // zone_optionalmoves.go
			return true
		}
	case kop.MayGuardMoveRemembered:
		if !s.optMoveRememberedAsks(k, in, next) { // zone_optionalmoves.go
			return true
		}
	case kop.MayGuardLibraryTop:
		if !s.libTopExists() { // zone_toplibrary.go
			return true
		}
	case kop.MayGuardSuspect:
		if !s.suspectWouldChange(k, next) { // act_suspect.go
			return true
		}
	case kop.MayGuardDraw, kop.MayGuardDigUntil:
		if !s.drawAsks(k, in, next) { // act_optionaldecider.go
			return true
		}
	case kop.MayGuardDamage:
		if !s.damageAsks(k, next) { // act_optionaldecider.go
			return true
		}
	case kop.MayGuardAnimateAll:
		if !s.animateAllAsks(k, next) { // copy_definedvalid.go
			return true
		}
	case kop.MayGuardStackSpell:
		if !s.stackSpellAsks(k, in, next) { // zone_stackexile.go
			return true
		}
	case kop.MayGuardCloneCopy:
		if !s.cloneAsks(k, next) { // copy_clone.go
			return true
		}
	case kop.MayGuardPut:
		// gorge putCounterOptionalDeclined: an Optional$ PutCounter with
		// RememberPut$ True asks only when it would actually put (n > 0) on
		// a live Defined target; a dead source has no slot (objRef), and a
		// declined put skips RememberPut$ too (gorge counters_put.go:73).
		if s.value(k, next.A) <= 0 || s.objRef(k, next.Obj) == NoSlot {
			return true
		}
	}
	askP := s.res.e.ctrl
	if in.Obj != kop.RefNone {
		if askP = s.mayAsker(k, in); askP == 255 { // act_optionaldecider.go
			return true
		}
	}
	s.res.ask, s.res.askP = raGenMayAsk, askP
	return false
}

// DebugNoLookAck drops OpLookAck's ask (the look mutation test proves the
// pool poses it).
var DebugNoLookAck bool
