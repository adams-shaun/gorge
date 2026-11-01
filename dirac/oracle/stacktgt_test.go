package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The stack-ability target lockstep: Disallow counters a target
// spell, activated ability or triggered ability, Stifle an activated or
// triggered one; the Prodigals' activations and Man-o'-War, Ravenous Rats
// and Elvish Visionary's enters triggers are the abilities. Counterspell
// removes a spell beneath a targeted ability, so a target outlives an entry
// below it leaving the stack. Ertai Resurrected plays in its own matchup
// (kstacktgtErtaiDecks).
var kstacktgtDecks = [2][]cardset.Entry{
	{
		{Name: "Disallow", Count: 4}, {Name: "Stifle", Count: 3}, {Name: "Counterspell", Count: 3},
		{Name: "Prodigal Sorcerer", Count: 4}, {Name: "Man-o'-War", Count: 4}, {Name: "Ravenous Rats", Count: 3},
		{Name: "Island", Count: 13}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Prodigal Pyromancer", Count: 3}, {Name: "Elvish Visionary", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Mountain", Count: 11}, {Name: "Forest", Count: 11},
	},
}

func TestStacktgtLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "stacktgt", kstacktgtDecks, 1000, "KSTACKTGT_SEEDS")
}

// TestStacktgtLockstepMutation proves a stack-ability target is found by
// its identity: looked up by its stack index instead, some seed diverges
// (an entry below it left the stack first).
func TestStacktgtLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugStackTgtByIndex = true
	defer func() { dirac.DebugStackTgtByIndex = false }()
	if !lockstepDiverges(t, "stacktgt", kstacktgtDecks, 1000) {
		t.Fatal("no seed diverged with index lookup: no targeted ability outlived an entry below it")
	}
}

// gorgeRememberForCounterTime is gorge ("write the linked remember
// params"), the commit time in UTC: before it gorge's effCounter left
// RememberForCounter$ unread, so Ertai Resurrected's "its controller draws a
// card" after its counter mode drew for no one.
const gorgeRememberForCounterTime = "20261026080405"

// kstacktgtErtaiDecks: Ertai Resurrected's enters charm (choose up to one:
// counter target spell, activated or triggered ability, or destroy another
// target creature or planeswalker; either way that thing's controller draws
// a card) with flash, against the Prodigal Pyromancer's activations,
// Elvish Visionary's enters trigger and burn it can counter and creatures it
// can destroy (its own side holds no abilities, so a countered thing's
// controller is the opponent).
var kstacktgtErtaiDecks = [2][]cardset.Entry{
	{
		{Name: "Ertai Resurrected", Count: 7}, {Name: "Wind Drake", Count: 4}, {Name: "Counterspell", Count: 3},
		{Name: "Island", Count: 13}, {Name: "Swamp", Count: 13},
	},
	{
		{Name: "Prodigal Pyromancer", Count: 4}, {Name: "Elvish Visionary", Count: 3}, {Name: "Shock", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 3}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Ertai Resurrected", Count: 1}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
		{Name: "Island", Count: 1}, {Name: "Swamp", Count: 1},
	},
}

// ertaiPolicy holds Ertai Resurrected back for a response: its controller
// casts it only above an opponent's spell or ability, and takes the counter
// mode whenever it is offered; every other decision is castFirstPolicy's.
// casts counts Ertai's counter-mode choices.
func ertaiPolicy(seed uint64, counters *int) Policy {
	base := castFirstPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		if g == nil {
			return base.Pick(c)
		}
		G := g.E.G
		buf = buf[:0]
		respond := len(G.Stack) > 0
		if respond {
			if o := G.Obj(G.Stack[len(G.Stack)-1]); o == nil || o.Controller == state.PlayerID(c.D.Player) {
				respond = false
			}
		}
		for _, a := range c.D.Options {
			opt, ok := g.GorgeOption(a)
			if !ok {
				buf = append(buf, a)
				continue
			}
			if opt.Kind == "mode" && strings.HasPrefix(opt.Label, "Counter") {
				*counters++
				return a
			}
			if a.Kind() == ActCast {
				if o := G.Obj(opt.Obj); o != nil && o.Face().Name == "Ertai Resurrected" {
					if respond {
						return a
					}
					continue
				}
			}
			buf = append(buf, a)
		}
		if len(buf) == 0 || len(buf) == len(c.D.Options) {
			return base.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// ertaiRun plays the Ertai matchup under ertaiPolicy and returns the
// divergent games and the counter-mode choices; report fails on each
// divergence, else the first stops the run.
func ertaiRun(t *testing.T, seeds uint64, report bool, mutate func(*kop.Table)) (div, counters int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("stacktgt")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kstacktgtErtaiDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kstacktgtErtaiDecks[0], kstacktgtErtaiDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kstacktgt-a", "kstacktgt-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, ertaiPolicy(seed, &counters), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div, counters
			}
			lo := max(d.At-12, 0)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail,
				actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
		}
	}
	return div, counters
}

func TestStacktgtErtaiLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeRememberForCounterTime) {
		t.Skip("gorge pin predates 84b5a37bd: Counter RememberForCounter$ unread, Ertai's draw goes to no one")
	}
	lockstepSet(t, "stacktgt", kstacktgtErtaiDecks, 500, "KSTACKTGT_SEEDS")
	if _, n := ertaiRun(t, 300, true, nil); n == 0 {
		t.Fatal("ertaiPolicy never chose Ertai's counter mode")
	} else {
		t.Logf("Ertai's counter mode chosen %d times, all in lockstep", n)
	}
}

// TestStacktgtErtaiLockstepMutation proves the pool resolves Ertai's
// counter mode: with its draw given to Ertai's controller instead of the
// countered thing's, some seed diverges.
func TestStacktgtErtaiLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeRememberForCounterTime) {
		t.Skip("gorge pin predates 84b5a37bd")
	}
	n, _ := ertaiRun(t, 300, false, func(tbl *kop.Table) {
		for i := range tbl.Cards {
			for j := range tbl.Cards[i].Abilities {
				a := &tbl.Cards[i].Abilities[j]
				for k := range a.Prog {
					if in := &a.Prog[k]; in.Op == kop.OpDraw && in.Obj == kop.RefTargetController {
						in.Obj = kop.RefController
					}
				}
			}
		}
	})
	if n == 0 {
		t.Fatal("no seed diverged with Ertai's counter-mode draw redirected: the pool never counters with Ertai")
	}
}
