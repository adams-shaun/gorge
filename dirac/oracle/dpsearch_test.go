package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The DefinedPlayer$ search lockstep: Path to Exile and Assassin's Trophy make
// the target creature's controller (the caster's opponent, when the caster
// targets the other side's creature) search their library for a basic land,
// and that player may decline the Optional$ search before any pick. The
// expected answer at every decision, including the confirm, is gorge's.

var dpsearchDecks = [][2][]cardset.Entry{{
	{
		{Name: "Path to Exile", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Assassin's Trophy", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	},
}, {
	{
		{Name: "Assassin's Trophy", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Path to Exile", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
	},
}, {
	{
		{Name: "Path to Exile", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Path to Exile", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
	},
}, {
	{
		{Name: "Assassin's Trophy", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Assassin's Trophy", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Forest", Count: 10},
	},
}}

// dpsearchCounts is the confirm-decision census: searches (accepts) and
// declines by the opponent of the caster, plus the carrier casts.
type dpsearchCounts struct {
	confirms, searched, declined, self int
	played                             map[string]int
}

// dpsearchPolicy casts or activates whenever it can (so the removal reaches
// the confirm), else random; it counts the search confirms the gorge side
// poses and the carrier plays.
func dpsearchPolicy(seed uint64, n *dpsearchCounts) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if c.G == nil {
			return a
		}
		if d := c.G.Pending(); d != nil && d.ResumeKind == "search_confirm" {
			n.confirms++
			opp := false
			if o := c.G.E.G.Obj(d.Source); o != nil && o.Controller != d.Player {
				opp = true
			}
			no, _ := chooseCode("no")
			switch {
			case a.Kind() == ActChoose+CanonKind(no):
				if opp {
					n.declined++
				} else {
					n.self++
				}
			default:
				if opp {
					n.searched++
				} else {
					n.self++
				}
			}
		}
		if k := a.Kind(); k == ActCast || k == ActActivate {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					n.played[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

func TestDpsearchLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("dpsearch")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	n := dpsearchCounts{played: map[string]int{}}
	for mi, md := range dpsearchDecks {
		var decks [2][]*cards.Card
		for s, d := range md {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"dpsearch-a", "dpsearch-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := dpsearchPolicy(seed, &n)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
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
	t.Logf("%d games, confirms %d, opponent searches %d, opponent declines %d, self %d, carriers %v",
		games, n.confirms, n.searched, n.declined, n.self, n.played)
	if n.searched < 30 {
		t.Errorf("opponent searches %d, want >= 30", n.searched)
	}
	if n.declined < 5 {
		t.Errorf("opponent declines %d, want >= 5", n.declined)
	}
	for _, name := range []string{"Path to Exile", "Assassin's Trophy"} {
		if n.played[name] == 0 {
			t.Errorf("carrier %s never cast", name)
		}
	}
}
