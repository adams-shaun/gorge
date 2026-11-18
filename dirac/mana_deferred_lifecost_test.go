package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A deferred source that charges life ("{T}, Pay 2 life": Boseiju) is a
// scripted-prefix gap only while its payer stays above 0 life: gorge's prefix
// search drops a branch that ends the game (potential_script.go visit), so no
// play only a lethal payment funds is offered. The mana window still lists the
// ability at exactly the owed life (CR 119.4: the payment is legal).
// Pyromancer's Goggles stands in with a "Pay 2 life" cost added to its {R}.
func TestDeferredLifeCostGapNeedsSurvival(t *testing.T) {
	deck := []cardset.Entry{{Name: "Pyromancer's Goggles", Count: 1}, {Name: "Shock", Count: 1}, {Name: "Mountain", Count: 20}}
	tbl, da, _, err := cardsettest.Set(t, "goggles").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]CardID{}
	for _, c := range da {
		ids[tbl.Cards[c].Name] = c
	}
	rewriteMana(tbl, ids["Pyromancer's Goggles"], func(a *kop.Ability) {
		a.Cost.Flags |= kop.CostPayLife
		a.Cost.N = 2
	})
	sc := newScenTableLib(t, tbl, ids["Mountain"], Config{NoElide: true})
	g := sc.slot(sc.perm(ids["Pyromancer's Goggles"], 0))
	s, k := sc.s, sc.k
	if !k.gen.has(s.cardAt(g), gfDeferred) || k.deferAb(s.cardAt(g)).Cost.Flags&kop.CostPayLife == 0 {
		t.Fatal("precondition: the Goggles are not a deferred source with a life cost")
	}
	for _, c := range []struct {
		life              int16
		script, window    bool
		census, filterOtr bool
	}{
		{3, true, true, true, true},
		{2, false, true, false, false}, // payable (CR 119.4) but lethal: no script
		{1, false, false, false, false},
	} {
		s.pl[0].life = c.life
		if got := s.gapMask(k, 0, gfDeferred).has(g); got != c.script {
			t.Errorf("life %d: gapMask has the source = %v, want %v", c.life, got, c.script)
		}
		if got := s.scriptDeferred(k, 0).has(g); got != c.census {
			t.Errorf("life %d: scriptDeferred has the source = %v, want %v", c.life, got, c.census)
		}
		if got := s.deferLifeOK(k, 0, g); got != c.window {
			t.Errorf("life %d: deferLifeOK = %v, want %v", c.life, got, c.window)
		}
		if got := s.deferLifeSurvives(k, 0, g); got != c.filterOtr {
			t.Errorf("life %d: deferLifeSurvives = %v, want %v", c.life, got, c.filterOtr)
		}
	}
}
