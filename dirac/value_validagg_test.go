package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestValidAgg: ExValidAgg folds gorge's Count$Valid $<prop> suffixes over
// the battlefield matches: extremes (0 with no match), sums, the distinct
// power set and the colours among the matches.
func TestValidAgg(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 2}, {Name: "Seedglaive Mentor", Count: 1}, {Name: "Plains", Count: 8}})
	vf, sw := id["Valley Floodcaller"], id["Seedglaive Mentor"]
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	agg := func(n int16) int {
		x := kop.Expr{Kind: kop.ExValidAgg, N: n, F: f}
		return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	if agg(kop.AggGreatestPower) != 0 || agg(kop.AggColors) != 0 || agg(kop.AggSumMV) != 0 {
		t.Fatal("no match must read 0")
	}
	sc.perm(vf, 0)
	sc.perm(vf, 0)
	sc.perm(sw, 0)
	sc.perm(sw, 1) // the opponent's does not match YouCtrl
	cv, cs := sc.k.t.Cards[vf], sc.k.t.Cards[sw]
	mv := func(c kop.CardDef) int { return int(c.Cost.MV()) }
	if got, want := agg(kop.AggGreatestPower), max(int(cv.Power), int(cs.Power)); got != want {
		t.Fatalf("greatest power %d, want %d", got, want)
	}
	if got, want := agg(kop.AggGreatestToughness), max(int(cv.Toughness), int(cs.Toughness)); got != want {
		t.Fatalf("greatest toughness %d, want %d", got, want)
	}
	if got, want := agg(kop.AggGreatestMV), max(mv(cv), mv(cs)); got != want {
		t.Fatalf("greatest mv %d, want %d", got, want)
	}
	if got, want := agg(kop.AggSumToughness), 2*int(cv.Toughness)+int(cs.Toughness); got != want {
		t.Fatalf("total toughness %d, want %d", got, want)
	}
	if got, want := agg(kop.AggSumMV), 2*mv(cv)+mv(cs); got != want {
		t.Fatalf("total mv %d, want %d", got, want)
	}
	wantD := 2
	if cv.Power == cs.Power {
		wantD = 1
	}
	if got := agg(kop.AggDistinctPower); got != wantD {
		t.Fatalf("distinct powers %d, want %d", got, wantD)
	}
	wantC := 0
	for c := cv.Colors | cs.Colors; c != 0; c &= c - 1 {
		wantC++
	}
	if got := agg(kop.AggColors); got != wantC {
		t.Fatalf("colors %d, want %d", got, wantC)
	}
}

// TestValidAggCreatureTypes: the distinct creature types among the matches, not a
// subtype of another kind, and every creature type once a Changeling matches.
func TestValidAggCreatureTypes(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // a table with Exprs prepares the gen tables
	tbl.Subtypes = append(append([]string(nil), vanilla.Table.Subtypes...), "Bear", "Elf", "Equipment")
	n := len(vanilla.Table.Subtypes)
	tbl.TypeVocab = make([]uint8, 50)
	subs := func(is ...int) (m kop.SubMask) {
		for _, i := range is {
			m = m.With(n + i)
		}
		return m
	}
	bear, elfBear, gear, shifter := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2), CardID(len(tbl.Cards)+3)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 1, Toughness: 1, Subtypes: subs(0)},
		kop.CardDef{Name: "Test Elf Bear", Types: kop.TCreature, Power: 1, Toughness: 1, Subtypes: subs(0, 1)},
		kop.CardDef{Name: "Test Gear", Types: kop.TCreature | kop.TArtifact, Power: 1, Toughness: 1, Subtypes: subs(2)},
		kop.CardDef{Name: "Test Shifter", Types: kop.TCreature, Power: 1, Toughness: 1, AllCreatureTypes: true})
	tbl.Filters = append(append([]kop.Filter(nil), vanilla.Table.Filters...), kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	count := func() int {
		x := kop.Expr{Kind: kop.ExValidAgg, N: kop.AggCreatureTypes, F: f}
		return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	if got := count(); got != 0 {
		t.Fatalf("no match reads %d", got)
	}
	sc.perm(bear, 0)
	sc.perm(gear, 0)
	if got := count(); got != 1 {
		t.Fatalf("a Bear and an Equipment read %d, want 1", got)
	}
	sc.perm(elfBear, 0)
	if got := count(); got != 2 {
		t.Fatalf("Bear and Elf read %d, want 2", got)
	}
	sc.perm(shifter, 1)
	if got := count(); got != 2 {
		t.Fatalf("the opponent's Changeling moved the count to %d", got)
	}
	sc.perm(shifter, 0)
	if got := count(); got != len(tbl.TypeVocab) {
		t.Fatalf("a Changeling reads %d, want the vocabulary's %d", got, len(tbl.TypeVocab))
	}
}
