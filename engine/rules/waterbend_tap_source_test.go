package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// patrolBoard stands up North Pole Patrol on seat 0 with `fillers` untapped
// artifact creatures, an empty pool, and a Grizzly Bears on seat 1.
func patrolBoard(t *testing.T, fillers int) (*Engine, state.ObjID, []state.ObjID, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, _ := searchEngine(t, reg, "North Pole Patrol")
	patrol := searchMoveByName(t, e, "North Pole Patrol", state.ZBattlefield)
	e.G.Obj(patrol).SummonSick = false
	var ids []state.ObjID
	for i := 0; i < fillers; i++ {
		ids = append(ids, onBoardReady(t, e, 0, "Name:Ornithopter\nManaCost:0\nTypes:Artifact Creature Thopter\nPT:0/2\nOracle:x\n"))
	}
	bears := onBoardReady(t, e, 1, "Name:Bear\nManaCost:0\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.priorityRound()
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool must be empty: got %d", got)
	}
	if o := e.G.Obj(patrol); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: Patrol must be an untapped battlefield permanent: %+v", o)
	}
	return e, patrol, ids, bears
}

// patrolWaterbendAbility finds the Patrol's `Tap target creature an opponent
// controls` ability (its Waterbend<3> T cost) among the pending options.
func patrolWaterbendAbility(e *Engine, patrol state.ObjID) (decision.Option, bool) {
	d := e.Pending()
	if d == nil {
		return decision.Option{}, false
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == patrol && strings.Contains(o.Label, "Tap target creature") {
			return o, true
		}
	}
	return decision.Option{}, false
}

// patrolToWaterbendAsk activates the Patrol's ability and answers its target prompt
// until the waterbend announcement is pending.
func patrolToWaterbendAsk(t *testing.T, e *Engine, patrol, bears state.ObjID) *decision.Decision {
	t.Helper()
	o, ok := patrolWaterbendAbility(e, patrol)
	if !ok {
		t.Fatal("Patrol's waterbend ability not offered")
	}
	submitChoices(t, e, o.Index)
	for i := 0; i < 6; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision pending before the waterbend ask")
		}
		for _, op := range d.Options {
			if op.Kind == "waterbend_generic" {
				return d
			}
		}
		pick := 0
		for j, op := range d.Options {
			if op.Obj == bears {
				pick = j
			}
		}
		submitChoices(t, e, d.Options[pick].Index)
	}
	t.Fatalf("never reached the waterbend ask: %+v", e.Pending())
	return nil
}

func TestSetAudit_tla_NorthPolePatrol_WaterbendExcludesTapSource(t *testing.T) {
	e, patrol, fillers, bears := patrolBoard(t, 3)
	d := patrolToWaterbendAsk(t, e, patrol, bears)
	var picks []int
	for _, o := range d.Options {
		if o.Obj == patrol {
			t.Fatalf("the Patrol's own {T} source is offered as a waterbend tap: %+v", o)
		}
		if o.Kind == "waterbend_generic" {
			picks = append(picks, o.Index)
		}
	}
	if len(picks) != len(fillers) {
		t.Fatalf("want one waterbend option per filler (%d), got %d: %+v", len(fillers), len(picks), d.Options)
	}
	submitChoices(t, e, picks...)
	if d := e.Pending(); d == nil || d.Kind != decision.KTarget || len(d.Options) != 1 || d.Options[0].Obj != bears {
		t.Fatalf("precondition: the Bear is the one target left to choose: %+v", d)
	}
	submitChoices(t, e, 0)
	if len(e.G.Stack) != 1 {
		t.Fatalf("the ability is not on the stack after payment: %d items", len(e.G.Stack))
	}
	for _, f := range fillers {
		if !e.G.Obj(f).Tapped {
			t.Fatalf("filler %d not tapped by the waterbend", f)
		}
	}
	if !e.G.Obj(patrol).Tapped {
		t.Fatal("the Patrol's own {T} did not tap it")
	}
	if n := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.Tap && ev.Obj == patrol }); n != 1 {
		t.Fatalf("Patrol tapped %d times, want exactly one", n)
	}
}

func TestSetAudit_tla_NorthPolePatrol_TwoFillersNotOffered(t *testing.T) {
	e, patrol, _, _ := patrolBoard(t, 2)
	if _, ok := patrolWaterbendAbility(e, patrol); ok {
		t.Fatal("Waterbend<3> T offered with only two other permanents to tap")
	}
	e3, patrol3, _, _ := patrolBoard(t, 3)
	if _, ok := patrolWaterbendAbility(e3, patrol3); !ok {
		t.Fatal("precondition: the same ability is offered with three fillers")
	}
}

func TestSetAudit_tla_NorthPolePatrol_IntentNamingSourceRejected(t *testing.T) {
	e, patrol, fillers, bears := patrolBoard(t, 3)
	d := patrolToWaterbendAsk(t, e, patrol, bears)
	// Forge the pre-fix option: the Patrol as a waterbend tap, spliced into
	// the pending decision the way an old option list carried it.
	forged := *d
	forged.Options = append(append([]decision.Option(nil), d.Options...), decision.Option{
		Index: len(d.Options), Kind: "waterbend_generic", Obj: patrol, Group: "payment:forged"})
	choices := []int{forged.Options[len(forged.Options)-1].Index}
	for _, o := range d.Options {
		if o.Obj == fillers[0] || o.Obj == fillers[1] {
			choices = append(choices, o.Index)
		}
	}
	err := e.validateCastContributions(&forged, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: choices})
	if err == nil {
		t.Fatal("an intent naming the {T} source as a waterbend tap was accepted")
	}
	// The legitimate three-filler answer still passes the same gate.
	var ok []int
	for _, o := range d.Options {
		ok = append(ok, o.Index)
	}
	if err := e.validateCastContributions(d, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ok}); err != nil {
		t.Fatalf("the legal three-filler answer was rejected: %v", err)
	}
}
