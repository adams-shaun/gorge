package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The dynamic charm count lockstep (CR 700.2, 700.11):
// a spell charm whose CharmNum$ is a Count$Compare. Molten Collapse chooses
// both modes once its controller descended this turn (a permanent card
// entered their graveyard); Wail of the Forgotten chooses up to three at
// eight permanent cards in the graveyard. gorge's CharmModeBounds evaluates
// the count at each ask; the counters below recompute it from gorge's own
// state at the modes decision, so a kernel that mis-evaluates it diverges.
var kcharmcmpDecks = [2][]cardset.Entry{
	{
		{Name: "Molten Collapse", Count: 4}, {Name: "Wail of the Forgotten", Count: 2},
		{Name: "Stitcher's Supplier", Count: 3}, {Name: "Village Rites", Count: 3},
		{Name: "Thought Scour", Count: 2}, {Name: "Grisly Salvage", Count: 2},
		{Name: "Diregraf Ghoul", Count: 2}, {Name: "Witch's Oven", Count: 1},
		{Name: "Wayfarer's Bauble", Count: 1}, {Name: "Shock", Count: 1},
		{Name: "Hill Giant", Count: 1}, {Name: "Vampiric Rites", Count: 1}, {Name: "Gray Ogre", Count: 1},
		{Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 5}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Wail of the Forgotten", Count: 4}, {Name: "Molten Collapse", Count: 2},
		{Name: "Stitcher's Supplier", Count: 3}, {Name: "Thought Scour", Count: 2},
		{Name: "Grisly Salvage", Count: 2}, {Name: "Hill Giant", Count: 1}, {Name: "Gray Ogre", Count: 1}, {Name: "Village Rites", Count: 2},
		{Name: "Diregraf Ghoul", Count: 2}, {Name: "Pitiless Plunderer", Count: 1},
		{Name: "Blood Fountain", Count: 1}, {Name: "Cathartic Pyre", Count: 1},
		{Name: "Coral Merfolk", Count: 2}, {Name: "Ornithopter", Count: 1},
		{Name: "Swamp", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 3},
	},
}

type kcharmcmpTally struct {
	evAt    int
	mSeq    uint64
	mSource state.ObjID
	mName   string
	mCap    int
	mLabels []string

	molten     [3]int // Molten Collapse modes decisions by cap (1, 2)
	wail       [4]int // Wail of the Forgotten by cap (1, 3)
	moltenBoth int    // Molten Collapse casts that chose both modes
	wailMulti  int    // Wail casts that chose two or more modes
}

// kcharmcmpCap recomputes the pick count from gorge's state for the player
// asked, as the printed card reads it.
func kcharmcmpCap(g *state.Game, name string, p state.PlayerID) int {
	switch name {
	case "Molten Collapse":
		for _, en := range g.Entered {
			if en.To == state.ZGraveyard && en.Owner == p && en.PermanentCard {
				return 2
			}
		}
		return 1
	case "Wail of the Forgotten":
		n := 0
		for _, id := range g.Zone(state.ZGraveyard, p) {
			o := g.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			perm := true
			for _, t := range o.Face().Types {
				if t == "Instant" || t == "Sorcery" {
					perm = false
				}
			}
			if perm {
				n++
			}
		}
		if n >= 8 {
			return 3
		}
		return 1
	}
	return 0
}

func (c *kcharmcmpTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.ModeChosen || c.mName == "" {
			continue
		}
		hits := 0
		for _, l := range c.mLabels {
			if l != "" && strings.Contains(e.Text, l) {
				hits++
			}
		}
		if hits >= 2 {
			switch c.mName {
			case "Molten Collapse":
				c.moltenBoth++
			case "Wail of the Forgotten":
				c.wailMulti++
			}
		}
		c.mName = ""
	}
	c.evAt = len(evs)
	if fr := ref.fr; fr != nil && fr.d != nil && string(fr.d.Kind) == "modes" && fr.d.Seq != c.mSeq {
		c.mSeq, c.mSource, c.mName = fr.d.Seq, fr.d.Source, ""
		if o := g.Obj(fr.d.Source); o != nil && o.Face() != nil {
			if n := o.Face().Name; n == "Molten Collapse" || n == "Wail of the Forgotten" {
				c.mName = n
				c.mCap = kcharmcmpCap(g, n, fr.d.Player)
				c.mLabels = c.mLabels[:0]
				for _, o := range fr.d.Options {
					c.mLabels = append(c.mLabels, o.Label)
				}
				if n == "Molten Collapse" {
					c.molten[c.mCap]++
				} else {
					c.wail[c.mCap]++
				}
			}
		}
	}
	return nil
}

// kcharmcmpPolicy casts and plays whenever it can on 3 of every 4 seeds, so
// the charms and the graveyard fillers are reached; the rest play random.
func kcharmcmpPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DPriority && seed%4 != 0 {
			buf = buf[:0]
			for _, a := range p.D.Options {
				if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *p.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
			}
		}
		return rnd.Pick(p)
	})
}

func kcharmcmpSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][2][]*cards.Card, [2]*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("charmcmp")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][2][]*cards.Card
	var tbls [2]*kop.Table
	for mi := 0; mi < 2; mi++ {
		md := [2][]cardset.Entry{kcharmcmpDecks[mi], kcharmcmpDecks[1-mi]}
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	return reg, set, decks, tbls
}

func kcharmcmpRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"kcharmcmp-a", "kcharmcmp-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, seed, k, kcharmcmpPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 14, Check: check})
	CloseSide(k)
	return res
}

const kcharmcmpSeeds = 256

func TestCharmcmpLockstep(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbls := kcharmcmpSetup(t)
	c := &kcharmcmpTally{}
	games := 0
	for mi := 0; mi < 2; mi++ {
		for seed := uint64(1); seed <= kcharmcmpSeeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			res := kcharmcmpRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, c.check)
			games++
			if res.Stall != "" && res.Stall != "turns" { // (the random policy rarely attacks: the 14-turn cap ends the game)
				t.Logf("last actions %v", actionStrings(res.Actions[max(0, len(res.Actions)-40):]))
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: Molten Collapse casts cap 2: %d, cap 1: %d, both modes chosen: %d; Wail casts cap 3: %d, cap 1: %d, two or more modes chosen: %d",
		games, c.molten[2], c.molten[1], c.moltenBoth, c.wail[3], c.wail[1], c.wailMulti)
	for _, n := range []struct {
		name      string
		got, want int
	}{
		{"Molten Collapse casts with the cap at 2", c.molten[2], 20}, {"Molten Collapse casts with the cap at 1", c.molten[1], 20},
		{"Molten Collapse casts that chose both modes", c.moltenBoth, 10},
		{"Wail casts with the cap at 3", c.wail[3], 5}, {"Wail casts with the cap at 1", c.wail[1], 20},
	} {
		if n.got < n.want {
			t.Errorf("%d %s, want at least %d", n.got, n.name, n.want)
		}
	}
}

// TestCharmcmpMutation: a kernel whose count is always 1 must diverge.
func TestCharmcmpMutation(t *testing.T) {
	if isolate(t) {
		return
	}

	reg, set, decks, tbls := kcharmcmpSetup(t)
	dirac.DebugCharmCmpCap = true
	defer func() { dirac.DebugCharmCmpCap = false }()
	for mi := 0; mi < 2; mi++ {
		for seed := uint64(1); seed <= 128; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			res := kcharmcmpRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, nil)
			if res.Div != nil {
				t.Logf("mutation: matchup %d seed %d diverged at decision %d: %s %s", mi, seed, res.Div.At, res.Div.What, res.Div.Field)
				return
			}
		}
	}
	t.Errorf("mutation DebugCharmCmpCap never diverged in 128 seeds")
}
