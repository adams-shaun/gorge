package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestEffectImprintOnHostRetainsRememberedObjects(t *testing.T) {
	g := state.NewGame([]string{"A"})
	host := g.AddObject(nil, 0)
	remembered := g.AddObject(nil, 0)
	if g.Obj(host.ID) == nil || g.Obj(remembered.ID) == nil {
		t.Fatal("test precondition failed: host and remembered objects must exist")
	}
	h := &fakeHost{g: g}
	sa := &cards.SA{API: "Effect", Params: map[string]string{
		"ImprintOnHost":   "True",
		"RememberObjects": "Targeted",
	}}

	effEffect(h, &Ctx{
		Source:     host.ID,
		Controller: 0,
		Targets:    []state.Target{{Obj: remembered.ID}},
	}, sa)

	imprinted := false
	for _, event := range h.log {
		if event.Kind == events.Imprint && event.Obj == host.ID {
			imprinted = true
		}
	}
	if !imprinted {
		t.Fatal("effEffect emitted no host Imprint event; ImprintOnHost handler did not run")
	}
	got := g.Obj(host.ID).Imprinted
	if len(got) != 1 || got[0] != remembered.ID {
		t.Fatalf("host imprint = %v, want remembered object %d", got, remembered.ID)
	}
}
