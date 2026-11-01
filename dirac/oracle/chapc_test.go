package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Saga chapter with its own Cost$ lockstep: Summon:
// G.F. Ifrit chapters I and II ("you may discard a card. If you do, draw a
// card") and The Huntsman's Redemption chapter II ("you may sacrifice a
// creature. If you do, search ...") open gorge's triggered-cost window
// (rules/saga.go, stack.go): pay, or decline and the body does not run. The
// check counts, from gorge's side, every such chapter ability that was paid
// (the Discard / Sacrifice cost event) and every one that was declined.

var kchapcDecks = [2][]cardset.Entry{
	{
		{Name: "Summon: G.F. Ifrit", Count: 4}, {Name: "The Huntsman's Redemption", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Giant Spider", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Vampire Cutthroat", Count: 3},
		{Name: "Air Elemental", Count: 3}, {Name: "Disperse", Count: 4}, {Name: "Island", Count: 11},
		{Name: "Swamp", Count: 11},
	},
}

type kchapcTally struct {
	paid, declined int
}

// kchapcWindow reports whether the gorge stack top is a chapter ability of
// Ifrit (chapters I, II) or the Huntsman's Redemption (chapter II), the
// ability a pay/decline decision then belongs to.
func kchapcWindow(g *GorgeSide) (id state.ObjID, ok bool) {
	st := g.E.G.Stack
	if len(st) == 0 {
		return 0, false
	}
	id = st[len(st)-1]
	o := g.E.G.Obj(id)
	if o == nil || o.Ability == nil {
		return 0, false
	}
	src := g.E.G.Obj(o.Source)
	if src == nil || src.Face() == nil {
		return 0, false
	}
	switch src.Face().Name {
	case "Summon: G.F. Ifrit":
		return id, src.Counter("LORE") <= 2
	case "The Huntsman's Redemption":
		return id, src.Counter("LORE") == 2
	}
	return 0, false
}

func kchapcPolicy(seed uint64, tal *kchapcTally) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		a := rnd.Pick(c)
		if c.G == nil || c.D.Kind != DChoose {
			return a
		}
		// a pay/decline choice posed to a chapter ability: both options on
		// the table (a lone decline is a cost that cannot be paid)
		pay, decline := "choose:trigger_cost_pay", "choose:trigger_cost_decline"
		var hasPay bool
		for _, o := range c.D.Options {
			hasPay = hasPay || strings.HasPrefix(o.String(), pay)
		}
		if !hasPay {
			return a
		}
		if _, ok := kchapcWindow(c.G); ok {
			switch {
			case strings.HasPrefix(a.String(), pay):
				tal.paid++
			case strings.HasPrefix(a.String(), decline):
				tal.declined++
			}
		}
		return a
	})
}

func kchapcRun(t *testing.T, n uint64, mut func(*kop.Instr)) (tal kchapcTally, div int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("chapc")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kchapcDecks {
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
	tbl, _, _, err := set.Matchup(kchapcDecks[0], kchapcDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mut != nil {
		for ci := range tbl.Cards {
			for ai := range tbl.Cards[ci].Abilities {
				for ii := range tbl.Cards[ci].Abilities[ai].Prog {
					mut(&tbl.Cards[ci].Abilities[ai].Prog[ii])
				}
			}
		}
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kchapc-a", "kchapc-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kchapcPolicy(seed, &tal), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40})
		CloseSide(k)
		if res.Stall != "" && res.Stall != "turns" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div++
			if mut == nil {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return tal, div
}

func TestChapcLockstep(t *testing.T) {
	t.Parallel()
	tal, _ := kchapcRun(t, 256, nil)
	t.Logf("chapter costs paid %d, declined %d", tal.paid, tal.declined)
	if tal.paid < 15 || tal.declined < 15 {
		t.Errorf("paid %d, declined %d; want at least 15 of each", tal.paid, tal.declined)
	}
}

// TestChapcMutation: with every OpIfPaid keeping only its pay kind (skip 0:
// the body runs on a decline) some seed diverges.
func TestChapcMutation(t *testing.T) {
	t.Parallel()
	_, div := kchapcRun(t, 64, func(in *kop.Instr) {
		if in.Op == kop.OpIfPaid {
			in.C &= 0xF
		}
	})
	if div == 0 {
		t.Fatal("running the body on a decline diverged in no game: the lockstep does not see the chapter cost")
	}
	t.Logf("%d of 64 seeds diverge under the mutation", div)
}
