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

// Trigger-line qualifiers beside their live carriers:
// ActivatorThisTurnCast$ GT1 (Geralf, the Fleshwright; Ichneumon Druid for
// the opponent's instants), AttackedTarget$ You (Orim's Prayer, Lulu, Reveille
// Squad) and TargetsValid$ with a player alternative (Reparations: "You,
// Creature.YouCtrl+inZoneBattlefield"). Jiang Yanggu (ValidAttackersAmount$
// EQ1, a counter on Defined$ TriggeredAttackers) and Sabotage Strategist
// (AttackedTarget$ You, -1/-0 on Defined$ TriggeredAttackers; M3,
// ) are live: the body acts on the event's
// whole attacker group (CR 508.3b). Esoteric Duplicator lowers its trigger
// line but its body stays refused (a delayed LKI copy), Yuriko and Jin Sakai
// by other mechanisms. Kingpin, Wilson Fisk (a
// Self alternative beside ValidPlayer$ You) is held back by gorge's per-stint
// trigger limit; Danitha, Spear of Agony (a zone-free Creature.OppCtrl) now
// lowers as the kernel's any-zone TargetsValid read (danitha_targetsvalid_test.go):
// no live card reaches ValidAttackersAmount EQ or the Self-or-filter sacrifice.

var ktrigqualsDecks = [][2][]cardset.Entry{
	{ // M1: Geralf (second spells) against the attacked-you enchantment and creatures
		{
			{Name: "Geralf, the Fleshwright", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Scathe Zombies", Count: 4},
			{Name: "Essence Scatter", Count: 3}, {Name: "Diabolic Edict", Count: 3}, {Name: "Mind Stone", Count: 2},
			{Name: "Island", Count: 10}, {Name: "Swamp", Count: 10},
		},
		{
			{Name: "Orim's Prayer", Count: 4}, {Name: "Lulu, Stern Guardian", Count: 4}, {Name: "Reveille Squad", Count: 4},
			{Name: "Essence Scatter", Count: 3}, {Name: "Mind Stone", Count: 3}, {Name: "Plains", Count: 11}, {Name: "Island", Count: 11},
		},
	},
	{ // M2: Reparations (a player alternative) against burn, Ichneumon Druid and pump
		{
			{Name: "Reparations", Count: 4}, {Name: "Shock", Count: 5}, {Name: "Lightning Bolt", Count: 5}, {Name: "Essence Scatter", Count: 3},
			{Name: "Raging Goblin", Count: 3},
			{Name: "Plains", Count: 6}, {Name: "Island", Count: 6}, {Name: "Mountain", Count: 8},
		},
		{
			{Name: "Ichneumon Druid", Count: 4}, {Name: "Grizzly Bears", Count: 3}, {Name: "Raging Goblin", Count: 3}, {Name: "Hill Giant", Count: 3},
			{Name: "Llanowar Elves", Count: 3}, {Name: "Giant Growth", Count: 3}, {Name: "Shock", Count: 2}, {Name: "Lightning Bolt", Count: 2},
			{Name: "Forest", Count: 9}, {Name: "Mountain", Count: 8},
		},
	},
	{ // M3: Jiang (one attacker alone) and Sabotage (an attacked-you pump of the whole group) against creatures and burn
		{
			{Name: "Jiang Yanggu, Alone", Count: 3}, {Name: "Sabotage Strategist", Count: 4}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Raging Goblin", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Llanowar Elves", Count: 3},
			{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Giant Growth", Count: 2},
			{Name: "Mountain", Count: 5}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 2},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Llanowar Elves", Count: 4},
			{Name: "Giant Growth", Count: 3}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Mind Stone", Count: 2},
			{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 5},
		},
	},
}

// ktrigqualsStats is what the gorge-side event log says each qualifier decided.
type ktrigqualsStats struct {
	geralfTrig   int // Geralf's trigger went on the stack (a second-or-later spell on its controller's turn)
	geralfFirst  int // a first spell of the turn cast beside Geralf: no trigger
	geralfOppT   int // a second spell cast on the opponent's turn beside Geralf: no trigger
	druidTrig    int // Ichneumon Druid's trigger (the opponent's second instant)
	druidFirst   int // an instant cast beside Druid by its opponent as that player's first spell
	atkYouTrig   int // Orim's Prayer, Lulu or Reveille Squad fired for an attack on its controller
	atkOwnNoTrig int // a player declared attackers while controlling one of those three: no trigger for the attacker's own
	reparBurn    int // Reparations' trigger on a Lightning Bolt or Shock cast at the controller or its creature
	reparOwn     int // ... with the burn aimed at the controller's own creature
	reparNo      int // a spell the opponent of a Reparations controller cast without a trigger (no target, or a target that is not the controller or its creature)
	reparOppCrea int // ... of which aimed at the caster's own creature
	jiangOne     int // Jiang's trigger for a declaration of exactly one attacker (ValidAttackersAmount EQ1)
	jiangMulti   int // a declaration of two or more attackers by a Jiang's controller: no trigger
	sabTrig      int // Sabotage Strategist's trigger (an attack on its controller)
	sabGroup     int // ... for a declaration of two or more attackers (the whole group is pumped)
	sabOwnNo     int // a declaration by a Sabotage Strategist's own controller: no trigger
}

type ktrigqualsTally struct {
	evAt     int
	repCrea  bool // the last spell's chosen target is a permanent of the Reparations controller (the caster's opponent)
	ownCrea  bool // ... a permanent its caster controls
	turn     state.PlayerID
	spells   [2]int
	last     string
	lastCtrl state.PlayerID
	lastN    int
	pushed   bool
	rep      bool // the last spell's caster faces a Reparations
	atkN     int  // attackers of the last DeclareAttackers event
}

func (p *ktrigqualsTally) scan(ref *GorgeSide, st *ktrigqualsStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TurnChange:
			p.turn, p.spells = e.Player, [2]int{}
		case events.Resolve, events.StepChange:
			p.settle(g, st) // the cast's triggers are pushed before anything resolves
		case events.PutOnStack:
			p.settle(g, st)
			n := chName(g, e.Obj)
			if n == "" {
				break
			}
			c := e.Player
			p.repCrea, p.ownCrea = false, false
			p.spells[c]++
			p.last, p.lastCtrl, p.lastN, p.pushed = n, c, p.spells[c], false
			if battlefieldHasName(g, c, "Geralf, the Fleshwright") {
				switch {
				case c != p.turn:
					if p.spells[c] >= 2 {
						st.geralfOppT++
					}
				case p.spells[c] == 1:
					st.geralfFirst++
				}
			}
			p.rep = battlefieldHasName(g, 1-c, "Reparations")
		case events.TargetsChosen:
			if len(e.IDs) > 0 {
				if o := g.Obj(e.IDs[0]); o != nil && o.Zone == state.ZBattlefield {
					p.ownCrea, p.repCrea = o.Controller == p.lastCtrl, o.Controller != p.lastCtrl
				}
			}
		case events.TriggerPush:
			switch chName(g, e.Obj) {
			case "Geralf, the Fleshwright":
				st.geralfTrig++
				p.pushed = true
			case "Ichneumon Druid":
				st.druidTrig++
			case "Orim's Prayer", "Lulu, Stern Guardian", "Reveille Squad":
				st.atkYouTrig++
			case "Jiang Yanggu, Alone":
				if p.atkN == 1 {
					st.jiangOne++
				}
			case "Sabotage Strategist":
				st.sabTrig++
				if p.atkN >= 2 {
					st.sabGroup++
				}
			case "Reparations":
				p.pushed = true
				if p.last == "Shock" || p.last == "Lightning Bolt" {
					st.reparBurn++
				}
				if p.repCrea {
					st.reparOwn++
				}
			}
		case events.DeclareAttackers:
			p.atkN = max(len(e.IDs), 1)
			a := e.Obj
			if len(e.IDs) > 0 {
				a = e.IDs[0]
			}
			if o := g.Obj(a); o != nil {
				if battlefieldHasName(g, o.Controller, "Sabotage Strategist") {
					st.sabOwnNo++
				}
				if p.atkN >= 2 && battlefieldHasName(g, o.Controller, "Jiang Yanggu, Alone") {
					st.jiangMulti++
				}
			}
			if o := g.Obj(e.Obj); o != nil {
				c := o.Controller
				if battlefieldHasName(g, c, "Orim's Prayer") || battlefieldHasName(g, c, "Lulu, Stern Guardian") || battlefieldHasName(g, c, "Reveille Squad") {
					st.atkOwnNoTrig++
				}
			} else if len(e.IDs) > 0 {
				if o := g.Obj(e.IDs[0]); o != nil && (battlefieldHasName(g, o.Controller, "Orim's Prayer") || battlefieldHasName(g, o.Controller, "Lulu, Stern Guardian") || battlefieldHasName(g, o.Controller, "Reveille Squad")) {
					st.atkOwnNoTrig++
				}
			}
		}
	}
	p.evAt = len(evs)
}

// settle books the last cast spell once its triggers have been pushed: a
// spell cast against a Reparations without its trigger, a first instant
// beside Druid.
func (p *ktrigqualsTally) settle(g *state.Game, st *ktrigqualsStats) {
	if p.last == "" {
		return
	}
	if !p.pushed && p.rep {
		st.reparNo++
		if p.ownCrea {
			st.reparOppCrea++
		}
	}
	if p.last == "Shock" || p.last == "Lightning Bolt" || p.last == "Diabolic Edict" || p.last == "Giant Growth" || p.last == "Essence Scatter" {
		if battlefieldHasName(g, 1-p.lastCtrl, "Ichneumon Druid") && p.lastN == 1 {
			st.druidFirst++
		}
	}
	p.last = ""
}

// ktrigqualsPolicy plays at random among casting, activating and playing
// lands (so the carriers are reached), attacks whenever it can, and is
// seeded.
func ktrigqualsPolicy(seed uint64, st *ktrigqualsStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x7472696771756c0a)) // "trigqul"
	var buf []Canon
	var tl ktrigqualsTally
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

// runTrigquals plays the constructed matchups, each in both seatings, and
// returns the games, divergences, stalls and the counts. mutate inverts the
// kernel's qualifier answers (the mutation must diverge).
func runTrigquals(t *testing.T, seeds int, mutate bool) (games int, divs, stalls [3]int, st [3]ktrigqualsStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigquals")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugTrigQualsFlip
	dirac.DebugTrigQualsFlip = mutate
	defer func() { dirac.DebugTrigQualsFlip = old }()
	for mi, md0 := range ktrigqualsDecks {
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
				m, err := NewMatchup(reg, [2]string{"ktrigquals-a", "ktrigquals-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, ktrigqualsPolicy(seed, &st[mi]), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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
						if os.Getenv("KTQ_DEBUG") != "" {
							evs := g.E.L.Events
							for i := max(0, len(evs)-14); i < len(evs); i++ {
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

func TestTrigqualsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 per matchup
	games, divs, stalls, st := runTrigquals(t, 96, false)
	t.Logf("%d games, %v divergences, %v stalls", games, divs, stalls)
	t.Logf("M1 %+v", st[0])
	t.Logf("M2 %+v", st[1])
	t.Logf("M3 %+v", st[2])
	if divs != [3]int{} || stalls != [3]int{} {
		t.Fatalf("%v divergences, %v stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Geralf triggers on a second spell", st[0].geralfTrig}, {"Geralf first spells (no trigger)", st[0].geralfFirst},
		{"Attacked-you triggers", st[0].atkYouTrig}, {"attacks by a controller of an attacked-you card", st[0].atkOwnNoTrig},
		{"Druid triggers (second instant)", st[1].druidTrig}, {"Druid first instants (no trigger)", st[1].druidFirst},
		{"Reparations triggers on burn", st[1].reparBurn}, {"Reparations triggers on a creature of its controller", st[1].reparOwn},
		{"spells cast against Reparations without a trigger", st[1].reparNo}, {"spells aimed at the caster's own creature against Reparations", st[1].reparOppCrea},
		{"Jiang triggers on a lone attacker", st[2].jiangOne}, {"Sabotage triggers", st[2].sabTrig},
		{"Sabotage triggers on a declaration of two or more attackers", st[2].sabGroup}, {"declarations by a Sabotage controller (no trigger)", st[2].sabOwnNo},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestTrigqualsMutation inverts the kernel's qualifier answers (the count
// gates, the AttackedTarget$ You defender and Reparations' player alternative):
// each matchup must diverge. A green mutation means the lockstep never
// reaches a decision the qualifiers shape.
func TestTrigqualsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runTrigquals(t, 64, true)
	for mi, d := range divs {
		if d == 0 {
			t.Errorf("matchup %d: mutation inverted the qualifier answers with no divergence", mi+1)
		}
	}
	t.Logf("mutation diverged %v in %d games", divs, games)
}
