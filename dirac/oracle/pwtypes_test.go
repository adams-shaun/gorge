package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pwtypesDecks: Tam, the Possibility (proliferate once per planeswalker type
// among your planeswalkers), Tromell, Seymour's Butler (once per nontoken
// creature that entered under you this turn), beside two Ajanis, a Jace and
// a Chandra and the five basics Tam's cost needs.
var pwtypesDecks = [2][]cardset.Entry{
	{
		{Name: "Tam, the Possibility", Count: 4}, {Name: "Tromell, Seymour's Butler", Count: 3},
		{Name: "Ajani, Mentor of Heroes", Count: 3}, {Name: "Ajani, Caller of the Pride", Count: 4}, {Name: "Jace Beleren", Count: 3}, {Name: "Chandra, Novice Pyromancer", Count: 3},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Tam, the Possibility", Count: 3}, {Name: "Tromell, Seymour's Butler", Count: 4},
		{Name: "Ajani, Mentor of Heroes", Count: 3}, {Name: "Ajani, Caller of the Pride", Count: 3}, {Name: "Jace Beleren", Count: 4}, {Name: "Chandra, Novice Pyromancer", Count: 3},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
}

// A proliferate whose Amount$ is a count proliferates that many times, each
// a fresh choice, and a count of zero proliferates nothing (CR 701.34a): Tam
// counts the distinct planeswalker types among its controller's
// planeswalkers (CR 205.3j), so two Ajanis count once. Both engines agree on
// every proliferate ask and the counters it adds.
func TestCountedProliferateLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "pwtypes", pwtypesDecks, 64, "KPWTYPES_SEEDS")
	lockstepSet(t, "pwtypes", [2][]cardset.Entry{pwtypesDecks[1], pwtypesDecks[0]}, 64, "KPWTYPES_SEEDS")
}

// TestCountedProliferateMutation: with Tam's distinct-type count read as the
// number of planeswalkers instead, some seed diverges, so the pool reaches
// Tam's activation with two walkers of one type.
func TestCountedProliferateMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "pwtypes", pwtypesDecks, 64, false, func(tbl *kop.Table) {
		exprs := append([]kop.Expr(nil), tbl.Exprs...)
		for i := range exprs {
			if exprs[i].Kind == kop.ExCount && exprs[i].N == 1 {
				exprs[i].N = 0
			}
		}
		tbl.Exprs = exprs
	})
	if n == 0 {
		t.Fatal("no seed diverged with the planeswalker types counted as planeswalkers")
	}
}
