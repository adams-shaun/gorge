package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCounterGoneSelfCastSpellKeepsStack: a Dash Hopes countered while its
// own cast trigger waits resolves that trigger into nothing when a player
// pays (gorge effCounter: the spell has left the stack). The kernel's
// "counter that spell" never removes the resolving trigger's own entry, so
// the Grizzly Bears spell below it keeps its stack entry.
func TestCounterGoneSelfCastSpellKeepsStack(t *testing.T) {
	sc, id := selfCastScen(t, "Dash Hopes", "Grizzly Bears")
	sc.lands(0, id["Forest"], 2)
	sc.lands(1, id["Swamp"], 6)
	gb := sc.hand(id["Grizzly Bears"], 0)
	sc.castCard(t, gb)
	dh1 := sc.hand(id["Dash Hopes"], 1)
	dh2 := sc.obj(id["Dash Hopes"], 1)
	sc.s.place(dh2, kop.ZoneHand, false)
	// priorityTo steps to player p's priority, with the top entry a
	// trigger when trig is set, leaving that decision unanswered.
	priorityTo := func(p uint8, trig bool) {
		t.Helper()
		for i := 0; i < 20; i++ {
			d := sc.s.Step()
			top := sc.s.sp > 0 && sc.s.stack[sc.s.sp-1].kind == seTrigger
			if d.Kind == DkPriority && d.Player == p && (top || !trig) {
				return
			}
			if d.Kind == DkPriority {
				sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			} else {
				sc.s.Apply(d.Options[0])
			}
		}
		t.Fatalf("player %d never got priority", p)
	}
	// castAt casts o and answers its target choice with tgt.
	castAt := func(o, tgt Obj) {
		t.Helper()
		sc.castCard(t, o)
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Target() == TCode(tgt) || a.Source() == TCode(tgt) {
				sc.s.Apply(a)
				if sc.s.stack[sc.s.sp-1].tgt[0] != TCode(tgt) {
					t.Fatalf("%d targets %d, want %d", o, sc.s.stack[sc.s.sp-1].tgt[0], tgt)
				}
				return
			}
		}
		t.Fatalf("target %d for %d not offered: kind %d %v", tgt, o, d.Kind, d.Options[:d.N])
	}
	priorityTo(1, false)
	castAt(dh1, gb)
	priorityTo(1, true) // stack: [Bears, Dash Hopes #1, its trigger]
	castAt(dh2, dh1)
	priorityTo(0, true) // stack: [Bears, DH1, DH1 trigger, DH2, DH2 trigger]
	if sc.s.sp != 5 {
		t.Fatalf("stack %d entries, want 5", sc.s.sp)
	}
	// Both decline DH2's trigger, so DH2 resolves and counters DH1; then
	// DH1's trigger resolves and player 0 pays.
	stage := 0 // 0,1: DH2 trigger asks (decline); 2: DH1 trigger ask (pay)
	for i := 0; i < 60 && stage < 3; i++ {
		d := sc.s.Step()
		switch {
		case d.Kind == DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case d.Kind == DkChoose && d.Sub == ChGenPayMode:
			a := uint8(0)
			if stage == 2 {
				if z := sc.s.objs[dh1].zone; z != kop.ZoneGraveyard {
					t.Fatalf("Dash Hopes #1 in zone %d at its trigger's ask, want the graveyard", z)
				}
				a = 1
			}
			stage++
			sc.s.Apply(MakeAction(AkChoose, 0, 0, a))
		default:
			sc.s.Apply(d.Options[0])
		}
	}
	if stage != 3 {
		t.Fatalf("reached ask stage %d, want 3", stage)
	}
	for i := 0; i < 6; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.pl[0].life != 15 {
		t.Fatalf("player 0 life %d, want 15 (paid)", sc.s.pl[0].life)
	}
	if z := sc.s.objs[gb].zone; z != kop.ZoneStack {
		t.Fatalf("Grizzly Bears in zone %d, want the stack", z)
	}
	if sc.s.stackIndexOf(gb) < 0 {
		t.Fatalf("Grizzly Bears has no stack entry (stack %d entries)", sc.s.sp)
	}
}
