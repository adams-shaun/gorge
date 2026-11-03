package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Assimilation Aegis lockstep: the Equipment's enter trigger
// exiles a creature until it leaves, and each equip makes the bearer a copy
// of the exiled card (TrigAttached, Ext AegisClone) until it is unattached
// (the Equipment moving, leaving, or its bearer leaving). The expected answer
// at every decision is gorge's.
var kaegisDecks = [2][]cardset.Entry{
	{
		{Name: "Assimilation Aegis", Count: 6}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Serra Angel", Count: 3},
		{Name: "Elvish Visionary", Count: 4}, {Name: "Savannah Lions", Count: 3}, {Name: "Mind Stone", Count: 2}, {Name: "Shatter", Count: 3},
		{Name: "Plains", Count: 10}, {Name: "Island", Count: 8},
	},
	{
		{Name: "Assimilation Aegis", Count: 3}, {Name: "Pestermite", Count: 4}, {Name: "Prodigal Sorcerer", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Serra Angel", Count: 2}, {Name: "Savannah Lions", Count: 3}, {Name: "Shatter", Count: 3},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 5}, {Name: "Forest", Count: 3}, {Name: "Mountain", Count: 2},
	},
}

// gorgeAegisFixTime is gorge son-exiledwith-cr4007 / son-clone-untilunattached
// (73fafdba3, 79bd6874a), both before 71304b28d (2026-10-26 17:36:20Z).
const gorgeAegisFixTime = "20261026173620"

// kaegisCounts is what the gorge side did over the run.
type kaegisCounts struct{ attaches, clones, ends int }

func kaegisRun(t *testing.T, n uint64, report bool, mut func(*kop.Table)) (div int, c kaegisCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("aegis")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kaegisDecks {
		for _, e := range d {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], card)
			}
		}
	}
	tbl, _, _, err := set.Matchup(kaegisDecks[0], kaegisDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut != nil {
		mut(tbl)
	}
	for seed := uint64(1); seed <= n; seed++ {
		m, err := NewMatchup(reg, [2]string{"ag-a", "ag-b"}, gd, 1)
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
				switch ev := evs[seen]; {
				case ev.Kind == events.Attach && len(ev.IDs) > 0 && ref.E.G.Obj(ev.Obj) != nil && ref.E.G.Obj(ev.Obj).Face() != nil && ref.E.G.Obj(ev.Obj).Face().Name == "Assimilation Aegis":
					c.attaches++
				case ev.Kind == events.ClonePermanent:
					c.clones++
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				return div, c
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return div, c
}

// TestAegisLockstep: 600 games, 0 divergences, with real equips and copies.

func TestAegisLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeAegisFixTime) {
		t.Skip("gorge before son-clone-untilunattached / son-exiledwith-cr4007")
	}
	_, c := kaegisRun(t, 600, true, nil)
	t.Logf("600 games: %d Aegis attaches, %d copies", c.attaches, c.clones)
	if c.attaches < 1000 || c.clones < 1000 {
		t.Errorf("%d attaches, %d copies; want at least 1000 of each", c.attaches, c.clones)
	}
}

// TestAegisLockstepMutation: with the copy dropped, or never ended, some
// seed diverges.
func TestAegisLockstepMutation(t *testing.T) {
	// not parallel: it flips dirac.DebugAegisNoUnlink
	if !gorgePinAtLeast(t, gorgeAegisFixTime) {
		t.Skip("gorge before son-clone-untilunattached / son-exiledwith-cr4007")
	}
	n, _ := kaegisRun(t, 300, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtAegisClone {
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
		t.Fatal("no seed diverged with the copy dropped")
	}
	dirac.DebugAegisNoUnlink = true
	defer func() { dirac.DebugAegisNoUnlink = false }()
	if n, _ := kaegisRun(t, 300, false, nil); n == 0 {
		t.Fatal("no seed diverged with the copy never ended")
	}
}
