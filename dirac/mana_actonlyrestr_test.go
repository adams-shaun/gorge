package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// actOnlyScen: the std-dft synth table holding Boommobile (a resolving
// activated-only mana source), a plain land and a creature, and the marker
// filter of Boommobile's colour choice.
func actOnlyScen(t *testing.T) (sc *scen, id map[string]CardID, f kop.FilterID) {
	t.Helper()
	sc, id = synthScen(t, "std-dft", "Boommobile", "Plains", "Mountain", "Tune Up")
	if _, ok := id["Boommobile"]; !ok {
		t.Skip("Boommobile is not in the std-dft table")
	}
	for _, a := range sc.k.t.Cards[id["Boommobile"]].Abilities {
		for i := range a.Prog {
			if in := &a.Prog[i]; in.Op == kop.OpExt && in.A.N == int16(kop.ExtManaChoice) {
				f = in.F
			}
		}
	}
	if f == 0 || sc.k.t.Filters[f].Special != kop.SpmActivated {
		t.Fatalf("Boommobile's colour choice F = %d, want the SpmActivated marker filter", f)
	}
	return sc, id, f
}

// TestActOnlyRestrPool: a resolving DB$ Mana restricted to activated
// abilities (the SpmActivated marker filter, Boommobile) floats as restricted
// mana that pays an activated ability from any source, pays no spell and no
// non-spell payment, and is typed (poolArtRestr) when an artifact made it.
func TestActOnlyRestrPool(t *testing.T) {
	sc, id, f := actOnlyScen(t)
	s, k := sc.s, sc.k
	if got := k.gen.restrF; got[0] != f && got[1] != f && got[2] != f && got[3] != f {
		t.Fatalf("the marker filter %d is not a registered restriction %v", f, got)
	}
	if !k.artMana || !k.cards[id["Boommobile"]].manaTyped || k.cards[id["Plains"]].manaTyped {
		t.Fatalf("precondition: artMana %v, Boommobile typed %v, Plains typed %v; want true, true, false", k.artMana, k.cards[id["Boommobile"]].manaTyped, k.cards[id["Plains"]].manaTyped)
	}

	// a resolving add from a non-artifact source floats the batch untyped
	s.res.e.card = id["Plains"]
	s.manaChoiceAdd(k, 0, int(kop.R), 4, f)
	if s.restrN(0) != 4 || s.g.poolArt[0]&poolArtRestr != 0 {
		t.Fatalf("non-artifact add: restricted %d, typed %v; want 4 untyped", s.restrN(0), s.g.poolArt[0]&poolArtRestr != 0)
	}
	spell := payDesc{class: pdSpell, card: id["Tune Up"], src: NoSlot}
	act := payDesc{class: pdAct, card: id["Boommobile"], src: NoSlot}
	if m := s.restrPoolAct(k, 0, &act); m == 0 {
		t.Error("the marker batch does not pay an activated ability")
	}
	if m := s.restrSpendable(k, 0, id["Tune Up"], spell.og); m != 0 {
		t.Errorf("the marker batch pays a non-artifact spell: mask %b", m)
	}
	if m := s.restrSpendable(k, 0, id["Boommobile"], spell.og); m != 0 {
		t.Errorf("the marker batch pays an artifact spell: mask %b", m)
	}
	if m := s.restrPoolNonSpell(k, 0); m != 0 {
		t.Errorf("the marker batch pays a non-spell payment: mask %b", m)
	}
	if m := s.restrFloatAdmits(k, 0, &spell); m != 0 {
		t.Errorf("restrFloatAdmits(spell) = %b, want 0", m)
	}
	if m := s.restrFloatAdmits(k, 0, &act); m == 0 {
		t.Error("restrFloatAdmits(activated) = 0, want the batch")
	}

	// an artifact source tags the pool typed
	var s2 State
	s2.res.e.card = id["Boommobile"]
	s2.manaChoiceAdd(k, 1, int(kop.U), 4, f)
	if s2.restrN(1) != 4 || s2.g.poolArt[1]&poolArtRestr == 0 {
		t.Fatalf("artifact add: restricted %d, typed %v; want 4 typed", s2.restrN(1), s2.g.poolArt[1]&poolArtRestr != 0)
	}
	if !s2.rbTyped(1) {
		t.Error("an artifact's restricted units are not spent typed-order")
	}
}

// TestActOnlyRestrOffersActivation: four activated-only red units floating pay
// Boommobile's {2}{R}{X} exhaust ability (X = 1) and nothing else: the
// activation is offered, a spell in hand is not. Under gorge's pricing an {X}
// activation is offered only when the floating pool pays it, the restricted
// units the activation may spend included (poolPaysAct).
func TestActOnlyRestrOffersActivation(t *testing.T) {
	sc, id, f := actOnlyScen(t)
	s, k := sc.s, sc.k
	s.cfg |= cfgGorgeUnpriced // gorge offers an {X} activation only when the floating pool pays it
	boom := sc.perm(id["Boommobile"], 0)
	for i := 0; i < 4; i++ {
		sc.s.perms[sc.slot(sc.perm(id["Mountain"], 0))].flags |= pfTapped
	}
	spell := sc.hand(id["Tune Up"], 0)
	s.res.e.card = id["Boommobile"]
	s.manaChoiceAdd(k, 0, int(kop.R), 4, f)
	if s.g.poolArt[0]&poolArtRestr == 0 {
		t.Fatal("precondition: the artifact's units are not typed")
	}
	d := s.Step()
	acts := 0
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkActivate && a.Source() == TCode(uint8(boom)) {
			acts++
		}
	}
	if acts == 0 {
		t.Fatalf("no activation of Boommobile offered on four activated-only red units: %v", d.Options[:d.N])
	}
	if castOffered(&d, spell) {
		t.Fatalf("a spell is offered on activated-only units: %v", d.Options[:d.N])
	}
}
