package effects

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestDiscoverExilesUntilCastableCardAndOffersItFree(t *testing.T) {
	h := &askHost{}
	h.suspendAfterAsk = true
	h.g = state.NewGame(names(2))
	land := h.g.AddObject(mkCard(t, "Name:Top Land\nManaCost:no cost\nTypes:Land\nOracle:x\n"), 0)
	tooBig := h.g.AddObject(mkCard(t, "Name:Too Big\nManaCost:5\nTypes:Creature\nPT:5/5\nOracle:x\n"), 0)
	found := h.g.AddObject(mkCard(t, "Name:Found\nManaCost:2\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0)
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{land.ID, tooBig.ID, found.ID})

	Resolve(h, &Ctx{Controller: 0}, sa(t, "DB$ Discover | Num$ 2"))
	if h.asked == nil || h.asked.Kind != decision.KModes || h.asked.ResumeKind != "play" {
		t.Fatalf("decision = %+v, want a free-play decision", h.asked)
	}
	if len(h.asked.Options) != 1 || h.asked.Options[0].Obj != found.ID {
		t.Fatalf("options = %+v, want only found card %d", h.asked.Options, found.ID)
	}
	if h.asked.ResumeSA == nil || h.asked.ResumeSA.Params["WithoutManaCost"] != "True" {
		t.Fatalf("resume SA = %+v, want WithoutManaCost True", h.asked.ResumeSA)
	}
	var moves []string
	for _, ev := range h.log {
		if ev.Kind == events.MoveZone {
			moves = append(moves, fmt.Sprintf("%d:%v>%v", ev.Obj, ev.From, ev.To))
		}
	}
	want := []string{
		fmt.Sprintf("%d:%v>%v", land.ID, state.ZLibrary, state.ZExile),
		fmt.Sprintf("%d:%v>%v", tooBig.ID, state.ZLibrary, state.ZExile),
		fmt.Sprintf("%d:%v>%v", found.ID, state.ZLibrary, state.ZExile),
		fmt.Sprintf("%d:%v>%v", land.ID, state.ZExile, state.ZLibrary),
		fmt.Sprintf("%d:%v>%v", tooBig.ID, state.ZExile, state.ZLibrary),
	}
	if !reflect.DeepEqual(moves, want) {
		t.Fatalf("moves = %v, want %v", moves, want)
	}
}
