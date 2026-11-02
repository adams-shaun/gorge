package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// End-step DelayedTrigger RememberObjects$ TriggeredCard /
// TriggeredCardLKICopy: Esoteric Duplicator's
// Sacrificed trigger pays {2} and copies the sacrificed artifact at the next
// end step (CR 603.7a, 608.2h, 707.2); Resurrection Orb returns its dead
// creature; Biolume Egg and Loyal Cathar return transformed (CR 712.14a).
// Village Rites and Carrion Feeder sacrifice; Mind Stone, Ornithopter and the
// Duplicator's own draw feed the Duplicator.

var klkidelayedOpp = []cardset.Entry{
	{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Llanowar Elves", Count: 2},
	{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Giant Growth", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
}

var klkidelayedDecks = [][2][]cardset.Entry{
	{ // M1: Esoteric Duplicator and artifact sacrifices against creatures and burn
		{
			{Name: "Esoteric Duplicator", Count: 4}, {Name: "Mind Stone", Count: 4}, {Name: "Carrion Feeder", Count: 4}, {Name: "Village Rites", Count: 4},
			{Name: "Ornithopter", Count: 4}, {Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6}, {Name: "Mountain", Count: 4},
		},
		klkidelayedOpp,
	},
	{ // M2: the delayed returns (Biolume Egg, Loyal Cathar, Resurrection Orb) against creatures and burn
		{
			{Name: "Biolume Egg", Count: 4}, {Name: "Loyal Cathar", Count: 4}, {Name: "Resurrection Orb", Count: 3}, {Name: "Carrion Feeder", Count: 4},
			{Name: "Village Rites", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Shock", Count: 2},
			{Name: "Plains", Count: 6}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 7},
		},
		klkidelayedOpp,
	},
}

// klkidelayedStats is what the gorge-side event log says each carrier did.
type klkidelayedStats struct {
	dupTrig    int // Esoteric Duplicator's Sacrificed trigger went on the stack
	dupCopy    int // a token copy was created (only the Duplicator's end-step program copies)
	eggReturn  int // Biolume Serpent entered from the graveyard (the Egg's delayed transformed return)
	cathReturn int // Unhallowed Cathar entered from the graveyard (the Cathar's delayed transformed return)
	orbReturn  int // the Orb's delayed program fired and its creature entered from the graveyard
}

type klkidelayedTally struct {
	evAt   int
	orbDue int // Orb delayed programs fired, not yet answered by a return
}

func (p *klkidelayedTally) scan(ref *GorgeSide, st *klkidelayedStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt, p.orbDue = 0, 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TurnChange:
			p.orbDue = 0
		case events.TriggerPush:
			if chName(g, e.Obj) == "Esoteric Duplicator" {
				st.dupTrig++
			}
		case events.CopyToken:
			st.dupCopy++
		case events.DelayedPush:
			if chName(g, e.Obj) == "Resurrection Orb" {
				p.orbDue++
			}
		case events.MoveZone:
			if e.From != state.ZGraveyard || e.To != state.ZBattlefield {
				continue
			}
			switch chName(g, e.Obj) {
			case "Biolume Serpent":
				st.eggReturn++
			case "Unhallowed Cathar":
				st.cathReturn++
			default:
				if p.orbDue > 0 {
					p.orbDue--
					st.orbReturn++
				}
			}
		}
	}
	p.evAt = len(evs)
}

// klkidelayedSac are the cards whose casts and activations sacrifice (or
// set up a sacrifice); the policy favours them.
var klkidelayedSac = map[string]bool{"Village Rites": true, "Carrion Feeder": true, "Mind Stone": true,
	"Esoteric Duplicator": true, "Resurrection Orb": true}

// klkidelayedPolicy pays the Duplicator's optional {2}, favours the
// sacrifice outlets and the Orb's equip, attacks whenever it can, and plays
// at random among casting, activating and playing lands otherwise.
func klkidelayedPolicy(seed uint64, st *klkidelayedStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6b6c6b690a)) // "klki"
	var buf []Canon
	var tl klkidelayedTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		if c.G != nil && c.D.Kind != DPriority && c.D.Kind != DAttackers {
			for _, a := range c.D.Options {
				if oa, ok := c.G.GorgeOption(a); ok && oa.Kind == "trigger_cost_pay" && prng.IntN(8) != 0 {
					return a
				}
			}
		}
		switch c.D.Kind {
		case DAttackers:
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return buf[prng.IntN(len(buf))]
			}
		case DPriority:
			if c.G != nil && prng.IntN(2) == 0 {
				g := c.G.E.G
				buf = buf[:0]
				for _, a := range c.D.Options {
					if k := a.Kind(); k != ActCast && k != ActActivate {
						continue
					}
					if oa, ok := c.G.GorgeOption(a); ok && oa.Obj != 0 && klkidelayedSac[chName(g, oa.Obj)] {
						buf = append(buf, a)
					}
				}
				if len(buf) > 0 {
					return buf[prng.IntN(len(buf))]
				}
			}
			buf = buf[:0]
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
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

// runLkidelayed plays the constructed matchups, each in both seatings, and
// returns the games, divergences, stalls and the carrier counts.
func runLkidelayed(t *testing.T, seeds int) (games int, divs, stalls [2]int, st [2]klkidelayedStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("lkidelayed")
	if err != nil {
		t.Fatal(err)
	}
	for mi, md0 := range klkidelayedDecks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
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
					t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
				}
			}
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= uint64(seeds); seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"klkidelayed-a", "klkidelayed-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, klkidelayedPolicy(seed, &st[mi]), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls[mi]++
					t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					divs[mi]++
					t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestLkidelayedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 48 seeds per seating: 96 per matchup
	games, divs, stalls, st := runLkidelayed(t, 48)
	t.Logf("%d games, %v divergences, %v stalls", games, divs, stalls)
	t.Logf("M1 %+v", st[0])
	t.Logf("M2 %+v", st[1])
	if divs != [2]int{} || stalls != [2]int{} {
		t.Fatalf("%v divergences, %v stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Esoteric Duplicator Sacrificed triggers", st[0].dupTrig}, {"end-step token copies", st[0].dupCopy},
		{"Biolume Egg transformed returns", st[1].eggReturn}, {"Loyal Cathar transformed returns", st[1].cathReturn},
		{"Resurrection Orb returns", st[1].orbReturn},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}
