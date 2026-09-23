package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestDestroyAllUsesNamedZone(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	exiled := g.AddObject(mkCard(t, "Name:Exiled Trick\nTypes:Instant\nOracle:x\n"), 1)
	exiled.Zone = state.ZExile
	g.SetZone(state.ZExile, 1, []state.ObjID{exiled.ID})
	if exiled.Zone != state.ZExile || len(g.Zone(state.ZExile, 1)) != 1 {
		t.Fatal("precondition: test instant must be in opponent's exile")
	}

	Resolve(h, &Ctx{Controller: 0}, sa(t, "SP$ DestroyAll | ValidCards$ Instant | Zone$ Exile"))

	if got := g.Obj(exiled.ID).Zone; got != state.ZGraveyard {
		t.Fatalf("named-zone card moved to %s, want graveyard", got)
	}
	if got := g.Obj(ids["myInstant"]).Zone; got != state.ZGraveyard {
		t.Fatalf("matching card outside named zone moved to %s, want it to remain in graveyard", got)
	}
	if got := g.Obj(ids["myBear"]).Zone; got != state.ZBattlefield {
		t.Fatalf("nonmatching battlefield creature moved to %s, want battlefield", got)
	}
}
