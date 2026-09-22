package rules

// CR 702.148: the Ravenous keyword. Real corpus-card tests (the .cards corpus,
// never a committed .txt) for the 12-carrier population measured at task
// kw-ravenous; Jacked Rabbit is the pinned carrier because its script also
// reads Count$CardPower off the counters the keyword places, so the counter
// put and the attack-token half share one observable.
//
// What the keyword expands to (cards/kw_ravenous.go): an ETB trigger that
// puts X +1/+1 counters (X = the {X} the cast paid, Count$xPaid) and then, as
// a SubAbility$, draws a card gated on that same count with the shared
// SVar-condition engine (ConditionCheckSVar$ + ConditionSVarCompare$ GE5).
// The tests pin both halves of the oracle text: X=3 -> 3 counters and NO
// draw; X=5 -> 5 counters and one draw.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// ravenousConfig seeds a 2-seat game whose seat 0 carries one real Jacked
// Rabbit plus 39 Mountains, drives to the first Main 1 (whichever seat won
// the toss -- the starting player is seed-dependent, so the fixture must not
// assume seat 0 is active) and moves that seat's Rabbit to hand. The
// engine/config/id triple mirrors etbConfig, but the card is the compiled
// corpus card (mustCorpusCard) rather than an inline fixture, which is what
// makes this a real-carrier test.
func ravenousConfig(t *testing.T, seed uint64) (*Engine, Config, state.ObjID, state.PlayerID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	rabbit := mustCorpusCard(t, reg, "Jacked Rabbit")
	// The Rabbit is seeded into BOTH decks so the turn-1 active seat always
	// has one; the toss winner is seed-dependent and the brief's test must
	// not depend on the seed.
	seatDeck := func() []*cards.Card {
		return append([]*cards.Card{rabbit}, mountainDeck(t, 39)...)
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{seatDeck(), seatDeck()}})
	e := New(cfg)
	e.Advance()
	// Drive to the first Main 1 of the turn-1 active seat (toMain1 is
	// idempotent and keys on e.G.Turn/e.G.Active), then bridge that seat's
	// Rabbit into its hand.
	toMain1(t, e)
	caster := e.G.Active
	id := findByName(e, "Jacked Rabbit", caster)
	if id == 0 {
		t.Fatalf("corpus Jacked Rabbit not found for seat %d -- corpus missing?", caster)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: state.ZHand})
	e.pending = nil
	e.priorityRound()
	return e, cfg, id, caster
}

// castRavenous casts the rabbit at the given X and returns the draw events
// observed while the stack drained.
func castRavenous(t *testing.T, e *Engine, id state.ObjID, x int) int {
	t.Helper()
	submitChoices(t, e, castOptionFor(t, e, id).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("X decision for the Ravenous cast: %+v", d)
	}
	submitChoices(t, e, x)
	before := countDraw(e)
	passUntilStackEmpty(t, e, 30)
	return countDraw(e) - before
}

// TestJackedRabbitRavenousX3CountersNoDraw is the brief's X=3 leg: the
// creature enters with 3 +1/+1 counters and does NOT draw (3 < 5).
func TestJackedRabbitRavenousX3CountersNoDraw(t *testing.T) {
	e, cfg, id, caster := ravenousConfig(t, 131)
	addMana(t, e, caster, "WWWWW") // {X}{1}{W} with X=3 -> {4}{W}
	drawn := castRavenous(t, e, id, 3)
	o := e.G.Obj(id)
	// Precondition: the cast actually resolved the Rabbit onto the
	// battlefield. A silently-countered or still-in-hand Rabbit would make
	// the counter/draw assertions meaningless.
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition failed: Jacked Rabbit zone=%s, want battlefield", o.Zone)
	}
	if got := o.Counter("P1P1"); got != 3 {
		t.Fatalf("Ravenous X=3 put %d +1/+1 counters, want 3", got)
	}
	if e.Power(id) != 4 {
		t.Fatalf("Ravenous X=3 power = %d, want 4 (printed 1 + 3 counters)", e.Power(id))
	}
	if drawn != 0 {
		t.Fatalf("Ravenous X=3 drew %d cards, want 0 (the X>=5 gate must not fire)", drawn)
	}
	replayCheck(t, e, cfg)
}

// TestJackedRabbitRavenousX5CountersAndDraw is the brief's X=5 leg: 5
// +1/+1 counters AND the entry draw.
func TestJackedRabbitRavenousX5CountersAndDraw(t *testing.T) {
	e, cfg, id, caster := ravenousConfig(t, 132)
	addMana(t, e, caster, "WWWWWWW") // {X}{1}{W} with X=5 -> {6}{W}
	drawn := castRavenous(t, e, id, 5)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("precondition failed: Jacked Rabbit zone=%s, want battlefield", o.Zone)
	}
	if got := o.Counter("P1P1"); got != 5 {
		t.Fatalf("Ravenous X=5 put %d +1/+1 counters, want 5", got)
	}
	if e.Power(id) != 6 {
		t.Fatalf("Ravenous X=5 power = %d, want 6 (printed 1 + 5 counters)", e.Power(id))
	}
	if drawn != 1 {
		t.Fatalf("Ravenous X=5 drew %d cards, want exactly 1 (the X>=5 gate)", drawn)
	}
	replayCheck(t, e, cfg)
}

// TestJackedRabbitRavenousAttackTokensReadTheCounters proves the two halves
// of the card interact: the attack trigger's TokenAmount$ Count$CardPower
// reads the power the Ravenous counters raised, so an X=3 Rabbit attacks and
// makes 4 Rabbits -- not the 1 it would make with no counters.
func TestJackedRabbitRavenousAttackTokensReadTheCounters(t *testing.T) {
	e, cfg, id, caster := ravenousConfig(t, 133)
	addMana(t, e, caster, "WWWWW")
	if drawn := castRavenous(t, e, id, 3); drawn != 0 {
		t.Fatalf("precondition failed: X=3 drew %d cards, want 0", drawn)
	}
	if e.G.Obj(id).Counter("P1P1") != 3 {
		t.Fatalf("precondition failed: Ravenous counters = %d, want 3", e.G.Obj(id).Counter("P1P1"))
	}
	// Advance into seat 0's combat and declare it as an attacker, then let
	// the attack trigger resolve.
	driveToStep(t, e, e.G.Turn, caster, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackerAt(t, e, id, 1-caster)
	before := countTokenCreate(e)
	passUntilStackEmpty(t, e, 40)
	made := countTokenCreate(e) - before
	if made != 4 {
		t.Fatalf("attack trigger minted %d tokens, want 4 (power 4, one per point of power)", made)
	}
	replayCheck(t, e, cfg)
}

// countTokenCreate counts the whole-table TokenCreate events, the observable
// the attack trigger's TokenAmount$ Count$CardPower drives.
func countTokenCreate(e *Engine) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.TokenCreate {
			n++
		}
	}
	return n
}
