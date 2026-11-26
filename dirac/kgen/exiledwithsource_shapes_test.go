package kgen

import (
	"strings"
	"testing"
)

// TestDemoExiledwithsourceShapes: the ExiledWithSource readers outside a
// resolution-time mover lower. Ugin's Labyrinth reads the link in its mana
// ability's Amount$ (Count$ValidExile), Valakut Exploration in its end-step
// Phase trigger's CheckSVar$ (Count$ValidExile), and Vault 13 in the Saga
// chapters that exile and return the cards (ExileUntilLeaves then a hidden
// pick and a sweep, which remember what they return). The read is the
// matcher's own source, which the kernel's exwFlag takes from ctx.src.
func TestDemoExiledwithsourceShapes(t *testing.T) {
	names := []string{"Ugin's Labyrinth", "Valakut Exploration", "Vault 13: Dweller's Journey"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	seen := map[string]*lCard{}
	for _, c := range lw.cards {
		seen[c.Name] = c
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	dump := map[string]string{}
	for _, c := range lw.cards {
		var sb strings.Builder
		dumpLIR(&sb, c)
		dump[c.Name] = sb.String()
		if !c.ExiledWith {
			t.Errorf("%s lowers without recording its ExiledWithSource read", c.Name)
		}
	}
	for _, w := range []struct{ card, want string }{
		// the mana amount's Count$Compare reads the link during resolution
		{"Ugin's Labyrinth", "CountExile({types=0x0 ExiledWith})"},
		// the end-step trigger's condition reads it as the trigger fires
		{"Valakut Exploration", "trigger Phase"},
		{"Valakut Exploration", "CountExile({types=0x0 ExiledWith})"},
		// the Saga chapter III returns two and the sweep leaves them behind
		{"Vault 13: Dweller's Journey", "ChooseMove obj=You A=2 C=1 CS=from=Exile|Remember|Mandatory"},
		{"Vault 13: Dweller's Journey", "NotRemembered ExiledWith"},
		{"Vault 13: Dweller's Journey", "ExileUntilLeaves"},
	} {
		if !strings.Contains(dump[w.card], w.want) {
			t.Errorf("%s: LIR lacks %q:\n%s", w.card, w.want, dump[w.card])
		}
	}
	if c := seen["Vault 13: Dweller's Journey"]; c == nil || c.SagaN != 3 {
		t.Errorf("Vault 13: SagaN %v, want 3", c)
	}
}
