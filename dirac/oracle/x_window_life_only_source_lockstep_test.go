package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestXWindowLifeOnlySourceReprosLockstep replays the recorded Lunar Frenzy
// cast whose X was paid in the manual window beside a life-only mana source
// (Treasonous Ogre): gorge activates it, so the kernel must too.
func TestXWindowLifeOnlySourceReprosLockstep(t *testing.T) {
	stdReproNamed(t, "x_window_life_only_source")
}

// TestXWindowLifeOnlySourceMutation restores the old witness census, which
// skipped a life-only mana source behind availUngranted's tap gate: the same
// recording must diverge again, so the repro test above cannot pass without
// the fix.
func TestXWindowLifeOnlySourceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "x_window_life_only_source.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugXWindowSkipLifeOnly
	dirac.DebugXWindowSkipLifeOnly = true
	defer func() { dirac.DebugXWindowSkipLifeOnly = old }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatalf("the old skip replayed %d decisions with no divergence", res.Decisions)
	}
	t.Logf("mutation diverged at decision %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}

// xWindowLifeOnlyPodDeck holds Lunar Frenzy and Treasonous Ogre beside the
// dr-mbc-bbbb pool's own lands. That pool has no painland and no
// Phyrexian-mana spell, so the life-only source is the life-cost enabler the
// pod's X casts are paid with.
func xWindowLifeOnlyPodDeck() []string {
	d := make([]string, 0, 40)
	for range 8 {
		d = append(d, "Lunar Frenzy")
	}
	for range 8 {
		d = append(d, "Treasonous Ogre")
	}
	for range 12 {
		d = append(d, "Mountain")
	}
	for range 12 {
		d = append(d, "Swamp")
	}
	return d
}

// xWindowLifeOnlyXAsk reports whether d is gorge's announced-X ask (CR
// 601.2b), the decision the payment window and its life-only source are read
// against.
func xWindowLifeOnlyXAsk(d *decision.Decision) bool {
	return d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "x"
}

// TestXWindowLifeOnlySourceFourSeat plays 20 four-seat pods on dr-mbc-bbbb
// and requires no divergence. A check hook counts the X ask posed while an
// untapped Treasonous Ogre is on the caster's battlefield, so the
// no-divergence check cannot pass without the mechanism being reached.
func TestXWindowLifeOnlySourceFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	set := cardsettest.Synth(t, "dr-mbc-bbbb")
	deck := xWindowLifeOnlyPodDeck()
	reached := 0
	check := func(g *GorgeSide, _ Side) *Divergence {
		d := g.Pending()
		if !xWindowLifeOnlyXAsk(d) {
			return nil
		}
		for _, id := range g.E.G.Zone(state.ZBattlefield, d.Player) {
			o := g.E.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == "Treasonous Ogre" && !o.Tapped {
				reached++
				break
			}
		}
		return nil
	}
	play := fourSeatSetPlay(t, set, [4][]string{deck, deck, deck, deck}, nil, check, nil,
		func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 20; seed++ {
		res := play(seed)
		if res.Div != nil {
			d := res.Div
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	if reached == 0 {
		t.Fatal("no X ask was posed with an untapped Treasonous Ogre in 20 pods: the mechanism was never reached")
	}
	t.Logf("%d X asks with an untapped Treasonous Ogre across 20 pods", reached)
}
