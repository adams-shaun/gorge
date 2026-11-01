package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// gorge made TargetsValid$ any-match and IsSingleTarget$
// a distinct count, both over the WHOLE target chain (Targets + SubTargets).
// The pool pairs the lowered TargetsValid$ carriers (Gnarlback Rhino, Ms.
// Marvel, Scolding Administrator, Tiller of Flesh) and Leyline of Resonance
// with chained and two-target spells (Prey Upon: pump + fight link; Arc Trail;
// Twin Bolt) and single-target burn and pumps.

var targetsValidDecks = [][2][]cardset.Entry{
	{ // M1: heroic-style carriers vs a plain deck
		{
			{Name: "Gnarlback Rhino", Count: 4}, {Name: "Ms. Marvel, Kamala Khan", Count: 4},
			{Name: "Scolding Administrator", Count: 4}, {Name: "Tiller of Flesh", Count: 4},
			{Name: "Prey Upon", Count: 4}, {Name: "Giant Growth", Count: 3}, {Name: "Grizzly Bears", Count: 3},
			{Name: "Forest", Count: 6}, {Name: "Plains", Count: 2}, {Name: "Island", Count: 2}, {Name: "Swamp", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4},
			{Name: "Child of Night", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
			{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 4},
		},
	},
	{ // M2: Leyline and the two-target burn
		{
			{Name: "Leyline of Resonance", Count: 4}, {Name: "Arc Trail", Count: 4}, {Name: "Twin Bolt", Count: 4},
			{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Giant Growth", Count: 2},
			{Name: "Raging Goblin", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Forest", Count: 4},
		},
		{
			{Name: "Gnarlback Rhino", Count: 4}, {Name: "Scolding Administrator", Count: 4}, {Name: "Prey Upon", Count: 4},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Shock", Count: 2},
			{Name: "Forest", Count: 8}, {Name: "Plains", Count: 4}, {Name: "Swamp", Count: 4},
		},
	},
}

type targetsValidStats struct {
	chainCasts int // a chained spell (a SubAbility$ link's target) went on the stack
	chainTrig  int // a TargetsValid$ carrier triggered on such a cast
	carrierTrg int // a TargetsValid$ carrier triggered at all
	leyCopies  int // Leyline copied an instant or sorcery
	preyCasts  int
}

type targetsValidTally struct {
	evAt    int
	chained map[state.ObjID]bool // stack objects that declared a SubAbility$ link target
	pending bool                 // a chained spell is on the stack and unresolved
}

func (p *targetsValidTally) scan(ref *GorgeSide, st *targetsValidStats) {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt, p.chained, p.pending = 0, nil, false
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TargetsChosen:
			if e.Text == events.SubTargetNotice {
				if p.chained == nil {
					p.chained = make(map[state.ObjID]bool)
				}
				p.chained[e.Obj] = true
				p.pending = true
			}
		case events.PutOnStack:
			if chName(g, e.Obj) == "Prey Upon" {
				st.preyCasts++
			}
		case events.StackCopy:
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && (o.Face().IsInstant() || o.Face().IsSorcery()) {
				st.leyCopies++
			}
		case events.TriggerPush:
			switch chName(g, e.Obj) {
			case "Gnarlback Rhino", "Ms. Marvel, Kamala Khan", "Scolding Administrator", "Tiller of Flesh":
				st.carrierTrg++
				if p.pending {
					st.chainTrig++
				}
			}
		case events.Resolve:
			if p.chained[e.Obj] {
				st.chainCasts++
				p.pending = false
			}
		case events.TurnChange:
			p.pending, p.chained = false, nil
		}
	}
	p.evAt = len(evs)
}

// targetsValidPolicy leans toward casting and toward targeting the controller's
// own creatures (so the carriers' own-permanent filters match).
func targetsValidPolicy(seed uint64, st *targetsValidStats) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x6b666f6c6c6f)) // "kfollo"
	var buf []Canon
	var tl targetsValidTally
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			tl.scan(c.G, st)
		}
		switch c.D.Kind {
		case DPriority:
			if c.G != nil {
				buf = buf[:0]
				for _, a := range c.D.Options {
					if a.Kind() == ActCast {
						buf = append(buf, a)
					}
				}
				if len(buf) > 0 && prng.IntN(4) != 0 {
					return buf[prng.IntN(len(buf))]
				}
			}
		case DTarget:
			if c.G != nil && prng.IntN(3) != 0 {
				buf = buf[:0]
				for _, a := range c.D.Options {
					o, ok := c.G.GorgeOption(a)
					if !ok || o.Obj == 0 {
						continue
					}
					ob := c.G.E.G.Obj(o.Obj)
					if ob != nil && ob.Face() != nil && ob.Face().IsCreature() && ob.Controller == state.PlayerID(c.D.Player) {
						buf = append(buf, a)
					}
				}
				if len(buf) > 0 {
					return buf[prng.IntN(len(buf))]
				}
			}
		}
		return rnd.Pick(c)
	})
}

func TestTargetsValidOwnPermanentLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("targetsvalid")
	if err != nil {
		t.Fatal(err)
	}
	var st targetsValidStats
	games, divs, stalls := 0, 0, 0
	const seeds = 16 // per seating per matchup: 4 x 16 = 64 games
	for mi, md0 := range targetsValidDecks {
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
			for seed := uint64(1); seed <= seeds; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"deck-a", "deck-b"}, decks, 1)
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
				res := LockstepLed(m, seed, g, k, targetsValidPolicy(seed, &st), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
				CloseSide(k)
				games++
				if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
					stalls++
					t.Errorf("matchup %d swap %d seed %d: kernel stall %q", mi, swap, seed, res.Stall)
				}
				if d := res.Div; d != nil {
					divs++
					t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	t.Logf("%d games, %d divergences, %d stalls", games, divs, stalls)
	t.Logf("%+v", st)
	for _, c := range []struct {
		name string
		n    int
	}{
		{"chained casts", st.chainCasts}, {"carrier triggers", st.carrierTrg},
		{"carrier triggers on a chained cast", st.chainTrig}, {"Leyline copies", st.leyCopies},
	} {
		if c.n < 3 {
			t.Errorf("%s: %d (want >= 3)", c.name, c.n)
		}
	}
}
