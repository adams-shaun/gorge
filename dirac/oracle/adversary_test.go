package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The Adversaries lockstep: "When it enters, you may pay {1}{W} any
// number of times. When you pay this cost one or more times, put that many
// valor counters on it" (Intrepid Adversary: creatures you control get +1/+1
// per valor counter) and "... {1}{G} ... that many +1/+1 counters, then up to
// that many target lands you control become 3/3 Wolf creatures with haste"
// (Primal Adversary). No AnnounceMax$: gorge offers every count the potential
// mana covers. The expected answer at every decision is gorge's.
var kadversaryMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Intrepid Adversary", Count: 12}, {Name: "Grizzly Bears", Count: 4}, {Name: "Plains", Count: 24},
	},
	{
		{Name: "Primal Adversary", Count: 12}, {Name: "Shock", Count: 4}, {Name: "Forest", Count: 24},
	},
}, {
	{
		{Name: "Intrepid Adversary", Count: 6}, {Name: "Primal Adversary", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 12},
	},
	{
		{Name: "Intrepid Adversary", Count: 6}, {Name: "Primal Adversary", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 12},
	},
}}

// kadversaryTally counts, on gorge's side, repetition announcements by the
// largest count offered and games.
type kadversaryTally struct {
	games int
	top   [9]int // top[n]: announcements whose largest option is n times
}

func kadversaryCheck(tal *kadversaryTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	var last *decision.Decision
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, last = ref, nil
			tal.games++
		}
		d := ref.E.Pending()
		if d == nil || d == last {
			return nil
		}
		last = d
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "trigger_cost_x" {
			tal.top[min(len(d.Options)-1, 8)]++
		}
		return nil
	}
}

func TestAdversaryLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeNumTimesFixTime) {
		t.Skip("gorge before the Announce$ NumTimes fix (a2c2e9ddf)")
	}
	var tal kadversaryTally
	_, stalled := sonRunStalls(t, "adversary", kadversaryMatchups, 300, true, nil, kadversaryCheck(&tal))
	t.Logf("%d games: announcements by the largest count offered (1..8): %v; %d stalled loud on an eighth payable repetition", tal.games, tal.top[1:], stalled)
	if stalled > 12 {
		t.Errorf("%d games stalled on StallNumTimes; want at most 12", stalled)
	}
	if tal.top[1] < 20 || tal.top[2] < 20 || tal.top[3] < 5 {
		t.Errorf("announcements by largest count %v (want >= 20 of 1, 20 of 2, 5 of 3)", tal.top[1:])
	}
}

// TestAdversaryLockstepMutation: with the announcement dropped, some seed
// diverges.
func TestAdversaryLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeNumTimesFixTime) {
		t.Skip("gorge before the Announce$ NumTimes fix (a2c2e9ddf)")
	}
	n := sonRun(t, "adversary", kadversaryMatchups, 100, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtNumTimes {
						p[x] = kop.Instr{Op: kop.OpNop}
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}, nil)
	if n == 0 {
		t.Fatal("no seed diverged with the announcement dropped")
	}
}
