package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kxpaid: the X a trigger binds from its trigger object's spell (CR
// 107.3m), the spell hold and the mandatory dig (trig_xpaid.go).

// chooseX answers the cast's X ask with x.
func (sc *scen) chooseX(t *testing.T, x uint8) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == x {
			sc.s.Apply(d.Options[i])
			return
		}
	}
	t.Fatalf("X = %d not offered (%d options)", x, d.N)
}

// TestTrigXBound: Triceraton Commander cast for X = 2 makes two tokens: its
// enters trigger reads the X the resolving spell was paid, carried by the
// pending entry into the stack entry; the projection shows the trigger's
// own X (0), as gorge's stack object does.
func TestTrigXBound(t *testing.T) {
	sc, id := replRegenerateScen(t, "Triceraton Commander")
	sc.lands(0, id["Plains"], 6)
	sc.castCard(t, sc.hand(id["Triceraton Commander"], 0))
	sc.chooseX(t, 2)
	for i := 0; i < 20 && !(sc.s.sp > 0 && sc.s.stack[sc.s.sp-1].kind == seTrigger); i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		} else {
			sc.s.Apply(d.Options[0])
		}
	}
	if sc.s.sp == 0 || sc.s.stack[sc.s.sp-1].kind != seTrigger {
		t.Fatal("the enters trigger never reached the stack")
	}
	if e := &sc.s.stack[sc.s.sp-1]; e.x != 2 {
		t.Fatalf("the enters trigger's bound X is %d, want 2", e.x)
	}
	var p Projection
	sc.s.Project(&p)
	if x := p.Stack[p.NStack-1].X; x != 0 {
		t.Fatalf("the trigger's projected X is %d, want 0 (gorge's own X)", x)
	}
	sc.resolveAll(t)
	n := 0
	for m := sc.s.ctrlM[0].and(sc.s.occ); m.any(); m = m.rest() {
		if sc.s.objs[sc.s.perms[m.first()].obj].of&ofToken != 0 {
			n++
		}
	}
	if n != 2 {
		t.Fatalf("%d tokens, want 2", n)
	}
}

// TestMandatoryDigForced: a mandatory dig (Geometer's Arthropod's "put one
// of them into your hand") offers no stop, and with only one card to take
// takes it unasked (gorge effDig's Min and its no-choice tail).
func TestMandatoryDigForced(t *testing.T) {
	sc, id := replRegenerateScen(t, "Geometer's Arthropod")
	s := sc.s
	o := sc.perm(id["Geometer's Arthropod"], 0)
	ab := &sc.k.t.Cards[id["Geometer's Arthropod"]].Abilities[0]
	s.res = resCtx{e: stackEntry{kind: seTrigger, obj: o, srcVer: s.objs[o].ver, card: id["Geometer's Arthropod"], ab: 0, ctrl: 0, x: 3, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	if ab.Prog[0].Op != kop.OpDigEx {
		t.Fatalf("Arthropod's program starts with %s", ab.Prog[0].Op)
	}
	hand := s.pl[0].handN
	s.runProg(sc.k)
	d := s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenDig || d.N != 3 {
		t.Fatalf("decision kind %d sub %d with %d options, want the dig pick over 3 cards and no stop", d.Kind, d.Sub, d.N)
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkDone {
			t.Fatal("a mandatory dig offered a stop")
		}
	}
	s.Apply(d.Options[0])
	if s.pl[0].handN != hand+1 {
		t.Fatalf("hand %d, want %d", s.pl[0].handN, hand+1)
	}
	// one card looked at: taken unasked
	s.res = resCtx{e: stackEntry{kind: seTrigger, obj: o, srcVer: s.objs[o].ver, card: id["Geometer's Arthropod"], ab: 0, ctrl: 0, x: 1, trigObj: NoObj}, active: true, lastTok: NoObj, eachObj: NoObj}
	s.ctl = ctlPriority
	s.runProg(sc.k)
	if s.res.ask == raGenDig || s.pl[0].handN != hand+2 {
		t.Fatalf("one card: asked %v, hand %d, want taken unasked (%d)", s.res.ask == raGenDig, s.pl[0].handN, hand+2)
	}
}
