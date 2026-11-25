package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Koskun Keep paid filter set: {T}: Add {C}, {1},{T}: Add {R} and
// {2},{T}: Add {B} or {G}. Deck A's other lands make black only, so its red
// and green spells are paid by the Keep's paid abilities: gorge's prefix
// search branches the choose-a-mana-ability ask's answers, funds each fee
// from the floating pool (CR 601.2b, 605.3b) and asks the choice filter's
// colour; the kernel mirrors that ask's answer order (mana_filterset.go
// filtSetGap): a choice ability's needed colour is hoisted ahead of the {C},
// a fixed-colour ability's is not. Seeds 1..40 all agree (a Black Knight's
// {B}{B} over a row of Keeps takes the {2} filter's {B} third, as gorge
// does), and the cast counts prove the reach: Grizzly Bears' {1}{G} has no
// green source but the Keep's {2} choice, Shock's {R} none but its {1}.
var kkoskunDecks = [2][]cardset.Entry{
	{
		{Name: "Koskun Keep", Count: 10}, {Name: "Swamp", Count: 15}, {Name: "Grizzly Bears", Count: 7},
		{Name: "Shock", Count: 4}, {Name: "Black Knight", Count: 4},
	},
	{
		{Name: "Plains", Count: 18}, {Name: "Savannah Lions", Count: 12}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Shock", Count: 6},
	},
}

// kkoskunSeeds are seeds 1..40, every one: seed%4 == 0 runs gorge's autopay
// surface, the rest its full surface.
var kkoskunSeeds = func() (s []uint64) {
	for i := uint64(1); i <= 40; i++ {
		s = append(s, i)
	}
	return s
}()

func TestKoskunKeepFilterSetLockstep(t *testing.T) {
	t.Parallel()
	reg, set, gd := lockstepPool(t, "koskunkeep", kkoskunDecks)
	tbl, _, _, err := set.Matchup(kkoskunDecks[0], kkoskunDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var bears, shock int
	for _, seed := range kkoskunSeeds {
		m, err := NewMatchup(reg, [2]string{"kk-a", "kk-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, pkPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s after %q\n diff %v\n ref %v\n ker %v\n detail %s",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.LastSourced, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
		}
		for _, a := range res.Actions {
			switch firstObjectCard(m, a.String()) {
			case "Grizzly Bears":
				bears++
			case "Shock":
				shock++
			}
		}
	}
	t.Logf("%d games: Grizzly Bears casts %d (Keep's {2} choice), Shock casts %d (Keep's {1})", len(kkoskunSeeds), bears, shock)
	if bears == 0 || shock == 0 {
		t.Errorf("Grizzly Bears casts %d, Shock casts %d; want both paid filter abilities reached", bears, shock)
	}
}
