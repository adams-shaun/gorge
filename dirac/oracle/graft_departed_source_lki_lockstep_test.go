package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGraftDepartedSourceLKIReprosLockstep replays the recorded game where a
// Graft creature leaves while its trigger waits: the intervening-if "Self has
// a +1/+1 counter" reads the source's last known counters (CR 603.4, 608.2h),
// so the trigger still resolves in both engines.
func TestGraftDepartedSourceLKIReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "graft_departed_source_lki")
}

// graftLKIDeck holds the carrier and both ways to make it leave while its
// Graft trigger waits (a bounce, a sacrifice).
var graftLKIDeck = []string{
	"Cytoplast Root-Kin", "Cytoplast Root-Kin", "Flock Impostor", "Flock Impostor",
	"Blasting Station", "Blasting Station", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest",
	"Island", "Island", "Island", "Island", "Island", "Island",
}

// TestGraftDepartedSourceLKIFourSeat plays four-seat pods whose decks all hold
// Cytoplast Root-Kin, Flock Impostor and Blasting Station: a Graft trigger
// still on the stack after its source left must resolve the same in both
// engines, and the games reach that.
func TestGraftDepartedSourceLKIFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	var reached int
	check := func(g *GorgeSide, _ Side) *Divergence {
		for _, id := range g.E.G.Stack {
			o := g.E.G.Obj(id)
			if o == nil || o.Ability == nil || o.Source == 0 {
				continue
			}
			src := g.E.G.Obj(o.Source)
			if src != nil && src.Zone != state.ZBattlefield && src.Face() != nil && src.Face().Name == "Cytoplast Root-Kin" {
				reached++
			}
		}
		return nil
	}
	set := cardsettest.Synth(t, "dr-mbc-bbaa")
	play := fourSeatSetPlay(t, set, [4][]string{graftLKIDeck, graftLKIDeck, graftLKIDeck, graftLKIDeck},
		nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 20; seed++ {
		if res := play(seed); res.Div != nil {
			d := res.Div
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
	}
	if reached == 0 {
		t.Fatal("no Graft trigger was on the stack after its source left: the mechanism was never reached")
	}
	t.Logf("%d decision points with a departed-source Graft trigger on the stack", reached)
}

// TestGraftDepartedSourceLKIMutation restores the live-battlefield count: the
// recorded game then diverges, so the reads above are the ones that matter.
func TestGraftDepartedSourceLKIMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugGraftLiveCount
	dirac.DebugGraftLiveCount = true
	defer func() { dirac.DebugGraftLiveCount = old }()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "graft_departed_source_lki.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if res.Div == nil {
		t.Fatal("live-count mutation replayed the recorded game with no divergence")
	}
	t.Logf("mutation diverged at %d: %s", res.Div.At, res.Div.What)
}
