package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestAuroraShifterCloneGrantsNamedTriggerAndSVars(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := searchEngine(t, reg, "Aurora Shifter", "Grizzly Bears")
	shifter := searchMoveByName(t, e, "Aurora Shifter", state.ZBattlefield)
	bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
	o, target := e.G.Obj(shifter), e.G.Obj(bear)
	if o == nil || target == nil || o.Zone != state.ZBattlefield || target.Zone != state.ZBattlefield || o.Face().Name == target.Face().Name {
		t.Fatal("clone operands must be distinct battlefield permanents")
	}
	sa := cards.ResolveSVar(o.Face().SVars, "TrigCopy")
	if sa == nil || sa.Params["AddTriggers"] != "TrigDamage" || sa.Params["AddSVars"] != "TrigDamage,TrigEnergy" {
		t.Fatalf("Aurora Shifter grant precondition failed: %+v", sa)
	}
	effects.Resolve(e, &effects.Ctx{Source: shifter, Controller: o.Controller, Targets: []state.Target{{Obj: bear}}, SVars: o.Face().SVars, OfferedSA: sa}, sa)
	if e.G.Obj(shifter).Face().Name != "Grizzly Bears" {
		t.Fatal("clone did not replace face")
	}
	// The grants are copiable values (CR 707.9a): on the copy's basis face.
	f := e.G.Obj(shifter).Face()
	if _, ok := f.SVars["TrigEnergy"]; !ok {
		t.Fatal("clone lost the granted energy SVar")
	}
	if _, ok := f.SVars["TrigDamage"]; !ok {
		t.Fatal("clone lost the granted damage SVar")
	}
	found := false
	for _, tr := range f.Triggers {
		if tr.Mode == "DamageDone" && tr.Effect != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("clone did not grant the named damage trigger")
	}
	replayCheck(t, e, cfg)
}
