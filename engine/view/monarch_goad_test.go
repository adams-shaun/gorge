package view

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestProjectMonarchIsPublic pins PlayerView.IsMonarch: the CR 725 monarch
// is shown on the one seat that holds it, to every viewer, and the key is
// absent on the wire for every other seat.
func TestProjectMonarchIsPublic(t *testing.T) {
	g := fourSeatBoard(t)
	g.HasMonarch, g.Monarch = true, 2
	for viewer := state.PlayerID(0); viewer < 4; viewer++ {
		v := Project(g, flatChars{g}, viewer, nil)
		for i, pv := range v.Players {
			if pv.IsMonarch != (i == 2) {
				t.Errorf("viewer %d: seat %d IsMonarch = %v, want %v", viewer, i, pv.IsMonarch, i == 2)
			}
		}
	}
	g.HasMonarch = false
	v := Project(g, flatChars{g}, 0, nil)
	b, err := json.Marshal(v.Players[2])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "is_monarch") {
		t.Errorf("no monarch: seat 2 serialises %s, want no is_monarch key", b)
	}
}

// TestProjectGoadedBy pins CardView.GoadedBy, read from state: a
// battlefield creature lists, in seat order and once each, the seats whose
// goad relationship on it is in force for every viewer; an "as long as" goad
// whose source has left is not; a creature nobody goads lists none and omits
// the key.
func TestProjectGoadedBy(t *testing.T) {
	g := fourSeatBoard(t)
	goaded := g.Zone(state.ZBattlefield, 0)[0]
	gone := g.Zone(state.ZLibrary, 2)[0]
	g.Obj(goaded).Goads = []state.GoadEffect{
		{Player: 3}, {Player: 1}, {Player: 3},
		{Player: 2, Source: gone, Duration: "AsLongAsInPlay"},
	}
	for viewer := state.PlayerID(0); viewer < 4; viewer++ {
		v := Project(g, flatChars{g}, viewer, nil)
		if got := v.Players[0].Battlefield[0].GoadedBy; !slices.Equal(got, []state.PlayerID{1, 3}) {
			t.Errorf("viewer %d: GoadedBy = %v, want [1 3]", viewer, got)
		}
		if got := v.Players[1].Battlefield[0].GoadedBy; got != nil {
			t.Errorf("viewer %d: an ungoaded creature's GoadedBy = %v, want nil", viewer, got)
		}
	}
	cv := Project(g, flatChars{g}, 0, nil).Players[1].Battlefield[0]
	b, err := json.Marshal(cv)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "goaded_by") {
		t.Errorf("an ungoaded card serialises %s, want no goaded_by key", b)
	}
}
