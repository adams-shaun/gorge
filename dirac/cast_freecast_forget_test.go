package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFreePlayForgetPlayedReversedKeepsRemembered: ForgetPlayed$ drops a card
// from the remembered set only once its cast commits on the stack (CR 601.2a,
// gorge's PlayDone pass). A cast that is begun and then reversed (CR 733.1-2)
// returns the card to the zone it was cast from and never commits, so the card
// stays remembered at its original index — otherwise the chained "if you don't
// cast it" arm would miss a card that was never cast.
func TestFreePlayForgetPlayedReversedKeepsRemembered(t *testing.T) {
	sc, synth, id := m11Scen(t, []kop.Instr{{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: 29}})
	var rem [2]Obj
	// Shock carries a mandatory target, so the build pauses after the push at
	// its target ask: the cast is begun (on the stack) but not committed.
	rem[0] = sc.obj(id["Shock"], 0)
	rem[1] = sc.obj(id["Opt"], 0)
	for i := range rem {
		sc.s.place(rem[i], kop.ZoneExile, false)
		sc.s.g.rem[i] = rem[i]
	}
	sc.s.g.nrem = 2
	m11Start(sc, synth, nil)
	d := m11FreeAsk(t, sc)
	m11Choose(t, sc, d, rem[0])

	begun := false
	for i := 0; i < 50 && !begun; i++ {
		d2 := sc.s.Step()
		if d2.Kind == DkNone {
			break
		}
		if sc.s.ctl == ctlBuild && sc.s.bld().e.obj == rem[0] && sc.s.bld().bfl&bflPushed != 0 {
			begun = true
			break
		}
		if d2.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		sc.s.Apply(d2.Options[0])
	}
	if !begun {
		t.Fatalf("the picked card's cast never began (zone %d, ctl %d)", sc.s.objs[rem[0]].zone, sc.s.ctl)
	}
	// the precondition the rule reads: the card is on the stack, and the pick
	// itself has not forgotten it
	if z := sc.s.objs[rem[0]].zone; z != kop.ZoneStack {
		t.Fatalf("the begun cast is in zone %d, want the stack", z)
	}
	if sc.s.g.nrem != 2 {
		t.Fatalf("remembered set %v before the reversal, want both cards", sc.s.g.rem[:sc.s.g.nrem])
	}

	sc.s.reverseCast(sc.k)

	if z := sc.s.objs[rem[0]].zone; z != kop.ZoneExile {
		t.Fatalf("the reversed cast is in zone %d, want exile (the zone it was cast from)", z)
	}
	if sc.s.g.nrem != 2 || sc.s.g.rem[0] != rem[0] || sc.s.g.rem[1] != rem[1] {
		t.Fatalf("remembered set %v, want %d then %d (a reversed cast never happened)", sc.s.g.rem[:sc.s.g.nrem], rem[0], rem[1])
	}

	// the drained queue must not forget it either
	m11Drive(t, sc, func() bool { return sc.s.res.ask == raNone && sc.s.ctl == ctlPriority })
	if sc.s.g.nrem != 2 || sc.s.g.rem[0] != rem[0] || sc.s.g.rem[1] != rem[1] {
		t.Fatalf("remembered set %v after the tail, want %d then %d", sc.s.g.rem[:sc.s.g.nrem], rem[0], rem[1])
	}
}
