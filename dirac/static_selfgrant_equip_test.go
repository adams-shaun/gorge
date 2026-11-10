package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSelfGrantedEquipHostedOnItsOwnPermanent: Domesticated Watercourse's
// Animate makes it an Equipment artifact with equip {2} while it keeps its
// abilities. The override record that grants the latent Equip ability lies on
// the Watercourse and comes from its own card, so the kernel hosts the
// activation on the animated permanent itself (s.selfGrantHost): gorge offers
// it once the Animate resolves and the {2} is payable, and the activation
// attaches the Watercourse to the creature it targets.
func TestSelfGrantedEquipHostedOnItsOwnPermanent(t *testing.T) {
	set := cardsettest.Set(t, "watercourse")
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
		t.Fatalf("%s not in the watercourse pool", name)
		return 0
	}
	sc := newScenTableLib(t, tbl, id("Island"), Config{NoElide: true})
	s := sc.s
	wc := sc.perm(id("Domesticated Watercourse"), 0)
	bears := sc.perm(id("Grizzly Bears"), 0)
	wsl, bsl := sc.slot(wc), sc.slot(bears)
	if !sc.onBF(wc) || !sc.onBF(bears) {
		t.Fatal("precondition: the Watercourse and Grizzly Bears are on the battlefield")
	}
	if p, tt := s.pt(sc.k, bsl); p != 2 || tt != 2 {
		t.Fatalf("precondition: Grizzly Bears %d/%d, want 2/2", p, tt)
	}

	// Before the Animate the Watercourse grants no activation: {2} in the
	// pool buys nothing without the latent Equip.
	s.pl[0].pool[kop.C] = 2
	if sc.grantedActOffered(wc) {
		t.Fatal("precondition: the Watercourse offers an equip before its Animate")
	}
	// The Animate costs {U}{B}; floating both proves the activation is priced
	// by the pool, and resolveAll empties the stack.
	s.pl[0].pool[kop.C] = 0
	s.pl[0].pool[kop.U] = 1
	s.pl[0].pool[kop.B] = 1
	d := s.Step()
	animated := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == wc && a.Mode()&0x80 == 0 {
			s.Apply(a)
			animated = true
			break
		}
	}
	if !animated {
		t.Fatalf("the Animate activation is not offered: %v", d.Options[:d.N])
	}
	sc.resolveAll(t)
	if s.typesGen(sc.k, wsl)&kop.TEquipment == 0 {
		t.Fatal("after the Animate the Watercourse is not an Equipment")
	}

	// Float {2}: gorge's granted-keyword offer prices the real pool
	// (rules.offerCastable, no hypothetical source bound), so this is the
	// ordinary price the kernel's Config.GorgeUnpriced gate mirrors.
	s.pl[0].pool[kop.C] = 2
	if !sc.grantedActOffered(wc) {
		t.Fatal("the self-granted equip is not offered on the animated Watercourse")
	}
	d = s.Step()
	equipped := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == wc && Obj(a.Target()) == wc && a.Mode()&0x80 != 0 {
			s.Apply(a)
			equipped = true
			break
		}
	}
	if !equipped {
		t.Fatalf("the self-granted equip activation is not offered: %v", d.Options[:d.N])
	}
	d = s.Step()
	targeted := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkTarget && Obj(a.Target()) == bears {
			s.Apply(a)
			targeted = true
			break
		}
	}
	if !targeted {
		t.Fatalf("Grizzly Bears is not offered as the equip target: kind %d %v", d.Kind, d.Options[:d.N])
	}
	sc.resolveAll(t)
	if got := s.perms[wsl].attached; got != bsl {
		t.Fatalf("the Watercourse is attached to slot %d, want Grizzly Bears at %d", got, bsl)
	}

	// The Animate lasts until end of turn: once it ends the Watercourse stops
	// being an Equipment, so the attachment is illegal and SBA unattaches it
	// (CR 704.5p), as gorge's attachmentSBAs does.
	for i := 0; i < 80 && s.step != kop.StepCleanup; i++ {
		d := s.Step()
		if d.Kind == DkPriority {
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		s.Apply(d.Options[0])
	}
	if s.step != kop.StepCleanup {
		t.Fatal("precondition: the turn never reached its cleanup step")
	}
	s.Step()
	if got := s.perms[wsl].attached; got != NoSlot {
		t.Fatalf("the Watercourse is still attached to slot %d after its Animate ended", got)
	}
}
