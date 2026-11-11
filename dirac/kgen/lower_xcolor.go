package kgen

import "strings"

// xcolorParam reads an activated ability's XColor$ ("Spend only black mana on
// X": Crypt Rats), gorge pay.AbilityXColor: one colour, a colour word or a
// letter, makes the cost's {X} that colour's pips once X is announced
// (Cost.WithX); a multi-colour or unknown value, or a cost with no mana {X},
// leaves the X generic, so the parameter is read and dropped. gorge reads it
// on no spell or trigger, so kgen's unread-params refusal stays there. The
// kernel's fold (cost_xcolor.go) prices one {X} beside other mana and {T}
// only; any other part beside it refuses by name.
func xcolorParam(c *lCost, ps *params, activated bool) error {
	if !activated {
		return nil
	}
	v, ok := ps.get("XColor")
	if !ok {
		return nil
	}
	col := xColorIndex(strings.TrimSpace(v))
	if col == 0 || !c.Mana.X {
		return nil
	}
	rest := *c
	rest.Tap, rest.Mana = false, manaCost{}
	if c.Mana.XX || len(c.Mana.Hybrid) != 0 || len(c.Mana.Twobrid) != 0 || len(c.Mana.Phyrexian) != 0 || !rest.pureMana() {
		return refuse("XColor$ %s beside a cost part other than mana and {T}", strings.TrimSpace(v))
	}
	c.XColor = col
	return nil
}

// xColorIndex is 1 + the WUBRG index of one colour word ("Black") or letter
// ("R"), 0 for anything else (gorge effects.ColorLetters' vocabulary, one
// colour only).
func xColorIndex(v string) int {
	for i, w := range [...]string{"white", "blue", "black", "red", "green"} {
		if strings.EqualFold(v, w) || v == "WUBRG"[i:i+1] {
			return i + 1
		}
	}
	return 0
}
