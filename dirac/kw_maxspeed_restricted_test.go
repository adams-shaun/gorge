package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMaxSpeedGraveOfferAdmitsActivatedOnlyMana: a max-speed graveyard
// ability is offered out of the floating pool alone (kw_maxspeed.go), and
// that pool includes a restricted batch an activated-ability payment admits
// (Boommobile's "spend this mana only to activate abilities", CR 106.6). The
// gate must read poolPaysAct, not the ordinary pool: otherwise the ability is
// withheld while gorge offers it.
func TestMaxSpeedGraveOfferAdmitsActivatedOnlyMana(t *testing.T) {
	sc, id := synthScen(t, "std-dft", "Boommobile", "Leonin Surveyor", "Tune Up", "Plains")
	s, k := sc.s, sc.k
	var f kop.FilterID
	for _, a := range k.t.Cards[id["Boommobile"]].Abilities {
		for i := range a.Prog {
			if in := &a.Prog[i]; in.Op == kop.OpExt && in.A.N == int16(kop.ExtManaChoice) {
				f = in.F
			}
		}
	}
	if f == 0 || k.t.Filters[f].Special != kop.SpmActivated {
		t.Fatalf("Boommobile's colour choice F = %d, want the SpmActivated marker filter", f)
	}
	var ab *kop.Ability
	for i := range k.t.Cards[id["Leonin Surveyor"]].Abilities {
		a := &k.t.Cards[id["Leonin Surveyor"]].Abilities[i]
		if a.Kind == kop.AbActivated && a.ActZone == kop.ZoneGraveyard {
			ab = a
			break
		}
	}
	if ab == nil || ab.Gate.Ctr != kop.CtrMaxSpeed {
		t.Fatalf("Leonin Surveyor's graveyard ability: %+v, want a max-speed gate", ab)
	}
	o := sc.obj(id["Leonin Surveyor"], 0)
	s.g.speed = 0
	s.speedAdd(0, speedMaxV)
	if s.poolPays(0, &ab.Cost.Mana) {
		t.Fatal("precondition: the ordinary pool pays the ability")
	}
	if s.maxSpeedGraveOK(k, 0, o, ab) {
		t.Fatal("the ability is offered on an empty pool")
	}
	s.res.e.card = id["Boommobile"]
	s.manaChoiceAdd(k, 0, int(kop.G), 4, f)
	if s.restrN(0) != 4 || s.poolPays(0, &ab.Cost.Mana) {
		t.Fatalf("precondition: restricted %d, ordinary pool pays %v; want 4 and false", s.restrN(0), s.poolPays(0, &ab.Cost.Mana))
	}
	if !s.maxSpeedGraveOK(k, 0, o, ab) {
		t.Error("the ability is withheld though four activated-only units float and pay its {3}")
	}
}
