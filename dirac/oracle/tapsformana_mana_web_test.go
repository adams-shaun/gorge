package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Mana Web's stack TapsForMana line: whenever a land an opponent controls is
// tapped for mana, tap all lands that player controls that could produce a
// type of mana the tapped land could produce (dirac/kgen/lower_tapsformana.go,
// the kernel's FwCanProduceSameManaTypeWith filter word, and gorge's
// canProduceSameManaTypeWith). The decks give the watched seat lands of two
// basic types so the body's intersection is a proper subset of its board:
// tapping a Mountain must tap the other Mountains, never the Forests.
func manaWebDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Mana Web", Count: 4},
			{Name: "Mountain", Count: 8},
			{Name: "Forest", Count: 8},
			{Name: "Wastes", Count: 4},
			{Name: "Grizzly Bears", Count: 8},
			{Name: "Shock", Count: 8},
		},
		{
			{Name: "Mountain", Count: 6},
			{Name: "Forest", Count: 6},
			{Name: "Wastes", Count: 4},
			{Name: "Grizzly Bears", Count: 12},
			{Name: "Shock", Count: 6},
			{Name: "Giant Growth", Count: 6},
		},
	}
}

// requireLoweredCard fails when the pool's table still refuses a card: a
// lockstep that plays a not-lowered stand-in would pass vacuously.
func requireLoweredCard(t *testing.T, setName, card string) {
	t.Helper()
	// The pool lookup reads the generated tables under .cards, so a fresh
	// clone (no corpus, no tables) must skip here before it can fail; the
	// lockstep that follows would skip anyway.
	corpusDirOrSkip(t)
	set, err := cardset.RawByName(setName)
	if err != nil {
		t.Fatal(err)
	}
	i, ok := set.Lookup(card)
	if !ok {
		t.Fatalf("%s is not in the %s pool", card, setName)
	}
	if st := set.Status(i); st == cardset.StatusNotLowered || st == cardset.StatusRefused {
		t.Fatalf("%s is %s in the %s pool: %s", card, st, setName, set.Reason(i))
	}
}

// TestTapsForManaManaWebLockstep replays the pool's games with Mana Web's
// trigger and body in the kernel's table; any divergence is a mismatch in the
// trigger's controller clause, the body predicate, or Defined$
// TriggeredCardController's scope.
func TestTapsForManaManaWebLockstep(t *testing.T) {
	requireLoweredCard(t, "tapsformana", "Mana Web")
	lockstepSet(t, "tapsformana", manaWebDecks(), 80, "TAPSMANA_MANAWEB_SEEDS")
}

// flipManaWebPredicate inverts every FwCanProduceSameManaTypeWith literal in
// the kernel table's filter programs and reports whether it found one. That
// mutation flips the body's intersection to its complement without changing
// the planner class (dropping the line would), so a lockstep that never
// resolves the body cannot pass it.
func flipManaWebPredicate(tbl *kop.Table) bool {
	found := false
	for i := range tbl.FilterProgs {
		lits := tbl.FilterProgs[i].Lits
		for j := range lits {
			if lits[j].W == kop.FwCanProduceSameManaTypeWith {
				lits[j].Neg = !lits[j].Neg
				found = true
			}
		}
	}
	return found
}

// TestTapsForManaManaWebPredicateMutation proves the games resolve the body:
// with the predicate flipped, some seed diverges. The precondition assertion
// keeps the test honest if the word ever leaves the table's programs.
func TestTapsForManaManaWebPredicateMutation(t *testing.T) {
	requireLoweredCard(t, "tapsformana", "Mana Web")
	found := false
	mutate := func(tbl *kop.Table) {
		found = flipManaWebPredicate(tbl)
	}
	if n := lockstepRun(t, "tapsformana", manaWebDecks(), 80, false, mutate); n == 0 {
		t.Fatal("no seed diverged with the body predicate flipped: the games never resolve Mana Web's body")
	}
	if !found {
		t.Fatal("the kernel table holds no FwCanProduceSameManaTypeWith literal")
	}
}
