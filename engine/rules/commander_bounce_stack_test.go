package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	plainBounceFixtureSrc = `Name:Fixture Plain Bounce
ManaCost:0
Types:Instant
A:SP$ ChangeZone | Origin$ Battlefield | Destination$ Hand | ValidTgts$ Creature | SpellDescription$ Return target creature to its owner's hand.
Oracle:Return target creature to its owner's hand.
`
	// A bounce with a follow-up instruction: its controller loses 1 life.
	snagBounceFixtureSrc = `Name:Fixture Snag Bounce
ManaCost:0
Types:Instant
A:SP$ ChangeZone | Origin$ Battlefield | Destination$ Hand | ValidTgts$ Creature | SubAbility$ DBLose | SpellDescription$ Return target creature to its owner's hand. Its controller loses 1 life.
SVar:DBLose:DB$ LoseLife | Defined$ TargetedController | LifeAmount$ 1
Oracle:Return target creature to its owner's hand. Its controller loses 1 life.
`
	// An activated bounce: the ability, not a spell, is on the stack.
	bombBounceFixtureSrc = `Name:Fixture Bounce Bomb
ManaCost:0
Types:Artifact
A:AB$ ChangeZone | Cost$ Sac<1/CARDNAME> | Origin$ Battlefield | Destination$ Hand | ValidTgts$ Creature | SpellDescription$ Bounce target creature.
Oracle:Sacrifice: Return target creature to its owner's hand.
`
)

// pickBounceTarget answers the pending target ask with the option naming obj.
func pickBounceTarget(t *testing.T, e *Engine, obj state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %v, want the target ask", pendingKind(d))
	}
	for _, o := range d.Options {
		if o.Obj == obj {
			submit(t, e, o.Index)
			return
		}
	}
	t.Fatalf("object %d not offered as a target: %+v", obj, d.Options)
}

// TestCommanderBounceAsksWhileResolvingObjectIsOnStack pins CR 608.2n against
// CR 903.9b and CR 608.2c: the instructions of a resolving spell or ability
// are followed in order and the object leaves the stack only as the final part
// of its resolution, and the owner's command-zone choice is made when the
// bounce instruction would move their commander. So at the choice the spell
// (or the ability) is still on the stack and the instructions after the bounce
// have not run; all of that finishes once the choice is answered.
func TestCommanderBounceAsksWhileResolvingObjectIsOnStack(t *testing.T) {
	t.Parallel()
	for _, src := range []struct {
		name, card, obj string
		ability, loses  bool
	}{
		{"spell", plainBounceFixtureSrc, "Fixture Plain Bounce", false, false},
		{"spell then life loss", snagBounceFixtureSrc, "Fixture Snag Bounce", false, true},
		{"ability", bombBounceFixtureSrc, "Fixture Bounce Bomb", true, false},
	} {
		for _, tc := range []struct {
			name   string
			choice int
			want   state.Zone
		}{
			{"accept", 0, state.ZCommand},
			{"decline", 1, state.ZHand},
		} {
			t.Run(src.name+"/"+tc.name, func(t *testing.T) {
				e, _ := colourIdentityGame(t, 11, FormatCommander, card(t, tinyCmdSrc), nil, card(t, src.card))
				toMain1(t, e)
				cmd := fieldCommander(t, e, 0, 0)
				life := e.G.Players[0].Life
				var res state.ObjID
				if src.ability {
					res = moveToBattlefieldByName(t, e, 0, src.obj)
					e.priorityRound()
					act := -1
					for _, o := range e.Pending().Options {
						if o.Kind == "ability" && o.Obj == res {
							act = o.Index
						}
					}
					if act < 0 {
						t.Fatalf("%s: ability not offered", src.obj)
					}
					submit(t, e, act)
				} else {
					res = moveToHandByName(t, e, 0, src.obj)
					e.priorityRound()
					opt := castByName(t, e, 0, src.obj)
					if opt == nil {
						t.Fatalf("%s not castable", src.obj)
					}
					submit(t, e, opt.Index)
				}
				pickBounceTarget(t, e, cmd)
				d := passPriorityUntilNonPriority(t, e)
				if d.Kind != decision.KCommanderZone {
					t.Fatalf("pending after the bounce = %v, want the commander-zone choice", pendingKind(d))
				}
				if got := e.G.Obj(cmd).Zone; got != state.ZBattlefield {
					t.Fatalf("commander zone before the answer = %v, want battlefield (the move is parked)", got)
				}
				onStack := false
				for _, id := range e.G.Stack {
					if id == res || e.G.Obj(id).Source == res {
						onStack = true
					}
				}
				if !onStack {
					t.Fatalf("the resolving object left the stack before the choice was answered (CR 608.2n); stack %v", e.G.Stack)
				}
				if src.loses && e.G.Players[0].Life != life {
					t.Fatalf("life before the answer = %d, want %d: the instruction after the bounce ran early (CR 608.2c)",
						e.G.Players[0].Life, life)
				}
				submit(t, e, tc.choice)
				if got := e.G.Obj(cmd).Zone; got != tc.want {
					t.Fatalf("commander zone after answer = %v, want %v", got, tc.want)
				}
				if len(e.G.Stack) != 0 {
					t.Fatalf("stack after the answer = %v, want empty", e.G.Stack)
				}
				if !src.ability {
					if got := e.G.Obj(res).Zone; got != state.ZGraveyard {
						t.Fatalf("spell zone after the answer = %v, want graveyard", got)
					}
				}
				if src.loses && e.G.Players[0].Life != life-1 {
					t.Fatalf("life after the answer = %d, want %d", e.G.Players[0].Life, life-1)
				}
			})
		}
	}
}
