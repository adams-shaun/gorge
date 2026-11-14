package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC combat-7 spelling family: S:Mode$ CantAttack | ValidCard$ Card.Self
// | Target$ Player.CardOwner,Planeswalker.ControlledBy Player.CardOwner
// (Xantcha, Sleeper Agent: it can't attack its owner or the planeswalkers its
// owner controls). Xantcha itself also needs the ChoosePlayer replacement, a
// param-Draw Activator$ and a CardController LoseLife, none of them lowered
// here, so the carrier is an inline creature holding only that static; the
// stealing is Control Magic's, so the creature is controlled by a player who
// is not its owner and the defender, the controller's opponent, is the owner.
//
// Both engines must agree at every declare-attackers decision: the creature
// is offered as an attacker to its owner (controller == owner) and to nobody
// else while an opponent controls it.

// combat7Cards are the inline scripts of the mini corpus the pool lowers from
// (the registry gorge plays and the table kgen builds read the same files).
var combat7Cards = map[string]string{
	"reluctant_sleeper": "Name:Reluctant Sleeper\nManaCost:1 R\nTypes:Creature Phyrexian Minion\nPT:3/3\n" +
		"S:Mode$ CantAttack | ValidCard$ Card.Self | Target$ Player.CardOwner,Planeswalker.ControlledBy Player.CardOwner | Description$ CARDNAME can't attack its owner or planeswalkers its owner controls.\n" +
		"Oracle:Reluctant Sleeper can't attack its owner or planeswalkers its owner controls.\n",
	"island_drifter": "Name:Island Drifter\nManaCost:1 U\nTypes:Creature Merfolk\nPT:2/2\nOracle:\n",
	"hill_giant":     "Name:Hill Giant\nManaCost:3 R\nTypes:Creature Giant\nPT:3/3\nOracle:\n",
	"mountain":       "Name:Mountain\nManaCost:no cost\nTypes:Basic Land Mountain\nOracle:({T}: Add {R}.)\n",
	"island":         "Name:Island\nManaCost:no cost\nTypes:Basic Land Island\nOracle:({T}: Add {U}.)\n",
	"mind_grip": "Name:Mind Grip\nManaCost:2 U U\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ Continuous | Description$ Enchanted creature answers to you. | GainControl$ You | AffectedDefined$ Enchanted\n" +
		"Oracle:Enchant creature\\nYou control enchanted creature.\n",
}

var kcombat7Decks = [2][]cardset.Entry{
	{
		{Name: "Reluctant Sleeper", Count: 14}, {Name: "Hill Giant", Count: 6}, {Name: "Mountain", Count: 20},
	},
	{
		{Name: "Mind Grip", Count: 14}, {Name: "Island Drifter", Count: 6}, {Name: "Island", Count: 20},
	},
}

type combat7Tally struct {
	stolenDecls int // attack declarations of a seat controlling the sleeper it does not own
	ownOffered  int // attack declarations offering the sleeper to its owner
	stolenAsked int // the sleeper offered as an attacker to the seat that stole it: must stay 0
	stolen      int // games reaching a stolen sleeper
}

func combat7Policy(seed uint64, t *combat7Tally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActActivate:
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) > 0 {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		} else {
			a = rnd.Pick(c)
		}
		if c.G != nil {
			combat7Count(c, t)
		}
		return a
	})
}

func combat7Count(c *PickCtx, t *combat7Tally) {
	pd := c.G.Pending()
	if pd == nil || pd.Kind != decision.KAttackers {
		return
	}
	g := c.G.E.G
	offered := map[state.ObjID]bool{}
	for _, o := range pd.Options {
		if o.Kind == "attacker" {
			offered[o.Obj] = true
		}
	}
	var ids []state.ObjID
	for p := range g.Players {
		ids = append(ids, g.Zone(state.ZBattlefield, state.PlayerID(p))...)
	}
	for _, id := range ids {
		o := g.Obj(id)
		if o == nil || o.Face() == nil || o.Face().Name != "Reluctant Sleeper" || o.Controller != state.PlayerID(pd.Player) {
			continue
		}
		switch {
		case o.Owner != o.Controller:
			t.stolenDecls++
			if offered[id] {
				t.stolenAsked++
			}
		case offered[id]:
			t.ownOffered++
		}
	}
}

// combat7Pool writes the mini corpus, builds its kernel table with kgen and
// returns the gorge registry, the set and the decks.
func combat7Pool(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card) {
	t.Helper()
	dir := t.TempDir()
	folder := filepath.Join(dir, "cardsfolder", "x")
	if err := os.MkdirAll(folder, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, src := range combat7Cards {
		if err := os.WriteFile(filepath.Join(folder, name+".txt"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	list := filepath.Join(dir, "cards.txt")
	names := "Reluctant Sleeper\nIsland Drifter\nHill Giant\nMountain\nIsland\nMind Grip\n"
	if err := os.WriteFile(list, []byte(names), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if code := kgen.Main([]string{"-cards", dir, "-card-list", list, "-pool", "combat7", "-out", out}); code != 0 {
		t.Fatalf("kgen: exit %d", code)
	}
	pd, err := cardset.LoadFile(filepath.Join(out, "combat7.ktab"))
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(cardset.NewDataSet(pd))
	reg, err := cards.OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kcombat7Decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the mini corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	return reg, set, gd
}

func TestMBCCombat7Lowers(t *testing.T) {
	t.Parallel()
	reg, set, gd := combat7Pool(t)
	tbl, _, _, err := set.Matchup(kcombat7Decks[0], kcombat7Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	tally := &combat7Tally{}
	for seed := uint64(1); seed <= 150; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"combat7-a", "combat7-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		before := tally.stolenDecls
		res := Lockstep(m, seed, k, combat7Policy(seed, tally), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if tally.stolenDecls > before {
			tally.stolen++
		}
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%+v", *tally)
	if tally.stolenDecls < 10 || tally.ownOffered < 10 {
		t.Fatalf("the carrier was not exercised: %d declarations by a seat controlling the stolen sleeper, %d offering it to its owner, want at least 10 of each", tally.stolenDecls, tally.ownOffered)
	}
	if tally.stolenAsked != 0 {
		t.Errorf("the sleeper was offered as an attacker %d times to the seat that stole it", tally.stolenAsked)
	}
}
