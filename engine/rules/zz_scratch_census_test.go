package rules

import (
	"fmt"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// zz_scratch_census_test.go is a throwaway diagnostic for the Iron Fist
// lockstep divergence: what gorge's shared mana census (windowManaUnitsWith)
// prices for a printed and for a World Tree-granted "Add one mana of any
// color" tap. Delete before commit.
func TestZZScratchGorgeCensusAny(t *testing.T) {
	anyLand := "Name:AnyLand\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any color.\nOracle:x\n"
	worldTree := "Name:WorldTree\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Add {G}.\n" +
		"S:Mode$ Continuous | Affected$ Land.YouCtrl | AddAbility$ AnyMana | Description$ lands you control gain a tap\n" +
		"SVar:AnyMana:AB$ Mana | Cost$ T | Produced$ Any | Amount$ 1 | SpellDescription$ Add one mana of any color.\nOracle:x\n"
	swamp := "Name:PlainSwamp\nManaCost:no cost\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ B | SpellDescription$ Add {B}.\nOracle:x\n"

	t.Run("printed", func(t *testing.T) {
		e, _, fx := newFixtureDeck(t, 91, anyLand)
		_ = fx
		id := findIn(e, "AnyLand", t)
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
		e.priorityRound()
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("the land is not on the battlefield: %+v", o)
		}
		if e.G.Obj(id).Tapped {
			t.Fatal("the land is tapped")
		}
		dumpCensus(t, e, id)
	})
	t.Run("granted", func(t *testing.T) {
		e, cfg, id := newFixtureDeck(t, 92, swamp, worldTree)
		_ = cfg
		_ = id
		tree := findIn(e, "WorldTree", t)
		sw := findIn(e, "PlainSwamp", t)
		e.emit(events.Event{Kind: events.MoveZone, Obj: tree, From: state.ZHand, To: state.ZBattlefield})
		e.emit(events.Event{Kind: events.MoveZone, Obj: sw, From: state.ZHand, To: state.ZBattlefield})
		e.priorityRound()
		if o := e.G.Obj(sw); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("the swamp is not on the battlefield")
		}
		if e.G.Obj(sw).Tapped {
			t.Fatal("the swamp is tapped")
		}
		dumpCensus(t, e, sw)
		// the swamp's window abilities: does the grant reach them?
		mas := e.availableManaAbilitiesForWindow(0, sw, false)
		for _, ma := range mas {
			mf := e.manaStaticFor(0, sw, ma)
			dbgline(t, "window ability %s: free=%v tap=%v any=%v counts=%v amount=%d", ma.Line, mf.FreeCost, mf.Tap, mf.Any, mf.Counts, mf.Amount)
		}
		var _ *cards.Card
	})
}

func findIn(e *Engine, name string, t *testing.T) state.ObjID {
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, cand := range e.G.Zone(z, 0) {
			if e.G.Obj(cand).Face().Name == name {
				return cand
			}
		}
	}
	t.Fatalf("no %s in hand or library", name)
	return 0
}

func dbgline(t *testing.T, format string, args ...any) {
	f, err := os.OpenFile("/home/sadams/projects/gorge/.worktrees/cli-20261121T163523Z-ea41785b/.ds4/scratch/census.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fmt.Fprintf(f, format+"\n", args...)
}

func dumpCensus(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	units := e.windowManaUnitsWith(0, []state.ObjID{id}, nil)
	if len(units) == 0 {
		dbgline(t, "census[%s]: no unit for obj %d", t.Name(), id)
		return
	}
	for _, u := range units {
		for _, al := range u.Alts {
			t.Logf("census unit id=%d free=%d alt counts=%v amt=%d any=%v freeTap=%v", u.ID, u.FreeCount, al.Counts, al.Amt, al.Any, al.FreeTap)
		}
		if len(u.Alts) == 0 {
			t.Logf("census unit id=%d free=%d (no priceable alt)", u.ID, u.FreeCount)
		}
	}
}
