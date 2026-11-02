package mull

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMulliganProducesComboLand pins the kernel-side "any" reading on a real
// Produced$ Combo dual: Bloodfell Caves is B/R on the card, but the choice
// marker kgen carries makes produces report all five colours, exactly as
// gorge's producesColour does for ManaProduction.Any. A basic still reports
// only its own colour, so the marker is not a blanket "all mana abilities
// are any".
func TestMulliganProducesComboLand(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	i, ok := set.Lookup("Bloodfell Caves")
	if !ok {
		t.Fatal("the fdn pool has no Bloodfell Caves")
	}
	caves := CardOf(*set.Def(i))
	if !caves.Land || !caves.Prod || !caves.ProdAny {
		t.Fatalf("bloodfell caves facts %+v", caves)
	}
	if caves.Produces != [5]bool{false, false, true, true, false} {
		t.Fatalf("bloodfell caves produces %v, want B/R", caves.Produces)
	}
	for col := 0; col < 5; col++ {
		if !produces(caves, col) {
			t.Errorf("bloodfell caves does not produce colour %d", col)
		}
	}

	plains := CardOf(vanilla.Table.Cards[vanilla.Plains])
	if !plains.Land || !plains.Prod || plains.ProdAny {
		t.Fatalf("plains facts %+v", plains)
	}
	if !produces(plains, 0) {
		t.Error("plains does not produce white")
	}
	for col := 1; col < 5; col++ {
		if produces(plains, col) {
			t.Errorf("plains produces colour %d", col)
		}
	}
}

// TestForgeCardFacts pins the CardOf fields the forge policy reads: a basic
// land, a vanilla creature's printed power, and a non-producing land.
func TestForgeCardFacts(t *testing.T) {
	plains := CardOf(vanilla.Table.Cards[vanilla.Plains])
	if !plains.Land || !plains.Basic || plains.Creature || !plains.Prod ||
		plains.PrintedName != "Plains" {
		t.Fatalf("plains facts %+v", plains)
	}
	bears := CardOf(vanilla.Table.Cards[vanilla.GrizzlyBears])
	if bears.Land || bears.Basic || !bears.Creature || bears.Power != 2 ||
		bears.CMC != 2 || bears.PrintedName != "Grizzly Bears" {
		t.Fatalf("grizzly bears facts %+v", bears)
	}
	set := cardsettest.Set(t, "fdn")
	i, ok := set.Lookup("Evolving Wilds")
	if !ok {
		t.Fatal("the fdn pool has no Evolving Wilds")
	}
	wilds := CardOf(*set.Def(i))
	if !wilds.Land || wilds.Basic || wilds.Creature || wilds.Prod {
		t.Fatalf("evolving wilds facts %+v", wilds)
	}
}

// TestForgeBottoming pins the non-producing-land-first branch of the forge
// bottom rule (gorge forgeTuck): a lone bottom from three Plains, an Evolving
// Wilds and three Bears takes the Wilds.
func TestForgeBottoming(t *testing.T) {
	plains := CardOf(vanilla.Table.Cards[vanilla.Plains])
	bears := CardOf(vanilla.Table.Cards[vanilla.GrizzlyBears])
	set := cardsettest.Set(t, "fdn")
	i, ok := set.Lookup("Evolving Wilds")
	if !ok {
		t.Fatal("the fdn pool has no Evolving Wilds")
	}
	wilds := CardOf(*set.Def(i))
	hand := []Card{plains, plains, plains, wilds, bears, bears, bears}
	var w Work
	got := BottomForge(hand, 1, &w)
	if len(got) != 1 || got[0] != 3 {
		t.Fatalf("BottomForge = %v, want [3]", got)
	}
}
