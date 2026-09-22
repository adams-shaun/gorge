package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestFerraforETBCreatesSaprolingsForTargetPlayersCreatureCounters(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	ferrafor, ok := reg.Lookup("Ferrafor, Young Yew")
	if !ok {
		t.Fatal("corpus has no Ferrafor, Young Yew")
	}

	e := corpusEngine(t, reg, []*cards.Card{ferrafor}, nil)
	controllerCreature := onBoard(t, e, 0, "Name:Controller Counterbear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	targetCreatureA := onBoard(t, e, 1, "Name:Target Counterbear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	targetCreatureB := onBoard(t, e, 1, "Name:Target Counterwolf\nTypes:Creature Wolf\nPT:2/2\nOracle:x\n")
	seedObjectCounter(t, e, controllerCreature, "P1P1", 2)
	seedObjectCounter(t, e, targetCreatureA, "P1P1", 2)
	seedObjectCounter(t, e, targetCreatureA, "LORE", 1)
	seedObjectCounter(t, e, targetCreatureB, "TIME", 2)

	if len(ferrafor.Faces) != 1 {
		t.Fatalf("test precondition: Ferrafor faces = %d, want 1", len(ferrafor.Faces))
	}
	face := ferrafor.Faces[0]
	if len(face.Triggers) != 1 {
		t.Fatalf("test precondition: Ferrafor triggers = %d, want 1", len(face.Triggers))
	}
	trig := face.Triggers[0]
	if trig.Mode != "ChangesZone" || trig.Params["Origin"] != "Any" ||
		trig.Params["Destination"] != "Battlefield" || trig.Params["ValidCard"] != "Card.Self" ||
		trig.Params["Execute"] != "TrigToken" {
		t.Fatalf("test precondition: Ferrafor trigger = %+v", trig)
	}
	tokenSA := cards.ResolveSVar(face.SVars, "TrigToken")
	if tokenSA == nil || tokenSA.API != "Token" ||
		tokenSA.Params["TokenAmount"] != "Count$Valid Creature.ControlledBy TargetedPlayer$CardCounters.ALL" ||
		tokenSA.Params["ValidTgts"] != "Player" || tokenSA.Params["TokenScript"] != "g_1_1_saproling" {
		t.Fatalf("test precondition: Ferrafor TrigToken = %+v", tokenSA)
	}
	if got := e.G.Obj(controllerCreature).Counter("P1P1"); got != 2 {
		t.Fatalf("test precondition: controller counters = %d, want 2", got)
	}
	if got := e.G.Obj(targetCreatureA).Counter("P1P1") + e.G.Obj(targetCreatureA).Counter("LORE") + e.G.Obj(targetCreatureB).Counter("TIME"); got != 5 {
		t.Fatalf("test precondition: target counters = %d, want 5", got)
	}
	if e.G.Obj(controllerCreature).Counter("P1P1") == 5 {
		t.Fatal("test precondition: controller and target totals must differ")
	}

	ferraforID := moveByName(t, e, 0, "Ferrafor, Young Yew", state.ZBattlefield)
	if e.G.Obj(ferraforID).Zone != state.ZBattlefield || len(e.pendingTriggers) != 1 {
		t.Fatalf("test precondition: Ferrafor ETB zone=%s pending triggers=%d, want battlefield/1", e.G.Obj(ferraforID).Zone, len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("Ferrafor ETB pending = %+v, want player target", d)
	}
	idx := -1
	for _, option := range d.Options {
		if option.Kind == "player" && option.Player == 1 {
			idx = option.Index
			break
		}
	}
	if idx < 0 {
		t.Fatalf("Ferrafor target decision did not offer seat 1: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	e.resolveTop()

	created := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.TokenCreate && ev.Player == 0 && ev.Text == "g_1_1_saproling" {
			created++
		}
	}
	if created != 5 {
		t.Fatalf("Ferrafor created %d Saproling TokenCreate events, want 5", created)
	}
	tokens := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o.IsToken && o.Face() != nil && o.Face().Name == "Saproling Token" {
			tokens++
			if o.Owner != 0 || o.Controller != 0 {
				t.Fatalf("Saproling token %d owner/controller = %d/%d, want 0/0", id, o.Owner, o.Controller)
			}
		}
	}
	if tokens != 5 {
		t.Fatalf("seat 0 battlefield has %d Saproling tokens, want 5", tokens)
	}
}
