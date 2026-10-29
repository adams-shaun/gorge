package neutral_test

import (
	"fmt"
	"testing"
)

// Farsight Ritual has no ChangeValid$, so every looked-at card is eligible
// (effects.compileDig defaults the spec to Card). The rebuild must offer all
// four cards; the bot takes the first Max two in offered order, which
// hiddenOrder/xview sort by card name: Island, Scroll Thief.
func TestFarsightDigPartitionTakesByName(t *testing.T) {
	tb := newTable(t, "Farsight Ritual", "Island", "Scroll Thief", "Training Grounds", "Thalakos Dreamsower")
	src := tb.spell("o-fr", "Farsight Ritual")
	window := []string{"Training Grounds", "Scroll Thief", "Island", "Thalakos Dreamsower"}
	refs := tb.look("looked_at", window...)
	steps := 2*len(window) - 1
	dest := map[string]string{}
	for i := range window {
		var cands []string
		for _, d := range []string{"hand", "bottom"} {
			cands = append(cands, fmt.Sprintf(`{"kind":"arrange_card","source":%s,"purpose":"dig","card":%s,"card_index":%d,"card_count":%d,"destination":%q}`,
				src, refs[i], i, len(window), d))
		}
		dest[window[i]] = tb.ask(choiceCtx(src, "dig"), [2]int{i, steps}, cands)["destination"].(string)
	}
	tb.clean()
	if dest["Island"] != "hand" || dest["Scroll Thief"] != "hand" {
		t.Fatalf("partition %v, want the first two by name in hand", dest)
	}
	if dest["Training Grounds"] != "bottom" || dest["Thalakos Dreamsower"] != "bottom" {
		t.Fatalf("partition %v, want the rest on the bottom", dest)
	}
}
