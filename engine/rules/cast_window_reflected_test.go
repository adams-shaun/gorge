package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// reflectedDownsize builds the Star Compass fixture: Downsize in seat 0's
// hand, a Bear on seat 1's battlefield, seat 0's lands (Savannah, two
// Plains, two of second) all tapped and Star Compass the only untapped
// source, so the Compass reflects {W} plus second's colour.
func reflectedDownsize(t *testing.T, second string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := handEngine(t, choiceCard(t, "Downsize"))
	down := e.G.Zone(state.ZHand, 0)[0]
	bear := e.G.AddObject(card(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 1)
	bear.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 1, append(e.G.Zone(state.ZBattlefield, 1), bear.ID))
	for _, n := range []string{"Savannah", "Plains", "Plains", second, second} {
		id := onBoardCard(t, e, 0, choiceCard(t, n))
		e.G.Obj(id).Tapped = true
	}
	compass := onBoardCard(t, e, 0, choiceCard(t, "Star Compass"))
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.Tapped != (id != compass) {
			t.Fatalf("precondition: %s tapped=%v", o.Face().Name, o.Tapped)
		}
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: floating pool %v, want empty", e.G.Players[0].Pool)
	}
	return e, down, bear.ID
}

// walkToTarget announces Downsize and answers the mana window (Star
// Compass, then its colour ask, picking U when offered) until the cast
// reaches its target ask or leaves the pending cast.
func walkToTarget(t *testing.T, e *Engine, down state.ObjID) *decision.Decision {
	t.Helper()
	e.priorityRound()
	d, a := announceAction(t, e, down)
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Announce: &decision.AnnounceSelection{ActionID: a.ID}}); err != nil {
		t.Fatalf("Submit announce: %v", err)
	}
	for step := 0; step < 20; step++ {
		w := e.Pending()
		if w == nil || w.Kind != decision.KChoose {
			return w
		}
		var pick *decision.Option
		for i := range w.Options {
			o := &w.Options[i]
			if o.ManaSymbol == "U" {
				pick = o
				break
			}
		}
		for i := range w.Options {
			if pick == nil && (w.Options[i].Kind == "autofill" || w.Options[i].Kind == "mana") {
				pick = &w.Options[i]
			}
		}
		if pick == nil {
			t.Logf("window offers nothing to answer: %#v", w.Options)
			return w
		}
		apAnswer(t, e, *pick)
	}
	t.Fatal("mana window walk did not terminate")
	return nil
}

func spellReturnedToHand(e *Engine, id state.ObjID) bool {
	for _, ev := range e.L.Events {
		if ev.Kind == events.MoveZone && ev.Obj == id && ev.To == state.ZHand {
			return true
		}
	}
	return false
}

// TestReflectedWindowCastReachesTarget pins the cast-window census pricing a
// reflected source as its reflected colours: Star Compass reflecting {W}/{U}
// pays Downsize's {U}, so the target ask offers the Bear rather than the
// gate dropping it and aborting the cast.
func TestReflectedWindowCastReachesTarget(t *testing.T) {
	e, down, bear := reflectedDownsize(t, "Island")
	d := walkToTarget(t, e, down)
	if hasNote(e, "cast aborted") || spellReturnedToHand(e, down) {
		t.Fatalf("cast aborted before payment; pending = %#v", d)
	}
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %#v, want the target ask", d)
	}
	var opt *decision.Option
	for i := range d.Options {
		if d.Options[i].Obj == bear {
			opt = &d.Options[i]
		}
	}
	if opt == nil {
		t.Fatalf("target ask does not offer the Bear: %#v", d.Options)
	}
	apAnswer(t, e, *opt)
	for step := 0; step < 10 && e.G.Obj(down).Zone != state.ZStack; step++ {
		w := e.Pending()
		if w == nil || w.Kind != decision.KChoose || len(w.Options) == 0 {
			break
		}
		apAnswer(t, e, w.Options[0])
	}
	if z := e.G.Obj(down).Zone; z != state.ZStack {
		t.Fatalf("Downsize zone = %v, want stack; pending = %#v", z, e.Pending())
	}
	if hasNote(e, "cast aborted") || spellReturnedToHand(e, down) {
		t.Fatal("cast aborted after the target ask")
	}
}

// TestReflectedWindowDoesNotOverpromise is the fail-closed boundary: Star
// Compass reflecting only {W}/{R} cannot pay {U}. The reflected pass prices
// it as exactly {W} and {R} (never {U}, never the {C} stand-in), and the
// planner offers no Downsize payment action, so the census and the plan
// agree that the cast is unaffordable.
func TestReflectedWindowDoesNotOverpromise(t *testing.T) {
	e, down, _ := reflectedDownsize(t, "Mountain")
	compass := e.G.Zone(state.ZBattlefield, 0)[5]
	var got []state.Mana
	for _, u := range e.castWindowReflectedUnits(&pendingCast{player: 0}, e.windowManaUnits(0)) {
		if u.ID == compass {
			for _, a := range u.Alts {
				got = append(got, a.Mana())
			}
		}
	}
	want := []state.Mana{{1, 0, 0, 0, 0, 0}, {0, 0, 0, 1, 0, 0}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("Star Compass alts = %v, want %v ({W}, {R})", got, want)
	}
	e.priorityRound()
	for _, a := range e.EnsurePaymentActions() {
		if a.Cast.Object == down {
			t.Fatalf("planner offers Downsize %#v; {W}/{R} cannot pay {U}", a)
		}
	}
	if hasNote(e, "cast aborted") {
		t.Fatal("unexpected cast abort")
	}
}
