package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// A no-engine host is the deterministic-decline path used by effect-unit
// tests and fuzzing. It must preserve UnlessSwitched's orientation: a decline
// runs an ordinary "unless" effect but not an "if they do" switched effect.
func TestUnlessNoHostDeclineHonoursOrientation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		sa       string
		wantLife int32
	}{
		{
			name:     "unswitched runs on decline",
			sa:       "SP$ GainLife | Defined$ You | LifeAmount$ 2 | UnlessCost$ 1",
			wantLife: 22,
		},
		{
			name:     "switched skips on decline",
			sa:       "SP$ GainLife | Defined$ You | LifeAmount$ 2 | UnlessCost$ 1 | UnlessSwitched$ True",
			wantLife: 20,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHost(t, 2)
			Resolve(h, &Ctx{Controller: 0}, sa(t, tc.sa))
			if got := h.g.Players[0].Life; got != tc.wantLife {
				t.Fatalf("life = %d, want %d", got, tc.wantLife)
			}
		})
	}
}

// TriggeredSourceSAController is source-relative, not target-relative. In
// particular it must not fall through to TargetedController when a trigger's
// source controller differs from the affected object's controller.
func TestUnlessPayerTriggeredSourceSAController(t *testing.T) {
	h := &askHost{fakeHost: fakeHost{g: state.NewGame(names(2))}}
	Resolve(h, &Ctx{
		Controller: 0,
		Targets:    []state.Target{{Player: 1, IsPlayer: true}},
	}, sa(t, "DB$ GainLife | Defined$ You | LifeAmount$ 2 | UnlessCost$ 1 | UnlessPayer$ TriggeredSourceSAController"))
	if h.asked == nil || h.asked.Player != 0 {
		t.Fatalf("unless payer = %+v, want source controller seat 0", h.asked)
	}
}
