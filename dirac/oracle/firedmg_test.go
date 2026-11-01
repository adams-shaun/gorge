package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Fated Firepower lockstep: "If a source you control
// would deal damage to an opponent or a permanent an opponent controls, it
// deals that much damage plus an amount of damage equal to the number of fire
// counters on this enchantment instead." The plus operand is not a constant:
// its Count$CardCounters.FIRE SVar reads the host's fire counters at the moment
// the replacement applies (gorge rewriteReplaceCountOperand).
var kfiredmgMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Fated Firepower", Count: 2}, {Name: "Shock", Count: 8}, {Name: "Lightning Bolt", Count: 6},
		{Name: "Prodigal Pyromancer", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Mountain", Count: 14},
	},
	{
		{Name: "Prodigal Sorcerer", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8},
	},
}, {
	// Fated Firepower beside Twinflame Tyrant's doubling and Valley
	// Flamecaller's +1 (one copy of each per deck, so a combat event never
	// holds two doublers): two or more applicable replacements are gorge's CR
	// 616.1 order ask, which the kernel stalls loud (StallReplPlus)
	{
		{Name: "Fated Firepower", Count: 1}, {Name: "Twinflame Tyrant", Count: 1}, {Name: "Valley Flamecaller", Count: 1},
		{Name: "Shock", Count: 8}, {Name: "Lightning Bolt", Count: 6}, {Name: "Prodigal Pyromancer", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 15},
	},
	{
		{Name: "Fated Firepower", Count: 1}, {Name: "Twinflame Tyrant", Count: 1}, {Name: "Valley Flamecaller", Count: 1},
		{Name: "Shock", Count: 6}, {Name: "Lightning Bolt", Count: 6}, {Name: "Grizzly Bears", Count: 7},
		{Name: "Mountain", Count: 18},
	},
}}

// kfiredmgTally counts, on gorge's side, the damage events that landed above
// the source's printed amount while a Fated Firepower with fire counters was
// on the source controller's battlefield.
type kfiredmgTally struct {
	games, dmg, host, plus int
}

// kfiredmgPrinted is the pool's printed damage amount for a source card:
// the spell's damage or the creature's power (no pool card pumps).
func kfiredmgPrinted(name string) int {
	switch name {
	case "Shock":
		return 2
	case "Lightning Bolt", "Hill Giant", "Twinflame Tyrant", "Valley Flamecaller":
		return 3
	case "Prodigal Pyromancer", "Prodigal Sorcerer":
		return 1
	case "Grizzly Bears":
		return 2
	}
	return 0
}

// kfiredmgHost reports whether ctrl has a Fated Firepower with a fire counter
// on the battlefield.
func kfiredmgHost(g *state.Game, ctrl state.PlayerID) bool {
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Controller != ctrl || o.Face() == nil {
			continue
		}
		if o.Face().Name == "Fated Firepower" && o.Counter("FIRE") > 0 {
			return true
		}
	}
	return false
}

func kfiredmgCheck(tal *kfiredmgTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			if ev.Kind != events.DamageProvenance || ev.Amount <= 0 || len(ev.IDs) == 0 {
				continue
			}
			tal.dmg++
			src := ref.E.G.Obj(ev.Obj)
			if src == nil || src.Face() == nil || !kfiredmgHost(ref.E.G, src.Controller) {
				continue
			}
			tal.host++
			if int(ev.Amount) > kfiredmgPrinted(src.Face().Name) {
				tal.plus++
			}
		}
		return nil
	}
}

func TestFiredmgLockstep(t *testing.T) {
	t.Parallel()
	var tal kfiredmgTally
	_, stalled := sonRunStalls(t, "firedmg", kfiredmgMatchups, 800, true, nil, kfiredmgCheck(&tal))
	t.Logf("%d games: %d damage events (%d with a fire-countered Fated Firepower and %d above the printed amount); %d stalled loud on the damage-replacement order ask",
		tal.games, tal.dmg, tal.host, tal.plus, stalled)
	if tal.plus < 50 {
		t.Errorf("damage events above the printed amount with fire counters: %d, want >= 50", tal.plus)
	}
}

// TestFiredmgMutation: with the counter-host amount off, some seed diverges.
// The pool still stalls on a plus replacement beside another candidate;
// unmodelledKnown names that stall for this test too.
func TestFiredmgMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugDmgPlusCtrOff = true
	defer func() { dirac.DebugDmgPlusCtrOff = false }()
	n, _ := sonRunStalls(t, "firedmg", kfiredmgMatchups[:1], 128, false, nil, nil)
	if n == 0 {
		t.Errorf("mutation: no seed diverged in 128 games")
	}
	t.Logf("mutation diverged (the first divergence stops the run): %d", n)
}
