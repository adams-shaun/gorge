package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Tishana's Tidebinder lockstep: the flash 3/2's enter trigger
// counters an activated or triggered ability (Prodigal Sorcerer's ping,
// Elvish Visionary's draw, Mind Stone's sacrifice-draw), and the countered
// ability's artifact or creature source loses all abilities while the
// Tidebinder stays (Ext CounterLose). The expected answer at every decision
// is gorge's.
var ktideDecks = [2][]cardset.Entry{
	{
		{Name: "Tishana's Tidebinder", Count: 14}, {Name: "Elvish Visionary", Count: 2}, {Name: "Mind Stone", Count: 4},
		{Name: "Island", Count: 14}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Prodigal Sorcerer", Count: 6}, {Name: "Pestermite", Count: 10}, {Name: "Mind Stone", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 10}, {Name: "Mountain", Count: 6},
	},
}

// gorgeTideFixTime is gorge son-tidebinder-host (5137d4811; after son-counter-remember-source 890505b74): Counter
// RememberCountered$ remembers a countered ability's SOURCE card.
const gorgeTideFixTime = "20261026173435"

// ktideRun plays seeds 1..n and returns the divergent games and the
// number of graveyard-to-stack casts on gorge's side.
func ktideRun(t *testing.T, n uint64, report bool, mut func(*kop.Table)) (div, grave int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tide")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range ktideDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	tbl, _, _, err := set.Matchup(ktideDecks[0], ktideDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut != nil {
		mut(tbl)
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"td-a", "td-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := castFirstPolicy(seed)
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		seen := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				if ev := evs[seen]; ev.Kind == events.MoveZone && ev.From == state.ZStack && ev.To == state.ZExile && ev.Text == "countered" {
					grave++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				return div, grave
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div, grave
}

// TestTideLockstep: countering another Tidebinder's trigger must not strip
// the host Tidebinder (gorge son-tidebinder-host, 5137d4811): seeds 168, 304,
// 544 of the second matchup deck diverged before it.
func TestTideLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeTideFixTime) {
		t.Skip("gorge before son-tidebinder-host (5137d4811)")
	}
	_, grave := ktideRun(t, 600, true, nil)
	t.Logf("600 games: %d abilities countered", grave)
	if grave < 60 {
		t.Errorf("%d abilities countered; want at least 60", grave)
	}
}

// TestTideLockstepMutation: with the counter dropped, some seed
// diverges.
func TestTideLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeTideFixTime) {
		t.Skip("gorge before son-tidebinder-host (5137d4811)")
	}
	n, _ := ktideRun(t, 300, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtCounterLose {
						p[x] = kop.Instr{Op: kop.OpNop}
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the counter dropped")
	}
}
