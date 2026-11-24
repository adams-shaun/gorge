package rules

// Nimble Hobbit is the real carrier of a CONTROLLER GenericChoice whose every
// Choices$ body carries UnlessCost$: "Whenever Nimble Hobbit attacks, you may
// sacrifice a Food or pay {2}{W}. When you do, tap target creature an
// opponent controls." Forge's ChooseGenericEffect drops each choice the
// chooser cannot pay and, with the list empty, runs FallbackAbility$ (absent
// here) instead of asking. The trigger placement ask (askTriggerModes) and
// effCharm's resolution ask must do the same: an unpayable mode is never
// offered, and when none is payable the trigger resolves with no mode ask at
// all.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// nimbleHobbitAttacks puts Nimble Hobbit (seat 0) and a Grizzly Bears (seat 1)
// on the battlefield with no lands in play and declares the Hobbit attacking.
// withFood mints one Food token under seat 0 first, so the Sac<1/Food> mode is
// payable. Seat 1's Bears is the "target creature an opponent controls" the
// Hobbit's mode would tap.
func nimbleHobbitAttacks(t *testing.T, withFood bool) (*Engine, Config, state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	hobbit := mustCorpusCard(t, reg, "Nimble Hobbit")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	cfg := seatZeroStart(Config{Seed: 2413, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{
			append([]*cards.Card{hobbit}, mountainDeck(t, 39)...),
			append([]*cards.Card{bear}, mountainDeck(t, 39)...),
		}})
	e := New(cfg)
	e.Advance()
	hobbitObj := placeInDeck(t, e, 0, hobbit, state.ZBattlefield)
	bearObj := placeInDeck(t, e, 1, bear, state.ZBattlefield)
	// SetupSettled is the logged "continuously controlled since before the
	// scenario" marker, so the replay rebuilds the same unsick Hobbit.
	e.emit(events.Event{Kind: events.SetupSettled, Obj: hobbitObj})
	if withFood {
		mintToken(t, e, 0, "c_a_food_sac")
	}

	// PRECONDITIONS the assertions below depend on: the Hobbit is a real
	// battlefield permanent seat 0 can attack with, seat 1's Bears is a
	// legal untapped target, seat 0 controls no land (no {2}{W}), and the
	// Food count is exactly what the arm asked for.
	if o := e.G.Obj(hobbitObj); o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Hobbit zone/controller = %s/%d, want Battlefield/0", o.Zone, o.Controller)
	}
	if o := e.G.Obj(bearObj); o.Zone != state.ZBattlefield || o.Controller != 1 || o.Tapped {
		t.Fatalf("precondition: Bears zone/controller/tapped = %s/%d/%v, want Battlefield/1/false", o.Zone, o.Controller, o.Tapped)
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face().HasTypeWord("Land") {
			t.Fatalf("precondition: seat 0 controls land %q, want no lands (the {2}{W} mode must be unpayable)", o.Face().Name)
		}
	}
	if got, want := countTokensNamedOnSeat(t, e, 0, "Food Token"), map[bool]int{false: 0, true: 1}[withFood]; got != want {
		t.Fatalf("precondition: seat 0 Food tokens = %d, want %d", got, want)
	}

	e.G.Active = 0
	e.G.Priority = 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	e.askAttackers()
	submitAttackersOnly(t, e, hobbitObj)
	return e, cfg, hobbitObj, bearObj
}

// drainUntilNoModes answers only priority while the attack trigger is still
// on the stack or pending, and returns whether any KModes ask appeared. It
// mirrors TestPerforatingArtistNoPayableChoiceRunsFallback's drain: a
// non-priority decision other than the wanted KModes is a fixture bug, not a
// silent pass.
func drainUntilNoModes(t *testing.T, e *Engine, limit int) bool {
	t.Helper()
	for i := 0; i < limit; i++ {
		d := e.Pending()
		if d == nil || (len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0) {
			return false
		}
		if d.Kind == decision.KModes {
			return true
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %+v while draining the unpayable GenericChoice", d)
		}
		submitPass(t, e)
	}
	t.Fatalf("the attack trigger did not finish within %d priority passes", limit)
	return false
}

// TestNimbleHobbitUnpayableChoicesAskNothing pins the bug: with neither choice
// payable, the attack trigger poses no modes ask and no unless-pay ask, and
// its tap never happens. With one Food and still no mana, only the sacrifice
// choice is offered; the {2}{W} mode is dropped.
func TestNimbleHobbitUnpayableChoicesAskNothing(t *testing.T) {
	t.Parallel()

	// Arm 1: no Food, no mana -- both choices unpayable, so nothing is asked.
	e, cfg, hobbitObj, bearObj := nimbleHobbitAttacks(t, false)
	if !hasTriggerPush(e, hobbitObj) {
		t.Fatal("precondition: Nimble Hobbit's attack trigger was never pushed")
	}
	if drainUntilNoModes(t, e, 80) {
		t.Fatalf("posed a modes ask although neither UnlessCost$ is payable: %+v", e.Pending())
	}
	passUntilStackEmpty(t, e, 80)
	for _, ev := range e.L.Events {
		if ev.Kind == events.ModeChosen {
			t.Fatalf("ModeChosen recorded for unpayable choices: player %d text %q", ev.Player, ev.Text)
		}
	}
	if o := e.G.Obj(bearObj); o.Tapped {
		t.Fatal("the opponent's Bears was tapped, but both choices were unpayable")
	}
	replayCheck(t, e, cfg)

	// Arm 2: one Food, still no mana -- the lone payable choice is offered.
	e2, cfg2, hobbit2, bear2 := nimbleHobbitAttacks(t, true)
	if !hasTriggerPush(e2, hobbit2) {
		t.Fatal("precondition: Nimble Hobbit's attack trigger was never pushed (Food arm)")
	}
	d := passUntilAskKind(t, e2, decision.KModes, 80)
	if d.Player != 0 {
		t.Fatalf("mode chooser = seat %d, want the trigger controller seat 0", d.Player)
	}
	if d.ResumeKind != "modes" {
		t.Fatalf("mode ask ResumeKind = %q, want the placement ask %q", d.ResumeKind, "modes")
	}
	if len(d.Options) != 1 {
		t.Fatalf("mode options = %d, want the one payable Sac<1/Food> choice", len(d.Options))
	}
	if label := d.Options[0].Label; !strings.Contains(label, "sacrifice a Food") {
		t.Fatalf("lone offered mode = %q, want the sacrifice-a-Food choice", label)
	}
	submitChoices(t, e2, d.Options[0].Index)
	d = passUntilAskKind(t, e2, decision.KModes, 80)
	if d.ResumeKind != "unless_pay" {
		t.Fatalf("after choosing the Food mode, next KModes = %q, want unless_pay", d.ResumeKind)
	}
	submitChoices(t, e2, d.Options[len(d.Options)-1].Index)
	passUntilStackEmpty(t, e2, 80)
	if o := e2.G.Obj(bear2); o.Tapped {
		t.Fatal("declining the Food unless-pay tapped the Bears")
	}
	replayCheck(t, e2, cfg2)
}

// hasTriggerPush reports whether the source's attack trigger was put on the
// stack (events.TriggerPush names the source in Obj).
func hasTriggerPush(e *Engine, source state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush && ev.Obj == source {
			return true
		}
	}
	return false
}

// genericChoiceControllerFallbackBody is the controller/FallbackAbility$ arm:
// every choice carries an UnlessCost$ the controller cannot pay, and
// FallbackAbility$ loses 1 life instead of asking.
const genericChoiceControllerFallbackBody = "Name:Trial of Cost\nManaCost:R\nTypes:Enchantment\n" +
	"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ TrigYou | TriggerDescription$ At the beginning of your upkeep, choose.\n" +
	"SVar:TrigYou:DB$ GenericChoice | Defined$ You | Choices$ PayA,PayB | FallbackAbility$ Fallback | SpellDescription$ You choose.\n" +
	"SVar:PayA:DB$ GainLife | Defined$ You | LifeAmount$ 5 | UnlessCost$ 2 W | UnlessPayer$ You | UnlessSwitched$ True | SpellDescription$ A\n" +
	"SVar:PayB:DB$ GainLife | Defined$ You | LifeAmount$ 5 | UnlessCost$ Sac<1/Food> | UnlessPayer$ You | UnlessSwitched$ True | SpellDescription$ B\n" +
	"SVar:Fallback:DB$ LoseLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ F\n" +
	"Oracle:x\n"

// TestGenericChoiceControllerUnpayableRunsFallback pins the resolution half
// for the controller ask: with no choice payable, Forge runs FallbackAbility$
// and poses no mode decision.
func TestGenericChoiceControllerUnpayableRunsFallback(t *testing.T) {
	t.Parallel()
	trial := card(t, genericChoiceControllerFallbackBody)
	cfg := seatZeroStart(Config{Seed: 2414, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{trial}, mountainDeck(t, 39)...),
			mountainDeck(t, 40),
		}})
	e := New(cfg)
	e.Advance()
	id := placeInDeck(t, e, 0, trial, state.ZBattlefield)
	if z := e.G.Obj(id).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Trial of Cost zone = %s, want Battlefield", z)
	}
	for _, bid := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(bid); o != nil && o.Face().HasTypeWord("Land") {
			t.Fatalf("precondition: seat 0 controls land %q, want no lands", o.Face().Name)
		}
	}
	life := e.G.Players[0].Life
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepUpkeep)
	if drainUntilNoModes(t, e, 80) {
		t.Fatalf("posed a modes ask although neither UnlessCost$ is payable: %+v", e.Pending())
	}
	passUntilStackEmpty(t, e, 80)
	for _, ev := range e.L.Events {
		if ev.Kind == events.ModeChosen {
			t.Fatalf("ModeChosen recorded for unpayable choices: player %d text %q", ev.Player, ev.Text)
		}
	}
	if got := e.G.Players[0].Life; got != life-1 {
		t.Fatalf("controller life = %d, want %d (FallbackAbility$ ran once)", got, life-1)
	}
	replayCheck(t, e, cfg)
}
