package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// notOfThisWorldOffers builds the Not of This World fixture: seat 0 holds the
// counter, seat 0 and seat 1 each control a Grizzly Bears, and mint puts one
// ability aimed at the bear it is handed on the stack. It returns the ability
// object and the counter's candidate list.
func notOfThisWorldOffers(t *testing.T, aimAtOpp, triggered bool) (abID state.ObjID, offered []targetCandidate) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	nott := lookup(t, reg, "Not of This World")
	bears := lookup(t, reg, "Grizzly Bears")
	sorc := lookup(t, reg, "Prodigal Sorcerer")
	mow := lookup(t, reg, "Man-o'-War")
	e := corpusEngine(t, reg, []*cards.Card{nott, bears, sorc, mow}, []*cards.Card{bears})
	var counter state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		for _, id := range e.G.Zone(z, 0) {
			if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Not of This World" {
				counter = id
			}
		}
	}
	if counter == 0 {
		t.Fatal("fixture: Not of This World is in neither hand nor library")
	}
	mine := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	theirs := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	aim := mine
	if aimAtOpp {
		aim = theirs
	}
	if o := e.G.Obj(aim); o == nil || o.Zone != state.ZBattlefield || o.Controller != map[bool]state.PlayerID{false: 0, true: 1}[aimAtOpp] {
		t.Fatalf("precondition: aimed bear %d is not on its controller's battlefield: %+v", aim, o)
	}
	if triggered {
		moveByName(t, e, 0, "Man-o'-War", state.ZBattlefield)
		d := drainToTargetAsk(t, e, 40)
		if d.Kind != decision.KTarget {
			t.Fatalf("Man-o'-War's enters trigger did not ask for a target: %+v", d)
		}
		idx := indexOfObjOption(d, aim)
		if idx < 0 {
			t.Fatalf("Man-o'-War's ask does not offer bear %d: %+v", aim, d.Options)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("choose the bear: %v", err)
		}
	} else {
		src := moveByName(t, e, 0, "Prodigal Sorcerer", state.ZBattlefield)
		e.emit(events.Event{Kind: events.AbilityPush, Obj: src, Player: 0, Amount: 0})
		st := e.G.Zone(state.ZStack, 0)
		if len(st) == 0 {
			t.Fatal("precondition: the activated ability is not on the stack")
		}
		e.emit(events.Event{Kind: events.TargetsChosen, Obj: st[len(st)-1], Player: 0, IDs: []state.ObjID{aim}})
	}
	st := e.G.Zone(state.ZStack, 0)
	if len(st) == 0 {
		t.Fatal("precondition: nothing on the stack")
	}
	abID = st[len(st)-1]
	ab := e.G.Obj(abID)
	if ab == nil || len(ab.Targets) != 1 || ab.Targets[0].Obj != aim {
		t.Fatalf("precondition: the stack ability %+v does not carry the bear %d as its target", ab, aim)
	}
	sa := e.G.Obj(counter).Face().SpellAbility()
	if sa == nil {
		t.Fatal("Not of This World has no spell ability")
	}
	return abID, e.legalTargetCandidates(0, counter, counter, sa)
}

func hasCandidate(c []targetCandidate, id state.ObjID) bool {
	for _, x := range c {
		if x.obj == id {
			return true
		}
	}
	return false
}

// TestTargetValidTargetingOffersStackAbilities pins that an activated and a
// triggered ability on the stack are candidates of a TargetValidTargeting$
// counter exactly when their own targets match the clause: aimed at the
// counter's controller's creature they are offered, aimed at the opponent's
// they are not.
func TestTargetValidTargetingOffersStackAbilities(t *testing.T) {
	t.Parallel()
	for _, triggered := range []bool{false, true} {
		name := "activated"
		if triggered {
			name = "triggered"
		}
		t.Run(name+"/yours", func(t *testing.T) {
			ab, got := notOfThisWorldOffers(t, false, triggered)
			if !hasCandidate(got, ab) {
				t.Fatalf("%s ability %d aimed at your creature not offered: %+v", name, ab, got)
			}
		})
		t.Run(name+"/theirs", func(t *testing.T) {
			ab, got := notOfThisWorldOffers(t, true, triggered)
			if hasCandidate(got, ab) {
				t.Fatalf("%s ability %d aimed at the opponent's creature offered: %+v", name, ab, got)
			}
		})
	}
}
