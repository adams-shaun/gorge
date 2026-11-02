package oracle

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed matchups over the generated graveaggs pool -- the
// Count$ValidGraveyard mana-value folds (ExGraveAgg): Wick's Patrol's
// greatest mana value, Inferno Project's sum, and the distinct-mana-value
// count of Sudden Insight, Tainted Indulgence, Eris and the five-values
// statics -- beside self-mill, cheap instants and vanilla creatures, so the
// expected answer at every decision is gorge's. The frozen FDN and BLB pair
// pools never cast Wick's Patrol.

var graveaggsDecks = [][2][]cardset.Entry{{
	{
		{Name: "Snooping Newsie", Count: 4}, {Name: "Tainted Indulgence", Count: 4}, {Name: "Aven Heartstabber", Count: 4},
		{Name: "Sudden Insight", Count: 2}, {Name: "Syndicate Infiltrator", Count: 2}, {Name: "Divination", Count: 2},
		{Name: "Mind Rot", Count: 2}, {Name: "Unsummon", Count: 2}, {Name: "Cancel", Count: 2}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Wick's Patrol", Count: 4}, {Name: "Sanguine Spy", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 2}, {Name: "Gray Ogre", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
}, {
	{
		{Name: "Inferno Project", Count: 4}, {Name: "Eris, Roar of the Storm", Count: 2}, {Name: "Shock", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Divination", Count: 4}, {Name: "Unsummon", Count: 2}, {Name: "Gray Ogre", Count: 2},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Wick's Patrol", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Giant Spider", Count: 4}, {Name: "Mind Rot", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Forest", Count: 9},
	},
}}

// graveaggsCarriers names the cards whose casts and activations the test counts,
// with the minimum plays each must reach (about half of the 96 games'
// counts), and two state probes: five_values counts decisions with a
// graveyard of five or more distinct mana values, inferno_sum Inferno
// Project casts over a graveyard instant or sorcery.
var graveaggsCarriers = []struct {
	name string
	min  int
}{
	{"Wick's Patrol", 30}, {"Inferno Project", 20}, {"Eris, Roar of the Storm", 15}, {"Sudden Insight", 15},
	{"Tainted Indulgence", 40}, {"Snooping Newsie", 40}, {"Aven Heartstabber", 40}, {"Sanguine Spy", 80}, {"Syndicate Infiltrator", 10},
	{"five_values", 3000}, {"inferno_sum", 20},
}

// graveaggsPolicy casts or activates whenever it can (so the games reach the
// carriers), else random; it counts the carrier plays the gorge side poses.
func graveaggsPolicy(seed uint64, played map[string]int) Policy {
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
		if c.G != nil && graveaggsFiveValues(c.G.E.G) {
			played["five_values"]++ // a decision with a graveyard holding five mana values
		}
		if k := a.Kind(); (k == ActCast || k == ActActivate) && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					played[ob.Face().Name]++
					if ob.Face().Name == "Inferno Project" && graveaggsGraveSpells(c.G.E.G, ob.Owner) {
						played["inferno_sum"]++ // cast over an instant or sorcery in the graveyard
					}
				}
			}
		}
		return a
	})
}

// graveaggsFiveValues reports whether either graveyard holds cards of five or
// more distinct mana values (the GE5 statics' and Tainted Indulgence's true
// arm).
func graveaggsFiveValues(g *state.Game) bool {
	for p := 0; p < 2; p++ {
		var seen uint64
		for _, id := range g.Zone(state.ZGraveyard, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && o.Face() != nil {
				seen |= 1 << (uint(state.ManaValue(g, o)) & 63)
			}
		}
		if bits.OnesCount64(seen) >= 5 {
			return true
		}
	}
	return false
}

// graveaggsGraveSpells reports whether player p's graveyard holds an instant or
// sorcery card (Inferno Project's sum is then above zero, or a land-free 0).
func graveaggsGraveSpells(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZGraveyard, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil {
			for _, t := range o.Face().Types {
				if t == "Instant" || t == "Sorcery" {
					return true
				}
			}
		}
	}
	return false
}

func TestGraveyardManaValueFoldsLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("graveaggs")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	for mi, md := range graveaggsDecks {
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
			m, err := NewMatchup(reg, [2]string{"b14-a", "b14-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := graveaggsPolicy(seed, played)
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
	t.Logf("%d games, carrier plays %v", games, played)
	for _, c := range graveaggsCarriers {
		if played[c.name] < c.min {
			t.Errorf("carrier %s played %d times, want at least %d", c.name, played[c.name], c.min)
		}
	}
}
