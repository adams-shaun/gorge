package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edh_trigf_test.go plays Hibernation's End's printed Mode$ PayCumulativeUpkeep
// line through a real game (ticket edh-trigF): the payment-settled marker the
// cumulative-upkeep window emits queues the trigger with the paid age as its
// amount, and the sacrifice branch queues nothing.

// trigFCub is a mana-value-1 fixture creature the search body can find at age
// 1 (the body's ChangeType$ Creature.cmcEQX).
const trigFCub = "Name:Testcub\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nOracle:x\n"

// trigFEndPrecondition asserts the corpus carries the printed line.
func trigFEndPrecondition(t *testing.T, reg *cards.Registry) {
	t.Helper()
	end := searchCorpusCard(t, reg, "Hibernation's End")
	found := false
	for _, tr := range end.Faces[0].Triggers {
		if tr.Mode == "PayCumulativeUpkeep" {
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: Hibernation's End has no printed PayCumulativeUpkeep line")
	}
}

// trigFEngine seeds seat 0's library with the REAL corpus Hibernation's End
// plus the MV-1 fixture cub, returns the engine and Hibernation's End's
// battlefield id.
func trigFEngine(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	trigFEndPrecondition(t, reg)
	end, ok := reg.Lookup("Hibernation's End")
	if !ok {
		t.Fatal("corpus fixture: Hibernation's End missing")
	}
	cub := card(t, trigFCub)
	e := New(Config{Seed: 7, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{append([]*cards.Card{end, cub}, mountainDeck(t, 38)...), mountainDeck(t, 40)}})
	// Genesis dealt seat 0's hand from the deck order, so the End is in hand
	// (moveByName's first search) and the cub behind it. No Advance: the test
	// drives the upkeep's own resolution as a kernel probe, which needs no
	// posed decision (a turn loop would leave a priority one up), so the
	// game stands at turn 1, seat 0, Main1 like handEngine leaves it.
	e.G.Step = state.StepMain1
	e.G.Active, e.G.Priority = 0, 0
	e.G.Turn = 1
	he := moveByName(t, e, 0, "Hibernation's End", state.ZBattlefield)
	// The search body reads Origin$ Library: make sure the cub is there.
	inLib := false
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Testcub" {
			inLib = true
		}
	}
	if !inLib {
		moveByName(t, e, 0, "Testcub", state.ZLibrary)
	}
	// The {1} age cost's pool mana, emitted directly.
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "C", Amount: 1})
	return e, he
}

// trigFUpkeepAsk drives one beginning-of-upkeep StepChange, places the queued
// cumulative-upkeep trigger on the stack and resolves it, leaving the engine
// at the pay/sacrifice ask.
func trigFUpkeepAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	e.G.Active, e.G.Priority = 0, 0
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	e.putTriggersOnStack()
	found := false
	for _, sid := range e.G.Stack {
		if o := e.G.Obj(sid); o != nil && o.Ability != nil && o.Ability.API == "CumulativeUpkeep" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cumulative upkeep was not placed as a triggered ability: stack=%v", e.G.Stack)
	}
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("no cumulative pay/sacrifice ask: %+v", d)
	}
	return d
}

// TestEDHTrigFHibernationsEndCumulative plays the printed line: paying the
// upkeep's mana fires the trigger, whose optional search body puts the MV-1
// cub on the battlefield; the sacrifice branch fires nothing.
func TestEDHTrigFHibernationsEndCumulative(t *testing.T) {
	t.Parallel()
	t.Run("pay fires with the paid age", func(t *testing.T) {
		e, he := trigFEngine(t)
		if got := e.G.Obj(he).Counter("AGE"); got != 0 {
			t.Fatalf("precondition: age counters %d, want 0", got)
		}
		d := trigFUpkeepAsk(t, e)
		pay := -1
		for _, o := range d.Options {
			if o.Kind == "cumulative_pay" {
				pay = o.Index
			}
		}
		if pay < 0 {
			t.Fatalf("the pay option is not offered: %+v", d.Options)
		}
		submitChoices(t, e, pay)
		// The payment settled; the marker queued Hibernation's End's trigger
		// with the paid age as its amount.
		if len(e.pendingTriggers) == 0 {
			t.Fatal("the paid upkeep queued no trigger")
		}
		for _, pt := range e.pendingTriggers {
			if pt.Source != he {
				t.Fatalf("queued a trigger from %d, want Hibernation's End %d", pt.Source, he)
			}
			if amt := pt.Ctx.TriggerContext.TriggerAmount; amt != 1 {
				t.Fatalf("trigger amount %d, want the paid age 1", amt)
			}
		}
		// The turn loop's next round would drain the queue; the test drives
		// the drain itself (the answer above was served inline).
		e.putTriggersOnStack()
		// Drain through the optional ask and the search body's resolution:
		// take the "you may" election, pick the cub (MV 1 = the age paid),
		// and let the resolution end.
		sawOptional := false
		for i := 0; i < 80; i++ {
			d := e.Pending()
			if d == nil {
				if len(e.G.Stack) == 0 {
					break
				}
				e.resolveTop()
				continue
			}
			switch {
			case d.Kind == decision.KPriority:
				for _, o := range d.Options {
					if o.Kind == "pass" {
						submitChoices(t, e, o.Index)
					}
				}
			case d.Kind == decision.KTriggerOrder:
				choices := make([]int, 0, len(d.Options))
				for _, o := range d.Options {
					choices = append(choices, o.Index)
				}
				submitChoices(t, e, choices...)
			case d.Kind == decision.KTriggerOptional:
				sawOptional = true
				if len(d.Options) < 2 || d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
					t.Fatalf("unexpected optional ask: %+v", d.Options)
				}
				submitChoices(t, e, d.Options[0].Index)
			case d.Kind == decision.KChoose && d.ResumeKind == "search":
				if !sawOptional {
					t.Fatal("the search ask surfaced before the optional election")
				}
				cub := -1
				for _, o := range d.Options {
					if o2 := e.G.Obj(o.Obj); o2 != nil && o2.Face() != nil && o2.Face().Name == "Testcub" {
						cub = o.Index
					}
				}
				if cub < 0 {
					t.Fatalf("the search ask offered no Testcub: %+v", d.Options)
				}
				submitChoices(t, e, cub)
			default:
				t.Fatalf("unexpected ask %v in the trigger's resolution: %+v", d.Kind, d)
			}
		}
		if !sawOptional {
			t.Fatal("the trigger resolved without posing its optional election")
		}
		onBoard := false
		for _, id := range e.G.Zone(state.ZBattlefield, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Testcub" {
				onBoard = true
			}
		}
		if !onBoard {
			t.Fatal("the paid upkeep's search did not put the cub on the battlefield")
		}
	})

	t.Run("sacrifice fires nothing", func(t *testing.T) {
		e, he := trigFEngine(t)
		d := trigFUpkeepAsk(t, e)
		sac := -1
		for _, o := range d.Options {
			if o.Kind == "cumulative_sac" {
				sac = o.Index
			}
		}
		if sac < 0 {
			t.Fatalf("the sacrifice option is not offered: %+v", d.Options)
		}
		submitChoices(t, e, sac)
		if len(e.pendingTriggers) != 0 {
			t.Fatalf("the sacrifice branch queued %d triggers, want none: %+v",
				len(e.pendingTriggers), e.pendingTriggers)
		}
		if o := e.G.Obj(he); o == nil || o.Zone != state.ZGraveyard {
			t.Fatalf("the sacrifice branch did not sacrifice the End: %+v", o)
		}
	})
}
