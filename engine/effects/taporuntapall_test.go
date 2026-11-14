package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestTapOrUntapAllTargetPlayerScope(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g}
	for _, name := range []string{"myBear", "theirBig"} {
		if o := g.Obj(ids[name]); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: %s is not on the battlefield", name)
		}
	}
	if g.Obj(ids["myBear"]).Controller == g.Obj(ids["theirBig"]).Controller {
		t.Fatal("precondition: myBear and theirBig share a controller")
	}
	if g.Obj(ids["myBear"]).Tapped || g.Obj(ids["theirBig"]).Tapped {
		t.Fatal("precondition: both permanents should start untapped")
	}

	Resolve(h, playerCtx(1), sa(t, "SP$ TapOrUntapAll | ValidTgts$ Player | ValidCards$ Creature"))
	if !g.Obj(ids["theirBig"]).Tapped {
		t.Error("target player's creature was not tapped")
	}
	if g.Obj(ids["myBear"]).Tapped {
		t.Error("non-target player's creature was tapped")
	}
}
