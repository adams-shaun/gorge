package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A pass with a legend duplicate poses the legend ask before any order ask for
// its casualties: nothing applies under it (CR 704.3), and the answer applies the
// casualties, which is when their replacements are ordered.
func TestLegendAskBeforeParkedDeath(t *testing.T) {
	sc, id := replCountersScen(t, []cardset.Entry{
		{Name: "Misery's Shadow", Count: 1},
		{Name: "Stone of Erech", Count: 1},
		{Name: "Mauhúr, Uruk-hai Captain", Count: 2},
		{Name: "Grizzly Bears", Count: 1},
		{Name: "Forest", Count: 1},
	})
	s, k := sc.s, sc.k
	bears := sc.perm(id["Grizzly Bears"], 0)
	sc.perm(id["Misery's Shadow"], 1)
	sc.perm(id["Stone of Erech"], 1)
	keep := sc.perm(id["Mauhúr, Uruk-hai Captain"], 1)
	sc.perm(id["Mauhúr, Uruk-hai Captain"], 1)
	s.perms[sc.slot(bears)].damage = 9
	if !s.sba(k) || s.ctl != ctlLegend {
		t.Fatalf("ctl %d, want the legend ask first", s.ctl)
	}
	s.applyLegend(k, MakeAction(AkChoose, TCode(keep), 0, 0))
	if s.ctl != ctlExileOrder {
		t.Fatalf("ctl %d after the legend answer, want the order ask", s.ctl)
	}
	if !sc.onBF(bears) {
		t.Fatal("the casualty moved under its order ask")
	}
}
