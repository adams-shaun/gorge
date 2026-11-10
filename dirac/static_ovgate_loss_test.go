package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// irencragDrive answers decisions until the stack is empty and priority is
// back: yes to a may, tgt for a target, else the first option.
func (sc *scen) irencragDrive(t *testing.T, tgt Obj) {
	t.Helper()
	for i := 0; i < 80; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority && sc.s.sp == 0 && sc.s.npend == 0 {
			return
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		pick := d.Options[0]
		for j := 0; j < int(d.N); j++ {
			a := d.Options[j]
			if a.Kind() == AkTarget && Obj(a.Target()) == tgt {
				pick = a
				break
			}
			if a.Kind() == AkChoose && a.Mode() == 1 && d.Options[0].Kind() == AkChoose && d.N == 2 {
				pick = a
			}
		}
		sc.s.Apply(pick)
	}
	t.Fatal("the stack never emptied")
}

// TestOvGateStaticOutlivesOwnLoseAbilities (CR 613.1f, 613.7): The
// Irencrag's Animate makes it lose all other abilities and gain "Equipped
// creature gets +3/+3" in the same effect, so the granted pump survives the
// loss: equipped to Grizzly Bears it makes them 5/5, while the {T}: Add {C}
// is gone. Once The Irencrag leaves, the record ends and the pump with it.
func TestOvGateStaticOutlivesOwnLoseAbilities(t *testing.T) {
	set := cardsettest.Set(t, "irencrag")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := func(name string) CardID {
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == name {
				return CardID(i)
			}
		}
		t.Fatalf("%s not in the irencrag pool", name)
		return 0
	}
	sc := newScenTableLib(t, tbl, id("Plains"), Config{NoElide: true})
	s := sc.s
	ir := sc.perm(id("The Irencrag"), 0)
	bears := sc.perm(id("Grizzly Bears"), 0)
	for i := 0; i < 3; i++ {
		sc.perm(id("Plains"), 0)
	}
	bsl, isl := sc.slot(bears), sc.slot(ir)
	if p, tt := s.pt(sc.k, bsl); p != 2 || tt != 2 {
		t.Fatalf("precondition: Grizzly Bears %d/%d, want 2/2", p, tt)
	}
	if s.genLost(sc.k, isl) {
		t.Fatal("precondition: The Irencrag lost its abilities before the trigger")
	}

	isa := sc.obj(id("Isamaru, Hound of Konda"), 0)
	s.enter(sc.k, isa, 0, false, kop.ZoneNone)
	if s.npend == 0 {
		t.Fatal("precondition: a legendary creature entering fired no Irencrag trigger")
	}
	sc.irencragDrive(t, bears)
	if !s.genLost(sc.k, isl) {
		t.Fatal("The Irencrag did not lose its abilities (the trigger was not answered yes)")
	}

	if !sc.activateIrencragEquip() {
		t.Fatal("The Irencrag offers no Equip activation after the animation")
	}
	sc.irencragDrive(t, bears)
	if s.perms[isl].attached != bsl {
		t.Fatalf("Everflame is not attached to Grizzly Bears (attached to %d)", s.perms[isl].attached)
	}
	if p, tt := s.pt(sc.k, bsl); p != 5 || tt != 5 {
		t.Fatalf("equipped Grizzly Bears %d/%d, want 5/5 (the granted pump died with its own loss of abilities)", p, tt)
	}

	s.leave(sc.k, isl, kop.ZoneHand, false)
	if p, tt := s.pt(sc.k, bsl); p != 2 || tt != 2 {
		t.Fatalf("Grizzly Bears %d/%d after The Irencrag left, want 2/2", p, tt)
	}
}

// activateIrencragEquip activates The Irencrag's granted Equip when it is
// offered at the next decision.
func (sc *scen) activateIrencragEquip() bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkActivate && sc.s.objs[a.Source()].card == sc.idOf("The Irencrag") {
			sc.s.Apply(a)
			return true
		}
	}
	return false
}

func (sc *scen) idOf(name string) CardID {
	for i := range sc.k.t.Cards {
		if sc.k.t.Cards[i].Name == name {
			return CardID(i)
		}
	}
	return CardID(len(sc.k.t.Cards))
}
