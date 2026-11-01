package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The flashback-cost lockstep (CR 702.34a): flashback
// costs with a non-mana part (CardDef.FbCost, kgen lower_fbcost.go), paid
// through the cast's sacrifice pick in place of the mana. The policy casts
// every flashback it is offered. Counters from gorge's log and options:
//
//  1. a flashback cast per carrier (CastInfo "flashback"), 2. a sacrifice
//     pick with more candidates than it needs, 3. with exactly as many
//     (still asked), 4. withheld: a carrier in the graveyard whose
//     sacrifice its controller cannot pay, where neither side offers it.
var fbDecks = [2][]cardset.Entry{
	{
		{Name: "Dread Return", Count: 4}, {Name: "Lava Dart", Count: 6}, {Name: "Walking Corpse", Count: 7},
		{Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Savannah Lions", Count: 9}, {Name: "Battle Screech", Count: 5}, {Name: "Lava Dart", Count: 6},
		{Name: "Plains", Count: 12}, {Name: "Mountain", Count: 8},
	},
}

var fbCarrierNames = []string{"Dread Return", "Lava Dart", "Battle Screech"}

type fbTally struct {
	evAt     int
	carriers map[string]int
	sacMany  int
	sacExact int
	wHeld    int
	tapPicks int
	badOffer int
}

// fbNeed is a carrier's flashback sacrifice: how many of p's permanents
// match (ok false for a card without one).
func fbNeed(g *state.Game, p state.PlayerID, o *state.Object) (need, have int, ok bool) {
	if o == nil || o.Face() == nil {
		return 0, 0, false
	}
	var spec string
	switch o.Face().Name {
	case "Dread Return":
		need, spec = 3, "Creature"
	case "Lava Dart":
		need, spec = 1, "Mountain"
	case "Battle Screech":
		need, spec = 3, "White"
	default:
		return 0, 0, false
	}
	for _, id := range g.Zone(state.ZBattlefield, p) {
		b := g.Obj(id)
		if b == nil || b.Controller != p || b.Face() == nil {
			continue
		}
		if spec == "White" && b.Face().IsCreature() && !b.Tapped && fbWhite(b.Face()) ||
			spec == "Creature" && b.Face().IsCreature() || spec == "Mountain" && strings.Contains(b.Face().Name, "Mountain") {
			have++
		}
	}
	return need, have, true
}

func (p *fbTally) check(ref *GorgeSide, k Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		if e := &evs[j]; e.Kind == events.CastInfo {
			for _, f := range strings.Split(e.Counter, ",") {
				if strings.TrimSpace(f) == "flashback" {
					if o := g.Obj(e.Obj); o != nil && o.Face() != nil {
						p.carriers[o.Face().Name]++
					}
				}
			}
		}
	}
	p.evAt = len(evs)
	fr := ref.fr
	if fr == nil || fr.d == nil {
		return nil
	}
	if fr.d.Kind == decision.KChoose {
		n, taps := 0, 0
		for _, o := range fr.d.Options {
			if o.Kind == "sacrifice" {
				n++
			}
			if strings.Contains(o.Kind, "tap") {
				taps++
			}
		}
		if taps > 0 {
			p.tapPicks++
		}
		if n > 0 {
			if n > fr.d.Max {
				p.sacMany++
			} else {
				p.sacExact++
			}
		}
		return nil
	}
	if fr.d.Kind != decision.KPriority {
		return nil
	}
	actor := state.PlayerID(fr.d.Player)
	for _, id := range g.Zone(state.ZGraveyard, actor) {
		o := g.Obj(id)
		need, have, ok := fbNeed(g, actor, o)
		if !ok || have >= need {
			continue
		}
		if o.Face().IsSorcery() && (g.Active != actor || (g.Step != state.StepMain1 && g.Step != state.StepMain2)) {
			continue
		}
		p.wHeld++
		for _, op := range fr.d.Options {
			if op.Kind == "cast" && op.Mode == "flashback" && op.Obj == id {
				p.badOffer++
			}
		}
	}
	return nil
}

// fbPolicy casts every flashback it is offered and otherwise plays at
// random among its plays (a pass only when none).
func fbPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority {
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActCast && a.Mode()&0x0f == 2 {
					return a
				}
				if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				d := *c.D
				d.Options = buf
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

func TestFbcostLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("fbcost")
	if err != nil {
		t.Fatal(err)
	}
	p := fbTally{carriers: map[string]int{}}
	games := 0
	for mi, md := range [][2][]cardset.Entry{{fbDecks[0], fbDecks[1]}, {fbDecks[1], fbDecks[0]}, {fbDecks[0], fbDecks[0]}} {
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
		var tbl *kop.Table
		if tbl, _, _, err = set.Matchup(md[0], md[1]); err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= 60; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"fb-a", "fb-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := fbPolicy(seed)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			p.evAt = 0
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: p.check})
			CloseSide(k)
			games++
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: flashback casts %v; sacrifice picks %d with spare candidates, %d exact; withheld frames %d (offered %d); tap-creature picks %d",
		games, p.carriers, p.sacMany, p.sacExact, p.wHeld, p.badOffer, p.tapPicks)
	for _, name := range fbCarrierNames {
		if p.carriers[name] < 10 {
			t.Errorf("carrier %q flashed back %d times, want at least 10", name, p.carriers[name])
		}
	}
	for _, c := range []struct {
		name string
		n    int
	}{{"sacrifice picks with spare candidates", p.sacMany}, {"exact sacrifice picks", p.sacExact}, {"withheld frames", p.wHeld}, {"tap-creature picks", p.tapPicks}} {
		if c.n < 10 {
			t.Errorf("%d %s, want at least 10", c.n, c.name)
		}
	}
	if p.badOffer != 0 {
		t.Errorf("%d flashback offers in withheld frames, want 0", p.badOffer)
	}
}

// fbWhite reports whether face f is white: a W in its mana cost or a white
// colour indicator (the Bird token's).
func fbWhite(f *cards.Face) bool {
	return strings.Contains(f.ManaCost, "W") || strings.Contains(strings.ToLower(f.Colors), "w")
}
