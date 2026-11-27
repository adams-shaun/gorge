package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckKohGainedTrigger (trig_gainedtrig_build_test.go): Koh, the Face
// Stealer's GainsTriggerAbsOf$ walk. Koh enters and exiles the opponent's
// Blood Artist, its Pay-life activation names that card, and Koh then has
// Blood Artist's triggered abilities as its own: a creature dying queues the
// death drain with Koh as its source (the target ask is Koh's controller's,
// "you gain 1 life" is seat 0). With DebugNoGainsTrig the drain never queues:
// the same script leaves both players' life unchanged.
func CheckKohGainedTrigger(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()

	// kohBoard casts Koh (the ETB exile trigger exiling the opponent's Blood
	// Artist) and names the Artist through the Pay-life activation.
	kohBoard := func(t *testing.T) (*scen, Obj, Obj) {
		t.Helper()
		sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
		for i := 0; i < 6; i++ {
			sc.perm(id["Swamp"], 0)
		}
		ba := sc.perm(id["Blood Artist"], 1)
		ser := sc.perm(id["Serendib Efreet"], 1)
		koh := sc.hand(id["Koh, the Face Stealer"], 0)
		// passStack passes every priority round while the stack has entries
		// or pending triggers, and stops at a real decision or a settled
		// stack: Koh's Pay-life activation is repeatable, so a priority
		// round carries its option beside the pass and must still be passed
		// (the shared do/noPass only see single-option pass rounds).
		passStack := func() {
			for i := 0; i < 16; i++ {
				d := sc.s.Step()
				if d.Kind != DkPriority {
					return // a mid-resolution ask: the next do answers it
				}
				if sc.s.sp == 0 && sc.s.npend == 0 {
					return // the stack settled: the caller acts
				}
				for j := 0; j < int(d.N); j++ {
					if d.Options[j].Kind() == AkPass {
						sc.s.Apply(d.Options[j])
						break
					}
				}
			}
			t.Fatal("the stack never settled")
		}
		sc.do(t, AkCast, TCode(koh), scAny)
		passStack() // the spell resolves; the ETB trigger asks its target
		sc.do(t, AkTarget, scAny, TCode(ba))
		passStack() // the trigger resolves: the Artist is exiled with Koh
		// name the exiled Artist through the Pay-1-life ChooseCard activation
		sc.do(t, AkActivate, TCode(koh), scAny)
		passStack() // the activation resolves into the card ask
		sc.do(t, AkChoose, TCode(ba), scAny)
		return sc, koh, ser
	}

	// toNextMain passes to seat 0's next main phase (turns increment one per
	// player turn, so the opponent's turn is played through, its Serendib
	// burning itself): Koh was cast this turn and is summoning sick until then.
	toNextMain := func(t *testing.T, sc *scen) {
		turn := sc.s.turn
		for i := 0; i < 300; i++ {
			if sc.s.active == 0 && sc.s.turn > turn && sc.s.step == kop.StepMain1 {
				return
			}
			d := sc.s.Step()
			if d.Kind == DkNone {
				t.Fatal("the game ended")
			}
			switch d.Kind {
			case DkPriority:
				sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			case DkAttackers:
				sc.s.Apply(MakeAction(AkDone, 0, 0, 0)) // nobody attacks
			default:
				t.Fatalf("unexpected decision kind %d sub %d player %d", d.Kind, d.Sub, d.Player)
			}
		}
		t.Fatal("never reached seat 0's next main phase")
	}

	// combat has Koh attack into the Serendib's block: the 2/3 dies to the
	// 6/6, and the lender's drain queues as Koh's trigger.
	combat := func(t *testing.T, sc *scen, koh, ser Obj) (l0, l1 int16) {
		t.Helper()
		sc.passUntil(t, kop.StepAttackers)
		sc.do(t, AkAttack, TCode(koh), scAny)
		sc.do(t, AkDone, scAny, scAny)
		sc.passUntil(t, kop.StepBlockers)
		sc.do(t, AkBlock, TCode(ser), TCode(koh))
		sc.do(t, AkDone, scAny, scAny)
		sc.passUntil(t, kop.StepMain2)
		l0, l1 = sc.s.pl[0].life, sc.s.pl[1].life
		if sc.onBF(ser) {
			t.Fatal("the blocking Serendib survived combat with Koh")
		}
		return l0, l1
	}

	// settle answers everything the death queued: the two triggers share a
	// source (Koh), so the order ask comes first; the drain then asks its
	// controller (seat 0) for a target and Koh's own dies trigger its may ask.
	settle := func(t *testing.T, sc *scen) {
		t.Helper()
		for i := 0; i < 20; i++ {
			if sc.s.sp == 0 && sc.s.npend == 0 {
				return
			}
			d := sc.s.Step()
			if d.Kind == DkNone {
				return
			}
			applied := false
			for j := 0; j < int(d.N); j++ {
				a := d.Options[j]
				switch a.Kind() {
				case AkTarget:
					if a.Target() == PlayerTarget1 {
						sc.s.Apply(a)
						applied = true
					}
				case AkOrder, AkChoose, AkDone, AkPass:
					sc.s.Apply(a)
					applied = true
				}
				if applied {
					break
				}
			}
			if !applied {
				t.Fatalf("no answerable option in decision kind %d: %v", d.Kind, d.Options[:d.N])
			}
		}
		t.Fatal("the stack never settled after the death")
	}

	t.Run("the named lender's death drain fires as Koh's", func(t *testing.T) {
		sc, koh, ser := kohBoard(t)
		if sc.s.objs[ba2Obj(sc, id)].zone != kop.ZoneExile {
			t.Fatal("the Blood Artist was not exiled by the ETB trigger")
		}
		toNextMain(t, sc)
		l0, l1 := combat(t, sc, koh, ser)
		settle(t, sc)
		if sc.s.pl[1].life != l1-1 || sc.s.pl[0].life != l0+1 {
			t.Fatalf("life %d/%d after the death, want %d/%d: the Blood Artist's death drain did not fire as Koh's",
				sc.s.pl[0].life, sc.s.pl[1].life, l0+1, l1-1)
		}
	})

	t.Run("with the walk dropped nothing queues", func(t *testing.T) {
		if DebugNoGainsTrig {
			t.Skip("the mutation harness owns the flag")
		}
		DebugNoGainsTrig = true
		defer func() { DebugNoGainsTrig = false }()
		sc, koh, ser := kohBoard(t)
		toNextMain(t, sc)
		l0, l1 := combat(t, sc, koh, ser)
		settle(t, sc)
		if sc.s.pl[0].life != l0 || sc.s.pl[1].life != l1 {
			t.Fatalf("life %d/%d with the gained-trigger walk dropped, want %d/%d: the drain still fired",
				sc.s.pl[0].life, sc.s.pl[1].life, l0, l1)
		}
	})
}

// ba2Obj finds the opponent's Blood Artist (still in the exile zone after the
// ETB trigger exiled it) for the zone assertion.
func ba2Obj(sc *scen, id map[string]CardID) Obj {
	for i := 0; i < int(sc.s.pl[1].exileN); i++ {
		if o := sc.s.pl[1].exile[i]; sc.s.objs[o].card == id["Blood Artist"] {
			return o
		}
	}
	return NoObj
}
