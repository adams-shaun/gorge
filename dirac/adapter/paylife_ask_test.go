package adapter

import (
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/decision"
)

// TestPayLifeAskReachesWire: the kernel's as-enters pay-any-amount-of-life ask
// (ChGenPayLife) is posed as gorge's KChoose of "paylife" options, the
// amount on Amount and in the label, one per kernel action in order
// (etbPayLifeOptions' shape), never as a "number" ask.
func TestPayLifeAskReachesWire(t *testing.T) {
	e, c := benchCommander(t)
	gm, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	g := gm.(*Game)
	g.d = dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChGenPayLife, Player: 0, N: 4}
	for i := 0; i < 4; i++ {
		g.d.Options[i] = dirac.MakeAction(dirac.AkChoose, 0, 0, uint8(i*3))
	}
	g.pose()
	d := g.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Player != 0 || len(d.Options) != 4 {
		t.Fatalf("posed %+v, want the payer's four-option choose", d)
	}
	for i, o := range d.Options {
		if o.Kind != "paylife" || o.Amount != i*3 || o.Label != strconv.Itoa(i*3)+" life" {
			t.Errorf("option %d = kind %q amount %d label %q, want paylife %d", i, o.Kind, o.Amount, o.Label, i*3)
		}
	}
	if len(g.acts) != 4 || g.acts[2] != g.d.Options[2] {
		t.Fatalf("choices play %v, want the kernel options in order", g.acts)
	}
}
