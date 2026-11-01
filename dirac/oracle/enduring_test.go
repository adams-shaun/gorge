package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Enduring Innocence lockstep: lifelink; "whenever one or more
// other creatures you control with power 2 or less enter, draw a card" (a
// ChangesZoneAll trigger, once each turn); and when it dies as a creature it
// returns as a non-creature enchantment. Shock, Prodigal Sorcerer and Hill
// Giant kill it; the expected answer at every decision is gorge's.
var kenduringMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Enduring Innocence", Count: 10}, {Name: "Savannah Lions", Count: 8}, {Name: "Suntail Hawk", Count: 6}, {Name: "Plains", Count: 16},
	},
	{
		{Name: "Shock", Count: 6}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Suntail Hawk", Count: 4}, {Name: "Mountain", Count: 11}, {Name: "Island", Count: 9},
	},
}}

// kenduringTally counts, on gorge's side, Enduring Innocence's batch draws
// (a draw while it is on the battlefield as a creature trigger resolves is
// not told apart: the draws beyond the turn's normal one are counted by the
// trigger's stack resolution) and its returns as an enchantment.
type kenduringTally struct{ returns, games int }

func kenduringCheck(tal *kenduringTally) func(*GorgeSide, Side) *Divergence {
	seen := 0
	var cur *GorgeSide
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, seen = ref, 0
			tal.games++
		}
		evs := ref.E.L.Events
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			if ev.Kind == events.MoveZone && ev.From == state.ZGraveyard && ev.To == state.ZBattlefield {
				if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Enduring Innocence" {
					tal.returns++
				}
			}
		}
		return nil
	}
}

func TestEnduringLockstep(t *testing.T) {
	t.Parallel()
	var tal kenduringTally
	// more than maxUsed (8) once-per-turn records in one turn (several
	// Innocences, each a new object after every return) stalls loudly
	// (StallUsed); a few games reach it
	_, stalls := sonRunStalls(t, "enduring", kenduringMatchups, 600, true, nil, kenduringCheck(&tal))
	t.Logf("%d games: %d Enduring Innocence returns, %d used stalls", tal.games, tal.returns, stalls)
	if stalls > 12 {
		t.Errorf("%d used stalls of %d games", stalls, tal.games)
	}
	if tal.returns < 50 {
		t.Errorf("returns %d; want at least 50", tal.returns)
	}
}

// TestEnduringLockstepMutation: with the batch draw dropped, some seed
// diverges (the draw is the ChangesZoneAll trigger's whole body).
func TestEnduringLockstepMutation(t *testing.T) {
	t.Parallel()
	n := sonRun(t, "enduring", kenduringMatchups, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Trig.Event == kop.TrigETB {
					p := append([]kop.Instr(nil), abs[j].Prog...)
					for x := range p {
						if p[x].Op == kop.OpDraw {
							p[x] = kop.Instr{Op: kop.OpNop}
						}
					}
					abs[j].Prog = p
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with the draw dropped")
	}
}
