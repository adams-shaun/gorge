package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFiltRepeatActivatesTwice: Orochi Leafcaller's "{G}: Add one mana of any
// color" has no {T} and no limit (kop.FeeOnlyManaAb), so Serra Angel's {3}{W}{W}
// is paid over five Forests by activating it twice, each Forest's {G} into a
// {W}. gorge's census leaves the ability unrelaxable and unbounded: every
// insufficient play is searched and the funding never binds
// (mana_filterrepeat.go).
func TestFiltRepeatActivatesTwice(t *testing.T) {
	deck := []cardset.Entry{{Name: "Orochi Leafcaller", Count: 2}, {Name: "Serra Angel", Count: 1},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 8}}
	set := cardsettest.Set(t, "manaonly")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	for i := 0; i < 5; i++ {
		sc.perm(id["Forest"], 0)
	}
	oro := sc.perm(id["Orochi Leafcaller"], 0)
	angel := sc.hand(id["Serra Angel"], 0)
	s, k := sc.s, sc.k
	// preconditions: Orochi is a repeatable census gap, the Forests are the
	// whole of what pays, and one conversion alone cannot pay the cast.
	sl := sc.slot(oro)
	gaps := s.scriptFilt(k, 0)
	if !gaps.has(sl) || !s.filtRepeat(k, sl) {
		t.Fatalf("Orochi is no repeatable census gap: %v", gaps)
	}
	if s.filtRelaxable(k, gaps) {
		t.Fatal("a repeatable census must be unrelaxable")
	}
	if got := s.filtFee(k, sl); got != 1 {
		t.Fatalf("Orochi's fee %d, want 1 ({G})", got)
	}
	var cast Action
	found := false
	d := s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && Obj(a.Source()) == angel {
			cast, found = a, true
		}
	}
	if !found {
		t.Fatal("Serra Angel's cast is not offered over a repeatable Orochi")
	}
	var fs filtSearch
	fs.a, fs.p, fs.gaps, fs.own, fs.sp0 = cast, 0, gaps, NoSlot, s.sp
	fs.need = s.playNeed(k, cast)
	fs.other = s.otherGaps(k, 0, cast)
	fs.fees = s.filtFunding(k, 0, gaps)
	tt := s.fork()
	tt.g.noGrant = true
	ok := fs.visit(k, tt, 0, false)
	s.unfork()
	if !ok {
		t.Fatal("no script for Serra Angel over five Forests and Orochi")
	}
	uses := 0
	for _, st := range fs.path[:fs.n] {
		if st.gap && st.ans > 0 && st.sl == sl {
			uses++
		}
	}
	if uses < 2 {
		t.Fatalf("the script activates Orochi %d times, want at least 2 ({W}{W})", uses)
	}
}
