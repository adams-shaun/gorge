package rules

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runG120Scenario runs one oracle-form scenario and fails with its transcript.
func runG120Scenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	var sc oracleScenario
	if err := json.Unmarshal([]byte(raw), &sc); err != nil {
		t.Fatal(err)
	}
	if fails, tr, _ := runOracleScenario(reg, sc); len(fails) > 0 {
		t.Fatalf("%s\n  transcript:\n    %s", strings.Join(fails, "\n"), strings.Join(tr, "\n    "))
	}
}

// TestPutCounterRememberAmountFeedsReflexiveTrigger pins CR 603.12 on
// Earthbender Ascension: the landfall quest counter that lands remembers its
// amount, so the "when you do" trigger fires and gives the target creature a
// +1/+1 counter and trample. The precondition (3 quest counters before, 4
// after) is asserted on the enchantment itself.
func TestPutCounterRememberAmountFeedsReflexiveTrigger(t *testing.T) {
	t.Parallel()
	runG120Scenario(t, `{"name":"earthbender","cr":["603.12"],"why":"x",
	 "setup":{"p0":{"battlefield":["Earthbender Ascension","Grizzly Bears"],"hand":["Forest"],"counters":{"Earthbender Ascension":{"QUEST":3}}}},
	 "steps":[{"op":"play","seat":0,"card":"p0:Forest"},{"op":"resolve","targets":["p0:Grizzly Bears"]},{"op":"resolve"}],
	 "expect":[{"card":"p0:Earthbender Ascension","counters":{"QUEST":4}},{"card":"p0:Grizzly Bears","counters":{"P1P1":1},"keywords":["Trample"],"pt":"3/3"}]}`)
}

// TestWishingWellCoinCounterCastsMatchingManaValue: the coin counter's
// remembered amount fires the "when you do" cast of an instant whose mana
// value equals the coin counters (2 after the put).
func TestWishingWellCoinCounterCastsMatchingManaValue(t *testing.T) {
	t.Parallel()
	runG120Scenario(t, `{"name":"well","cr":["603.12"],"why":"x",
	 "setup":{"p0":{"battlefield":["Wishing Well"],"graveyard":["Think Twice"],"counters":{"Wishing Well":{"COIN":1}}}},
	 "steps":[{"op":"activate","seat":0,"card":"p0:Wishing Well"},{"op":"resolve","targets":["p0:Think Twice"],"answers":[{"kind":"modes","pick":["Play Think Twice"]}]},{"op":"resolve"},{"op":"resolve"}],
	 "expect":[{"card":"p0:Wishing Well","counters":{"COIN":2}},{"card":"p0:Think Twice","zone":"exile"}]}`)
}

// g120Chain resolves sa (and its SubAbility$ chain) for source on seat 0
// through the real kernel, with targets as the root's chosen targets.
func g120Chain(e *Engine, face *cards.Face, src state.ObjID, sa *cards.SA, targets []state.Target, remembered []state.Target) {
	kr4Resolve(e, func() *effects.Ctx {
		return &effects.Ctx{Source: src, Controller: 0, SVars: face.SVars, Targets: targets, Remembered: remembered, TargetsOffered: len(targets) > 0}
	}, sa)
}

func g120Sa(t *testing.T, face *cards.Face, name, api string) *cards.SA {
	t.Helper()
	sa := cards.ResolveSVar(face.SVars, name)
	if sa == nil || sa.API != api {
		t.Fatalf("%s = %+v, want %s", name, sa, api)
	}
	return sa
}

// g120DragonBoard seats Call the Spirit Dragons on p0's battlefield beside
// the named Dragons and returns the engine, the enchantment and the Dragons.
func g120DragonBoard(t *testing.T, names ...string) (*Engine, state.ObjID, []state.ObjID) {
	t.Helper()
	e := corpusTokenEngine(t)
	call := onBoardCard(t, e, 0, mshCorpusCard(t, "Call the Spirit Dragons"))
	var ds []state.ObjID
	for _, n := range names {
		ds = append(ds, onBoardCard(t, e, 0, mshCorpusCard(t, n)))
	}
	return e, call, ds
}

func g120CallResolve(t *testing.T, e *Engine, call state.ObjID) {
	t.Helper()
	face := e.G.Obj(call).Face()
	g120Chain(e, face, call, g120Sa(t, face, "PutCounter", "PutCounter"), nil, nil)
}

// TestCallTheSpiritDragonsFiveDistinctWins: one monocoloured Dragon per
// colour each take a +1/+1 counter, five different Dragons, and p0 wins
// (CR 105.1 for the five colours).
func TestCallTheSpiritDragonsFiveDistinctWins(t *testing.T) {
	t.Parallel()
	e, call, ds := g120DragonBoard(t, "Alabaster Dragon", "Archive Dragon", "Black Dragon", "Shivan Dragon", "Canopy Dragon")
	if e.G.Over {
		t.Fatal("precondition: the game is already over")
	}
	g120CallResolve(t, e, call)
	for i, id := range ds {
		if got := e.G.Obj(id).Counter("P1P1"); got != 1 {
			t.Fatalf("Dragon %d P1P1 = %d, want 1", i, got)
		}
	}
	if !e.G.Over || e.G.Winner != 0 {
		t.Fatalf("over=%v winner=%d, want p0 to win", e.G.Over, e.G.Winner)
	}
}

// TestCallTheSpiritDragonsOneMulticolourDragonDoesNotWin: a lone WUBRG
// Dragon is the pick of all five colours, so it takes five counters but is
// one Dragon, not five.
func TestCallTheSpiritDragonsOneMulticolourDragonDoesNotWin(t *testing.T) {
	t.Parallel()
	e, call, ds := g120DragonBoard(t, "Scion of the Ur-Dragon")
	g120CallResolve(t, e, call)
	if got := e.G.Obj(ds[0]).Counter("P1P1"); got != 5 {
		t.Fatalf("Scion P1P1 = %d, want 5", got)
	}
	if e.G.Over {
		t.Fatalf("game over (winner %d) after five counters on one Dragon", e.G.Winner)
	}
	if o := e.G.Obj(call); o.ChosenColor != "" {
		t.Fatalf("source ChosenColor = %q, want the transient colour never written to the object", o.ChosenColor)
	}
}

// TestPutCounterUpToAsksAmount: Esper Terra's chapter copy of a Saga takes
// the number of lore counters the controller picks (up to three), not
// always three.
func TestPutCounterUpToAsksAmount(t *testing.T) {
	t.Parallel()
	e := corpusTokenEngine(t)
	terra := onBoardCard(t, e, 0, mshCorpusCard(t, "Terra, Magical Adept"))
	face := e.G.Obj(terra).Card.Faces[1]
	saga := onBoard(t, e, 0, "Name:Test Saga\nManaCost:1\nTypes:Enchantment Saga\nOracle:x\n")
	sa := g120Sa(t, face, "DBPutCounter", "PutCounter")
	g120Chain(e, face, terra, sa, nil, []state.Target{{Obj: saga}})
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "counter_upto" {
		t.Fatalf("pending = %+v, want the up-to amount ask", d)
	}
	var labels []string
	pick := -1
	for _, o := range d.Options {
		labels = append(labels, o.Label)
		if o.Amount == 1 {
			pick = o.Index
		}
	}
	if strings.Join(labels, ",") != "3,2,1,0" || pick < 0 {
		t.Fatalf("options = %v, want 3,2,1,0", labels)
	}
	submitChoices(t, e, pick)
	if got := e.G.Obj(saga).Counter("LORE"); got != 1 {
		t.Fatalf("LORE after answering 1 = %d, want 1", got)
	}
}

// g120RhysBoard seats Rhys and a Grizzly Bears with the given counters.
func g120RhysBoard(t *testing.T, counters map[string]int32) (*Engine, *cards.Face, state.ObjID, state.ObjID, *cards.SA) {
	t.Helper()
	e := corpusTokenEngine(t)
	rhys := onBoardCard(t, e, 0, mshCorpusCard(t, "Rhys, the Evermore"))
	bear := onBoardCard(t, e, 0, mshCorpusCard(t, "Grizzly Bears"))
	for _, k := range []string{"M1M1", "P1P1", "STUN"} {
		if n := counters[k]; n > 0 {
			e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: k, Amount: n})
		}
	}
	face := e.G.Obj(rhys).Face()
	var sa *cards.SA
	for _, a := range face.Abilities {
		if a.API == "RemoveCounter" {
			sa = a
		}
	}
	if sa == nil {
		t.Fatal("Rhys has no RemoveCounter ability")
	}
	return e, face, rhys, bear, sa
}

// TestRemoveCounterAnyNumberRhys: Rhys's "remove any number of counters"
// takes as many as the controller picks, down to none.
func TestRemoveCounterAnyNumberRhys(t *testing.T) {
	t.Parallel()
	e, face, rhys, bear, sa := g120RhysBoard(t, map[string]int32{"M1M1": 3})
	if got := e.G.Obj(bear).Counter("M1M1"); got != 3 {
		t.Fatalf("precondition: M1M1 = %d, want 3", got)
	}
	g120Chain(e, face, rhys, sa, []state.Target{{Obj: bear}}, nil)
	for i := 0; i < 3; i++ {
		d := e.Pending()
		if d == nil || d.ResumeKind != "counter_kind" {
			t.Fatalf("ask %d = %+v, want a counter_kind ask", i, d)
		}
		submitChoices(t, e, optionIndexByLabel(d, "M1M1"))
	}
	if got := e.G.Obj(bear).Counter("M1M1"); got != 0 {
		t.Fatalf("M1M1 after picking three = %d, want 0", got)
	}

	// Pick none: Done at the first step removes nothing.
	e, face, rhys, bear, sa = g120RhysBoard(t, map[string]int32{"M1M1": 3})
	g120Chain(e, face, rhys, sa, []state.Target{{Obj: bear}}, nil)
	d := e.Pending()
	done := -1
	for _, o := range d.Options {
		if o.Kind == "remove_counter_done" {
			done = o.Index
		}
	}
	if d == nil || done < 0 {
		t.Fatalf("first ask = %+v, want a Done option", d)
	}
	submitChoices(t, e, done)
	if got := e.G.Obj(bear).Counter("M1M1"); got != 3 {
		t.Fatalf("M1M1 after Done = %d, want 3", got)
	}
}

// TestRemoveCounterChoicesAnyKindMisterHyde: mode 2 removes a counter of the
// kind the controller names from the chosen creature, and only then draws.
func TestRemoveCounterChoicesAnyKindMisterHyde(t *testing.T) {
	t.Parallel()
	e := corpusTokenEngine(t)
	hyde := onBoardCard(t, e, 0, mshCorpusCard(t, "Mister Hyde, Monster Within"))
	bear := onBoardCard(t, e, 0, mshCorpusCard(t, "Grizzly Bears"))
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "STUN", Amount: 1})
	// Hyde itself has no counters, so the bear is the only HasCounters creature.
	face := e.G.Obj(hyde).Face()
	handBefore := len(e.G.Zone(state.ZHand, 0))
	g120Chain(e, face, hyde, g120Sa(t, face, "DBRemoveCounter", "RemoveCounter"), nil, nil)
	d := e.Pending()
	if d == nil || d.ResumeKind != "counter_kind" {
		t.Fatalf("pending = %+v, want the counter kind ask", d)
	}
	submitChoices(t, e, optionIndexByLabel(d, "STUN"))
	o := e.G.Obj(bear)
	if o.Counter("STUN") != 0 || o.Counter("P1P1") != 1 {
		t.Fatalf("counters = %+v, want STUN removed and P1P1 kept", o.Counters)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+1 {
		t.Fatalf("hand = %d, want %d (the rider draws)", got, handBefore+1)
	}
}

// TestPutCounterAllSubLinkPlayerTarget: Twisted Fates' PutCounterAll link
// targets its own player (p0), not the root Destroy's object target.
func TestPutCounterAllSubLinkPlayerTarget(t *testing.T) {
	t.Parallel()
	e := corpusTokenEngine(t)
	fates := onBoardCard(t, e, 0, mshCorpusCard(t, "Twisted Fates"))
	mine := onBoardCard(t, e, 0, mshCorpusCard(t, "Grizzly Bears"))
	theirs := onBoardCard(t, e, 1, mshCorpusCard(t, "Grizzly Bears"))
	victim := onBoardCard(t, e, 1, mshCorpusCard(t, "Ornithopter"))
	face := e.G.Obj(fates).Face()
	kr4Resolve(e, func() *effects.Ctx {
		c := &effects.Ctx{Source: fates, Controller: 0, SVars: face.SVars,
			Targets: []state.Target{{Obj: victim}}, TargetsOffered: true}
		c.PickedTargets = []state.Target{{Player: 0, IsPlayer: true}}
		return c
	}, g120Sa(t, face, "DBPutCounterAll", "PutCounterAll"))
	if got := e.G.Obj(mine).Counter("P1P1"); got != 1 {
		t.Fatalf("p0's creature P1P1 = %d, want 1", got)
	}
	if got := e.G.Obj(theirs).Counter("P1P1"); got != 0 {
		t.Fatalf("p1's creature P1P1 = %d, want 0", got)
	}
}

// TestPutCounterAllSubLinkPlayerTargetCastEndToEnd drives the real cast of
// Twisted Fates: both targets announced at cast, the Destroy kills p1's
// Ornithopter, and only the targeted player's (p0's) creature grows.
func TestPutCounterAllSubLinkPlayerTargetCastEndToEnd(t *testing.T) {
	t.Parallel()
	runG120Scenario(t, `{"name":"fates","cr":["608.2b"],"why":"x",
	 "setup":{"p0":{"battlefield":["Grizzly Bears"],"hand":["Twisted Fates"]},"p1":{"battlefield":["Ornithopter","Runeclaw Bear"]}},
	 "steps":[{"op":"cast","seat":0,"card":"p0:Twisted Fates","mana":"CCWWB","targets":["p1:Ornithopter","p0"]},{"op":"resolve"}],
	 "expect":[{"card":"p1:Ornithopter","zone":"graveyard"},{"card":"p0:Grizzly Bears","counters":{"P1P1":1}},{"card":"p1:Runeclaw Bear","counters":{"P1P1":0}}]}`)
}
