package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// restrictInPlayScen builds a scenario over the restrictinplay fixture.
func restrictInPlayScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "restrictinplay")
	for _, n := range []string{"Swamp", "Vedalken Engineer", "Scrapwork Rager", "Mind Stone"} {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the restrictinplay table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

// The Rager's unearth is offered from this graveyard when plain mana pays it.
func TestGraveyardUnearthOfferedOnPlainMana(t *testing.T) {
	sc, id := restrictInPlayScen(t)
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	rager := sc.obj(id["Scrapwork Rager"], 0)
	sc.s.place(rager, kop.ZoneGraveyard, false)
	sc.s.ctl = ctlPriority
	if n := sc.graveActivations(rager); n != 1 {
		t.Fatalf("Scrapwork Rager's unearth offered %d times with four Swamps, want 1", n)
	}
}

// Vedalken Engineer's mana pays abilities of artifacts on the battlefield
// only (RestrictValid$ Activated.Artifact+inZoneBattlefield; gorge
// RestrictValidMatches' needsBattlefield, CR 106.6, 602.2): Scrapwork
// Rager's unearth from the graveyard may not spend it, so two Plains beside
// the Engineer cannot fund {3}{B}.
func TestGraveyardUnearthRefusesBattlefieldOnlyMana(t *testing.T) {
	sc, id := restrictInPlayScen(t)
	sc.perm(id["Vedalken Engineer"], 0)
	sc.perm(id["Plains"], 0)
	sc.perm(id["Plains"], 0)
	rager := sc.obj(id["Scrapwork Rager"], 0)
	sc.s.place(rager, kop.ZoneGraveyard, false)
	sc.s.ctl = ctlPriority
	if n := sc.graveActivations(rager); n != 0 {
		t.Fatalf("Scrapwork Rager's unearth offered %d times on Vedalken Engineer's battlefield-only mana, want 0", n)
	}
}

// The same mana still pays an artifact's ability on the battlefield: the
// Engineer alone funds Mind Stone's {1}, {T}, sacrifice draw.
func TestBattlefieldOnlyManaPaysAnArtifactInPlay(t *testing.T) {
	sc, id := restrictInPlayScen(t)
	sc.perm(id["Vedalken Engineer"], 0)
	stone := sc.perm(id["Mind Stone"], 0)
	sc.s.ctl = ctlPriority
	if n := sc.graveActivations(stone); n != 1 {
		t.Fatalf("Mind Stone's draw offered %d times on Vedalken Engineer's mana, want 1", n)
	}
}
