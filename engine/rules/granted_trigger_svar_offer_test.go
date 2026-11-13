// The CR 603.3d target offer of an Animate Triggers$ granted trigger reads
// the GRANTOR's SVar table, not the animated object's face. The real corpus
// carrier is Guardian Scalelord: its Backup rider animates the target
// creature with its own Attacks trigger granted until end of turn, and that
// granted body returns a nonland permanent card of mana value at most X from
// its controller's graveyard, with X bound (only on Guardian's face) to the
// attacking creature's power. The other participant is Old One Eye's 5/5
// Tyranid token (mana value 6 after the Backup counter: X, the attacking
// token's power, is 6), so with Old One Eye in the graveyard the trigger has
// a legal target and must be placed with its target ask; with an empty
// graveyard it has none and CR 603.3d drops it. On the recipient's table the
// X read finds nothing and resolved to the paid/stack 0, so the
// mana-value-at-most-X predicate admitted nothing in both branches (the
// divergence dirac's guardian-scalelord refusal recorded). No corpus .txt is
// inlined.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// scalelordTokenEngine casts the real corpus Guardian Scalelord with Old One
// Eye's Tyranid token (the g_5_5_tyranid corpus token script) as the Backup
// target, then returns the engine, the token id and the Old One Eye card id.
// The Backup cast put a +1/+1 counter on the token (power 6) and granted it
// Guardian's AttackTrig until end of turn.
func scalelordTokenEngine(t *testing.T, seed uint64) (*Engine, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	gs := lookup(t, reg, "Guardian Scalelord")
	ooe := lookup(t, reg, "Old One Eye")
	tokenScript, ok := reg.Tokens["g_5_5_tyranid"]
	if !ok || tokenScript == nil {
		t.Fatal("precondition failed: g_5_5_tyranid token script missing")
	}
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{gs, ooe}, nil)

	// Old One Eye's own token mint, event-sourced like putToken's
	// battlefield arm.
	toMain1(t, e)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "g_5_5_tyranid"})
	e.pending = nil
	tokenID := e.G.NextID - 1

	// PRECONDITION: the token is really on the battlefield under seat 0,
	// untapped and ready to attack at its printed 5/5.
	tok := e.G.Obj(tokenID)
	if tok == nil || tok.Zone != state.ZBattlefield || tok.Controller != 0 {
		t.Fatalf("token precondition: obj=%+v, want a battlefield permanent under seat 0", tok)
	}
	if tok.Face().Power() != 5 || tok.Counter("P1P1") != 0 {
		t.Fatalf("token precondition: power %d + %d P1P1, want the printed 5/5", tok.Face().Power(), tok.Counter("P1P1"))
	}
	tok.SummonSick = false

	// Cast Guardian Scalelord and aim the Backup counter at the token.
	gid := findByName(e, "Guardian Scalelord", 0)
	if gid == 0 {
		t.Fatal("precondition failed: Guardian Scalelord not in seat 0's library")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: gid, From: e.G.Obj(gid).Zone, To: state.ZHand})
	addMana(t, e, 0, "WWWWW")
	opt := castOptionFor(t, e, gid)
	if opt.Obj != gid {
		t.Fatalf("precondition failed: no cast option for Guardian Scalelord (got %+v)", opt)
	}
	submitChoices(t, e, opt.Index)
	backupAnswerTarget(t, e, tokenID, 30)
	passUntilStackEmpty(t, e, 30)
	tok = e.G.Obj(tokenID)
	if tok.Zone != state.ZBattlefield || tok.Counter("P1P1") != 1 {
		t.Fatalf("precondition failed: token zone/counters %v/%d, want battlefield/1", tok.Zone, tok.Counter("P1P1"))
	}
	if o := e.G.Obj(findByName(e, "Guardian Scalelord", 0)); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition failed: Guardian Scalelord zone %v, want battlefield", o)
	}
	// The granted trigger leaves with the animation at end of turn; the
	// attack below happens inside the same turn.
	return e, tokenID
}

// tokenAttackQueuesTheGrant attacks the token and returns the number of
// stack objects minted from its granted trigger plus the pending decision.
func tokenAttackQueuesTheGrant(t *testing.T, e *Engine, tokenID state.ObjID) (int, *decision.Decision) {
	t.Helper()
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	e.askAttackers()
	submitAttackersOnly(t, e, tokenID)
	pushes := 0
	for _, ev := range e.L.Events {
		if (ev.Kind == events.TriggerPush || ev.Kind == events.GrantTriggerPush) && ev.Obj == tokenID {
			pushes++
		}
	}
	for _, id := range e.G.Stack {
		if o := e.G.Obj(id); o != nil && o.Source == tokenID && o.StackKind == state.StackKindTriggered {
			pushes++
		}
	}
	return pushes, e.Pending()
}

func TestGuardianScalelordGrantedAttackTriggerTargetOffer(t *testing.T) {
	t.Run("empty graveyard drops the granted trigger", func(t *testing.T) {
		e, tokenID := scalelordTokenEngine(t, 611)
		if n := len(e.G.Zone(state.ZGraveyard, 0)); n != 0 {
			t.Fatalf("precondition failed: seat 0's graveyard holds %d cards, want none", n)
		}
		pushes, d := tokenAttackQueuesTheGrant(t, e, tokenID)
		if pushes == 0 {
			t.Fatal("precondition failed: the granted attack trigger never fired")
		}
		if d != nil && d.Kind == decision.KTarget {
			t.Fatalf("CR 603.3d: the granted trigger was placed with a target ask (%+v) despite an empty graveyard", d)
		}
		for _, id := range e.G.Stack {
			if o := e.G.Obj(id); o != nil && o.Source == tokenID && o.StackKind == state.StackKindTriggered {
				t.Fatalf("CR 603.3d: the granted trigger stayed on the stack with no legal target")
			}
		}
	})
	t.Run("mana value 6 Old One Eye keeps it on the stack", func(t *testing.T) {
		e, tokenID := scalelordTokenEngine(t, 612)
		ooe := moveByName(t, e, 0, "Old One Eye", state.ZGraveyard)
		if o := e.G.Obj(ooe); o == nil || o.Zone != state.ZGraveyard || o.Face().ManaValue() != 6 {
			t.Fatalf("precondition failed: Old One Eye %+v, want mana value 6 in the graveyard", o)
		}
		tok := e.G.Obj(tokenID)
		if tok.Face().Power()+int(tok.Counter("P1P1")) != 6 {
			t.Fatalf("precondition failed: token power %d+%d, want 6 so X equals Old One Eye's mana value",
				tok.Face().Power(), tok.Counter("P1P1"))
		}
		pushes, d := tokenAttackQueuesTheGrant(t, e, tokenID)
		if pushes == 0 {
			t.Fatal("precondition failed: the granted attack trigger never fired")
		}
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("the granted trigger must ask its target on placement (CR 603.3d), pending=%+v", d)
		}
		found := false
		for _, o := range d.Options {
			if o.Obj == ooe {
				found = true
			}
		}
		if !found {
			t.Fatalf("the cmcLEX offer missed Old One Eye (mana value 6, X = the token's power 6): options %+v", d.Options)
		}
	})
}
