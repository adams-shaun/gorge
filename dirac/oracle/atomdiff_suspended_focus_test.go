package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// suspendedAtom is the T13c follow-up's fw:Card.suspended: gorge's compiled
// `suspended` read (predicates["suspended"], engine/effects/filter.go:637, which
// the matcher consults before wordSuspended): a Suspend card in exile with a
// positive time counter. It reads nowhere but exile (its manifest context is
// card, so a battlefield or stack matcher refuses it), and is proven here over
// the suspend lockstep's decks, whose games exile cards both through the special
// action and through a spell's own exile. The seed sets hold no suspend card, so
// (as DoubleFaced) it is not in the registry and leaves atomdiff-unproven.txt alone.
var suspendedAtom = FilterAtom{Name: "fw:Card.suspended", GorgeSpec: "Card.suspended", Ctx: AtomAnyZone &^ (AtomBattlefield | AtomStack)}

// zoneProvSuspendGames plays the suspend lockstep's decks under the atom check
// and folds each game's reads into total. It returns the number of games.
func zoneProvSuspendGames(t *testing.T, reg *cards.Registry, atoms []FilterAtom, total *AtomStats) int {
	t.Helper()
	set, err := cardset.RawByName("suspend")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	for di := range ksDecks[:2] {
		var decks [2][]*cards.Card
		for s := range decks {
			for _, e := range ksDecks[di] {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
		}
		tbl, _, _, err := set.Matchup(ksDecks[di], ksDecks[di])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 4; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ksatom-a", "ksatom-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			st := NewAtomStats()
			st.Seed = seed
			res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 30, Check: AtomCheckFor(st, atoms, nil)})
			CloseSide(k)
			games++
			total.Add(st)
			if d := res.Div; d != nil {
				t.Errorf("suspend deck %d seed %d: decision %d: %s %s: %s", di, seed, d.At, d.What, d.Field, d.Detail)
			}
		}
	}
	return games
}
