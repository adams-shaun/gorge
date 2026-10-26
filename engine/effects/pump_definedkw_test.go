package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPumpDefinedKWSubstitutesTheChoice covers the corpus's DefinedKW$ shapes
// this build can substitute: ChosenColor (Skrelv, Mondo Gecko, Sungold
// Sentinel: Hexproof:ChosenColor) and ChosenType (Illusionary Presence,
// Giant Slug: Landwalk:ChosenType), plus the loud refusals -- no recorded
// choice withholds the keyword, an unsubstitutable value (the player-valued
// protection grants) grants as written with a Note.
func TestPumpDefinedKWSubstitutesTheChoice(t *testing.T) {
	cases := []struct {
		name   string
		script string
		colour string
		typ    string
		want   []string
		note   bool
	}{
		{"colour", "DB$ Pump | Defined$ Self | KW$ Hexproof:ChosenColor & Toxic:1 | DefinedKW$ ChosenColor", "U", "", []string{"Hexproof:blue", "Toxic:1"}, false},
		{"type", "DB$ Pump | Defined$ Self | KW$ Landwalk:ChosenType | DefinedKW$ ChosenType", "", "Swamp", []string{"Landwalk:Swamp"}, false},
		{"no colour recorded", "DB$ Pump | Defined$ Self | KW$ Hexproof:ChosenColor & Toxic:1 | DefinedKW$ ChosenColor", "", "", []string{"Toxic:1"}, true},
		{"unsupported value", "DB$ Pump | Defined$ Self | KW$ Protection:Player.PlayerUID_ChosenPlayerUID:ChosenPlayerName | DefinedKW$ ChosenPlayer", "", "",
			[]string{"Protection:Player.PlayerUID_ChosenPlayerUID:ChosenPlayerName"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g, ids := board(t)
			h := &fakeHost{g: g}
			src := ids["myBear"]
			g.Obj(src).ChosenColor, g.Obj(src).ChosenType = tc.colour, tc.typ
			if g.Obj(src).Zone != state.ZBattlefield {
				t.Fatal("precondition: the pump source must be on the battlefield")
			}
			Resolve(h, &Ctx{Source: src, Controller: 0}, sa(t, tc.script))
			var got []string
			for _, ce := range h.continuous {
				got = append(got, ce.AddKeywords...)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("granted keywords = %v, want %v", got, tc.want)
			}
			noted := false
			for _, ev := range h.log {
				if ev.Kind == events.Note {
					noted = true
				}
			}
			if noted != tc.note {
				t.Fatalf("Note emitted = %v, want %v", noted, tc.note)
			}
		})
	}
}
