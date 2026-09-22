package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestGlavaFiveAdventsSpellAbilityCastSpellArm pins the real Glava trigger's
// spell half.  The same Mode$ SpellAbilityCast line also covers activated
// abilities; this case specifically reaches it through PutOnStack.
func TestGlavaFiveAdventsSpellAbilityCastSpellArm(t *testing.T) {
	e, cfg, _ := etbConfig(t, 131, []string{glavaSrc, xCostHydraSrc, nonXBeastSrc}, nil)
	glava := moveSeeded(t, e, 0, glavaSrc, state.ZBattlefield)
	if o := e.G.Obj(glava); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Glava precondition: zone = %v, want battlefield", o.Zone)
	}

	xSpell := moveSeeded(t, e, 0, xCostHydraSrc, state.ZHand)
	xFace := e.G.Obj(xSpell).Face()
	if xFace == nil || !strings.Contains(xFace.ManaCost, "X") {
		t.Fatalf("X-spell precondition: face = %+v, want printed X mana cost", xFace)
	}
	if !xFace.IsPermanent() {
		t.Fatalf("X-spell precondition: types = %v, want permanent", xFace.Types)
	}
	addMana(t, e, 0, "GGG")
	d := e.Pending()
	if d == nil {
		t.Fatal("no cast decision for X spell")
	}
	castIdx := -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == xSpell {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("X spell was not offered to cast: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{castIdx}}); err != nil {
		t.Fatalf("cast X spell: %v", err)
	}
	if d := e.Pending(); d == nil || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("X spell did not announce X: %+v", d)
	}
	submitChoices(t, e, 2)
	if o := e.G.Obj(xSpell); o == nil || o.Zone != state.ZStack {
		t.Fatalf("X permanent spell precondition: zone = %v, want stack", o.Zone)
	}
	foundPut := false
	for _, ev := range e.L.Events {
		if ev.Kind == events.PutOnStack && ev.Obj == xSpell {
			foundPut = true
			break
		}
	}
	if !foundPut {
		t.Fatalf("X permanent spell did not emit PutOnStack")
	}
	drainTriggerAsks(t, e, 30)
	if got := pushCount(e, glava); got != 1 {
		t.Fatalf("Glava TriggerPush count after X permanent spell = %d, want 1", got)
	}

	nonX := moveSeeded(t, e, 0, nonXBeastSrc, state.ZHand)
	nonXFace := e.G.Obj(nonX).Face()
	if nonXFace == nil || strings.Contains(nonXFace.ManaCost, "X") {
		t.Fatalf("non-X precondition: face = %+v, want a different printed mana cost", nonXFace)
	}
	if !nonXFace.IsPermanent() {
		t.Fatalf("non-X precondition: types = %v, want permanent", nonXFace.Types)
	}
	addMana(t, e, 0, "GG")
	d = e.Pending()
	if d == nil {
		t.Fatal("no cast decision for non-X spell")
	}
	castIdx = -1
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == nonX {
			castIdx = o.Index
		}
	}
	if castIdx < 0 {
		t.Fatalf("non-X spell was not offered to cast: %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{castIdx}}); err != nil {
		t.Fatalf("cast non-X spell: %v", err)
	}
	drainTriggerAsks(t, e, 30)
	if got := pushCount(e, glava); got != 1 {
		t.Fatalf("Glava TriggerPush count after non-X permanent spell = %d, want still 1", got)
	}
	replayCheck(t, e, cfg)
}
