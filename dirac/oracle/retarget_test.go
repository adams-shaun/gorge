package oracle

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ChangeTargets over spells and abilities (CR 115.7): Deflecting Swat (any
// spell or ability, optional), Misdirection (a spell with a single target, cast
// for an exile of a blue card from hand), Imp's Mischief (the same, and the
// life lost for the spell's mana value), Reroute (an activated ability with a
// single target, then a draw), Sideswipe (an Arcane spell, optional), and
// Spellskite and Hydroelectric Specimen, which take a target of a spell or
// ability (Hydroelectric's of an instant or sorcery with a single target) for
// themselves.
//
// R1 is a blue-red deck holding all of them but Imp's Mischief against a
// black-red burn deck that holds Imp's Mischief; both sides cast and activate
// whenever they can, so every redirect meets a burn spell, a Rod of Ruin or
// Prodigal Sorcerer activation, or Flametongue Kavu's trigger.

var kretargetDecks = [2][]cardset.Entry{
	{
		{Name: "Deflecting Swat", Count: 3}, {Name: "Misdirection", Count: 3}, {Name: "Reroute", Count: 3},
		{Name: "Sideswipe", Count: 3}, {Name: "Essence Scatter", Count: 2}, {Name: "Opt", Count: 1},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 2}, {Name: "Rod of Ruin", Count: 2},
		{Name: "Prodigal Sorcerer", Count: 1}, {Name: "Spellskite", Count: 2}, {Name: "Hydroelectric Specimen", Count: 2},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Imp's Mischief", Count: 4}, {Name: "Lava Spike", Count: 3}, {Name: "First Volley", Count: 3},
		{Name: "Rend Flesh", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Flametongue Kavu", Count: 3}, {Name: "Rod of Ruin", Count: 2}, {Name: "Hill Giant", Count: 1},
		{Name: "Spellskite", Count: 2},
		{Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 6},
	},
}

// kretargetStats is what gorge's event log says each carrier did.
type kretargetStats struct {
	cast    map[string]int
	changed int // a stack object's targets changed between two decisions
	exiled  int // a card left a hand for exile (Misdirection's alternative cost)
}

// kretargetMagnets counts, per decision, the stack objects whose targets now
// include a Spellskite or Hydroelectric Specimen they did not before: the
// redirect that moved a target onto its own source.
func kretargetMagnets(g *GorgeSide, snap map[state.ObjID][]state.Target, into map[string]int) {
	live := map[state.ObjID]bool{}
	for _, id := range g.E.G.Stack {
		o := g.E.G.Obj(id)
		if o == nil {
			continue
		}
		live[id] = true
		now := append([]state.Target(nil), o.Targets...)
		for _, t := range now {
			if t.IsPlayer || slices.Contains(snap[id], t) || len(snap[id]) == 0 {
				continue
			}
			if tg := g.E.G.Obj(t.Obj); tg != nil && tg.Zone == state.ZBattlefield && tg.Face() != nil {
				switch nm := tg.Face().Name; nm {
				case "Spellskite", "Hydroelectric Specimen":
					into["->"+nm]++
				}
			}
		}
		snap[id] = now
	}
	for id := range snap {
		if !live[id] {
			delete(snap, id)
		}
	}
}

// runRetarget plays the matchup in both seatings; mutate keeps every redirect's
// old target (it must diverge).
func runRetarget(t *testing.T, seeds int, mutate bool) (games, divs int, st kretargetStats) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("retarget")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugChangeTargetsOff
	dirac.DebugChangeTargetsOff = mutate
	defer func() { dirac.DebugChangeTargetsOff = old }()
	st.cast = map[string]int{}
	for swap := 0; swap < 2; swap++ {
		md := kretargetDecks
		if swap == 1 {
			md = [2][]cardset.Entry{kretargetDecks[1], kretargetDecks[0]}
		}
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
				t.Fatalf("deck %d: %d cards", s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kretarget-a", "kretarget-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			var gs *GorgeSide
			var tl kstackrewriteTally
			var sw kstackrewriteStats
			snap := map[state.ObjID][]state.Target{}
			inner := eagerPolicy(seed, map[string]int{})
			pol := PolicyFunc(func(c *PickCtx) Canon {
				if c.G != nil {
					gs = c.G
					tl.scan(c.G, &sw)
					kretargetMagnets(c.G, snap, st.cast)
				}
				return inner.Pick(c)
			})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			st.changed += sw.changed
			if gs != nil {
				for _, ev := range gs.E.L.Events {
					switch {
					case ev.Kind == events.PutOnStack && ev.From == state.ZHand:
						if ob := gs.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
							st.cast[ob.Face().Name]++
						}
					case ev.Kind == events.MoveZone && ev.From == state.ZHand && ev.To == state.ZExile:
						st.exiled++
					}
				}
			}
			if d := res.Div; d != nil {
				divs++
				if mutate {
					continue
				}
				lo := max(0, d.At-12)
				t.Logf("swap %d seed %d last actions %v", swap, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	}
	return
}

func TestRetargetLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, st := runRetarget(t, 150, false)
	var names []string
	for n := range st.cast {
		names = append(names, n)
	}
	sort.Strings(names)
	var cs []string
	for _, n := range names {
		cs = append(cs, fmt.Sprintf("%s=%d", n, st.cast[n]))
	}
	t.Logf("%d games, %d divergences, %d redirects that changed a target, %d hand exiles; casts %s", games, divs, st.changed, st.exiled, strings.Join(cs, " "))
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	for _, n := range []string{"Deflecting Swat", "Misdirection", "Imp's Mischief", "Reroute", "Sideswipe", "Spellskite", "Hydroelectric Specimen"} {
		if st.cast[n] < 4 {
			t.Errorf("%s cast %d times (want >= 4)", n, st.cast[n])
		}
	}
	for _, n := range []string{"->Spellskite", "->Hydroelectric Specimen"} {
		if st.cast[n] < 1 {
			t.Errorf("redirects that moved a target onto %s: %d (want >= 1)", n[2:], st.cast[n])
		}
	}
	if st.changed < 4 {
		t.Errorf("redirects that changed a target: %d (want >= 4)", st.changed)
	}
}

// TestRetargetMutation keeps every redirect's old target: the matchup must diverge.
func TestRetargetMutation(t *testing.T) {
	games, divs, _ := runRetarget(t, 16, true)
	if divs == 0 {
		t.Errorf("the redirect mutation kept every old target with no divergence")
	}
	t.Logf("mutation diverged %d of %d games", divs, games)
}
