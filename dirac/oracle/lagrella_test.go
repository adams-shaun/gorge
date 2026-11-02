package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorgeLagrellaFixTime is the commit time (UTC) of gorge, the merge
// of the fix (ChangeZone ReturnAbility$/ReturnValid$ runs on a
// card returned by Duration$ UntilHostLeavesPlay, CR 610.3): before it a
// card returned from Lagrella's exile came back with no counters.
const gorgeLagrellaFixTime = "20261026165709"

// The Lagrella, the Magpie lockstep: "exile any number of other
// target creatures controlled by different players until Lagrella leaves the
// battlefield. When an exiled card enters under your control this way, put two
// +1/+1 counters on it." Lightning Bolt and Terror kill her; her own
// creatures come back with two counters, an opponent's with none. The
// expected answer at every decision is gorge's.
var klagrellaMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Lagrella, the Magpie", Count: 12}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Forest", Count: 7}, {Name: "Plains", Count: 7}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Lightning Bolt", Count: 6}, {Name: "Terror", Count: 4}, {Name: "Hill Giant", Count: 6},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Prodigal Sorcerer", Count: 3}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 7},
	},
}, {
	{
		{Name: "Lagrella, the Magpie", Count: 10}, {Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Lagrella, the Magpie", Count: 10}, {Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Mountain", Count: 6},
	},
}}

// klagrellaTally counts, on gorge's side, cards that came back from exile
// carrying two +1/+1 counters (the rider fired), ones that came back with
// none, and games.
type klagrellaTally struct{ with, without, games int }

func klagrellaCheck(tal *klagrellaTally) func(*GorgeSide, Side) *Divergence {
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
			if ev.Kind != events.MoveZone || ev.From != state.ZExile || ev.To != state.ZBattlefield {
				continue
			}
			if o := ref.E.G.Obj(ev.Obj); o != nil {
				if o.Counter("P1P1") >= 2 {
					tal.with++
				} else {
					tal.without++
				}
			}
		}
		return nil
	}
}

func TestLagrellaLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeLagrellaFixTime) {
		t.Skip("gorge before the ReturnAbility$ fix (5dc9b0697)")
	}
	var tal klagrellaTally
	sonRun(t, "lagrella", klagrellaMatchups, 300, true, nil, klagrellaCheck(&tal))
	t.Logf("%d games: %d returns with two counters, %d without", tal.games, tal.with, tal.without)
	if tal.with < 30 || tal.without < 10 {
		t.Errorf("returns with counters %d (want >= 30), without %d (want >= 10)", tal.with, tal.without)
	}
}

// TestLagrellaLockstepMutation: with the rider dropped from the exile, some
// seed diverges.
func TestLagrellaLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeLagrellaFixTime) {
		t.Skip("gorge before the ReturnAbility$ fix (5dc9b0697)")
	}
	n := sonRun(t, "lagrella", klagrellaMatchups, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpExileUntilLeaves {
						p[x].A = kop.ValueRef{}
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with the return rider dropped")
	}
}
