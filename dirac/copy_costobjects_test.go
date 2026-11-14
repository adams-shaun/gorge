package dirac

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCopiedFlingUsesTheOriginalSacrifice (CR 707.10, gorge
// TestCR707_10CopiedFlingUsesTheOriginalSacrifice): Fling ("As an
// additional cost, sacrifice a creature ... deals damage equal to the
// sacrificed creature's power") copied by Thousand-Year Storm: the copy
// uses the original's sacrificed creature, so the opponent takes its power
// twice. FDN has no Fling, so the test gives Burst Lightning Fling's
// lowering (a Sac1{types=0x2} spell cost, DealDamage A=PowerOf(Sacrificed),
// Heartfire Immolator's program).
func TestCopiedFlingUsesTheOriginalSacrifice(t *testing.T) {
	deck := []cardset.Entry{{Name: "Thousand-Year Storm", Count: 1}, {Name: "Burst Lightning", Count: 1},
		{Name: "Heartfire Immolator", Count: 1}, {Name: "Serra Angel", Count: 1}, {Name: "Mountain", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl0, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append(append([]kop.Filter(nil), tbl0.Filters...), kop.Filter{Types: kop.TCreature})
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	fl := &tbl.Cards[id["Burst Lightning"]]
	fl.Abilities = append([]kop.Ability(nil), fl.Abilities...)
	fl.Abilities[0].Cost = kop.Cost{Flags: kop.CostSac, N: 1, Filter: kop.FilterID(len(tbl.Filters) - 1)}
	fl.Abilities[0].Prog = tbl.Cards[id["Heartfire Immolator"]].Abilities[0].Prog
	for _, paid := range []bool{false, true} {
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		s := sc.s
		if paid {
			sc.perm(id["Thousand-Year Storm"], 0)
			s.g.isCast[0] = 1
		}
		sc.lands(0, id["Mountain"], 2)
		angel := sc.perm(id["Serra Angel"], 0)
		f := sc.hand(id["Burst Lightning"], 0)
		life := s.pl[1].life
		sc.do(t, AkCast, TCode(f), scAny)
		for i := 0; i < 40; i++ {
			d := s.Step()
			if d.Kind == DkNone || s.sp == 0 && i > 2 && d.Kind == DkPriority && s.pl[1].life != life {
				break
			}
			var pick Action
			found := false
			for j := 0; j < int(d.N) && !found; j++ {
				a := d.Options[j]
				switch {
				case d.Kind == DkTarget && a.Target() == PlayerTarget0+1,
					d.Kind == DkChoose && Obj(a.Source()) == angel,
					d.Kind == DkPriority && a.Kind() == AkPass:
					pick, found = a, true
				}
			}
			if !found {
				pick = d.Options[0]
			}
			s.Apply(pick)
		}
		if sc.onBF(angel) {
			t.Fatal("Serra Angel was not sacrificed")
		}
		want := 4
		if paid {
			want = 8
		}
		if got := int(life - s.pl[1].life); got != want {
			t.Fatalf("storm=%v: opponent lost %d, want %d (the sacrificed Angel's power per copy)", paid, got, want)
		}
	}
}

// TestCopyOfAHastyCopyHasHaste (CR 707.9b, gorge
// TestTokenCopyOfAnEternalizedTokenKeepsTheExceptions): Electroduplicate's
// token copy "except it has haste" has haste as a copiable value, so
// Self-Reflection's token copy OF that token has haste too; a copy of the
// plain creature does not.
func TestCopyOfAHastyCopyHasHaste(t *testing.T) {
	deck := []cardset.Entry{{Name: "Electroduplicate", Count: 1}, {Name: "Self-Reflection", Count: 2},
		{Name: "Savannah Lions", Count: 1}, {Name: "Mountain", Count: 10}}
	sc, id := newObjScen(t, deck)
	s := sc.s
	lions := sc.perm(id["Savannah Lions"], 0)
	s.pl[0].pool[kop.R] = 3
	ed := sc.hand(id["Electroduplicate"], 0)
	sc.do(t, AkCast, TCode(ed), scAny)
	sc.do(t, AkTarget, scAny, TCode(lions))
	sc.resolveAll(t)
	cs := sc.copiesOf(id["Savannah Lions"])
	var hasty Obj = NoObj
	for _, sl := range cs {
		if o := s.perms[sl].obj; s.objs[o].of&ofToken != 0 {
			hasty = o
		}
	}
	if hasty == NoObj || s.kw(s.k(), s.objs[hasty].slot)&kop.KwHaste == 0 {
		t.Fatalf("Electroduplicate's copy missing or without haste (%d Lions)", len(cs))
	}
	for i, src := range []Obj{hasty, lions} {
		s.pl[0].pool[kop.U] = 6
		sr := sc.hand(id["Self-Reflection"], 0)
		before := map[Obj]bool{}
		for _, sl := range sc.copiesOf(id["Savannah Lions"]) {
			before[s.perms[sl].obj] = true
		}
		sc.do(t, AkCast, TCode(sr), scAny)
		sc.do(t, AkTarget, scAny, TCode(src))
		sc.resolveAll(t)
		var got Obj = NoObj
		for _, sl := range sc.copiesOf(id["Savannah Lions"]) {
			if o := s.perms[sl].obj; !before[o] {
				got = o
			}
		}
		if got == NoObj {
			t.Fatalf("copy %d: Self-Reflection made no copy", i)
		}
		haste := s.kw(s.k(), s.objs[got].slot)&kop.KwHaste != 0
		if haste != (src == hasty) {
			t.Fatalf("copy of %d (hasty source %v): haste %v", src, src == hasty, haste)
		}
	}
}

// A copy of a hasty copy carries both exceptions (CR 707.9b), and gorge lists
// haste once for each; the keyword is redundant (CR 702.10d).
func TestCopyOfAHastyCopyListsHasteTwice(t *testing.T) {
	deck := []cardset.Entry{{Name: "Electroduplicate", Count: 2}, {Name: "Savannah Lions", Count: 1}, {Name: "Mountain", Count: 10}}
	sc, id := newObjScen(t, deck)
	s := sc.s
	lions := sc.perm(id["Savannah Lions"], 0)
	copyOf := func(src Obj) Obj {
		before := map[Obj]bool{}
		for _, sl := range sc.copiesOf(id["Savannah Lions"]) {
			before[s.perms[sl].obj] = true
		}
		s.pl[0].pool[kop.R] = 3
		sc.do(t, AkCast, TCode(sc.hand(id["Electroduplicate"], 0)), scAny)
		sc.do(t, AkTarget, scAny, TCode(src))
		sc.resolveAll(t)
		for _, sl := range sc.copiesOf(id["Savannah Lions"]) {
			if o := s.perms[sl].obj; !before[o] {
				return o
			}
		}
		t.Fatal("no copy made")
		return NoObj
	}
	haste := bits.TrailingZeros32(uint32(kop.KwHaste))
	first := copyOf(lions)
	if _, rep, _ := s.FeatGrantedRep(first); rep[haste] != 0 {
		t.Fatalf("a copy of the printed creature repeats haste %d times", rep[haste])
	}
	if _, rep, _ := s.FeatGrantedRep(copyOf(first)); rep[haste] != 1 {
		t.Fatalf("a copy of the hasty copy repeats haste %d times, want 1", rep[haste])
	}
}
