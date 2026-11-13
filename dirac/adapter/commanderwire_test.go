package adapter

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// TestCommanderDamageReachesView: the damage a seat took from each commander is the
// board's Player.CmdDamage, dense in seat order, and the view keys it by commander.
func TestCommanderDamageReachesView(t *testing.T) {
	e, c := benchCommander(t)
	gm, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	g := gm.(*Game)
	if got := g.Board().Players[1].CmdDamage; len(got) != 2 || got[0] != 0 || got[1] != 0 {
		t.Fatalf("a fresh table: seat 1 took %v, want one zero per commander", got)
	}
	g.kp.CmdDmg[1][0] = 7
	g.boardGame()
	b := g.Board()
	if got := b.Players[1].CmdDamage; len(got) != 2 || got[0] != 7 || got[1] != 0 {
		t.Fatalf("seat 1 took %v, want [7 0]", got)
	}
	if got := b.Players[0].CmdDamage; len(got) != 2 || got[0] != 0 || got[1] != 0 {
		t.Fatalf("seat 0 took %v, want none", got)
	}
	v := view.ProjectFor(b, g.Chars(), 0, view.Public, nil)
	cmd := b.Players[0].Commanders[0]
	if got := v.Players[1].CmdDamage[cmd]; got != 7 {
		t.Fatalf("view: seat 1 took %d from seat 0's commander, want 7 (%v)", got, v.Players[1].CmdDamage)
	}
	if len(v.Players[0].CmdDamage) != 0 {
		t.Fatalf("view: seat 0 took %v, want none", v.Players[0].CmdDamage)
	}
}

// TestCommanderZoneAskReachesWire: the kernel's CR 903.9a ask is posed as the owner's
// commander_zone decision naming the commander, command zone first, and each choice
// plays the kernel action of the same index.
func TestCommanderZoneAskReachesWire(t *testing.T) {
	e, c := benchCommander(t)
	gm, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	g := gm.(*Game)
	cmd := g.Board().Zone(state.ZCommand, 0)
	if len(cmd) != 1 {
		t.Fatalf("seat 0 command zone holds %d cards", len(cmd))
	}
	var obj dirac.Obj = dirac.NoObj
	for i := 0; i < int(g.kp.NObj); i++ {
		if g.ids.obj(uint8(i)) == cmd[0] {
			obj = dirac.Obj(i)
		}
	}
	if obj == dirac.NoObj {
		t.Fatal("the commander has no kernel object")
	}
	g.d = dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChCommanderZone, Player: 0, N: 2}
	g.d.Options[0] = dirac.MakeAction(dirac.AkChoose, dirac.TCode(obj), 0, 0)
	g.d.Options[1] = dirac.MakeAction(dirac.AkChoose, dirac.TCode(obj), 0, 1)
	g.pose()
	d := g.Pending()
	if d == nil || d.Kind != decision.KCommanderZone || d.Player != 0 {
		t.Fatalf("posed %+v, want the owner's commander_zone ask", d)
	}
	if len(d.Options) != 2 || d.Options[0].Obj != cmd[0] || d.Options[1].Obj != cmd[0] {
		t.Fatalf("options %+v, want two naming the commander %d", d.Options, cmd[0])
	}
	if len(g.acts) != 2 || g.acts[0] != g.d.Options[0] || g.acts[1] != g.d.Options[1] {
		t.Fatalf("choices play %v, want the kernel options in order", g.acts)
	}
}
