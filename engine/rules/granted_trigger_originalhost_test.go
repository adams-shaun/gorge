package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestGrantedTriggerCostOriginalHostNamesTheGrantor: an Equipment that grants
// "whenever this creature attacks, you may sacrifice [the Equipment]; when you
// do, ..." has its optional cost paid with the Equipment, not with the
// attacker that carries the trigger (CR 603.5, 701.21a). Before the cost
// window bound the grantor, the sacrifice matched nothing and only the decline
// was offered.
func TestGrantedTriggerCostOriginalHostNamesTheGrantor(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Sunfire Torch", "Spare Dagger"} {
		name := name
		for _, pay := range []bool{true, false} {
			pay := pay
			label := name + "/decline"
			if pay {
				label = name + "/pay"
			}
			t.Run(label, func(t *testing.T) {
				t.Parallel()
				e := cr70114AttackTable(t)
				torch := onBoardCard(t, e, 0, mustCorpusCard(t, reg, name))
				giant := onBoardReady(t, e, 0, "Name:Giant\nManaCost:3 R\nTypes:Creature Giant\nPT:3/3\nOracle:x\n")
				attachFixture(t, e, torch, giant)
				e.askAttackers()
				submitAttackersOnly(t, e, giant)
				var sawPay, sawTarget bool
				for i := 0; i < 40 && !e.G.Over; i++ {
					d := e.Pending()
					if d == nil {
						break
					}
					p, dec := -1, -1
					for _, o := range d.Options {
						switch o.Kind {
						case "trigger_cost_pay":
							p = o.Index
						case "trigger_cost_decline":
							dec = o.Index
						}
					}
					if dec >= 0 {
						sawPay = p >= 0
						if pay {
							if p < 0 {
								t.Fatalf("the granted trigger's sacrifice of the Equipment was not offered: %+v", d.Options)
							}
							submitChoices(t, e, p)
						} else {
							submitChoices(t, e, dec)
						}
						continue
					}
					if d.Kind == decision.KTarget {
						// the reflexive ability's "any target" ask (CR 603.12): the opponent.
						sawTarget = true
						for _, o := range d.Options {
							if o.Player == 1 {
								submitChoices(t, e, o.Index)
								break
							}
						}
						continue
					}
					if d.Kind != decision.KPriority || len(e.G.Stack) == 0 {
						break
					}
					passPriorityOnce(t, e)
				}
				if !sawPay {
					t.Fatalf("no pay option reached the cost window")
				}
				o := e.G.Obj(torch)
				if pay && (o == nil || o.Zone != state.ZGraveyard) {
					t.Fatalf("the Equipment zone after paying = %+v, want the graveyard", o)
				}
				if !pay && (o == nil || o.Zone != state.ZBattlefield) {
					t.Fatalf("the Equipment left the battlefield after a decline: %+v", o)
				}
				if g := e.G.Obj(giant); g == nil || g.Zone != state.ZBattlefield {
					t.Fatalf("the attacker must never pay the Equipment's cost: %+v", g)
				}
				sawSac := false
				for _, ev := range e.L.Events {
					if events.IsSacrifice(ev) && ev.Obj == torch {
						sawSac = true
					}
				}
				if sawTarget != pay {
					t.Fatalf("reflexive target ask seen = %v, want %v", sawTarget, pay)
				}
				if pay {
					if got := e.G.Players[1].Life; got >= 20 {
						t.Fatalf("opponent life = %d, want the reflexive damage dealt", got)
					}
				}
				if sawSac != pay {
					t.Fatalf("sacrifice of the Equipment logged = %v, want %v", sawSac, pay)
				}
			})
		}
	}
}
