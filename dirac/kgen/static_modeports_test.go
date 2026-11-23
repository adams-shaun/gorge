package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The T17b static-mode ports: each mode's census example cards lower.
// These are the sole-blocker carriers of the four classes — a card listed
// here unlocks exactly when the named mode lowers beside everything else it
// prints, so the precondition (every listed card refuses only at its mode
// today) is what the RED run pins.
//
// TestStatModePortsLower: MustBlock, UnspentMana, CanBlockIfShadow and
// IgnoreLegendRule lower AND BUILD through the shared static machinery — the
// mode body consumes the mode's params, readCond reads the shared gates, and
// the build emits the manifest kind (staticFor's per-kind builder). The pool
// is built, not just lowered: the LIR layer alone accepts any kind name, so
// only the build exercises the builder dispatch. gorge registers all four
// (engine/rules/statics.go, block_restrict.go, statics_echo.go, sba.go's
// IgnoreLegendRule walk).
func TestStatModePortsLower(t *testing.T) {
	for _, c := range []struct {
		mode  string
		names []string
	}{
		{"MustBlock", []string{"Iron Golem", "Razorgrass Screen", "Relentless Raptor", "Spirespine", "Watchdog", "Grand Melee"}},
		{"UnspentMana", []string{"Ashling, Flame Dancer", "Electro, Assaulting Battery", "Fangorn, Tree Shepherd", "Upwelling"}},
		{"CanBlockIfShadow", []string{"Aether Web", "Aetherflame Wall", "Heartwood Dryad", "Wall of Diffusion"}},
		{"IgnoreLegendRule", []string{"Brothers Yamazaki", "Cadric, Soul Kindler", "Council of Reeds", "Mirror Gallery", "Try-My-Deck Elemental"}},
	} {
		t.Run(c.mode, func(t *testing.T) {
			for _, c := range lowerPoolBuilt(t, c.names) {
				for _, r := range c.refusals {
					t.Errorf("%s: refused: %s", c.name, r.Reason)
				}
			}
		})
	}
}

// TestStatModePortsRefuses: the shapes the four modes' bodies do not model
// refuse by name, fail-closed the way gorge reads them — a MustBlock
// ValidCard$ binds nothing in gorge's matcher, an UnspentMana player scope
// the fold does not read would over-protect, an unknown ManaType$ word leaves
// the line out of gorge's keep set.
func TestStatModePortsRefuses(t *testing.T) {
	for _, c := range []struct {
		name   string
		params map[string]string
		want   string
	}{
		{"MustBlock ValidCard", map[string]string{"Mode": "MustBlock", "ValidCard": "Creature"}, "MustBlock ValidCard"},
		{"UnspentMana opponent scope", map[string]string{"Mode": "UnspentMana", "ValidPlayer": "Player.Opponent"}, "UnspentMana ValidPlayer"},
		{"UnspentMana unknown colour", map[string]string{"Mode": "UnspentMana", "ManaType": "Sprocket"}, "UnspentMana ManaType"},
	} {
		t.Run(c.name, func(t *testing.T) {
			lc := statLower(t, c.params)
			if len(lc.Refusals) == 0 || !strings.Contains(lc.Refusals[0].Reason, c.want) {
				t.Errorf("refusals %v, want one naming %q", lc.Refusals, c.want)
			}
		})
	}
}

// statLower lowers one synthesized S: static with the given params.
func statLower(t *testing.T, params map[string]string) *lCard {
	t.Helper()
	face := &cards.Face{Name: "Synthetic", ManaCost: "1 W", Types: []string{"Enchantment"},
		Statics: []cards.Static{{Mode: params["Mode"], Params: params}}}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}
