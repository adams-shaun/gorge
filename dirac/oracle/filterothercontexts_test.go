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

// The 14 non-Standard cards lifted as a side effect, each
// reaching a filter word in a context the filterzonepower pool does not: Coerced
// Confession and Sidequest: Catch a Fish (inZone<Zone> on remembered cards),
// Reprieve (inZoneStack target), High Score (inZoneBattlefield in a counter
// replacement; greatestPower end-step draw), Éomer of the Riddermark, Might
// Makes Right, Primal Empathy, Topple and Strength-Testing Hammer
// (greatestPower as a trigger condition, a count, a branch, a target filter
// and an LKI copy; Historian's Wisdom is refused in kgen), Viper, Cruel Conspirator
// (attackingAlone targets), Slitherwisp, Errant and Giada and Waterlogged
// Teachings (hasKeywordFlash on a spell, a library card and a search).

var kprops7bDecks = [][2][]cardset.Entry{
	{ // M1 (G/W counters, Hammer and Topple vs R/G burn and Éomer)
		{
			{Name: "High Score", Count: 4}, {Name: "Topple", Count: 3}, {Name: "Reprieve", Count: 2},
			{Name: "Strength-Testing Hammer", Count: 3}, {Name: "Sidequest: Catch a Fish", Count: 3},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Craw Wurm", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Glory Seeker", Count: 3},
			{Name: "Plains", Count: 6}, {Name: "Forest", Count: 6},
		},
		{
			{Name: "Éomer of the Riddermark", Count: 4}, {Name: "Might Makes Right", Count: 3}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
			{Name: "Hill Giant", Count: 5}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3},
			{Name: "Mountain", Count: 7}, {Name: "Forest", Count: 6},
		},
	},
	{ // M2 (U/B flash and mill vs W/U Errant and Giada)
		{
			{Name: "Slitherwisp", Count: 4}, {Name: "Viper, Cruel Conspirator", Count: 4}, {Name: "Coerced Confession", Count: 4},
			{Name: "Waterlogged Teachings", Count: 3}, {Name: "Spectral Sailor", Count: 3}, {Name: "Brineborn Cutthroat", Count: 3},
			{Name: "Counterspell", Count: 3}, {Name: "Divination", Count: 2}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 7},
		},
		{
			{Name: "Errant and Giada", Count: 4}, {Name: "Reprieve", Count: 3}, {Name: "Spectral Sailor", Count: 3}, {Name: "Brineborn Cutthroat", Count: 3},
			{Name: "Savannah Lions", Count: 4}, {Name: "Glory Seeker", Count: 4}, {Name: "Counterspell", Count: 3}, {Name: "Cancel", Count: 2},
			{Name: "Plains", Count: 7}, {Name: "Island", Count: 7},
		},
	},
	{ // M3 (G/U Primal Empathy, High Score and flash vs W/R Topple, Éomer and Might Makes Right)
		{
			{Name: "Primal Empathy", Count: 4}, {Name: "High Score", Count: 3}, {Name: "Ambush Viper", Count: 4}, {Name: "Strength-Testing Hammer", Count: 2},
			{Name: "Spectral Sailor", Count: 3}, {Name: "Grizzly Bears", Count: 5}, {Name: "Llanowar Elves", Count: 6},
			{Name: "Forest", Count: 8}, {Name: "Island", Count: 5},
		},
		{
			{Name: "Topple", Count: 3}, {Name: "Might Makes Right", Count: 3}, {Name: "Éomer of the Riddermark", Count: 3}, {Name: "Hill Giant", Count: 5},
			{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Savannah Lions", Count: 3}, {Name: "Glory Seeker", Count: 3},
			{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 7},
		},
	},
}

// kprops7bStats is what the gorge-side event log says each context decided.
type kprops7bStats struct {
	hsCounter    int // a +1/+1 counter put with High Score's replacement applying (two counters for one)
	hsDraw       int // High Score's end-step trigger drew (its controller has the greatest power)
	hsNoDraw     int // ... resolved and drew nothing
	hammerDraw   int // Strength-Testing Hammer's attack trigger drew
	hammerNoDraw int // ... resolved and drew nothing
	eoAttack     int // Éomer attacked
	eoTrig       int // ... and its intervening-if trigger fired
	mmrCombat    int // a beginning of combat on its controller's turn with Might Makes Right on the battlefield
	mmrTrig      int // ... and its trigger fired (EQ0 held)
	peDraw       int // Primal Empathy's upkeep trigger drew
	peCounter    int // ... took the counter branch
	topple       int // Topple resolved and exiled a creature
	viperAlone   int // a lone attacker declared with Viper, Cruel Conspirator on the battlefield
	viperGang    int // two or more attackers declared with Viper on the battlefield
	viperAct     int // Viper activations
	viperLone    int // ... while a lone attacker was declared this combat (a legal attackingAlone target)
	slithTrig    int // Slitherwisp's flash-spell trigger fired
	libCast      int // Errant and Giada: a spell cast from the library
	reprieve     int // Reprieve resolved and returned a spell
	ccDraw       int // Coerced Confession resolved and drew (a creature was milled)
	ccNone       int // ... resolved and drew nothing
	sqTrig       int // Sidequest's upkeep trigger resolved
	sqTake       int // ... and the revealed card went to the hand
	wtSearch     int // Waterlogged Teachings resolved and fetched a card
}

// kprops7bTally reads the gorge event log between decisions. cur is the
// resolving stack object's source card name (cleared at the next priority),
// the flags are what its resolution did.
type kprops7bTally struct {
	evAt    int
	cur     string
	trig    bool // cur is a triggered ability
	drew    bool
	ctr     bool // a counter was put
	exiled  bool // a battlefield card went to exile
	search  bool // a card moved library -> hand
	bounced bool // a spell went stack -> hand
	curObj  state.ObjID
	lone    bool // exactly one attacker was declared this combat
}

// srcName is the card name behind a stack object (a triggered or activated
// ability resolves under its source's name).
func kprops7bSrcName(g *state.Game, id state.ObjID) string {
	o := g.Obj(id)
	if o == nil {
		return ""
	}
	if o.Source != 0 {
		return chName(g, o.Source)
	}
	return chName(g, id)
}

func (p *kprops7bTally) flush(st *kprops7bStats) {
	switch p.cur {
	case "High Score":
		if p.trig {
			if p.drew {
				st.hsDraw++
			} else {
				st.hsNoDraw++
			}
		}
	case "Strength-Testing Hammer":
		if p.trig {
			if p.drew {
				st.hammerDraw++
			} else {
				st.hammerNoDraw++
			}
		}
	case "Primal Empathy":
		if p.drew {
			st.peDraw++
		}
		if p.ctr {
			st.peCounter++
		}
	case "Topple":
		if !p.trig && p.exiled {
			st.topple++
		}
	case "Reprieve":
		if !p.trig && p.bounced {
			st.reprieve++
		}
	case "Coerced Confession":
		if !p.trig {
			if p.drew {
				st.ccDraw++
			} else {
				st.ccNone++
			}
		}
	case "Sidequest: Catch a Fish", "Cooking Campsite":
		if p.trig {
			st.sqTrig++
			if p.search {
				st.sqTake++
			}
		}
	case "Waterlogged Teachings":
		if !p.trig && p.search {
			st.wtSearch++
		}
	}
	p.cur, p.trig, p.drew, p.ctr, p.exiled, p.search, p.bounced = "", false, false, false, false, false, false
}

func (p *kprops7bTally) onBattlefield(g *state.Game, name string) bool {
	for pl := state.PlayerID(0); pl < 2; pl++ {
		for _, b := range g.Zone(state.ZBattlefield, pl) {
			if chName(g, b) == name {
				return true
			}
		}
	}
	return false
}

func (p *kprops7bTally) scan(ref *GorgeSide, st *kprops7bStats) {
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
			if e.Kind != events.Priority {
				p.lone = false
			}
			if e.Kind == events.StepChange && e.Step == state.StepBeginCombat && p.onBattlefield(g, "Might Makes Right") {
				st.mmrCombat++
			}
		case events.Resolve:
			p.flush(st)
			p.cur = kprops7bSrcName(g, e.Obj)
			p.curObj = e.Obj
			if o := g.Obj(e.Obj); o != nil {
				p.trig = state.StackKindOf(g, o) != state.StackKindSpell
			}
		case events.Draw:
			p.drew = true
		case events.CounterChange:
			if e.Amount > 0 && e.Counter == "P1P1" {
				p.ctr = true
				if e.Amount >= 2 {
					st.hsCounter++
				}
			}
		case events.MoveZone:
			if e.From == state.ZBattlefield && e.To == state.ZExile && p.cur != "" {
				p.exiled = true
			}
			if e.From == state.ZStack && e.To == state.ZHand && e.Obj != p.curObj && p.cur != "" {
				p.bounced = true
			}
			if e.From == state.ZLibrary && e.To == state.ZHand && p.cur != "" {
				p.search = true
			}
		case events.PutOnStack:
			if e.From == state.ZLibrary && p.onBattlefield(g, "Errant and Giada") {
				st.libCast++
			}
		case events.TriggerPush:
			switch chName(g, e.Obj) {
			case "Éomer of the Riddermark":
				st.eoTrig++
			case "Might Makes Right":
				st.mmrTrig++
			case "Slitherwisp":
				st.slithTrig++
			}
		case events.AbilityPush:
			if chName(g, e.Obj) == "Viper, Cruel Conspirator" {
				st.viperAct++
				if p.lone {
					st.viperLone++
				}
			}
		case events.DeclareAttackers:
			ids := e.IDs
			if len(ids) == 0 {
				ids = []state.ObjID{e.Obj}
			}
			for _, id := range ids {
				if chName(g, id) == "Éomer of the Riddermark" {
					st.eoAttack++
				}
			}
			p.lone = len(ids) == 1
			if p.onBattlefield(g, "Viper, Cruel Conspirator") {
				if len(ids) == 1 {
					st.viperAlone++
				} else if len(ids) >= 2 {
					st.viperGang++
				}
			}
		}
	}
	p.evAt = len(evs)
}

// kprops7bPolicy plays at random among casting, activating and playing lands
// (so the carriers are reached), attacks at random, and counts what the
// gorge side logs.
func kprops7bPolicy(seed uint64, st *kprops7bStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x70726f70733762)) // "props7b"
	var buf []Canon
	var tl kprops7bTally
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
			if len(buf) > 0 && prng.IntN(3) != 0 {
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

// runFilterothercontexts plays the constructed matchups, each in both seatings, and
// returns the games, divergences, stalls and the word counts. mutate negates
// every kernel PredWord7 answer (the mutation must diverge).
func runFilterothercontexts(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, st kprops7bStats) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("filterothercontexts")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugWord7Flip
	dirac.DebugWord7Flip = mutate
	defer func() { dirac.DebugWord7Flip = old }()
	for mi, md0 := range kprops7bDecks {
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
				m, err := NewMatchup(reg, [2]string{"kprops7b-a", "kprops7b-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, kprops7bPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestFilterWordsOtherContextsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	// 96 seeds per seating: 192 per matchup
	games, divs, stalls, st := runFilterothercontexts(t, 96, false)
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"High Score counters doubled", st.hsCounter}, {"High Score end-step draws", st.hsDraw}, {"High Score end steps without the greatest power", st.hsNoDraw},
		{"Strength-Testing Hammer draws", st.hammerDraw}, {"Strength-Testing Hammer no draw", st.hammerNoDraw},
		{"Éomer attacks with the trigger", st.eoTrig}, {"Éomer attacks without the greatest power", st.eoAttack - st.eoTrig},
		{"Might Makes Right combats", st.mmrCombat}, {"Might Makes Right triggers", st.mmrTrig},
		{"Primal Empathy draws", st.peDraw}, {"Primal Empathy counters", st.peCounter},
		{"Topple exiles", st.topple},
		{"Viper lone attacker", st.viperAlone}, {"Viper multiple attackers", st.viperGang}, {"Viper activations", st.viperAct}, {"Viper activations during a lone attack", st.viperLone},
		{"Slitherwisp triggers", st.slithTrig}, {"Errant and Giada library casts", st.libCast},
		{"Reprieve resolutions", st.reprieve},
		{"Coerced Confession draws", st.ccDraw}, {"Coerced Confession no creature milled", st.ccNone},
		{"Sidequest triggers", st.sqTrig}, {"Sidequest takes", st.sqTake}, {"Waterlogged Teachings searches", st.wtSearch},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}

// TestFilterWordsOtherContextsMutation negates every kernel PredWord7 answer: the games must
// diverge. A green mutation means the lockstep never reaches a decision the
// words shape.
func TestFilterWordsOtherContextsMutation(t *testing.T) {
	games, divs, _, _ := runFilterothercontexts(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation negated every PredWord7 answer in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
