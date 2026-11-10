package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// irencragDrive answers decisions until the stack is empty and priority is
// back: yes to a may, else the first option.
func (sc *scen) irencragDrive(t *testing.T) {
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
			if a := d.Options[j]; a.Kind() == AkChoose && a.Mode() == 1 && d.Options[0].Kind() == AkChoose && d.N == 2 {
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
// loss: attached to Grizzly Bears it makes them 5/5, while the {T}: Add {C}
// is gone with the loss. Once The Irencrag leaves, the record ends and the
// pump with it. Everflame is attached directly, as gorge's own
// TestIrencragEverflamePumpsEquippedCreature does: gorge offers no equip
// activation beside the same Animate's RemoveAllAbilities$, so neither does
// the kernel (a granted activation needs an attached host).
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
	sc.irencragDrive(t)
	if !s.genLost(sc.k, isl) {
		t.Fatal("The Irencrag did not lose its abilities (the trigger was not answered yes)")
	}
	if s.typesGen(sc.k, isl)&kop.TEquipment == 0 {
		t.Fatal("Everflame is not an Equipment")
	}

	s.pl[0].pool[kop.W] = 3
	if sc.grantedActOffered(ir) {
		t.Fatal("Everflame's granted equip is offered: gorge offers none beside the same Animate's RemoveAllAbilities$")
	}
	s.pl[0].pool[kop.W] = 0

	s.attachTo(sc.k, isl, bsl)
	if s.perms[isl].attached != bsl {
		t.Fatalf("precondition: Everflame is not attached to Grizzly Bears (attached to %d)", s.perms[isl].attached)
	}
	if p, tt := s.pt(sc.k, bsl); p != 5 || tt != 5 {
		t.Fatalf("equipped Grizzly Bears %d/%d, want 5/5 (the granted pump died with its own loss of abilities)", p, tt)
	}

	s.leave(sc.k, isl, kop.ZoneHand, false)
	if p, tt := s.pt(sc.k, bsl); p != 2 || tt != 2 {
		t.Fatalf("Grizzly Bears %d/%d after The Irencrag left, want 2/2", p, tt)
	}
}

// grantedActOffered reports whether the next decision offers a granted
// activation (mode 0x80|ab) of o hosted by o itself.
func (sc *scen) grantedActOffered(o Obj) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == o && Obj(a.Target()) == o && a.Mode()&0x80 != 0 {
			return true
		}
	}
	return false
}
