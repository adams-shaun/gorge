package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Task prolif1: api:Proliferate (CR 701.27) end to end on the REAL corpus
// carriers. Before this task `DB$ Proliferate` was unregistered, so
// Tezzeret's Gambit's "then proliferate" half was one "unimplemented API
// Proliferate" Note, and Karn's Bastion's {4},{T} activation did nothing.
//
// Every fixture mutation goes through e.emit, so every test ends
// replay-verified.

// proliferateEngine deals a corpus-only game whose seat 0 library holds the
// named fixtures, drives to seat 0's Main1, and returns the engine.
func proliferateEngine(t *testing.T, reg *cards.Registry, fixtures ...string) (*Engine, Config) {
	t.Helper()
	mountain := searchCorpusCard(t, reg, "Mountain")
	grizzly := searchCorpusCard(t, reg, "Grizzly Bears")
	deck := make([]*cards.Card, 0, 40)
	for _, name := range fixtures {
		deck = append(deck, searchCorpusCard(t, reg, name))
	}
	for len(deck) < 40 {
		deck = append(deck, grizzly)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 7717, Names: []string{"prolif", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// putNamedOnBattlefield moves the first found copy of name from seat 0's
// hand/library to the battlefield and returns its id.
func putNamedOnBattlefield(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			o := e.G.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == name {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: z, To: state.ZBattlefield})
				e.pending = nil
				e.priorityRound()
				return id
			}
		}
	}
	t.Fatalf("corpus fixture %q absent", name)
	return 0
}

// putCountersOn adds n counters of kind to the first battlefield permanent
// seat p controls whose name matches, returning its id.
func putCountersOn(t *testing.T, e *Engine, p state.PlayerID, name, kind string, n int32) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == name {
			if n > 0 {
				e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: n})
			}
			return id
		}
	}
	t.Fatalf("no battlefield permanent %q for seat %d", name, p)
	return 0
}

// answerPips answers the pending mana-cost-payment choose (the Phyrexian/hybrid
// pip ask a cast poses at CR 601.2g) by taking its first offered face, if one
// is pending. A no-op otherwise.
func answerPips(t *testing.T, e *Engine) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 {
		return
	}
	for _, o := range d.Options {
		if len(o.Kind) > 4 && o.Kind[:4] == "pay_" {
			submitChoices(t, e, o.Index)
			return
		}
	}
}

// answerProliferate submits the named option indices of a pending
// proliferate ask and drives to the next non-priority decision.
func answerProliferate(t *testing.T, e *Engine, d *decision.Decision, opts ...int) *decision.Decision {
	t.Helper()
	if d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		t.Fatalf("pending decision = %+v, want a proliferate KChoose", d)
	}
	choices := make([]int, 0, len(opts))
	for _, o := range opts {
		choices = append(choices, d.Options[o].Index)
	}
	submitChoices(t, e, choices...)
	return passUntilNonPriority(t, e, 60)
}

// TestTezzeretsGambitProliferatesEndToEnd is the reported carrier: cast the
// spell (the Phyrexian pip UP is paid with blue mana from the pool), draw two,
// and the proliferate ask is posted to the caster. The answered batch puts +1
// of each kind on the chosen permanent AND a real PlayerCounterChange on the
// chosen player; a permanent carrying no counter is not in the option list.
func TestTezzeretsGambitProliferatesEndToEnd(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Tezzeret's Gambit", "Grizzly Bears")

	// A carrier permanent and a bare permanent for seat 0; poison on seat 0.
	// Two Grizzly Bears (the deck's filler) are moved onto the battlefield: one
	// carries counters, one does not.
	putNamedOnBattlefield(t, e, "Grizzly Bears")
	putNamedOnBattlefield(t, e, "Grizzly Bears")
	e.priorityRound()
	carrier := putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 2)
	var bare state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if id != carrier && e.G.Obj(id).Face() != nil && e.G.Obj(id).Face().Name == "Grizzly Bears" {
			bare = id
		}
	}
	if bare == 0 {
		t.Fatal("no second bare Grizzly Bears on the battlefield")
	}
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: 1})
	e.priorityRound()

	// Count the bare permanent's absence by name is ambiguous (same card); use
	// the ids directly.
	gambit := findAndMoveToHand(t, e, 0, "Tezzeret's Gambit")
	handAfterMove := len(e.G.Zone(state.ZHand, 0))
	addMana(t, e, 0, "UUUU")
	castFromPriority(t, e, gambit)
	answerPips(t, e)

	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		t.Fatalf("decision = %+v, want the proliferate ask", d)
	}
	if d.Player != 0 {
		t.Fatalf("proliferate ask player = %d, want the caster 0", d.Player)
	}
	// Draw two already happened.
	if got := len(e.G.Zone(state.ZHand, 0)); got != handAfterMove-1+2 {
		t.Fatalf("hand = %d, want %d (spell left, two drawn)", got, handAfterMove-1+2)
	}
	// The bare permanent must not be offered.
	optForObj := func(id state.ObjID) int {
		for i, o := range d.Options {
			if o.Obj == id {
				return i
			}
		}
		return -1
	}
	if optForObj(bare) != -1 {
		t.Fatalf("a zero-counter permanent was offered: %+v", d.Options)
	}
	carrierOpt := optForObj(carrier)
	if carrierOpt < 0 {
		t.Fatalf("carrier not offered: %+v", d.Options)
	}
	playerOpt := -1
	for i, o := range d.Options {
		if o.Obj == 0 && o.Player == 0 {
			playerOpt = i
		}
	}
	if playerOpt < 0 {
		t.Fatalf("counter-carrying player not offered: %+v", d.Options)
	}

	answerProliferate(t, e, d, carrierOpt, playerOpt)
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 3 {
		t.Fatalf("carrier P1P1 = %d, want 3 (+1)", got)
	}
	if got := e.G.Players[0].Counter("POISON"); got != 2 {
		t.Fatalf("player POISON = %d, want 2 (+1)", got)
	}
	sawPlayer := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.PlayerCounterChange && ev.Player == 0 &&
			ev.Counter == "POISON" && ev.Amount == 1 {
			sawPlayer = true
		}
	}
	if !sawPlayer {
		t.Fatal("no PlayerCounterChange on the chosen player")
	}
	replayCheck(t, e, cfg)
}

// TestKarnsBastionActivationProliferates pins the AB$ spelling: {4},{T} is
// offered, resolves, asks, and the answered batch lands.
func TestKarnsBastionActivationProliferates(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Karn's Bastion", "Grizzly Bears")
	karn := searchMoveByName(t, e, "Karn's Bastion", state.ZBattlefield)
	putNamedOnBattlefield(t, e, "Grizzly Bears")
	carrier := putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 1)
	addMana(t, e, 0, "CCCC")

	// Karn's Bastion prints two ABs: index 0 is the mana ability, index 1 is
	// proliferate. The mana ability's cost is {T}, so activating index 1
	// ({4},{T}) requires the land untapped.
	opt, ok := findAbilityOption(e, karn, 1)
	if !ok {
		t.Fatalf("Karn's Bastion proliferate ability not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		t.Fatalf("decision = %+v, want the proliferate ask", d)
	}
	answerProliferate(t, e, d, 0)
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 2 {
		t.Fatalf("carrier P1P1 = %d, want 2 (+1)", got)
	}
	replayCheck(t, e, cfg)
}

// TestContagionEngineProliferatesTwice pins the Amount$ 2 shape on a real
// carrier: the answer adds +2 of each kind (one ask, the documented
// single-batch read).
func TestContagionEngineProliferatesTwice(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Contagion Engine", "Grizzly Bears")
	engineID := searchMoveByName(t, e, "Contagion Engine", state.ZBattlefield)
	putNamedOnBattlefield(t, e, "Grizzly Bears")
	carrier := putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 1)
	addMana(t, e, 0, "CCCCCC")

	opt, ok := findAbilityOption(e, engineID, 0)
	if !ok {
		t.Fatalf("Contagion Engine proliferate ability not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.ResumeKind != "proliferate" {
		t.Fatalf("decision = %+v, want the proliferate ask", d)
	}
	carrierOpt := -1
	for i, o := range d.Options {
		if o.Obj == carrier {
			carrierOpt = i
		}
	}
	if carrierOpt < 0 {
		t.Fatalf("carrier not offered: %+v", d.Options)
	}
	answerProliferate(t, e, d, carrierOpt)
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 3 {
		t.Fatalf("carrier P1P1 = %d, want 3 (+2)", got)
	}
	replayCheck(t, e, cfg)
}

// TestProliferateZeroEligibleCompletesSilently: resolving proliferate with
// nothing carrying a counter poses no ask, records no Note, and completes
// (no wedge).
func TestProliferateZeroEligibleCompletesSilently(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Tezzeret's Gambit")
	gambit := findAndMoveToHand(t, e, 0, "Tezzeret's Gambit")
	handAfterMove := len(e.G.Zone(state.ZHand, 0))
	addMana(t, e, 0, "UUUU")
	castFromPriority(t, e, gambit)
	answerPips(t, e)
	d := passUntilNonPriority(t, e, 60)
	// With nothing carrying a counter the spell simply resolves: the next
	// decision is an ordinary priority round, never a proliferate ask.
	if d != nil && d.Kind == decision.KChoose && d.ResumeKind == "proliferate" {
		t.Fatalf("zero-eligible proliferate posed an ask: %+v", d)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handAfterMove-1+2 {
		t.Fatalf("hand = %d, want %d (spell left, two drawn)", got, handAfterMove-1+2)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Text == "unimplemented API Proliferate" {
			t.Fatal("proliferate still hit the unimplemented fallback")
		}
	}
	replayCheck(t, e, cfg)
}

// TestProliferateAsLoyaltyAbilityIsOncePerTurn pins the CR 606 path on the
// exact ability body Ichormoon Gauntlet grants its planeswalkers ("[0]:
// Proliferate" -- `Cost$ AddCounter<0/LOYALTY> | Planeswalker$ True`): the
// ability is classified a loyalty ability, offered, its free cost consumes
// nothing, and the second activation in the same turn is withheld (CR 606.3).
// The proliferate then asks and adds +1 loyalty to the walker it names.
//
// NOTE: Ichormoon Gauntlet's own Continuous `AddAbility$ PWProliferate &
// PWExtraTurn` grant is NOT offered by this build — rules' `grantedAbilities`
// reads state.ContinuousEffect.AddAbilities, which nothing populates for a
// printed Continuous AddAbility$ static — so the carrier is pinned through a
// synthetic walker printing the SAME body. See the report's Issues section.
func TestProliferateAsLoyaltyAbilityIsOncePerTurn(t *testing.T) {
	t.Parallel()
	src := "Name:Testwalker\nTypes:Planeswalker Test\nLoyalty:3\n" +
		"A:AB$ Proliferate | Cost$ AddCounter<0/LOYALTY> | Planeswalker$ True | SpellDescription$ Proliferate.\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 8801, src)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepMain1})
	e.priorityRound()

	if got := e.G.Obj(id).Counter("LOYALTY"); got != 3 {
		t.Fatalf("walker entered with %d loyalty, want 3", got)
	}
	opt, ok := findAbilityOption(e, id, 0)
	if !ok {
		t.Fatalf("the proliferate loyalty ability was not offered: %+v", e.Pending().Options)
	}
	loyaltyBefore := e.G.Obj(id).Counter("LOYALTY")
	submitChoices(t, e, opt.Index)
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.ResumeKind != "proliferate" {
		t.Fatalf("decision = %+v, want the proliferate ask", d)
	}
	selfOpt := -1
	for i, o := range d.Options {
		if o.Obj == id {
			selfOpt = i
		}
	}
	if selfOpt < 0 {
		t.Fatalf("the walker was not offered: %+v", d.Options)
	}
	answerProliferate(t, e, d, selfOpt)
	if got := e.G.Obj(id).Counter("LOYALTY"); got != loyaltyBefore+1 {
		t.Fatalf("walker loyalty = %d, want %d (+1 proliferate)", got, loyaltyBefore+1)
	}
	// CR 606.3: the second activation in the same turn is withheld.
	e.priorityRound()
	if _, ok := findAbilityOption(e, id, 0); ok {
		t.Fatal("the loyalty ability was offered a second time in the same turn")
	}
	replayCheck(t, e, cfg)
}
