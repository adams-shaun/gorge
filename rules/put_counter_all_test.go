package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The PutCounterAll primitive (the "put a counter on each <thing>" sweep,
// 298 raw corpus lines over 291 files) did not exist before this file's
// fix: effects/counters.go registered only PutCounter/RemoveCounterAll/
// Regenerate, so every such line fell into the unimplemented-API fallback
// Note and placed nothing. These tests pin the implemented shapes on real
// corpus cards and keep the exotic shapes loud.

// putCounterTable seats the prepared seat-0/seat-1 decks (protagonist and
// extras already appended by the caller) with seat 0 as the starting player
// and drives to seat 0's first priority ask. Card sources must be distinct
// by name for the finders below to address them.
func putCounterTable(t *testing.T, seed uint64, seat0, seat1 []*cards.Card) (*Engine, Config) {
	t.Helper()
	cfg := seatZeroStart(Config{Seed: seed,
		Names:  []string{"a", "b"},
		Decks:  [][]*cards.Card{append(mountainDeck(t, 40), seat0...), append(mountainDeck(t, 40), seat1...)},
		Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	return e, cfg
}

// findAndMoveToBattlefield moves the named card from its holder's hand or
// library onto the battlefield, through the event the replay folds, and
// returns its object id.
func findAndMoveToBattlefield(t *testing.T, e *Engine, p state.PlayerID, name string) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, p) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZBattlefield})
				return id
			}
		}
	}
	t.Fatalf("%s not in seat %d's hand or library", name, p)
	return 0
}

// counterBear returns the card source for a distinctly named 2/2 Bear.
func counterBear(name string) string {
	return "Name:" + name + "\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
}

// counterChanges returns the log's CounterChange events for obj.
func counterChanges(e *Engine, obj state.ObjID) []events.Event {
	var out []events.Event
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == obj {
			out = append(out, ev)
		}
	}
	return out
}

// TestAronBenaliasRuinPutsCountersOnEachCreatureYouControl drives Aron's
// real AB$ PutCounterAll activation end to end: {W}{B}, {T} and the
// sacrifice of another creature pay, and EVERY creature the controller still
// controls gets exactly one +1/+1 counter (Aron included -- "each creature
// you control"); the sacrificed creature left the battlefield before
// resolution and takes nothing, and seat 1's creatures take nothing. Before
// the fix the resolution emitted "unimplemented API PutCounterAll" and moved
// nothing.
func TestAronBenaliasRuinPutsCountersOnEachCreatureYouControl(t *testing.T) {
	aronSrc := corpusCard(t, "Aron, Benalia's Ruin")
	e, cfg := putCounterTable(t, 196,
		[]*cards.Card{aronSrc, card(t, counterBear("Victim Bear")), card(t, counterBear("Kept Bear"))},
		[]*cards.Card{card(t, counterBear("Enemy Bear"))})
	aron := findAndMoveToBattlefield(t, e, 0, "Aron, Benalia's Ruin")
	victim := findAndMoveToBattlefield(t, e, 0, "Victim Bear")
	kept := findAndMoveToBattlefield(t, e, 0, "Kept Bear")
	enemy := findAndMoveToBattlefield(t, e, 1, "Enemy Bear")
	// The {T} cost needs a non-summoning-sick Aron (CR 302.6); the TurnChange
	// is the event the replay folds to clear it.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "B", Amount: 1})
	e.priorityRound()

	opt := abilityOption(t, e, aron, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "sacrifice" {
		t.Fatalf("no sacrifice cost ask after activation: %+v", d)
	}
	sacIdx := -1
	for _, o := range d.Options {
		if o.Obj == victim {
			sacIdx = o.Index
		}
	}
	if sacIdx < 0 {
		t.Fatalf("sacrifice ask does not offer Victim Bear: %+v", d.Options)
	}
	submitChoices(t, e, sacIdx)
	passUntilStackEmpty(t, e, 30)

	if z := e.G.Obj(victim).Zone; z != state.ZGraveyard {
		t.Fatalf("sacrificed Victim Bear zone = %s, want graveyard", z)
	}
	for _, id := range []state.ObjID{aron, kept} {
		if got := e.G.Obj(id).Counter("P1P1"); got != 1 {
			t.Fatalf("creature %d has %d P1P1 counters, want 1", id, got)
		}
	}
	if got := e.G.Obj(enemy).Counter("P1P1"); got != 0 {
		t.Fatalf("seat 1's Enemy Bear took %d counters, want 0", got)
	}
	if n := len(counterChanges(e, victim)); n != 0 {
		t.Fatalf("the sacrificed creature received %d CounterChange events, want 0", n)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("pool not drained: %d", e.G.Players[0].Pool.Total())
	}
	replayCheck(t, e, cfg)
}

// TestPutCounterAllSweepFollowsBattlefieldZoneOrder: the sweep walks seats
// in g.AliveFrom(0) order and each seat's battlefield in zone order (the
// effRemoveCounterAll shape), so the per-object CounterChange sequence must
// be exactly the zone order -- never a map range.
func TestPutCounterAllSweepFollowsBattlefieldZoneOrder(t *testing.T) {
	aronSrc := corpusCard(t, "Aron, Benalia's Ruin")
	e, cfg := putCounterTable(t, 197,
		[]*cards.Card{aronSrc, card(t, counterBear("First Bear")), card(t, counterBear("Second Bear")), card(t, counterBear("Third Bear"))},
		[]*cards.Card{card(t, counterBear("Enemy Bear"))})
	aron := findAndMoveToBattlefield(t, e, 0, "Aron, Benalia's Ruin")
	first := findAndMoveToBattlefield(t, e, 0, "First Bear")
	second := findAndMoveToBattlefield(t, e, 0, "Second Bear")
	third := findAndMoveToBattlefield(t, e, 0, "Third Bear")
	enemy := findAndMoveToBattlefield(t, e, 1, "Enemy Bear")
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "B", Amount: 1})
	e.priorityRound()
	opt := abilityOption(t, e, aron, 0)
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "sacrifice" {
		t.Fatalf("no sacrifice cost ask after activation: %+v", d)
	}
	// Sacrifice the LAST bear in zone order: the survivors' CounterChange
	// sequence is then the surviving zone order, not the full zone order.
	sacIdx := -1
	for _, o := range d.Options {
		if o.Obj == third {
			sacIdx = o.Index
		}
	}
	if sacIdx < 0 {
		t.Fatalf("sacrifice ask does not offer Third Bear: %+v", d.Options)
	}
	submitChoices(t, e, sacIdx)
	passUntilStackEmpty(t, e, 30)
	if z := e.G.Obj(third).Zone; z != state.ZGraveyard {
		t.Fatalf("sacrificed Third Bear zone = %s, want graveyard", z)
	}

	want := []state.ObjID{aron, first, second}
	var got []state.ObjID
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Counter == "P1P1" && ev.Amount == 1 {
			got = append(got, ev.Obj)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("sweep emitted %d CounterChange events, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sweep order: got %v, want zone order %v", got, want)
		}
	}
	if got := e.G.Obj(enemy).Counter("P1P1"); got != 0 {
		t.Fatalf("seat 1's Enemy Bear took %d counters, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// TestPutCounterAllPlayerTargetedSweep pins the ValidTgts$ Player shape on
// its real corpus carrier Meadowboon ("When CARDNAME leaves the battlefield,
// put a +1/+1 counter on each creature target player controls"): the chosen
// target player's creatures get the batch and nobody else's do.
func TestPutCounterAllPlayerTargetedSweep(t *testing.T) {
	meadow := corpusCard(t, "Meadowboon")
	e, cfg := putCounterTable(t, 198,
		[]*cards.Card{meadow, card(t, counterBear("Meadow Bear")), card(t, counterBear("Meadow Second"))},
		[]*cards.Card{card(t, counterBear("Enemy Bear"))})
	meadowID := findAndMoveToBattlefield(t, e, 0, "Meadowboon")
	first := findAndMoveToBattlefield(t, e, 0, "Meadow Bear")
	second := findAndMoveToBattlefield(t, e, 0, "Meadow Second")
	enemy := findAndMoveToBattlefield(t, e, 1, "Enemy Bear")
	e.emit(events.Event{Kind: events.MoveZone, Obj: meadowID, From: state.ZBattlefield, To: state.ZGraveyard})
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("no target ask for Meadowboon's leave trigger: %+v", d)
	}
	submitChoices(t, e, indexOfPlayerOption(d, 0))
	passUntilStackEmpty(t, e, 30)

	for _, id := range []state.ObjID{first, second} {
		if got := e.G.Obj(id).Counter("P1P1"); got != 1 {
			t.Fatalf("target player's creature %d has %d P1P1 counters, want 1", id, got)
		}
	}
	if got := e.G.Obj(enemy).Counter("P1P1"); got != 0 {
		t.Fatalf("seat 1's Enemy Bear took %d counters, want 0", got)
	}
	if n := len(counterChanges(e, meadowID)); n != 0 {
		t.Fatalf("the departed Meadowboon received %d CounterChange events, want 0", n)
	}
	replayCheck(t, e, cfg)
}

// TestPutCounterAllSecondBatchPlacesBothKinds pins the ValidCards2$/
// CounterType2$ shape on its real corpus carrier Brokers Ascendancy ("put a
// +1/+1 counter on each creature you control and a loyalty counter on each
// planeswalker you control"): one resolution, two sweeps, two counter kinds.
func TestPutCounterAllSecondBatchPlacesBothKinds(t *testing.T) {
	brokers := corpusCard(t, "Brokers Ascendancy")
	gideon := corpusCard(t, "Gideon, Ally of Zendikar")
	e, cfg := putCounterTable(t, 199,
		[]*cards.Card{brokers, gideon, card(t, counterBear("Broker Bear"))}, nil)
	findAndMoveToBattlefield(t, e, 0, "Brokers Ascendancy")
	gideonID := findAndMoveToBattlefield(t, e, 0, "Gideon, Ally of Zendikar")
	bear := findAndMoveToBattlefield(t, e, 0, "Broker Bear")
	driveToStepAll(t, e, e.G.Turn, e.G.Active, state.StepEnd)
	passUntilStackEmpty(t, e, 30)

	if got := e.G.Obj(bear).Counter("P1P1"); got != 1 {
		t.Fatalf("Broker Bear has %d P1P1 counters, want 1", got)
	}
	if got := e.G.Obj(gideonID).Counter("LOYALTY"); got != 5 {
		t.Fatalf("Gideon has %d LOYALTY counters, want 5 (entered at 4, sweep added 1)", got)
	}
	replayCheck(t, e, cfg)
}

// TestPutCounterAllExoticShapesStayLoud: registering the API removes the
// generic unimplemented-API fallback, so the shapes the core sweep cannot
// express -- Placer$ (who places), and a ValidZone$ naming a zone other than
// the battlefield (the two suspended-TIME carriers) -- must emit an explicit
// unimplemented-shape Note and place NOTHING, never go silent.
func TestPutCounterAllExoticShapesStayLoud(t *testing.T) {
	placerSrc := "Name:Placer\nManaCost:2 U\nTypes:Creature Wizard\nPT:2/2\n" +
		"A:AB$ PutCounterAll | Cost$ 1 | Placer$ Controller | ValidCards$ Creature | CounterType$ P1P1 | CounterNum$ 1 | SpellDescription$ x\nOracle:x\n"
	zoneSrc := "Name:Timekeeper\nManaCost:2 U\nTypes:Creature Wizard\nPT:2/2\n" +
		"A:AB$ PutCounterAll | Cost$ 1 | ValidCards$ Card | CounterType$ TIME | CounterNum$ 2 | ValidZone$ Exile | SpellDescription$ x\nOracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 220, placerSrc, zoneSrc, counterBear("Loud Bear"))
	placer := moveSeeded(t, e, 0, placerSrc, state.ZBattlefield)
	bear := putCreature(t, e, 0, counterBear("Loud Bear"))
	timekeeper := moveSeeded(t, e, 0, zoneSrc, state.ZBattlefield)

	// Placer$ shape: offered, payable, then loud and inert.
	addMana(t, e, 0, "UU")
	e.Advance()
	opt := abilityOption(t, e, placer, 0)
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	if !hasNote(e, "unimplemented PutCounterAll shape") {
		t.Fatal("no unimplemented-shape note for the Placer$ sweep")
	}

	// ValidZone$ Exile shape: same contract.
	addMana(t, e, 0, "UU")
	e.Advance()
	opt = abilityOption(t, e, timekeeper, 0)
	submitChoices(t, e, opt.Index)
	passUntilStackEmpty(t, e, 20)
	if !hasNote(e, "unimplemented PutCounterAll shape") {
		t.Fatal("no unimplemented-shape note for the ValidZone$ Exile sweep")
	}

	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange {
			t.Fatalf("an exotic sweep placed counters anyway: %+v", ev)
		}
	}
	if got := e.G.Obj(bear).Counter("P1P1"); got != 0 {
		t.Fatalf("Loud Bear took %d counters from a declined shape, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// TestMethodsOfTheMightyCharmModePutsCounters pins the one repo-deck card
// whose behaviour this registration changes (avengers-assemble.json carries
// Methods of the Mighty): choosing its DB$ PutCounterAll Charm mode places a
// real +1/+1 batch on every creature you control -- before the fix the mode
// emitted the fallback Note and did nothing.
func TestMethodsOfTheMightyCharmModePutsCounters(t *testing.T) {
	methods := corpusCard(t, "Methods of the Mighty")
	e, cfg := putCounterTable(t, 210,
		[]*cards.Card{methods, card(t, counterBear("Charm Bear"))},
		[]*cards.Card{card(t, counterBear("Enemy Bear"))})
	// The spell is an instant: make sure it sits in hand so the cast option
	// is offered from there (the corpus deal may have left it in the library).
	var methodsID state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Methods of the Mighty" {
				methodsID = id
				if z == state.ZLibrary {
					e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZHand})
				}
			}
		}
	}
	if methodsID == 0 {
		t.Fatal("Methods of the Mighty was not dealt")
	}
	bear := findAndMoveToBattlefield(t, e, 0, "Charm Bear")
	enemy := findAndMoveToBattlefield(t, e, 1, "Enemy Bear")
	for _, r := range []string{"W", "W", "W", "W"} {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: r, Amount: 1})
	}
	e.priorityRound()

	d := e.Pending()
	if d == nil {
		t.Fatal("no decision pending")
	}
	idx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == methodsID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no cast option for Methods of the Mighty: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	mode := modeOptionContaining(t, e.Pending(), "counter on each creature you control")
	submitChoices(t, e, mode)
	passUntilStackEmpty(t, e, 30)

	if got := e.G.Obj(bear).Counter("P1P1"); got != 1 {
		t.Fatalf("Charm Bear has %d P1P1 counters, want 1", got)
	}
	if got := e.G.Obj(enemy).Counter("P1P1"); got != 0 {
		t.Fatalf("seat 1's Enemy Bear took %d counters, want 0", got)
	}
	replayCheck(t, e, cfg)
}
