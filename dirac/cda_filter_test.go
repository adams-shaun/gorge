package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A filter's P/T predicate reads a characteristic-defining P/T off the
// battlefield (CR 208.2a, 604.3; gorge): FDN pair 864, Squad
// Rallier's "power 2 or less" look must not take a Consuming Aberration
// whose CDA reads 3 cards in its owner's opponent's graveyard. On the stack
// an {X} spell's mana value includes the chosen X (CR 202.3e; gorge).
func TestFilterCDAOffBattlefield(t *testing.T) {
	deck := []cardset.Entry{{Name: "Consuming Aberration", Count: 1}, {Name: "Island", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	ab := id["Consuming Aberration"]
	o := sc.hand(ab, 0)
	le := func(n int16) bool {
		fl := kop.Filter{Pred: kop.PredPowerLE, PredN: n}
		return sc.s.matchCardOne(sc.k, &fl, ab, 0, 0, o)
	}
	if !le(0) {
		t.Fatalf("Consuming Aberration in hand with empty graveyards: power > 0")
	}
	for i := 0; i < 3; i++ {
		sc.s.place(sc.obj(id["Island"], 1), kop.ZoneGraveyard, false)
	}
	if le(2) || !le(3) {
		t.Fatalf("Consuming Aberration in hand, 3 cards in the opponent's graveyard: power-2 filter %v, power-3 filter %v; want false, true", le(2), le(3))
	}
	sc.s.objs[o].zone = kop.ZoneStack
	sc.s.push(stackEntry{kind: seSpell, obj: o, ctrl: 0, card: ab, trigObj: NoObj, x: 4})
	if got := sc.s.stackXMV(o, 2); got != 8 {
		t.Fatalf("XX spell on the stack with X=4 adds %d mana value, want 8", got)
	}
}
