package oracle

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The charm-shapes lockstep. Counters from gorge's event log:
//
//  1. Hullbreaker Horror's trigger, 2. a spell returned from the stack to
//     its owner's hand, 3. a permanent bounced by it,
//  4. Gix's Command resolving, 5. a +1/+1 counter it put, 6. a creature
//     it returned from a graveyard, 7. a creature dying during it (the
//     sweeper or the greatest-power sacrifice),
//  8. Phantom Interference cast, 9. its Spirit token, 10. a spell it
//     countered (Spree: each chosen mode adds its cost).
var chDecks = [2][]cardset.Entry{
	{
		{Name: "Hullbreaker Horror", Count: 6}, {Name: "Gix's Command", Count: 5}, {Name: "Opt", Count: 7},
		{Name: "Phantom Interference", Count: 4}, {Name: "Hill Giant", Count: 2}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Shock", Count: 8}, {Name: "Hill Giant", Count: 2}, {Name: "Gray Ogre", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Craw Wurm", Count: 3}, {Name: "Giant Spider", Count: 3}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}

type chTally struct {
	evAt                        int
	hbTrig, hbSpell, hbPerm     int
	gix, gixCtr, gixRet, gixDie int
	ph, phTok, phCtr, phCtrMode int
	phOn                        bool
	hbOn, gixOn                 bool
}

func (p *chTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TriggerPush:
			if chName(g, e.Obj) == "Hullbreaker Horror" {
				p.hbTrig++
			}
		case events.Resolve:
			n := chName(g, e.Obj)
			p.hbOn = n == "" // a trigger object (the pool's only trigger is Hullbreaker's)
			p.gixOn = n == "Gix's Command"
			p.phOn = n == "Phantom Interference"
			if p.gixOn {
				p.gix++
			}
		case events.PutOnStack:
			if chName(g, e.Obj) == "Phantom Interference" {
				p.ph++
			}
		case events.ModeChosen:
			if chName(g, e.Obj) == "Phantom Interference" && strings.Contains(e.Text, "Counter target spell") {
				p.phCtrMode++
			}
		case events.TokenCreate:
			if p.phOn {
				p.phTok++
			}
		case events.Priority:
			p.hbOn, p.gixOn, p.phOn = false, false, false
		case events.CounterChange:
			if p.gixOn && e.Amount > 0 {
				p.gixCtr++
			}
		case events.MoveZone:
			switch {
			case p.hbOn && e.From == state.ZStack && e.To == state.ZHand:
				p.hbSpell++
			case p.hbOn && e.From == state.ZBattlefield && e.To == state.ZHand:
				p.hbPerm++
			case p.gixOn && e.From == state.ZGraveyard && e.To == state.ZHand:
				p.gixRet++
			case p.gixOn && e.From == state.ZBattlefield && e.To == state.ZGraveyard:
				p.gixDie++
			case p.phOn && e.From == state.ZStack && e.To == state.ZGraveyard && chName(g, e.Obj) != "Phantom Interference":
				p.phCtr++
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// seedHold holds Phantom Interference on two decisions of three (the
// rest it may cast for its token mode alone).
func seedHold(seq int) bool { return seq%3 != 0 }

// chPolicy is fbPolicy holding Opt for a response: with the stack empty
// the Hullbreaker player does not cast it, so it is cast with an
// opponent's spell on the stack, the return-a-spell mode's target.
func chPolicy(seed uint64) Policy {
	base := fbPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DPriority || c.G == nil || len(c.G.E.G.Stack) != 0 {
			return base.Pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if n := chName(c.G.E.G, state.ObjID(a.Src())+1); a.Kind() == ActCast && (n == "Opt" || n == "Phantom Interference" && seedHold(c.Seq)) {
				continue
			}
			buf = append(buf, a)
		}
		d := *c.D
		d.Options = buf
		return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestCharmLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("charm")
	if err != nil {
		t.Fatal(err)
	}
	var p chTally
	games := 0
	matchups := [][2][]cardset.Entry{chDecks, {chDecks[1], chDecks[0]}}
	for mi, md := range matchups {
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
		for seed := uint64(1); seed <= 100; seed++ {
			if v := os.Getenv("KCHARM_SEED"); v != "" && v != fmt.Sprint(mi, "/", seed) {
				continue
			}
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ch-a", "ch-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := chPolicy(seed) // plays at random among its plays, Opt held for a response
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			p.evAt, p.hbOn, p.gixOn, p.phOn = 0, false, false, false
			lead, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			res := LockstepLed(m, seed, lead, k, pol, RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 80, Check: p.check})
			CloseSide(k)
			games++
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				if os.Getenv("KCHARM_DEBUG") != "" {
					evs := lead.E.L.Events
					var sb strings.Builder
					for j := max(len(evs)-30, 0); j < len(evs); j++ {
						e := &evs[j]
						fmt.Fprintf(&sb, "%d %s p%d %d(%s) %d->%d amt=%d c=%q t=%q ids=%v\n", j, e.Kind, e.Player, e.Obj, chName(lead.E.G, e.Obj), e.From, e.To, e.Amount, e.Counter, e.Text, e.IDs)
					}
					t.Logf("gorge log tail:\n%s", sb.String())
				}
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: Hullbreaker triggers %d (spells returned %d, permanents bounced %d); Gix's Command %d (counters %d, returns %d, deaths %d); Phantom Interference %d (tokens %d, counter mode chosen %d, countered %d)",
		games, p.hbTrig, p.hbSpell, p.hbPerm, p.gix, p.gixCtr, p.gixRet, p.gixDie, p.ph, p.phTok, p.phCtrMode, p.phCtr)
	for _, c := range []struct {
		name string
		n    int
	}{{"Hullbreaker triggers", p.hbTrig}, {"spells returned", p.hbSpell}, {"permanents bounced", p.hbPerm},
		{"Gix's Commands", p.gix}, {"Gix counters", p.gixCtr}, {"Gix returns", p.gixRet}, {"Gix deaths", p.gixDie},
		{"Phantom Interferences", p.ph}, {"Phantom tokens", p.phTok}, {"Phantom counter modes", p.phCtrMode}} {
		if c.n < 5 {
			t.Errorf("%d %s, want at least 5", c.n, c.name)
		}
	}
}
