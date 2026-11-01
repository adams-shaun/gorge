package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runSBABoard plays one oracle scenario object and fails on any unmet expect.
func runSBABoard(t *testing.T, sc string) {
	t.Helper()
	res, err := RunOracleAuditScenarioJSON(testutil.CorpusRegistry(t), []byte(sc), true)
	if err != nil || len(res.Fails) != 0 {
		t.Fatalf("err=%v fails=%v\n%s", err, res.Fails, strings.Join(res.Transcript, "\n"))
	}
}

// TestCR704_8YoungWolfKeepsItsCounterInLastKnownInformation is the CR's own
// example with a bigger spell: a +1/+1 counter, then enough -1/-1 counters to
// kill it. The creature dies in the same check that would annihilate the counters,
// so it last existed with the +1/+1 counter and undying does not return it.
func TestCR704_8YoungWolfKeepsItsCounterInLastKnownInformation(t *testing.T) {
	t.Parallel()
	runSBABoard(t, `{"name":"young-wolf","cr":["704.8"],"why":"CR 704.8's example",
"setup":{"p0":{"hand":["Blight Rot"],"mana":"BBB","battlefield":["Young Wolf"],"counters":{"Young Wolf":{"P1P1":1}}},"p1":{}},
"steps":[{"op":"cast","seat":0,"card":"p0:Blight Rot","mana":"BBB","targets":["p0:Young Wolf"]},{"op":"resolve"},{"op":"resolve"}],
"expect":[{"card":"p0:Young Wolf","zone":"graveyard"}]}`)
}

// TestCR704_8CounterReaderSeesBothCountersOfACasualty: Meltstrider Eulogist draws
// for a creature that died with a +1/+1 counter, even though the same check
// would have annihilated that counter.
func TestCR704_8CounterReaderSeesBothCountersOfACasualty(t *testing.T) {
	t.Parallel()
	runSBABoard(t, `{"name":"eulogist-lki-counter","cr":["704.8"],"why":"the casualty's counters as it last existed",
"setup":{"p0":{"hand":["Blight Rot"],"mana":"BBB","battlefield":["Meltstrider Eulogist","Grizzly Bears"],"library_top":["Island"],"counters":{"Grizzly Bears":{"P1P1":1}}},"p1":{}},
"steps":[{"op":"cast","seat":0,"card":"p0:Blight Rot","mana":"BBB","targets":["p0:Grizzly Bears"]},{"op":"resolve"},{"op":"resolve"}],
"expect":[{"card":"p0:Grizzly Bears","zone":"graveyard"},{"hand_size":{"p0":1}}]}`)
}

// TestCR704_3WorldRuleCountsAWorldPermanentDyingInTheSameCheck: both conditions
// are read on one board. A younger world creature with lethal damage still sends
// the older world permanent to the graveyard.
func TestCR704_3WorldRuleCountsAWorldPermanentDyingInTheSameCheck(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	older := onBoard(t, e, 0, worldEnchantment("Older World"))
	younger := onBoard(t, e, 0, "Name:Younger World\nManaCost:2 B\nTypes:World Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: younger, Amount: 2})
	e.checkStateBased()
	if e.G.Obj(younger).Zone != state.ZGraveyard || e.G.Obj(older).Zone != state.ZGraveyard {
		t.Fatalf("zones: younger %v, older %v; want both in the graveyard", e.G.Obj(younger).Zone, e.G.Obj(older).Zone)
	}
}

// TestCR704_3DepartureFamiliesShareOneLookBackBoard: a creature with lethal damage
// and a planeswalker with no loyalty leave in one check, so the creature's "whenever
// a planeswalker you control dies" sees the planeswalker die (CR 603.10a, 704.8).
func TestCR704_3DepartureFamiliesShareOneLookBackBoard(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	observer := onBoard(t, e, 0, "Name:Observer\nTypes:Creature Bear\nPT:2/2\n"+
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Planeswalker.YouCtrl | TriggerZones$ Battlefield | Execute$ ObserverGain | TriggerDescription$ x\n"+
		"SVar:ObserverGain:DB$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n")
	walker := onBoard(t, e, 0, "Name:Walker\nTypes:Planeswalker Jace\nLoyalty:1\nOracle:x\n")
	e.emit(events.Event{Kind: events.CounterChange, Obj: walker, Counter: "LOYALTY", Amount: -1})
	e.emit(events.Event{Kind: events.Damage, Obj: observer, Amount: 2})
	e.checkStateBased()
	if e.G.Obj(observer).Zone != state.ZGraveyard || e.G.Obj(walker).Zone != state.ZGraveyard {
		t.Fatalf("zones: observer %v, walker %v", e.G.Obj(observer).Zone, e.G.Obj(walker).Zone)
	}
	if len(e.pendingTriggers) != 1 || e.pendingTriggers[0].Source != observer {
		t.Fatalf("queued %d triggers, want the observer's one", len(e.pendingTriggers))
	}
}

// TestCR704_3ALosersCreatureWithLethalDamageStillDies: a player's loss and their
// creature's lethal damage are found in one check, so the creature dies before its
// owner's departure removes it.
func TestCR704_3ALosersCreatureWithLethalDamageStillDies(t *testing.T) {
	t.Parallel()
	e := New(seatZeroStart(Config{Seed: 1, Names: []string{"a", "b", "c"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40), mountainDeck(t, 40)}}))
	onBoard(t, e, 0, "Name:Watcher\nTypes:Creature Bear\nPT:2/2\n"+
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature | TriggerZones$ Battlefield | Execute$ WatcherGain | TriggerDescription$ x\n"+
		"SVar:WatcherGain:DB$ GainLife | Defined$ You | LifeAmount$ 3\nOracle:x\n")
	bear := onBoard(t, e, 1, "Name:Doomed\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 2})
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -e.G.Players[1].Life})
	e.checkStateBased()
	if !e.G.Players[1].Lost {
		t.Fatal("the player at 0 life did not lose")
	}
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("queued %d triggers, want the watcher seeing the creature die", len(e.pendingTriggers))
	}
}

// TestCR704_3CreaturePlaneswalkerTakesBothActions: regeneration replaces the
// destruction, not the zero-loyalty move found in the same check.
func TestCR704_3CreaturePlaneswalkerTakesBothActions(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Both\nTypes:Legendary Planeswalker Creature Jace\nPT:2/2\nLoyalty:1\nOracle:x\n")
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "LOYALTY", Amount: -1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Shield", Amount: 1})
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 2})
	e.checkStateBased()
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("zone %v; zero loyalty still puts it into the graveyard", o.Zone)
	}
}

// TestCR704_3DayboundPermanentDyingInTheCheckStillMakesItDay: CR 702.145d reads
// the same board as the lethal damage.
func TestCR704_3DayboundPermanentDyingInTheCheckStillMakesItDay(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	id := onBoard(t, e, 0, "Name:Dawn Bear\nTypes:Creature Bear\nPT:2/2\nK:Daybound\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 2})
	e.checkStateBased()
	if e.G.Obj(id).Zone != state.ZGraveyard || e.G.DayNight != state.DayNightDay {
		t.Fatalf("zone %v, day/night %v; want the graveyard and day", e.G.Obj(id).Zone, e.G.DayNight)
	}
}

// TestDeferredLegendChoiceLeavesTheCheckStable: with a decision outstanding the
// legend ask can't be posed, so the legends and the casualties wait for the pass
// after the answer. The check found nothing it can do, so it is stable, not a
// cycle that spends the pass budget.
func TestDeferredLegendChoiceLeavesTheCheckStable(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	src := "Name:Twin\nTypes:Legendary Creature Bear\nPT:2/2\nOracle:x\n"
	onBoard(t, e, 0, src)
	onBoard(t, e, 0, src)
	bear := onBoard(t, e, 0, "Name:Doomed\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 2})
	e.pending = &decision.Decision{Kind: decision.KPriority, Player: 0}
	n0 := len(e.L.Events)
	e.checkStateBased()
	if e.G.Obj(bear).Zone != state.ZBattlefield {
		t.Fatalf("zone %v; the casualty waits with the deferred legend ask", e.G.Obj(bear).Zone)
	}
	if n := len(e.L.Events); n != n0 {
		t.Fatalf("the deferred check emitted %d events, want none: %+v", n-n0, e.L.Events[n0:])
	}
}
