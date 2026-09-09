package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// Synthetic witnesses deliberately add an intervening-if and a counter
// predicate: retaining just the dead sources' zone would not pass this pin.
const batchWitness = "Name:Batch witness\nManaCost:B\nTypes:Creature Vampire\nPT:1/1\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.counters_GE1_mark | IsPresent$ Creature | PresentCompare$ EQ2 | Execute$ Gain\n" +
	"SVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

func TestSBABatchUsesPreDepartureBoard(t *testing.T) {
	for _, shape := range []string{"lethal", "zero toughness", "regenerated", "exiled", "different controllers", "legend rule"} {
		t.Run(shape, func(t *testing.T) {
			e := layerEngine(t)
			src := batchWitness
			if shape == "legend rule" {
				src = strings.Replace(src, "Types:Creature", "Types:Legendary Creature", 1)
			}
			a := onBoard(t, e, 0, src)
			p := state.PlayerID(0)
			if shape == "different controllers" {
				p = 1
			}
			b := onBoard(t, e, p, src)
			for _, id := range []state.ObjID{a, b} {
				e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "mark", Amount: 1})
				e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 1})
			}
			switch shape {
			case "legend rule":
				e.emit(events.Event{Kind: events.CounterChange, Obj: b, Counter: "Shield", Amount: 1})
			case "zero toughness":
				e.emit(events.Event{Kind: events.CounterChange, Obj: a, Counter: "M1M1", Amount: 1})
			case "regenerated":
				e.emit(events.Event{Kind: events.CounterChange, Obj: a, Counter: "Shield", Amount: 1})
			case "exiled":
				onBoard(t, e, 0, "Name:Exile marked deaths\nTypes:Enchantment\n"+
					"R:Event$ Moved | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature | ReplaceWith$ Exile\n"+
					"SVar:Exile:DB$ ChangeZone | Defined$ ReplacedCard | Origin$ Battlefield | Destination$ Exile\nOracle:x\n")
			}
			e.checkStateBased()
			want := 4
			if shape == "regenerated" {
				want = 2 // both sources observe b; a never died
			}
			if shape == "exiled" {
				want = 0 // replacement moves are not deaths
			}
			if len(e.pendingTriggers) != want {
				t.Fatalf("queued %d triggers, want %d", len(e.pendingTriggers), want)
			}
			seen := map[[2]state.ObjID]bool{}
			for _, pt := range e.pendingTriggers {
				lki := pt.Ctx.LKI
				if lki == nil || lki.Zone != state.ZBattlefield || lki.Counter("mark") != 1 {
					t.Fatalf("missing pre-batch LKI: %+v", lki)
				}
				key := [2]state.ObjID{pt.Source, lki.ID}
				if seen[key] || (shape == "regenerated" && lki.ID != b) {
					t.Fatalf("duplicate or prevented death: %v", key)
				}
				seen[key] = true
			}
			if want > 0 {
				if !e.putTriggersOnStack() || e.Pending().Kind != decision.KTriggerOrder {
					t.Fatal("simultaneous triggers did not ask their controller for order")
				}
				d := e.Pending()
				n := want
				if shape == "different controllers" {
					n = 2
				}
				if d.Player != 0 || len(d.Options) != n {
					t.Fatalf("wrong APNAP ordering ask: %+v", d)
				}
			}
		})
	}
}

func TestSBABatchDoesNotLookBackForFromAnywhereTriggers(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Graveyard arrival\nTypes:Creature Bear\nPT:1/1\n"+
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Graveyard | TriggerZones$ Graveyard | ValidCard$ Card.Self | Execute$ Gain\n"+
		"SVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 1})
	e.checkStateBased()
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != id {
		t.Fatal("from-anywhere graveyard arrival must see the post-event source zone")
	}
}

func TestSBABatchParkedMovesRetainLookBackAcrossClone(t *testing.T) {
	src := strings.Replace(batchWitness, "Types:Creature", "Types:Legendary Creature", 1)
	e, _ := cmdZoneGame(t, [][]string{{src}, {src}})
	for p := state.PlayerID(0); p < 2; p++ {
		id := e.G.Zone(state.ZCommand, p)[0]
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZCommand, To: state.ZBattlefield})
		e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "mark", Amount: 1})
		e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 1})
	}
	e.checkStateBased()
	if len(e.cmdZone) != 2 {
		t.Fatalf("parked %d moves, want 2", len(e.cmdZone))
	}
	clone := e.Clone()
	for _, engine := range []*Engine{e, clone} {
		for len(engine.cmdZone) > 0 {
			d := engine.Pending()
			engine.pending = nil
			engine.handleCmdZone(d, decision.Intent{Player: d.Player, Choices: []int{1}})
		}
		if len(engine.pendingTriggers) != 4 {
			t.Fatalf("resumed deaths queued %d triggers, want 4", len(engine.pendingTriggers))
		}
	}
	if e.L.Head() != clone.L.Head() {
		t.Fatal("clone's resumed death log diverged")
	}
}

func TestSBABatchLookBackEndsBeforeNextPass(t *testing.T) {
	e := layerEngine(t)
	// No intervening-if here: the assertion is about which batch a source
	// belongs to, not whether a condition masks an incorrectly retained one.
	a := onBoard(t, e, 0, "Name:Witness lord\nTypes:Creature Vampire\nPT:1/1\n"+
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature | Execute$ Gain\n"+
		"SVar:Gain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n")
	b := onBoard(t, e, 0, "Name:Dependent\nTypes:Creature Bear\nPT:1/1\nOracle:x\n")
	e.AddContinuous(ContinuousEffect{Source: a, Timestamp: 1, Layer: LPT, Sub: SubModify,
		Affects: "Creature.Other+YouCtrl", Controller: 0, AddToughness: 1})
	for _, id := range []state.ObjID{a, b} {
		e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 1})
	}
	e.checkStateBased()
	if e.G.Obj(a).Zone != state.ZGraveyard || e.G.Obj(b).Zone != state.ZGraveyard {
		t.Fatal("fixed point did not remove both creatures")
	}
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Ctx.LKI.ID != a {
		t.Fatalf("dead lord observed a later batch: %+v", e.pendingTriggers)
	}
	if e.triggerBefore != nil {
		t.Fatal("look-back leaked beyond batch")
	}
}
