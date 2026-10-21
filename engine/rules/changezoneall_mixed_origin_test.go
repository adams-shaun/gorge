package rules

// A ChangeZoneAll whose mixed Origin$ list includes the battlefield reads its
// leading Permanent base as a permanent card in every NON-battlefield scan
// (Once More with Feeling's "Exile all permanents and all cards from all
// graveyards"), while the battlefield scan keeps the on-the-battlefield
// object reading. Regression: the whole-origin rewrite only fired when
// Origin$ EXCLUDED the battlefield, so a mixed origin matched the bare
// Permanent in a graveyard, where an on-the-battlefield object can never be,
// and the graveyard half of the sweep moved nothing.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestChangeZoneAllMixedOriginPermanentIsPerZone(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg, ids := edrBoard(t, reg, 9181, map[string]state.Zone{
		"Goblin Piker":   state.ZBattlefield,
		"Grizzly Bears":  state.ZGraveyard,
		"Lightning Bolt": state.ZGraveyard})
	piker, bears, bolt := ids["Goblin Piker"], ids["Grizzly Bears"], ids["Lightning Bolt"]
	if z := e.G.Obj(piker).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition seq %d: Goblin Piker zone = %s, want Battlefield", len(e.L.Events), z)
	}
	if z := e.G.Obj(bears).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition seq %d: Grizzly Bears zone = %s, want Graveyard", len(e.L.Events), z)
	}
	if z := e.G.Obj(bolt).Zone; z != state.ZGraveyard {
		t.Fatalf("precondition seq %d: Lightning Bolt zone = %s, want Graveyard", len(e.L.Events), z)
	}
	omwf := mustCorpusCard(t, reg, "Once More with Feeling")
	effects.Resolve(e, effects.NewCtxPtr(0, 0, effects.CtxInit{}), omwf.Faces[0].Abilities[0])
	if z := e.G.Obj(bears).Zone; z != state.ZExile {
		t.Errorf("mixed Origin$ Battlefield,Graveyard seq %d: Grizzly Bears zone = %s, want Exile (a creature card in the graveyard is a permanent card)", len(e.L.Events), z)
	}
	if z := e.G.Obj(bolt).Zone; z != state.ZGraveyard {
		t.Errorf("mixed Origin$ Battlefield,Graveyard seq %d: Lightning Bolt zone = %s, want Graveyard (an instant is no permanent card)", len(e.L.Events), z)
	}
	if z := e.G.Obj(piker).Zone; z != state.ZExile {
		t.Errorf("mixed Origin$ Battlefield,Graveyard seq %d: Goblin Piker zone = %s, want Exile (the battlefield scan still reads Permanent as an on-the-battlefield object)", len(e.L.Events), z)
	}
	replayCheck(t, e, cfg)
}
