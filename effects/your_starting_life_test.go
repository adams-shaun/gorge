package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

func TestCountYourStartingLifeRighteousValkyrie(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Righteous Valkyrie")
	if !ok || len(card.Faces) == 0 {
		t.Fatal("precondition: corpus Righteous Valkyrie face is present")
	}
	body, ok := card.Faces[0].SVars["Y"]
	if !ok || body != "Count$YourStartingLife/Plus.7" {
		t.Fatalf("precondition: Righteous Valkyrie Y = %q, want Count$YourStartingLife/Plus.7", body)
	}
	h := newHost(t, 2)
	o := h.g.AddObject(card, 0)
	o.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})
	if got := h.g.Zone(state.ZBattlefield, 0); len(got) != 1 || got[0] != o.ID {
		t.Fatalf("precondition: Righteous Valkyrie source %d is not on the battlefield: %v", o.ID, got)
	}
	h.startingLife = 20
	h.g.Players[0].Life = 23
	if h.g.Players[0].Life == h.startingLife {
		t.Fatal("precondition: current life must differ from starting life")
	}
	ctx := &Ctx{Source: o.ID, Controller: 0, SVars: card.Faces[0].SVars}
	got, resolved := EvalCountOK(h, ctx, body)
	if !resolved || got != 27 {
		t.Fatalf("EvalCountOK(%q) = (%d, %v), want (27, true)", body, got, resolved)
	}
}
