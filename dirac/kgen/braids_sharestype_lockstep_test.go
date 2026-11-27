package kgen

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Braids, Arisen Nightmare (hosted decks hearthhull-worldseed-landfall and
// pro-shaper; dirac gap cli-20261127T065130Z-0be3787f): at your end step you
// may sacrifice an artifact, creature, enchantment, land or planeswalker; if
// you do, each opponent may sacrifice a permanent that shares a card type with
// it, and each opponent who does not loses 2 life while you draw a card. The
// census refusal was `DB$ RepeatEach: RepeatEach body RememberSacrificed$
// "True"`.
//
// The lowering test proves the form (the optional remembering Sacrifice, the
// gated per-opponent loop, the shares-a-type pick and the X-gated pay-off);
// the lockstep plays the real card, both seats running it, gorge leading the
// decisions. The scan is the precondition the comparison depends on: it fails
// the test outright when no seed ever reaches the opponent's election, or
// never sees an opponent take each of its two answers.

var braidsFillers = []string{"Grizzly Bears", "Hill Giant", "Ornithopter", "Glorious Anthem", "Mountain", "Swamp"}

func braidsDecks() [2][]cardset.Entry {
	d0 := []cardset.Entry{
		{Name: "Braids, Arisen Nightmare", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
		{Name: "Ornithopter", Count: 3},
		{Name: "Glorious Anthem", Count: 2},
		{Name: "Mountain", Count: 8},
		{Name: "Swamp", Count: 19},
	}
	d1 := []cardset.Entry{
		{Name: "Braids, Arisen Nightmare", Count: 4},
		{Name: "Hill Giant", Count: 4},
		{Name: "Ornithopter", Count: 3},
		{Name: "Glorious Anthem", Count: 2},
		{Name: "Mountain", Count: 9},
		{Name: "Swamp", Count: 18},
	}
	return [2][]cardset.Entry{slices.Clone(d0), slices.Clone(d1)}
}

// TestBraidsSharesTypeSacrificeLowering: Braids lowers with no refusal, the
// shares-a-type pick reads the remembered card, and the pay-off ops carry the
// X == 0 gate.
func TestBraidsSharesTypeSacrificeLowering(t *testing.T) {
	t.Parallel()
	ratchetCards(t)
	c := lowerCorpusCard(t, "Braids, Arisen Nightmare")
	for _, rf := range c.Refusals {
		t.Errorf("Braids refused: %s", rf.Reason)
	}
	sawRepeat, sawShare := false, false
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op == "RepeatEach" && in.Obj.Kind == rEachOpp {
				sawRepeat = true
			}
			if in.Op == "Sacrifice" && in.Obj.Kind == rEach && in.F != nil {
				for _, cl := range in.F.Clauses {
					for _, l := range cl.Spill[:cl.NSpill] {
						if l.W == kop.FwSharesCardType && l.R == kop.RefRemembered {
							sawShare = true
						}
					}
				}
			}
		}
	}
	if !sawRepeat || !sawShare {
		t.Errorf("Braids lowered no per-opponent shares-a-type Sacrifice (repeat %v share %v)", sawRepeat, sawShare)
	}
}

// braidsSeen records what the scan saw across the seeds.
type braidsSeen struct {
	oppAsk, oppSacrificed, oppDeclined bool
}

// braidsGame is one game's pending-election memory: the opponent's life and
// battlefield size when its ask appeared.
type braidsGame struct {
	armed        bool
	opp          state.PlayerID
	life         int32
	perms        int
	ownerHand    int
	askedAtCount int
}

func braidsPerms(g *state.Game, p state.PlayerID) int { return len(g.Zone(state.ZBattlefield, p)) }

func braidsScan(c *oracle.PickCtx, seen *braidsSeen, gm *braidsGame) {
	if c.G == nil {
		return
	}
	e := c.G.E
	g := e.G
	if gm.armed {
		gm.armed = false
		switch {
		case braidsPerms(g, gm.opp) < gm.perms:
			seen.oppSacrificed = true
		case g.Players[gm.opp].Life == gm.life-2 && len(g.Zone(state.ZHand, 1-gm.opp)) > gm.ownerHand:
			seen.oppDeclined = true
		}
	}
	d := e.Pending()
	if d == nil || !strings.Contains(d.Prompt, "to sacrifice, or none") {
		return
	}
	src := g.Obj(d.Source)
	if src == nil || src.Card == nil || src.Card.Faces[0].Name != "Braids, Arisen Nightmare" || d.Player == src.Controller {
		return
	}
	seen.oppAsk = true
	gm.armed, gm.opp = true, d.Player
	gm.life, gm.perms = g.Players[d.Player].Life, braidsPerms(g, d.Player)
	gm.ownerHand = len(g.Zone(state.ZHand, 1-d.Player))
}

func braidsPolicy(seed uint64, seen *braidsSeen) oracle.Policy {
	rnd := oracle.RandomPolicy(seed)
	var gm braidsGame
	return oracle.PolicyFunc(func(c *oracle.PickCtx) oracle.Canon {
		braidsScan(c, seen, &gm)
		buf := make([]oracle.Canon, 0, 8)
		for _, a := range c.D.Options {
			switch c.D.Kind {
			case oracle.DAttackers, oracle.DBlockers:
				if a.Kind() == oracle.ActDone || a.Kind() == oracle.ActPass {
					buf = append(buf, a) // no combat: keep the permanents for the election
				}
			default:
				switch a.Kind() {
				case oracle.ActCast, oracle.ActCastFace, oracle.ActActivate, oracle.ActPlayLand:
					buf = append(buf, a)
				}
			}
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&oracle.PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestBraidsSharesTypeSacrificeLockstep: Braids plays decision-for-decision
// with gorge over the seeds, and the opponent's election was genuinely posed
// and answered both ways.
func TestBraidsSharesTypeSacrificeLockstep(t *testing.T) {
	t.Parallel()
	const seeds = 40
	cardsDir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no corpus: %v", err)
	}
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	names := append([]string{"Braids, Arisen Nightmare"}, braidsFillers...)
	p, err := loadCardNames(names, cardsDir)
	if err != nil {
		t.Fatalf("fixture pool: %v", err)
	}
	pd, _, _, err := tableOf(p, "braids-fixture", nil)
	if err != nil {
		t.Fatalf("table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(pd))
	decks := braidsDecks()
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s := range gd {
		for _, e := range decks[s] {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	var seen braidsSeen
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := oracle.NewMatchup(reg, [2]string{"braids-a", "braids-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := oracle.Config{}
		k, err := oracle.SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := braidsPolicy(seed, &seen)
		if seed%8 == 0 {
			pol = oracle.RandomPolicy(seed)
		}
		res := oracle.Lockstep(m, seed, k, pol, oracle.RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30})
		oracle.CloseSide(k)
		if d := res.Div; d != nil {
			div++
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s", seed, d.At, d.What, d.Field, d.Diff, d.Detail)
		}
	}
	if !seen.oppAsk || !seen.oppSacrificed || !seen.oppDeclined {
		t.Fatalf("over %d seeds the election was posed/answered %+v: the lockstep never reached both outcomes", seeds, seen)
	}
	if div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	t.Logf("election seen %+v over %d seeds", seen, seeds)
}
