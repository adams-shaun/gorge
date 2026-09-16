package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// devilToBattlefield puts the named damage-offer Devil into seat 0's hand and
// returns its hand id. The engine test below casts it for real, so the whole
// path — cast, stack resolve, ETB trigger push, mid-resolution unless-pay
// ask — runs end to end.
func devilToHand(t *testing.T, reg *cards.Registry, name string) (*Engine, state.ObjID) {
	t.Helper()
	e := handEngine(t, mustCorpusCard(t, reg, name))
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(id).Face().Name == name {
			return e, id
		}
	}
	t.Fatalf("hand missing %q", name)
	return nil, 0
}

// drainUntilKModes passes every priority decision until a mid-resolution
// KModes ask becomes pending, and returns it.
func drainUntilKModes(t *testing.T, e *Engine, limit int) *decision.Decision {
	t.Helper()
	for i := 0; i < limit && !e.G.Over && e.Pending() != nil; i++ {
		d := e.Pending()
		if d.Kind == decision.KModes {
			return d
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision kind %v while seeking the unless-pay ask", d.Kind)
		}
		castFirst(t, e, "pass")
	}
	return nil
}

// countPlayerDamage sums Damage events naming player p.
func countPlayerDamage(e *Engine, p state.PlayerID) int {
	n := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.Damage && ev.Player == p {
			n += int(ev.Amount)
		}
	}
	return n
}

// TestEngineVexingDevilAcceptanceDealsDamageAndSacrifices is the engine-level
// half of the fb-20260916T070855Z-0532c816 regression: a REAL cast of Vexing
// Devil resolves, its ETB trigger asks seat 1 (the opponent) through rules'
// resume machinery, the acceptance emits the Damage event FROM RULES (the
// split the brief names: effects never emits the payment) and then
// sacrifices the Devil.
func TestEngineVexingDevilAcceptanceDealsDamageAndSacrifices(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, devil := devilToHand(t, reg, "Vexing Devil")
	e.G.Players[0].Pool = state.Mana{state.MR: 1}
	e.askPriority(0)
	submitChoices(t, e, passToCast(t, e, devil))

	ask := drainUntilKModes(t, e, 60)
	if ask == nil {
		t.Fatal("no damage offer posed for the resolving Vexing Devil")
	}
	if ask.Player != 1 {
		t.Fatalf("offer player = seat %d, want the opponent seat 1", ask.Player)
	}
	if ask.ResumeKind != "unless_pay" {
		t.Fatalf("offer resume kind = %q, want unless_pay", ask.ResumeKind)
	}
	if got := ask.Options[0].Label; got != "Take 4 damage" {
		t.Fatalf("accept label = %q, want the rendered take-4 offer", got)
	}

	// Accept: rules pays the damage, then the Devil is sacrificed.
	submitChoices(t, e, ask.Options[0].Index)
	if n := countPlayerDamage(e, 1); n != 4 {
		t.Fatalf("seat 1 took %d damage, want 4 (the accepted offer, dealt from the Devil)", n)
	}
	if z := e.G.Obj(devil).Zone; z != state.ZGraveyard {
		t.Fatalf("accepted Devil zone = %v, want graveyard (sacrificed)", z)
	}
	if life := e.G.Players[1].Life; life != 16 {
		t.Fatalf("seat 1 life = %d, want 16 (20 - 4)", life)
	}
	// Drive to the end: the game must keep running, not wedge.
	for i := 0; i < 200 && e.Pending() != nil && !e.G.Over; i++ {
		d := e.Pending()
		if d.Kind == decision.KPriority {
			castFirst(t, e, "pass")
		} else if d.Kind == decision.KModes && len(d.Options) > 0 {
			submitChoices(t, e, d.Options[0].Index)
		} else {
			submitChoices(t, e, d.Options[0].Index)
		}
	}
}

// TestEngineVexingDevilDeclineLeavesItInPlay pins the decline branch through
// the engine: the opponent refuses and the 4/3 stays on the battlefield.
func TestEngineVexingDevilDeclineLeavesItInPlay(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, devil := devilToHand(t, reg, "Vexing Devil")
	e.G.Players[0].Pool = state.Mana{state.MR: 1}
	e.askPriority(0)
	submitChoices(t, e, passToCast(t, e, devil))

	ask := drainUntilKModes(t, e, 60)
	if ask == nil {
		t.Fatal("no damage offer posed for the resolving Vexing Devil")
	}
	submitChoices(t, e, ask.Options[1].Index)
	if z := e.G.Obj(devil).Zone; z != state.ZBattlefield {
		t.Fatalf("declined Devil zone = %v, want battlefield", z)
	}
	if n := countPlayerDamage(e, 1); n != 0 {
		t.Fatalf("declining seat 1 took %d damage, want none", n)
	}
	if life := e.G.Players[1].Life; life != 20 {
		t.Fatalf("seat 1 life = %d, want 20 (no damage)", life)
	}
}

// TestEngineUpkeepSacrificeUnlessPayThroughPayMana drives Whipstitched
// Zombie's real upkeep Execute through the engine: the echo card asks its
// controller at upkeep, the paid answer (floating {B} in the pool) spares
// the Zombie via the SHARED unless_pay arm, and a decline sacrifices it.
func TestEngineUpkeepSacrificeUnlessPayThroughPayMana(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := New(Config{Seed: 1, Names: []string{"a", "b"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	// Put the Zombie on the battlefield directly (test-setup concern, the
	// same discipline sacrifice_test.go's putBattlefield uses) and float the
	// {B} the pay arm will spend.
	o := e.G.AddObject(mustCorpusCard(t, reg, "Whipstitched Zombie"), 0)
	o.Zone = state.ZBattlefield
	zid := o.ID
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), zid))
	e.G.Players[0].Pool = state.Mana{state.MB: 1}

	// Walk into seat 0's upkeep: the echo trigger fires on the StepChange
	// and asks (the same direct-event setup cardname_cost_test uses).
	e.Advance()
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	for i := 0; i < 100; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("engine stalled with no pending decision before the upkeep")
		}
		if d.Kind == decision.KModes && d.ResumeKind == "unless_pay" && d.ResumeSA != nil && d.ResumeSA.API == "Sacrifice" {
			// Paid answer: rules' payMana spends the floating {B}.
			if d.Player != 0 {
				t.Fatalf("pay ask player = seat %d, want the controller", d.Player)
			}
			before := e.G.Players[0].Pool[state.MB]
			submitChoices(t, e, d.Options[0].Index)
			if e.G.Obj(zid).Zone != state.ZBattlefield {
				t.Fatalf("paid echo sacrifice moved the Zombie off the battlefield")
			}
			if e.G.Players[0].Pool[state.MB] != before-1 {
				t.Fatalf("pool B = %d after paying, want %d-1 (payMana spent it)", e.G.Players[0].Pool[state.MB], before)
			}
			return
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected %v before the upkeep ask", d.Kind)
		}
		castFirst(t, e, "pass")
	}
	t.Fatal("no Sacrifice unless-pay ask posed by turn 100")
}

// TestEngineUpkeepSacrificeDeclineSacrifices pins the echo decline branch:
// an empty pool cannot pay, so a "pay" answer degrades to a decline and the
// Zombie is sacrificed.
func TestEngineUpkeepSacrificeDeclineSacrifices(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e := New(Config{Seed: 1, Names: []string{"a", "b"}, Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	for p := state.PlayerID(0); p < 2; p++ {
		e.G.SetZone(state.ZHand, p, nil)
	}
	o := e.G.AddObject(mustCorpusCard(t, reg, "Whipstitched Zombie"), 0)
	o.Zone = state.ZBattlefield
	zid := o.ID
	e.G.SetZone(state.ZBattlefield, 0, append(e.G.Zone(state.ZBattlefield, 0), zid))

	e.Advance()
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	for i := 0; i < 100; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("engine stalled with no pending decision before the upkeep")
		}
		if d.Kind == decision.KModes && d.ResumeKind == "unless_pay" && d.ResumeSA != nil && d.ResumeSA.API == "Sacrifice" {
			// Answer "pay" from an EMPTY pool: the resume arm cannot cover
			// {B}, records the decline, and the Zombie is sacrificed.
			submitChoices(t, e, d.Options[0].Index)
			if z := e.G.Obj(zid).Zone; z != state.ZGraveyard {
				t.Fatalf("unpayable pay answer zone = %v, want graveyard (declined)", z)
			}
			return
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected %v before the upkeep ask", d.Kind)
		}
		castFirst(t, e, "pass")
	}
	t.Fatal("no Sacrifice unless-pay ask posed by turn 100")
}

// TestEngineLonghornFirebeastFiveThroughEngine pins the second damage-offer
// card end to end: N=5, acceptance deals 5 and sacrifices.
func TestEngineLonghornFirebeastFiveThroughEngine(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, beast := devilToHand(t, reg, "Longhorn Firebeast")
	e.G.Players[0].Pool = state.Mana{state.MR: 3}
	e.askPriority(0)
	submitChoices(t, e, passToCast(t, e, beast))

	ask := drainUntilKModes(t, e, 60)
	if ask == nil {
		t.Fatal("no damage offer posed for the resolving Longhorn Firebeast")
	}
	if got := ask.Options[0].Label; got != "Take 5 damage" {
		t.Fatalf("accept label = %q, want the rendered take-5 offer", got)
	}
	submitChoices(t, e, ask.Options[0].Index)
	if n := countPlayerDamage(e, 1); n != 5 {
		t.Fatalf("seat 1 took %d damage, want 5", n)
	}
	if z := e.G.Obj(beast).Zone; z != state.ZGraveyard {
		t.Fatalf("accepted Firebeast zone = %v, want graveyard", z)
	}
}

// TestEngineVexingDevilLifelinkSourceGainsLife pins the rider on rules'
// payment event: a Vexing Devil granted lifelink (printed keyword on the
// corpus face) makes its CONTROLLER gain the 4 the accepting opponent took.
func TestEngineVexingDevilLifelinkSourceGainsLife(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	src := mustCorpusCard(t, reg, "Vexing Devil")
	src.Faces[0].Keywords = append(src.Faces[0].Keywords, "Lifelink")
	e := handEngine(t, src)
	for _, id := range e.G.Zone(state.ZHand, 0) {
		if e.G.Obj(id).Face().Name == "Vexing Devil" {
			e.G.Players[0].Pool = state.Mana{state.MR: 1}
			e.askPriority(0)
			submitChoices(t, e, passToCast(t, e, id))
			ask := drainUntilKModes(t, e, 60)
			if ask == nil {
				t.Fatal("no damage offer posed")
			}
			submitChoices(t, e, ask.Options[0].Index)
			if n := countPlayerDamage(e, 1); n != 4 {
				t.Fatalf("seat 1 took %d damage, want 4", n)
			}
			if life := e.G.Players[0].Life; life != 24 {
				t.Fatalf("seat 0 life = %d, want 24 (20 + 4 lifelink)", life)
			}
			return
		}
	}
	t.Fatal("Vexing Devil not in hand")
}
