package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCommandZoneConvokeCommanderIsOfferedOnItsCredit: Convoke functions while
// the spell is on the stack, whatever zone it was cast from (CR 702.51a), so a
// commander cast from the command zone (CR 903.8) is offered on the same
// untapped-creature credit as a hand cast. Three creatures and an empty pool
// pay {2}{W}: the offer is there, and taking it taps them for the commander.
func TestCommandZoneConvokeCommanderIsOfferedOnItsCredit(t *testing.T) {
	t.Parallel()
	cmdr := card(t, "Name:Test Convoker\nManaCost:2 W\nTypes:Legendary Creature Angel\nPT:3/3\nK:Convoke\nOracle:x\n")
	e, _ := colourIdentityGame(t, 12901, FormatCommander, cmdr, nil)
	toMain1(t, e)
	cmd := e.G.Zone(state.ZCommand, 0)
	if len(cmd) != 1 || e.G.Obj(cmd[0]).Face().Name != "Test Convoker" {
		t.Fatalf("command zone = %v, want the commander alone", cmd)
	}
	kid := cmd[0]
	e.priorityRound()
	if hasCastOption(castOptions(t, e), kid) {
		t.Fatal("offered with no creature and an empty pool: nothing pays the cost")
	}
	var convokers []state.ObjID
	for range 3 {
		convokers = append(convokers, onBoardReady(t, e, 0, "Name:Test Cleric\nManaCost:W\nTypes:Creature Human\nPT:1/1\nOracle:x\n"))
	}
	e.priorityRound()
	if p := e.G.Players[0].Pool; p.Total() != 0 {
		t.Fatalf("pool = %v, want empty: the creatures alone must pay", p)
	}
	opts := castOptions(t, e)
	if !hasCastOption(opts, kid) {
		t.Fatalf("the convoke commander is not offered from the command zone: %+v", opts)
	}
	for _, o := range opts {
		if o.Obj == kid {
			submitChoices(t, e, o.Index)
			break
		}
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Source != kid {
		t.Fatalf("pending %+v, want the convoke ask", d)
	}
	// One role per creature: the first white creature pays {W}, the others {1}.
	var picks []int
	used := map[state.ObjID]bool{}
	for _, kind := range []string{"convoke_W", "convoke_generic", "convoke_generic"} {
		for _, o := range d.Options {
			if o.Kind == kind && !used[o.Obj] {
				picks, used[o.Obj] = append(picks, o.Index), true
				break
			}
		}
	}
	if len(picks) != 3 {
		t.Fatalf("convoke ask %+v, want a {W} and two generic convokers", d.Options)
	}
	submitChoices(t, e, picks...)
	if z := e.G.Obj(kid).Zone; z != state.ZStack {
		t.Fatalf("commander zone = %v after the cast, want the stack (pending %+v)", z, e.Pending())
	}
	for _, id := range convokers {
		if !e.G.Obj(id).Tapped {
			t.Fatalf("convoker %d untapped: the cost was not paid by convoke (pending %+v)", id, e.Pending())
		}
	}
}
