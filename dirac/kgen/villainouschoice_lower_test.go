package kgen

import (
	"strings"
	"testing"
)

// TestVillainousChoiceLowers pins api:VillainousChoice's lowering
// (lower_villainous.go): the five carriers whose branches all lower go end
// to end, each as a RepeatEach over the victims or a targeted chooser
// with the choose-branch op's C 1 (every choice offered, payable or not, as
// gorge's KModes does). Ensnared by the Mara reads Remembered after its
// own RememberFound$/RememberChanged$, a mixed player-and-card list the
// kernel's card-only remembered set cannot tell apart, so it refuses by that
// reason, never by the api name.
func TestVillainousChoiceLowers(t *testing.T) {
	lowered := []string{"Damocles Base, Sword of Kang", "Dr. Eggman", "Genesis of the Daleks",
		"Great Intelligence's Plan", "The Dalek Emperor"}
	refused := map[string]string{
		"Ensnared by the Mara":      "reads Remembered after remembering cards",
		"Sycorax Commander":         "unknown property",
		"This Is How It Ends":       "unknown property",
		"Midnight Crusader Shuttle": "unread params",
	}
	names := append([]string{}, lowered...)
	for n := range refused {
		names = append(names, n)
	}
	kp := kfhBuild(t, names, nil)
	for _, n := range lowered {
		if got := refusalText(kp.cards[n]); got != "" {
			t.Errorf("%s: refused: %s", n, got)
		}
	}
	for n, want := range refused {
		got := refusalText(kp.cards[n])
		if !strings.Contains(got, want) {
			t.Errorf("%s: refusals %q, want %q", n, got, want)
		}
		if strings.Contains(got, "api VillainousChoice") {
			t.Errorf("%s: still refused at the api: %s", n, got)
		}
	}

	// Defined$ Opponent loops over the victims; ValidTgts$ Opponent is one
	// targeted chooser with no loop.
	lir, _ := lowerCarriers(t, []string{"Great Intelligence's Plan", "Dr. Eggman"})
	plan := section(lir, "Great Intelligence's Plan")
	if strings.Count(plan, "GenericChoice obj=Target0 A=2 C=1") != 1 || strings.Contains(plan, "RepeatEach") {
		t.Errorf("Great Intelligence's Plan: want one targeted GenericChoice C=1 and no RepeatEach:\n%s", plan)
	}
	egg := section(lir, "Dr. Eggman")
	if strings.Count(egg, "RepeatEach obj=EachOpp") != 1 || strings.Count(egg, "GenericChoice obj=Each A=2 C=1") != 1 {
		t.Errorf("Dr. Eggman: want a RepeatEach over the opponents with one Each GenericChoice C=1:\n%s", egg)
	}
}
