package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// "You gain life equal to the damage dealt this way" (Corrupt): gorge reads the
// spell's own Count$TotalDamageDoneByThisTurn, the damage that landed after
// prevention (a Crystal Barricade makes a hit on another creature of its controller deal none), and kgen lowers the read
// to the kernel's last-damage value when the spell's one DealDamage is the
// program before it. TestCorruptLastDamageLockstep plays the card against gorge
// on 300 seeds; TestCorruptLastDamageMutation shows a gain that ignores the
// damage diverges, so the lockstep reads the amount.
var corruptDecks = [2][]cardset.Entry{
	{
		{Name: "Corrupt", Count: 8}, {Name: "Crystal Barricade", Count: 4}, {Name: "Craw Wurm", Count: 4},
		{Name: "Swamp", Count: 14}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Corrupt", Count: 6}, {Name: "Crystal Barricade", Count: 4}, {Name: "Craw Wurm", Count: 6},
		{Name: "Swamp", Count: 14}, {Name: "Plains", Count: 6},
	},
}

// corruptRun plays n games and returns the divergences and, from gorge's
// events, how many Corrupt resolutions gained life and how many gained none (a
// prevented or fizzled hit).
func corruptRun(t *testing.T, n uint64, mut func(*kop.Table)) (div, gained, none int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range corruptDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
				names[s] = append(names[s], e.Name)
			}
		}
	}
	set := printedDelveSet(t, [2][]string{{"Corrupt", "Crystal Barricade", "Craw Wurm", "Swamp", "Plains"}})
	tbl, _, _, err := set.Matchup(corruptDecks[0], corruptDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut != nil {
		mut(tbl)
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"corrupt-a", "corrupt-b"}, gd, 1)
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
		inCorrupt := false
		flush := func() {
			if inCorrupt {
				none++
			}
			inCorrupt = false
		}
		check := func(ref *GorgeSide, _ Side) *Divergence {
			evs := ref.E.L.Events
			for ; seen < len(evs); seen++ {
				switch ev := evs[seen]; ev.Kind {
				case events.Resolve:
					flush()
					o := ref.E.G.Obj(ev.Obj)
					inCorrupt = o != nil && o.Face() != nil && o.Face().Name == "Corrupt"
				case events.LifeChange:
					if inCorrupt && ev.Amount > 0 {
						gained++
						inCorrupt = false
					}
				case events.StepChange, events.TurnChange:
					flush()
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if mut != nil {
				return div, gained, none
			}
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div, gained, none
}

func TestCorruptLastDamageLockstep(t *testing.T) {
	t.Parallel()
	_, gained, none := corruptRun(t, 300, nil)
	t.Logf("300 games: %d Corrupt resolutions gained life, %d gained none", gained, none)
	if gained < 20 || none < 3 {
		t.Errorf("%d Corrupt resolutions gained life and %d none; want at least 20 and 3", gained, none)
	}
}

func TestCorruptLastDamageMutation(t *testing.T) {
	t.Parallel()
	n, gained, _ := corruptRun(t, 300, func(tbl *kop.Table) {
		dropped := 0
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Op == kop.OpGainLife && p[x].A.K == kop.VLastDamage {
						p[x].A = kop.Const(0)
						dropped++
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		if dropped == 0 {
			t.Fatal("precondition: no GainLife reads the last damage in the table")
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatalf("no seed diverged with the gain dropped (%d gains seen)", gained)
	}
}
