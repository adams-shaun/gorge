package rules

// CR 508.4: Adeline, Resplendent Cathar's "for each opponent, create a 1/1
// white Human creature token that's tapped and attacking that player or a
// planeswalker they control" -- the token's controller chooses which, for each
// opponent, when that opponent controls a planeswalker.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const adelineWalkerSrc = "Name:Adeline Test Walker\nManaCost:3\nTypes:Planeswalker Jace\nLoyalty:3\nOracle:x\n"

// adelineWithWalker returns an engine where seat 0's Adeline trigger is on
// the stack with seat 1 controlling one planeswalker, and that walker's id.
func adelineWithWalker(t *testing.T) (*Engine, state.ObjID) {
	t.Helper()
	reg := searchTestRegistry(t)
	e, _ := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Adeline, Resplendent Cathar"), lookup(t, reg, "Grizzly Bears")},
		[]*cards.Card{})
	moveByName(t, e, 0, "Adeline, Resplendent Cathar", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	// Enter through MoveZone so the walker gets its printed loyalty and the
	// zero-loyalty state-based action leaves it on the battlefield.
	wo := e.G.AddObject(card(t, adelineWalkerSrc), 1)
	walker := wo.ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: walker, From: wo.Zone, To: state.ZBattlefield})
	if o := e.G.Obj(walker); o == nil || o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("precondition: walker = %+v, want battlefield under seat 1", o)
	}
	e.emit(events.Event{Kind: events.DeclareAttackers, Player: 1, IDs: []state.ObjID{bear}})
	e.putTriggersOnStack()
	if len(e.G.Stack) == 0 {
		t.Fatal("Adeline's AttackersDeclared trigger did not reach the stack")
	}
	return e, walker
}

// adelineDefenderAsk drains the stack until the defender ask is pending.
func adelineDefenderAsk(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatalf("no decision while draining (stack depth %d)", len(e.G.Stack))
		}
		if d.Kind == decision.KChoose && d.ResumeKind == "entering_defender" {
			return d
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision %+v", d)
		}
		submitChoices(t, e, 0)
		if len(e.G.Stack) == 0 {
			break
		}
	}
	t.Fatal("Adeline never asked which player or planeswalker the token attacks")
	return nil
}

func adelineHuman(t *testing.T, e *Engine) *state.Object {
	t.Helper()
	var tok *state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Human Token" {
			if tok != nil {
				t.Fatal("more than one Human token")
			}
			tok = o
		}
	}
	if tok == nil {
		t.Fatal("no Human token minted")
	}
	return tok
}

func TestAdelineAttacksPlaneswalker(t *testing.T) {
	t.Parallel()
	e, walker := adelineWithWalker(t)
	d := adelineDefenderAsk(t, e)
	if d.Player != 0 {
		t.Fatalf("ask routed to seat %d, want Adeline's controller 0", d.Player)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "player" || d.Options[0].Player != 1 ||
		d.Options[1].Obj != walker {
		t.Fatalf("options = %+v, want [player 1, walker %d]", d.Options, walker)
	}
	submitChoices(t, e, d.Options[1].Index)
	passUntilStackEmpty(t, e, 40)
	tok := adelineHuman(t, e)
	if !tok.Tapped || !tok.IsAttacking || tok.Attacking != 1 || tok.AttackingBattle != walker {
		t.Fatalf("token tapped=%v attacking=%v seat=%d at=%d, want tapped, attacking walker %d",
			tok.Tapped, tok.IsAttacking, tok.Attacking, tok.AttackingBattle, walker)
	}
	if hasNote(e, "planeswalker arm is not implemented") {
		t.Fatal("the modelled planeswalker arm still emits the unimplemented Note")
	}
}

func TestAdelineAttacksPlayerWhenChosen(t *testing.T) {
	t.Parallel()
	e, _ := adelineWithWalker(t)
	d := adelineDefenderAsk(t, e)
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 40)
	tok := adelineHuman(t, e)
	if !tok.IsAttacking || tok.Attacking != 1 || tok.AttackingBattle != 0 {
		t.Fatalf("token attacking=%v seat=%d at=%d, want attacking player 1", tok.IsAttacking, tok.Attacking, tok.AttackingBattle)
	}
}
