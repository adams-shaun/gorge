package kgen

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The kop forms of the LIR ops for control, attachment and permanent state (instr.go).
func init() {
	instrOp("Earthbend", (*instrCtx).opEarthbend)
	instrOp("Prepare", (*instrCtx).opPrepare)
	instrOp("ChangeSpeed", (*instrCtx).opChangeSpeed)
	instrOp("Transform", (*instrCtx).opTransform)
	instrOp("ClassLevelUp", (*instrCtx).opClassLevelUp)
	instrOp("ExchangeControl", (*instrCtx).opExchangeControl)
	instrOp("TurnFaceUp", (*instrCtx).opTurnFaceUp)
	instrOp("UnlockDoor", (*instrCtx).opUnlockDoor)
	instrOp("BreachChoose", (*instrCtx).opBreachChoose)
	instrOp("BreachReturn", (*instrCtx).opBreachReturn)
	instrOp("Airbend", (*instrCtx).opAirbend)
	instrOp("Meld", (*instrCtx).opMeld)
	instrOp("ChangeTargets", (*instrCtx).opChangeTargets)
	instrOp("Phases", (*instrCtx).opPhases)
	instrOp("Attach", (*instrCtx).opAttach)
	instrOp("Exploit", (*instrCtx).opExploit)
	instrOp("AttachObj", (*instrCtx).opAttachObj)
	instrOp("Connive", (*instrCtx).opConnive)
	instrOp("ConniveCounters", (*instrCtx).opConniveCounters)
	instrOp("GainControlEOT", (*instrCtx).opGainControlEOT)
	instrOp("GainControl", (*instrCtx).opGainControl)
}

func (ic *instrCtx) opEarthbend() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpEarthbend
	i, err := cb.override(cb.c.overrides[in.C])
	if err != nil {
		return err
	}
	ki.C = int16(i)
	return nil
}

func (ic *instrCtx) opPrepare() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C = kop.OpPrepare, int16(in.C) // C=1: unprepare (Activate$ False)
	return nil
}

func (ic *instrCtx) opChangeSpeed() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.C, ki.B = kop.OpChangeSpeed, int16(in.C), kop.Const(in.B.N)
	return nil
}

func (ic *instrCtx) opTransform() error {
	ki := &ic.ki
	ki.Op = kop.OpTransform
	return nil
}

func (ic *instrCtx) opClassLevelUp() error {
	ki := &ic.ki
	in := &ic.in
	if ki.Obj != kop.RefSelf || !in.A.isConst() {
		return fmt.Errorf("ClassLevelUp on %v", in.Obj)
	}
	ki.Op = kop.OpClassLevelUp
	return nil
}

func (ic *instrCtx) opExchangeControl() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtExchangeControl)), int16(in.C)
	return nil
}

func (ic *instrCtx) opTurnFaceUp() error {
	ki := &ic.ki
	in := &ic.in
	// (CR 708.6; gorge effSetState Mode$ TurnFaceUp) OpExt ExtTurnFaceUp:
	// C 0 the target in Obj, C 1 the up-to-one pick over F, C 2 the
	// mandatory Transform pick over F (lower_setstate.go)
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtTurnFaceUp)), int16(in.C)
	if in.F != nil {
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opUnlockDoor() error {
	ki := &ic.ki
	in := &ic.in
	// (CR 709.5f-g; gorge effUnlockDoor) OpExt
	// ExtUnlockDoor over the Room target slot, C 0 unlock / 1 lock-or-unlock
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtUnlockDoor)), int16(in.C)
	return nil
}

func (ic *instrCtx) opBreachChoose() error {
	ki := &ic.ki
	// (Breach the Multiverse) OpExt ExtBreachChoose
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtBreachChoose))
	return nil
}

func (ic *instrCtx) opBreachReturn() error {
	ki := &ic.ki
	// (Breach the Multiverse) OpExt ExtBreachReturn
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtBreachReturn))
	return nil
}

func (ic *instrCtx) opAirbend() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtAirbend over Obj, or F (Obj RefNone) less
	// target slot C-1
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtAirbend)), int16(in.C)
	if in.F != nil {
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

// opMeld is OpExt ExtMeld (dirac/face_meld.go): Obj the source, F the
// partner's filter.
func (ic *instrCtx) opMeld() error {
	ic.ki.Op, ic.ki.A = kop.OpExt, kop.Const(int(kop.ExtMeld))
	return ic.objFilter(kop.ZoneBattlefield)
}

func (ic *instrCtx) opChangeTargets() error {
	ki := &ic.ki
	in := &ic.in
	// OpExt ExtChangeTargets over Obj (the
	// TgtStackObj slot, or the triggering spell); C bit 0: Optional$
	ki.Op, ki.A, ki.C = kop.OpExt, kop.Const(int(kop.ExtChangeTargets)), int16(in.C)
	return nil
}

func (ic *instrCtx) opPhases() error {
	ki := &ic.ki
	in := &ic.in
	// (Spectral Adversary) OpExt ExtPhases, Obj the permanent or target slot
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtPhases))
	if in.F != nil { // AllValid$: Obj RefNone, every battlefield permanent matching F
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	return nil
}

func (ic *instrCtx) opAttach() error {
	ki := &ic.ki
	in := &ic.in
	ki.Op = kop.OpAttach
	switch in.CS {
	case "":
	case "Remember":
		ki.C = int16(kop.MvRemember) // RememberAttached$
	case "Unattach":
		ki.C = int16(kop.AttachUnattach) // Reconfigure's unattach half (CR 702.151a)
	default:
		return fmt.Errorf("Attach CS %q", in.CS)
	}
	return nil
}

func (ic *instrCtx) opExploit() error {
	ki := &ic.ki
	ki.Op, ki.A = kop.OpExt, kop.Const(int(kop.ExtExploit)) // dirac/kw_exploit.go
	return nil
}

func (ic *instrCtx) opAttachObj() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	ki.Op = kop.OpAttachObj
	r, err := cb.ref(in.Obj2)
	if err != nil {
		return err
	}
	ki.C = int16(r)
	if in.Note == attachRememberNote {
		ki.B = kop.Const(1) // remember each object attached
	}
	return nil
}

func (ic *instrCtx) opConnive() error {
	ki := &ic.ki
	ki.Op = kop.OpConnive
	return nil
}

func (ic *instrCtx) opConniveCounters() error {
	ki := &ic.ki
	ki.Op = kop.OpConniveCounters
	return nil
}

func (ic *instrCtx) opGainControlEOT() error {
	ki := &ic.ki
	in := &ic.in
	if in.CS != "Untap|kw=Haste" {
		return fmt.Errorf("gain control riders %q", in.CS)
	}
	ki.Op = kop.OpGainControlEOT
	return nil
}

func (ic *instrCtx) opGainControl() error {
	ki := &ic.ki
	in := &ic.in
	cb := ic.cb
	fl := ic.fl
	ki.Op = kop.OpGainControl
	if in.F != nil { // AllValid$: every battlefield permanent F admits (lower_gaincontrolall.go)
		if err := ic.objFilter(kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	var rid int
	for _, f := range strings.Split(in.CS, "|") {
		switch {
		case f == "Untap":
			rid |= kop.GcUntap
		case f == "kw=Haste":
			rid |= kop.GcHasteEOT // until end of turn on DurEOT, else while the control record lives (opGainControl)
		case f == "Remember":
			rid |= kop.GcRemember
		case strings.HasPrefix(f, "dur="):
		default:
			return fmt.Errorf("gain control rider %q", f)
		}
	}
	ki.B = kop.Const(rid)
	switch fl["dur"] {
	case "EOT":
		ki.A = kop.Const(kop.DurEOT)
	case "Permanent":
		ki.A = kop.Const(kop.DurPermanent)
	case "WhileSource":
		ki.A = kop.Const(kop.DurWhileSource)
	case "WhileControlSource":
		ki.A = kop.Const(kop.DurWhileControlSource)
	case "YourNextTurn":
		ki.A = kop.Const(kop.DurYourNextTurn)
	default:
		return fmt.Errorf("gain control duration %q", fl["dur"])
	}
	if in.C != 0 {
		r, err := cb.ref(lRef{Kind: refKind(in.C >> 8), N: in.C & 0xFF})
		if err != nil {
			return err
		}
		if _, isTgt := tgtSlotOfRef(r); r != kop.RefOpponent && r != kop.RefEachOpponent && r != kop.RefTriggerPlayer && r != kop.RefTriggerTgtCtrl && r != kop.RefChosenPlayer && !isTgt {
			return fmt.Errorf("gain control new controller ref %d", r)
		}
		ki.C = int16(r)
	}
	return nil
}
