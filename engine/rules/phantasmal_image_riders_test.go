package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPhantasmalImageETBCopyKeepsRiders casts the real Phantasmal Image and
// has it enter as a copy of Grizzly Bears: the AddTypes$ Illusion rider makes
// it an Illusion, and the AddTriggers$/AddSVars$ riders put the granted
// "becomes the target ... sacrifice it" trigger (and its Execute SVar) on the
// copy. Before the ETB whitelist admitted AddTriggers$/AddSVars$ the body
// took the loud "unimplemented API Clone" fallback and the Image entered
// 0/0 and died.
func TestPhantasmalImageETBCopyKeepsRiders(t *testing.T) {
	t.Parallel()
	e := goadGrantEngine(t, corpusAlternativeCard(t, "Phantasmal Image"))
	bear := onBoardReadyCard(t, e, 1, corpusAlternativeCard(t, "Grizzly Bears"))
	e.G.Players[0].Pool[state.MC] = 1
	e.G.Players[0].Pool[state.MU] = 1
	img := e.G.Zone(state.ZHand, 0)[0]

	castMode(t, e, img, "")
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("expected the ETB copy election (whitelist declined the riders?), got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("copy target not offered: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 8)

	o := e.G.Obj(img)
	if o == nil || o.Zone != state.ZBattlefield || o.Face().Name != "Grizzly Bears" {
		t.Fatalf("image = %+v, want a Grizzly Bears copy on the battlefield", o)
	}
	if !e.hasType(o, "Illusion") || !e.hasType(o, "Bear") {
		t.Fatal("copy must be an Illusion in addition to its other types (AddTypes$)")
	}
	if e.hasType(e.G.Obj(bear), "Illusion") {
		t.Fatal("the copied template must not gain Illusion")
	}
	// The riders are copiable values (CR 707.9a): they live on the copy's
	// basis face, not in a granted continuous effect.
	trig := false
	for _, tr := range o.Face().Triggers {
		if tr.Mode == "BecomesTarget" && tr.Effect != nil {
			trig = true
		}
	}
	_, svar := o.Face().SVars["PhantasmalImageSac"]
	if !trig {
		t.Fatal("copy carries no granted BecomesTarget trigger (AddTriggers$)")
	}
	if !svar {
		t.Fatal("AddSVars$ PhantasmalImageSac not granted")
	}
}
