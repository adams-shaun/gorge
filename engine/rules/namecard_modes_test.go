package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestReallyCharmingPrinceNameCardRandomUsesListedCorpusChoices(t *testing.T) {
	t.Parallel()
	e, reg := nameCardEngine(t)
	prince, ok := reg.Lookup("Really Charming Prince")
	if !ok {
		t.Fatal("corpus precondition: Really Charming Prince missing")
	}
	sa := cards.ResolveSVar(prince.Faces[0].SVars, "TrigChoose")
	if sa == nil || sa.API != "NameCard" || sa.Params["AtRandom"] != "True" || sa.Params["ChooseFromList"] == "" {
		t.Fatalf("corpus precondition: TrigChoose = %+v, want NameCard AtRandom with ChooseFromList", sa)
	}
	source := e.G.AddObject(prince, 0).ID
	e.Emit(events.Event{Kind: events.MoveZone, Obj: source, From: state.ZHand, To: state.ZBattlefield, Player: 0})
	if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("corpus precondition: prince source = %+v, want battlefield", o)
	}
	listed := map[string]bool{}
	for _, name := range splitNameList(sa.Params["ChooseFromList"]) {
		listed[name] = true
	}
	if len(listed) < 2 {
		t.Fatalf("corpus precondition: choices = %q, want multiple choices", sa.Params["ChooseFromList"])
	}
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: 0}, sa)
	// The SubAbility chain ends in `DB$ Cleanup | ClearNamedCard$ True`, so the
	// chosen name is consumed by the resolution (cr-g229) rather than left on
	// the permanent. Read the recorded name event the handler emitted.
	chosen := ""
	for _, event := range e.L.Events {
		if event.Kind == events.Choose && event.Obj == source && event.Counter == "name" {
			chosen = event.Text
		}
	}
	if !listed[chosen] {
		t.Fatalf("Really Charming Prince chose %q outside its ChooseFromList %q", chosen, sa.Params["ChooseFromList"])
	}
	if got := e.G.Obj(source).ChosenName; got != "" {
		t.Fatalf("ChosenName after the chained ClearNamedCard cleanup = %q, want empty", got)
	}
}

func splitNameList(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if name := strings.TrimSpace(part); name != "" {
			out = append(out, name)
		}
	}
	return out
}
