package cards

// CR 605.1a: an activated ability is a mana ability only if "its cost and
// effect don't move any card to or from a library". IsManaAbilitySA reads the
// head's Cost$ (movesLibraryCard's costMovesLibraryCard) and every
// SubAbility$ node (effectMovesLibraryCard), so Chromatic Sphere's "Add one
// mana of any color. Draw a card." and Millikin's "{T}, Mill a card: Add {C}"
// take the ordinary activated-ability path (the stack), as Forge does.

import (
	"slices"
	"testing"
)

// libraryMovingManaCarriers are the printed AB$ Mana / ManaReflected /
// ChooseColor-chain abilities whose cost or effect moves a library card,
// measured over the pinned corpus (FORGE_REF): the chain carriers draw (the
// Spheres and Eggs, Rainbow Dash) or dig (Chandra, Flameshaper's +2), and the
// cost carriers mill one card.
var libraryMovingManaCarriers = []string{
	"Chandra, Flameshaper", "Charmed Pendant", "Chromatic Sphere", "Darkwater Egg",
	"Deranged Assistant", "Manakin and Millikin", "Millikin", "Mossfire Egg",
	"Rainbow Dash", "Shadowblood Egg", "Skycloud Egg", "Sungrass Egg",
}

func TestCR605_1aManaAbilityLibraryGateMovesTheLibraryCarriers(t *testing.T) {
	reg := openCardsTestCorpus(t)
	var got []string
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, a := range f.Abilities {
				if a == nil || !a.IsActivated() || requiresTarget(a) || !movesLibraryCard(a) {
					continue
				}
				if !IsManaAbilityAPI(a.API) && !(a.API == "ChooseColor" && ManaChainProduction(a) != nil) {
					continue
				}
				if IsManaAbilitySA(a) {
					t.Errorf("CR 605.1a %s: a library-moving %s ability still classifies as a mana ability: %s", f.Name, a.API, a.Line)
				}
				if slices.Contains(f.ManaAbilities(), a) {
					t.Errorf("CR 605.1a %s: a library-moving ability is listed in Face.ManaAbilities(): %s", f.Name, a.Line)
				}
				got = append(got, f.Name)
			}
		}
	}
	slices.Sort(got)
	got = slices.Compact(got)
	if want := slices.Sorted(slices.Values(libraryMovingManaCarriers)); !slices.Equal(got, want) {
		t.Errorf("CR 605.1a library-moving mana-shaped abilities = %v, want %v", got, want)
	}
}

// TestCR605_1aLibraryMoveShapes pins movesLibraryCard on hand-built heads:
// each cost token and chain API that moves a library card disqualifies, and
// the near misses (a graveyard-to-hand return, a hand exile, a Scry that only
// reorders, a Manifest from hand) do not.
func TestCR605_1aLibraryMoveShapes(t *testing.T) {
	head := func(cost string, sub *SA) *SA {
		return &SA{Kind: "AB", API: "Mana", Sub: sub, Params: map[string]string{"Cost": cost, "Produced": "C"}}
	}
	db := func(api string, kv ...string) *SA {
		p := map[string]string{}
		for i := 0; i+1 < len(kv); i += 2 {
			p[kv[i]] = kv[i+1]
		}
		return &SA{Kind: "DB", API: api, Params: p}
	}
	for _, tc := range []struct {
		name string
		sa   *SA
		want bool
	}{
		{"bare tap", head("T", nil), false},
		{"Mill cost", head("T Mill<1>", nil), true},
		{"Draw cost", head("T Draw<1/You>", nil), true},
		{"ExileFromTop cost", head("T ExileFromTop<1/Card>", nil), true},
		{"PutCardToLib cost", head("T PutCardToLibFromHand<1/0/Card>", nil), true},
		{"Sac cost", head("T Sac<1/CARDNAME>", nil), false},
		{"Draw sub", head("T", db("Draw")), true},
		{"Dig sub two deep", head("T", withSub(db("Pump", "Defined", "Self"), db("Dig", "DigNum", "3"))), true},
		{"ChangeZone to library", head("T", db("ChangeZone", "Origin", "Graveyard", "Destination", "Library")), true},
		{"ChangeZone from Graveyard,Library", head("T", db("ChangeZone", "Origin", "Graveyard,Library", "Destination", "Hand")), true},
		{"ChangeZone graveyard to hand", head("T", db("ChangeZone", "Origin", "Graveyard", "Destination", "Hand")), false},
		{"ChangeZoneAll from All", head("T", db("ChangeZoneAll", "Origin", "All", "Destination", "Exile")), true},
		{"Scry", head("T", db("Scry", "ScryNum", "1")), false},
		{"Manifest top", head("T", db("Manifest")), true},
		{"Manifest from hand", head("T", db("Manifest", "ChoiceZone", "Hand")), false},
		{"Play from library", head("T", db("Play", "ValidZone", "Library")), true},
	} {
		if got := movesLibraryCard(tc.sa); got != tc.want {
			t.Errorf("CR 605.1a movesLibraryCard(%s) = %v, want %v", tc.name, got, tc.want)
		}
		if got := IsManaAbilitySA(tc.sa); got == tc.want {
			t.Errorf("CR 605.1a IsManaAbilitySA(%s) = %v, want %v", tc.name, got, !tc.want)
		}
	}
}

func withSub(s, sub *SA) *SA { s.Sub = sub; return s }
