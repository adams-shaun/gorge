package rules

// Myriad's CR 702.116a copy-attack defender and its CR 702.116a / 511.2
// delayed end-of-combat exile trigger. CR 702.116a: each token is "tapped and
// attacking that player or a planeswalker they control"; CR 511.2: "exile the
// tokens at end of combat" is a delayed triggered ability that triggers at
// the beginning of the end of combat step, goes on the stack and gives
// players priority while the tokens are still on the battlefield.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const myriadTestWalkerSrc = "Name:Myriad Test Walker\nTypes:Planeswalker Jace\nLoyalty:3\nOracle:x\n"

// myriadPutWalker puts one planeswalker under seat q's control on the
// battlefield, entered through MoveZone so its printed loyalty stands (the
// zero-loyalty state-based action would otherwise sweep it away), and
// returns its id.
func myriadPutWalker(t *testing.T, e *Engine, q state.PlayerID) state.ObjID {
	t.Helper()
	wo := e.G.AddObject(card(t, myriadTestWalkerSrc), q)
	walker := wo.ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: walker, From: wo.Zone, To: state.ZBattlefield})
	if o := e.G.Obj(walker); o == nil || o.Zone != state.ZBattlefield || o.Controller != q {
		t.Fatalf("precondition: walker = %+v, want battlefield under seat %d", o, q)
	}
	return walker
}

// myriadStackExileTrigger reports the id of the end-of-combat exile delayed
// trigger on the stack over the Myriad token token (the self-sourced
// __kwWarpExile ChangeZone body), or 0.
func myriadStackExileTrigger(e *Engine, token state.ObjID) state.ObjID {
	for _, id := range e.G.Stack {
		o := e.G.Obj(id)
		if o != nil && o.Ability != nil && o.Ability.API == "ChangeZone" && o.Source == token {
			return id
		}
	}
	return 0
}

// TestMyriadAttacksPlaneswalker: in 4 seats the Myriad copy for an opponent
// who controls a planeswalker offers that player or the planeswalker (and
// the decline), and the copy attacks the one chosen. The copy for an
// opponent with no planeswalker still asks the plain two-option question.
func TestMyriadAttacksPlaneswalker(t *testing.T) {
	e, _, _ := myriadCombat(t, 4)
	// The walker enters out-of-band (AddObject + MoveZone, the watcher-test
	// convention), so the event log cannot rebuild it and this test does not
	// replayCheck.
	walker := myriadPutWalker(t, e, 2)

	// First ask: seat 2 controls a planeswalker, so its ask offers the
	// player, the planeswalker and the decline.
	d := passToDecision(t, e, 8)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "myriad" || d.Player != 0 {
		t.Fatalf("Myriad choice = %+v", d)
	}
	if len(d.Options) != 3 {
		t.Fatalf("Myriad choice for seat 2 = %+v, want player / planeswalker / decline", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{1}}); err != nil {
		t.Fatalf("attack the planeswalker: %v", err)
	}

	// Second ask: seat 3 controls no planeswalker, so its ask is the plain
	// player-or-decline pair, answered with the player.
	d = passToDecision(t, e, 8)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "myriad" || len(d.Options) != 2 {
		t.Fatalf("Myriad choice for seat 3 = %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0}}); err != nil {
		t.Fatalf("attack seat 3: %v", err)
	}
	passUntilStackEmpty(t, e, 20)

	pwAttacker, seatAttacker := 0, 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if !o.IsMyriad {
			continue
		}
		switch {
		case o.AttackingBattle == walker:
			pwAttacker++
			if o.Attacking != 2 {
				t.Fatalf("Myriad copy attacks seat %d via the walker, want seat 2", o.Attacking)
			}
		case o.AttackingBattle == 0:
			seatAttacker++
			if o.Attacking != 3 {
				t.Fatalf("Myriad copy attacks seat %d, want seat 3", o.Attacking)
			}
		default:
			t.Fatalf("Myriad copy attacks battle %d, want the seat-2 walker or no battle", o.AttackingBattle)
		}
	}
	if pwAttacker != 1 || seatAttacker != 1 {
		t.Fatalf("Myriad made %d walker attackers and %d seat-3 attackers, want 1 and 1", pwAttacker, seatAttacker)
	}
}

// TestMyriadExileIsDelayedTrigger: at the beginning of the end of combat
// step, the exile trigger is on the stack and a player has priority while
// the tokens are still on the battlefield; resolving it exiles them.
func TestMyriadExileIsDelayedTrigger(t *testing.T) {
	e, cfg, _ := myriadCombat(t, 4)
	// Create the copy for seat 2, decline seat 3's.
	d := passToDecision(t, e, 8)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "myriad" {
		t.Fatalf("Myriad choice = %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{0}}); err != nil {
		t.Fatalf("create Myriad copy: %v", err)
	}
	d = passToDecision(t, e, 8)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "myriad" {
		t.Fatalf("Myriad choice for seat 3 = %+v", d)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 0, Choices: []int{1}}); err != nil {
		t.Fatalf("decline Myriad copy for seat 3: %v", err)
	}
	var tokenID state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).IsMyriad {
			tokenID = id
		}
	}
	if tokenID == 0 {
		t.Fatal("no Myriad token on the battlefield after the create answer")
	}

	// Pass priority only (never drain a non-priority ask) until the end of
	// combat step has begun and the exile trigger waits on the stack: the
	// moment a plain passUntilStackEmpty has already resolved it past.
	found := false
	for i := 0; i < 40 && !found; i++ {
		if e.G.Step == state.StepEndCombat && myriadStackExileTrigger(e, tokenID) != 0 {
			if d := e.Pending(); d != nil {
				found = true
				break
			}
		}
		pd := e.Pending()
		if pd == nil {
			t.Fatalf("no decision while combat runs (step %v)", e.G.Step)
		}
		if pd.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %+v while combat runs", pd)
		}
		if err := e.Submit(decision.Intent{Seq: pd.Seq, Player: pd.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("pass priority: %v", err)
		}
	}
	if !found {
		t.Fatal("the end-of-combat exile trigger never waited on the stack with priority outstanding")
	}
	if e.G.Obj(tokenID) == nil || e.G.Obj(tokenID).Zone != state.ZBattlefield {
		t.Fatal("the Myriad token left the battlefield before the exile trigger resolved")
	}
	passUntilStackEmpty(t, e, 20)
	// A token exiled this way ceases to exist (CR 704.5d): the proof is the
	// resolution's logged battlefield-to-exile move plus the token's absence
	// from the battlefield.
	if o := e.G.Obj(tokenID); o != nil && o.Zone == state.ZBattlefield {
		t.Fatal("the Myriad token is still on the battlefield after the exile trigger resolved")
	}
	exiled := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == tokenID && ev.To == state.ZExile {
			exiled = true
		}
	}
	if !exiled {
		t.Fatal("the resolved exile trigger did not exile the Myriad token")
	}
	replayCheck(t, e, cfg)
}
