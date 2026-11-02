package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Case of the Uneaten Feast lockstep: solved by Inspiring
// Cleric's 4 life plus the Case's own 1, then sacrificed so the creature
// cards in its controller's graveyard (Shocked Clerics and Ambush Vipers,
// the flash ones castable on the opponent's turn) may be cast from there
// this turn (Ext MayPlayGrave). The expected answer at
// every decision is gorge's.
var kcasefeastDecks = [2][]cardset.Entry{
	{
		{Name: "Case of the Uneaten Feast", Count: 8}, {Name: "Inspiring Cleric", Count: 8}, {Name: "Ambush Viper", Count: 8},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Shock", Count: 10}, {Name: "Grizzly Bears", Count: 10}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

// gorgeCaseFeastFixTime is gorge cr-animate-grants-graveyard-mayplay
// (0be2d58dc): Animate staticAbilities$ grants a self MayPlay$ True
// Continuous static to the graveyard cards it names.
const gorgeCaseFeastFixTime = "20261026150640"

// kcasefeastRun plays seeds 1..n and returns the divergent games and the
// number of graveyard-to-stack casts on gorge's side.
func kcasefeastRun(t *testing.T, n uint64, report bool, mut func(*kop.Table)) (div, grave int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("casefeast")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kcasefeastDecks {
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
	tbl, _, _, err := set.Matchup(kcasefeastDecks[0], kcasefeastDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut != nil {
		mut(tbl)
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"cf-a", "cf-b"}, gd, 1)
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
				if ev := evs[seen]; ev.Kind == events.PutOnStack && ev.From == state.ZGraveyard {
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

func TestCasefeastLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeCaseFeastFixTime) {
		t.Skip("gorge before the Animate self MayPlay grant (0be2d58dc)")
	}
	_, grave := kcasefeastRun(t, 600, true, nil)
	t.Logf("600 games: %d creature casts from the graveyard", grave)
	if grave < 300 {
		t.Errorf("%d casts from the graveyard; want at least 300", grave)
	}
}

// TestCasefeastLockstepMutation: with the permissions dropped, some seed
// diverges.
func TestCasefeastLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeCaseFeastFixTime) {
		t.Skip("gorge before the Animate self MayPlay grant (0be2d58dc)")
	}
	n, _ := kcasefeastRun(t, 300, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtMayPlayGrave {
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
		t.Fatal("no seed diverged with the permissions dropped")
	}
}
