package feat

import (
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// tyPool is a pool over a small FDN matchup whose specs spell real type
// lines, one of them with a printed subtype the table does not carry
// ("Spirit"), so a derived line keeps it as extra.
func tyPool(t *testing.T) (*Pool, *kop.Table, []kop.CardID, []kop.CardID) {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	tbl, da, db, err := set.Matchup(
		[]cardset.Entry{{Name: "Mountain", Count: 20}, {Name: "Healer's Hawk", Count: 20}},
		[]cardset.Entry{{Name: "Plains", Count: 20}, {Name: "Healer's Hawk", Count: 20}},
	)
	if err != nil {
		t.Fatal(err)
	}
	// the pool's table names subtypes (the matchup references none)
	pt := *tbl
	pt.Subtypes = []string{"Bird", "Cleric", "Soldier", "Time Lord"}
	specs := make([]CardSpec, len(tbl.Cards))
	for i := range specs {
		specs[i] = CardSpec{Name: tbl.Cards[i].Name, Types: "Creature Bird Spirit"}
	}
	p := NewPool(&pt, specs)
	p.SetCreatureTypes([]string{"Advisor", "Bird", "Time Lord"})
	return p, tbl, da, db
}

// tyShape is the i-th of a family of type-line shapes no FDN override
// makes (two type changes and a subtype set at once).
type tyShape struct {
	types       kop.TypeMask
	subs        kop.SubMask
	all, setSub bool
}

func tyShapes(nsub int) []tyShape {
	typesets := [...]kop.TypeMask{
		kop.TLand | kop.TCreature, kop.TArtifact | kop.TCreature, kop.TEnchantment | kop.TCreature | kop.TLegendary,
		kop.TArtifact, kop.TLand | kop.TBasic, kop.TPlaneswalker | kop.TLegendary, kop.TBattle, kop.TKindred | kop.TInstant,
	}
	var out []tyShape
	for _, ty := range typesets {
		out = append(out, tyShape{types: ty, all: true})
		for i := 0; i < nsub; i++ {
			out = append(out, tyShape{types: ty, subs: kop.SubMask{}.With(i)})
			out = append(out, tyShape{types: ty, subs: kop.SubMask{}.With(i), setSub: true})
		}
	}
	return out
}

// refTypeLine is typeLine's line as develop 499097d9 built it (strings
// joined and re-split, the rows' prefixes concatenated): the exactness
// reference for fillTypeLine.
func refTypeLine(p *Pool, c *card, types kop.TypeMask, subs kop.SubMask, all, setSub bool) *typeView {
	types &= lineTypes
	if all {
		subs = kop.SubMask{}
	}
	extra := ""
	if !all && !setSub {
		extra = c.untab
	}
	var words []string
	for _, w := range lineTypeWords {
		if types&w.m != 0 {
			words = append(words, w.s)
		}
	}
	if all {
		words = append(words, p.allTypes...)
	} else {
		words = append(words, strings.Fields(extra)...)
		for i, n := range p.subNames {
			if subs.Has(i) {
				words = append(words, n)
			}
		}
	}
	line := strings.Join(words, " ")
	t := &typeView{types: types, subs: subs, all: all, extra: extra, words: words}
	t.fold, t.exact = typeMasks(line)
	t.creature = t.fold&tbCreature != 0
	t.land = t.fold&tbLand != 0
	for z, zn := range zcNames {
		for _, w := range words {
			t.typeRows[z] = append(t.typeRows[z], row(add(add(off, "mz|"+zn+"|type|"), w)))
		}
	}
	for _, w := range words {
		t.stackTy[0] = append(t.stackTy[0], row(add(add(off, "mz|stack|me|type|"), w)))
		t.stackTy[1] = append(t.stackTy[1], row(add(add(off, "mz|stack|opp|type|"), w)))
	}
	return p.unlessPrinted(t, c)
}

func sameView(a, b *typeView) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.types != b.types || a.subs != b.subs || a.all != b.all || a.extra != b.extra ||
		a.fold != b.fold || a.exact != b.exact || a.creature != b.creature || a.land != b.land ||
		strings.Join(a.words, "\x00") != strings.Join(b.words, "\x00") || len(a.words) != len(b.words) {
		return false
	}
	eq := func(x, y []uint16) bool {
		if len(x) != len(y) {
			return false
		}
		for i := range x {
			if x[i] != y[i] {
				return false
			}
		}
		return true
	}
	return eq(a.typeRows[0], b.typeRows[0]) && eq(a.typeRows[1], b.typeRows[1]) && eq(a.typeRows[2], b.typeRows[2]) &&
		eq(a.stackTy[0], b.stackTy[0]) && eq(a.stackTy[1], b.stackTy[1])
}

// TestTypeLineFirstSightAllocsNothing: after Reserve, an encoder's first
// sight of a type-line shape its pool did not precompute builds the line
// into its reserved memo without a heap allocation, and leaves the shared
// pool untouched.
func TestTypeLineFirstSightAllocsNothing(t *testing.T) {
	p, _, _, _ := tyPool(t)
	c := p.card(0)
	shapes := tyShapes(len(p.subNames))
	for _, sh := range shapes {
		ty, subs, extra := typeKey(c, sh.types, sh.subs, sh.all, sh.setSub)
		if p.findTypeLine(ty, subs, sh.all, extra) != nil {
			t.Fatalf("shape %+v is precomputed: the test needs shapes the pool lacks", sh)
		}
	}
	const runs = 20
	if len(shapes) < runs+1 {
		t.Fatalf("%d shapes, need %d distinct first sights", len(shapes), runs+1)
	}
	e := NewEncoder(p)
	e.Reserve()
	npool := len(p.tys)
	i := 0
	allocs := testing.AllocsPerRun(runs, func() {
		sh := shapes[i]
		i++
		e.typeLine(c, sh.types, sh.subs, sh.all, sh.setSub)
	})
	if allocs != 0 {
		t.Fatalf("a first-sight type line allocated %.1f objects per build after Reserve, want 0", allocs)
	}
	if len(e.tym.tys) != runs+1 {
		t.Fatalf("encoder memo holds %d lines, want %d (every call a first sight)", len(e.tym.tys), runs+1)
	}
	if len(p.tys) != npool {
		t.Fatalf("the shared pool grew from %d to %d lines during an encode", npool, len(p.tys))
	}
}

// TestTypeLineMatchesReference: every line the encoder builds (its memo,
// arena-backed) equals develop's heap-built line field for field, and a
// shape seen again is the same pointer (the content caches key on it).
func TestTypeLineMatchesReference(t *testing.T) {
	p, _, _, _ := tyPool(t)
	e := NewEncoder(p)
	e.Reserve()
	for ci := 0; ci < 2; ci++ {
		c := p.card(kop.CardID(ci))
		for _, sh := range tyShapes(len(p.subNames)) {
			got := e.typeLine(c, sh.types, sh.subs, sh.all, sh.setSub)
			want := refTypeLine(p, c, sh.types, sh.subs, sh.all, sh.setSub)
			if !sameView(got, want) {
				t.Fatalf("card %d shape %+v: line %+v, want %+v", ci, sh, got, want)
			}
			if again := e.typeLine(c, sh.types, sh.subs, sh.all, sh.setSub); again != got {
				t.Fatalf("card %d shape %+v: a second sight returned another line", ci, sh)
			}
		}
	}
	// a line spelling exactly the printed words is nil
	c := p.card(0)
	if got := e.typeLine(c, kop.TCreature, kop.SubMask{}.With(0), false, false); got != nil {
		t.Fatalf("the printed line came back derived: %+v", got)
	}
	// a heap-built line (the pool's precompute: no arena) equals it too
	for _, sh := range tyShapes(len(p.subNames)) {
		ty, subs, extra := typeKey(c, sh.types, sh.subs, sh.all, sh.setSub)
		h := &typeView{types: ty, subs: subs, all: sh.all, extra: extra}
		p.fillTypeLine(h, nil)
		if want := refTypeLine(p, c, sh.types, sh.subs, sh.all, sh.setSub); want != nil && !sameView(h, want) {
			t.Fatalf("shape %+v: heap-built line %+v, want %+v", sh, h, want)
		}
	}
}

// TestTypeLineConcurrentEncoders: two encoders on one pool, building type
// lines and encoding games at once, share nothing they write (run under
// -race).
func TestTypeLineConcurrentEncoders(t *testing.T) {
	p, tbl, da, db := tyPool(t)
	shapes := tyShapes(len(p.subNames))
	var wg sync.WaitGroup
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()
			e := NewEncoder(p)
			e.Reserve()
			for ci := 0; ci < 2; ci++ {
				c := p.card(kop.CardID(ci))
				for _, sh := range shapes {
					e.typeLine(c, sh.types, sh.subs, sh.all, sh.setSub)
				}
			}
			s := dirac.New(tbl, da, db, seed)
			for n := 0; n < 200; n++ {
				d := s.Step()
				if d.Kind == dirac.DkNone {
					break
				}
				e.EncodeStateFor(s, &d, d.Player)
				s.Apply(d.Options[0])
			}
		}(uint64(7 + w))
	}
	wg.Wait()
}

// BenchmarkTypeLineMemo is the steady-state lookup of a line the encoder
// memoised (the pool did not precompute it).
func BenchmarkTypeLineMemo(b *testing.B) {
	t := &testing.T{}
	p, _, _, _ := tyPool(t)
	e := NewEncoder(p)
	e.Reserve()
	c := p.card(0)
	shapes := tyShapes(len(p.subNames))[:8]
	for _, sh := range shapes {
		e.typeLine(c, sh.types, sh.subs, sh.all, sh.setSub)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sh := &shapes[i&7]
		e.typeLine(c, sh.types, sh.subs, sh.all, sh.setSub)
	}
}
