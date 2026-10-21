package rules

// Valiant$ True on Mode$ BecomesTarget (the BLB ability word, CR 603.2's
// "for the first time each turn" family): Seedglaive Mentor
// (`.cards/cardsfolder/s/seedglaive_mentor.txt`, the measured carrier from
// four-yay's kernel B7 report). The gate is
// trigmatch.valiantFirstTimeTargetedByYouThisTurn: the once-each-turn count
// is per CONTROLLER -- only targetings of a spell or ability the trigger's
// controller controls count, so an opponent's targeting neither fires nor
// disarms (XMage's ValiantTriggeredAbility, Forge's Card.isValiant).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestValiantFiresOncePerTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	trig := corpusTriggerMode(t, mustCorpusCard(t, reg, "Seedglaive Mentor"), "BecomesTarget")
	if trig.Params["Valiant"] != "True" {
		t.Fatalf("corpus fixture drift: Seedglaive Mentor Valiant$ = %q, want True", trig.Params["Valiant"])
	}
	if trig.Params["ValidSource"] != "SpellAbility.YouCtrl" {
		t.Fatalf("corpus fixture drift: Seedglaive Mentor ValidSource$ = %q, want SpellAbility.YouCtrl", trig.Params["ValidSource"])
	}
	e, ids := becomesTargetOnceBoard(t, reg,
		[]string{"Seedglaive Mentor", "Prodigal Pyromancer", "Prodigal Pyromancer"},
		[]string{"Prodigal Pyromancer"})
	mentor := ids["Seedglaive Mentor"]
	if o := e.G.Obj(mentor); o == nil || o.Zone != state.ZBattlefield || targetCounterN(o, "P1P1") != 0 {
		t.Fatalf("precondition: Seedglaive Mentor not a counterless battlefield permanent: %+v", o)
	}
	// The board holds the mentor and two your-side Pyromancers; seat 1 holds
	// the opponent's.
	yours, theirs := []state.ObjID{}, state.ObjID(0)
	for _, id := range append(e.G.Zone(state.ZBattlefield, 0), e.G.Zone(state.ZBattlefield, 1)...) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != "Prodigal Pyromancer" {
			continue
		}
		if o.Controller == 0 {
			yours = append(yours, id)
		} else {
			theirs = id
		}
	}
	if len(yours) != 2 || theirs == 0 {
		t.Fatalf("precondition: %d your-side Pyromancers, theirs set: %v", len(yours), theirs)
	}
	counters := func() int { return targetCounterN(e.G.Obj(mentor), "P1P1") }
	heal := func() { e.G.Obj(mentor).Damage = 0 }
	// A ping resolves one {T}: deals-1-damage activation of the Pyromancer
	// `p` against the mentor. The activations tap their sources; each
	// harness step uses a different one, and the heal between steps is
	// setup-only state repair so the 3/2 mentor survives the assertion
	// sequence (a lethal ping would move it out from under the counters).
	ping := func(p, target state.ObjID) {
		t.Helper()
		seat := e.G.Obj(p).Controller
		if o := e.G.Obj(p); o == nil || o.Tapped || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: source %d not an untapped battlefield permanent: %+v", p, o)
		}
		e.askPriority(seat)
		submitChoices(t, e, abilityOption(t, e, p, 0).Index)
		submitChoices(t, e, targetOptionIdx(t, e, target))
		passUntilStackEmpty(t, e, 40)
	}
	newTurn := func() {
		t.Helper()
		e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
		e.G.Step = state.StepMain1
		for _, id := range append(yours, theirs) {
			e.G.Obj(id).Tapped = false
		}
	}

	// An opponent's targeting fires nothing (ValidSource$ SpellAbility.YouCtrl)
	// and must not arm the once-per-turn count.
	ping(theirs, mentor)
	if got := counters(); got != 0 {
		t.Fatalf("opponent targeting fired the Valiant trigger: counters = %d, want 0", got)
	}
	heal()
	// Your first targeting this turn fires.
	ping(yours[0], mentor)
	if got := counters(); got != 1 {
		t.Fatalf("first your-side targeting this turn did not fire: counters = %d, want 1", got)
	}
	heal()
	// Your second targeting the same turn fires again ONLY under the bug:
	// the ability word's once-each-turn clause must hold it back.
	ping(yours[1], mentor)
	if got := counters(); got != 1 {
		t.Fatalf("second your-side targeting this turn fired: counters = %d, want 1", got)
	}
	heal()
	// A new turn re-arms the count.
	newTurn()
	ping(yours[0], mentor)
	if got := counters(); got != 2 {
		t.Fatalf("first your-side targeting of the next turn did not fire: counters = %d, want 2", got)
	}
	heal()
	// The next turn, an opponent's targeting first: still nothing, and it
	// must not disarm the count, so your first targeting after it fires.
	newTurn()
	ping(theirs, mentor)
	if got := counters(); got != 2 {
		t.Fatalf("opponent targeting fired or disarmed across turns: counters = %d, want 2", got)
	}
	heal()
	ping(yours[1], mentor)
	if got := counters(); got != 3 {
		t.Fatalf("your first targeting after an opponent's targeting did not fire: counters = %d, want 3", got)
	}
}
