package rules

// charm2: a distinct-mode Charm whose chosen target-bearing modes do not all
// declare exactly 1..1 targets. CR 601.2c makes every chosen mode declare and
// choose its OWN targets; the single combined decision the 1..1 shape uses
// cannot express a mode with different (or plural) bounds, so the engine asks
// each such mode sequentially and binds the answers through the same
// positional machinery (recheckCharmTargets -> charmDistinctTargetRun). These
// tests pin the plural shape, the order-flip, the trigger path, the infeasible
// plural minimum, and the choose-one latent shape that must keep the ordinary
// flat ask.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// pluralCharmScript is the measured repro from the report: a CharmNum$ 2
// Charm whose first mode is a 1..1 player target and whose second is a plural
// "exile up to two target cards from graveyards" (TargetMin$ 0 / TargetMax$ 2).
// LoseLife is observably seat-indexed and ChangeZone is observably
// zone-indexed, so both modes' bindings are independently assertable.
func pluralCharmScript() string {
	return "Name:Plural Charm\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ Charm | CharmNum$ 2 | Choices$ LoseMode,ExileMode\n" +
		"SVar:LoseMode:DB$ LoseLife | ValidTgts$ Player | LifeAmount$ 2 | SpellDescription$ Target player loses 2 life.\n" +
		"SVar:ExileMode:DB$ ChangeZone | TargetMin$ 0 | TargetMax$ 2 | Origin$ Graveyard | Destination$ Exile | TgtPrompt$ Choose target card in a graveyard | ValidTgts$ Card | SpellDescription$ Exile up to two target cards from graveyards.\n" +
		"Oracle:x\n"
}

// otherCreatureScript is a second, distinct graveyard fixture so a plural
// "up to two" mode can offer TWO candidates and its 0..2 bound is not clamped
// to a single one.
const otherCreatureScript = "Name:Other Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// charmModeOptions returns the indices of the options in d whose Obj is obj.
func charmModeOptions(d *decision.Decision, obj state.ObjID) []int {
	var out []int
	for _, o := range d.Options {
		if o.Obj == obj {
			out = append(out, o.Index)
		}
	}
	return out
}

// TestPluralCharmModeGetsItsOwnTargetAsk is the measured repro. With the
// 1..1-only gate reverted, the second (exile) mode never asks: the single flat
// ask is Min == Max == 1 over players, the Bear stays in the graveyard, and
// only the lose mode's target lands.
func TestPluralCharmModeGetsItsOwnTargetAsk(t *testing.T) {
	t.Parallel()
	charm := pluralCharmScript()
	e, cfg, id := newFixtureDeck(t, 6310, charm, vanillaCreatureScript, otherCreatureScript)
	bear := addToGraveyard(t, e, 0, vanillaCreatureScript)
	if other := addToGraveyard(t, e, 0, otherCreatureScript); other == 0 {
		t.Fatal("precondition: second graveyard card was not seeded")
	}
	// Preconditions the assertions depend on: the Bear is really in a
	// graveyard (the exile mode's origin), and seat 1's life makes the 2-point
	// loss non-vacuous.
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: seeded Bear zone = %+v, want graveyard", o)
	}
	if e.G.Players[1].Life != 20 {
		t.Fatalf("precondition: seat 1 life = %d, want 20", e.G.Players[1].Life)
	}
	addMana(t, e, 0, "B")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the cast mode announcement", d)
	}
	submitChoices(t, e, 0, 1) // LoseMode then ExileMode

	// Mode 0 (1..1 player): its own ask.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 1 || d.Max != 1 {
		t.Fatalf("pending = %+v, want mode 0's 1..1 player ask", d)
	}
	if d.ResumeKind != "charm_mode_seq" {
		t.Fatalf("mode 0 ask resume kind = %q, want the sequential distinct-mode ask", d.ResumeKind)
	}
	seat1 := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			seat1 = o.Index
		}
	}
	if seat1 < 0 {
		t.Fatalf("mode 0 ask did not offer seat 1: %+v", d.Options)
	}
	submitChoices(t, e, seat1)

	// Mode 1 (plural 0..2 graveyard cards): its own ask, offering the Bear.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 0 || d.Max != 2 {
		t.Fatalf("pending = %+v, want mode 1's 0..2 graveyard ask", d)
	}
	if d.ResumeKind != "charm_mode_seq" {
		t.Fatalf("mode 1 ask resume kind = %q, want the sequential distinct-mode ask", d.ResumeKind)
	}
	opts := charmModeOptions(d, bear)
	if len(opts) == 0 {
		t.Fatalf("mode 1 ask did not offer the graveyard Bear %d: %+v", bear, d.Options)
	}
	submitChoices(t, e, opts[0])

	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("seat 1 life = %d, want 18 (mode 0's target landed)", got)
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Bear zone = %+v, want exile (mode 1's target landed)", o)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("resolved Charm zone = %+v, want graveyard", o)
	}
	replayCheck(t, e, cfg)
}

// TestPluralCharmModeChosenFirstStillAsksBothModes flips the announcement
// order: the plural mode is chosen first. CR 608.2c/700.2a still asks and
// resolves the modes in PRINTED order, so the singleton (printed first) asks
// first even though it was named second. With only the 1..1-only gate, the
// first target-bearing mode's declaration is used for the whole flat ask, so
// the 1..1 mode would silently lose its own target. Each mode must still get
// its own ask.
func TestPluralCharmModeChosenFirstStillAsksBothModes(t *testing.T) {
	t.Parallel()
	charm := pluralCharmScript()
	e, cfg, id := newFixtureDeck(t, 6313, charm, vanillaCreatureScript, otherCreatureScript)
	bear := addToGraveyard(t, e, 0, vanillaCreatureScript)
	if other := addToGraveyard(t, e, 0, otherCreatureScript); other == 0 {
		t.Fatal("precondition: second graveyard card was not seeded")
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: seeded Bear zone = %+v, want graveyard", o)
	}
	if e.G.Players[1].Life != 20 {
		t.Fatalf("precondition: seat 1 life = %d, want 20", e.G.Players[1].Life)
	}
	addMana(t, e, 0, "B")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the cast mode announcement", d)
	}
	submitChoices(t, e, 1, 0) // answer ExileMode (plural) FIRST, then LoseMode

	// Printed order wins: LoseMode (mode 0) asks first despite being named
	// second.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 1 || d.Max != 1 {
		t.Fatalf("pending = %+v, want the singleton mode's 1..1 ask first", d)
	}
	seat1 := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			seat1 = o.Index
		}
	}
	if seat1 < 0 {
		t.Fatalf("singleton ask did not offer seat 1: %+v", d.Options)
	}
	submitChoices(t, e, seat1)

	// The plural mode must still ask: 0..2 over the graveyard Bear.
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 0 || d.Max != 2 {
		t.Fatalf("pending = %+v, want the plural mode's 0..2 ask", d)
	}
	opts := charmModeOptions(d, bear)
	if len(opts) == 0 {
		t.Fatalf("plural ask did not offer the Bear: %+v", d.Options)
	}
	submitChoices(t, e, opts[0])

	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Bear zone = %+v, want exile (plural mode landed)", o)
	}
	if got := e.G.Players[1].Life; got != 18 {
		t.Fatalf("seat 1 life = %d, want 18 (singleton mode still landed)", got)
	}
	replayCheck(t, e, cfg)
}

// TestTriggeredPluralCharmBindsBothModeTargets is the trigger-placement twin:
// a triggered Charm with one 1..1 mode and one plural mode must ask and bind
// both, not fall through to the first target-bearing mode alone.
func TestTriggeredPluralCharmBindsBothModeTargets(t *testing.T) {
	t.Parallel()
	charm := "Name:Triggered Plural Charm\nManaCost:1 B\nTypes:Creature Bear\nPT:2/2\n" +
		"T:Mode$ Phase | Phase$ BeginCombat | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigCharm\n" +
		"SVar:TrigCharm:DB$ Charm | CharmNum$ 2 | Choices$ PlayerMode,ExileMode\n" +
		"SVar:PlayerMode:DB$ LoseLife | ValidTgts$ Player | LifeAmount$ 2\n" +
		"SVar:ExileMode:DB$ ChangeZone | TargetMin$ 0 | TargetMax$ 2 | Origin$ Graveyard | Destination$ Exile | TgtPrompt$ Choose target card in a graveyard | ValidTgts$ Card\n" +
		"Oracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 6314, charm, vanillaCreatureScript, otherCreatureScript)
	bearer := putCreature(t, e, 0, charm)
	if o := e.G.Obj(bearer); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: trigger source not on battlefield: %+v", o)
	}
	bear := addToGraveyard(t, e, 0, vanillaCreatureScript)
	if other := addToGraveyard(t, e, 0, otherCreatureScript); other == 0 {
		t.Fatal("precondition: second graveyard card was not seeded")
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: seeded Bear zone = %+v, want graveyard", o)
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepBeginCombat})
	e.putTriggersOnStack()
	if len(e.G.Stack) != 1 {
		t.Fatalf("trigger stack = %v, want one ability", e.G.Stack)
	}
	id := e.G.Stack[0]
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || len(d.Options) != 2 {
		t.Fatalf("placement = %+v, want two selectable modes", d)
	}
	life := e.G.Players[1].Life
	submitChoices(t, e, 0, 1) // PlayerMode then ExileMode

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 1 || d.Max != 1 || d.ResumeKind != "charm_mode_seq" {
		t.Fatalf("pending = %+v, want the 1..1 player mode's ask", d)
	}
	seat1 := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == 1 {
			seat1 = o.Index
		}
	}
	if seat1 < 0 {
		t.Fatalf("player mode ask did not offer seat 1: %+v", d.Options)
	}
	submitChoices(t, e, seat1)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 0 || d.Max != 2 || d.ResumeKind != "charm_mode_seq" {
		t.Fatalf("pending = %+v, want the plural mode's ask", d)
	}
	opts := charmModeOptions(d, bear)
	if len(opts) == 0 {
		t.Fatalf("plural mode ask did not offer the Bear: %+v", d.Options)
	}
	submitChoices(t, e, opts[0])

	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		pd := e.Pending()
		if pd == nil {
			break
		}
		if pd.Kind != decision.KPriority {
			t.Fatalf("unexpected pending while draining: %+v", pd)
		}
		pass := -1
		for _, o := range pd.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		submitChoices(t, e, pass)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZExile {
		t.Fatalf("resolved trigger zone = %+v, want exile", o)
	}
	if got := e.G.Players[1].Life; got != life-2 {
		t.Fatalf("seat 1 life = %d, want %d (player mode landed)", got, life-2)
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Bear zone = %+v, want exile (plural mode landed)", o)
	}
	replayCheck(t, e, cfg)
}

// TestPluralCharmUnmeetableMinimumFizzles proves the new sequential shape
// keeps the infeasible arm: a triggered Charm whose plural mode has a
// mandatory minimum the board cannot meet must fizzle to exile, not pose an
// unanswerable ask or run the first mode alone.
func TestPluralCharmUnmeetableMinimumFizzles(t *testing.T) {
	t.Parallel()
	charm := "Name:Unmeetable Plural Charm\nManaCost:1 B\nTypes:Creature Bear\nPT:2/2\n" +
		"T:Mode$ Phase | Phase$ BeginCombat | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigCharm\n" +
		"SVar:TrigCharm:DB$ Charm | CharmNum$ 2 | Choices$ PlayerMode,ExileMode\n" +
		"SVar:PlayerMode:DB$ LoseLife | ValidTgts$ Player | LifeAmount$ 2\n" +
		"SVar:ExileMode:DB$ ChangeZone | TargetMin$ 2 | TargetMax$ 2 | Origin$ Graveyard | Destination$ Exile | TgtPrompt$ Choose two target cards in graveyards | ValidTgts$ Card\n" +
		"Oracle:x\n"
	e, cfg, _ := newFixtureDeck(t, 6315, charm, vanillaCreatureScript)
	bearer := putCreature(t, e, 0, charm)
	if o := e.G.Obj(bearer); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: trigger source not on battlefield: %+v", o)
	}
	// Exactly ONE card in a graveyard, but the plural mode's minimum is two.
	bear := addToGraveyard(t, e, 0, vanillaCreatureScript)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: seeded Bear zone = %+v, want graveyard", o)
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepBeginCombat})
	e.putTriggersOnStack()
	// CR 700.2a withholds the plural mode at placement, so the "choose two"
	// cannot be announced and the trigger is removed (CR 603.3c) without a
	// mode ask.
	if len(e.G.Stack) != 0 {
		t.Fatalf("trigger stack = %v, want the unannounceable ability removed", e.G.Stack)
	}
	id := triggerObjOf(t, e, bearer).ID
	if d := e.Pending(); d != nil && d.Kind == decision.KModes {
		t.Fatalf("placement posed an unannounceable mode ask: %+v", d)
	}
	// The board genuinely cannot meet the plural mode's minimum: exactly one
	// graveyard card against TargetMin$ 2.
	face := e.G.Obj(bearer).Face()
	if n := len(e.legalTargetCandidates(0, id, id, cards.ResolveSVar(face.SVars, "ExileMode"))); n != 1 {
		t.Fatalf("precondition: plural mode legal candidates = %d, want 1 (< its minimum of 2)", n)
	}
	life := e.G.Players[1].Life
	if pd := e.Pending(); pd != nil && pd.Kind == decision.KTarget {
		t.Fatalf("unanswerable target decision posed: %+v", pd)
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZExile {
		t.Fatalf("untargetable ability zone = %+v, want exile", o)
	}
	if got := e.G.Players[1].Life; got != life {
		t.Fatalf("first mode ran despite infeasible plural mode: life %d -> %d", life, got)
	}
	replayCheck(t, e, cfg)
}

// TestChooseOnePluralCharmKeepsFlatAsk guards the 33 latent choose-one cards:
// a Charm that may choose only ONE mode rides the ordinary flat ask, which is
// correct for a single target-bearing mode. The fix must not force the
// sequential per-mode path on it.
func TestChooseOnePluralCharmKeepsFlatAsk(t *testing.T) {
	t.Parallel()
	charm := "Name:Choose One Plural Charm\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ Charm | CharmNum$ 1 | Choices$ LoseMode,ExileMode\n" +
		"SVar:LoseMode:DB$ LoseLife | ValidTgts$ Player | LifeAmount$ 2 | SpellDescription$ Target player loses 2 life.\n" +
		"SVar:ExileMode:DB$ ChangeZone | TargetMin$ 0 | TargetMax$ 2 | Origin$ Graveyard | Destination$ Exile | TgtPrompt$ Choose target card in a graveyard | ValidTgts$ Card | SpellDescription$ Exile up to two target cards from graveyards.\n" +
		"Oracle:x\n"
	e, cfg, id := newFixtureDeck(t, 6316, charm, vanillaCreatureScript, otherCreatureScript)
	bear := addToGraveyard(t, e, 0, vanillaCreatureScript)
	if other := addToGraveyard(t, e, 0, otherCreatureScript); other == 0 {
		t.Fatal("precondition: second graveyard card was not seeded")
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: seeded Bear zone = %+v, want graveyard", o)
	}
	addMana(t, e, 0, "B")
	d := castFixture(t, e, id, -1)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the cast mode announcement", d)
	}
	submitChoices(t, e, 1) // ExileMode only
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the ordinary flat target ask", d)
	}
	if d.ResumeKind != "" {
		t.Fatalf("choose-one plural Charm took the sequential per-mode path (resume kind %q), want the ordinary flat ask", d.ResumeKind)
	}
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("flat ask bounds = %d..%d, want the plural mode's 0..2", d.Min, d.Max)
	}
	opts := charmModeOptions(d, bear)
	if len(opts) == 0 {
		t.Fatalf("flat ask did not offer the graveyard Bear: %+v", d.Options)
	}
	submitChoices(t, e, opts[0])
	passUntilStackEmpty(t, e, 20)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZExile {
		t.Fatalf("Bear zone = %+v, want exile", o)
	}
	if !strings.HasPrefix(e.G.Obj(id).Face().Name, "Choose One") {
		t.Fatalf("fixture face name = %q", e.G.Obj(id).Face().Name)
	}
	replayCheck(t, e, cfg)
}

// TestCharmAllModesIllegalDoesNotResolve pins CR 608.2b: each mode's target is
// judged by that mode. Here both are illegal, one gone and one no longer tapped,
// so the spell does not resolve, even though the second object still fits the
// first mode's description.
func TestCharmAllModesIllegalDoesNotResolve(t *testing.T) {
	t.Parallel()
	const charm = "Name:Two Blasts\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ Charm | CharmNum$ 2 | Choices$ KillAny,KillTapped\n" +
		"SVar:KillAny:DB$ Destroy | ValidTgts$ Creature | SpellDescription$ Destroy target creature.\n" +
		"SVar:KillTapped:DB$ Destroy | ValidTgts$ Creature.tapped | SpellDescription$ Destroy target tapped creature.\n" +
		"Oracle:x\n"
	e, _, id := newFixtureDeck(t, 6311, charm)
	first := onBoard(t, e, 1, vanillaCreatureScript)
	second := onBoard(t, e, 1, otherCreatureScript)
	e.emit(events.Event{Kind: events.Tap, Obj: second})
	addMana(t, e, 0, "B")
	if d := castFixture(t, e, id, -1); d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the mode announcement", d)
	}
	submitChoices(t, e, 0, 1)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Max != 2 {
		t.Fatalf("pending = %+v, want the combined target ask", d)
	}
	asSecond := charmModeOptions(d, second)
	submitChoices(t, e, charmModeOptions(d, first)[0], asSecond[len(asSecond)-1])
	if e.G.Obj(id).Zone != state.ZStack {
		t.Fatalf("the charm is not on the stack: %+v", e.Pending())
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: first, From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.Untap, Obj: second})
	before := len(e.L.Events)
	passUntilStackEmpty(t, e, 20)
	for _, ev := range e.L.Events[before:] {
		if ev.Kind == events.Resolve && ev.Obj == id {
			t.Fatal("the charm resolved with every target illegal")
		}
	}
	if z := e.G.Obj(second).Zone; z != state.ZBattlefield {
		t.Fatalf("second creature zone %s, want it on the battlefield", z)
	}
}
