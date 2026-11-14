package dirac

import (
	"fmt"
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Twobrid mana (mana_twobrid.go; CR 107.4e) on the generated twobrid
// pool: the mana value, the per-pip face announcement, the paths that must
// never see an unresolved pip, and the cost-modifier stall.

func twobridScen(t *testing.T, names ...string) (*scen, map[string]CardID, *kop.Table) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "twobrid")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the twobrid table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	return sc, id, tbl
}

// TestTwobridMV: two per pip (CR 202.3f); the marker never reaches Resolve.
func TestTwobridMV(t *testing.T) {
	_, id, tbl := twobridScen(t, "Spectral Procession", "Karn, Gilded Guardian", "Defibrillating Current")
	for name, want := range map[string]int{"Spectral Procession": 6, "Karn, Gilded Guardian": 10, "Defibrillating Current": 6} {
		if got := tbl.Cards[id[name]].Cost.MV(); got != want {
			t.Errorf("%s: mana value %d, want %d", name, got, want)
		}
	}
	if n := unsafe.Sizeof(kop.ManaCost{}); n != 18 { // XPip (16 before); a table field, not State
		t.Errorf("sizeof(ManaCost) = %d, want 18", n)
	}
	c := tbl.Cards[id["Defibrillating Current"]].Cost
	if c.Tb == 0 || c.Hyb[0] != kop.HybTwobrid || c.Hyb[1] != 0 || c.TbN() != 3 {
		t.Fatalf("Defibrillating Current cost %+v", c)
	}
	if c.TbCol(0) != kop.R || c.TbCol(1) != kop.W || c.TbCol(2) != kop.B {
		t.Errorf("pips %d %d %d, want R W B in printed order", c.TbCol(0), c.TbCol(1), c.TbCol(2))
	}
	r := c.ResolveTb(true)
	if r.Generic != 2 || r.TbN() != 2 || r.Hyb[0] != kop.HybTwobrid {
		t.Errorf("generic face of pip 0: %+v", r)
	}
	r = r.ResolveTb(false).ResolveTb(false)
	if r.Hyb[0] != 0 || r.Tb != 0 || r.Pips[kop.W] != 1 || r.Pips[kop.B] != 1 || r.MV() != 4 {
		t.Errorf("fully resolved: %+v", r)
	}
	defer func() {
		if recover() == nil {
			t.Error("Resolve accepted the twobrid marker")
		}
	}()
	c.Resolve(7)
}

// pipAsks casts o and answers every pip ask with pick(asked modes), returning
// each ask's offered modes.
func pipAsks(t *testing.T, sc *scen, o Obj, pick func(i int, modes []uint8) uint8) [][]uint8 {
	t.Helper()
	sc.do(t, AkCast, TCode(o), scAny)
	var asks [][]uint8
	for {
		d := sc.s.Step()
		if d.Kind != DkChoose || d.Sub != ChGenPip {
			return asks
		}
		var modes []uint8
		for i := 0; i < int(d.N); i++ {
			modes = append(modes, d.Options[i].Mode())
		}
		asks = append(asks, modes)
		sc.s.Apply(MakeAction(AkChoose, 0, 0, pick(len(asks)-1, modes)))
	}
}

// TestTwobridAnnounce mirrors gorge's TestSpectralProcessionTwobridPayments:
// a face is offered only when the whole cost stays payable with it, the
// colour first, then the generic face (mode 6).
func TestTwobridAnnounce(t *testing.T) {
	first := func(_ int, m []uint8) uint8 { return m[0] }
	cases := []struct {
		name string
		pool [6]uint8
		want [][]uint8
	}{
		{"WWW: only the colour each pip", [6]uint8{kop.W: 3}, [][]uint8{{kop.W}, {kop.W}, {kop.W}}},
		{"six colourless: only the generic face", [6]uint8{kop.C: 6}, [][]uint8{{6}, {6}, {6}}},
		{"W plus four colourless: both on pip 0, then only generic", [6]uint8{kop.W: 1, kop.C: 4}, [][]uint8{{kop.W, 6}, {6}, {6}}},
	}
	for _, c := range cases {
		sc, id, _ := twobridScen(t, "Spectral Procession")
		o := sc.hand(id["Spectral Procession"], 0)
		sc.s.pl[0].pool = c.pool
		got := pipAsks(t, sc, o, first)
		if fmt.Sprint(got) != fmt.Sprint(c.want) {
			t.Errorf("%s: asks %v, want %v", c.name, got, c.want)
		}
		if sc.s.pl[0].pool != ([6]uint8{}) {
			t.Errorf("%s: pool after the cast %v, want empty", c.name, sc.s.pl[0].pool)
		}
	}
	// answering the generic face first leaves W plus two colourless
	sc, id, _ := twobridScen(t, "Spectral Procession")
	o := sc.hand(id["Spectral Procession"], 0)
	sc.s.pl[0].pool = [6]uint8{kop.W: 1, kop.C: 4}
	got := pipAsks(t, sc, o, func(i int, m []uint8) uint8 { return m[len(m)-1] })
	if fmt.Sprint(got) != fmt.Sprint([][]uint8{{kop.W, 6}, {kop.W, 6}, {kop.W}}) {
		t.Errorf("generic face first: asks %v", got)
	}
}

// TestTwobridNoWitness: no kicker, alternative or additional cost, and no
// ability cost of the pool carries a twobrid pip (addCost never composes
// one); the hybrid witness refuses the marker; and a cast paid from lands
// (the planner taps for the resolved cost) completes.
func TestTwobridNoWitness(t *testing.T) {
	sc, id, tbl := twobridScen(t, "Spectral Procession", "Defibrillating Current", "Karn, Gilded Guardian")
	for i := range tbl.Cards {
		c := &tbl.Cards[i]
		for _, m := range []kop.ManaCost{c.Kicker, c.AltCost, c.WarpCost, c.PlotCost, c.SneakCost, c.MorphCost, c.AltAddCost.Mana, c.SpellCost.Mana} {
			if m.Tb != 0 || m.Hyb[0] == kop.HybTwobrid {
				t.Errorf("%s: a composed cost carries twobrid pips: %+v", c.Name, m)
			}
		}
		for j := range c.Abilities {
			if m := c.Abilities[j].Cost.Mana; m.Tb != 0 || m.Hyb[0] == kop.HybTwobrid {
				t.Errorf("%s ability %d: a twobrid ability cost", c.Name, j)
			}
		}
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("hybWitness accepted a twobrid cost")
			}
		}()
		var av avail
		av.hybWitness(tbl.Cards[id["Spectral Procession"]].Cost, 0)
	}()
	// six Plains: the planner taps for the announced faces
	o := sc.hand(id["Spectral Procession"], 0)
	for i := 0; i < 6; i++ {
		sc.perm(id["Plains"], 0)
	}
	asks := pipAsks(t, sc, o, func(_ int, m []uint8) uint8 { return m[0] })
	if len(asks) != 3 {
		t.Fatalf("asks %v", asks)
	}
	if sc.s.pl[0].pool != ([6]uint8{}) {
		t.Errorf("pool after the cast %v, want empty", sc.s.pl[0].pool)
	}
}

// TestTwobridTable: a table with a twobrid card is flagged, and a cost
// reduction beside it stalls nothing (the statics compose on the announced
// faces, foldTb).
func TestTwobridTable(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 10}, {Name: "Spectral Procession", Count: 10}}
	set := cardsettest.Set(t, "twobrid")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if !tableTwobrid(tbl) {
		t.Fatal("a table with a twobrid card not flagged")
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Reducer", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatReduceCost, A: 1}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if r := sc.s.StallReason(); r != StallNone {
		t.Fatalf("stall reason %d, want none", r)
	}
}

// twobridReduced is twobridScen with a permanent that reduces every spell
// by one generic mana on seat 0's battlefield.
func twobridReduced(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "twobrid")
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Reducer", Types: kop.TEnchantment, Abilities: []kop.Ability{
		{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatReduceCost, A: 1}}}})
	id["Test Reducer"] = CardID(len(tbl.Cards) - 1)
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	sc.perm(id["Test Reducer"], 0)
	return sc, id
}

// TestTwobridReducedGenericFaces: a generic reduction applies to the cost the
// announced faces made (CR 601.2b, then 601.2f), where the unresolved {2/W}
// has no generic for it to take. Three generic faces under a {1} reduction
// are {5}: five colourless mana cast Spectral Procession, the first pip
// offering only its generic face ({W} cannot be paid).
func TestTwobridReducedGenericFaces(t *testing.T) {
	sc, id := twobridReduced(t, "Spectral Procession")
	o := sc.hand(id["Spectral Procession"], 0)
	sc.s.pl[0].pool = [6]uint8{kop.C: 5}
	asks := pipAsks(t, sc, o, func(_ int, m []uint8) uint8 { return m[0] })
	if fmt.Sprint(asks) != fmt.Sprint([][]uint8{{6}, {6}, {6}}) {
		t.Errorf("asks %v, want the generic face each pip", asks)
	}
	if sc.s.pl[0].pool != ([6]uint8{}) {
		t.Errorf("pool after the cast %v, want empty", sc.s.pl[0].pool)
	}
}

// TestTwobridReducedLaterPips: a pip's colour is offered when some completion
// of the later pips makes the reduced total payable (the announcement walks
// the completions as gorge FeasibleAny does). {2/W}{2/W}{2/W} under a {1}
// reduction with {W}{C}{C}{C}: the colour face on pip 0 and two generic faces
// cost {W}{4}-{1}, so pip 0 offers both faces.
func TestTwobridReducedLaterPips(t *testing.T) {
	sc, id := twobridReduced(t, "Spectral Procession")
	o := sc.hand(id["Spectral Procession"], 0)
	sc.s.pl[0].pool = [6]uint8{kop.W: 1, kop.C: 3}
	asks := pipAsks(t, sc, o, func(_ int, m []uint8) uint8 { return m[0] })
	if len(asks) != 3 || fmt.Sprint(asks[0]) != fmt.Sprint([]uint8{kop.W, 6}) {
		t.Errorf("asks %v, want {W,6} on pip 0", asks)
	}
}
