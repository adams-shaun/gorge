package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBestowedAuraSubtypesReprosLockstep replays the recorded dr-mbc-bbaa game
// where a bestowed Nyxborn Hydra has phased out with the creature it enchants:
// the Aura subtype (CR 205.3h) is part of the bestowed permanent's derived
// type line (CR 702.103b), and the projection of a phased-out permanent keeps
// the card's own rule-based type switches (CR 702.26b).
func TestBestowedAuraSubtypesReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "bestowed_aura_subtypes")
}

// bestowedAuraDeck is a mono-green pod deck carrying Nyxborn Hydra (the pool's
// only bestow card) beside creatures to enchant. Sage's Reverie itself is
// never needed: it names the Aura subtype, so the matchup's subtype table
// holds the bit the bestow switch adds.
var bestowedAuraDeck = func() []string {
	d := make([]string, 0, 40)
	for range 8 {
		d = append(d, "Nyxborn Hydra")
	}
	for range 12 {
		d = append(d, "Quirion Elves")
	}
	d = append(d, "Sage's Reverie", "Sage's Reverie")
	for len(d) < 40 {
		d = append(d, "Forest")
	}
	return d
}()

// TestBestowedAuraSubtypesFourSeat plays twenty four-seat pods whose decks all
// hold Nyxborn Hydra: a bestowed Hydra is projected as an Aura in both
// engines, from the turn it attaches through phasing and the host leaving, and
// the games reach an attached bestow permanent at least once (so the test
// cannot pass empty).
func TestBestowedAuraSubtypesFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	reached := 0
	check := func(g *GorgeSide, _ Side) *Divergence {
		if g == nil || g.E == nil || g.E.G == nil {
			return nil
		}
		for p := range g.E.G.Players {
			for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.E.G.Obj(id); o != nil && o.BestowedAttached() {
					reached++
					return nil
				}
			}
		}
		return nil
	}
	set := cardsettest.Synth(t, "dr-mbc-bbaa")
	play := fourSeatSetPlay(t, set, [4][]string{bestowedAuraDeck, bestowedAuraDeck, bestowedAuraDeck, bestowedAuraDeck},
		nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, nil) })
	for seed := uint64(1); seed <= 20; seed++ {
		if res := play(seed); res.Div != nil {
			d := res.Div
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
	}
	if reached == 0 {
		t.Fatal("no bestowed permanent was ever attached: the mechanism was never reached")
	}
	t.Logf("%d decision points with an attached bestowed permanent", reached)
}

// TestBestowedAuraSubtypesMutation restores the pre-switch subtype handling
// (no Aura subtype derived for the bestowed permanent): the recorded game then
// diverges at the projection, so the Aura subtype above is what the fix adds.
func TestBestowedAuraSubtypesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugBestowKeepsSubtypes
	dirac.DebugBestowKeepsSubtypes = true
	defer func() { dirac.DebugBestowKeepsSubtypes = old }()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "bestowed_aura_subtypes.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if res.Div == nil {
		t.Fatal("pre-switch subtype mutation replayed the recorded game with no divergence")
	}
	t.Logf("mutation diverged at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}
