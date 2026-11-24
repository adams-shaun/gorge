package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// CR 707.9b applies a copy's exceptions in the order the copy effect adds
// them. A copy of a copy carries its template's exception records first and
// its own after (dirac/copy_exceptions.go), so an inherited "except it has
// changeling" (AllCreatureTypes) followed by a SetCreatureTypes exception
// ("except it's a Frog", RemoveCreatureSub) ends with the Frog's creature
// types, while the reverse order leaves the changeling's every creature type
// standing. The seed sites used to combine copyExcSetsSub/copyExcAll as
// booleans, so either order kept every creature type.

// copyExcOrderScen builds a vanilla table with the subtype vocabulary the two
// exceptions name and appends the two copy-exception overrides. It returns
// the scenario, the fxCopyExc indices (index+1) of the changeling and the
// Frog exception, and the Bear/Frog subtype ids.
func copyExcOrderScen(t *testing.T) (sc *scen, changeling, setFrog uint8, bear, frog int) {
	t.Helper()
	tbl := v.NewTable()
	// a non-nil Exprs forces the generated prep (k.gen) for the seeded
	// exceptions; a plain vanilla table would leave it nil.
	tbl.Exprs = make([]kop.Expr, 1)
	bear, frog = 0, 1
	tbl.Subtypes = []string{"Bear", "Frog"}
	tbl.Cards[v.GrizzlyBears].Subtypes = kop.SubMask{}.With(bear)
	// fxCopyExc stores the override index+1, so record the value after the
	// append.
	changeling = uint8(len(tbl.Overrides) + 1)
	tbl.Overrides = append(tbl.Overrides, kop.Override{AllCreatureTypes: true})
	setFrog = uint8(len(tbl.Overrides) + 1)
	tbl.Overrides = append(tbl.Overrides, kop.Override{RemoveCreatureSub: true, AddSub: kop.SubMask{}.With(frog)})
	sc = newScenTable(t, tbl, Config{})
	sc.k.gen.cpxSub = true // no table ability references the appended overrides
	return sc, changeling, setFrog, bear, frog
}

// copyExcOrderSlot puts a Grizzly Bears on the battlefield and pushes the two
// exception records in the given order, returning its slot and object.
func copyExcOrderSlot(sc *scen, first, second uint8) (Slot, Obj) {
	o := sc.perm(v.GrizzlyBears, 0)
	sl := sc.slot(o)
	sc.s.copyExcPush(sl, first)
	sc.s.copyExcPush(sl, second)
	return sl, o
}

// copyExcRecords lists sl's fxCopyExc record indices (override index+1) in
// pool order, so the test's precondition is the order the readers see.
func copyExcRecords(s *State, sl Slot) (out []uint8) {
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.sl == sl && r.kind == fxCopyExc {
			out = append(out, r.r.idx)
		}
	}
	return out
}

func TestCopyExcChangelingOrder(t *testing.T) {
	// The later Frog exception wins: the inherited changeling's every
	// creature type goes, and only the Frog remains.
	t.Run("set after changeling", func(t *testing.T) {
		sc, changeling, setFrog, bear, frog := copyExcOrderScen(t)
		sl, o := copyExcOrderSlot(sc, changeling, setFrog)
		s, k := sc.s, sc.k
		if got := copyExcRecords(s, sl); len(got) != 2 || got[0] != changeling || got[1] != setFrog {
			t.Fatalf("precondition: fxCopyExc records %v, want [%d %d]", got, changeling, setFrog)
		}
		if !s.fxM.has(sl) {
			t.Fatal("precondition: the slot has no copy-exception marker")
		}
		if s.hasSubGen(k, sl, subOutside) {
			t.Error("the later SetCreatureTypes left the inherited changeling's every creature type standing")
		}
		if s.hasSubGen(k, sl, uint8(bear)) {
			t.Error("the later SetCreatureTypes left the copied Bear subtype standing")
		}
		if !s.hasSubGen(k, sl, uint8(frog)) {
			t.Error("the later SetCreatureTypes lost its Frog")
		}
		subs, all := s.subsGen(k, sl)
		if all || subs.Has(bear) || !subs.Has(frog) {
			t.Errorf("subsGen = %v all=%v, want only Frog, no all-mark", subs, all)
		}
		if c := s.chars(k, sl, upTo4); c.all || c.subs.Has(bear) || !c.subs.Has(frog) {
			t.Errorf("layer-4 walk = subs %v all=%v, want only Frog", c.subs, c.all)
		}
		if subs := s.lkiSubs(k, sl); subs.Has(bear) || !subs.Has(frog) {
			t.Errorf("lkiSubs = %v, want only Frog", subs)
		}
		var p Projection
		s.Project(&p)
		s.ProjectChars(&p)
		pc := p.Chars[o]
		if pc.AllSub || pc.Subs.Has(bear) || !pc.Subs.Has(frog) {
			t.Errorf("ProjectChars = subs %v all=%v, want only Frog", pc.Subs, pc.AllSub)
		}
	})

	// The later changeling exception wins the other way too: a setter first
	// and the changeling after leaves every creature type (the guard against
	// making any setter unconditional).
	t.Run("changeling after set", func(t *testing.T) {
		sc, changeling, setFrog, _, frog := copyExcOrderScen(t)
		sl, o := copyExcOrderSlot(sc, setFrog, changeling)
		s, k := sc.s, sc.k
		if got := copyExcRecords(s, sl); len(got) != 2 || got[0] != setFrog || got[1] != changeling {
			t.Fatalf("precondition: fxCopyExc records %v, want [%d %d]", got, setFrog, changeling)
		}
		if !s.hasSubGen(k, sl, subOutside) {
			t.Error("the later changeling exception did not restore every creature type")
		}
		subs, all := s.subsGen(k, sl)
		if !all || !subs.Has(frog) {
			t.Errorf("subsGen = %v all=%v, want all creature types with Frog", subs, all)
		}
		if c := s.chars(k, sl, upTo4); !c.all {
			t.Error("the later changeling exception did not restore the layer-4 all-creature-types seed")
		}
		if subs := s.lkiSubs(k, sl); !subs.Has(frog) {
			t.Errorf("lkiSubs = %v, want the Frog and every other creature type", subs)
		}
		var p Projection
		s.Project(&p)
		s.ProjectChars(&p)
		if pc := p.Chars[o]; !pc.AllSub || !pc.Subs.Has(frog) {
			t.Errorf("ProjectChars = subs %v all=%v, want all creature types", pc.Subs, pc.AllSub)
		}
	})
}
