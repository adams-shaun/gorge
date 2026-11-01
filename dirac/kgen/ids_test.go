package kgen

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestIDsNeverMove: a pool change keeps every existing CardID, gives a new
// card and a new token the next free ones (cards first), and leaves a
// departed card's id in the index.
func TestIDsNeverMove(t *testing.T) {
	prev := []idEntry{{0, false, "Bear"}, {1, false, "Elf"}, {2, true, "t_1_1"}}
	all := []*lCard{{Name: "Ant"}, {Name: "Elf"}, {Name: "t_1_1", Token: true}, {Name: "t_0_0", Token: true}}
	ids, idx, err := assignIDs(prev, all)
	if err != nil {
		t.Fatal(err)
	}
	if want := []int{3, 1, 2, 4}; len(ids) != len(want) || ids[0] != want[0] || ids[1] != want[1] || ids[2] != want[2] || ids[3] != want[3] {
		t.Fatalf("ids %v, want %v", ids, want)
	}
	if len(idx) != 5 || idx[0].name != "Bear" || idx[3].name != "Ant" || !idx[4].token {
		t.Fatalf("index %+v", idx)
	}
	back, err := readIDs(filepath.Join(t.TempDir(), "none.tsv"))
	if err != nil || back != nil {
		t.Fatalf("missing index: %v %v", back, err)
	}
}

// TestPoolTableIDs: the checked-in pool table puts every pool card at its
// indexed CardID, inert where unsupported.
func TestPoolTableIDs(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.Cards) != set.NumIDs() {
		t.Fatalf("%d cards, want %d", len(tbl.Cards), set.NumIDs())
	}
	inert := 0
	for i := 0; i < set.NumCards()+set.NumTokens(); i++ {
		id := set.PoolID(i)
		if tbl.Cards[id].Name != set.Names()[i] {
			t.Fatalf("CardID %d holds %q, want %q", id, tbl.Cards[id].Name, set.Names()[i])
		}
		if set.InertID(id) != set.Inert(i) {
			t.Fatalf("CardID %d inert mismatch", id)
		}
		if set.Inert(i) {
			inert++
		}
	}
	t.Logf("%d CardIDs, %d inert", len(tbl.Cards), inert)
}
