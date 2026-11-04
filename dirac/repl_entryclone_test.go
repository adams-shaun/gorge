package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// cloneEntryFix is std-dft's Waxen Shapethief cast beside two Thunderous
// Velocipedes (each other creature or Vehicle enters with a +1/+1 counter, three
// when its mana value is above four) with a Hulldrifter (a Vehicle of mana
// value five) and a Starting Column (an artifact that is neither a creature nor
// a Vehicle) to copy.
type cloneEntryFix struct {
	sc            *scen
	v1, v2        Obj
	hull, col, sp Obj
	shape         CardID
}

func newCloneEntryFix(t *testing.T) *cloneEntryFix {
	t.Helper()
	set := cardsettest.Synth(t, "std-dft")
	var deck []cardset.Entry
	for _, n := range []string{"Waxen Shapethief", "Thunderous Velocipede", "Hulldrifter", "Starting Column", "Island"} {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-dft table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	f := &cloneEntryFix{sc: sc, shape: id["Waxen Shapethief"]}
	f.v1 = sc.perm(id["Thunderous Velocipede"], 0)
	f.v2 = sc.perm(id["Thunderous Velocipede"], 0)
	f.hull = sc.perm(id["Hulldrifter"], 0)
	f.col = sc.perm(id["Starting Column"], 0)
	f.sp = sc.hand(f.shape, 0)
	sc.s.pl[0].pool[kop.U] = 4
	return f
}

// toAsk casts the Shapethief, copies target and returns the order ask.
func (f *cloneEntryFix) toAsk(t *testing.T, target Obj) Decision {
	t.Helper()
	sc, s := f.sc, f.sc.s
	sc.castCard(t, f.sp)
	for i := 0; i < 40; i++ {
		d := s.Step()
		switch {
		case d.Kind == DkChoose && d.Sub == ChGenClone:
			for j := 0; j < int(d.N); j++ {
				if Obj(d.Options[j].Source()) == target && d.Options[j].Mode() == 0 {
					s.Apply(d.Options[j])
					return f.ask(t)
				}
			}
			t.Fatalf("%d is not among the clone options %v", target, d.Options[:d.N])
		case d.Kind == DkPriority:
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			t.Fatalf("decision kind %d sub %d before the clone election", d.Kind, d.Sub)
		}
	}
	t.Fatal("no clone election")
	return Decision{}
}

func (f *cloneEntryFix) ask(t *testing.T) Decision {
	t.Helper()
	d := f.sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenReplOrder {
		t.Fatalf("decision kind %d sub %d, want DkChoose/ChGenReplOrder", d.Kind, d.Sub)
	}
	return d
}

// modes is the options' scan indices, in the order they are offered.
func modes(d Decision) []uint8 {
	var out []uint8
	for _, a := range d.Options[:d.N] {
		out = append(out, a.Mode())
	}
	return out
}

func srcList(d Decision) []Obj {
	var out []Obj
	for _, a := range d.Options[:d.N] {
		out = append(out, Obj(a.Source()))
	}
	return out
}

func equalObjs(a, b []Obj) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestCloneEntryOrderThreeCandidates: CR 616.1 for an as-enters copy beside two
// other replacements (gorge replacement_updated.go). The first ask has one
// option per source, scan order, the permanent spell still on the stack. A
// counter source named first moves the permanent in with every counter body
// folded, and the two left (a counter source and the copy, which commute with
// nothing) are asked with it on the battlefield, the copy not yet applied; the
// copy named first changes nothing the permanent shows: gorge has folded the
// counter bodies into the move, matched against the Shapethief as cast, so a
// copy of a Hulldrifter (mana value five) or of an artifact no Velocipede
// matches still gets one counter from each, and nothing is asked again.
func TestCloneEntryOrderThreeCandidates(t *testing.T) {
	ctrs := func(f *cloneEntryFix, o Obj) int {
		return f.sc.s.perms[f.sc.slot(o)].ctrN(f.sc.s, int(kop.CtrP1P1))
	}
	t.Run("the_copy_first", func(t *testing.T) {
		for _, name := range []string{"hulldrifter", "starting_column"} {
			f := newCloneEntryFix(t)
			sc := f.sc
			target := f.hull
			if name == "starting_column" {
				target = f.col
			}
			d := f.toAsk(t, target)
			if got, want := srcList(d), []Obj{f.v1, f.v2, f.sp}; !equalObjs(got, want) {
				t.Fatalf("%s: first ask options %v, want the Velocipedes then the spell's own line %v", name, got, want)
			}
			if got := modes(d); got[0] != 0 || got[1] != 1 || got[2] != 2 {
				t.Fatalf("%s: scan indices %v, want 0 1 2", name, got)
			}
			if sc.s.objs[f.sp].zone != kop.ZoneStack || sc.s.sp != 1 {
				t.Fatalf("%s: zone %d, stack %d: want the spell still on the stack", name, sc.s.objs[f.sp].zone, sc.s.sp)
			}
			sc.s.Apply(pick(d, f.sp))
			if !sc.onBF(f.sp) || sc.s.ctl != ctlPriority {
				t.Fatalf("%s: on battlefield %v ctl %d, want entered with nothing left to ask", name, sc.onBF(f.sp), sc.s.ctl)
			}
			sl := sc.slot(f.sp)
			if sc.s.perms[sl].ovr.flags&ovCopy == 0 {
				t.Fatalf("%s: the permanent is not a copy", name)
			}
			if n := ctrs(f, f.sp); n != 2 {
				t.Fatalf("%s: %d counters, want 2 (one from each Velocipede, matched against the Shapethief as cast)", name, n)
			}
		}
	})
	t.Run("a_counter_source_first", func(t *testing.T) {
		for _, second := range []string{"the_other_counter_source", "the_copy"} {
			f := newCloneEntryFix(t)
			sc := f.sc
			d := f.toAsk(t, f.hull)
			sc.s.Apply(pick(d, f.v1))
			if !sc.onBF(f.sp) {
				t.Fatal("the permanent is not on the battlefield for the second ask")
			}
			sl := sc.slot(f.sp)
			if sc.s.perms[sl].ovr.flags&ovCopy != 0 || sc.s.cardAt(sl) != f.shape {
				t.Fatalf("%s: the copy applied before the second ask", second)
			}
			if n := ctrs(f, f.sp); n != 2 {
				t.Fatalf("%s: %d counters at the second ask, want 2 (one from each Velocipede, folded at the move)", second, n)
			}
			if sc.s.sp != 0 {
				t.Fatalf("%s: stack %d at the second ask, want empty (the enters triggers wait)", second, sc.s.sp)
			}
			d2 := f.ask(t)
			if got, want := srcList(d2), []Obj{f.v2, f.sp}; !equalObjs(got, want) {
				t.Fatalf("%s: second ask options %v, want the sources not named, in scan order %v", second, got, want)
			}
			if got := modes(d2); got[0] != 1 || got[1] != 2 {
				t.Fatalf("%s: scan indices %v, want 1 2 (the first ask's)", second, got)
			}
			if d2.Player != 0 {
				t.Fatalf("%s: second ask for player %d", second, d2.Player)
			}
			if second == "the_copy" {
				sc.s.Apply(pick(d2, f.sp))
			} else {
				sc.s.Apply(pick(d2, f.v2))
			}
			if sc.s.perms[sl].ovr.flags&ovCopy == 0 || sc.s.ctl != ctlPriority {
				t.Fatalf("%s: copy %v ctl %d, want the copy applied and no third ask", second, sc.s.perms[sl].ovr.flags&ovCopy != 0, sc.s.ctl)
			}
			if n := ctrs(f, f.sp); n != 2 {
				t.Fatalf("%s: %d counters, want 2 (the amount was set at the move, before the copy)", second, n)
			}
		}
	})
}
