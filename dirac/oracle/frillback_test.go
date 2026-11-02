package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
)

// gorgeNumTimesFixTime is the commit time (UTC) of gorge, the merge
// of the fix (Announce$ NumTimes asks 0..AnnounceMax and
// charges N x the unit, CR 601.2b): before it the `Mana<G\NumTimes>` cost fell
// to a generic {1} and the reflexive charm saw one repetition.
const gorgeNumTimesFixTime = "20261026165708"

// The Tranquil Frillback lockstep: "When it enters, you may pay {G}
// up to three times. When you pay this cost one or more times, choose up to
// that many -- destroy target artifact or enchantment; exile target player's
// graveyard; you gain 4 life." Forest-heavy decks cast it with mana to spare;
// Pacifism and Bonesplitter give the first mode targets. The expected answer
// at every decision is gorge's.
var kfrillbackMatchups = [][2][]cardset.Entry{{
	{
		{Name: "Tranquil Frillback", Count: 10}, {Name: "Grizzly Bears", Count: 4}, {Name: "Pacifism", Count: 2},
		{Name: "Forest", Count: 24},
	},
	{
		{Name: "Pacifism", Count: 4}, {Name: "Bonesplitter", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Shock", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Mountain", Count: 10},
	},
}, {
	{
		{Name: "Tranquil Frillback", Count: 8}, {Name: "Bonesplitter", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Forest", Count: 14}, {Name: "Mountain", Count: 10},
	},
	{
		{Name: "Tranquil Frillback", Count: 8}, {Name: "Bonesplitter", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Forest", Count: 14}, {Name: "Mountain", Count: 10},
	},
}}

// kfrillbackTally counts, on gorge's side, repetition announcements, the
// announced counts' downstream charm asks (a mode ask of max 1, 2 and 3) and
// games.
type kfrillbackTally struct {
	asks, games int
	modes       [4]int
}

func kfrillbackCheck(tal *kfrillbackTally) func(*GorgeSide, Side) *Divergence {
	var cur *GorgeSide
	var lastAsk *decision.Decision
	return func(ref *GorgeSide, _ Side) *Divergence {
		if ref != cur {
			cur, lastAsk = ref, nil
			tal.games++
		}
		d := ref.E.Pending()
		if d == nil || d == lastAsk {
			return nil
		}
		lastAsk = d
		if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "trigger_cost_x" {
			tal.asks++
		}
		if d.Kind == decision.KModes && d.Min == 0 && d.Max >= 1 && d.Max <= 3 && len(d.Options) >= 3 {
			tal.modes[d.Max]++
		}
		return nil
	}
}

func TestFrillbackLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeNumTimesFixTime) {
		t.Skip("gorge before the Announce$ NumTimes fix (a2c2e9ddf)")
	}
	var tal kfrillbackTally
	sonRun(t, "frillback", kfrillbackMatchups, 300, true, nil, kfrillbackCheck(&tal))
	t.Logf("%d games: %d announcements, charm asks up to 1/2/3 modes: %v", tal.games, tal.asks, tal.modes[1:])
	if tal.asks < 100 || tal.modes[1] < 10 || tal.modes[2] < 10 || tal.modes[3] < 10 {
		t.Errorf("announcements %d (want >= 100), charm asks %v (want >= 10 of each size)", tal.asks, tal.modes[1:])
	}
}

// TestFrillbackLockstepMutation: with the announcement dropped, some seed
// diverges.
func TestFrillbackLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeNumTimesFixTime) {
		t.Skip("gorge before the Announce$ NumTimes fix (a2c2e9ddf)")
	}
	n := sonRun(t, "frillback", kfrillbackMatchups, 100, false, func(tbl *kop.Table) {
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
