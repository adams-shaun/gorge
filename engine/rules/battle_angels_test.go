package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// battleAngelsSetup is one scenario's board for the Battle Angels of Tyr
// combat-damage trigger: who holds more cards, who controls more lands, and
// the pre-combat life totals.
type battleAngelsSetup struct {
	seat1HandExtra, seat2HandExtra int
	seat1Lands, seat2Lands         int
	seat1Life, seat2Life           int32
}

// battleAngelsRun is the fixture a test inspects after combat.
type battleAngelsRun struct {
	e        *Engine
	cfg      Config
	attacker state.ObjID
	draws0   int
	tokens0  int
	life0    int32
}

// battleAngelsCombat builds a 3-seat board with Battle Angels of Tyr on seat
// 0's battlefield, parked at seat 0's declare-attackers step (the
// combatTriggerBoard shape generalised to three seats). The attacker is
// armed, the extra cards/lands are logged MoveZone events, and the life
// totals are logged LifeChange events, so the whole board replays.
func battleAngelsCombat(t *testing.T, s battleAngelsSetup) battleAngelsRun {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	ba, ok := reg.Lookup("Battle Angels of Tyr")
	if !ok || ba == nil {
		t.Fatal("corpus fixture: Battle Angels of Tyr missing")
	}
	names := []string{"a", "b", "c"}
	decks := [][]*cards.Card{
		append([]*cards.Card{ba}, mountainDeck(t, 39)...),
		mountainDeck(t, 40),
		mountainDeck(t, 40),
	}
	cfg := seatZeroStart(Config{Seed: 196, Names: names, Decks: decks, Tokens: reg.Tokens})
	e := New(cfg)

	var attacker state.ObjID
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.Owner == 0 && o.Card == ba {
			attacker = o.ID
			break
		}
	}
	if attacker == 0 {
		t.Fatal("Battle Angels of Tyr was not dealt to seat 0")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: attacker, From: e.G.Obj(attacker).Zone, To: state.ZBattlefield})
	e.G.Obj(attacker).SummonSick = false
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})

	setLife := func(p state.PlayerID, want int32) {
		if d := want - e.G.Players[p].Life; d != 0 {
			e.emit(events.Event{Kind: events.LifeChange, Player: p, Amount: d})
		}
	}
	setLife(0, 20)
	setLife(1, s.seat1Life)
	setLife(2, s.seat2Life)

	move := func(p state.PlayerID, to state.Zone) state.ObjID {
		lib := e.G.Zone(state.ZLibrary, p)
		if len(lib) == 0 {
			t.Fatalf("seat %d library is empty", p)
		}
		id := lib[0]
		e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: e.G.Obj(id).Zone, To: to})
		return id
	}
	for i := 0; i < s.seat1HandExtra; i++ {
		move(1, state.ZHand)
	}
	for i := 0; i < s.seat2HandExtra; i++ {
		move(2, state.ZHand)
	}
	for i := 0; i < s.seat1Lands; i++ {
		move(1, state.ZBattlefield)
	}
	for i := 0; i < s.seat2Lands; i++ {
		move(2, state.ZBattlefield)
	}

	run := battleAngelsRun{e: e, cfg: cfg, attacker: attacker,
		draws0: countDraws(e, 0), tokens0: countTokenCreates(e, 0), life0: e.G.Players[0].Life}
	e.askAttackers()
	return run
}

// resolveBattleAngelsCombat declares the Battle Angels attacker, declines the
// Myriad copy (so only the real attacker connects), and passes through the
// combat-damage step.
func resolveBattleAngelsCombat(t *testing.T, run battleAngelsRun) battleAngelsRun {
	t.Helper()
	e := run.e
	// A 3-seat board offers one attack option per defender; choose the one
	// that attacks seat 1 (the loaded target), not every seat.
	d := e.Pending()
	if d == nil || d.Kind != decision.KAttackers {
		t.Fatalf("expected an attackers decision, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == run.attacker && o.Player == 1 {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("no Battle Angels attack option at seat 1: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit Battle Angels attacker: %v", err)
	}
	drainCombatPriority(t, e)
	// The Myriad keyword is a may: decline the copy for the other opponent
	// before damage, or the fixture would create an extra attacker.
	d = passToDecision(t, e, 8)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "myriad" || len(d.Options) != 2 {
		t.Fatalf("Myriad choice = %+v, want a two-option KChoose before damage", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{1}}); err != nil {
		t.Fatalf("decline Myriad copy: %v", err)
	}
	// Close the declare-attackers/blockers windows, then run the damage step
	// (which resolves the Battle Angels trigger after the damage batch).
	drainCombatPriority(t, e)
	drainCombatDamagePriority(t, e)
	passUntilStackEmpty(t, e, 30)
	return run
}

// TestBattleAngelsOfTyrCombatDamageTriggerChecksEachOtherPlayer pins the
// three "that player has more ... than each other player" gates on the real
// corpus script, end to end: the damaged player's hand, lands and life are
// compared against every OTHER player (the count group excludes the damaged
// player). Case one: seat 1 leads all three, so seat 0 draws, makes a
// Treasure and gains 3. Case two: seat 1 ties seat 2 on hand and lands and
// leads only on life, so only the life gain happens.
func TestBattleAngelsOfTyrCombatDamageTriggerChecksEachOtherPlayer(t *testing.T) {
	t.Parallel()

	// Case one: seat 1 strictly ahead on hand, lands and life.
	run := battleAngelsCombat(t, battleAngelsSetup{
		seat1HandExtra: 2, seat2HandExtra: 0,
		seat1Lands: 3, seat2Lands: 1,
		seat1Life: 30, seat2Life: 20,
	})
	run = resolveBattleAngelsCombat(t, run)
	e := run.e
	if got := countDraws(e, 0) - run.draws0; got != 1 {
		t.Fatalf("seat 0 drew %d cards from the trigger, want 1", got)
	}
	if got := countTokenCreates(e, 0) - run.tokens0; got != 1 {
		t.Fatalf("seat 0 created %d Treasure tokens, want 1", got)
	}
	if got := e.G.Players[0].Life - run.life0; got != 3 {
		t.Fatalf("seat 0 gained %d life, want 3", got)
	}
	replayCheck(t, e, run.cfg)

	// Case two: seat 2 ties seat 1 on hand and lands; seat 1 leads on life.
	run = battleAngelsCombat(t, battleAngelsSetup{
		seat1HandExtra: 2, seat2HandExtra: 2,
		seat1Lands: 3, seat2Lands: 3,
		seat1Life: 30, seat2Life: 20,
	})
	run = resolveBattleAngelsCombat(t, run)
	e = run.e
	if got := countDraws(e, 0) - run.draws0; got != 0 {
		t.Fatalf("seat 0 drew %d cards with a hand tie, want 0", got)
	}
	if got := countTokenCreates(e, 0) - run.tokens0; got != 0 {
		t.Fatalf("seat 0 created %d Treasure tokens with a land tie, want 0", got)
	}
	if got := e.G.Players[0].Life - run.life0; got != 3 {
		t.Fatalf("seat 0 gained %d life, want 3 (seat 1 leads on life alone)", got)
	}
	replayCheck(t, e, run.cfg)
}
