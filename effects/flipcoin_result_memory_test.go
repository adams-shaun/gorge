package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestFlipCoinRememberResultGatesFlippedSets contrasts the explicit True
// carrier with absent and False forms. Results are only the
// RememberResult$-controlled input to Defined$ FlippedHeads/FlippedTails;
// the per-flip Wins/Losses publication remains live for every form.
func TestFlipCoinRememberResultGatesFlippedSets(t *testing.T) {
	cases := []struct {
		name      string
		line      string
		wantHeads bool
	}{
		{name: "absent", line: "DB$ FlipCoin"},
		{name: "false", line: "DB$ FlipCoin | RememberResult$ False"},
		{name: "true", line: "DB$ FlipCoin | RememberResult$ True", wantHeads: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, c := fixtureHost(t)
			Resolve(h, c, sa(t, tc.line))

			// The fake host's deterministic Rand returns heads. This confirms a
			// flip occurred and the independent per-flip publication did not get
			// accidentally gated together with result memory.
			wins, winsOK := runtimePublished(c, "Wins")
			losses, lossesOK := runtimePublished(c, "Losses")
			if !winsOK || !lossesOK || wins != 1 || losses != 0 {
				t.Fatalf("precondition: one heads flip must publish Wins/Losses=1/0; got %d/%d (present %v/%v)", wins, losses, winsOK, lossesOK)
			}

			heads, headsOK := definedSpec(h, c, "FlippedHeads")
			tails, tailsOK := definedSpec(h, c, "FlippedTails")
			if !headsOK || !tailsOK {
				t.Fatalf("FlippedHeads/FlippedTails did not resolve: %v/%v", headsOK, tailsOK)
			}
			if tc.wantHeads {
				want := []state.Target{{Player: c.Controller, IsPlayer: true}}
				if !targetsEqual(heads, want) || len(tails) != 0 {
					t.Fatalf("RememberResult$ True: heads/tails = %v/%v, want %v/empty", heads, tails, want)
				}
				return
			}
			if len(heads) != 0 || len(tails) != 0 {
				t.Fatalf("RememberResult$ %s leaked result memory: heads/tails = %v/%v, want empty/empty", tc.name, heads, tails)
			}
		})
	}
}
