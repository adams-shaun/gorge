package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// The initiative lockstep (CR 726 / 309 / 701.49): Avenging
// Hunter, Bloodboil Sorcerer, Stirring Bard and Dungeoneer's Pack against
// Goliath Paladin, Underdark Explorer and Aarakocra Sneak, so both players
// take the initiative, venture through the Undercity at their upkeep (the
// two-arrow rooms ask), complete it and begin again, and steal it back with
// combat damage. The expected answer at every decision is gorge's.
var kinitDecks = [2][]cardset.Entry{
	{
		{Name: "Avenging Hunter", Count: 4}, {Name: "Bloodboil Sorcerer", Count: 3}, {Name: "Stirring Bard", Count: 3},
		{Name: "Dungeoneer's Pack", Count: 2}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Forest", Count: 11}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Goliath Paladin", Count: 3}, {Name: "Underdark Explorer", Count: 3}, {Name: "Aarakocra Sneak", Count: 3},
		{Name: "Savannah Lions", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Elite Vanguard", Count: 3},
		{Name: "Shock", Count: 2}, {Name: "Plains", Count: 9}, {Name: "Swamp", Count: 6}, {Name: "Island", Count: 3},
	},
}

// throneSVar is the Undercity's Throne of the Dead Three room (tokenscripts
// undercity.txt).
const throneSVar = "DB$ Dig | DigNum$ 10 | Reveal$ True | ChangeValid$ Creature | DestinationZone$ Battlefield | WithCounters$ P1P1 | WithCountersAmount$ 3 | RememberChanged$ True | LibraryPosition2$ 0 | SkipReorder$ True"

// gorgeDigWithCounters probes the gorge pin: false while its Dig leaves
// WithCounters$ unread (gorge effects/dig_params.go; cr-dig-withcounters),
// so Throne's creature enters without its three +1/+1 counters.
func gorgeDigWithCounters() bool {
	sa := cards.ResolveSVar(map[string]string{"Throne": throneSVar}, "Throne")
	if sa == nil {
		return false
	}
	for _, u := range effects.DigOf(sa).Unread {
		if u == "WithCounters" || u == "WithCountersAmount" {
			return false
		}
	}
	return true
}

// throneAsGorge rewrites the table's Throne room to gorge's play at a pin
// without Dig WithCounters$: no counters, and the hexproof ends at end of
// turn (gorge registerPumpEffects reads Duration$ UntilYourNextTurn as
// UntilEOT; cr-pump-untilyournextturn). The kernel's generated table plays
// the card text; only this test's copy is rewritten.
func throneAsGorge(tbl *kop.Table) {
	for i := range tbl.Cards {
		c := &tbl.Cards[i]
		if len(c.DungeonNext) == 0 {
			continue
		}
		for r, nx := range c.DungeonNext {
			if nx != 0 {
				continue
			}
			prog := append([]kop.Instr(nil), c.Abilities[r].Prog...)
			for j := range prog {
				switch prog[j].Op {
				case kop.OpDigCounters:
					prog[j].A = kop.Const(0)
				case kop.OpOverride:
					prog[j].A = kop.Const(kop.DurEOT)
				}
			}
			abs := append([]kop.Ability(nil), c.Abilities...)
			abs[r].Prog = prog
			c.Abilities = abs
		}
	}
}

// TestInitLockstep: every decision of every seed matches gorge. At a gorge
// pin without Dig WithCounters$ the Throne room plays gorge's way (the only
// difference throneAsGorge makes), so every other divergence still fails;
// TestInitLockstepThrone counts the card-text Throne's divergences.
func TestInitLockstep(t *testing.T) {
	t.Parallel()
	var mutate func(*kop.Table)
	if !gorgeDigWithCounters() {
		mutate = throneAsGorge
	}
	seeds := uint64(600)
	if v := os.Getenv("KINIT_SEEDS"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			seeds = n
		}
	}
	lockstepRun(t, "init", kinitDecks, seeds, true, mutate)
}

// TestInitLockstepThrone plays the card-text Throne. At a pin with Dig
// WithCounters$ it is strict. Without, the pool must reach a Throne
// resolution, which then diverges (TestInitLockstep proves every other
// decision clean; measured 2026-10-26: 162 of 200 seeds reach a Throne).
func TestInitLockstepThrone(t *testing.T) {
	t.Parallel()
	if !gorgeDigWithCounters() {
		if !lockstepDiverges(t, "init", kinitDecks, 40) {
			t.Fatal("no seed reached the Throne of the Dead Three in 40")
		}
		t.Log("gorge pin without Dig WithCounters$ (cr-dig-withcounters): the card-text Throne diverges, as expected")
		return
	}
	lockstepSet(t, "init", kinitDecks, 600, "KINIT_SEEDS")
}

// TestInitLockstepMutation proves the pool steals the initiative with
// combat damage: with the steal dropped, some seed diverges.
func TestInitLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoInitSteal = true
	defer func() { dirac.DebugNoInitSteal = false }()
	if !lockstepDiverges(t, "init", kinitDecks, 60) {
		t.Fatal("no seed diverged with the combat-damage steal dropped: the pool never steals the initiative")
	}
}
