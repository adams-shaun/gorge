package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Momentum Breaker's enters trigger makes each opponent who could not sacrifice
// discard a card (Defined$ OppNonRememberedController, Mode$ TgtChoose). Like
// each opponent's discard, every such opponent chooses in turn with the earlier
// picks still in hand, and the cards then move together (CR 101.4, 608.2c).
// The kernel resolved only the first opponent's discard, at the first answer,
// so at three opponents the second asked in gorge never asked in the kernel
// (a four-seat pod, decision 458). The kernel now runs the same chain it runs
// for each opponent over the non-remembered set (interp.go OpDiscard).
//
// Each row is a four-seat recording (pool "one-off", the table built from its
// four decks alone) and what its game must show, so a recording that stops
// reaching the discard fails instead of passing as a different game.
var eachOpponentDiscardRepros = []struct {
	name     string
	at       int // the decision the recording diverged at before the fix
	discards int // seats that discard from hand to graveyard in one resolution
}{
	{"momentum_breaker_discard_each_opponent_four_seat_pod", 458, 2},
}

// maxSeatsDiscardingTogether is the most seats that discarded from hand in one
// resolution of a card's trigger: hand to graveyard moves between the trigger's
// resolve and the next priority.
func maxSeatsDiscardingTogether(g *GorgeSide, card string) int {
	best := 0
	armed, resolved := false, false
	seats := map[state.PlayerID]bool{}
	for j := range g.E.L.Events {
		e := &g.E.L.Events[j]
		switch {
		case e.Kind == events.TriggerPush:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == card {
				armed, resolved = true, false
				clear(seats)
			}
		case armed && e.Kind == events.Resolve:
			resolved = true
		case armed && resolved && e.Kind == events.MoveZone && e.From == state.ZHand && e.To == state.ZGraveyard:
			if o := g.E.G.Obj(e.Obj); o != nil {
				seats[o.Owner] = true
				best = max(best, len(seats))
			}
		case armed && resolved && e.Kind == events.Priority:
			armed = false
		}
	}
	return best
}

func TestFourSeatEachOpponentDiscardLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range eachOpponentDiscardRepros {
		t.Run(c.name, func(t *testing.T) {
			reg := synthReg(t)
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			if r.Pool != "one-off" || len(r.Cards) != 2 || len(r.MoreCards) != 2 || r.Stall {
				t.Fatalf("pool %q, %d+%d decks, stall %v: want a clean one-off four-seat pod", r.Pool, len(r.Cards), len(r.MoreCards), r.Stall)
			}
			m, err := r.Matchup(reg)
			if err != nil {
				t.Fatal(err)
			}
			lists := [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]}
			var all []string
			for _, l := range lists {
				all = append(all, l...)
			}
			set := cardset.Raw(printedDelveSet(t, [2][]string{all, nil}))
			es := make([][]cardset.Entry, len(lists))
			for i, l := range lists {
				es[i] = ReproEntries(reg, set, l)
			}
			tbl, _, err := set.MatchupN(es)
			if err != nil {
				t.Fatal(err)
			}
			rc := r.RunConfig()
			rc.MaxDecisions = len(r.Actions) + 200
			k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
			if err != nil {
				t.Fatal(err)
			}
			defer CloseSide(k)
			g, err := NewGorgeSide(m, r.Seed, rc.Config, nil)
			if err != nil {
				t.Fatal(err)
			}
			pol := &ScriptPolicy{Actions: r.Actions}
			res := LockstepLed(m, r.Seed, g, k, pol, rc)
			if msg := reproSkipFailure(r, pol); msg != "" {
				t.Fatal(msg)
			}
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v %s", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
			}
			if res.Stall != "" && res.Stall != "decisions" { // the 200-decision tail past the script ends at its cap
				t.Fatalf("stall %s", res.Stall)
			}
			if res.Decisions <= c.at {
				t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, c.at)
			}
			if n := maxSeatsDiscardingTogether(g, "Momentum Breaker"); n < c.discards {
				t.Errorf("%d seats discarded to Momentum Breaker's trigger, want at least %d", n, c.discards)
			}
		})
	}
}
