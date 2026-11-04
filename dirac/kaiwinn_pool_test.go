package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Kai Winn, Faithless Leader on the pool table (set.Pool, the table self-play
// plays on by default): the as-enters name ask must offer the pool's nonland
// cards, not an empty list (a decision with no options panics the kernel).
func TestKaiWinnPoolTable(t *testing.T) {
	set := cardsettest.Set(t, "kaiwinn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	if len(tbl.NameCards) == 0 {
		t.Fatal("precondition: the pool table lists no nameable cards")
	}
	kw, ok := set.Lookup("Kai Winn, Faithless Leader")
	if !ok || set.InertID(set.PoolID(kw)) {
		t.Fatal("precondition: Kai Winn is not a playable pool card")
	}
	for _, other := range []string{"Swamp", "Mountain"} {
		oi, _ := set.Lookup(other)
		var da, db []CardID
		for k := 0; k < 20; k++ {
			da = append(da, set.PoolID(kw))
			db = append(db, set.PoolID(kw))
			da = append(da, set.PoolID(oi))
			db = append(db, set.PoolID(oi))
		}
		Register(tbl)
		asks := 0
		for seed := uint64(1); seed <= 12; seed++ {
			var s State
			var d Decision
			s.Init(tbl, da, db, seed, Config{Mulligans: 2})
			pol := NewRandomPolicy(seed)
			for n := 0; n < 4000; n++ {
				s.StepInto(&d)
				if d.Kind == DkNone {
					break
				}
				if d.Sub == ChGenName {
					asks++
				}
				s.Apply(pol.Pick(&d))
			}
		}
		ReleaseTable(tbl)
		// Kai Winn costs black mana: only the Swamp deck casts it
		if asks == 0 && other == "Swamp" {
			t.Fatal("no game asked a card name, so nothing was tested")
		}
	}
}
