package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: Make Your Own Luck (a Dig whose
// ChangeValidDesc caption gorge never reads) and Way of the Pyromancer (a
// granted "[+1]: Add {R}" whose AINoRecursiveCheck hint gorge never reads)
// beside Ajani, Caller of the Pride, vanilla creatures, burn and basics, so
// the expected answer at every decision is gorge's.
var kcosmeticDecks = [2][]cardset.Entry{
	{
		{Name: "Make Your Own Luck", Count: 4}, {Name: "Way of the Pyromancer", Count: 4}, {Name: "Ajani, Caller of the Pride", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2}, {Name: "Shock", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Way of the Pyromancer", Count: 4}, {Name: "Ajani, Caller of the Pride", Count: 2}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 6}, {Name: "Swamp", Count: 4},
	},
}

type kcosmeticStats struct {
	games, divs, stalls int
	luck, redMana       int
}

// kcosmeticPolicy casts or activates whenever it can (so Make Your Own Luck
// resolves and the granted loyalty mana ability is used), else random. It
// counts the picks it makes: no card in the pool counters a spell, so a cast
// of Make Your Own Luck resolves.
func kcosmeticPolicy(seed uint64, st *kcosmeticStats) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		var pick Canon
		if len(buf) == 0 {
			pick = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			pick = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if k := pick.Kind(); k == ActCast || k == ActActivate {
			if o, ok := c.G.GorgeOption(pick); ok {
				if k == ActCast && c.G.E.G.Obj(o.Obj) != nil && c.G.E.G.Obj(o.Obj).Face() != nil && c.G.E.G.Obj(o.Obj).Face().Name == "Make Your Own Luck" {
					st.luck++
				}
				if k == ActActivate && strings.Contains(o.Label, "Add {R}") && c.G.E.IsPlaneswalker(o.Obj) {
					st.redMana++
				}
			}
		}
		return pick
	})
}

func TestCosmeticLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cosmetic")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kcosmeticDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kcosmeticDecks[0], kcosmeticDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st := &kcosmeticStats{}
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcosmetic-a", "kcosmetic-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kcosmeticPolicy(seed, st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		st.games++
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			st.stalls++
			t.Errorf("seed %d: kernel stall %q", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			st.divs++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d divergences, %d stalls, Make Your Own Luck cast %d, \"Add {R}\" activations %d", st.games, st.divs, st.stalls, st.luck, st.redMana)
	if st.divs != 0 || st.stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", st.divs, st.stalls)
	}
	if st.luck < 10 {
		t.Errorf("Make Your Own Luck resolved only %d times", st.luck)
	}
	if st.redMana < 5 {
		t.Errorf("the granted \"[+1]: Add {R}\" was activated only %d times", st.redMana)
	}
}
