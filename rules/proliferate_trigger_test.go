package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// trig:Proliferate ("Whenever you proliferate, ..." -- CR 701.27, task
// trig-proliferate) fires once per completed proliferate action on the
// events.Proliferate marker api:Proliferate emits, never once per recipient
// or counter, and never for an ordinary counter addition (which carries no
// marker). Pinned on the real corpus carrier Scheming Aspirant
// ("Whenever you proliferate, each opponent loses 2 life and you gain 2
// life"): its life swing makes both the firing and the granularity directly
// observable, and its face carries no other trigger to muddy the count.

// countProliferateMarkers counts the completed-action markers in the log.
func countProliferateMarkers(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Proliferate && ev.Player == 0 {
			n++
		}
	}
	return n
}

// aspirantWithProliferateTrigger asserts the precondition the trigger tests
// hang on: the carrier is on seat 0's battlefield and its face really carries
// a Mode$ Proliferate trigger, so a no-fire below is meaningful and not a
// vacuously-absent trigger.
func aspirantWithProliferateTrigger(t *testing.T, e *Engine, aspirant state.ObjID) {
	t.Helper()
	o := e.G.Obj(aspirant)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Scheming Aspirant id %d not on the battlefield", aspirant)
	}
	found := false
	for _, trig := range o.Face().Triggers {
		if trig.Mode == "Proliferate" {
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: Scheming Aspirant has no Mode$ Proliferate trigger on its face")
	}
}

// TestProliferateTriggerFiresOncePerAction is the headline: Tezzeret's Gambit
// resolves, the answered proliferate picks TWO recipients (a permanent and a
// player), and the trigger fires exactly ONCE -- one marker, one life swing
// of 2, not one per recipient and not one per counter.
func TestProliferateTriggerFiresOncePerAction(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Scheming Aspirant", "Tezzeret's Gambit", "Grizzly Bears")
	aspirant := searchMoveByName(t, e, "Scheming Aspirant", state.ZBattlefield)
	aspirantWithProliferateTrigger(t, e, aspirant)
	putNamedOnBattlefield(t, e, "Grizzly Bears")
	carrier := putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 2)
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: 1})
	e.priorityRound()

	// Preconditions the assertions depend on: the recipients carry counters
	// (so the proliferate batch is non-empty) and the lives under comparison
	// actually differ from their post-trigger values.
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 2 {
		t.Fatalf("precondition: carrier P1P1 = %d, want 2", got)
	}
	if got := e.G.Players[0].Counter("POISON"); got != 1 {
		t.Fatalf("precondition: seat 0 POISON = %d, want 1", got)
	}
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
	if life0+2 == life0 || life1-2 == life1 {
		t.Fatal("precondition: a 2-point life swing would be unobservable")
	}

	gambit := findAndMoveToHand(t, e, 0, "Tezzeret's Gambit")
	addMana(t, e, 0, "UUUU")
	castFromPriority(t, e, gambit)
	answerPips(t, e)
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
		t.Fatalf("decision = %+v, want the proliferate ask", d)
	}
	carrierOpt, playerOpt := -1, -1
	for i, o := range d.Options {
		if o.Obj == carrier {
			carrierOpt = i
		}
		if o.Obj == 0 && o.Player == 0 {
			playerOpt = i
		}
	}
	if carrierOpt < 0 || playerOpt < 0 {
		t.Fatalf("proliferate recipients not offered: %+v", d.Options)
	}
	answerProliferate(t, e, d, carrierOpt, playerOpt)

	// The proliferate handler ran: both recipients took their +1.
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 3 {
		t.Fatalf("carrier P1P1 = %d, want 3 (+1)", got)
	}
	if got := e.G.Players[0].Counter("POISON"); got != 2 {
		t.Fatalf("player POISON = %d, want 2 (+1)", got)
	}
	// ONE completed-action marker for the whole two-recipient action.
	if n := countProliferateMarkers(e); n != 1 {
		t.Fatalf("logged %d Proliferate markers, want 1 (one trigger per ACTION, not per recipient)", n)
	}
	// The trigger fired exactly once: one 2-point swing, not one per recipient.
	if got := e.G.Players[0].Life; got != life0+2 {
		t.Fatalf("seat 0 life = %d, want %d (trigger fired exactly once)", got, life0+2)
	}
	if got := e.G.Players[1].Life; got != life1-2 {
		t.Fatalf("seat 1 life = %d, want %d (trigger fired exactly once)", got, life1-2)
	}
	replayCheck(t, e, cfg)
}

// TestProliferateTriggerOncePerActionAtAmount2 pins the amount axis of the
// same granularity: Contagion Engine's Amount$ 2 puts +2 of each kind in one
// batch as ONE proliferate action -- one marker, one trigger, one 2-point
// swing (a per-proliferation reading would swing 4).
func TestProliferateTriggerOncePerActionAtAmount2(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Scheming Aspirant", "Contagion Engine", "Grizzly Bears")
	aspirant := searchMoveByName(t, e, "Scheming Aspirant", state.ZBattlefield)
	aspirantWithProliferateTrigger(t, e, aspirant)
	engineID := searchMoveByName(t, e, "Contagion Engine", state.ZBattlefield)
	putNamedOnBattlefield(t, e, "Grizzly Bears")
	carrier := putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 1)
	e.priorityRound()
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 1 {
		t.Fatalf("precondition: carrier P1P1 = %d, want 1", got)
	}
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life

	addMana(t, e, 0, "CCCCCC")
	opt, ok := findAbilityOption(e, engineID, 0)
	if !ok {
		t.Fatalf("Contagion Engine proliferate ability not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt.Index)
	d := passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
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

	// The handler ran at Amount$ 2 (+2, not +1 or +4).
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 3 {
		t.Fatalf("carrier P1P1 = %d, want 3 (+2)", got)
	}
	if n := countProliferateMarkers(e); n != 1 {
		t.Fatalf("logged %d Proliferate markers, want 1 (one trigger per ACTION, not per amount step)", n)
	}
	if got := e.G.Players[0].Life; got != life0+2 {
		t.Fatalf("seat 0 life = %d, want %d (one trigger for the Amount$ 2 action)", got, life0+2)
	}
	if got := e.G.Players[1].Life; got != life1-2 {
		t.Fatalf("seat 1 life = %d, want %d (one trigger for the Amount$ 2 action)", got, life1-2)
	}
	replayCheck(t, e, cfg)
}

// TestProliferateTriggerFiresWithNoEligibleRecipient: a proliferate with
// nothing carrying a counter still HAPPENED (the action completed, silently,
// per the OnlyEmptyAnswer discipline), so the marker is recorded and the
// trigger fires -- the reason the signal is a marker and not the
// CounterChange events.
func TestProliferateTriggerFiresWithNoEligibleRecipient(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Scheming Aspirant", "Tezzeret's Gambit", "Grizzly Bears")
	aspirant := searchMoveByName(t, e, "Scheming Aspirant", state.ZBattlefield)
	aspirantWithProliferateTrigger(t, e, aspirant)
	e.priorityRound()
	// Precondition: nothing on the battlefield or any player carries a
	// counter, so the resolution below takes the silent zero-eligible path.
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if o := e.G.Obj(id); o != nil && len(o.Counters) > 0 {
				t.Fatalf("precondition: %s carries counters; the zero-eligible path is not exercised", o.Face().Name)
			}
		}
		if len(e.G.Players[p].Counters) > 0 {
			t.Fatalf("precondition: player %d carries counters", p)
		}
	}
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life

	gambit := findAndMoveToHand(t, e, 0, "Tezzeret's Gambit")
	addMana(t, e, 0, "UUUU")
	castFromPriority(t, e, gambit)
	answerPips(t, e)
	d := passUntilNonPriority(t, e, 60)
	if d != nil && d.Kind == decision.KChoose && d.ResumeKind == "proliferate" {
		t.Fatalf("zero-eligible proliferate posed an ask: %+v", d)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.Note && ev.Text == "unimplemented API Proliferate" {
			t.Fatal("proliferate still hit the unimplemented fallback")
		}
	}
	// The silent action is still a completed proliferate: one marker, one
	// trigger, one life swing.
	if n := countProliferateMarkers(e); n != 1 {
		t.Fatalf("logged %d Proliferate markers, want 1 (the action happened with no eligible recipient)", n)
	}
	if got := e.G.Players[0].Life; got != life0+2 {
		t.Fatalf("seat 0 life = %d, want %d (trigger fired on the recipient-less action)", got, life0+2)
	}
	if got := e.G.Players[1].Life; got != life1-2 {
		t.Fatalf("seat 1 life = %d, want %d (trigger fired on the recipient-less action)", got, life1-2)
	}
	replayCheck(t, e, cfg)
}

// TestNoProliferateTriggerOnOrdinaryCounterAddition: a plain CounterChange
// (no api:Proliferate anywhere) carries no marker, so the trigger must stay
// silent even with the carrier sitting on the battlefield.
func TestNoProliferateTriggerOnOrdinaryCounterAddition(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Scheming Aspirant", "Grizzly Bears")
	aspirant := searchMoveByName(t, e, "Scheming Aspirant", state.ZBattlefield)
	aspirantWithProliferateTrigger(t, e, aspirant)
	putNamedOnBattlefield(t, e, "Grizzly Bears")
	carrier := putCountersOn(t, e, 0, "Grizzly Bears", "P1P1", 2)
	e.priorityRound()
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life

	// The handler ran: an ordinary CounterChange really landed on the carrier.
	if got := e.G.Obj(carrier).Counter("P1P1"); got != 2 {
		t.Fatalf("precondition: carrier P1P1 = %d, want 2", got)
	}
	if n := countProliferateMarkers(e); n != 0 {
		t.Fatalf("logged %d Proliferate markers, want 0 (no proliferate action was taken)", n)
	}
	if got := e.G.Players[0].Life; got != life0 {
		t.Fatalf("seat 0 life = %d, want %d (an ordinary counter add must not fire the trigger)", got, life0)
	}
	if got := e.G.Players[1].Life; got != life1 {
		t.Fatalf("seat 1 life = %d, want %d (an ordinary counter add must not fire the trigger)", got, life1)
	}
	replayCheck(t, e, cfg)
}
