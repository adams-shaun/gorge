package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestAnyActivatorSorceryAndYou: an AnyActivator ability "only as a sorcery"
// (Sash and Waistcoat, Unmen's shape: CostSorcery beside a plain {1}) is not
// offered to the opponent of its controller during the controller's turn,
// and is offered on the opponent's own main phase with an empty stack (gorge
// legal_walk_battlefield.go: SorcerySpeed$ is the activator's sorcery window).
// Activated there, it is paid from the activator's land and its "you"
// (RefController) is the activator, who draws (CR 602.2a, 109.5).
func TestAnyActivatorSorceryAndYou(t *testing.T) {
	tbl, _, ids := newModeTable(kop.CardDef{Name: "Test Any Drawer", Types: kop.TCreature, Power: 2, Toughness: 2,
		Abilities: []kop.Ability{{Kind: kop.AbActivated, AnyActivator: true,
			Cost: kop.Cost{Mana: kop.ManaCost{Generic: 1}, Flags: kop.CostSorcery},
			Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyAnyAct {
		t.Fatal("the table does not arm the any-player offer (genPrep.anyAnyAct)")
	}
	src := sc.perm(ids[0], 0)
	land := sc.perm(v.Plains, 1)
	sc.do(t, AkPass, scAny, scAny) // seat 0 passes in its own main phase
	if d := sc.s.Step(); d.Kind != DkPriority || d.Player != 1 {
		t.Fatalf("decision kind %d player %d, want seat 1's priority", d.Kind, d.Player)
	}
	if sc.activateOffered(src, 0) {
		t.Fatal("a sorcery-speed any-player ability is offered to seat 1 during seat 0's turn")
	}
	// seat 1's own main phase, empty stack
	sc.s.active, sc.s.prio, sc.s.passes, sc.s.step, sc.s.ctl = 1, 1, 0, kop.StepMain1, ctlPriority
	if !sc.activateOffered(src, 0) {
		t.Fatal("the any-player ability is not offered to seat 1 in its own main phase")
	}
	hand0, hand1 := sc.s.pl[0].handN, sc.s.pl[1].handN
	sc.do(t, AkActivate, TCode(src), scAny)
	for i := 0; i < 20 && sc.s.ctl != ctlPriority; i++ {
		d := sc.s.Step()
		if d.N == 0 {
			t.Fatalf("decision kind %d with no options while activating", d.Kind)
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.sp != 1 || sc.s.stack[0].ctrl != 1 {
		t.Fatalf("stack %d (controller %d), want the ability on it under seat 1", sc.s.sp, sc.s.stack[0].ctrl)
	}
	if !sc.tapped(land) {
		t.Fatal("seat 1's land did not pay the activation")
	}
	sc.passBoth(t)
	if sc.s.sp != 0 || sc.s.pl[1].handN != hand1+1 || sc.s.pl[0].handN != hand0 {
		t.Fatalf("stack %d, hands %d/%d (want %d/%d: the activator drew)", sc.s.sp, sc.s.pl[0].handN, sc.s.pl[1].handN, hand0, hand1+1)
	}
}
