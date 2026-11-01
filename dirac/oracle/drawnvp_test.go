package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Gleaming Splendor lockstep: "Whenever an opponent
// draws their second card each turn, you create a Treasure token" is a Mode$
// Drawn trigger scoped by ValidPlayer$ Opponent (gorge trigmatch
// drawnMatches) with Number$ 2 counting the drawing player's own draws. The
// controller's own second draw must not trigger it. Gleaming's {2}{W} makes
// two target players each draw, so the opponent's second draw also comes from
// the controller's own activations. The expected answer is gorge's.
var kdrawnvpA = []cardset.Entry{
	{Name: "Gleaming Splendor", Count: 4}, {Name: "Opt", Count: 2}, {Name: "Savannah Lions", Count: 6},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Plains", Count: 12}, {Name: "Island", Count: 12},
}

var kdrawnvpB = []cardset.Entry{
	{Name: "Divination", Count: 4}, {Name: "Opt", Count: 4}, {Name: "Shock", Count: 1},
	{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 3}, {Name: "Island", Count: 12}, {Name: "Mountain", Count: 12},
}

// both seat orders
var kdrawnvpMatchups = [][2][]cardset.Entry{{kdrawnvpA, kdrawnvpB}, {kdrawnvpB, kdrawnvpA}}

// kdrawnvpTally counts, on gorge's side, Gleaming Splendor trigger pushes
// (one Treasure each) and the draws that reach a player's second of the turn
// while a Gleaming Splendor is out: an opponent's (must trigger) or the
// controller's own (must not).
type kdrawnvpTally struct {
	games, pushes, oppSecond, ownSecond int
}

func kdrawnvpCheck(tal *kdrawnvpTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	seen := 0
	var draws [2]int
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen, draws = ref, 0, [2]int{}
			tal.games++
		}
		g := ref.E.G
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := &evs[seen]
			switch ev.Kind {
			case events.TurnChange:
				draws = [2]int{}
			case events.Draw:
				draws[ev.Player]++
				if draws[ev.Player] != 2 {
					continue
				}
				for p := state.PlayerID(0); p < 2; p++ {
					for _, id := range g.Zone(state.ZBattlefield, p) {
						if o := g.Obj(id); o != nil && o.Controller == p && o.Face() != nil && o.Face().Name == "Gleaming Splendor" {
							if p == ev.Player {
								tal.ownSecond++
							} else {
								tal.oppSecond++
							}
						}
					}
				}
			case events.TriggerPush:
				if o := g.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Gleaming Splendor" {
					tal.pushes++
				}
			}
		}
		return nil
	}
}

func TestDrawnvpLockstep(t *testing.T) {
	t.Parallel()
	var tal kdrawnvpTally
	sonRun(t, "drawnvp", kdrawnvpMatchups, 300, true, nil, kdrawnvpCheck(&tal))
	t.Logf("%d games: %d Gleaming Splendor triggers resolved (Treasures), %d opponent second draws while it was out, %d controller second draws while it was out (no trigger)", tal.games, tal.pushes, tal.oppSecond, tal.ownSecond)
	if tal.pushes < 20 || tal.ownSecond < 5 {
		t.Errorf("triggers %d (want >= 20), controller second draws %d (want >= 5)", tal.pushes, tal.ownSecond)
	}
	if tal.pushes > tal.oppSecond {
		t.Errorf("%d triggers for %d opponent second draws: gorge fired outside the opponent's second draw", tal.pushes, tal.oppSecond)
	}
}

// TestDrawnvpMutation: Gleaming Splendor's draw trigger watching its
// controller instead of an opponent must make some seed diverge.
func TestDrawnvpMutation(t *testing.T) {
	mut := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbTriggered && abs[j].Trig.Event == kop.TrigDrawn {
					abs[j].Trig.Whose = kop.WhoseYou
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}
	if n := sonRun(t, "drawnvp", kdrawnvpMatchups, 64, false, mut, nil); n == 0 {
		t.Fatal("Whose=You on the Drawn trigger: no seed diverged")
	}
}
