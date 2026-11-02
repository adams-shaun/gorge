package kgen

import (
	"strings"
	"testing"
)

// TestExwExilePick: a hidden ChangeZone pick out of
// exile reads Card.ExiledWithSource (Omenpath Journey, Northampton Farm) and
// lowers to a ChooseMove from=Exile; the hidden pick into exile (Wormfang
// Newt/Turtle) stays refused by exwCheck, and so do the shapes the mover
// does not read.
func TestExwExilePick(t *testing.T) {
	want := map[string]string{
		"Omenpath Journey": "from=Exile|Tapped|Random",
		"Northampton Farm": "from=Exile|UnderYourControl|Mandatory",
		"Purgatory":        "from=Exile|Mandatory",
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		w, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), "ChooseMove") || !strings.Contains(sb.String(), w) {
			t.Errorf("%s: lowered IR lacks a ChooseMove %q:\n%s", c.Name, w, sb.String())
		}
		if !c.ExiledWith || !c.ExwWord {
			t.Errorf("%s: ExiledWith %v ExwWord %v, want both", c.Name, c.ExiledWith, c.ExwWord)
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d carriers", seen, len(want))
	}

	// the hidden battlefield->exile pick of Wormfang Newt/Turtle stays refused
	// (Purgatory's dies-trigger exile is lockstep-proven)
	for _, c := range lowerNamed(t, []string{"Wormfang Newt", "Wormfang Turtle"}) {
		found := false
		op := "ChooseMove"
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "ExiledWith reader with a "+op+" exile") {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want the %s exwCheck refusal, got %v", c.Name, op, c.Refusals)
		}
	}

	// the shapes the mover does not read refuse by name
	for _, c := range []struct{ name, root, want string }{
		{"Optional", "SP$ ChangeZone | Cost$ B | Hidden$ True | Origin$ Exile | Destination$ Battlefield | ChangeType$ Creature | Optional$ True", "exile pick with Optional$"},
		{"ValidTgts", "SP$ ChangeZone | Cost$ B | Hidden$ True | Origin$ Exile | Destination$ Battlefield | ChangeType$ Creature | DefinedPlayer$ You", "exile pick with DefinedPlayer$"},
		{"ChangeNum", "SP$ ChangeZone | Cost$ B | Hidden$ True | Origin$ Exile | Destination$ Battlefield | ChangeType$ Creature | ChangeNum$ 2", "exile pick ChangeNum$"},
		{"Remember", "SP$ ChangeZone | Cost$ B | Hidden$ True | Origin$ Exile | Destination$ Battlefield | ChangeType$ Creature | RememberChanged$ True", "exile pick with RememberChanged$"},
		{"to hand", "SP$ ChangeZone | Cost$ B | Hidden$ True | Origin$ Exile | Destination$ Hand | ChangeType$ Creature", "ChangeType$ move Exile->Hand"},
		{"link outside a mover", "SP$ ChangeZone | Cost$ B | Origin$ Exile | Destination$ Battlefield | ChangeType$ Card.ExiledWithSource", "ExiledWithSource outside a resolution-time mover"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, _ := czLowered(t, c.root)
			kdefWant(t, got, c.want)
		})
	}
	// a filter that does not name the link lowers to the pick
	got, ops := czLowered(t, "SP$ ChangeZone | Cost$ B | Hidden$ True | Origin$ Exile | Destination$ Battlefield | Tapped$ True | AtRandom$ True | ChangeType$ Creature")
	if len(got) != 0 || len(ops) != 1 || ops[0] != "ChooseMove:from=Exile|Tapped|Random" {
		t.Errorf("synthetic pick: refusals %v ops %v", got, ops)
	}
}
