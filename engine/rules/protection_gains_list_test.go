package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestProtectionGainsListIsEachQuality: Protection Gains$ green,white (Grotesque
// Hybrid: "protection from green and from white") protects from a green source
// and from a white one, and not from a red one: each comma-separated word is
// its own quality (CR 702.16g), as Forge's ProtectEffect splits the list.
func TestProtectionGainsListIsEachQuality(t *testing.T) {
	t.Parallel()
	protectionGainsCase(t, "green,white", true)
}

// TestProtectionGainsSingleQuality is the harness control: Gains$ green alone
// protects from the green source only.
func TestProtectionGainsSingleQuality(t *testing.T) {
	t.Parallel()
	protectionGainsCase(t, "green", false)
}

func protectionGainsCase(t *testing.T, gains string, wantWhite bool) {
	t.Helper()
	const fixture = "Name:Mass Ward\nManaCost:0\nTypes:Sorcery\nOracle:x\n"
	const mine = "Name:Ward Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	const green = "Name:Green Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	const white = "Name:White Knight\nManaCost:1 W\nTypes:Creature Knight\nPT:2/2\nOracle:x\n"
	const red = "Name:Red Giant\nManaCost:1 R\nTypes:Creature Giant\nPT:2/2\nOracle:x\n"
	e, _ := putCounterTable(t, 61,
		[]*cards.Card{card(t, fixture), card(t, mine)}, []*cards.Card{card(t, green), card(t, white), card(t, red)})
	bear := moveSeeded(t, e, 0, mine, state.ZBattlefield)
	g := moveSeeded(t, e, 1, green, state.ZBattlefield)
	w := moveSeeded(t, e, 1, white, state.ZBattlefield)
	r := moveSeeded(t, e, 1, red, state.ZBattlefield)
	source := moveSeeded(t, e, 0, fixture, state.ZStack)
	e.G.Obj(source).Ability = &cards.SA{Kind: "SP", API: "Protection", Params: map[string]string{
		"Defined": "Self", "Gains": gains,
	}}
	e.G.Obj(source).Source = bear
	e.pending = nil
	if e.protectedFrom(bear, g) || e.protectedFrom(bear, w) {
		t.Fatalf("the creature is protected before the grant")
	}
	e.resolveTop()
	if !e.protectedFrom(bear, g) {
		t.Errorf("no protection from the green source")
	}
	if e.protectedFrom(bear, w) != wantWhite {
		t.Errorf("protection from the white source = %v, want %v", !wantWhite, wantWhite)
	}
	if e.protectedFrom(bear, r) {
		t.Errorf("protection from the red source")
	}
}
