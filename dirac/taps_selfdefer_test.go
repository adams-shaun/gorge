package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsWatchClassSelfDefer pins the kernel's class walk over a source's own
// Taps line: Zhur-Taa Druid's self TapsForMana line defers the source (gorge's
// planner reads Mode$ TapsForMana as no vetted payer damage and defers it),
// while City of Brass's self Pain line stays the pain-only mana source
// prepSelfTapsPain prices, not a Defer-class watcher.
func TestTapsWatchClassSelfDefer(t *testing.T) {
	t.Run("self TapsForMana defers its source", func(t *testing.T) {
		set := cardsettest.Set(t, "tapsformana")
		deck := []cardset.Entry{{Name: "Zhur-Taa Druid", Count: 1}, {Name: "Forest", Count: 1}}
		tbl, da, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		zhur, forest := id["Zhur-Taa Druid"], id["Forest"]
		self := false
		for i := range tbl.Cards[zhur].Abilities {
			a := &tbl.Cards[zhur].Abilities[i]
			if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigTaps && a.Trig.Flags&kop.TfSelf != 0 &&
				a.Trig.TapPlan == kop.TapPlanDefer {
				self = true
			}
		}
		if !self {
			t.Fatal("precondition: Zhur-Taa Druid has no self Defer TapsForMana line")
		}
		sc := newScenTableLib(t, tbl, forest, Config{})
		if !sc.k.gen.anyTapsDefer || !sc.k.gen.has(zhur, gfTapsDefer) {
			t.Fatalf("anyTapsDefer %v, Zhur-Taa gfTapsDefer %v: want both", sc.k.gen.anyTapsDefer, sc.k.gen.has(zhur, gfTapsDefer))
		}
		sl := sc.slot(sc.perm(zhur, 0))
		pain, deferSrc := sc.s.tapsWatchClass(sc.k, 0, sl)
		if !deferSrc {
			t.Fatal("the source's own Defer TapsForMana line did not defer it")
		}
		if pain != 0 {
			t.Errorf("pain %d, want 0 (the line damages opponents, not the payer)", pain)
		}
	})

	t.Run("self pain line stays with prepSelfTapsPain", func(t *testing.T) {
		set := cardsettest.Set(t, "citybrass")
		deck := []cardset.Entry{{Name: "City of Brass", Count: 1}, {Name: "Plains", Count: 1}}
		tbl, da, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		city, plains := id["City of Brass"], id["Plains"]
		if n, ok := selfTapsPainN(&tbl.Cards[city]); !ok || n == 0 {
			t.Fatalf("precondition: City of Brass self Pain line = (%d, %v)", n, ok)
		}
		sc := newScenTableLib(t, tbl, plains, Config{})
		if pc := &sc.k.cards[city]; !pc.painOnly || pc.painN == 0 {
			t.Fatalf("precondition: City of Brass prepared painOnly %v painN %d, want its priced Pain line", pc.painOnly, pc.painN)
		}
		sl := sc.slot(sc.perm(city, 0))
		if _, deferSrc := sc.s.tapsWatchClass(sc.k, 0, sl); deferSrc {
			t.Fatal("the source's own Pain line classed it Defer: prepSelfTapsPain is the Pain arm")
		}
	})
}
