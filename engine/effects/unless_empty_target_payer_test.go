package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// starAthleteAttack seats the real Star Athlete (seat 0) and a Grizzly Bears
// (seat 1), and returns its attack trigger's head SA: the script's TargetOnly
// over an up-to-one slot, chained to a DealDamage "unless its controller
// sacrifices it".
func starAthleteAttack(t *testing.T) (h *fakeHost, athlete, bears state.ObjID, head *cards.SA) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	h = newHost(t, 2)
	athlete = corpusObjectInSeat(t, reg, h.g, "Star Athlete", 0, state.ZBattlefield).ID
	bears = corpusObjectInSeat(t, reg, h.g, "Grizzly Bears", 1, state.ZBattlefield).ID
	for _, tr := range h.g.Obj(athlete).Face().Triggers {
		if tr.Mode == "Attacks" {
			head = tr.Effect
		}
	}
	if head == nil || head.Sub == nil || head.Sub.API != "DealDamage" || !ActivationOf(head.Sub).Unless() {
		t.Fatalf("precondition: Star Athlete's attack trigger must chain into a DealDamage with an UnlessCost$: head %+v", head)
	}
	return h, athlete, bears, head
}

// resolveOffered resolves head as the engine resolves a placed ability whose
// placement ask covered head and was answered with targets.
func resolveOffered(h *fakeHost, source state.ObjID, head *cards.SA, targets []state.Target) {
	c := NewCtxPtr(source, 0, CtxInit{Targets: targets, SVars: h.g.Obj(source).Face().SVars})
	c.TargetsOffered, c.OfferedSA = true, head
	Resolve(h, c, head)
}

// anyDamage counts the log's Damage events, against players and objects.
func anyDamage(h *fakeHost) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.Damage {
			n++
		}
	}
	return n
}

// TestUnlessEmptyTargetAsksNobody: with no target chosen for an up-to-one slot
// (CR 115.6) there is no "its controller" to take the CR 118.12 election, and
// CR 608.2d offers nobody an impossible choice: nobody is asked.
func TestUnlessEmptyTargetAsksNobody(t *testing.T) {
	h, athlete, _, head := starAthleteAttack(t)
	resolveOffered(h, athlete, head, nil)
	if h.askCount != 0 {
		t.Fatalf("asked %d time(s), last of seat %d (%q); want no ask over an empty target",
			h.askCount, h.lastAsk.Player, h.lastAsk.Prompt)
	}
	if n := anyDamage(h); n != 0 {
		t.Fatalf("%d damage event(s) with no target; want none", n)
	}
}

// TestUnlessChosenTargetAsksItsController is the control: the same chain with a
// target asks that permanent's controller, and the declined election deals the
// damage to them.
func TestUnlessChosenTargetAsksItsController(t *testing.T) {
	h, athlete, bears, head := starAthleteAttack(t)
	resolveOffered(h, athlete, head, []state.Target{{Obj: bears}})
	if h.askCount != 1 || h.lastAsk.Player != 1 {
		t.Fatalf("askCount=%d lastAsk=%+v; want one ask of seat 1", h.askCount, h.lastAsk)
	}
	if n := anyDamage(h); n != 1 {
		t.Fatalf("%d damage event(s) after the declined election; want 1", n)
	}
}

// TestUnlessEmptyHeadSlotAsksNobody is the same rule on a head that carries the
// UnlessCost$ over its own up-to slot, and the control that an ability with no
// target slot at all still asks the resolving controller.
func TestUnlessEmptyHeadSlotAsksNobody(t *testing.T) {
	card := mkCard(t, "Name:Head Probe\nTypes:Creature\nPT:1/1\n"+
		"A:AB$ GainLife | Cost$ T | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 1 | Defined$ You | LifeAmount$ 1 | UnlessCost$ 1\n"+
		"A:AB$ GainLife | Cost$ T | Defined$ You | LifeAmount$ 1 | UnlessCost$ 1\n"+
		"Oracle:x\n")
	for _, tc := range []struct {
		name    string
		ability int
		asks    int
	}{
		{name: "empty own slot", ability: 0, asks: 0},
		{name: "no slot at all", ability: 1, asks: 1},
	} {
		h := newHost(t, 2)
		src := h.g.AddObject(card, 0)
		src.Zone = state.ZBattlefield
		sa := card.Faces[0].Abilities[tc.ability]
		if got := TargetsOf(sa).Targeted(); got != (tc.ability == 0) {
			t.Fatalf("%s: precondition: Targeted() = %v", tc.name, got)
		}
		c := NewCtxPtr(src.ID, 0, CtxInit{})
		if TargetsOf(sa).Targeted() {
			c.TargetsOffered, c.OfferedSA = true, sa
		}
		Resolve(h, c, sa)
		if h.askCount != tc.asks {
			t.Errorf("%s: asked %d time(s), want %d", tc.name, h.askCount, tc.asks)
		}
	}
}
