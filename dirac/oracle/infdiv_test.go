package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Infinite Diversity lockstep: "Exile target instant or sorcery card from your
// graveyard. Copy that card X times, where X is the number of creature types
// among non-Borg creatures you control. You may cast the copies without paying
// their mana costs." The Play's Amount$ X with AllowRepeats$ True and CopyCard$
// True offers the one remembered card X times; each pick is one copy cast
// (CR 707.12) with its own targets and modes, the original stays exiled, the
// ask is declinable at any count, and X = 0 casts nothing. The expected answer
// at every decision is gorge's. Creature types are counted over several
// creature cards, a shared type counting once.
var kinfdivMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Infinite Diversity", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Merfolk of the Pearl Trident", Count: 3}, {Name: "Giant Tortoise", Count: 3},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 5}, {Name: "Island", Count: 5},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Mountain", Count: 14}, {Name: "Forest", Count: 18},
	},
}, {
	{
		{Name: "Infinite Diversity", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 2},
		{Name: "Pyroclasm", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Coral Merfolk", Count: 3},
		{Name: "Giant Tortoise", Count: 3}, {Name: "Llanowar Elves", Count: 3}, {Name: "Mountain", Count: 7},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Infinite Diversity", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Merfolk of the Pearl Trident", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Mountain", Count: 8},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 7},
	},
}}

type kinfdivTally struct {
	games, cast, copies, resolved, multi, deep int
}

func kinfdivCheck(tal *kinfdivTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	var copyIDs map[state.ObjID]bool
	batch := 0
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen, copyIDs, batch = ref, 0, map[state.ObjID]bool{}, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			switch {
			case ev.Kind == events.PutOnStack:
				o := ref.E.G.Obj(ev.Obj)
				switch {
				case o != nil && o.IsCopy:
					copyIDs[ev.Obj] = true
					tal.copies++
					batch++
					if batch == 2 {
						tal.multi++
					}
					if batch >= 3 {
						tal.deep++
					}
				case o != nil && o.Face() != nil && o.Face().Name == "Infinite Diversity":
					tal.cast++
					batch = 0 // the spell's copies follow it
				default:
					batch = -1000
				}
			case ev.Kind == events.MoveZone && ev.From == state.ZStack && copyIDs[ev.Obj]:
				tal.resolved++
				delete(copyIDs, ev.Obj)
			}
		}
		return nil
	}
}

func TestInfiniteDiversityLockstep(t *testing.T) {
	t.Parallel()
	var tal kinfdivTally
	_, stalled := sonRunStalls(t, "infdiv", kinfdivMatchups, 200, true, nil, kinfdivCheck(&tal))
	t.Logf("%d games: %d casts, %d copies cast (%d resolved; %d batches of two or more, %d copies as a third or later of a batch); %d stalled loud", tal.games, tal.cast, tal.copies, tal.resolved, tal.multi, tal.deep, stalled)
	if tal.cast < 20 || tal.copies < 20 || tal.multi < 5 || tal.deep < 2 {
		t.Errorf("Infinite Diversity cast %d (want >= 20), copies cast %d (want >= 20), batches of two or more %d (want >= 5), third copies %d (want >= 2)", tal.cast, tal.copies, tal.multi, tal.deep)
	}
}

// TestInfiniteDiversityMutation: with the copies cast without the per-copy
// decision (every copy queued at the first pick), or each copy a cast of the
// original card (it leaves exile), some seed diverges.
func TestInfiniteDiversityMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, mc := range []struct {
		name string
		mut  func(*kop.Table)
		dbg  *bool
	}{
		{"no per-copy decision", nil, &dirac.DebugRepeatNoOptional},
		{"original cast", nil, &dirac.DebugCopyCastNoCopy},
	} {
		*mc.dbg = true
		n := sonRun(t, "infdiv", kinfdivMatchups, 100, false, mc.mut, nil)
		*mc.dbg = false
		if n == 0 {
			t.Errorf("mutation %q: no seed diverged", mc.name)
		}
	}
}
