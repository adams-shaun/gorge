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

// The cast-provenance lockstep: the readers of how a spell
// was cast. Counters from gorge's event log:
//
//  1. Soulhunter Rakshasa's enters trigger (cast from its owner's hand),
//  2. a Rakshasa entering uncast (Zombify, Unburial Rites: no trigger),
//  3. Knight-Errant of Eos cast with at least one convoking creature,
//  4. Knight-Errant's enters trigger,
//  5. a warp cast, 6. Full Bore cast while its caster controls a warped
//     creature,
//  7. Memory Deluge resolving, 8. cast with flashback.
var cpDecks = map[string][]cardset.Entry{
	"bw": {
		{Name: "Soulhunter Rakshasa", Count: 4}, {Name: "Zombify", Count: 3}, {Name: "Unburial Rites", Count: 2},
		{Name: "Village Rites", Count: 3}, {Name: "Knight-Errant of Eos", Count: 4}, {Name: "Savannah Lions", Count: 6},
		{Name: "Walking Corpse", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Plains", Count: 8},
	},
	"ur": {
		{Name: "Full Bore", Count: 6}, {Name: "Weftstalker Ardent", Count: 4}, {Name: "Red Tiger Mechan", Count: 4},
		{Name: "Nova Hellkite", Count: 2}, {Name: "Memory Deluge", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 10}, {Name: "Island", Count: 7},
	},
}

type cpTally struct {
	evAt                               int
	rakTrig, rakUncast, knConv, knTrig int
	warp, boreWarp, deluge, delugeFB   int
	knOn                               bool
	warped                             map[state.ObjID]bool
}

func (p *cpTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.CastInfo:
			if strings.Contains(e.Counter, "warp") && !p.warped[e.Obj] {
				p.warp++ // a cast emits several CastInfo events
				p.warped[e.Obj] = true
			}
		case events.PutOnStack:
			switch chName(g, e.Obj) {
			case "Knight-Errant of Eos":
				p.knOn = true
			case "Memory Deluge":
				if e.From == state.ZGraveyard {
					p.delugeFB++
				}
			case "Full Bore":
				for id := range p.warped {
					if o := g.Obj(id); o != nil && o.Zone == state.ZBattlefield && o.Controller == e.Player {
						p.boreWarp++
						break
					}
				}
			}
		case events.Tap:
			if o := g.Obj(e.Obj); p.knOn && o != nil && o.Face() != nil && o.Face().IsCreature() {
				p.knConv++
				p.knOn = false
			}
		case events.Priority:
			p.knOn = false
		case events.TriggerPush:
			switch chName(g, e.Obj) {
			case "Soulhunter Rakshasa":
				p.rakTrig++
			case "Knight-Errant of Eos":
				p.knTrig++
			}
		case events.MoveZone:
			n := chName(g, e.Obj)
			if e.From == state.ZBattlefield {
				delete(p.warped, e.Obj)
			}
			if n == "Soulhunter Rakshasa" && e.From == state.ZGraveyard && e.To == state.ZBattlefield {
				p.rakUncast++
			}
			if n == "Memory Deluge" && e.From == state.ZStack {
				p.deluge++
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

func TestCastprovLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("castprov")
	if err != nil {
		t.Fatal(err)
	}
	p := cpTally{warped: map[state.ObjID]bool{}}
	games := 0
	a, b := cpDecks["bw"], cpDecks["ur"]
	matchups := [][2][]cardset.Entry{{a, b}, {b, a}}
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
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"cp-a", "cp-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := fbPolicy(seed) // plays at random among its plays, flashback first
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			p.evAt, p.knOn = 0, false
			clear(p.warped)
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
				if os.Getenv("KCASTPROV_DEBUG") != "" {
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
	t.Logf("%d games: Rakshasa triggers %d, uncast entries %d; Knight convoked %d, triggers %d; warp casts %d, Full Bore beside a warped creature %d; Deluge %d (flashback %d)",
		games, p.rakTrig, p.rakUncast, p.knConv, p.knTrig, p.warp, p.boreWarp, p.deluge, p.delugeFB)
	for _, c := range []struct {
		name string
		n    int
	}{{"Rakshasa triggers", p.rakTrig}, {"uncast Rakshasa entries", p.rakUncast}, {"convoked Knights", p.knConv}, {"Knight triggers", p.knTrig},
		{"warp casts", p.warp}, {"Full Bores beside a warped creature", p.boreWarp}, {"Deluges", p.deluge}, {"flashback Deluges", p.delugeFB}} {
		if c.n < 5 {
			t.Errorf("%d %s, want at least 5", c.n, c.name)
		}
	}
}
