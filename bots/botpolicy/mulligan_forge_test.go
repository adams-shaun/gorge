package botpolicy

import (
	"slices"
	"testing"
)

func creatureCard(cmc, power int32, cost string) Card {
	return Card{PrintedName: "creature " + cost, Creature: true, Power: power, CMC: cmc, ManaCost: cost, Castable: true}
}

func instantCard(cmc int32, cost string) Card {
	return Card{PrintedName: "instant " + cost, InstantSpeed: true, CMC: cmc, ManaCost: cost, Castable: true}
}

func forgeBoard(cs []Card, deckLands, deckTotal int) Board {
	b := handBoard(MulliganForge, cs...)
	b.OwnDeck = manifestWithLands(deckLands, deckTotal)
	return b
}

func constructedBoard(cs []Card, deckLands, deckTotal int) Board {
	b := handBoard(MulliganConstructed, cs...)
	b.OwnDeck = manifestWithLands(deckLands, deckTotal)
	return b
}

// forgeKeeps runs the keep/mulligan ask and reports the chosen kind, asserting
// the rule consumes no rng.
func forgeKeeps(t *testing.T, b Board, bottom int) bool {
	t.Helper()
	d := keepMullDecision(bottom, true)
	r := rng(5)
	in := Decide(b, &d, r)
	if r.Uint64() != rng(5).Uint64() {
		t.Fatal("forge mulligan consumed the rng")
	}
	return d.Options[in.Choices[0]].Kind == "keep"
}

func TestForgeKeepOpeningSeven(t *testing.T) {
	for lands := 0; lands <= 7; lands++ {
		b := forgeBoard(sevenWith(lands), 24, 60)
		want := lands >= 2 && lands <= 5
		if got := forgeKeeps(t, b, 0); got != want {
			t.Errorf("%d lands: forge keep = %v, want %v", lands, got, want)
		}
		if got := forgeKeeps(t, constructedBoard(sevenWith(lands), 24, 60), 0); got != want {
			t.Errorf("%d lands: constructed keep = %v, want the same %v", lands, got, want)
		}
	}
}

func TestForgeKeepAfterMulliganFlood(t *testing.T) {
	flood := append([]Card{plainsCard(), plainsCard(), plainsCard(), plainsCard(), plainsCard(), plainsCard()}, spellCard(2, "1 W"))
	if !forgeKeeps(t, forgeBoard(flood, 24, 60), 1) {
		t.Error("6 Plains + a spell after one mulligan: forge must keep (5/6 lands)")
	}
	if forgeKeeps(t, constructedBoard(flood, 24, 60), 1) {
		t.Error("6 Plains + a spell after one mulligan: constructed must mulligan")
	}

	if forgeKeeps(t, forgeBoard(sevenWith(7), 24, 60), 1) {
		t.Error("7 Plains after one mulligan: forge must mulligan (6/6 lands, ratio 3)")
	}
	if forgeKeeps(t, constructedBoard(sevenWith(7), 24, 60), 1) {
		t.Error("7 Plains after one mulligan: constructed must mulligan")
	}

	if forgeKeeps(t, forgeBoard(sevenWith(0), 24, 60), 1) {
		t.Error("no-land seven after one mulligan: forge must mulligan")
	}
	if forgeKeeps(t, constructedBoard(sevenWith(0), 24, 60), 1) {
		t.Error("no-land seven after one mulligan: constructed must mulligan")
	}
}

func TestForgeKeepsAfterTwoMulligans(t *testing.T) {
	for _, lands := range []int{0, 7} {
		if !forgeKeeps(t, forgeBoard(sevenWith(lands), 24, 60), 2) {
			t.Errorf("%d lands after two mulligans: forge must always keep", lands)
		}
		if forgeKeeps(t, constructedBoard(sevenWith(lands), 24, 60), 2) {
			t.Errorf("%d lands after two mulligans: constructed must mulligan", lands)
		}
	}
}

func TestForgeKeepSpellHeavyDeck(t *testing.T) {
	if !forgeKeeps(t, forgeBoard(sevenWith(1), 8, 60), 0) {
		t.Error("one land in an 8/60 deck (ratio 7): forge must keep")
	}
	if forgeKeeps(t, constructedBoard(sevenWith(1), 8, 60), 0) {
		t.Error("one land in an 8/60 deck: constructed must mulligan")
	}
	if forgeKeeps(t, forgeBoard(sevenWith(1), 10, 60), 0) {
		t.Error("one land in a 10/60 deck (ratio 5): forge must mulligan")
	}
	if forgeKeeps(t, constructedBoard(sevenWith(1), 10, 60), 0) {
		t.Error("one land in a 10/60 deck: constructed must mulligan")
	}
}

func TestForgeKeepLandHeavyDeck(t *testing.T) {
	for _, lands := range []int{7, 6} {
		if !forgeKeeps(t, forgeBoard(sevenWith(lands), 40, 60), 0) {
			t.Errorf("%d lands in a 40/60 deck (ratio 1): forge must keep", lands)
		}
		if forgeKeeps(t, constructedBoard(sevenWith(lands), 40, 60), 0) {
			t.Errorf("%d lands in a 40/60 deck: constructed must mulligan", lands)
		}
	}
}

func TestForgeKeepNoLandDeck(t *testing.T) {
	if !forgeKeeps(t, forgeBoard(sevenWith(0), 0, 60), 0) {
		t.Error("no-land deck: forge must keep (the library holds no lands)")
	}
	if forgeKeeps(t, constructedBoard(sevenWith(0), 0, 60), 0) {
		t.Error("no-land deck: constructed must mulligan")
	}
}

func TestForgeKeepIgnoresColour(t *testing.T) {
	hand := []Card{swampCard(), swampCard(), swampCard(),
		spellCard(2, "1 W"), spellCard(2, "1 W"), spellCard(2, "1 W"), spellCard(2, "1 W")}
	if !forgeKeeps(t, forgeBoard(hand, 24, 60), 0) {
		t.Error("3 Swamps and white spells: forge must keep (no colour test)")
	}
	if forgeKeeps(t, constructedBoard(hand, 24, 60), 0) {
		t.Error("3 Swamps and white spells: constructed must mulligan")
	}
}

func TestForgeKeepNilDeck(t *testing.T) {
	b := handBoard(MulliganForge, sevenWith(1)...)
	if forgeKeeps(t, b, 0) {
		t.Error("nil deck, one land: forge must mulligan (neutral ratio 3)")
	}
	b = handBoard(MulliganForge, sevenWith(3)...)
	if !forgeKeeps(t, b, 0) {
		t.Error("nil deck, three lands: forge must keep")
	}
}

func TestForgeKeepCensusMismatch(t *testing.T) {
	b := handBoard(MulliganForge, sevenWith(0)...)
	b.HandSize = 8
	if !forgeKeeps(t, b, 0) {
		t.Error("census mismatch: forge must keep")
	}
	b = handBoard(MulliganConstructed, sevenWith(0)...)
	b.HandSize = 8
	if !forgeKeeps(t, b, 0) {
		t.Error("census mismatch: constructed must keep")
	}
}

func TestForgeBottomNonProducingLandFirst(t *testing.T) {
	hand := []Card{plainsCard(), plainsCard(), plainsCard(), wildsCard(),
		spellCard(2, "1 W"), spellCard(2, "1 W"), spellCard(2, "1 W")}
	d := bottomDecision(7, 1)
	got := Decide(handBoard(MulliganForge, hand...), &d, rng(1)).Choices
	if !slices.Equal(got, []int{3}) {
		t.Errorf("forge bottoming = %v, want [3] (Evolving Wilds)", got)
	}
	off := Decide(handBoard(MulliganConstructed, hand...), &d, rng(1)).Choices
	if !slices.Equal(off, []int{0}) {
		t.Errorf("constructed bottoming = %v, want [0]", off)
	}
}

func TestForgeBottomBasicWithMostCopies(t *testing.T) {
	hand := []Card{plainsCard(), swampCard(), swampCard(), swampCard(),
		spellCard(2, "1 B"), spellCard(2, "1 B"), spellCard(2, "1 B")}
	d := bottomDecision(7, 1)
	got := Decide(handBoard(MulliganForge, hand...), &d, rng(1)).Choices
	if !slices.Equal(got, []int{1}) {
		t.Errorf("forge bottoming = %v, want [1] (a Swamp: three copies)", got)
	}
	off := Decide(handBoard(MulliganConstructed, hand...), &d, rng(1)).Choices
	if !slices.Equal(off, []int{0}) {
		t.Errorf("constructed bottoming = %v, want [0] (the Plains)", off)
	}
}

func TestForgeBottomUncastableSpell(t *testing.T) {
	hand := []Card{plainsCard(), plainsCard(), spellCard(2, "1 W"),
		spellCard(5, "4 W"), spellCard(4, "3 W"), spellCard(1, "W"), spellCard(3, "2 W")}
	d := bottomDecision(7, 2)
	got := Decide(handBoard(MulliganForge, hand...), &d, rng(1)).Choices
	if !slices.Equal(got, []int{3, 4}) {
		t.Errorf("forge bottoming = %v, want [3,4]", got)
	}
	off := Decide(handBoard(MulliganConstructed, hand...), &d, rng(1)).Choices
	if !slices.Equal(off, []int{3, 4}) {
		t.Errorf("constructed bottoming = %v, want [3,4]", off)
	}
}

func TestForgeBottomWorstCreature(t *testing.T) {
	hand := []Card{plainsCard(), plainsCard(), plainsCard(),
		creatureCard(1, 1, "W"), creatureCard(2, 3, "1 W"), creatureCard(3, 2, "2 W"), creatureCard(2, 2, "1 W")}
	d := bottomDecision(7, 1)
	got := Decide(handBoard(MulliganForge, hand...), &d, rng(1)).Choices
	if !slices.Equal(got, []int{3}) {
		t.Errorf("forge bottoming = %v, want [3] (lowest power)", got)
	}
	off := Decide(handBoard(MulliganConstructed, hand...), &d, rng(1)).Choices
	if !slices.Equal(off, []int{5}) {
		t.Errorf("constructed bottoming = %v, want [5] (highest CMC)", off)
	}
}

func TestForgeBottomCheapestIsLastLand(t *testing.T) {
	hand := []Card{plainsCard(), plainsCard(), plainsCard(),
		instantCard(1, "W"), instantCard(2, "1 W"), instantCard(2, "1 W"), instantCard(3, "2 W")}
	d := bottomDecision(7, 1)
	got := Decide(handBoard(MulliganForge, hand...), &d, rng(1)).Choices
	if !slices.Equal(got, []int{2}) {
		t.Errorf("forge bottoming = %v, want [2] (cheapest, ties last)", got)
	}
	off := Decide(handBoard(MulliganConstructed, hand...), &d, rng(1)).Choices
	if !slices.Equal(off, []int{6}) {
		t.Errorf("constructed bottoming = %v, want [6] (highest CMC)", off)
	}
}

func TestForgeBottomDoubleCountQuirk(t *testing.T) {
	hand := []Card{plainsCard(), plainsCard(), plainsCard(), plainsCard(),
		spellCard(2, "1 W"), spellCard(2, "1 W"), spellCard(2, "1 W")}
	d := bottomDecision(7, 2)
	got := Decide(handBoard(MulliganForge, hand...), &d, rng(1)).Choices
	if !slices.Equal(got, []int{0, 4}) {
		t.Errorf("forge bottoming = %v, want [0,4] (bottomed lands subtracted twice)", got)
	}
	if slices.Equal(got, []int{0, 1}) {
		t.Error("forge bottomed a second land: the double-subtraction quirk was fixed")
	}
}

func TestForgeRegisteredByName(t *testing.T) {
	rule, err := MulliganByName("forge")
	if err != nil {
		t.Fatalf("MulliganByName(forge): %v", err)
	}
	if rule != MulliganForge {
		t.Fatalf("MulliganByName(forge) = %d, want %d", rule, MulliganForge)
	}
	if got := MulliganForge.String(); got != "forge" {
		t.Fatalf("MulliganForge.String() = %q, want forge", got)
	}
	hand := append([]Card{plainsCard(), plainsCard(), plainsCard(), plainsCard(), plainsCard(), plainsCard()}, spellCard(2, "1 W"))
	b := forgeBoard(hand, 24, 60)
	d := keepMullDecision(1, true)
	want := Decide(b, &d, rng(3)).Choices
	got, ok := MulliganAnswer(b, MulliganForge, &d)
	if !ok || !slices.Equal(got, want) {
		t.Fatalf("MulliganAnswer = %v %v, want Decide's %v", got, ok, want)
	}
}
