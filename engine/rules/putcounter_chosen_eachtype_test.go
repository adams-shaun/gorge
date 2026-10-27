package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// PutCounter's CounterTypes$ ChosenFromList (with TypeList$) and
// EachType_<spec> entries (cr-g225): Elspeth Resplendent's +1 puts a +1/+1
// counter and a keyword counter chosen from four; Exotic Pets puts one
// counter of each kind among your creatures on either of its two Fish.

func optionLabels(d *decision.Decision) []string {
	var out []string
	for _, o := range d.Options {
		out = append(out, o.Label)
	}
	return out
}

// passUntilCounterAsk passes priority until a non-priority ask is pending or the stack
// (objects and abilities alike) is empty, and returns the pending decision.
func passUntilCounterAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 20; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority || len(e.G.Stack) == 0 {
			return d
		}
		passPriorityOnce(t, e)
	}
	t.Fatal("stack did not drain")
	return nil
}

func noPutCounterSkipNote(t *testing.T, e *Engine) {
	t.Helper()
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && strings.Contains(ev.Text, "PutCounter CounterTypes$") {
			t.Fatalf("a CounterTypes$ entry was skipped: %q", ev.Text)
		}
	}
}

func TestElspethResplendentPlusOneAddsChosenKeywordCounter(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	elspeth := mustCorpusCard(t, reg, "Elspeth Resplendent")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	plusOne := jaceAbility(t, elspeth, "AddCounter", 1)
	e, cfg, walker := walkerBoard(t, reg, "Elspeth Resplendent", bear)
	e.Advance()
	bears := findBattlefieldCard(t, e, "Grizzly Bears")
	if got := counterOn(t, e, bears, "P1P1"); got != 0 {
		t.Fatalf("precondition: Bears already carry %d P1P1", got)
	}
	opt := abilityOption(t, e, walker, plusOne)
	submitChoices(t, e, opt.Index)
	targetObject(t, e, bears)
	d := passUntilCounterAsk(t, e)
	if d.Kind != decision.KChoose || d.ResumeKind != "counter_kind" {
		t.Fatalf("pending = %+v, want the counter_kind ask", d)
	}
	want := []string{"Flying", "First Strike", "Lifelink", "Vigilance"}
	if got := optionLabels(d); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("kind options = %v, want %v", got, want)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Label == "Lifelink" {
			pick = o.Index
		}
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 20)
	if got := counterOn(t, e, bears, "P1P1"); got != 1 {
		t.Fatalf("Bears P1P1 = %d, want 1", got)
	}
	if got := counterOn(t, e, bears, "Lifelink"); got != 1 {
		t.Fatalf("Bears Lifelink counters = %d, want 1", got)
	}
	if got := counterOn(t, e, bears, "Flying"); got != 0 {
		t.Fatalf("Bears Flying counters = %d, want 0 (only the chosen kind lands)", got)
	}
	if !e.HasKeyword(bears, "Lifelink") {
		t.Fatalf("Bears lack Lifelink despite the lifelink counter")
	}
	noPutCounterSkipNote(t, e)
	replayCheck(t, e, cfg)
}

// exoticPetsBoard seats Grizzly Bears carrying a P1P1 and a Flying counter and
// casts Exotic Pets; it returns the engine, the Bears and the first pending
// ask.
func exoticPetsBoard(t *testing.T, withCounters bool) (*Engine, Config, state.ObjID, *decision.Decision) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	pets := mustCorpusCard(t, reg, "Exotic Pets")
	e, cfg := tokenReplGame(t, 5, bear, pets)
	bears := moveSeededCard(t, e, 0, bear, state.ZBattlefield)
	if withCounters {
		e.emit(events.Event{Kind: events.CounterChange, Obj: bears, Counter: "P1P1", Amount: 1})
		e.emit(events.Event{Kind: events.CounterChange, Obj: bears, Counter: "Flying", Amount: 1})
	}
	d := castSpellNamed(t, e, reg, "Exotic Pets", "1WU")
	return e, cfg, bears, d
}

func fishTokens(t *testing.T, e *Engine) []state.ObjID {
	t.Helper()
	var out []state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Fish Token" {
			out = append(out, id)
		}
	}
	if len(out) != 2 {
		t.Fatalf("Fish tokens = %d, want 2", len(out))
	}
	return out
}

// answerPick submits the counter_pick ask's option for obj.
func answerPick(t *testing.T, e *Engine, d *decision.Decision, obj state.ObjID) {
	t.Helper()
	if d.Kind != decision.KChoose || d.ResumeKind != "counter_pick" {
		t.Fatalf("pending = %+v, want a counter_pick ask", d)
	}
	for _, o := range d.Options {
		if o.Obj == obj {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("object %d not offered: %+v", obj, d.Options)
}

func TestExoticPetsCopiesEachCounterKind(t *testing.T) {
	for _, split := range []bool{false, true} {
		name := "both on the first Fish"
		if split {
			name = "split across the Fish"
		}
		t.Run(name, func(t *testing.T) {
			e, cfg, bears, d := exoticPetsBoard(t, true)
			if got := counterOn(t, e, bears, "P1P1"); got != 1 {
				t.Fatalf("precondition: Bears P1P1 = %d, want 1", got)
			}
			fish := fishTokens(t, e)
			if len(d.Options) != 2 {
				t.Fatalf("first ask offers %d objects, want the two Fish: %+v", len(d.Options), d)
			}
			answerPick(t, e, d, fish[0])
			d = e.Pending()
			if d.ResumeKind != "counter_pick" {
				t.Fatalf("second ask = %+v, want a second counter_pick (one per kind)", d)
			}
			second := fish[0]
			if split {
				second = fish[1]
			}
			answerPick(t, e, d, second)
			passUntilStackEmpty(t, e, 20)
			// Kinds are gathered in the Bears' counter order: P1P1, then Flying.
			if got := counterOn(t, e, fish[0], "P1P1"); got != 1 {
				t.Fatalf("first Fish P1P1 = %d, want 1", got)
			}
			wantFlying0, wantFlying1 := int32(1), int32(0)
			if split {
				wantFlying0, wantFlying1 = 0, 1
			}
			if got := counterOn(t, e, fish[0], "Flying"); got != wantFlying0 {
				t.Fatalf("first Fish Flying = %d, want %d", got, wantFlying0)
			}
			if got := counterOn(t, e, fish[1], "Flying"); got != wantFlying1 {
				t.Fatalf("second Fish Flying = %d, want %d", got, wantFlying1)
			}
			if got := counterOn(t, e, fish[1], "P1P1"); got != 0 {
				t.Fatalf("second Fish P1P1 = %d, want 0", got)
			}
			noPutCounterSkipNote(t, e)
			replayCheck(t, e, cfg)
		})
	}
}

func TestExoticPetsNoCountersNoAsk(t *testing.T) {
	e, cfg, _, d := exoticPetsBoard(t, false)
	if d.ResumeKind == "counter_pick" {
		t.Fatalf("a counter_pick was posed with no counters among your creatures: %+v", d)
	}
	for _, id := range fishTokens(t, e) {
		if o := e.G.Obj(id); len(o.Counters) != 0 {
			t.Fatalf("Fish carries counters %v, want none", o.Counters)
		}
	}
	replayCheck(t, e, cfg)
}
