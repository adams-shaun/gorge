package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// Dash from the command zone (CR 702.109a, 118.9d, 903.8): a commander with
// printed dash may be cast for its dash cost from the command zone, paying the
// commander tax on top (an alternative cost replaces the mana cost, not
// additional costs). gorge offers the "dashed" cast beside the plain one
// (legal_walk_alt.go, the altCastModes loop); the kernel's command-zone offer
// (cast_commander.go) offers it through offerDashCommand (kw_dash.go).
//
// The decks are the cmdcastcount fixture's duel (Isamaru against Zurgo
// Bellstriker, K:Dash:1 R, as seat 1's commander); the policy takes a dashed
// command-zone cast whenever one is offered, so Zurgo is dashed, returns to
// hand at the end step, and is cast again from the command zone only after it
// dies: the tax rises with each command-zone cast.

// dashCmdSeeds are the seeds the lockstep plays.
var dashCmdSeeds = []uint64{1, 2, 3, 4, 5, 6, 7, 8}

// dashCmdPolicy takes the dashed command-zone cast whenever it is offered
// (three times in four), else casts like castFirstPolicy; picks counts them.
func dashCmdPolicy(seed uint64, picks *int) Policy {
	rest := castFirstPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x646173686d64)) // "dashmd"
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil && c.D.Kind == DPriority {
			for _, a := range c.D.Options {
				if a.Kind() != ActCast || a.Mode()&15 != mustCastModeCode("dashed") {
					continue
				}
				if o := c.G.E.G.Obj(c.G.id.obj[a.Src()]); o != nil && o.Zone == state.ZCommand && prng.IntN(4) != 0 {
					*picks++
					return a
				}
			}
		}
		return rest.Pick(c)
	})
}

func mustCastModeCode(mode string) uint8 {
	code, ok := canon.CastModeCode(mode)
	if !ok {
		panic("no cast mode code for " + mode)
	}
	return code
}

// dashCmdRun is the sums over a seed set of the Zurgo duel.
type dashCmdRun struct {
	div     *Divergence // the first divergence, nil when none
	divSeed uint64
	picks   int // dashed command-zone casts the policy took (gorge's side)
	kernel  int // dashed command-zone casts the kernel applied
}

func playDashCmd(t *testing.T, stopAtDiv bool) dashCmdRun {
	t.Helper()
	d := cmdCastPodEntries()
	p := newCmdCastPod(t, [][]cardset.Entry{d[0], d[1]}, false)
	var run dashCmdRun
	kernel := 0
	dirac.DebugDashCmdCasts = &kernel
	defer func() { dirac.DebugDashCmdCasts = nil }()
	for _, seed := range dashCmdSeeds {
		r := p.repro(seed)
		m, err := r.Matchup(p.reg)
		if err != nil {
			t.Fatal(err)
		}
		tbl, _, err := p.set.MatchupN(p.cardLists())
		if err != nil {
			t.Fatal(err)
		}
		rc := r.RunConfig()
		rc.MaxDecisions, rc.MaxTurns = 8000, 40
		k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		lead, err := NewGorgeSide(m, seed, rc.Config, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, lead, k, dashCmdPolicy(seed, &run.picks), rc)
		CloseSide(k)
		if res.Div != nil && run.div == nil {
			run.div, run.divSeed = res.Div, seed
			if stopAtDiv {
				break
			}
			continue
		}
		if res.Div == nil {
			(cmdCastResult{res: res}).check(t, seed)
		}
	}
	run.kernel = kernel
	return run
}

// TestDashCommandZoneLockstep: no divergence over the seeds, and the policy
// dashed the commander from the command zone on gorge's side and on the
// kernel's (the kernel's counter is the cast actions it applied).
func TestDashCommandZoneLockstep(t *testing.T) {
	run := playDashCmd(t, false)
	if run.div != nil {
		t.Fatalf("seed %d: diverged at %d: %s %s", run.divSeed, run.div.At, run.div.What, run.div.Field)
	}
	t.Logf("%d dashed command-zone casts picked, %d applied by the kernel", run.picks, run.kernel)
	if run.picks == 0 || run.kernel == 0 {
		t.Fatalf("picked %d, kernel applied %d dashed command-zone casts: want both above zero", run.picks, run.kernel)
	}
}

// TestDashCommandZoneMutation: with the kernel's offer skipped, gorge's
// dashed command-zone cast has no counterpart and the lockstep diverges.
func TestDashCommandZoneMutation(t *testing.T) {
	dirac.DebugDashCmdNoOffer = true
	defer func() { dirac.DebugDashCmdNoOffer = false }()
	run := playDashCmd(t, true)
	if run.div == nil {
		t.Fatal("skipping the kernel's dashed command-zone offer diverged on no seed")
	}
	t.Logf("seed %d: mutation diverged at %d: %s %s", run.divSeed, run.div.At, run.div.What, run.div.Field)
}
