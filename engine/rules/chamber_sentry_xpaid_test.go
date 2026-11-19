package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestChamberSentryActivatedForZeroDealsZero pins Count$xPaid on an
// activated ability that announced X=0: Chamber Sentry, cast for X=1, then
// activated ("{X}, {T}, Remove X +1/+1 counters from among creatures you
// control: It deals X damage to any target") for X=0 deals no damage. The
// read used to fall back to the permanent's cast X whenever the ability's
// own X was zero, a fallback meant only for a trigger that was never paid an
// X.
func TestChamberSentryActivatedForZeroDealsZero(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Chamber Sentry")}, nil)
	id := moveByName(t, e, 0, "Chamber Sentry", state.ZBattlefield)
	e.G.Obj(id).SummonSick = false
	e.emit(events.Event{Kind: events.XChange, Obj: id, Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
	addMana(t, e, 0, "R")
	life := e.G.Players[1].Life
	idx := oracleAbilityOption(e, id)
	if idx < 0 {
		t.Fatalf("no Chamber Sentry activation offered: %+v", e.Pending())
	}
	d := e.Pending()
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 40 && !e.G.Over; i++ {
		d = e.Pending()
		if d == nil {
			break
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			submitChoices(t, e, passPriorityOption(t, d))
			continue
		}
		pick := d.Options[0].Index
		for _, o := range d.Options {
			if o.Label == "0" || o.Player == 1 && o.Obj == 0 && o.Kind == "player" {
				pick = o.Index
			}
		}
		submitChoices(t, e, pick)
	}
	if got := e.G.Players[1].Life; got != life {
		t.Fatalf("Chamber Sentry for X=0 took the opponent from %d to %d life, want no damage", life, got)
	}
}
