package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Dauntless Bodyguard (MBC): "As this creature enters, choose another
// creature you control. Sacrifice this creature: The chosen creature gains
// indestructible until end of turn." gorge asks the as-enters ChooseCard as a
// KChoose of "card" options (none with no other creature) and its
// sacrifice-cost Pump reads the choice through Card.ChosenCardStrict from the
// sacrificed source's last known information (CR 607.2a, 608.2h). The kernel
// lowers the choice to ExtChooseCardEnter (dirac/repl_choosecard.go) and the
// Pump to a keyword grant over the seeded chosen object. The lockstep plays the
// bodyguard fixture pool, where Murder and Day of Judgment make the sacrifice
// matter, and counts the asks and the protected creatures gorge shows.

const bodyguard = "Dauntless Bodyguard"

var kbodyguardDecks = [2][]cardset.Entry{
	{
		{Name: bodyguard, Count: 10}, {Name: "Savannah Lions", Count: 8},
		{Name: "Day of Judgment", Count: 4}, {Name: "Plains", Count: 18},
	},
	{
		{Name: bodyguard, Count: 8}, {Name: "Savannah Lions", Count: 6}, {Name: "Murder", Count: 6},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10},
	},
}

// bodyguardTally is what the gorge side showed across the run.
type bodyguardTally struct {
	asks      int // as-enters creature choices posed for a Bodyguard
	activated int // Bodyguard sacrifice activations chosen
	protected int // creatures gorge showed indestructible (only the Pump grants it here)
	seen      map[[2]int]bool
}

// bodyguardPolicy casts or activates whenever it can, else picks at random,
// and tallies the Bodyguard's asks, activations and protected creatures.
func bodyguardPolicy(seed uint64, tl *bodyguardTally) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil {
			bodyguardObserve(g, tl)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		a := Canon(0)
		if len(buf) == 0 || c.Seq%3 == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActActivate && optionFaceName(c, a) == bodyguard {
			tl.activated++
		}
		return a
	})
}

// bodyguardObserve counts gorge's pending Bodyguard choice and the creatures
// it shows indestructible, each (turn, object) once.
func bodyguardObserve(g *GorgeSide, tl *bodyguardTally) {
	if d := g.E.Pending(); d != nil && d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "card" {
		if src := g.E.G.Obj(d.Source); src != nil && src.Face() != nil && src.Face().Name == bodyguard {
			tl.asks++
		}
	}
	for i := range g.E.G.Objs {
		o := &g.E.G.Objs[i]
		if o.Zone != state.ZBattlefield || !g.E.HasKeyword(o.ID, "Indestructible") {
			continue
		}
		key := [2]int{int(g.E.G.Turn), int(o.ID)}
		if !tl.seen[key] {
			tl.seen[key] = true
			tl.protected++
		}
	}
}

// bodyguardLowered reports whether the Bodyguard's table card carries the
// as-enters ExtChooseCardEnter and an activated keyword grant over the chosen
// object.
func bodyguardLowered(tbl *kop.Table) bool {
	for c := range tbl.Cards {
		cd := &tbl.Cards[c]
		if cd.Name != bodyguard {
			continue
		}
		enter, grant := false, false
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbReplacement && len(a.Prog) > 0 && a.Prog[0].Op == kop.OpExt && a.Prog[0].Ext() == kop.ExtChooseCardEnter {
				enter = true
			}
			if a.Kind == kop.AbActivated && len(a.Prog) > 0 && a.Prog[0].Op == kop.OpGrantKeyword && a.Prog[0].Obj == kop.RefChosen {
				grant = true
			}
		}
		return enter && grant
	}
	return false
}

func TestMBCChosenCardStrictLowers(t *testing.T) {
	t.Parallel()
	// the MBC draft pool plays the card, lowered
	mbc := cardsettest.Synth(t, "dr-mbc-bb")
	i, ok := mbc.Lookup(bodyguard)
	if !ok {
		t.Fatalf("%s is not in dr-mbc-bb", bodyguard)
	}
	if st := mbc.Status(i); st == cardset.StatusRefused {
		t.Fatalf("dr-mbc-bb refuses %s: %s", bodyguard, mbc.Reason(i))
	}
	mtbl, _, _, err := mbc.Matchup([]cardset.Entry{{Name: bodyguard, Count: 1}}, []cardset.Entry{{Name: bodyguard, Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if !bodyguardLowered(mtbl) {
		t.Fatalf("dr-mbc-bb's %s carries no ExtChooseCardEnter beside a chosen-object keyword grant", bodyguard)
	}

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("bodyguard")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kbodyguardDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for j := 0; j < e.Count; j++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kbodyguardDecks[0], kbodyguardDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if !bodyguardLowered(tbl) {
		t.Fatalf("the bodyguard pool's %s is not lowered", bodyguard)
	}
	tl := &bodyguardTally{seen: map[[2]int]bool{}}
	const seeds = 60
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"bodyguard-a", "bodyguard-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, bodyguardPolicy(seed, tl), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d Bodyguard choices asked, %d sacrifices, %d creatures protected", seeds, tl.asks, tl.activated, tl.protected)
	// the run reached the mechanism in both engines: the ask, the
	// sacrifice, and the chosen creature shown indestructible after it
	if tl.asks < 20 || tl.activated < 10 || tl.protected < 5 {
		t.Fatalf("too little evidence: %d asks, %d sacrifices, %d protected (want 20, 10, 5)", tl.asks, tl.activated, tl.protected)
	}
}
