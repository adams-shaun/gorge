package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// moveNamedToHand moves the first copy of name from seat 0's library to its
// hand through a logged MoveZone and returns its id.
func moveNamedToHand(t *testing.T, e *Engine, name string) state.ObjID {
	t.Helper()
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return id
		}
	}
	for _, id := range e.G.Zone(state.ZLibrary, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZHand})
			return id
		}
	}
	t.Fatalf("corpus fixture %q absent from seat 0's hand and library", name)
	return 0
}

// altCastOffer returns the cast option for id whose Mode is mode, or nil.
func altCastOffer(t *testing.T, e *Engine, id state.ObjID, mode string) *decision.Option {
	t.Helper()
	for _, o := range castOptions(t, e) {
		if o.Obj == id && o.Mode == mode {
			return &o
		}
	}
	return nil
}

// combatHitScenario builds a corpus engine holding spell in hand and
// attacker on the battlefield, lands a one-point hit from attacker on seat 1
// (combat when combat is set), and funds seat 0 with mana.
func combatHitScenario(t *testing.T, spell, attacker string, combat bool, mana string) (*Engine, Config, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, spell, attacker)
	id := moveNamedToHand(t, e, spell)
	src := putNamedOnBattlefield(t, e, attacker)
	life := e.G.Players[1].Life
	damageCountEvent(e, src, 0, 1, 1, combat)
	if e.G.Players[1].Life != life-1 {
		t.Fatalf("precondition: %s's hit did not land (life %d -> %d)", attacker, life, e.G.Players[1].Life)
	}
	if recs := e.G.Obj(src).DamageDealtThisTurn; len(recs) != 1 || recs[0].Combat != combat || !isPlayerRecipient(recs[0].Recipient) {
		t.Fatalf("precondition: %s's damage record = %+v", attacker, recs)
	}
	addMana(t, e, 0, mana)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: %s is not in hand", spell)
	}
	return e, cfg, id
}

// CR 702.76a: prowl may be paid only if you dealt combat damage to a player
// this turn with a source sharing a creature type with the spell. Auntie's
// Snitch is a Goblin Rogue; a Goblin Warrior's hit enables its {1}{B}.
func TestProwlNeedsASharedTypeCombatHit(t *testing.T) {
	t.Parallel()
	e, cfg, snitch := combatHitScenario(t, "Auntie's Snitch", "Goblin Piker", true, "BB")
	offer := altCastOffer(t, e, snitch, "prowled")
	if offer == nil {
		t.Fatal("prowl not offered after a Goblin's combat hit on a player")
	}
	pool := e.G.Players[0].Pool.Total()
	submitChoices(t, e, offer.Index)
	if got := e.G.Players[0].Pool.Total(); got != pool-2 {
		t.Fatalf("prowl paid %d mana, want 2 ({1}{B})", pool-got)
	}
	if o := e.G.Obj(snitch); o.Zone != state.ZStack || !effects.MatchesSpecCtx(e.G, "Card.Self+prowled", snitch, e.specCtx(snitch, 0)) {
		t.Fatalf("prowl cast: zone %s, Card.Self+prowled false (flags %+v)", o.Zone, o.CastFlags)
	}
	replayCheck(t, e, cfg)

	// A non-Goblin non-Rogue attacker's hit does not enable prowl.
	e2, cfg2, snitch2 := combatHitScenario(t, "Auntie's Snitch", "Grizzly Bears", true, "BBB")
	if altCastOffer(t, e2, snitch2, "") == nil {
		t.Fatal("precondition: Auntie's Snitch is not castable for its mana cost")
	}
	if altCastOffer(t, e2, snitch2, "prowled") != nil {
		t.Fatal("prowl offered after a Bear's combat hit")
	}
	replayCheck(t, e2, cfg2)

	// A Goblin's NON-combat hit does not enable prowl either.
	e3, cfg3, snitch3 := combatHitScenario(t, "Auntie's Snitch", "Goblin Piker", false, "BB")
	if altCastOffer(t, e3, snitch3, "prowled") != nil {
		t.Fatal("prowl offered after a non-combat hit")
	}
	replayCheck(t, e3, cfg3)
}

// CR 702.173a: freerunning may be paid only if you dealt combat damage to a
// player this turn with an Assassin or your commander.
func TestFreerunningNeedsAnAssassinCombatHit(t *testing.T) {
	t.Parallel()
	e, cfg, amb := combatHitScenario(t, "Brotherhood Ambushers", "Royal Assassin", true, "BBBB")
	offer := altCastOffer(t, e, amb, "freerun")
	if offer == nil {
		t.Fatal("freerunning not offered after an Assassin's combat hit on a player")
	}
	pool := e.G.Players[0].Pool.Total()
	submitChoices(t, e, offer.Index)
	if got := e.G.Players[0].Pool.Total(); got != pool-4 {
		t.Fatalf("freerunning paid %d mana, want 4 ({3}{B})", pool-got)
	}
	if o := e.G.Obj(amb); o.Zone != state.ZStack || effects.EvalCount(e, effects.NewCtxPtr(amb, 0, effects.CtxInit{}), "Count$Freerunning.1.0") != 1 {
		t.Fatalf("freerunning cast: zone %s, Count$Freerunning.1.0 not 1 (flags %+v)", o.Zone, o.CastFlags)
	}
	replayCheck(t, e, cfg)

	e2, cfg2, amb2 := combatHitScenario(t, "Brotherhood Ambushers", "Grizzly Bears", true, "BBBBB")
	if altCastOffer(t, e2, amb2, "") == nil {
		t.Fatal("precondition: Brotherhood Ambushers is not castable for its mana cost")
	}
	if altCastOffer(t, e2, amb2, "freerun") != nil {
		t.Fatal("freerunning offered after a non-Assassin's combat hit")
	}
	replayCheck(t, e2, cfg2)
}

func isPlayerRecipient(id state.ObjID) bool {
	_, ok := id.PlayerRef()
	return ok
}

// CR 702.173a's other half: a combat hit by a commander the caster
// controlled opens freerunning whatever its types ("an Assassin or
// commander you control"); the same commander controlled by another seat,
// and a plain non-Assassin non-commander, do not.
func TestFreerunningCountsTheCastersCommander(t *testing.T) {
	t.Parallel()
	g := &state.Game{Players: make([]state.Player, 2), Objs: make([]state.Object, 3)}
	for i := range g.Objs {
		g.Objs[i].ID = state.ObjID(i + 1)
	}
	g.Players[0].Commanders = []state.ObjID{1}
	g.Players[1].Commanders = []state.ObjID{2}
	hit := func(src state.ObjID, p state.PlayerID) {
		g.Objs[src-1].DamageDealtThisTurn = append(g.Objs[src-1].DamageDealtThisTurn, state.DamageDealtRecord{
			SourceControl: p, Recipient: state.PlayerRef(1 - p), Amount: 1, Combat: true,
			HasSourceZone: true, SourceZone: state.ZBattlefield, SourceTypes: []string{"Creature", "Human", "Warrior"}})
	}
	hit(3, 0) // a plain Human Warrior: not an Assassin, not a commander
	if combatHitThisTurn(g, 0, freerunningHit(g, 0)) {
		t.Fatal("a non-Assassin non-commander hit opened freerunning")
	}
	hit(2, 1) // seat 1's commander, controlled by seat 1: seat 1's hit, not seat 0's
	if combatHitThisTurn(g, 0, freerunningHit(g, 0)) {
		t.Fatal("another seat's commander hit, controlled by that seat, opened seat 0's freerunning")
	}
	hit(2, 0) // the same commander, controlled by seat 0 at the hit: CR 702.173a opens it
	if !combatHitThisTurn(g, 0, freerunningHit(g, 0)) {
		t.Fatal("a commander seat 0 controlled at the hit did not open freerunning")
	}
	hit(1, 0)
	if !combatHitThisTurn(g, 0, freerunningHit(g, 0)) {
		t.Fatal("seat 0's commander's combat hit did not open freerunning")
	}
	if !combatHitThisTurn(g, 1, freerunningHit(g, 1)) {
		t.Fatal("seat 1's commander's own hit did not open seat 1's freerunning")
	}
}
