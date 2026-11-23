package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// featBattlefieldRef is FeatBattlefield as it was before its keys moved to a
// local array: an insertion sort that reads each side's timestamp through
// objs -> perms on every comparison.
func featBattlefieldRef(s *State, p uint8, dst []Obj) []Obj {
	n0 := len(dst)
	m := s.ctrlM[SeatIx(p)].and(s.occ)
	for m.any() {
		sl := m.first()
		m = m.rest()
		dst = append(dst, s.perms[sl].obj)
		for j := len(dst) - 1; j > n0 && s.perms[s.objs[dst[j]].slot].ts < s.perms[s.objs[dst[j-1]].slot].ts; j-- {
			dst[j], dst[j-1] = dst[j-1], dst[j]
		}
	}
	return dst
}

// TestFeatBattlefieldOrderMatchesReference: at every decision of random-policy
// games over the committed fdn and blb bench decks, each seat's battlefield
// list is the one the object-chain insertion sort made (entry order), and a
// list appended after other entries leaves them alone.
func TestFeatBattlefieldOrderMatchesReference(t *testing.T) {
	checked, multi, reordered := 0, 0, 0
	for _, name := range []string{"fdn", "blb"} {
		decks := benchDecks(t, name)
		if len(decks) < 8 {
			t.Fatalf("%s: %d bench decks", name, len(decks))
		}
		set := cardsettest.Set(t, name)
		for g := 0; g < 6; g++ {
			tbl, da, db, err := set.Matchup(decks[2*g], decks[2*g+1])
			if err != nil {
				t.Fatalf("%s game %d: %v", name, g, err)
			}
			s := NewConfig(tbl, da, db, uint64(g)*0x9e37+5, Config{})
			pol := NewRandomPolicy(uint64(g)*7919 + 11)
			var d Decision
			var got, want []Obj
			for n := 0; n < 3000; n++ {
				s.StepInto(&d)
				if d.Kind == DkNone {
					break
				}
				for p := uint8(0); p < s.NumSeats(); p++ {
					got = s.FeatBattlefield(p, got[:0])
					want = featBattlefieldRef(s, p, want[:0])
					checked++
					if len(got) > 3 {
						multi++
					}
					if len(got) != len(want) {
						t.Fatalf("%s game %d decision %d seat %d: %d objects, want %d", name, g, n, p, len(got), len(want))
					}
					for i := range got {
						if i > 0 && s.objs[got[i-1]].slot > s.objs[got[i]].slot {
							reordered++ // the entry order differs from the slot order: the sort moved something
						}
						if got[i] != want[i] {
							t.Fatalf("%s game %d decision %d seat %d: entry %d is %d, want %d (%v vs %v)", name, g, n, p, i, got[i], want[i], got, want)
						}
					}
				}
				// appended after another entry: the prefix is untouched
				pre := s.FeatBattlefield(0, []Obj{NoObj})
				if len(pre) != len(got0(s))+1 || pre[0] != NoObj {
					t.Fatalf("%s game %d decision %d: appended list %v", name, g, n, pre)
				}
				s.Apply(pol.Pick(&d))
			}
			ReleaseTable(tbl)
		}
	}
	if reordered < 20 {
		t.Fatalf("only %d lists out of slot order over %d: the sort never moved an entry", reordered, checked)
	}
	if multi < 500 {
		t.Fatalf("only %d battlefield lists with more than 3 objects over %d: the reference never ordered anything", multi, checked)
	}
}

func got0(s *State) []Obj { return s.FeatBattlefield(0, nil) }
