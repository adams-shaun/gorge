package oracle

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPhasedOutRegenShieldsProjectionReprosLockstep replays the recorded game
// where Guardian of Faith's enter trigger phased out an Ancient Silverback
// holding three regeneration shields. Phasing is not a zone change (CR
// 702.26d) and the shield is a this-turn effect (CR 701.19c), so gorge keeps
// listing the "Shield" marker on the phased-out permanent: the kernel's
// feature must read the shields off the phased-out permanent's slot too.
func TestPhasedOutRegenShieldsProjectionReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// the preconditions: the phasing source and the shield holder are in the
	// game, so the compared counter differs without the fix
	r, err := ReadRepro(filepath.Join("testdata", "repro", "phased_out_regen_shields_projection.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Guardian of Faith", "Ancient Silverback"} {
		if !slices.Contains(r.Cards[0], want) && !slices.Contains(r.Cards[1], want) {
			t.Fatalf("the recording's decks hold no %s", want)
		}
	}
	stdReproNamed(t, "phased_out_regen_shields_projection")
}

// phasedShieldsDeck holds the shield holder and the phasing source: an
// Ancient Silverback pays G for regeneration shields (CR 701.19a) and a
// Guardian of Faith's enter trigger phases other creatures you control out
// (CR 702.26), on the green and white mana the pod's decks all hold.
var phasedShieldsDeck = []string{
	"Ancient Silverback", "Ancient Silverback", "Ancient Silverback", "Ancient Silverback",
	"Guardian of Faith", "Guardian of Faith", "Guardian of Faith", "Guardian of Faith",
	"Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest",
	"Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains", "Plains",
}

// TestPhasedOutRegenShieldsProjectionFourSeat plays four-seat pods whose decks
// all hold Ancient Silverback, Guardian of Faith and mana: a permanent phased
// out while holding regeneration shields keeps them in both engines'
// projections, and the games reach that.
func TestPhasedOutRegenShieldsProjectionFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	var reached, phaseOuts int
	check := func(g *GorgeSide, _ Side) *Divergence {
		for p := state.PlayerID(0); p < 4; p++ {
			for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
				o := g.E.G.Obj(id)
				if o == nil || !o.PhasedOut {
					continue
				}
				phaseOuts++
				if o.Counter("Shield") > 0 {
					reached++
				}
			}
		}
		return nil
	}
	play := fourSeatSetPlay(t, cardsettest.Synth(t, "dr-mbc-bbaa"),
		[4][]string{phasedShieldsDeck, phasedShieldsDeck, phasedShieldsDeck, phasedShieldsDeck},
		nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 20; seed++ {
		if res := play(seed); res.Div != nil {
			d := res.Div
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
	}
	if reached == 0 {
		t.Fatalf("no permanent phased out while holding regeneration shields: the mechanism was never reached (%d phased-out decisions without shields)", phaseOuts)
	}
	t.Logf("%d phased-out decisions, %d with regeneration shields", phaseOuts, reached)
}

// TestPhasedOutRegenShieldsProjectionMutation restores the phased-zone read
// zero: the recorded game then diverges again, so the reads above are the ones
// that matter.
func TestPhasedOutRegenShieldsProjectionMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugPhasedShieldsZero
	dirac.DebugPhasedShieldsZero = true
	defer func() { dirac.DebugPhasedShieldsZero = old }()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "phased_out_regen_shields_projection.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if res.Div == nil {
		t.Fatal("phased-zero mutation replayed the recorded game with no divergence")
	}
	t.Logf("mutation diverged at %d: %s", res.Div.At, res.Div.What)
}
