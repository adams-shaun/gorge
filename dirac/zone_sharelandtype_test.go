package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestShareLandTypeSecondPick: with SrShareLandType (carried in B) a search
// for two lands over a library of Forest, Forest, Plains offers every land at
// the first pick; after a Forest, the other Forest is a legal second pick and
// the Plains is not.
func TestShareLandTypeSecondPick(t *testing.T) {
	var forest, plains, bear CardID
	sc, prog := czScenWith(t, func(tbl *kop.Table) []kop.Instr {
		fi, pi := len(tbl.Subtypes), len(tbl.Subtypes)+1
		tbl.Subtypes = append(tbl.Subtypes, "Forest", "Plains")
		forest, plains, bear = CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Forest", Types: kop.TLand, Subtypes: kop.SubMask{}.With(fi)},
			kop.CardDef{Name: "Test Plains", Types: kop.TLand, Subtypes: kop.SubMask{}.With(pi)},
			kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
		tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand})
		return []kop.Instr{{Op: kop.OpSearchEx, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(kop.SrShareLandType),
			F: kop.FilterID(len(tbl.Filters) - 1), C: int16(kop.ZoneHand)}}
	})
	s := sc.s
	pl := &s.pl[0]
	for i, o := range pl.lib[pl.libLo:pl.libHi] {
		c := bear
		switch i {
		case 0, 1:
			c = forest
		case 2:
			c = plains
		}
		s.objs[o].card = c
	}
	if s.exec(sc.k, &prog[0]) || s.res.ask != raGenSearch {
		t.Fatalf("ask %d, want the search pick", s.res.ask)
	}
	cards := func(d *Decision) (n map[CardID]int) {
		n = map[CardID]int{}
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkChoose {
				n[s.objs[a.Source()].card]++
			}
		}
		return n
	}
	var d Decision
	s.resolveOptionsGen(sc.k, &d)
	if n := cards(&d); n[forest] != 2 || n[plains] != 1 || d.N != 4 {
		t.Fatalf("first pick: %v of %d options, want both Forests, the Plains and Done", n, d.N)
	}
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkChoose && s.objs[a.Source()].card == forest {
			s.applyResolveGen(sc.k, a)
			break
		}
	}
	d = Decision{}
	s.resolveOptionsGen(sc.k, &d)
	if n := cards(&d); n[forest] != 1 || n[plains] != 0 || d.N != 2 {
		t.Fatalf("after a Forest: %v of %d options, want the other Forest and Done, not the Plains", n, d.N)
	}
	s.applyResolveGen(sc.k, d.Options[0])
	if pl.handN != 2 || s.objs[pl.hand[0]].card != forest || s.objs[pl.hand[1]].card != forest {
		t.Fatalf("hand %d, want the two Forests found", pl.handN)
	}
}
