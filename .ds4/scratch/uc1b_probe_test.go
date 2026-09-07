package rules

import (
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
	"testing"
)

func TestUC1BProbe(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, seats := range []int{2, 4, 6, 8} {
		var last *Engine
		head := playAcceptance(t, reg, seats, func(e *Engine, _ int) { last = e })
		t.Logf("%d seats: chain head %s, golden %s", seats, head, acceptanceHeads[seats])
		counters, daze, pierce := 0, 0, 0
		for i, ev := range last.L.Events {
			if ev.Kind == events.Resolve {
				o := last.G.Obj(ev.Obj)
				if o != nil && o.Face() != nil {
					if o.Face().Name == "Daze" {
						daze++
						t.Logf("seats=%d resolve Daze#%d controller=%d", seats, o.ID, o.Controller)
					}
					if o.Face().Name == "Spell Pierce" {
						pierce++
					}
				}
			}
			if ev.Text == "countered" {
				counters++
			}
			if ev.Kind == events.ModeChosen {
				o := last.G.Obj(ev.Obj)
				t.Logf("seats=%d event=%d mode=%+v source=%s#%d", seats, i, ev, o.Face().Name, o.ID)
				for j := i + 1; j < len(last.L.Events) && j < i+4; j++ {
					t.Logf("following=%+v", last.L.Events[j])
				}
			}
		}
		t.Logf("seats=%d countered=%d Daze-resolves=%d Spell-Pierce-resolves=%d", seats, counters, daze, pierce)
	}
	for _, s := range []string{"ExileFromGrave<1/All>", "Discard<1/Hand>", "DamageYou<4>", "Y", "Z", "X"} {
		c := ParseCost(s)
		_, zero := c.Pay(state.Mana{})
		p := state.Mana{}
		p[state.MC] = 1
		_, one := c.Pay(p)
		t.Logf("cost=%s parsed=%+v empty=%v one=%v", s, c, zero, one)
	}
}
