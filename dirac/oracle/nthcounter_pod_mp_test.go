package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/state"
)

// "When the tenth +1/+1 counter is put on this creature" (Shang-Chi and the
// Ten Rings: Mode$ CounterAdded, CounterType$ P1P1, CounterAmount$ EQ10) is a
// crossing (CR 122.6, 603.2c). The kernel stores its threshold as a Gate whose
// kind is CtrP1P1, which is 0; the crossing test was skipped for Gate.Ctr == 0,
// so the first +1/+1 counter (Captain America, Team Leader puts one on each
// Hero entering) queued the draw-five trigger, a stack object gorge never
// had. The fix tests Gate.N as well (triggers_gen.go objTrigGen).
var nthCounterPodDeck = []cardset.Entry{
	{Name: "Shang-Chi and the Ten Rings", Count: 6}, {Name: "Captain America, Team Leader", Count: 5},
	{Name: "Grizzly Bears", Count: 3},
	{Name: "Plains", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
}

// nthCounterWatch: gorge is asked with a Shang-Chi on the battlefield that
// holds a +1/+1 counter, short of the tenth.
func nthCounterWatch(c *PickCtx) bool {
	g := c.G.E.G
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			o := g.Obj(id)
			if o != nil && o.Face() != nil && o.Face().Name == "Shang-Chi and the Ten Rings" && o.Counter("P1P1") > 0 {
				return true
			}
		}
	}
	return false
}

// TestFourSeatNthCounterLockstep: four-seat games with Shang-Chi and Captain
// America play in lockstep, the tenth-counter trigger queued only on crossing.
func TestFourSeatNthCounterLockstep(t *testing.T) {
	var unused bool
	st := runPodClass(t, nthCounterPodDeck, 24, nthCounterWatch, &unused, false)
	t.Logf("%d games, %d divergences, %d stalls, %d watched decisions", st.games, st.divs, st.stalls, st.hits)
	if st.hits < 10 {
		t.Errorf("only %d watched decisions, want >= 10", st.hits)
	}
}
