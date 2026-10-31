package pickbot_test

import (
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestBuilderPoolRarityReachesHeuristic pins the pool helper's rarity
// derivation: builderPool must carry each card's own printing rarity, so a
// pool card with no 17Lands rating falls back to Heuristic at its real
// rarity base rather than the common default of 0. The assertion uses a nil
// card so a creature's P/T term cannot mask a rarityBase of 0.
func TestBuilderPoolRarityReachesHeuristic(t *testing.T) {
	pool := builderPool(t)
	var ajani *limited.CardRef
	for i := range pool {
		if pool[i].Name == "Ajani, Caller of the Pride" {
			ajani = &pool[i]
		}
	}
	if ajani == nil {
		t.Fatal("builder pool does not contain Ajani, Caller of the Pride")
	}
	// Precondition: the pool card is a non-zero-base rarity. A pool that
	// derived its rarities from the lowercase sheet key would carry 'm'
	// here, and the base below would be 0.
	if ajani.Rarity != limited.Mythic {
		t.Fatalf("pool rarity for %q = %q, want %q", ajani.Name, ajani.Rarity, limited.Mythic)
	}
	if got := pb.Heuristic(nil, ajani.Rarity); got != 6 {
		t.Fatalf("Heuristic(nil, %q) = %g, want 6", ajani.Rarity, got)
	}
	// Precondition: the card really is a mythic on the FDN mythic sheet, so
	// the assertion above is not satisfied by a coincidentally-uppercase key.
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	found := false
	for _, sheet := range spec.Config.Sheets {
		for _, c := range sheet.Cards {
			if c.Name == ajani.Name && c.Rarity == limited.Mythic {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("%q is not a mythic on any FDN spec sheet", ajani.Name)
	}
}
