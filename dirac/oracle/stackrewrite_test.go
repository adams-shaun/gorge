package oracle

import (
	"math/rand/v2"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Operations on stack entries that already exist:
// ChangeTargets (Bolt Bend, Redirect Lightning and Untimely Malfunction's
// mode target a spell or ability with a single target; Speedball's trigger
// redirects the spell that fired it) and the mass Counter over Defined$
// ValidStack (Glen Elendra's Answer, with the Faerie tokens it counts).
//
// M1 is the redirect matchup: UR redirection against burn, pumps and the
// stack-targeting Essence Scatter / Negate (a redirected counter names another
// spell). M2 is Glen Elendra's Answer against a deck that fills the stack with
// spells and abilities (Mind Stone, Geralf, Ichneumon Druid). The policy casts
// and activates whenever it can, attacks, and answers every redirect at random.
//
// The counts come from gorge's log and live state: a stack object whose
// targets change between two decisions is a redirect that changed a target; a
// run of adjacent "countered" moves is one Counter effect (>= 2 and it is
// Glen Elendra's sweep).
//
// A redirect of a spell with two targets is one ask per declaration, or a multi
// pick over a declaration of several; no pool spell here reaches it, so the "two"
// count is logged, not required.

var kstackrewriteDecks = [][2][]cardset.Entry{
	{ // M1
		{
			{Name: "Bolt Bend", Count: 4}, {Name: "Redirect Lightning", Count: 4}, {Name: "Untimely Malfunction", Count: 3},
			{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Mountain", Count: 9}, {Name: "Island", Count: 8},
		},
		{
			{Name: "Speedball, New Warrior", Count: 4}, {Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3},
			{Name: "Giant Growth", Count: 3}, {Name: "Essence Scatter", Count: 3}, {Name: "Negate", Count: 3},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 2},
			{Name: "Mountain", Count: 6}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 4},
		},
	},
	{ // M2
		{
			{Name: "Glen Elendra's Answer", Count: 6}, {Name: "Essence Scatter", Count: 2}, {Name: "Opt", Count: 2},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Island", Count: 16}, {Name: "Forest", Count: 11},
		},
		{
			{Name: "Geralf, the Fleshwright", Count: 3}, {Name: "Ichneumon Druid", Count: 3}, {Name: "Mind Stone", Count: 4},
			{Name: "Walking Corpse", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Giant Growth", Count: 3},
			{Name: "Llanowar Elves", Count: 3}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 8},
		},
	},
}

// kstackrewriteStats is what gorge's event log and live state say each
// operation did.
type kstackrewriteStats struct {
	changed   int // a stack object's target set changed between decisions (a redirect that moved a target)
	two       int // ... of a spell or ability with two targets
	speedball int // Speedball's trigger went on the stack
	glenCast  int // Glen Elendra's Answer went on the stack
	glenTwo   int // Glen Elendra's Answer chosen with two or more opposing objects on the stack
	glen      int // a Counter effect that countered two or more objects
	glenAbil  int // ... of which one of them was an ability
	counter1  int // a Counter effect that countered one object
}

type kstackrewriteTally struct {
	evAt int
	snap map[state.ObjID][]state.Target
	run  int
	abil bool
}

func (p *kstackrewriteTally) flush(st *kstackrewriteStats) {
	switch {
	case p.run >= 2:
		st.glen++
		if p.abil {
			st.glenAbil++
		}
	case p.run == 1:
		st.counter1++
	}
	p.run, p.abil = 0, false
}

func (p *kstackrewriteTally) scan(ref *GorgeSide, st *kstackrewriteStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch {
		case e.Kind == events.MoveZone && e.Text == "countered":
			p.run++
			if o := g.Obj(e.Obj); o != nil && o.Ability != nil {
				p.abil = true
			}
		case e.Kind == events.Note, e.Kind == events.Choose: // RememberCounteredSA$ remembers between the moves
		default:
			p.flush(st)
			if e.Kind == events.TriggerPush && chName(g, e.Obj) == "Speedball, New Warrior" {
				st.speedball++
			}
			if e.Kind == events.PutOnStack && chName(g, e.Obj) == "Glen Elendra's Answer" {
				st.glenCast++
			}
		}
	}
	p.evAt = len(evs)
	if p.snap == nil {
		p.snap = map[state.ObjID][]state.Target{}
	}
	live := map[state.ObjID]bool{}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil {
			continue
		}
		live[id] = true
		old, had := p.snap[id]
		now := append([]state.Target(nil), o.Targets...)
		if had && len(old) > 0 && len(now) > 0 && !sameTargets(old, now) {
			st.changed++
			if len(now) == 2 {
				st.two++
			}
		}
		p.snap[id] = now
	}
	for id := range p.snap {
		if !live[id] {
			delete(p.snap, id)
		}
	}
}

func sameTargets(a, b []state.Target) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// kstackrewritePolicy plays at random among casting, activating and playing
// lands, attacks whenever it can, and answers every redirect at random.
func kstackrewritePolicy(seed uint64, st *kstackrewriteStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x7374636b72657772)) // "stckrewr"
	var buf, rest []Canon
	var tl kstackrewriteTally
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
			if len(buf) > 0 {
				return buf[prng.IntN(len(buf))]
			}
		case DPriority:
			buf, rest = buf[:0], rest[:0]
			theirs := 0 // stack objects the deciding player does not control
			if c.G != nil {
				for _, id := range c.G.E.G.Stack {
					if o := c.G.E.G.Obj(id); o != nil && uint8(o.Controller) != c.D.Player {
						theirs++
					}
				}
			}
			for _, a := range c.D.Options {
				// Glen Elendra's Answer is held until the opponent has two
				// things on the stack, then cast at once
				if a.Kind() == ActCast && gorgeOptionName(c, a) == "Glen Elendra's Answer" {
					if theirs < 2 {
						continue
					}
					st.glenTwo++
					return a
				}
				rest = append(rest, a)
				switch a.Kind() {
				case ActCast, ActActivate, ActPlayLand:
					buf = append(buf, a)
				}
			}
			d := *c.D
			d.Options = rest
			if len(buf) > 0 {
				d.Options = buf
			}
			if len(d.Options) > 0 {
				return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
			}
		}
		return rnd.Pick(c)
	})
}

// runStackrewrite plays the matchups, each in both seatings, and returns
// the games, divergences, stalls and the counts. mutate keeps every redirect's
// old target (the mutation must diverge).
func runStackrewrite(t *testing.T, seeds int, mutate bool) (games int, divs, stalls [2]int, st [2]kstackrewriteStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stackrewrite")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugChangeTargetsOff
	dirac.DebugChangeTargetsOff = mutate
	defer func() { dirac.DebugChangeTargetsOff = old }()
	for mi, md0 := range kstackrewriteDecks {
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
				m, err := NewMatchup(reg, [2]string{"kstackrewrite-a", "kstackrewrite-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, kstackrewritePolicy(seed, &st[mi]), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls[mi]++
					if !mutate {
						t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
					}
				}
				if d := res.Div; d != nil {
					divs[mi]++
					if !mutate {
						if os.Getenv("KSR_DEBUG") != "" {
							lo := max(0, d.At-14)
							t.Logf("last actions %v", actionStrings(res.Actions[lo:d.At]))
							evs := g.E.L.Events
							for i := max(0, len(evs)-60); i < len(evs); i++ {
								t.Logf("  ev %d kind=%v player=%d obj=%s %v->%v amt=%d text=%q", i, evs[i].Kind, evs[i].Player, chName(g.E.G, evs[i].Obj), evs[i].From, evs[i].To, evs[i].Amount, evs[i].Text)
							}
						}
						t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
				}
			}
		}
	}
	return games, divs, stalls, st
}

func TestStackrewriteLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 48 seeds per seating per matchup: 192 games
	games, divs, stalls, st := runStackrewrite(t, 48, false)
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
		{"redirects that changed a target", st[0].changed},
		{"Speedball triggers", st[0].speedball},
		{"Glen Elendra casts that countered two or more", st[1].glen}, {"... with an ability among them", st[1].glenAbil},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestStackrewriteMutation keeps every redirect's old target: M1 must diverge.
func TestStackrewriteMutation(t *testing.T) {
	games, divs, _, _ := runStackrewrite(t, 16, true)
	if divs[0] == 0 {
		t.Errorf("the redirect mutation kept every old target with no divergence")
	}
	t.Logf("mutation diverged %v in %d games", divs, games)
}

// Bolt Bend's target is an ability on the stack named by its identity: picked on
// the multi-target path it keeps that identity, so the stack still shows the target.
func TestChangeTargetsKeepsStackAbilityTarget(t *testing.T) {
	pinnedGames(t, "std-fdn", Config{GorgeAutopaySurface: true}, cardOut("Bolt Bend"))
}

// A required redirect leaves its current target out of the offer and asks nothing
// when no other is legal; Speedball's optional one walks a spell's declarations.
func TestRedirectMovesOrKeeps(t *testing.T) {
	pinnedGames(t, "std-msh", Config{}, cardOut("Speedball, New Warrior"))
	pinnedGames(t, "std-fdn", Config{GorgeAutopaySurface: true}, cardOut("Bolt Bend"))
	pinnedGames(t, "std-tla", Config{GorgeAutopaySurface: true}, cardOut("Redirect Lightning"))
}
