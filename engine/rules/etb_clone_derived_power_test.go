package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFrostkiteCopiesACreaturePumpedToPowerFour: Deceptive Frostkite's
// "creature you control with power 4 or greater" reads the template's current
// power (CR 613.4, 707.2), a layer-7c pump included. A printed 2/2 under two
// Glorious Anthems is 4/4: it is offered, and the chosen template is still
// legal when the replacement applies. The option list and the entry's
// revalidation both read a printed 2 once.
func TestFrostkiteCopiesACreaturePumpedToPowerFour(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Deceptive Frostkite"), lookup(t, reg, "Topan Ascetic"),
		lookup(t, reg, "Glorious Anthem"), lookup(t, reg, "Glorious Anthem")}, []*cards.Card{})
	template := moveByName(t, e, 0, "Topan Ascetic", state.ZBattlefield)
	moveByName(t, e, 0, "Glorious Anthem", state.ZBattlefield)
	moveByName(t, e, 0, "Glorious Anthem", state.ZBattlefield)
	id := moveByName(t, e, 0, "Deceptive Frostkite", state.ZHand)
	if p := e.Power(template); p != 4 {
		t.Fatalf("precondition: Topan Ascetic under two anthems has power %d, want 4", p)
	}
	e.G.Players[0].Pool[state.MU] = 2

	castMode(t, e, id, "")
	resolveTopProbe(e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "etb" {
		t.Fatalf("expected copy choice, got %+v", d)
	}
	choice := -1
	for _, opt := range d.Options {
		if opt.Kind == "clone" && opt.Obj == template {
			choice = opt.Index
		}
	}
	if choice < 0 {
		t.Fatalf("the pumped 4/4 template is absent from the choices: %+v", d.Options)
	}
	submitChoices(t, e, choice)
	settleDrain(e)
	if !hasEvent(e, events.ClonePermanent, id) {
		t.Fatal("Frostkite did not copy the 4/4 template it chose")
	}
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Face() == nil || o.Face().Name != "Topan Ascetic" {
		t.Fatalf("Frostkite should enter as Topan Ascetic, got %+v", o)
	}
}
