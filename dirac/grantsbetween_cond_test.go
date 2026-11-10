package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAttachedGrantHonoursCond2: an attached Continuous static's granted
// ability (Fishing Pole's) is listed only while the static's own condition
// holds; grantsBetween used to skip statCond, so the host kept the grant
// when the condition was false.
func TestAttachedGrantHonoursCond2(t *testing.T) {
	deck := []cardset.Entry{{Name: "Fishing Pole", Count: 1}, {Name: "Sanguine Syphoner", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	for _, edit := range []bool{false, true} {
		tbl, da, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		id := map[string]CardID{}
		for _, c := range da {
			id[tbl.Cards[c].Name] = c
		}
		if edit {
			tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondYourTurn})
			yt := kop.CondID(len(tbl.Conds) - 1)
			pc := &tbl.Cards[id["Fishing Pole"]]
			pc.Abilities = slices.Clone(pc.Abilities)
			n := 0
			for i := range pc.Abilities {
				a := &pc.Abilities[i]
				if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatContinuous && a.Static.Grant != 0 {
					a.Static.Cond2 = yt
					n++
				}
			}
			if n != 1 {
				t.Fatalf("edited %d Continuous grant statics, want 1", n)
			}
		}
		sc := newScenTable(t, tbl, Config{NoElide: true})
		src := sc.perm(id["Sanguine Syphoner"], 0)
		pole := sc.perm(id["Fishing Pole"], 0)
		sc.s.perms[sc.slot(pole)].attached = sc.slot(src)
		grants := func() int {
			n := 0
			sc.s.grantedAbilities(sc.k, sc.slot(src), func(c CardID, i uint8) {
				if c != id["Fishing Pole"] {
					t.Fatalf("grant from card %d, want Fishing Pole", c)
				}
				n++
			})
			return n
		}
		sc.s.active = 0
		if got := grants(); got != 1 {
			t.Fatalf("edit=%v, own turn: %d grants, want 1", edit, got)
		}
		if edit {
			sc.s.pl[0].pool[kop.W] = 1
			d := sc.s.Step()
			if _, ok := grantedAct(&d, src); !ok {
				t.Fatalf("granted activation not offered on the controller's turn: %v", d.Options[:d.N])
			}
		}
		sc.s.active = 1
		want := 1
		if edit {
			want = 0
		}
		if got := grants(); got != want {
			t.Fatalf("edit=%v, opponent's turn: %d grants, want %d", edit, got, want)
		}
	}
}
