package oracle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// commandZoneHybridWitnessRepro is a four-seat hosted-cmdr pod (kfuzz -kernel
// hosted-cmdr -seats 4 -policy heuristic, pod 30): the Deadly Disguise seat
// casts its commander Kaust, Eyes of the Glade ({R/W}{G}) from the command
// zone. gorge's cast planner declines a hybrid cost ("cost:hybrid"), so the
// play is a potential play the oracle lowers (CR 601.2g: the witness's mana
// abilities are activated at priority, then the spell is cast from the
// floating pool), and the hybrid pip is announced (CR 601.2b) with Gruul Turf
// already tapped for {R}{G}. The kernel lowered only a hand cast, so at the
// pip ask its Gruul Turf was still untapped and its pool empty.
const commandZoneHybridWitnessRepro = "command_zone_hybrid_cast_floats_witness_four_seat_pod"

// commandZoneHybridWitnessAt is the decision the recording diverged at.
const commandZoneHybridWitnessAt = 301

// hostedCmdrPodReplay replays a four-seat hosted-cmdr pod recording in lockstep
// on the hosted-cmdr tables over its own four decks, reading gorge's finished
// game through done.
func hostedCmdrPodReplay(t *testing.T, name string, done func(g *GorgeSide)) Result {
	t.Helper()
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Cards) != 2 || len(r.MoreCards) != 2 {
		t.Fatalf("recording has %d+%d decks, want a four-seat pod", len(r.Cards), len(r.MoreCards))
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("hosted-cmdr")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no hosted-cmdr table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	es := make([][]cardset.Entry, 0, 4)
	for _, names := range [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]} {
		es = append(es, ReproEntries(reg, set, names))
	}
	tbl, _, err := set.MatchupN(es)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	g, err := NewGorgeSide(m, r.Seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, r.Seed, g, k, &ScriptPolicy{Actions: r.Actions}, rc)
	done(g)
	return res
}

// TestHostedCmdrCommandZoneHybridWitnessLockstep: the pod plays past the
// commander's hybrid cast, and gorge did cast Kaust from the command zone (the
// precondition: the lowered command-zone cast the fix covers was made).
func TestHostedCmdrCommandZoneHybridWitnessLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cast := false
	res := hostedCmdrPodReplay(t, commandZoneHybridWitnessRepro, func(g *GorgeSide) {
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind != events.PutOnStack || e.From != state.ZCommand {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0].Name == "Kaust, Eyes of the Glade" {
				cast = true
			}
		}
	})
	if !cast {
		t.Fatalf("gorge never cast Kaust, Eyes of the Glade from the command zone in %d decisions", res.Decisions)
	}
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= commandZoneHybridWitnessAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, commandZoneHybridWitnessAt)
	}
}
