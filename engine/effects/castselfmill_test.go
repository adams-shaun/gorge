package effects_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCastNameKeyedSelfMillClass pins the self-mill guard's two IR facts on
// the corpus: the name-keyed own-library digs are in the class, the
// type-keyed ones (an ordinary deck holds the stopping card) and the
// targeted ones (another player's library) are not, and the empty-library
// winners are told apart by how they win.
func TestCastNameKeyedSelfMillClass(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cases := []struct {
		name     string
		selfMill bool
		win      uint8
	}{
		{"Demonic Consultation", true, effects.EmptyLibraryWinNone},
		{"Spoils of the Vault", true, effects.EmptyLibraryWinNone},
		{"Tainted Pact", true, effects.EmptyLibraryWinNone},
		{"Primal Surge", false, effects.EmptyLibraryWinNone},
		{"Rally the Horde", false, effects.EmptyLibraryWinNone},
		{"Tunnel Vision", false, effects.EmptyLibraryWinNone},
		{"Ad Nauseam", false, effects.EmptyLibraryWinNone},
		{"Thassa's Oracle", false, effects.EmptyLibraryWinETB},
		{"Laboratory Maniac", false, effects.EmptyLibraryWinDraw},
		{"Jace, Wielder of Mysteries", false, effects.EmptyLibraryWinDraw},
		{"Lightning Bolt", false, effects.EmptyLibraryWinNone},
	}
	for _, tc := range cases {
		c, ok := reg.Lookup(tc.name)
		if !ok {
			t.Fatalf("corpus has no %s", tc.name)
		}
		f := c.Faces[0]
		if got := effects.CastNameKeyedSelfMill(f); got != tc.selfMill {
			t.Errorf("%s: CastNameKeyedSelfMill = %v, want %v", tc.name, got, tc.selfMill)
		}
		if got := effects.EmptyLibraryWin(f); got != tc.win {
			t.Errorf("%s: EmptyLibraryWin = %d, want %d", tc.name, got, tc.win)
		}
	}
}
