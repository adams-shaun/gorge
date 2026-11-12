package rules

// CR 700.2a / 603.3c at a modal trigger's placement: a
// mode whose mandatory targets have no legal candidate can't be chosen, and a
// trigger left with fewer choosable modes than its minimum is removed from
// the stack. Ferocification (OTJ) with no creature: XMage's stack is empty at
// the next checkpoint; gorge posed the mode ask and then fizzled.

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const ferocificationSrc = "Name:Ferocification Fixture\nManaCost:2 R\nTypes:Enchantment\n" +
	"T:Mode$ Phase | Phase$ BeginCombat | ValidPlayer$ You | Execute$ OnCharm | TriggerZones$ Battlefield\n" +
	"SVar:OnCharm:DB$ Charm | Choices$ DBPump,DBPumpKW\n" +
	"SVar:DBPump:DB$ Pump | ValidTgts$ Creature.YouCtrl | NumAtt$ +2 | SpellDescription$ Pump mode.\n" +
	"SVar:DBPumpKW:DB$ Pump | ValidTgts$ Creature.YouCtrl | KW$ Menace & Haste | SpellDescription$ Keyword mode.\n" +
	"Oracle:x\n"

// triggerObjOf returns the ability object source's trigger minted, wherever
// it rests now (stack or, once removed, exile).
func triggerObjOf(t *testing.T, e *Engine, source state.ObjID) *state.Object {
	t.Helper()
	var found *state.Object
	for i := range e.G.Objs {
		if o := &e.G.Objs[i]; o.Ability != nil && o.Source == source {
			found = o
		}
	}
	if found == nil {
		t.Fatalf("no ability object minted from %d", source)
	}
	return found
}

func TestModalTriggerWithNoTargetableModeIsRemoved(t *testing.T) {
	t.Parallel()
	e, cfg, _ := newFixtureDeck(t, 7101, ferocificationSrc)
	src := putCreature(t, e, 0, ferocificationSrc)
	if o := e.G.Obj(src); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("trigger source not on battlefield: %+v", o)
	}
	for _, o := range e.G.Objs {
		if o.Zone == state.ZBattlefield && o.Controller == 0 && o.Face() != nil && o.Face().IsCreature() {
			t.Fatalf("fixture precondition: seat 0 controls creature %q", o.Face().Name)
		}
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepBeginCombat})
	e.putTriggersOnStack()
	ab := triggerObjOf(t, e, src)
	if d := e.Pending(); d != nil && (d.Kind == decision.KModes || d.Kind == decision.KTarget) {
		t.Fatalf("posed %s for a trigger with no choosable mode: %+v", d.Kind, d)
	}
	if ab.Zone != state.ZExile {
		t.Fatalf("modeless trigger zone = %s, want removed (exile)", ab.Zone)
	}
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack = %v, want empty", e.G.Stack)
	}
	replayCheck(t, e, cfg)
}

// TestOptionalModalTriggerWithNoTargetableModeIsRemoved: a "choose up to
// one" (Optional$, minimum 0) modal trigger whose every mode needs a target
// on an empty board has no choosable mode; it is removed, never posing a
// zero-option mode ask (which the asking layer rejects with a panic).
func TestOptionalModalTriggerWithNoTargetableModeIsRemoved(t *testing.T) {
	t.Parallel()
	src := strings.Replace(ferocificationSrc, "Choices$ DBPump,DBPumpKW",
		"Choices$ DBPump,DBPumpKW | Optional$ True", 1)
	e, cfg, _ := newFixtureDeck(t, 7103, src)
	s := putCreature(t, e, 0, src)
	face := e.G.Obj(s).Face()
	charm := cards.ResolveSVar(face.SVars, "OnCharm")
	if charm == nil || !effects.CharmOf(charm).OptionalTrue {
		t.Fatalf("fixture precondition: OnCharm not Optional$: %+v", charm)
	}
	for _, m := range []string{"DBPump", "DBPumpKW"} {
		if n := len(e.legalTargetCandidates(0, s, s, cards.ResolveSVar(face.SVars, m))); n != 0 {
			t.Fatalf("fixture precondition: %s has %d targets, want zero", m, n)
		}
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepBeginCombat})
	e.putTriggersOnStack()
	ab := triggerObjOf(t, e, s)
	if d := e.Pending(); d != nil && (d.Kind == decision.KModes || d.Kind == decision.KTarget) {
		t.Fatalf("posed %s for a trigger with no choosable mode: %+v", d.Kind, d)
	}
	if ab.Zone != state.ZExile || len(e.G.Stack) != 0 {
		t.Fatalf("trigger zone = %s stack = %v, want removed (exile), empty stack", ab.Zone, e.G.Stack)
	}
	replayCheck(t, e, cfg)
}

func TestModalTriggerOffersOnlyTargetableModes(t *testing.T) {
	t.Parallel()
	src := "Name:Half Targetable Charm\nManaCost:2 R\nTypes:Enchantment\n" +
		"T:Mode$ Phase | Phase$ BeginCombat | ValidPlayer$ You | Execute$ OnCharm | TriggerZones$ Battlefield\n" +
		"SVar:OnCharm:DB$ Charm | Choices$ DBArt,DBPump\n" +
		"SVar:DBArt:DB$ Destroy | ValidTgts$ Artifact | SpellDescription$ Artifact mode.\n" +
		"SVar:DBPump:DB$ Pump | ValidTgts$ Creature.YouCtrl | NumAtt$ +2 | SpellDescription$ Pump mode.\n" +
		"Oracle:x\n"
	bear := "Name:Modal Target Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 7102, src, bear)
	s := putCreature(t, e, 0, src)
	b := putCreature(t, e, 0, bear)
	face := e.G.Obj(s).Face()
	if n := len(e.legalTargetCandidates(0, s, s, cards.ResolveSVar(face.SVars, "DBArt"))); n != 0 {
		t.Fatalf("fixture precondition: %d artifact targets, want zero", n)
	}
	if n := len(e.legalTargetCandidates(0, s, s, cards.ResolveSVar(face.SVars, "DBPump"))); n == 0 {
		t.Fatal("fixture precondition: no creature target for the pump mode")
	}
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepBeginCombat})
	e.putTriggersOnStack()
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("placement = %+v, want a mode ask", d)
	}
	if len(d.Options) != 1 || !strings.Contains(d.Options[0].Label, "Pump") {
		t.Fatalf("mode options = %+v, want only the targetable pump mode", d.Options)
	}
	submitChoices(t, e, d.Options[0].Index)
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		submitChoices(t, e, d.Options[0].Index)
	}
	ab := triggerObjOf(t, e, s)
	if ab.Zone != state.ZStack || len(ab.ChosenModes) != 1 || ab.ChosenModes[0] != "DBPump" {
		t.Fatalf("trigger zone=%s modes=%v, want on stack with DBPump", ab.Zone, ab.ChosenModes)
	}
	if len(ab.Targets) != 1 || ab.Targets[0].Obj != b {
		t.Fatalf("trigger targets = %v, want the bear %d", ab.Targets, b)
	}
}

// TestModalTriggerTargetedModesCensus pins the corpus class the placement
// filter serves: triggered modal bodies (every Choices$ value resolves to an
// ability body, the askTriggerModes contract) with at least one mode carrying
// its own ValidTgts$. Counted per API so a new carrier family shows up here.
func TestModalTriggerTargetedModesCensus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	got := map[string]int{}
	ferocification := false
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			for _, tr := range f.Triggers {
				body := cards.ResolveSVar(f.SVars, tr.Params["Execute"])
				if body == nil || !effects.CharmOf(body).HasChoices {
					continue
				}
				modal, targeted := true, false
				for _, name := range effects.CharmOf(body).Modes {
					sub := cards.ResolveSVar(f.SVars, name)
					if sub == nil {
						modal = false
						break
					}
					if effects.TargetsOf(sub).Targeted() {
						targeted = true
					}
				}
				if modal && targeted {
					got[body.API]++
					if f.Name == "Ferocification" {
						ferocification = true
					}
				}
			}
		}
	}
	if !ferocification {
		t.Fatal("Ferocification missing from the census")
	}
	want := map[string]int{"Charm": 178}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if got[k] != want[k] {
			t.Errorf("%s: %d triggered modal bodies with a targeted mode, want %d", k, got[k], want[k])
		}
	}
	if len(got) != len(want) {
		t.Errorf("census APIs = %v, want %v", got, want)
	}
}

// TestFerocificationNoOwnCreatureRemovesTrigger is the Level B divergence
// itself, through the oracle scenario runner: Ferocification's controller has
// no creature (the only creature is the opponent's), so neither "target
// creature you control" mode can be chosen and the begin-combat trigger is
// removed -- XMage's stack is empty at the pass_to checkpoint.
func TestFerocificationNoOwnCreatureRemovesTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"ferocification-no-own-creature","cr":["700.2a","603.3c"],"why":"lb-modaltgt",` +
		`"setup":{"p0":{"battlefield":["Ferocification"]},"p1":{"battlefield":["Grizzly Bears"]}},` +
		`"steps":[{"op":"pass_to","seat":0,"step":"begin-combat"}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	snap := res.Snapshots[len(res.Snapshots)-1]
	if snap.Step != "begin-combat" {
		t.Fatalf("snapshot step = %q, want begin-combat", snap.Step)
	}
	if len(snap.Stack) != 0 {
		t.Fatalf("begin-combat stack = %+v, want empty (trigger removed)", snap.Stack)
	}
	for _, d := range res.Decisions {
		if d.Kind == "mode" || d.Kind == "target" {
			t.Fatalf("posed a %s decision for a trigger with no choosable mode", d.Kind)
		}
	}
}
