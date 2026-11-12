package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestManaReflectedProduceLowers: the opponents'-production reflected shape
// (AB$ ManaReflected | ColorOrType$ Color | Valid$ Land.OppCtrl |
// ReflectProperty$ Produce: Exotic Orchard, Fellwar Stone, Quirion Explorer,
// Sylvok Explorer) lowers with no refusal, its mana ability an
// AfManaReflect one with ManaProduce and a ManaAmong filter, and the pool
// cards' ProduceCol is the printed mana abilities' produced colours ({C}
// dropped). A mixed YouCtrl list — gorge's planner admits it
// (pay/planreflected.go reflectsStableObjects), the kernel's one walk is the
// opponents' alone — is refused with a true reason.
func TestManaReflectedProduceLowers(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(
		"Exotic Orchard\nFellwar Stone\nQuirion Explorer\nSylvok Explorer\nCommand Tower\nIsland\nForest\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Exotic Orchard", "Fellwar Stone", "Quirion Explorer", "Sylvok Explorer"} {
		var c *lCard
		for _, lc := range lw.cards {
			if lc.Name == n {
				c = lc
				break
			}
		}
		if c == nil {
			t.Fatalf("%s was not lowered", n)
		}
		var reasons []string
		for _, r := range c.Refusals {
			reasons = append(reasons, r.Node+": "+r.Reason)
		}
		if len(reasons) != 0 {
			t.Errorf("%s refused: %s", n, strings.Join(reasons, "; "))
			continue
		}
		found := false
		for _, a := range c.Abilities {
			if a.Mana == nil || a.Mana.Special != "Reflected" || !a.Mana.Produce {
				continue
			}
			found = true
		}
		if !found {
			t.Errorf("%s: no Produce-mode reflected mana ability", n)
		}
	}
	// the pool cards' producible colours: Island U, Forest G. Command
	// Tower nothing, gorge too: its Produced$ "Combo ColorIdentity" leaves
	// producibleSymbols' rune walk a lone {C}, which ColorOrType$ Color
	// drops (effects.producibleSymbols).
	want := map[string]kop.ColorMask{
		"Island":        kop.ColU,
		"Forest":        kop.ColG,
		"Command Tower": 0,
	}
	for _, c := range lw.cards {
		if w, ok := want[c.Name]; ok && c.ProduceCol != w {
			t.Errorf("%s: ProduceCol %d, want %d", c.Name, c.ProduceCol, w)
		}
	}
}
