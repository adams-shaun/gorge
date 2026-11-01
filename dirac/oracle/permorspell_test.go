package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Origin$ Battlefield,Stack lockstep: Fatehold
// Charm's second mode, Brutal Expulsion's first, Unsubstantiate, Jeskai
// Revelation and Venser, Shaper Savant's enters trigger return a target
// spell or creature / permanent to its owner's hand, Swat Away and Sudden
// Setback put one on its owner's choice of the top or bottom of their
// library (TgtPermOrSpell; no TgtZone$, gorge). Think Twice and
// Firebolt are the flashback spells (a flashback cast leaving the stack is
// exiled), Ornithopter, Jayemdae Tome and Glorious Anthem the noncreature
// permanents, Jayemdae Tome's activation and the Monks' prowess triggers the
// abilities on the stack that are not spells.
var kpermorspellDecks = [2][]cardset.Entry{
	{
		{Name: "Fatehold Charm", Count: 3}, {Name: "Jeskai Revelation", Count: 2}, {Name: "Think Twice", Count: 3},
		{Name: "Swat Away", Count: 2}, {Name: "Sudden Setback", Count: 2},
		{Name: "Savannah Lions", Count: 4}, {Name: "Venser, Shaper Savant", Count: 2}, {Name: "Unsubstantiate", Count: 2},
		{Name: "Glorious Anthem", Count: 2}, {Name: "Island", Count: 9}, {Name: "Plains", Count: 6}, {Name: "Mountain", Count: 3},
	},
	{
		{Name: "Fatehold Charm", Count: 3}, {Name: "Jeskai Revelation", Count: 2}, {Name: "Shock", Count: 3},
		{Name: "Firebolt", Count: 3}, {Name: "Brutal Expulsion", Count: 2}, {Name: "Raging Goblin", Count: 3},
		{Name: "Swat Away", Count: 2}, {Name: "Sudden Setback", Count: 2},
		{Name: "Hill Giant", Count: 2}, {Name: "Ornithopter", Count: 2}, {Name: "Jayemdae Tome", Count: 2},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 2},
	},
}

var kpermorspellCarriers = map[string]bool{
	"Fatehold Charm": true, "Jeskai Revelation": true, "Venser, Shaper Savant": true, "Unsubstantiate": true,
	"Brutal Expulsion": true, "Swat Away": true, "Sudden Setback": true,
}

// kpermorspellName is o's face name ("" for none).
func kpermorspellName(o *state.Object) string {
	if o == nil || o.Face() == nil {
		return ""
	}
	return o.Face().Name
}

// kpermorspellCounts tallies what the pool exercised.
type kpermorspellCounts struct {
	stackTgt, bfTgt int // carrier target choices of a spell / a permanent
	top, bottom     int // owners' library-position answers (Swat Away, Sudden Setback)
}

// kpermorspellPolicy holds the carriers for a response: above an
// opponent's spell it casts one when it can, on an empty stack only now and
// then; a carrier's target is a spell half the time one is offered and
// Fatehold Charm takes its return mode. Every other decision is
// castFirstPolicy's.
func kpermorspellPolicy(seed uint64, n *kpermorspellCounts) Policy {
	base := castFirstPolicy(seed)
	x := seed*0x9e3779b97f4a7c15 | 1
	coin := func() bool {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
		return x&1 == 0
	}
	var buf, stk []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		if g == nil || g.fr == nil {
			return base.Pick(c)
		}
		G := g.E.G
		d := g.fr.d
		srcCarrier := false
		if d != nil {
			if kpermorspellCarriers[kpermorspellName(G.Obj(d.Source))] {
				srcCarrier = true
			}
		}
		respond := len(G.Stack) > 0
		if respond {
			if o := G.Obj(G.Stack[len(G.Stack)-1]); o == nil || o.Controller == state.PlayerID(c.D.Player) {
				respond = false
			}
		}
		buf, stk = buf[:0], stk[:0]
		for _, a := range c.D.Options {
			opt, ok := g.GorgeOption(a)
			if !ok {
				buf = append(buf, a)
				continue
			}
			if opt.Kind == "mode" && srcCarrier && strings.HasPrefix(opt.Label, "Return") {
				return a
			}
			if a.Kind() == ActCast {
				if kpermorspellCarriers[kpermorspellName(G.Obj(opt.Obj))] {
					if respond {
						return a
					}
					if coin() && coin() {
						buf = append(buf, a)
					}
					continue
				}
			}
			if srcCarrier && opt.Obj != 0 {
				if o := G.Obj(opt.Obj); o != nil && o.Zone == state.ZStack {
					stk = append(stk, a)
				}
			}
			buf = append(buf, a)
		}
		pick := Canon(0)
		switch {
		case len(stk) > 0 && coin():
			dd := *c.D
			dd.Options = stk
			pick = base.Pick(&PickCtx{D: &dd, G: c.G, Seq: c.Seq})
		case len(buf) == 0 || len(buf) == len(c.D.Options):
			pick = base.Pick(c)
		default:
			dd := *c.D
			dd.Options = buf
			pick = base.Pick(&PickCtx{D: &dd, G: c.G, Seq: c.Seq})
		}
		if d != nil && d.ResumeKind == "changezone_alternative" {
			if opt, ok := g.GorgeOption(pick); ok {
				switch opt.Kind {
				case "primary":
					n.top++
				case "bottom":
					n.bottom++
				}
			}
		}
		if srcCarrier && d != nil && d.ResumeKind == "" {
			if opt, ok := g.GorgeOption(pick); ok && opt.Obj != 0 {
				if o := G.Obj(opt.Obj); o != nil && opt.Kind != "mode" {
					switch o.Zone {
					case state.ZStack:
						n.stackTgt++
					case state.ZBattlefield:
						n.bfTgt++
					}
				}
			}
		}
		return pick
	})
}

// kpermorspellRun plays seeds games under kpermorspellPolicy and returns
// the divergent games, the kernel stalls and the tallies; report fails on
// each divergence, else the first stops the run.
func kpermorspellRun(t *testing.T, seeds uint64, report bool) (div, stalls int, n kpermorspellCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("permorspell")
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range kpermorspellDecks {
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
	tbl, _, _, err := set.Matchup(kpermorspellDecks[0], kpermorspellDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchup(reg, [2]string{"kpermorspell-a", "kpermorspell-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kpermorspellPolicy(seed, &n), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				t.Logf("seed %d diverges (decision %d: %s)", seed, d.At, d.What)
				return div, stalls, n
			}
			lo := max(d.At-12, 0)
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nlast %v\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail,
				actionStrings(res.Actions[lo:d.At]), d.RefOptions, d.KerOptions)
		}
	}
	return div, stalls, n
}

func TestPermorspellLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	div, stalls, n := kpermorspellRun(t, 256, true)
	t.Logf("permorspell: 256 seeds, %d divergences, %d stalls; carrier targets: %d spells, %d permanents; library: %d top, %d bottom",
		div, stalls, n.stackTgt, n.bfTgt, n.top, n.bottom)
	if n.stackTgt < 30 || n.bfTgt < 30 || n.top < 10 || n.bottom < 10 {
		t.Fatalf("pool under-exercised: %+v", n)
	}
}

// TestPermorspellMutation proves the pool offers a carrier while it is on
// the stack with its own target slot open: with the source spell admitted as
// its own target (CR 115.5 ignored), some seed diverges.
func TestPermorspellMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPermOrSpellSelf = true
	defer func() { dirac.DebugPermOrSpellSelf = false }()
	if div, _, _ := kpermorspellRun(t, 256, false); div == 0 {
		t.Fatal("no seed diverged with the source admitted as its own target")
	}
}

// TestPermorspellNoExileMutation proves the pool returns a flashback spell
// from the stack: moved to its owner's hand without the flashback exile
// (gorge keywordExileDeparture, CR 702.34a), some seed diverges.
func TestPermorspellNoExileMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPermOrSpellNoExile = true
	defer func() { dirac.DebugPermOrSpellNoExile = false }()
	if div, _, _ := kpermorspellRun(t, 256, false); div == 0 {
		t.Fatal("no seed diverged with the flashback exile skipped: no flashback spell was returned from the stack")
	}
}

// TestPermorspellTopBottomNoExileMutation proves the pool puts a flashback
// spell from the stack on its owner's top or bottom (Swat Away, Sudden
// Setback): moved to the library without the flashback exile (gorge
// keywordExileDeparture, CR 702.34a), some seed diverges.
func TestPermorspellTopBottomNoExileMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPermOrSpellTopBottomNoExile = true
	defer func() { dirac.DebugPermOrSpellTopBottomNoExile = false }()
	if div, _, _ := kpermorspellRun(t, 256, false); div == 0 {
		t.Fatal("no seed diverged with the library move's flashback exile skipped: no flashback spell was put on a library from the stack")
	}
}
