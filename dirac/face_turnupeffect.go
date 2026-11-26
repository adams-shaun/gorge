package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An effect's turn-up: OpExt ExtTurnFaceUp, api:SetState Mode$ TurnFaceUp
// (gorge effSetState's turnUp arm, rules effects/resolve.go). It is not the
// CR 708.6 special action: nothing is paid and no Megamorph counter is put,
// but the permanent turns face up through the same turnUpFlip (its turn-up
// replacements, its face-up characteristics, the TurnFaceUp triggers; gorge
// emits the same events.TurnFaceUp). A Layer$ CantHappen replacement stops
// it (gorge's TurnFaceUp dispatch). C 0 turns up the target in Obj (Expose
// the Culprit); C 1 is gorge setStateChoose's "setstate_pick": the
// resolving controller picks up to one permanent matching F that would turn
// up, or Done (Hustle // Bustle). C 2 (turnUpPickTransform) is the same
// "setstate_pick" for Mode$ Transform with Mandatory$ True: one mandatory pick
// (Min = Max = 1, no Done; posed with one candidate too) over the permanents
// matching F that would transform, and the pick transforms (Sunder the
// Gateway's "transform an Incubator token you control").

// turnUpPickTransform is ExtTurnFaceUp's C for the Transform pick.
const turnUpPickTransform = 2

// raGenTurnUpPick is ExtTurnFaceUp C 1's pick (ChGenTurnUpPick).
const raGenTurnUpPick = raGenManifestDread + 45

// ChGenTurnUpPick is ExtTurnFaceUp's pick (gorge's "setstate_pick" KChoose):
// source = the permanent turned face up (C 2: transformed); AkDone declines.
const ChGenTurnUpPick ChoiceKind = 203

// extTurnFaceUp runs one ExtTurnFaceUp instruction; false: it asks.
func (s *State) extTurnFaceUp(k *prep, in *kop.Instr) bool {
	if in.C == 0 {
		if sl := s.objRef(k, in.Obj); sl != NoSlot && s.turnUpByEffect(k, sl) {
			return false // the attach election parked the resolution (face_turnupattach.go)
		}
		return true
	}
	var buf [MaxPerm]Slot
	if s.turnUpEffCands(k, in, &buf) == 0 {
		return true // gorge: an empty pool asks nothing
	}
	s.res.ask, s.res.askP = raGenTurnUpPick, s.res.e.ctrl
	return false
}

// turnUpEffOK is gorge setStateObjWouldChange's turnUp arm: a face-down
// permanent whose printed face is a permanent's (setStateTurnUpVerdict:
// an instant or sorcery is revealed, not turned up).
func (s *State) turnUpEffOK(k *prep, sl Slot) bool {
	return s.faceDown(k, sl) && !s.blankCopy(sl) &&
		k.t.Cards[s.cardAt(sl)].Types&(kop.TInstant|kop.TSorcery) == 0
}

// turnUpByEffect turns the battlefield permanent at sl face up, if it would.
// It reports whether the turn-up parked on the attach election
// (face_turnupattach.go): the caller must then not continue the program, the
// election's answer does.
func (s *State) turnUpByEffect(k *prep, sl Slot) bool {
	if !s.turnUpEffOK(k, sl) {
		return false
	}
	if k.gen.anyTurnUpRepl && s.turnUpBlocked(k, sl) {
		return false // face_turnfaceup.go: the CantHappen replacement prevents it
	}
	if k.gen.anyTurnUpRepl && s.turnUpAttachEffStage(k, sl) {
		return true // face_turnupattach.go: the attach ask parks the effect's resolution
	}
	pm := &s.perms[sl]
	// The turn-up event's actor is the resolving effect's controller, not the
	// permanent's: CR 603.2's ValidCause$ SpellAbility.YouCtrl reads that
	// controller (gorge turnFaceUpCauseAdmits's InResolution/ActionCause arm).
	s.turnUpFlip(k, sl, pm.obj, s.res.e.ctrl, 0) // face_facedown.go
	return false
}

// transformPickOK is gorge setStateObjWouldChange's Transform arm for a
// battlefield permanent: its card has another face and it is not daybound or
// nightbound (CR 702.145b/e). transformObj turns exactly these.
func (s *State) transformPickOK(k *prep, sl Slot) bool {
	o := s.perms[sl].obj
	cd := &k.t.Cards[s.objs[o].card]
	return len(cd.Faces) >= 2 && cd.FaceMode != kop.FacePrototype && !(k.anyDayNight && s.dnBound(k, o))
}

// turnUpEffCands writes the permanents matching the op's filter that would
// turn up (C 2: transform), seat 0 first, each seat in battlefield order
// (gorge setStateChoose's AliveFrom(0) walk).
func (s *State) turnUpEffCands(k *prep, in *kop.Instr, buf *[MaxPerm]Slot) int {
	var m slotMask
	src := s.resSrcSlot(k)
	for q := s.occ; q.any(); q = q.rest() {
		sl := q.first()
		if !s.matchPerm(k, in.F, sl, s.res.e.ctrl, src) {
			continue
		}
		if in.C == turnUpPickTransform && s.transformPickOK(k, sl) || in.C != turnUpPickTransform && s.turnUpEffOK(k, sl) {
			m = m.with(sl)
		}
	}
	return s.listenerOrder(m, buf) // every seat's battlefield, seat by seat
}

// askOptionsTurnUpEff builds the pick's options: each candidate, then Done
// (gorge's Min 0; none for the mandatory Transform pick); false: not this
// ask.
func (s *State) askOptionsTurnUpEff(k *prep, d *Decision) bool {
	if s.res.ask != raGenTurnUpPick {
		return false
	}
	d.Sub = ChGenTurnUpPick
	var buf [MaxPerm]Slot
	in := &s.prog(k)[s.res.pc]
	n := s.turnUpEffCands(k, in, &buf)
	for _, sl := range buf[:n] {
		d.add(MakeAction(AkChoose, TCode(s.perms[sl].obj), 0, 0))
	}
	if in.C != turnUpPickTransform {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
	return true
}

// applyAskTurnUpEff applies the pick (Done declines); false: not this ask.
func (s *State) applyAskTurnUpEff(k *prep, a Action) bool {
	if s.res.ask != raGenTurnUpPick {
		return false
	}
	s.res.ask = raNone
	if a.Kind() == AkChoose {
		if ob := &s.objs[Obj(a.Source())]; ob.zone == kop.ZoneBattlefield {
			if s.prog(k)[s.res.pc].C == turnUpPickTransform {
				s.transformObj(k, Obj(a.Source())) // act_transform.go
			} else if s.turnUpByEffect(k, ob.slot) {
				return true // parked on the attach election: its answer continues the program
			}
		}
	}
	s.continueRes(k, 0)
	return true
}
