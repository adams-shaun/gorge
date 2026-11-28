package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// commanderLibraryChoiceRepro is a four-seat hosted-cmdr pod (kfuzz -kernel
// hosted-cmdr -seats 4 -policy heuristic, pod 371): Temporal Cleansing's owner
// picks second from the top or the bottom for their commander Kaust, then
// takes the CR 903.9b command-zone replacement. The kernel's answer to the
// second ask reran the instruction, which posed the position ask again and
// left Kaust on the battlefield with the spell still on the stack.
const commanderLibraryChoiceRepro = "commander_tucked_second_from_top_asks_once_four_seat_pod"

// commanderLibraryChoiceAt is the decision the recording diverged at.
const commanderLibraryChoiceAt = 1055

func TestHostedCmdrCommanderLibraryChoiceLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	redirected := false
	res := hostedCmdrPodReplay(t, commanderLibraryChoiceRepro, func(g *GorgeSide) {
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind != events.MoveZone || e.From != state.ZBattlefield || e.To != state.ZCommand {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Kaust, Eyes of the Glade" {
				redirected = true
			}
		}
	})
	if !redirected {
		t.Fatalf("gorge never moved Kaust, Eyes of the Glade from the battlefield to the command zone in %d decisions", res.Decisions)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= commanderLibraryChoiceAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, commanderLibraryChoiceAt)
	}
}
