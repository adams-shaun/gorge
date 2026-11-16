package dirac

import "testing"

// TestGrantSearchWitnessKeepsOnlyTarget: with every other source gone, Larger
// Than Life ({1}{G}) is paid only by the Eldrazi Spawn's granted tap for {G}
// and its own sacrifice for {C}, and the Spawn is the only creature it can
// target. gorge's prefix search replays that witness (scriptSearch.priced,
// witnessReachesFrom), finds the cast without a target after it (CR 601.2c),
// and withholds the cast; so does the grant search (grantSrch.witnessEats).
// The precondition: with a second creature on the battlefield the same
// payment leaves a target and the cast is offered.
func TestGrantSearchWitnessKeepsOnlyTarget(t *testing.T) {
	offered := func(board ...string) bool {
		sc, id, _ := pathSpawnScen(t, Config{NoElide: true, GorgeLowering: true}, board...)
		ltl := sc.hand(id["Larger Than Life"], 0)
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(ltl)) {
				return true
			}
		}
		return false
	}
	if !offered("Path of Annihilation", "c_0_1_eldrazi_spawn_sac", "Goblin Taskmaster") {
		t.Fatal("precondition: {1}{G} from the Spawn's tap and sacrifice, a Goblin Taskmaster to target, not offered")
	}
	if offered("Path of Annihilation", "c_0_1_eldrazi_spawn_sac") {
		t.Error("Larger Than Life offered when its payment sacrifices its only target, want it withheld as gorge does")
	}
}
