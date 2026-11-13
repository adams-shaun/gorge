package oracle

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Constructed matchups over the generated mixedarms pool, chosen so the mixed arms
// come up in play. Every arm's occurrences are counted on the gorge side (stack
// objects, the asks posed while an arm card's object is on top, and the
// event log) and asserted, so the test proves its own coverage.

var mixedarmsDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "Ponder", Count: 3}, {Name: "Portent", Count: 2}, {Name: "Omen", Count: 2}, {Name: "Sage Owl", Count: 2}, {Name: "Twiddle", Count: 2}, {Name: "Jolt", Count: 1}, {Name: "Pestermite", Count: 2}, {Name: "Shipwreck Looter", Count: 2}, {Name: "Academy Wall", Count: 1}, {Name: "Windrider Wizard", Count: 2}, {Name: "Remand", Count: 2}, {Name: "Memory Lapse", Count: 2}, {Name: "Dissipate", Count: 1}, {Name: "Void Shatter", Count: 1}, {Name: "Island", Count: 15},
		},
		{
			{Name: "Wei Scout", Count: 2}, {Name: "Wei Strike Force", Count: 2}, {Name: "Wei Night Raiders", Count: 2}, {Name: "Yellow Scarves Cavalry", Count: 2}, {Name: "Highborn Ghoul", Count: 2}, {Name: "Krenko's Enforcer", Count: 2}, {Name: "Academy Raider", Count: 2}, {Name: "Sickle Ripper", Count: 2}, {Name: "Rustrazor Butcher", Count: 2}, {Name: "Lava Storm", Count: 2}, {Name: "Puncture Blast", Count: 2}, {Name: "Walking Ballista", Count: 1}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 9},
		},
	},
	{
		{
			{Name: "Jadelight Ranger", Count: 2}, {Name: "Merfolk Branchwalker", Count: 2}, {Name: "Tishana's Wayfinder", Count: 2}, {Name: "Cenote Scout", Count: 2}, {Name: "Dromoka Captain", Count: 2}, {Name: "Honor's Reward", Count: 2}, {Name: "Map the Wastes", Count: 1}, {Name: "Sandsteppe Scavenger", Count: 2}, {Name: "Cached Defenses", Count: 1}, {Name: "Evolution Sage", Count: 2}, {Name: "Bloom Hulk", Count: 1}, {Name: "Pollenbright Druid", Count: 2}, {Name: "Huatli's Raptor", Count: 2}, {Name: "Walk with the Ancestors", Count: 1}, {Name: "Forest", Count: 9}, {Name: "Plains", Count: 7},
		},
		{
			{Name: "Seekers' Squire", Count: 2}, {Name: "Deadeye Tracker", Count: 2}, {Name: "Scheming Aspirant", Count: 2}, {Name: "Drown in Ichor", Count: 2}, {Name: "Grim Affliction", Count: 1}, {Name: "Blightbelly Rat", Count: 2}, {Name: "Frost Breath", Count: 2}, {Name: "Chill of the Grave", Count: 1}, {Name: "Sage of Epityr", Count: 2}, {Name: "Spire Owl", Count: 2}, {Name: "Thrummingbird", Count: 2}, {Name: "Contentious Plan", Count: 1}, {Name: "Steady Progress", Count: 2}, {Name: "Island", Count: 9}, {Name: "Swamp", Count: 8},
		},
	},
	{
		{
			{Name: "Swamp Mosquito", Count: 3}, {Name: "Crypt Cobra", Count: 2}, {Name: "Suq'Ata Assassin", Count: 2}, {Name: "Mindstab Thrull", Count: 2}, {Name: "Dauthi Mindripper", Count: 2}, {Name: "Pistus Strike", Count: 2}, {Name: "Infectious Bite", Count: 2}, {Name: "Poison Dart Frog", Count: 2}, {Name: "Fog", Count: 2}, {Name: "Fog of Gnats", Count: 1}, {Name: "Rhonas's Last Stand", Count: 2}, {Name: "Bontu's Last Reckoning", Count: 1}, {Name: "Swamp", Count: 9}, {Name: "Forest", Count: 8},
		},
		{
			{Name: "Oketra's Last Mercy", Count: 2}, {Name: "Arbalest Elite", Count: 1}, {Name: "Homarid Warrior", Count: 1}, {Name: "Patrol Signaler", Count: 3}, {Name: "Safehold Sentry", Count: 3}, {Name: "Order of Whiteclay", Count: 2}, {Name: "Puresight Merrow", Count: 2}, {Name: "Coral Fighters", Count: 2}, {Name: "Jeskai Elder", Count: 2}, {Name: "Daring Saboteur", Count: 2}, {Name: "Blessed Wind", Count: 1}, {Name: "Twiddle", Count: 2}, {Name: "Plains", Count: 9}, {Name: "Island", Count: 8},
		},
	},
	{
		{
			{Name: "Clever Lumimancer", Count: 2}, {Name: "Symmetry Sage", Count: 2}, {Name: "Archmage Emeritus", Count: 1}, {Name: "Ponder", Count: 2}, {Name: "Portent", Count: 2}, {Name: "Twiddle", Count: 2}, {Name: "Memory Lapse", Count: 1}, {Name: "Walking Ballista", Count: 2}, {Name: "Steady Progress", Count: 2}, {Name: "Honor's Reward", Count: 2}, {Name: "Silkbind Faerie", Count: 2}, {Name: "Puresight Merrow", Count: 1}, {Name: "Shipwreck Looter", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 9},
		},
		{
			{Name: "Camellia, the Seedmiser", Count: 2}, {Name: "Savvy Hunter", Count: 2}, {Name: "Bake into a Pie", Count: 2}, {Name: "Fell the Pheasant", Count: 2}, {Name: "Primordial Gnawer", Count: 2}, {Name: "Digsite Conservator", Count: 2}, {Name: "Walk with the Ancestors", Count: 1}, {Name: "Juvenile Gloomwidow", Count: 2}, {Name: "Twinblade Slasher", Count: 2}, {Name: "Lifebane Zombie", Count: 1}, {Name: "Walking Ballista", Count: 2}, {Name: "Hidden Nursery", Count: 1}, {Name: "Hidden Necropolis", Count: 1}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 10},
		},
	},
	{
		// kxpaid: a trigger reading the X its permanent's spell was cast
		// for (CR 107.3m), and a cast trigger reading the cast spell's X
		{
			{Name: "Triceraton Commander", Count: 4}, {Name: "Knickknack Ouphe", Count: 2}, {Name: "Termagant Swarm", Count: 4}, {Name: "Geometer's Arthropod", Count: 3}, {Name: "Walking Ballista", Count: 3}, {Name: "Dromoka Captain", Count: 2}, {Name: "Sandsteppe Scavenger", Count: 1}, {Name: "Honor's Reward", Count: 2}, {Name: "Pollenbright Druid", Count: 2}, {Name: "Forest", Count: 9}, {Name: "Plains", Count: 5}, {Name: "Island", Count: 3},
		},
		{
			{Name: "The Meathook Massacre", Count: 2}, {Name: "Ajani's Anguish", Count: 2}, {Name: "Spiteful Banditry", Count: 2}, {Name: "Morlun, Devourer of Spiders", Count: 2}, {Name: "Wei Night Raiders", Count: 2}, {Name: "Highborn Ghoul", Count: 2}, {Name: "Krenko's Enforcer", Count: 2}, {Name: "Lava Storm", Count: 2}, {Name: "Puncture Blast", Count: 2}, {Name: "Sickle Ripper", Count: 2}, {Name: "Academy Raider", Count: 2}, {Name: "Walking Ballista", Count: 1}, {Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 8},
		},
	},
	{
		// kxpaid: Walk with the Ancestors' discover hitting a spell with
		// targets (Bake into a Pie) is cast with Walk held on the stack
		{
			{Name: "Walk with the Ancestors", Count: 4}, {Name: "Bake into a Pie", Count: 4}, {Name: "Fell the Pheasant", Count: 3}, {Name: "Primordial Gnawer", Count: 2}, {Name: "Digsite Conservator", Count: 2}, {Name: "Juvenile Gloomwidow", Count: 3}, {Name: "Twinblade Slasher", Count: 3}, {Name: "Savvy Hunter", Count: 2}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 9},
		},
		{
			{Name: "Clever Lumimancer", Count: 2}, {Name: "Symmetry Sage", Count: 2}, {Name: "Archmage Emeritus", Count: 1}, {Name: "Ponder", Count: 2}, {Name: "Portent", Count: 2}, {Name: "Twiddle", Count: 2}, {Name: "Memory Lapse", Count: 1}, {Name: "Walking Ballista", Count: 2}, {Name: "Steady Progress", Count: 2}, {Name: "Honor's Reward", Count: 2}, {Name: "Silkbind Faerie", Count: 2}, {Name: "Puresight Merrow", Count: 1}, {Name: "Shipwreck Looter", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 9},
		},
	},
}

// mixedarmsArm is one counted arm: the cards that carry it and how an occurrence
// is recognised on the gorge side.
type mixedarmsArm struct {
	name  string
	cards []string
	// how: "stack" = an arm card's spell or ability put on the stack;
	// "ask" = a non-priority decision posed while an arm card's object is
	// on top of the stack; "attack" = an arm card declared as an attacker;
	// "ev" = the event matcher ev; "xtrig" = an arm card's triggered
	// ability put on the stack while its source permanent holds a paid X
	// above 0; "hold" = a non-priority decision posed while an arm card's
	// spell is held under the top of the stack (a free cast's choices).
	how string
	// abil: a "stack" arm counts only the cards' abilities (not the
	// creature spells themselves)
	abil bool
	// dk / notDk: an "ask" arm counts only gorge decisions of kind dk
	// (when set), never of kind notDk
	dk, notDk decision.Kind
	ev        func(e *events.Event) bool
	evg       func(g *state.Game, e *events.Event) bool
	min       int
}

var mixedarmsArms = []mixedarmsArm{
	{name: "PayDraw", how: "ask", cards: []string{"Shipwreck Looter", "Academy Wall", "Windrider Wizard", "Jeskai Elder", "Daring Saboteur"}},
	{name: "{Q}", how: "stack", abil: true, cards: []string{"Patrol Signaler", "Safehold Sentry", "Order of Whiteclay", "Puresight Merrow", "Silkbind Faerie"}},
	{name: "{X}{X}", how: "stack", cards: []string{"Walking Ballista"}},
	{name: "Horsemanship", how: "attack", cards: []string{"Wei Scout", "Wei Strike Force", "Wei Night Raiders", "Yellow Scarves Cavalry"}},
	{name: "Intimidate", how: "attack", cards: []string{"Highborn Ghoul", "Krenko's Enforcer", "Academy Raider", "Lifebane Zombie"}},
	{name: "Wither", how: "ev", ev: func(e *events.Event) bool {
		return e.Kind == events.CounterChange && e.Counter == "M1M1" && e.Amount > 0
	}},
	{name: "Counter Destination$", how: "ev", evg: func(g *state.Game, e *events.Event) bool {
		if e.Kind != events.MoveZone || e.From != state.ZStack || e.To != state.ZHand && e.To != state.ZLibrary && e.To != state.ZExile {
			return false
		}
		o := g.Obj(e.Obj)
		return o != nil && o.Ability == nil && o.Card != nil // a countered spell, not an ability leaving the stack
	}},
	{name: "SkipNextUntap", how: "stack", cards: []string{"Frost Breath", "Chill of the Grave"}},
	{name: "SkipUntap", how: "stack", cards: []string{"Oketra's Last Mercy", "Rhonas's Last Stand", "Bontu's Last Reckoning", "Arbalest Elite", "Homarid Warrior"}},
	{name: "Fog", how: "stack", cards: []string{"Fog"}},
	{name: "Poison", how: "ev", ev: func(e *events.Event) bool {
		return e.Kind == events.PlayerCounterChange && e.Counter == "POISON" && e.Amount > 0
	}},
	{name: "SetLife", how: "stack", cards: []string{"Oketra's Last Mercy", "Blessed Wind"}},
	{name: "TapOrUntap", how: "ask", cards: []string{"Twiddle", "Jolt", "Pestermite"}},
	{name: "Explore", how: "ev", ev: func(e *events.Event) bool { return e.Kind == events.Explore }},
	{name: "Rearrange", how: "ask", dk: decision.KArrange, cards: []string{"Ponder", "Portent", "Omen", "Sage Owl", "Sage of Epityr", "Spire Owl"}},
	{name: "Rearrange MayShuffle", how: "ask", notDk: decision.KArrange, cards: []string{"Ponder", "Omen"}},
	{name: "GenericChoice", how: "ask", cards: []string{"Lava Storm"}},
	{name: "Discover", how: "ev", ev: func(e *events.Event) bool { return e.Kind == events.Discover }},
	{name: "Discover cast", how: "ask", cards: []string{"Primordial Gnawer", "Digsite Conservator", "Walk with the Ancestors", "Hidden Nursery", "Hidden Necropolis"}},
	{name: "AttackerUnblocked", how: "stack", abil: true, cards: []string{"Swamp Mosquito", "Crypt Cobra", "Suq'Ata Assassin", "Mindstab Thrull", "Dauthi Mindripper", "Coral Fighters"}},
	{name: "SpellCastOrCopy", how: "stack", abil: true, cards: []string{"Clever Lumimancer", "Symmetry Sage", "Archmage Emeritus"}},
	{name: "Bolster", how: "stack", cards: []string{"Dromoka Captain", "Honor's Reward", "Map the Wastes", "Sandsteppe Scavenger", "Cached Defenses"}},
	{name: "Bolster tie", how: "ask", cards: []string{"Dromoka Captain", "Honor's Reward", "Map the Wastes", "Sandsteppe Scavenger", "Cached Defenses"}},
	{name: "Proliferate", how: "ev", ev: func(e *events.Event) bool { return e.Kind == events.Proliferate }},
	{name: "Proliferate pick", how: "ask", cards: []string{"Evolution Sage", "Bloom Hulk", "Pollenbright Druid", "Huatli's Raptor", "Drown in Ichor", "Grim Affliction", "Blightbelly Rat", "Thrummingbird", "Contentious Plan", "Steady Progress"}},
	{name: "TrigProliferate", how: "stack", abil: true, cards: []string{"Scheming Aspirant"}},
	{name: "SacrificedOnce", how: "stack", abil: true, cards: []string{"Camellia, the Seedmiser"}},
	{name: "SacrificedOnce cost batch", how: "stack", abil: true, cards: []string{"Savvy Hunter"}},
	{name: "xPaid enters trigger", how: "xtrig", cards: []string{"Triceraton Commander", "Knickknack Ouphe", "Termagant Swarm", "The Meathook Massacre", "Ajani's Anguish", "Spiteful Banditry", "Morlun, Devourer of Spiders"}},
	{name: "xPaid cast trigger", how: "stack", abil: true, cards: []string{"Geometer's Arthropod"}},
	{name: "Spell hold", how: "hold", cards: []string{"Walk with the Ancestors"}, min: 10}, // a discovered spell with a choice: 14 on seeds 1..64
}

// mixedarmsCounter counts every arm's occurrences in one gorge game through
// RunConfig.Check (called at every decision both sides agree on).
type mixedarmsCounter struct {
	n      []int
	seen   map[state.ObjID]bool
	asked  map[[2]uint64]bool // (stack top, arm) already counted as an ask
	evAt   int
	byCard map[string][]int // card name -> arm indexes (stack/ask/attack)
}

func newMixedarmsCounter() *mixedarmsCounter {
	c := &mixedarmsCounter{n: make([]int, len(mixedarmsArms)), byCard: map[string][]int{}}
	for i, a := range mixedarmsArms {
		for _, n := range a.cards {
			c.byCard[n] = append(c.byCard[n], i)
		}
	}
	return c
}

func (c *mixedarmsCounter) reset() {
	c.seen, c.asked, c.evAt = map[state.ObjID]bool{}, map[[2]uint64]bool{}, 0
}

func (c *mixedarmsCounter) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	for _, id := range g.Stack {
		if c.seen[id] {
			continue
		}
		c.seen[id] = true
		ab := false
		if o := g.Obj(id); o != nil {
			ab = o.Ability != nil
		}
		for _, i := range c.byCard[sourceCardName(g, id)] {
			if mixedarmsArms[i].how == "stack" && (ab || !mixedarmsArms[i].abil) {
				c.n[i]++
			}
			if mixedarmsArms[i].how == "xtrig" && ab {
				if src := g.Obj(g.Obj(id).Source); src != nil && src.X > 0 {
					c.n[i]++
				}
			}
		}
	}
	if ref.fr != nil && ref.fr.d != nil && ref.fr.d.Kind != decision.KPriority && len(g.Stack) > 0 {
		top := g.Stack[len(g.Stack)-1]
		for _, i := range c.byCard[sourceCardName(g, top)] {
			k := [2]uint64{uint64(top), uint64(i)}
			if a := &mixedarmsArms[i]; a.how == "ask" && !c.asked[k] && (a.dk == "" || a.dk == ref.fr.d.Kind) && a.notDk != ref.fr.d.Kind {
				c.asked[k] = true
				c.n[i]++
			}
		}
		if len(g.Stack) >= 2 {
			below := g.Stack[len(g.Stack)-2]
			if o := g.Obj(below); o != nil && o.Ability == nil {
				for _, i := range c.byCard[sourceCardName(g, below)] {
					k := [2]uint64{uint64(below), uint64(i)}
					if mixedarmsArms[i].how == "hold" && !c.asked[k] {
						c.asked[k] = true
						c.n[i]++
					}
				}
			}
		}
	}
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind == events.DeclareAttackers {
			for _, id := range e.IDs {
				for _, i := range c.byCard[sourceCardName(g, id)] {
					if mixedarmsArms[i].how == "attack" {
						c.n[i]++
					}
				}
			}
		}
		for i := range mixedarmsArms {
			if mixedarmsArms[i].ev != nil && mixedarmsArms[i].ev(e) || mixedarmsArms[i].evg != nil && mixedarmsArms[i].evg(g, e) {
				c.n[i]++
			}
		}
	}
	c.evAt = len(evs)
	return nil
}

// mixedarmsPolicy prefers playing: with three chances in four it casts, activates
// or plays a land when it can, else it is random.
func mixedarmsPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	r := RandomPolicy(seed ^ 0x9e3779b97f4a7c15)
	var buf []Canon
	n := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		n++
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand || k == ActAttack {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || n%4 == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return r.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// mixedarmsEnv is the corpus, the mixedarms set and each matchup's decks and table.
type mixedarmsEnv struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [][2][]*cards.Card
	tbls  []*kop.Table
}

func newMixedarmsEnv(t *testing.T) *mixedarmsEnv {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mixedarms")
	if err != nil {
		t.Fatal(err)
	}
	env := &mixedarmsEnv{reg: reg, set: set}
	for mi, md := range mixedarmsDecks {
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
		env.decks = append(env.decks, decks)
		env.tbls = append(env.tbls, tbl)
	}
	return env
}

// game plays matchup mi on seed in lockstep: odd seeds on gorge's autopay
// surface, even seeds on the lowering surface; every fifth seed under the
// plain random policy, the others under mixedarmsPolicy.
func (env *mixedarmsEnv) game(t *testing.T, mi int, seed uint64, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	m, err := NewMatchup(env.reg, [2]string{"c4-a", "c4-b"}, env.decks[mi], 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: seed%2 == 1}
	k, err := SetSide(env.set, env.tbls[mi], cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := mixedarmsPolicy(seed)
	if seed%5 == 0 {
		pol = RandomPolicy(seed)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
	CloseSide(k)
	if d := res.Div; d != nil {
		lo := max(d.At-12, 0)
		t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
		t.Errorf("matchup %d seed %d (autopay %v): diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, cfg.GorgeAutopaySurface, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
	}
	return res
}

// TestMixedArmsCountedLockstep plays every matchup on seeds 1..64 (C4_SEEDS=n for a
// wider sweep, which skips the coverage assertion) and asserts each arm
// came up at least its minimum number of times.
func TestMixedArmsCountedLockstep(t *testing.T) {
	t.Parallel()

	env := newMixedarmsEnv(t)
	seeds := uint64(64)
	if v := os.Getenv("C4_SEEDS"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		seeds = n
	}
	cnt := newMixedarmsCounter()
	start := time.Now()
	stalls := map[string]int{}
	games := 0
	for mi := range mixedarmsDecks {
		for seed := uint64(1); seed <= seeds; seed++ {
			cnt.reset()
			res := env.game(t, mi, seed, cnt.check)
			games++
			if res.Stall != "" {
				stalls[res.Stall]++
			}
		}
	}
	var sk []string
	for s, n := range stalls {
		sk = append(sk, s+"="+strconv.Itoa(n))
	}
	sort.Strings(sk)
	t.Logf("%d games in %v; stalls [%s]", games, time.Since(start).Round(time.Millisecond), strings.Join(sk, " "))
	for i, a := range mixedarmsArms {
		min := a.min
		if min == 0 {
			min = 20
		}
		t.Logf("arm %-26s %d", a.name, cnt.n[i])
		if cnt.n[i] < min && os.Getenv("C4_SEEDS") == "" {
			t.Errorf("arm %s occurred %d times, want at least %d", a.name, cnt.n[i], min)
		}
	}
}

// mixedarmsRegressions are games outside TestMixedArmsCountedLockstep's seeds that diverged
// before a kverify-c4 fix (the fixes inside seeds 1..64 are held by
// TestMixedArmsCountedLockstep itself). Matchup 2 seed 376 is not here: gorge looks
// wrong there (Infectious Bite with its first target gone deals damage;
// HANDOFF-kverify-c4).
var mixedarmsRegressions = []struct {
	mi   int
	seed uint64
	fix  string
}{
	{2, 233, "Fog still asks the damage division"},
	{3, 73, "a trigger's mana cost is re-planned after each activation"},
	{3, 190, "a trigger's mana cost is re-planned after each activation"},
	{3, 325, "typed (Cave) mana ends the planned trigger-cost payment: the window pays the rest"},
}

func TestMixedArmsRegressions(t *testing.T) {
	t.Parallel()

	env := newMixedarmsEnv(t)
	for _, r := range mixedarmsRegressions {
		if res := env.game(t, r.mi, r.seed, nil); res.Div != nil {
			t.Errorf("matchup %d seed %d (%s) diverged", r.mi, r.seed, r.fix)
		}
	}
}
