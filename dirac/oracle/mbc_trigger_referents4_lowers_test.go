package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMBCTriggerReferents4Lowers plays the MBC trigger-referents-4 spelling
// kgen lowers, TargetsWithDefinedController$ TriggeredDefendingPlayer on an
// Attacks trigger's target ("target creature defending player controls"), in
// lockstep with gorge over the mbctrigref4 fixture pool
// (dirac/cardset/pools/fixtures/mbctrigref4): Elite Scaleguard (tap) and
// Purifying Dragon (damage). gorge offers only permanents the trigger's
// DefendingPlayer controls (filterTargetsWithDefinedController); the kernel
// reads the same through ControlledBy TriggeredDefendingPlayer
// (kop.F2DefendingCtrl, lower_targetset.go tgtCtrlDefending), so with the
// attacker's own creatures on the battlefield the option lists agree only
// while the restriction is lowered.
//
// Precondition: both cards are cast in some game and each lowered target
// slot carries the defending-controller flag. The mutation subtest drops the
// flag from the table and must diverge. The other trigger-referents-4
// spellings (Share the Spoils' LosesGame and LandPlayed ValidSA$, Glava's
// SpellAbilityCast ValidCard$ "") stay refused: see the report.
func TestMBCTriggerReferents4Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	watch := map[string]bool{"Elite Scaleguard": true, "Purifying Dragon": true}
	decks := []fixtureDecks{{
		{{Name: "Elite Scaleguard", Count: 8}, {Name: "Purifying Dragon", Count: 8}, {Name: "Raging Goblin", Count: 6}, {Name: "Plains", Count: 9}, {Name: "Mountain", Count: 9}},
		{{Name: "Raging Goblin", Count: 16}, {Name: "Mountain", Count: 24}},
	}}
	t.Run("lowered", func(t *testing.T) {
		run := runFixtureLockstep(t, "mbctrigref4", decks, watch, 24, true, func(tbl *kop.Table) {
			for name := range watch {
				if !defendingTargetSlot(tbl, name) {
					t.Fatalf("%s: no target slot reads the defending player's control (TargetsWithDefinedController$ is not lowered)", name)
				}
			}
		})
		t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
		for name := range watch {
			if run.casts[name] < 3 {
				t.Errorf("%s cast %d times, want at least 3", name, run.casts[name])
			}
		}
	})
	t.Run("mutation", func(t *testing.T) {
		run := runFixtureLockstep(t, "mbctrigref4", decks, watch, 24, false, func(tbl *kop.Table) {
			fs := append([]kop.Filter(nil), tbl.Filters...)
			for i := range fs {
				fs[i].Flags2 &^= kop.F2DefendingCtrl
			}
			tbl.Filters = fs
		})
		run.requireDivergence(t, "a target slot offered past the defending player's permanents")
	})
}

// defendingTargetSlot reports whether the named card has an ability whose
// target slot's filter carries kop.F2DefendingCtrl.
func defendingTargetSlot(tbl *kop.Table, name string) bool {
	for _, cd := range tbl.Cards {
		if cd.Name != name {
			continue
		}
		for _, a := range cd.Abilities {
			for i := 0; i < int(a.NTargets); i++ {
				if tbl.Filter(a.Targets[i].Filter).Flags2&kop.F2DefendingCtrl != 0 {
					return true
				}
			}
		}
	}
	return false
}
