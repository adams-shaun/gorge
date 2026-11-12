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

// Six filter words as PredWord7 sub-kinds: attacking
// Opponent (Oviya, Automech Artisan), EnchantedBy Aura.YouCtrl (Archon of the
// Wild Rose, beside Holy Strength), IsNotChosenType (Kindred Judgment),
// doesNotShareNameWith OtherYourBattlefield (Yenna, Redtooth Regent, beside a
// twinned and a unique enchantment) and sharesNameWith Valid
// Permanent.Legendary (Key to the Side-Door, beside Isamaru). faceUp lowers
// with them but no pool card is lifted by it alone.

var kprops8Decks = [][2][]cardset.Entry{
	{ // M1
		{
			{Name: "Oviya, Automech Artisan", Count: 4}, {Name: "Archon of the Wild Rose", Count: 4}, {Name: "Kindred Judgment", Count: 4},
			{Name: "Holy Strength", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Glory Seeker", Count: 3},
			{Name: "Plains", Count: 9}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Raging Goblin", Count: 4},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
		},
	},
	{ // M2
		{
			{Name: "Yenna, Redtooth Regent", Count: 4}, {Name: "Glorious Anthem", Count: 4}, {Name: "Honor of the Pure", Count: 3},
			{Name: "Key to the Side-Door", Count: 4}, {Name: "Isamaru, Hound of Konda", Count: 5}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Plains", Count: 9}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Elvish Mystic", Count: 3},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
		},
	},
}

// kprops8Stats is what the gorge-side event log says each word decided.
type kprops8Stats struct {
	oviya    int // a creature attacked while its controller's Oviya was on the battlefield (it has trample)
	archon   int // an Aura-enchanted attacker joined its controller's Archon
	kjSpare  int // Kindred Judgment destroyed creatures and spared one
	kjWipe   int // ... destroyed creatures and spared none
	yenna    int // Yenna's ability was put on the stack
	yennaTwn int // a priority decision where Yenna's controller held a twinned enchantment
	keyDisc  int // Key to the Side-Door's ability followed a discard of a legendary card
}

type kprops8Tally struct {
	evAt   int
	cur    string
	died   bool
	legDis bool
}

func (p *kprops8Tally) flush(g *state.Game, st *kprops8Stats) {
	if p.cur == "Kindred Judgment" && p.died {
		spared := false
		for pl := state.PlayerID(0); pl < 2; pl++ {
			for _, b := range g.Zone(state.ZBattlefield, pl) {
				if o := g.Obj(b); o != nil && o.Face() != nil && o.Face().IsCreature() {
					spared = true
				}
			}
		}
		if spared {
			st.kjSpare++
		} else {
			st.kjWipe++
		}
	}
	p.cur, p.died = "", false
}

func (p *kprops8Tally) scan(ref *GorgeSide, st *kprops8Stats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.Priority, events.StepChange:
			p.flush(g, st)
		case events.TurnChange:
			p.flush(g, st)
			p.legDis = false
		case events.Resolve:
			p.flush(g, st)
			p.cur = chName(g, e.Obj)
		case events.MoveZone:
			n := chName(g, e.Obj)
			if p.cur == "Kindred Judgment" && e.From == state.ZBattlefield && e.To == state.ZGraveyard {
				p.died = true
			}
			if e.From == state.ZHand && e.To == state.ZGraveyard && (n == "Isamaru, Hound of Konda" || n == "Yenna, Redtooth Regent") {
				p.legDis = true
			}
		case events.AbilityPush:
			switch chName(g, e.Obj) {
			case "Yenna, Redtooth Regent":
				st.yenna++
			case "Key to the Side-Door":
				if p.legDis {
					st.keyDisc++
					p.legDis = false
				}
			}
		case events.DeclareAttackers:
			ids := e.IDs
			if len(ids) == 0 {
				ids = []state.ObjID{e.Obj}
			}
			for _, id := range ids {
				a := g.Obj(id)
				if a == nil {
					continue
				}
				oviya, archon, aura := false, false, false
				for _, o := range battlefieldObjects(g, a.Controller) {
					switch chName(g, o.ID) {
					case "Oviya, Automech Artisan":
						oviya = true
					case "Archon of the Wild Rose":
						archon = true
					}
					if o.AttachedTo == id && chName(g, o.ID) == "Holy Strength" {
						aura = true
					}
				}
				if oviya {
					st.oviya++
				}
				if archon && aura && chName(g, id) != "Archon of the Wild Rose" {
					st.archon++
				}
			}
		}
	}
	p.evAt = len(evs)
}

// kprops8Types are the creature types of the pool's creatures.
var kprops8Types = map[string][]string{
	"Oviya, Automech Artisan": {"Human", "Artificer"}, "Archon of the Wild Rose": {"Archon"}, "Grizzly Bears": {"Bear"},
	"Glory Seeker": {"Human", "Soldier"}, "Hill Giant": {"Giant"}, "Raging Goblin": {"Goblin", "Berserker"},
	"Yenna, Redtooth Regent": {"Elf", "Noble"}, "Isamaru, Hound of Konda": {"Dog"}, "Llanowar Elves": {"Elf", "Druid"},
	"Elvish Mystic": {"Elf", "Druid"},
}

// kprops8Policy plays at random among casting, activating and playing lands
// (so the carriers are reached) and attacks whenever it can.
func kprops8Policy(seed uint64, st *kprops8Stats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x70726f7073380a)) // "props8"
	var buf []Canon
	var tl kprops8Tally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		if c.G != nil && len(c.D.Options) > 0 && c.D.Kind == DChoose {
			// Kindred Judgment's type ask: name a type a battlefield creature has (else
			// the pick is random and nearly always a wipe)
			if o, ok := c.G.GorgeOption(c.D.Options[0]); ok && o.Kind == "type" {
				g := c.G.E.G
				have := map[string]bool{}
				for pl := state.PlayerID(0); pl < 2; pl++ {
					for _, o := range battlefieldObjects(g, pl) {
						for _, ty := range kprops8Types[chName(g, o.ID)] {
							have[ty] = true
						}
					}
				}
				buf = buf[:0]
				for _, a := range c.D.Options {
					if oa, _ := c.G.GorgeOption(a); have[oa.Label] {
						buf = append(buf, a)
					}
				}
				if len(buf) > 0 && prng.IntN(4) != 0 {
					return buf[prng.IntN(len(buf))]
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
			if c.G != nil {
				g := c.G.E.G
				bf := battlefieldObjects(g, state.PlayerID(c.D.Player))
				yenna := false
				names := map[string]int{}
				for _, o := range bf {
					n := chName(g, o.ID)
					if n == "Yenna, Redtooth Regent" && !o.Tapped {
						yenna = true
					}
					if n == "Glorious Anthem" || n == "Honor of the Pure" {
						names[n]++
					}
				}
				if yenna && (names["Glorious Anthem"] > 1 || names["Honor of the Pure"] > 1) {
					st.yennaTwn++
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

// runFilterattacking plays the constructed matchups, each in both seatings, and
// returns the games, divergences, stalls and the word counts. mutate negates
// every kernel PredWord7 answer (the mutation must diverge).
func runFilterattacking(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kprops8Stats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("filterattacking")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugWord7Flip
	dirac.DebugWord7Flip = mutate
	defer func() { dirac.DebugWord7Flip = old }()
	for mi, md0 := range kprops8Decks {
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
				m, err := NewMatchup(reg, [2]string{"kprops8-a", "kprops8-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, kprops8Policy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestFilterWordsAttackingEnchantedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 per matchup
	games, divs, stalls, st := runFilterattacking(t, 96, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Oviya trample grants", st.oviya}, {"Archon pumps an enchanted attacker", st.archon},
		{"Kindred Judgment spared a creature", st.kjSpare}, {"Yenna activations", st.yenna},
		{"Yenna beside a twinned enchantment", st.yennaTwn}, {"Key discards of a legendary card", st.keyDisc},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestFilterWordsAttackingEnchantedMutation negates every kernel PredWord7 answer: the games must
// diverge. A green mutation means the lockstep never reaches a decision the
// words shape.
func TestFilterWordsAttackingEnchantedMutation(t *testing.T) {
	games, divs, _, _ := runFilterattacking(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation negated every PredWord7 answer in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
