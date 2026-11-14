package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// driveTriggerChain answers e's decisions until the stack is empty: priority
// passes, a yes/no takes its first ("Yes") option, and a target ask takes
// pick(d) (the test's chosen option index, or -1 to fail). It returns every
// target ask it answered, in order, with the stack depth it was posed at.
func driveTriggerChain(t *testing.T, e *Engine, pick func(d *decision.Decision) int) []decision.Decision {
	t.Helper()
	var asked []decision.Decision
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil || e.G.Over {
			t.Fatalf("game stopped mid-drive: pending=%+v", d)
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 && len(asked) > 0 {
				return asked
			}
			submitChoicePass(t, e)
			continue
		}
		if d.Kind == decision.KTarget {
			asked = append(asked, *d)
			idx := pick(d)
			if idx < 0 {
				t.Fatalf("unexpected target ask %+v", d)
			}
			submitChoices(t, e, idx)
			continue
		}
		submitChoices(t, e, 0)
	}
	t.Fatalf("stack never emptied: %+v pending=%+v", e.G.Stack, e.Pending())
	return nil
}

// TestIrohTeaMasterGivesTheChainTarget is lb-chainsubtgt's repro (TLA
// Iroh, Tea Master trigger#0.1): "you may have target opponent gain control
// of target permanent you control". The second target lives on the
// SubAbility$ link `DB$ GainControl | ValidTgts$ Permanent.YouCtrl |
// Defined$ Targeted`. CR 603.3d: it is chosen as the trigger is put on the
// stack (a trig_sub placement ask), and the GainControl hands exactly that
// permanent to the opponent; the reflexive "when you do" then makes the
// Ally with one +1/+1 counter.
func TestIrohTeaMasterGivesTheChainTarget(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Setup: map[string]oracleSeat{
			"p0": {Battlefield: []string{"Iroh, Tea Master", "Ornithopter"}},
			"p1": {Battlefield: []string{"Grizzly Bears"}},
		},
		Steps: []oracleStep{{Op: "pass_to", Seat: 0, Step: "begin-combat"}},
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 || run.e == nil {
		t.Fatalf("%v\ntranscript:\n%v", fails, transcript)
	}
	e := run.e
	thopter, err := run.resolve("p0:Ornithopter")
	if err != nil {
		t.Fatal(err)
	}
	iroh, err := run.resolve("p0:Iroh, Tea Master")
	if err != nil {
		t.Fatal(err)
	}
	if o := e.G.Obj(thopter); o == nil || o.Zone != state.ZBattlefield || o.Controller != 0 {
		t.Fatalf("precondition: Ornithopter on p0's battlefield, got %+v", o)
	}
	if n := len(e.G.Stack); n == 0 || e.G.Obj(e.G.Stack[n-1]).Ability == nil {
		t.Fatalf("precondition: Iroh's begin-combat trigger on the stack, got %+v", e.G.Stack)
	}
	trigger := e.G.Stack[len(e.G.Stack)-1]
	asked := driveTriggerChain(t, e, func(d *decision.Decision) int {
		if d.ResumeKind == "trig_sub" {
			return indexOfObjOption(d, thopter)
		}
		for i, o := range d.Options {
			if o.Kind == "player" && o.Player == 1 {
				return i
			}
		}
		return -1
	})
	if len(asked) != 2 || asked[1].ResumeKind != "trig_sub" || asked[1].Source != trigger {
		t.Fatalf("target asks = %+v, want the opponent then the chained permanent at placement", asked)
	}
	if indexOfObjOption(&asked[1], iroh) < 0 || indexOfObjOption(&asked[1], thopter) < 0 {
		t.Fatalf("chained ask offers %+v, want both permanents p0 controls", asked[1].Options)
	}
	if o := e.G.Obj(thopter); o.Zone != state.ZBattlefield || o.Controller != 1 {
		t.Fatalf("Ornithopter controller=%d zone=%s after the trigger, want controlled by p1", o.Controller, o.Zone)
	}
	if o := e.G.Obj(iroh); o.Controller != 0 {
		t.Fatalf("Iroh changed controller to %d; only the chosen permanent moves", o.Controller)
	}
	allies := 0
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if f := o.Face(); o.Zone == state.ZBattlefield && o.IsToken && o.Controller == 0 &&
			f != nil && strings.Contains(f.Name, "Ally") {
			allies++
			if got := counterCount(o, "P1P1"); got != 1 {
				t.Fatalf("Ally token has %d +1/+1 counters, want 1 (one permanent p0 owns that p1 controls)", got)
			}
		}
	}
	if allies != 1 {
		t.Fatalf("Ally tokens = %d, want 1 from the reflexive trigger", allies)
	}
	replayCheck(t, e, run.cfg)
}

// TestVedalkenPlotterExchangesTheChainTarget is the ExchangeControl sibling
// of the same shape: an ExchangeControl of the parent's target with a
// targeted land an opponent controls. The opponent's land is asked at placement and
// the two lands swap controllers.
func TestVedalkenPlotterExchangesTheChainTarget(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := oracleScenario{
		Setup: map[string]oracleSeat{
			"p0": {Hand: []string{"Vedalken Plotter"}, Battlefield: []string{"Forest"}},
			"p1": {Battlefield: []string{"Island"}},
		},
		Steps: []oracleStep{{Op: "cast", Seat: 0, Card: "p0:Vedalken Plotter", Mana: "UUU"}},
	}
	fails, transcript, run := runOracleScenario(reg, sc)
	if len(fails) > 0 || run.e == nil {
		t.Fatalf("%v\ntranscript:\n%v", fails, transcript)
	}
	e := run.e
	forest, err := run.resolve("p0:Forest")
	if err != nil {
		t.Fatal(err)
	}
	island, err := run.resolve("p1:Island")
	if err != nil {
		t.Fatal(err)
	}
	if e.G.Obj(forest).Controller != 0 || e.G.Obj(island).Controller != 1 {
		t.Fatal("precondition: Forest controlled by p0, Island by p1")
	}
	asked := driveTriggerChain(t, e, func(d *decision.Decision) int {
		if d.ResumeKind == "trig_sub" {
			return indexOfObjOption(d, island)
		}
		return indexOfObjOption(d, forest)
	})
	if len(asked) != 2 || asked[1].ResumeKind != "trig_sub" {
		t.Fatalf("target asks = %+v, want the own land then the opponent's land at placement", asked)
	}
	if c := e.G.Obj(forest).Controller; c != 1 {
		t.Fatalf("Forest controller = %d after the exchange, want 1", c)
	}
	if c := e.G.Obj(island).Controller; c != 0 {
		t.Fatalf("Island controller = %d after the exchange, want 0", c)
	}
	changes := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.ControlChange {
			changes++
		}
	}
	if changes != 2 {
		t.Fatalf("ControlChange events = %d, want 2", changes)
	}
	replayCheck(t, e, run.cfg)
}

// TestTriggerChainLinkPoolReadsTheRootTarget: a placement link whose pool
// names an earlier target (Glen Elendra's `ControlledBy ... ParentTarget`
// shape; here `Creature.TargetedPlayerCtrl`) is offered against the root's
// announced target, as the cast flow's subTargetAsk binds it -- not failed
// closed to an empty pool that would counter the trigger.
func TestTriggerChainLinkPoolReadsTheRootTarget(t *testing.T) {
	e := layerEngine(t)
	src := onBoard(t, e, 0,
		"Name:Chain Trigger\nTypes:Creature\nPT:1/1\n"+
			"T:Mode$ ChangesZone | ValidCard$ Card.Self | Destination$ Battlefield | Execute$ Root | TriggerDescription$ x\n"+
			"SVar:Root:DB$ Pump | ValidTgts$ Player | SubAbility$ Link\n"+
			"SVar:Link:DB$ Pump | ValidTgts$ Creature.TargetedPlayerCtrl\nOracle:x\n")
	theirs := onBoard(t, e, 1, "Name:Their Bear\nTypes:Creature\nPT:2/2\nOracle:x\n")
	o := e.G.Obj(src)
	if o == nil || o.Face() == nil || len(o.Face().Triggers) != 1 || o.Face().Triggers[0].Effect == nil {
		t.Fatalf("precondition: printed trigger compiled, got %+v", o)
	}
	root := o.Face().Triggers[0].Effect
	if root.Sub == nil || root.Sub.Params["ValidTgts"] != "Creature.TargetedPlayerCtrl" {
		t.Fatalf("precondition: a root-relative chain link, got %+v", root.Sub)
	}
	e.pushTrigger(pendingTrigger{Source: src, Controller: 0, Idx: 0, SA: root})
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind == "trig_sub" {
		t.Fatalf("root ask = %+v, want the player target first", d)
	}
	pick := -1
	for i, opt := range d.Options {
		if opt.Kind == "player" && opt.Player == 1 {
			pick = i
		}
	}
	if pick < 0 {
		t.Fatalf("root ask offers %+v, want player 1", d.Options)
	}
	submitChoices(t, e, pick)
	for _, ev := range e.L.Events {
		if ev.Text == "countered: no legal targets" {
			t.Fatal("the link's root-relative pool failed closed and countered the trigger")
		}
	}
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "trig_sub" {
		t.Fatalf("chain ask = %+v, want a placement trig_sub", d)
	}
	if len(d.Options) != 1 || indexOfObjOption(d, theirs) != 0 {
		t.Fatalf("chain ask offers %+v, want only player 1's creature %d", d.Options, theirs)
	}
}
