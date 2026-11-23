package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// hornChosenRepro is a four-seat edh pod (kfuzz -kernel edh -seats 4 -seed 5,
// pod 217): Herald's Horn (chosen Soldier) reveals Beregond of the Guard from
// the library top at its controller's upkeep and ConditionPresent$
// Creature.ChosenType over the remembered card puts it into the hand. gorge reads
// the Horn's choice; the kernel's remembered-card match saw only a resolution's own
// ChooseType answer, so the card stayed in the library (CR 701.20a, 608.2c).
const hornChosenRepro = "heralds_horn_chosen_type_reveal_four_seat_pod"

// hornChosenAt is the decision the recording diverged at.
const hornChosenAt = 2491

func TestFourSeatHornChosenTypeReveal(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatEdhReplay(t, hornChosenRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= hornChosenAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, hornChosenAt)
	}
}

// TestFourSeatHornChosenTypeRevealMutation proves the replay reaches the card:
// reading only a resolution's own answer diverges.
func TestFourSeatHornChosenTypeRevealMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugRefChosenResOnly = true
	defer func() { dirac.DebugRefChosenResOnly = false }()
	if res := fourSeatEdhReplay(t, hornChosenRepro); res.Div == nil {
		t.Fatal("the resolution-only read changed no game: the replay does not reach the card")
	}
}
