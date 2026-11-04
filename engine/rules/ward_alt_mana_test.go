package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A ward of "discard a card or pay {2}" pays its mana half like any ward mana
// cost (CR 702.21a): the payer's untapped lands are activated in the mana
// window, not required to have floated the mana beforehand.
func TestWardDiscardOrPayManaOpensTheManaWindow(t *testing.T) {
	t.Parallel()
	e := combatEngine(t)
	warded := onBoard(t, e, 0, "Name:Warded Beast\nManaCost:1\nTypes:Creature\nPT:2/2\nK:Ward:Discard<1/Card>:2\nOracle:x\n")
	onBoard(t, e, 1, "Name:Ward Island\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ U\nOracle:x\n")
	onBoard(t, e, 1, "Name:Second Island\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ U\nOracle:x\n")
	cause := e.G.Zone(state.ZLibrary, 1)[0]
	e.emit(events.Event{Kind: events.PutOnStack, Obj: cause, Player: 1, From: state.ZLibrary, To: state.ZStack})
	e.emit(events.Event{Kind: events.TargetsChosen, Obj: cause, IDs: []state.ObjID{warded}})
	e.putTriggersOnStack()
	e.resolveTop()
	if err := e.Submit(decision.Intent{Seq: e.Pending().Seq, Player: 1, Choices: []int{0}}); err != nil {
		t.Fatal(err)
	}
	d := e.Pending()
	if d == nil || d.ResumeKind != "ward_alt" {
		t.Fatalf("ward alternative ask = %+v", d)
	}
	pick := -1
	for i, o := range d.Options {
		if o.Kind == "ward_mana" {
			pick = i
		}
	}
	if pick < 0 {
		t.Fatalf("no ward_mana option in %+v", d.Options)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: 1, Choices: []int{pick}}); err != nil {
		t.Fatal(err)
	}
	d = e.Pending()
	if d == nil || d.ResumeKind != "ward_mana" {
		t.Fatalf("paying the {2} half posed no mana window: pending %+v, cause on %s", d, e.G.Obj(cause).Zone)
	}
}
