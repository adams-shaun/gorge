package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// triggerReadTally follows one source's triggered ability from its push to the
// priority after it resolves, counting what its controller drew and the life
// they paid inside that window, so a recorded game can be required to reach
// the effect the lowering reads.
type triggerReadTally struct {
	card                string
	seen                int
	pending, resolving  bool
	draws, paid         int
	drewSmall, drewNone bool
	paidLife            bool
}

// check consumes gorge's new events; it reports nothing and records the verdicts.
func (y *triggerReadTally) check(ref *GorgeSide) {
	g := ref.E.G
	evs := ref.E.L.Events
	for ; y.seen < len(evs); y.seen++ {
		switch ev := evs[y.seen]; ev.Kind {
		case events.TriggerPush:
			if o := g.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == y.card {
				y.pending, y.draws, y.paid = true, 0, 0
			}
		case events.Draw:
			if y.pending && ev.Player == 0 {
				y.draws++
			}
		case events.LifeChange:
			if y.pending && ev.Player == 0 && ev.Amount < 0 {
				y.paid++
			}
		case events.Resolve:
			if y.pending {
				y.resolving = true
			}
		case events.Priority:
			if y.pending && y.resolving {
				y.pending, y.resolving = false, false
				y.drewSmall = y.drewSmall || y.draws > 0
				y.drewNone = y.drewNone || y.draws == 0 && len(g.Zone(state.ZGraveyard, 1)) > 0
				y.paidLife = y.paidLife || y.paid > 0
			}
		}
	}
}

// Hollow Marauder's enters trigger makes the opponent discard, then draws a
// card for each discarding opponent whose discard was not mana value 4 or
// more: the draw count is the targeted-player count (one opponent, or none)
// less the remembered discards of mana value 4 or more. Both engines agree on a
// game where the discard was a land and the controller drew, and on one where
// the discarded card was big and nothing was drawn.
func TestHollowMarauderDrawCountLockstep(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		file string
		want func(y *triggerReadTally) bool
	}{
		{"hollow_marauder_draws_for_a_small_discard", func(y *triggerReadTally) bool { return y.drewSmall }},
		{"hollow_marauder_no_draw_for_a_big_discard", func(y *triggerReadTally) bool { return y.drewNone }},
	} {
		t.Run(c.file, func(t *testing.T) {
			y := &triggerReadTally{card: "Hollow Marauder"}
			stdReproLockstepReaching(t, "testdata/repro/"+c.file+".repro.json", "std-otj", func(ref *GorgeSide) bool {
				y.check(ref)
				return c.want(y)
			})
		})
	}
}

// Madame Null's trigger offers to pay life equal to the entering creature's
// power and puts that many +1/+1 counters on it: the life is read as the
// payment is offered and the counters at resolution. Both engines agree on a
// game where the controller paid.
func TestMadameNullPaidPowerLockstep(t *testing.T) {
	t.Parallel()
	y := &triggerReadTally{card: "Madame Null, Power Broker"}
	stdReproLockstepReaching(t, "testdata/repro/madame_null_pays_life_for_counters.repro.json", "std-tmt", func(ref *GorgeSide) bool {
		y.check(ref)
		return y.paidLife
	})
}
