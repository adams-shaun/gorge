package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestProducibleSymbolsFailsClosedOnComboWords: a Produced$ that is a word,
// not a rune list, contributes no symbol. "Combo ColorIdentity" (Command
// Tower) used to shed its C into the reflected census, which a ColorOrType$
// Type read keeps: a Reflecting Pool then advertised a colourless the board
// could not make.
func TestProducibleSymbolsFailsClosedOnComboWords(t *testing.T) {
	cases := []struct{ name, produced, want string }{
		{"identity combo", "Combo ColorIdentity", ""},
		{"chosen combo", "Combo Chosen", ""},
		{"mixed combo naming colourless", "Combo W B G C", ""},
		{"colour combo", "Combo W B", "WB"},
		{"plain colourless", "C", "C"},
		{"plain rune list", "W U", "WU"},
		{"any", "Any", "WUBRG"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h := &fakeHost{g: state.NewGame(names(2))}
			src := h.g.AddObject(mkCard(t, "Name:Source\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ "+tc.produced+
				" | SpellDescription$ x\nOracle:x\n"), 0)
			h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{src.ID})
			o := h.g.Obj(src.ID)
			if o == nil || o.Face() == nil || len(o.Face().ManaAbilities()) == 0 {
				t.Fatal("precondition: the source has no mana ability")
			}
			if got := producibleSymbols(o); got != tc.want {
				t.Fatalf("producibleSymbols(%q) = %q, want %q", tc.produced, got, tc.want)
			}
		})
	}

	// The reflected census keeps {C} for a Type read, so the fix must be the
	// fold's read, not WidenType's delete: "Combo ColorIdentity" names nothing
	// here, and the Type read adds no colourless on its own.
	h := &fakeHost{g: state.NewGame(names(2))}
	tower := h.g.AddObject(mkCard(t, "Name:Tower\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Combo ColorIdentity"+
		" | SpellDescription$ x\nOracle:x\n"), 0)
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{tower.ID})
	s := sa(t, "AB$ ManaReflected | Cost$ T | ReflectProperty$ Produce | ColorOrType$ Type | Valid$ Land")
	if !ManaReflectedOf(s).WidenType {
		t.Fatal("precondition: the reflected read is not ColorOrType$ Type")
	}
	got := ManaReflectedFold(h, s, []state.ObjID{tower.ID})
	if slices.Contains(got, "C") || len(got) != 0 {
		t.Fatalf("Type reflected fold over Combo ColorIdentity = %v, want no symbols", got)
	}
}
