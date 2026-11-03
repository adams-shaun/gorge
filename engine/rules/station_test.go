package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The Hearthhull report: these tests pin the Station integration
// (engine/rules/station.go) on the reported card -- Hearthhull, the Worldseed --
// including the negative offer gates and the KChoose continuation across
// Engine.Clone.

// findStationOption returns the "station" option for spacecraft id in d, or -1.
func findStationOption(d *decision.Decision, id state.ObjID) int {
	for _, o := range d.Options {
		if o.Kind == "station" && o.Obj == id {
			return o.Index
		}
	}
	return -1
}

// TestStationHearthhullCloneContinuationReplays is the Hearthhull leaf of the
// reported feedback, at the real corpus card: the sorcery-window option
// appears, the KChoose offers another untapped creature (a summoning-sick
// one -- being tapped to pay another permanent's cost is not sickness's own
// tap-ability gate, CR 302.6), and never the Station permanent itself. The
// choice is taken while a KChoose is pending, so the continuation survives an
// Engine.Clone taken exactly there: the clone answers, the original does not
// move, feeding the original the same intent lands both on the same chain
// head, and the whole game replays byte-identically from the log.
func TestStationHearthhullCloneContinuationReplays(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Hearthhull, the Worldseed"), card(t, bearSrc)},
		[]*cards.Card{})
	hearth := moveByName(t, e, 0, "Hearthhull, the Worldseed", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Bear", state.ZBattlefield)
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision (got %+v)", d)
	}
	idx := findStationOption(d, hearth)
	if idx < 0 {
		t.Fatalf("no station option for Hearthhull at sorcery speed: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit station: %v", err)
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("no tap pick after the station option (got %+v)", d)
	}
	pick, srcOffered := -1, false
	for _, o := range d.Options {
		if o.Obj == bear {
			pick = o.Index
		}
		if o.Obj == hearth {
			srcOffered = true
		}
	}
	if srcOffered {
		t.Fatal("the Station permanent itself was offered as its own tap payer")
	}
	if pick < 0 {
		t.Fatalf("the summoning-sick Bear was not offered as a station tap: %+v", d.Options)
	}

	// Clone with the tap pick outstanding and check the continuation on the
	// clone: the copied engine must hold the same pending decision and answer
	// it to the same board.
	c := e.Clone()
	if diff := diffGames(e.G, c.G); diff != "" {
		t.Fatalf("clone differs from original with the Station pick pending:\n%s", diff)
	}
	cd := c.Pending()
	if cd == nil || cd.Kind != decision.KChoose || cd.Seq != d.Seq || len(cd.Options) != len(d.Options) {
		t.Fatalf("clone's pending Station pick differs: %+v vs %+v", cd, d)
	}

	if err := c.Submit(decision.Intent{Seq: cd.Seq, Player: cd.Player, Choices: []int{pick}}); err != nil {
		t.Fatalf("submit tap pick on the clone: %v", err)
	}
	if !c.G.Obj(bear).Tapped {
		t.Fatal("the clone did not tap the chosen creature")
	}
	// CR 702.184a: the Station ability resolves from the stack.
	passUntilStackEmpty(t, c, 20)
	if got := c.G.Obj(hearth).Counter("CHARGE"); got != 2 {
		t.Fatalf("the clone's Hearthhull has %d CHARGE counters, want 2 (the Bear's power)", got)
	}
	if e.G.Obj(bear).Tapped || e.G.Obj(hearth).Counter("CHARGE") != 0 {
		t.Fatal("answering on the clone moved the original engine")
	}

	// The original answers the same pick: identical chain heads prove the
	// clone copied every piece of engine state the continuation reads.
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pick}}); err != nil {
		t.Fatalf("submit tap pick on the original: %v", err)
	}
	passUntilStackEmpty(t, e, 20)
	if e.L.Head() != c.L.Head() {
		t.Fatalf("chain heads differ after lockstep: %s vs %s", e.L.Head(), c.L.Head())
	}
	// The board was reached through events: one Tap for the creature, one
	// CounterChange for the CHARGE counters.
	tapped, charged := false, false
	for _, ev := range e.L.Events {
		if ev.Kind == events.Tap && ev.Obj == bear {
			tapped = true
		}
		if ev.Kind == events.CounterChange && ev.Obj == hearth && ev.Counter == "CHARGE" && ev.Amount == 2 {
			charged = true
		}
	}
	if !tapped || !charged {
		t.Fatalf("missing event-driven Station effects: tap=%v charge=%v", tapped, charged)
	}
	replayCheck(t, e, cfg)
	replayCheck(t, c, cfg)
}

// TestStationNegativeGates pins the offer gates the happy path cannot prove:
// with every other creature tapped the option is withheld (an unpayable
// station is never offered), with a spell on the stack the window is not
// sorcery speed so the option is withheld and returns when the stack drains,
// and an animated Hearthhull (a creature in its own right) is still never its
// own tap payer.
func TestStationNegativeGates(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)

	// Gate 1: no other untapped creature -- the Bear is tapped, so the
	// option must not be offered at all.
	e := corpusEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Hearthhull, the Worldseed"), card(t, bearSrc)},
		[]*cards.Card{})
	hearth := moveByName(t, e, 0, "Hearthhull, the Worldseed", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Bear", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Tap, Obj: bear})
	e.priorityRound()
	if d := e.Pending(); d != nil && findStationOption(d, hearth) >= 0 {
		t.Fatalf("station offered with no untapped creature to pay: %+v", d.Options)
	}

	// Gate 2: not sorcery speed. The Bear is cast from hand so the stack is
	// nonempty; the pending priority decision must not offer Station while
	// the spell resolves, and must offer it again once the stack drains.
	e2 := corpusEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "Hearthhull, the Worldseed"), card(t, bearSrc)},
		[]*cards.Card{})
	hearth2 := moveByName(t, e2, 0, "Hearthhull, the Worldseed", state.ZBattlefield)
	moveByName(t, e2, 0, "Bear", state.ZHand)
	addMana(t, e2, 0, "GG")
	castNamed(t, e2, "Bear")
	if len(e2.G.Stack) == 0 {
		t.Fatal("fixture error: the Bear never reached the stack")
	}
	if d := e2.Pending(); d != nil && findStationOption(d, hearth2) >= 0 {
		t.Fatalf("station offered with a spell on the stack: %+v", d.Options)
	}
	passUntilStackEmpty(t, e2, 40)
	e2.priorityRound()
	d2 := e2.Pending()
	if d2 == nil {
		t.Fatal("no priority after the stack drained")
	}
	if findStationOption(d2, hearth2) < 0 {
		t.Fatalf("station did not return at sorcery speed after the stack drained: %+v", d2.Options)
	}

	// Gate 3: the Station permanent itself is never a candidate, even when a
	// live type-layer effect makes Hearthhull a creature and it is untapped.
	e3 := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Hearthhull, the Worldseed")}, nil)
	hearth3 := moveByName(t, e3, 0, "Hearthhull, the Worldseed", state.ZBattlefield)
	e3.AddContinuous(state.ContinuousEffect{Source: hearth3, Controller: 0,
		Affects: "Card.Self", Layer: state.LType, AddTypes: []string{"Creature"}, UntilEOT: true})
	if cands := e3.stationCandidates(0, hearth3); len(cands) != 0 {
		t.Fatalf("animated Hearthhull offered as its own tap payer: %v", cands)
	}
	e3.priorityRound()
	if d := e3.Pending(); d != nil && findStationOption(d, hearth3) >= 0 {
		t.Fatalf("station offered whose only creature candidate is the source: %+v", d.Options)
	}
}

// stationActivate offers and answers Station on spacecraft, tapping payer.
func stationActivate(t *testing.T, e *Engine, spacecraft, payer state.ObjID) {
	t.Helper()
	e.priorityRound()
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		t.Fatalf("no priority decision (got %+v)", d)
	}
	idx := findStationOption(d, spacecraft)
	if idx < 0 {
		t.Fatalf("no station option: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("no tap pick after the station option (got %+v)", d)
	}
	for _, o := range d.Options {
		if o.Obj == payer {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("payer not offered: %+v", d.Options)
}

// TestStationUsesTheStack pins CR 702.184a: Station is an ACTIVATED ability
// ("Tap another untapped creature you control: Put a number of charge
// counters on this permanent equal to the tapped creature's power. Activate
// only as a sorcery."), so after its cost is paid it waits on the stack (CR
// 602.2a, 405.1) and places nothing until it resolves; the amount is the
// tapped creature's power as the ability resolves (a +2/+0 applied while it
// waits counts); and the counters are an EFFECT of a resolving ability, so
// Doubling Season's "if an effect would put one or more counters" doubles
// them (CR 122.6, 614.1a).
func TestStationUsesTheStack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)

	t.Run("waits on the stack, power read at resolution", func(t *testing.T) {
		e, cfg := corpusEngineCfg(t, reg,
			[]*cards.Card{lookup(t, reg, "Exploration Broodship"), card(t, bearSrc)}, nil)
		brood := moveByName(t, e, 0, "Exploration Broodship", state.ZBattlefield)
		bear := moveByName(t, e, 0, "Bear", state.ZBattlefield)
		stationActivate(t, e, brood, bear)
		if !e.G.Obj(bear).Tapped {
			t.Fatal("the cost did not tap the Bear")
		}
		if len(e.G.Stack) != 1 {
			t.Fatalf("stack %v, want the Station ability alone", e.G.Stack)
		}
		ab := e.G.Obj(e.G.Stack[0])
		if ab == nil || ab.Source != brood || ab.StackKind != state.StackKindActivated {
			t.Fatalf("stack object %+v is not the Broodship's activated ability", ab)
		}
		if got := e.G.Obj(brood).Counter("CHARGE"); got != 0 {
			t.Fatalf("%d CHARGE counters before the ability resolved, want 0", got)
		}
		e.AddContinuous(state.ContinuousEffect{Source: bear, Controller: 0, Affects: "Card.Self",
			Layer: state.LPT, Sub: state.SubModify, AddPower: 2, UntilEOT: true})
		passUntilStackEmpty(t, e, 20)
		if got := e.G.Obj(brood).Counter("CHARGE"); got != 4 {
			t.Fatalf("%d CHARGE counters, want 4 (the Bear's power 2+2 as the ability resolved)", got)
		}
		replayCheck(t, e, cfg)
	})

	t.Run("Doubling Season doubles the counters", func(t *testing.T) {
		e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Exploration Broodship"),
			lookup(t, reg, "Doubling Season"), card(t, bearSrc)}, nil)
		brood := moveByName(t, e, 0, "Exploration Broodship", state.ZBattlefield)
		moveByName(t, e, 0, "Doubling Season", state.ZBattlefield)
		bear := moveByName(t, e, 0, "Bear", state.ZBattlefield)
		stationActivate(t, e, brood, bear)
		passUntilStackEmpty(t, e, 20)
		if got := e.G.Obj(brood).Counter("CHARGE"); got != 4 {
			t.Fatalf("%d CHARGE counters, want 4 (2 doubled: the counters are a resolving ability's effect)", got)
		}
	})
}

// TestEveryStationCarrierStationsThroughTheStack is the corpus census for
// the Station class (CR 702.184a): every K:Station carrier, on the
// battlefield beside a 2/2, offers Station, pays it by tapping the 2/2, puts
// the ability on the stack with no counter placed yet, and resolves it to two
// CHARGE counters. A carrier that breaks any of the four fails by name.
func TestEveryStationCarrierStationsThroughTheStack(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var names []string
	for _, c := range reg.AllCards() {
		if c == nil || len(c.Faces) == 0 || c.Faces[0] == nil {
			continue
		}
		if _, ok := c.Faces[0].KeywordParam("Station"); ok {
			names = append(names, c.Faces[0].Name)
		}
	}
	if len(names) == 0 {
		t.Fatal("no Station carriers in the corpus: the census read nothing")
	}
	for _, name := range names {
		e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, name), card(t, bearSrc)},
			[]*cards.Card{lookup(t, reg, "Plains")})
		// The Spacecraft enters first and its enters trigger (Wedgelight
		// Rammer's Robot, Candela's bounce, Extinguisher Battleship's destroy
		// -- aimed at p1's Plains, never the ship) resolves before the Bear
		// arrives, so the census stations in an empty-stack sorcery window
		// and the Bear is neither bounced nor pumped.
		moveByName(t, e, 1, "Plains", state.ZBattlefield)
		ship := moveByName(t, e, 0, name, state.ZBattlefield)
		stationCensusDrain(t, e, ship)
		// A ship whose enters trigger phases itself out is not on the table
		// to station until its controller's untap step (CR 702.26b); the
		// census skips that wait and phases it back in.
		if e.G.Obj(ship).PhasedOut {
			e.emit(events.Event{Kind: events.PhaseOut, Obj: ship, Amount: -1})
		}
		bear := moveByName(t, e, 0, "Bear", state.ZBattlefield)
		stationCensusDrain(t, e, ship)
		want := e.tapPowerValue(bear, stationSAKind)
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority || findStationOption(d, ship) < 0 {
			t.Errorf("%s: Station not offered at sorcery speed beside an untapped 2/2", name)
			continue
		}
		stationActivate(t, e, ship, bear)
		if len(e.G.Stack) != 1 || e.G.Obj(ship).Counter("CHARGE") != 0 {
			t.Errorf("%s: after paying Station the stack is %v with %d CHARGE counters, want the ability alone and 0",
				name, e.G.Stack, e.G.Obj(ship).Counter("CHARGE"))
			continue
		}
		stationCensusDrain(t, e, ship)
		if got := e.G.Obj(ship).Counter("CHARGE"); got != want {
			t.Errorf("%s: %d CHARGE counters after Station resolved, want %d (the Bear's power)", name, got, want)
		}
	}
}

// stationCensusDrain opens a priority round and resolves the stack, passing
// priority and answering any other ask with its minimum (an "up to one"
// target ask with none), never picking avoid while another option exists.
func stationCensusDrain(t *testing.T, e *Engine, avoid state.ObjID) {
	t.Helper()
	e.priorityRound()
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil || (d.Kind == decision.KPriority && len(e.G.Stack) == 0) {
			return
		}
		if d.Kind == decision.KPriority {
			passFirst(t, e)
			continue
		}
		var picks []int
		for _, o := range d.Options {
			if len(picks) < d.Min && (o.Obj != avoid || len(d.Options) <= d.Min) {
				picks = append(picks, o.Index)
			}
		}
		// A yes/no ask (an optional trigger) tags every option with its source,
		// so when avoiding the ship leaves too few picks, take the first ones.
		for _, o := range d.Options {
			if len(picks) < d.Min && !slices.Contains(picks, o.Index) {
				picks = append(picks, o.Index)
			}
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: picks}); err != nil {
			t.Fatalf("answer %s: %v", d.Kind, err)
		}
	}
	t.Fatal("stack never drained")
}
