package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckKohGainedTriggerGates (trig_gainedtrig_gate_build_test.go): the
// gained-trigger walk's event gates. A GainsTrig recipient listens to its
// lenders' events even when no battlefield permanent subscribes to them: Koh
// exiles Ajani's Pridemate (Mode$ LifeGained), names it, and the Pristine
// Talisman's life gain queues the Cat's "whenever you gain life" line as
// Koh's, so the +1/+1 counter lands on Koh. The queued stack entry keeps
// trigHost 0 (the recipient is the source; Forge's OriginalHost is not the
// exiled lender). The second subtest is the gate: Koh exiles Endrider
// Spikespitter (a max-speed-gated upkeep trigger), names it, and seat 0's
// speed is 0, so the gate fails on the recipient and nothing queues; without
// the gate check the gained line would fire unconditionally as Koh's.
func CheckKohGainedTriggerGates(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()

	// namedLender casts Koh, exiles the opponent's lender with the ETB
	// trigger, and names it through the Pay-life ChooseCard activation.
	namedLender := func(t *testing.T, lender CardID) (*scen, Obj, Obj) {
		t.Helper()
		sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
		for i := 0; i < 6; i++ {
			sc.perm(id["Swamp"], 0)
		}
		lo := sc.perm(lender, 1)
		koh := sc.hand(id["Koh, the Face Stealer"], 0)
		passStack := func() {
			for i := 0; i < 16; i++ {
				d := sc.s.Step()
				if d.Kind != DkPriority {
					return
				}
				if sc.s.sp == 0 && sc.s.npend == 0 {
					return
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
		sc.do(t, AkTarget, scAny, TCode(lo))
		passStack() // the trigger resolves: the lender is exiled with Koh
		if sc.s.objs[lo].zone != kop.ZoneExile {
			t.Fatalf("the lender was not exiled by the ETB trigger (zone %d)", sc.s.objs[lo].zone)
		}
		sc.do(t, AkActivate, TCode(koh), scAny)
		passStack() // the activation resolves into the card ask
		sc.do(t, AkChoose, TCode(lo), scAny)
		return sc, koh, lo
	}

	// toNextUpkeep passes to seat 0's next upkeep.
	toNextUpkeep := func(t *testing.T, sc *scen) {
		turn := sc.s.turn
		for i := 0; i < 400; i++ {
			if sc.s.active == 0 && sc.s.turn > turn && sc.s.step == kop.StepUpkeep {
				return
			}
			d := sc.s.Step()
			if d.Kind == DkNone {
				t.Fatal("the game ended")
			}
			switch d.Kind {
			case DkPriority:
				sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			case DkAttackers, DkBlockers:
				sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
			default:
				t.Fatalf("unexpected decision kind %d sub %d player %d", d.Kind, d.Sub, d.Player)
			}
		}
		t.Fatal("never reached seat 0's next upkeep")
	}

	t.Run("an exiled LifeGained lender fires on a battlefield-empty event", func(t *testing.T) {
		sc, koh, lo := namedLender(t, id["Ajani's Pridemate"])
		// precondition: the exiled lender's slot is no longer a subscriber
		// (Koh may itself be one after the fix; the point is the lender is off
		// the battlefield when the event happens).
		if sc.s.subs[kop.TrigLifeGained].has(sc.s.objs[lo].slot) {
			t.Fatal("setup: the exiled lender still subscribes to LifeGained")
		}
		sc.s.gainLife(sc.k, 0, 1) // the LifeGained event site
		if sc.s.npend == 0 {
			t.Fatal("the exiled Ajani's Pridemate's life-gain trigger never queued as Koh's")
		}
		for i := 0; i < 8 && sc.s.sp == 0 && sc.s.npend > 0; i++ {
			sc.s.Step()
		}
		if sc.s.sp == 0 {
			t.Fatalf("the gained trigger never reached the stack (pending %d)", sc.s.npend)
		}
		e := &sc.s.stack[sc.s.sp-1]
		if e.card != id["Ajani's Pridemate"] {
			t.Fatalf("stack entry card %q, want Ajani's Pridemate", tbl.Cards[e.card].Name)
		}
		if h := e.trigHost(); h != 0 {
			t.Fatalf("gained trigger's trigHost = %d, want 0 (the recipient, not the exiled lender, is the source)", h)
		}
		for i := 0; i < 20 && (sc.s.sp != 0 || sc.s.npend != 0); i++ {
			d := sc.s.Step()
			if d.Kind == DkNone {
				break
			}
			for j := 0; j < int(d.N); j++ {
				if d.Options[j].Kind() == AkPass {
					sc.s.Apply(d.Options[j])
					break
				}
			}
		}
		if n := sc.s.perms[sc.slot(koh)].ctrN(sc.s, int(kop.CtrP1P1)); n != 1 {
			t.Fatalf("Koh has %d +1/+1 counters, want 1: the gained life-gain trigger did not resolve as Koh's", n)
		}
	})

	t.Run("a max-speed gate is read on the recipient, not the lender", func(t *testing.T) {
		sc, _, _ := namedLender(t, id["Endrider Spikespitter"])
		if sc.s.speedOf(0) != 0 {
			t.Fatalf("setup: seat 0's speed is %d, want 0", sc.s.speedOf(0))
		}
		toNextUpkeep(t, sc)
		if sc.s.sp != 0 || sc.s.npend != 0 {
			t.Fatalf("the gained upkeep trigger queued (stack %d pending %d): the max-speed gate was not read on Koh", sc.s.sp, sc.s.npend)
		}
	})
}
