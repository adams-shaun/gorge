package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// the four Count$ heads over cast, sacrifice and
// crew ledgers. Agency Coroner (Sacrificed$Valid Card.IsSuspected, the LKI
// designation), Magebane Lizard (ThisTurnCast Card.nonCreature+ControlledBy
// TriggeredActivator), April O'Neil, Hacktivist (ThisTurnCast
// Card.YouCtrl$CardTypes) and Luxurious Locomotive (Count$CrewSize) meet
// their support decks in a constructed lockstep; the mutation zeroes every
// kernel read and must diverge.

var kcountheads10Decks = [][2][]cardset.Entry{
	{ // M1 (B/U) vs M1' (R/G)
		{
			{Name: "Agency Coroner", Count: 4}, {Name: "Clandestine Meddler", Count: 4},
			{Name: "April O'Neil, Hacktivist", Count: 4}, {Name: "Opt", Count: 4},
			{Name: "Divination", Count: 3}, {Name: "Brineborn Cutthroat", Count: 3},
			{Name: "Wind Drake", Count: 2}, {Name: "Island", Count: 8}, {Name: "Swamp", Count: 8},
		},
		{
			{Name: "Magebane Lizard", Count: 4}, {Name: "Luxurious Locomotive", Count: 3},
			{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Giant Growth", Count: 3}, {Name: "Grizzly Bears", Count: 2},
			{Name: "Hill Giant", Count: 2}, {Name: "Llanowar Elves", Count: 1},
			{Name: "Raging Goblin", Count: 1}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
		},
	},
}

// kcountheads10Stats is what the gorge-side event log says each head did.
type kcountheads10Stats struct {
	coronerSuspect int // Agency Coroner resolved and drew 2 (the sacrificed creature was suspected)
	coronerOther   int // ... resolved and drew 1 (not suspected)
	magebane2      int // a Magebane Lizard trigger dealt X >= 2
	april2         int // April's end-step trigger drew 2 or more (2+ card types)
	loco2          int // Luxurious Locomotive attacked with 2+ crewers this turn
}

// kcountheads10Tally reads the gorge event log between decisions. cur is the
// resolving ability's source (cleared at the next priority/step), draws is
// what its resolution drew.
type kcountheads10Tally struct {
	evAt  int
	cur   string
	draws int
}

func (p *kcountheads10Tally) flush(st *kcountheads10Stats) {
	switch p.cur {
	case "Agency Coroner":
		switch p.draws {
		case 2:
			st.coronerSuspect++
		case 1:
			st.coronerOther++
		}
	case "April O'Neil, Hacktivist":
		if p.draws >= 2 {
			st.april2++
		}
	}
	p.cur, p.draws = "", 0
}

func (p *kcountheads10Tally) scan(ref *GorgeSide, st *kcountheads10Stats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Priority, events.StepChange, events.TurnChange:
			p.flush(st)
		case events.Resolve:
			p.flush(st)
			p.cur = resolveSourceName(g, e.Obj)
		case events.Draw:
			if p.cur != "" {
				p.draws++
			}
		case events.DamageProvenance:
			// Magebane Lizard's trigger: the damage event carries the source
			// permanent and the dealt amount (gorge's X).
			if e.Amount >= 2 && chName(g, e.Obj) == "Magebane Lizard" {
				st.magebane2++
			}
		case events.DeclareAttackers:
			ids := e.IDs
			if len(ids) == 0 {
				ids = []state.ObjID{e.Obj}
			}
			for _, id := range ids {
				if chName(g, id) != "Luxurious Locomotive" {
					continue
				}
				n := 0
				for seat := state.PlayerID(0); seat < 2; seat++ {
					for _, bid := range g.Zone(state.ZBattlefield, seat) {
						o := g.Obj(bid)
						if o == nil || o.CrewedTurn != g.Turn {
							continue
						}
						for _, v := range o.CrewedVehicles {
							if v == id {
								n++
								break
							}
						}
					}
				}
				if n >= 2 {
					st.loco2++
				}
			}
		}
	}
	p.evAt = len(evs)
}

// kcountheads10Policy plays at random among casts, activations and lands
// (casting whenever possible and often crewing), attacks at random, and
// overpays a Crew/Saddle tap-cost pick so the Locomotive can record two or
// more crewers in one activation. It counts each head from the gorge log.
func kcountheads10Policy(seed uint64, st *kcountheads10Stats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6368_3130_0a)) // "ch10\n"
	tapcost, _ := chooseCode("tapcost")
	var buf []Canon
	var tl kcountheads10Tally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		switch c.D.Kind {
		case DAttackers:
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 && prng.IntN(4) != 0 {
				return buf[prng.IntN(len(buf))]
			}
		case DPriority:
			buf = buf[:0]
			var casts, acts, lands []Canon
			for _, a := range c.D.Options {
				switch a.Kind() {
				case ActCast:
					casts = append(casts, a)
				case ActActivate:
					acts = append(acts, a)
				case ActPlayLand:
					lands = append(lands, a)
				}
			}
			if len(acts) > 0 && prng.IntN(2) == 0 {
				return acts[prng.IntN(len(acts))]
			}
			if len(casts) > 0 {
				return casts[prng.IntN(len(casts))]
			}
			if len(acts) > 0 {
				return acts[prng.IntN(len(acts))]
			}
			if len(lands) > 0 {
				return lands[prng.IntN(len(lands))]
			}
		case DChoose:
			for _, a := range c.D.Options {
				if a.Kind() == ActChoose+CanonKind(tapcost) {
					// A tap-cost pick: choose among candidates and Done.
					// Until the floor is reached only candidates are
					// offered; after it the uniform pick sometimes keeps
					// tapping, giving the Locomotive a second crewer in
					// one activation.
					return rnd.Pick(c)
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runCountledgers plays the constructed matchup in both seatings.
func runCountledgers(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kcountheads10Stats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("countledgers")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCountHeads10Off
	dirac.DebugCountHeads10Off = mutate
	defer func() { dirac.DebugCountHeads10Off = old }()
	for mi, md0 := range kcountheads10Decks {
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
				m, err := NewMatchup(reg, [2]string{"kcountheads10-a", "kcountheads10-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, kcountheads10Policy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
					}
				}
				if d := res.Div; d != nil {
					divs++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestCountHeadsCastSacrificeCrewLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runCountledgers(t, 192, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Agency Coroner sacrificed a suspected creature", st.coronerSuspect},
		{"Agency Coroner sacrificed a non-suspected creature", st.coronerOther},
		{"Magebane Lizard X >= 2", st.magebane2},
		{"April O'Neil drew for 2+ card types", st.april2},
		{"Luxurious Locomotive attacked with 2+ crewers", st.loco2},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestCountHeadsCastSacrificeCrewMutation zeroes every kernel read of the four heads: the
// games must diverge.
func TestCountHeadsCastSacrificeCrewMutation(t *testing.T) {
	games, divs, _, _ := runCountledgers(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation zeroed the four heads in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
