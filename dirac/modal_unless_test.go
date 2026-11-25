package dirac

// A triggered api:GenericChoice is a charm marked kop.AfCharmTrigger: Forge's
// ChooseGenericEffect (and gorge's GenericChoiceAvailable) drops a mode whose
// UnlessCost$ the chooser cannot pay, so the mode offer must not present it.

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

func TestModeUnlessPayableFiltersTheOffer(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	a := kop.Ability{Kind: kop.AbTriggered, NModes: 1, ModesPick: 1, AFlags: kop.AfCharmTrigger}
	a.Prog = []kop.Instr{
		{Op: kop.OpCharm, C: 0, A: kop.Const(2)},
		{Op: kop.OpUnlessSwitched, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(1), C: int16(kop.PayMana)},
	}
	// PRECONDITION: the mode block really opens with the unpayable unless
	// cost the gate reads, and the pool is empty.
	if a.Prog[1].Op != kop.OpUnlessSwitched || a.Prog[1].C&0xF != kop.PayMana {
		t.Fatalf("precondition: mode head = %+v, want an unless mana cost", a.Prog[1])
	}
	if sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("precondition: pool = %v, want empty", sc.s.pl[0].pool)
	}
	if sc.s.modeFeasible(sc.k, &a, 0) {
		t.Fatal("offered a GenericChoice mode whose UnlessCost$ the controller cannot pay")
	}

	// Payable: the {2}{W} is in the pool, so the mode is offered.
	sc.s.pl[0].pool[kop.W] = 3
	if !sc.s.modeFeasible(sc.k, &a, 0) {
		t.Fatal("dropped a GenericChoice mode the controller can pay")
	}

	// An ordinary Charm (no mark) keeps the historic every-mode offer.
	a.AFlags = 0
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{}
	if !sc.s.modeFeasible(sc.k, &a, 0) {
		t.Fatal("an unmarked Charm mode lost its every-mode offer")
	}
}
