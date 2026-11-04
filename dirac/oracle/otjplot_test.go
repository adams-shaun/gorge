package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The effect-granted plot lockstep (CR 702.170b-d): Kellan
// Joins Up's "you may exile a nonland card with mana value 3 or less from
// your hand. If you do, it becomes plotted" (gorge effAlterAttribute
// Plotted over Defined$ Remembered, gated on exile). The plotted card --
// a plot card (Aloe Alchemist) or not (Grizzly Bears, Llanowar Elves,
// Opt) -- may be cast from exile without paying its mana cost on a later
// turn at sorcery speed; Aloe Alchemist's own BecomesPlotted trigger
// fires. The counters come from gorge's events and option lists.
var kotjplotDecks = [2][]cardset.Entry{
	{
		{Name: "Kellan Joins Up", Count: 6}, {Name: "Aloe Alchemist", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Opt", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 5}, {Name: "Plains", Count: 5},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 8},
	},
}

// kotjTally counts the arms from gorge's log (the cursor resets with a
// rebuilt side, as plotTally's).
type kotjTally struct {
	evAt     int
	lastMove map[state.ObjID]string // the text of the object's last move to exile
	kellan   map[state.ObjID]int32  // Kellan-plotted cards still exiled: the turn plotted
	aloe     map[state.ObjID]bool   // Kellan-plotted Aloe Alchemists
	granted  int                    // Kellan's Plotted grants
	casts    int                    // Kellan-plotted cards cast on a later turn
	sameWith int                    // sorcery frames on the plotting turn withholding the card
	sameTurn int                    // sorcery frames on the plotting turn offering it (must be 0)
	aloeRes  int                    // Kellan-plotted Aloe Alchemists' triggers resolved
	maxMay   int
}

func (p *kotjTally) reset() {
	p.evAt = 0
	p.lastMove = map[state.ObjID]string{}
	p.kellan = map[state.ObjID]int32{}
	p.aloe = map[state.ObjID]bool{}
}

func (p *kotjTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.reset()
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.MoveZone:
			if e.To == state.ZExile {
				p.lastMove[e.Obj] = e.Text
			}
		case events.AlterAttribute:
			// the plot action's own grant follows a "plotted" move
			if e.Text != "Plotted" || e.Amount < 1 || p.lastMove[e.Obj] == "plotted" {
				continue
			}
			p.granted++
			p.kellan[e.Obj] = g.Turn
			if o := g.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Aloe Alchemist" {
				p.aloe[e.Obj] = true
			}
		case events.PutOnStack:
			if t, ok := p.kellan[e.Obj]; ok && g.Turn > t {
				p.casts++
			}
			delete(p.kellan, e.Obj)
		case events.Resolve:
			if o := g.Obj(e.Obj); o != nil && o.Ability != nil && p.aloe[o.Source] {
				p.aloeRes++
				delete(p.aloe, o.Source)
			}
		}
	}
	p.evAt = len(evs)
	fr := ref.fr
	if fr == nil || fr.d == nil || fr.d.Kind != "priority" {
		return nil
	}
	actor := state.PlayerID(fr.d.Player)
	if g.Active != actor || !g.Step.IsMain() || len(g.Stack) != 0 {
		return nil
	}
	for _, id := range g.Zone(state.ZExile, actor) {
		if t, ok := p.kellan[id]; !ok || t != g.Turn {
			continue
		}
		offered := false
		for _, o := range fr.d.Options {
			if o.Kind == "cast" && o.Mode == "plot_cast" && o.Obj == id {
				offered = true
			}
		}
		if offered {
			p.sameTurn++
		} else {
			p.sameWith++
		}
	}
	return nil
}

var (
	kotjDone = ActChoose + kotjCode("done")
	kotjNo   = ActChoose + kotjCode("no")
)

func kotjCode(kind string) CanonKind {
	c, _ := chooseCode(kind)
	return CanonKind(c)
}

// kotjPolicy casts a plotted card when offered, then Kellan Joins Up, else
// a random play; at a choice it takes the optional exile (never Done or No
// while another answer stands).
func kotjPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && a.Mode()&0x0f == 11 {
				return a
			}
		}
		fr := (*frame)(nil)
		if c.G != nil {
			fr = c.G.fr
		}
		if fr != nil && fr.d != nil && fr.d.Kind == "priority" {
			g := c.G.E.G
			for _, pe := range fr.prio {
				if pe.a.Kind() != ActCast || pe.opt < 0 || pe.opt >= len(fr.d.Options) {
					continue
				}
				o := g.Obj(fr.d.Options[pe.opt].Obj)
				if o == nil || o.Face() == nil || o.Face().Name != "Kellan Joins Up" {
					continue
				}
				for _, x := range c.D.Options {
					if x == pe.a {
						return x
					}
				}
			}
		}
		chooses := false
		for _, a := range c.D.Options {
			if a.Kind() >= ActChoose || a.Kind() == ActYes {
				chooses = true
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			k := a.Kind()
			switch {
			case chooses:
				if k == ActDone || k == ActNo || k == kotjDone || k == kotjNo {
					continue
				}
				buf = append(buf, a)
			case k == ActCast || k == ActActivate || k == ActPlayLand:
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestOtjPlotGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks, tbl := kotjSetup(t)
	var p kotjTally
	p.reset()
	prevHook := OccupancyHook
	OccupancyHook = func(s *dirac.State) {
		if n := s.FeatMayCount(); n > p.maxMay {
			p.maxMay = n
		}
	}
	defer func() { OccupancyHook = prevHook }()
	games := 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kotj-a", "kotj-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kotjPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: p.check})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d Kellan grants, %d later-turn casts, %d same-turn withheld (%d offered), %d Aloe triggers resolved, max nmay %d",
		games, p.granted, p.casts, p.sameWith, p.sameTurn, p.aloeRes, p.maxMay)
	if p.casts < 10 {
		t.Errorf("%d Kellan-plotted cards cast on a later turn, want at least 10", p.casts)
	}
	if p.sameWith < 10 {
		t.Errorf("%d same-turn withheld frames, want at least 10", p.sameWith)
	}
	if p.sameTurn != 0 {
		t.Errorf("%d same-turn plot_cast offers of a Kellan-plotted card, want 0", p.sameTurn)
	}
	if p.aloeRes < 10 {
		t.Errorf("%d Kellan-plotted Aloe Alchemist triggers resolved, want at least 10", p.aloeRes)
	}
}

func kotjSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("otjplot")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kotjplotDecks {
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kotjplotDecks[0], kotjplotDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}
