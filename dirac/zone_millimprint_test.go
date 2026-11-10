package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// topCards returns player p's top n library objects.
func topCards(s *State, p uint8, n int) []Obj {
	pl := &s.pl[p]
	return append([]Obj(nil), pl.lib[pl.libLo:int(pl.libLo)+n]...)
}

// wantImprinted requires the imprinted set to be exactly want, in order.
func wantImprinted(t *testing.T, s *State, what string, want []Obj) {
	t.Helper()
	if int(s.g.nimp) != len(want) {
		t.Fatalf("%s: imprinted set %v (n %d), want %v", what, s.g.imp[:s.g.nimp], s.g.nimp, want)
	}
	for i, o := range want {
		if s.g.imp[i] != o {
			t.Fatalf("%s: imprinted set %v, want %v", what, s.g.imp[:s.g.nimp], want)
		}
	}
}

// TestMillImprintDirectAndMay: OpMill with MvImprint adds each milled card to
// the imprinted set (Patient Naturalist), apart from the remembered set; with
// MvMayMill the cards join it on a yes and nothing is milled or imprinted on
// a no.
func TestMillImprintDirectAndMay(t *testing.T) {
	prog := []kop.Instr{{Op: kop.OpMill, Obj: kop.RefController, A: kop.Const(3), C: kop.MvImprint}}
	sc := czScen(t, prog)
	s := sc.s
	top := topCards(s, 0, 3)
	if s.g.nimp != 0 || s.g.nrem != 0 {
		t.Fatalf("precondition: imprinted %d, remembered %d before the mill", s.g.nimp, s.g.nrem)
	}
	s.exec(sc.k, &prog[0])
	wantImprinted(t, s, "direct", top)
	if s.g.nrem != 0 {
		t.Fatalf("Imprint$ alone remembered %d cards", s.g.nrem)
	}
	for _, o := range top {
		if s.objs[o].zone != kop.ZoneGraveyard {
			t.Fatalf("milled object %d sits in zone %d, want the graveyard", o, s.objs[o].zone)
		}
	}

	// both flags: the two sets are independent
	prog = []kop.Instr{{Op: kop.OpMill, Obj: kop.RefController, A: kop.Const(2), C: kop.MvImprint | kop.MvRemember}}
	sc = czScen(t, prog)
	top = topCards(sc.s, 0, 2)
	sc.s.exec(sc.k, &prog[0])
	wantImprinted(t, sc.s, "imprint+remember", top)
	if sc.s.g.nrem != 2 {
		t.Fatalf("remembered %d, want 2", sc.s.g.nrem)
	}

	for _, answer := range []uint8{1, 0} {
		prog = []kop.Instr{{Op: kop.OpMill, Obj: kop.RefController, A: kop.Const(3), C: kop.MvImprint | kop.MvMayMill}}
		sc = czScen(t, prog)
		s = sc.s
		top = topCards(s, 0, 3)
		if s.g.nimp != 0 {
			t.Fatal("precondition: imprinted before the ask")
		}
		if s.exec(sc.k, &prog[0]) || s.res.ask != raGenMayMill {
			t.Fatalf("answer %d: no may-mill ask", answer)
		}
		s.applyAskCzParams(sc.k, MakeAction(AkChoose, 0, 0, answer))
		if answer == 1 {
			wantImprinted(t, s, "may yes", top)
		} else if s.g.nimp != 0 || s.pl[0].graveN != 0 {
			t.Fatalf("may no: imprinted %d, graveyard %d, want none", s.g.nimp, s.pl[0].graveN)
		}
	}
}

// TestMillImprintParked: a milled card whose graveyard move competes (two
// Rest in Peace) is parked; the imprint mark rides in the parked record and
// is applied when its turn runs.
func TestMillImprintParked(t *testing.T) {
	deck := []cardset.Entry{{Name: "Rest in Peace", Count: 2}, {Name: "Swamp", Count: 8}}
	tbl, da, _, err := cardsettest.Synth(t, "std-big").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, flags := range []int{kop.MvImprint, kop.MvImprint | kop.MvRemember, kop.MvRemember} {
		sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
		sc.perm(id["Rest in Peace"], 0)
		sc.perm(id["Rest in Peace"], 1)
		s, k := sc.s, sc.k
		top := topCards(s, 0, 3)
		if s.g.nimp != 0 || s.g.nrem != 0 {
			t.Fatal("precondition: sets not empty")
		}
		if !s.parkMills(k, 0, 3, flags) {
			t.Fatalf("flags %b: the mill's first card has no order ask", flags)
		}
		var q []parkedMove
		for i := range s.cold.parked {
			if r := s.cold.parked[i]; r.kind == parkMill {
				q = append(q, r)
			}
		}
		if len(q) != 3 {
			t.Fatalf("flags %b: %d parked mills, want 3", flags, len(q))
		}
		for _, r := range q {
			s.runParked(k, r)
		}
		if flags&kop.MvImprint != 0 {
			wantImprinted(t, s, "parked", top)
		} else if s.g.nimp != 0 {
			t.Fatalf("flags %b: imprinted %d", flags, s.g.nimp)
		}
		if want := flags & kop.MvRemember; (want != 0) != (s.g.nrem == 3) || s.g.nrem != 0 && s.g.nrem != 3 {
			t.Fatalf("flags %b: remembered %d", flags, s.g.nrem)
		}
	}
}

// TestMillImprintShuffling: on a table where a graveyard-bound card may be
// shuffled back (Progenitus), the milled cards are fixed first and each is
// imprinted whichever zone it landed in.
func TestMillImprintShuffling(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Progenitus", Count: 1}, {Name: "Swamp", Count: 8}})
	s, k := sc.s, sc.k
	if k.gen == nil || k.gen.anyRepl&rbGraveLib == 0 {
		t.Fatal("precondition: the table has no grave-to-library replacement")
	}
	_ = id
	top := topCards(s, 0, 3)
	if s.g.nimp != 0 {
		t.Fatal("precondition: imprinted before the mill")
	}
	s.millShuffling(k, 0, 3, kop.MvImprint)
	wantImprinted(t, s, "shuffling", top)
	s.g.nimp = 0
	prog := []kop.Instr{{Op: kop.OpMill, Obj: kop.RefController, A: kop.Const(2), C: kop.MvImprint}}
	top = topCards(s, 0, 2)
	s.res.e.ctrl = 0
	s.exec(k, &prog[0])
	wantImprinted(t, s, "OpMill under rbGraveLib", top)
}

// TestMillImprintedFilter: the PredWord7 W7Imprinted filter matches a
// graveyard card or an exiled card in the imprinted set and not one outside
// it.
func TestMillImprintedFilter(t *testing.T) {
	tbl := castMaypermissionLimitTable() // a table whose prep arms k.gen
	tbl.Filters = append(tbl.Filters, kop.Filter{Pred: kop.PredWord7, PredN: kop.W7Imprinted, Zone: kop.ZoneGraveyard},
		kop.Filter{Pred: kop.PredWord7, PredN: kop.W7Imprinted, Zone: kop.ZoneExile})
	fg, fe := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	in, out := sc.obj(v.Plains, 0), sc.obj(v.Plains, 0)
	s.place(in, kop.ZoneGraveyard, false)
	s.place(out, kop.ZoneGraveyard, false)
	s.imprint(in)
	if !s.impHas(in) || s.impHas(out) {
		t.Fatal("precondition: only the first card is imprinted")
	}
	m := func(f kop.FilterID, o Obj) bool { return s.matchCardGen(k, f, s.objs[o].card, 0, 0, o) }
	if !m(fg, in) || m(fg, out) {
		t.Fatalf("graveyard: imprinted card matches %v, other card matches %v; want true, false", m(fg, in), m(fg, out))
	}
	s.place(in, kop.ZoneExile, false)
	s.place(out, kop.ZoneExile, false)
	if !m(fe, in) || m(fe, out) {
		t.Fatalf("exile: imprinted card matches %v, other card matches %v; want true, false", m(fe, in), m(fe, out))
	}
	s.g.nimp = 0
	if m(fe, in) {
		t.Fatal("a cleared set still matches")
	}
}
