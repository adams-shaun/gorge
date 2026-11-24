package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestGarzasAssassinRecoverPaysHalfLife: Garza's Assassin's Recover cost is
// "pay half your life, rounded up" — the ask must offer a real Pay option and
// charge ceil(life/2), then return the card to hand. Odd life rounds up.
func TestGarzasAssassinRecoverPaysHalfLife(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ life, wantLife int32 }{{20, 10}, {15, 7}} {
		e, spell := recoverFixtureLife(t, 8117, "Garza's Assassin", tc.life)
		ask := recoverUnlessPayAsk(t, e, 0)
		if len(ask.Options) < 2 || ask.Options[0].Mode == "unless_decline" {
			t.Fatalf("life %d: want a Pay option before Don't pay, got %+v", tc.life, ask.Options)
		}
		submitChoices(t, e, ask.Options[0].Index)
		if got := e.G.Players[0].Life; got != tc.wantLife {
			t.Fatalf("life %d: life after Recover = %d, want %d", tc.life, got, tc.wantLife)
		}
		if z := e.G.Obj(spell).Zone; z != state.ZHand {
			t.Fatalf("life %d: Garza's Assassin zone = %s, want Hand", tc.life, z)
		}
	}
}
