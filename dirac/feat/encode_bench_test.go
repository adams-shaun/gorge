package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// BenchmarkEncodeGame encodes every decision of an FDN game played on its
// first option (one op: one game, kernel stepping included).
func BenchmarkEncodeGame(b *testing.B) {
	set := cardsettest.Set(b, "fdn")
	tbl, da, db, err := set.Matchup(
		[]cardset.Entry{{Name: "Mountain", Count: 20}, {Name: "Healer's Hawk", Count: 20}},
		[]cardset.Entry{{Name: "Plains", Count: 20}, {Name: "Healer's Hawk", Count: 20}},
	)
	if err != nil {
		b.Fatal(err)
	}
	e := NewEncoder(NewPool(tbl, make([]CardSpec, len(tbl.Cards))))
	e.Reserve()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := dirac.New(tbl, da, db, uint64(i%8))
		for n := 0; n < 400; n++ {
			d := s.Step()
			if d.Kind == dirac.DkNone {
				break
			}
			e.EncodeStateFor(s, &d, d.Player)
			s.Apply(d.Options[0])
		}
	}
}
