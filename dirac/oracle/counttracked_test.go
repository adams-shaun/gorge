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

// six value heads that read an object the kernel already
// tracks -- Mathemagics' Number$2/Pow.X, Golbez's Remembered$CardPower,
// Jadeheart Attendant's ExiledWith$CardManaCost, Spider-Man Noir's
// TriggeredAttacker$CardCounters.ALL, Venom's Sacrificed$CardManaCost and
// Bolg's Spawner>Sacrificed$CardPower. Bolg's capture of the sacrificed
// creature's power was always exact; gorge read the front face from the
// graveyard (Venom 3, not 5) until (CR 608.2h last known
// information), fixed at the gorge pin 6ab69318c.

var kcountheads11Decks = [][2][]cardset.Entry{
	{ // M1: U/B/G artifacts (Mathemagics, Golbez, Jade) vs R/G aggro
		{
			{Name: "Mathemagics", Count: 4}, {Name: "Golbez, Crystal Collector", Count: 4},
			{Name: "Jade Seedstones", Count: 4}, {Name: "Opt", Count: 3},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3},
			{Name: "Darksteel Relic", Count: 3}, {Name: "Ornithopter", Count: 3},
			{Name: "Seat of the Synod", Count: 3}, {Name: "Tree of Tales", Count: 3},
			{Name: "Vault of Whispers", Count: 3}, {Name: "Island", Count: 1},
			{Name: "Forest", Count: 1}, {Name: "Swamp", Count: 1},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Raging Goblin", Count: 4},
			{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Bonesplitter", Count: 4}, {Name: "Leonin Scimitar", Count: 4},
			{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
		},
	},
	{ // M2: B/R/G (Noir, Eddie/Venom, Bolg) vs U/R tempo
		{
			{Name: "Bolg of the North", Count: 4},
			{Name: "Spider-Man Noir", Count: 4}, {Name: "Eddie Brock", Count: 4},
			{Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 4},
			{Name: "Llanowar Elves", Count: 3}, {Name: "Raging Goblin", Count: 3},
			{Name: "Mountain", Count: 4},
			{Name: "Forest", Count: 5}, {Name: "Swamp", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
			{Name: "Llanowar Elves", Count: 4}, {Name: "Wind Drake", Count: 4},
			{Name: "Brineborn Cutthroat", Count: 4}, {Name: "Opt", Count: 4},
			{Name: "Shock", Count: 4}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 6},
		},
	},
}

// kcountheads11Stats is what the gorge-side event log says each head did.
type kcountheads11Stats struct {
	mathemagics2 int // a Mathemagics resolution drew >= 4 (X >= 2)
	golbezTrig   int // a Golbez end-step trigger resolved
	golbezRead   int // ... and the opponent lost life (8+ artifacts: CardPower read)
	jadeCraft    int // Jadeheart Attendant entered via Jade Seedstones' craft
	noirPre      int // a Noir trigger resolved with a pre-existing counter on the attacker
	eddieSac     int // Venom drew >= 1 (sacrificed mana value >= 1)
	bolgSac      int // Bolg's reflexive trigger dealt >= 2 (sacrificed power >= 2)
	divMatch     [2]int
}

// kcountheads11Tally reads the gorge event log between decisions. cur is the
// resolving ability's source (cleared at the next priority/step), draws is
// what its resolution drew.
type kcountheads11Tally struct {
	evAt  int
	cur   string
	draws int
}

func (p *kcountheads11Tally) flush(st *kcountheads11Stats) {
	switch p.cur {
	case "Mathemagics":
		if p.draws >= 4 {
			st.mathemagics2++ // 2^X drawn, X >= 2
		}
	case "Golbez, Crystal Collector":
		st.golbezTrig++
	case "Venom, Lethal Protector":
		if p.draws >= 1 {
			st.eddieSac++
		}
	}
	p.cur, p.draws = "", 0
}

func (p *kcountheads11Tally) scan(ref *GorgeSide, st *kcountheads11Stats) {
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
			if p.cur == "Spider-Man Noir" {
				// the lone attacker's counters BEFORE this trigger's own
				// PutCounter: the surveil X is the live CardCounters.ALL read
				ctrl := state.PlayerID(255)
				if o := g.Obj(e.Obj); o != nil {
					ctrl = o.Controller
				}
				if ctrl < 2 {
					for _, id := range g.Zone(state.ZBattlefield, ctrl) {
						if o := g.Obj(id); o != nil && o.IsAttacking && o.Counter("ALL") >= 1 {
							st.noirPre++
							break
						}
					}
				}
			}
		case events.Draw:
			if p.cur != "" {
				p.draws++
			}
		case events.LifeChange:
			// Golbez's "each opponent loses life equal to that card's
			// power" only runs with 8+ artifacts, so a life loss in its
			// resolution is the Remembered$CardPower read.
			if p.cur == "Golbez, Crystal Collector" && e.Amount < 0 {
				st.golbezRead++
			}
		case events.DamageProvenance:
			// Bolg's reflexive "deals damage equal to that creature's
			// power": the sacrificed power is the dealt amount.
			if p.cur == "Bolg of the North" && e.Amount >= 2 && chName(g, e.Obj) == "Bolg of the North" {
				st.bolgSac++
			}
		case events.MoveZone:
			if e.To == state.ZBattlefield && chName(g, e.Obj) == "Jadeheart Attendant" {
				st.jadeCraft++
			}
		}
	}
	p.evAt = len(evs)
}

// kcountheads11Policy plays at random among casts, activations and lands,
// accepts every optional trigger-cost payment (the sacrifices Eddie Brock and
// Bolg offer) and every yes/no optional, and declares a lone attacker half
// the time so Spider-Man Noir's Alone trigger fires.
func kcountheads11Policy(seed uint64, st *kcountheads11Stats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6368_3131_0a)) // "ch11\n"
	pay, _ := chooseCode("trigger_cost_pay")
	yes, _ := chooseCode("yes")
	var buf []Canon
	var tl kcountheads11Tally
	atkPicked := 0
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
			if atkPicked == 0 && len(buf) > 0 && prng.IntN(2) == 0 {
				atkPicked = 1
				return buf[prng.IntN(len(buf))]
			}
			if atkPicked == 1 {
				for _, a := range c.D.Options {
					if a.Kind() == ActDone {
						atkPicked = 0
						return a
					}
				}
			}
			atkPicked = 0
		case DPriority:
			atkPicked = 0
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
				if a.Kind() == ActChoose+CanonKind(pay) || a.Kind() == ActChoose+CanonKind(yes) {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

// runCounttracked plays both constructed matchups in both seatings.
func runCounttracked(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kcountheads11Stats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("counttracked")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugCountHeads11Off
	dirac.DebugCountHeads11Off = mutate
	defer func() { dirac.DebugCountHeads11Off = old }()
	for mi, md0 := range kcountheads11Decks {
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
				m, err := NewMatchup(reg, [2]string{"kcountheads11-a", "kcountheads11-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, kcountheads11Policy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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
					if mutate {
						st.divMatch[mi]++
					}
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestValueHeadsTrackedObjectLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, st := runCounttracked(t, 192, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Mathemagics resolved with X >= 2", st.mathemagics2},
		{"Golbez end-step trigger resolved", st.golbezTrig},
		{"Golbez read Remembered$CardPower (8+ artifacts)", st.golbezRead},
		{"Jade Seedstones crafted", st.jadeCraft},
		{"Noir trigger with a pre-existing counter", st.noirPre},
		{"Venom sacrificed a creature with mana value >= 1", st.eddieSac},
		{"Bolg's reflexive trigger dealt >= 2", st.bolgSac},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestValueHeadsTrackedObjectMutation zeroes the new kernel reads (ExPow's exponent,
// the exiled-with mana-value sum, the trigger-cost sacrifice accumulator and
// the attacker's live counter read): the games must diverge.
func TestValueHeadsTrackedObjectMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, st := runCounttracked(t, 64, true)
	if divs == 0 {
		t.Fatalf("mutation zeroed the new heads in %d games with no divergence", games)
	}
	for mi, n := range st.divMatch {
		if n == 0 {
			t.Errorf("matchup %d: the mutation did not diverge in any game", mi)
		}
	}
	t.Logf("mutation diverged in %d of %d games (%v)", divs, games, st.divMatch)
}
