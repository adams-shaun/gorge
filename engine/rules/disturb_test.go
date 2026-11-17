package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func disturbOption(t *testing.T, e *Engine, id state.ObjID) *decision.Option {
	t.Helper()
	for _, o := range castOptions(t, e) {
		if o.Mode == "disturb" && o.Obj == id {
			cp := o
			return &cp
		}
	}
	return nil
}

// CR 702.146a-b: Disturb lets the owner cast the card transformed from the
// graveyard for its disturb cost; the spell has its back face's characteristics
// (Hook-Haunt Drifter, 1/2 flying) and, when it would go to a graveyard from
// anywhere, is exiled instead (the back face's own replacement).
func TestDisturbCastsTheBackFaceFromGraveyard(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Baithook Angler")
	id := putNamed(t, e, 0, "Baithook Angler", state.ZGraveyard)
	addMana(t, e, 0, "UU")
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard || o.FaceIdx != 0 {
		t.Fatalf("precondition: Baithook Angler = zone %s face %d", o.Zone, o.FaceIdx)
	}
	opt := disturbOption(t, e, id)
	if opt == nil {
		t.Fatalf("disturb cast not offered from the graveyard: %+v", castOptions(t, e))
	}
	submitChoices(t, e, opt.Index)
	o := e.G.Obj(id)
	if o.Zone != state.ZStack || o.Face().Name != "Hook-Haunt Drifter" {
		t.Fatalf("after paying: zone %s face %q, want the back face on the stack", o.Zone, o.Face().Name)
	}
	passUntilStackEmpty(t, e, 40)
	o = e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.Face().Name != "Hook-Haunt Drifter" {
		t.Fatalf("resolved: zone %s face %q, want Hook-Haunt Drifter on the battlefield", o.Zone, o.Face().Name)
	}
	if pw, th := e.Power(id), e.Toughness(id); pw != 1 || th != 2 {
		t.Fatalf("Hook-Haunt Drifter = %d/%d, want 1/2", pw, th)
	}
	e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 99})
	e.checkStateBased()
	e.priorityRound()
	if z := e.G.Obj(id).Zone; z != state.ZExile {
		t.Fatalf("dead Hook-Haunt Drifter is in %s, want exile", z)
	}
	replayCheck(t, e, cfg)
}

// Baithook Angler, Beloved Beggar and Binding Geist are carried by kw:Disturb
// alone: once it is registered none reports a missing feature.
func TestDisturbCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Baithook Angler", "Beloved Beggar", "Binding Geist"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}
