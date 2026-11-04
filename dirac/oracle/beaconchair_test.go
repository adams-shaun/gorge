package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Beacon of Kahless and Command Chair lockstep. Each has a first mana
// ability (the Beacon's one mana of any colour, the Chair's three of any one
// colour) beside a second whose amount the two-player game bounds: the
// opponents the Beacon's controller attacked this turn (none or one: then
// it adds what the first does) and the greatest mana value among the
// commanders the Chair's controller controls (no permanent is a commander:
// nothing). gorge's payment window asks which ability to activate and the
// oracle answers the first. Both decks attack early and cast after combat.
// The expected answer at every decision is gorge's.
var kbeaconchairDecks = [2][]cardset.Entry{
	{
		{Name: "The Beacon of Kahless", Count: 8}, {Name: "Command Chair", Count: 4}, {Name: "Savannah Lions", Count: 8},
		{Name: "Craw Wurm", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "The Beacon of Kahless", Count: 6}, {Name: "Command Chair", Count: 6}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Hill Giant", Count: 6}, {Name: "Shock", Count: 2}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6},
	},
}

func TestBeaconChairLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "beaconchair", kbeaconchairDecks, 600, "KBEACONCHAIR_SEEDS")
}

// TestBeaconChairActivations: gorge activates both cards' mana abilities in
// these games, the Beacon's while its controller has attacked this turn
// among them, and every game plays in lockstep.
func TestBeaconChairActivations(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("beaconchair")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kbeaconchairDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kbeaconchairDecks[0], kbeaconchairDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var beacon, beaconAttacked, chair int
	for seed := uint64(1); seed <= 200; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"bc-a", "bc-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			if e := ref.E; e.ManaAbilityHook == nil {
				e.ManaAbilityHook = func(p state.PlayerID, src state.ObjID, _ *cards.SA) {
					o := e.G.Obj(src)
					if o == nil || o.Face() == nil {
						return
					}
					switch o.Face().Name {
					case "The Beacon of Kahless":
						beacon++
						if e.G.Players[p].AttackedPlayersThisTurn != 0 {
							beaconAttacked++
						}
					case "Command Chair":
						chair++
					}
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s", seed, d.At, d.What, d.Detail)
		}
	}
	t.Logf("200 games: Beacon activations %d (%d after its controller attacked), Command Chair activations %d", beacon, beaconAttacked, chair)
	if beacon < 100 || beaconAttacked < 10 || chair < 100 {
		t.Errorf("activations: Beacon %d (%d after attacking), Chair %d; want at least 100, 10, 100", beacon, beaconAttacked, chair)
	}
}
