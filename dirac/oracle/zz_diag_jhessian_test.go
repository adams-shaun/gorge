package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Temporary diagnostic: drive gorge solo through the jhessian recording's
// prefix and dump p0's board and mana picture at the divergence decision.
func TestZZDiagJhessian(t *testing.T) {
	r, err := ReadRepro("testdata/repro/typecycling_offered_with_two_untapped_lands.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	reg := synthReg(t)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = r.Surface != "full"
	lead, err := NewGorgeSide(m, r.Seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	for i := 0; i < 562; i++ {
		dl := lead.Step()
		if e := lead.Err(); e != nil {
			t.Fatalf("step %d: %v", i, e)
		}
		if pol.Exhausted() {
			break
		}
		a := pol.Pick(&PickCtx{D: &dl, G: lead, Seq: i})
		lead.Apply(a)
		if e := lead.Err(); e != nil {
			t.Fatalf("apply %d: %v", i, e)
		}
	}
	e := lead.E
	p := state.PlayerID(0)
	fmt.Printf("DIAG p0 pool=%v life=%d\n", e.G.Players[p].Pool, e.G.Players[p].Life)
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		fmt.Printf("DIAG bf #%d %q tapped=%v\n", id, o.Face().Name, o.Tapped)
	}
	fmt.Printf("DIAG hand:")
	for _, id := range e.G.Zone(state.ZHand, p) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil {
			fmt.Printf(" #%d %q", id, o.Face().Name)
		}
	}
	fmt.Println()
	fmt.Printf("DIAG bound=%v\n", e.PotentialMana(p))
	t.Fatal("diag done")
}
