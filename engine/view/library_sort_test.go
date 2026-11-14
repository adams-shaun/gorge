package view

import (
	"cmp"
	"math/rand/v2"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestSortByNameIDMatchesStructSort pins unorderedLibrary's index sort to
// the struct sort it replaced: for libraries of every small length and of
// Commander size, with many repeated names (basic lands), the permuted list
// is element-for-element the one slices.SortFunc over the structs leaves,
// and one reused perm scratch serves lists of different lengths.
func TestSortByNameIDMatchesStructSort(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 11))
	names := []string{"Forest", "Island", "Sol Ring", "Arcane Signet", "Llanowar Elves", "", "Forest"}
	var perm []int32
	for _, n := range []int{0, 1, 2, 3, 5, 8, 17, 40, 60, 99, 100} {
		for trial := 0; trial < 20; trial++ {
			cvs := make([]CardView, n)
			for i := range cvs {
				cvs[i] = CardView{
					ID:       state.ObjID(r.IntN(1000) + 1 + i*1000), // distinct, in shuffled order
					Name:     names[r.IntN(len(names))],
					Keywords: []string{names[r.IntN(len(names))]},
				}
			}
			r.Shuffle(len(cvs), func(i, j int) { cvs[i], cvs[j] = cvs[j], cvs[i] })
			want := slices.Clone(cvs)
			slices.SortFunc(want, func(a, b CardView) int {
				if c := strings.Compare(a.Name, b.Name); c != 0 {
					return c
				}
				return cmp.Compare(a.ID, b.ID)
			})
			sortByNameID(cvs, &perm)
			if !reflect.DeepEqual(cvs, want) {
				t.Fatalf("n=%d trial %d: index sort differs from the struct sort", n, trial)
			}
		}
	}
}
