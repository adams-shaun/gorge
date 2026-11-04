package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Plot lockstep (CR 702.170): constructed matchups over the generated
// plot pool. The decks carry every arm:
//
//   - a sorcery-speed plot special action (canonical cast mode "plot"),
//     including actions the pool cannot pay before the pick (lowered plans);
//   - the free cast from exile on a LATER turn (canonical face mode
//     "plot_cast"), and no offer on the plotting turn itself;
//   - no plot action and no plotted cast at instant speed;
//   - no plotted cast for a plot card exiled from a graveyard (Tormod's
//     Crypt).
//
// The counters come from gorge's event log and option lists, so the test
// fails if the kernel stops matching any arm. The mutations clear HasPlot or
// relax one timing rule and must diverge, proving the counters are not
// vacuous.
var plotDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "Cunning Coyote", Count: 4}, {Name: "Slickshot Show-Off", Count: 2},
			{Name: "Irascible Wolverine", Count: 2}, {Name: "Stingerback Terror", Count: 2},
			{Name: "Brimstone Roundup", Count: 2}, {Name: "Demonic Ruckus", Count: 2},
			{Name: "Beastbond Outcaster", Count: 2}, {Name: "Rise of the Varmints", Count: 2},
			{Name: "Tormod's Crypt", Count: 2}, {Name: "Railway Brawler", Count: 2},
			{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
		},
		{
			{Name: "Djinn of Fool's Fall", Count: 2}, {Name: "Loan Shark", Count: 2},
			{Name: "Plan the Heist", Count: 2}, {Name: "Slickshot Lockpicker", Count: 2},
			{Name: "Blacksnag Buzzard", Count: 2}, {Name: "Rictus Robber", Count: 2},
			{Name: "Unscrupulous Contractor", Count: 2}, {Name: "Pitiless Carnage", Count: 2},
			{Name: "Sheriff of Safe Passage", Count: 2}, {Name: "Stagecoach Security", Count: 2},
			{Name: "Island", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Plains", Count: 6},
		},
	},
}

// plotNames is the set of plot cards the decks use.
var plotNames = map[string]bool{
	"Cunning Coyote": true, "Slickshot Show-Off": true, "Irascible Wolverine": true,
	"Stingerback Terror": true, "Brimstone Roundup": true, "Demonic Ruckus": true,
	"Beastbond Outcaster": true, "Rise of the Varmints": true, "Railway Brawler": true,
	"Djinn of Fool's Fall": true, "Loan Shark": true, "Plan the Heist": true,
	"Slickshot Lockpicker": true, "Blacksnag Buzzard": true, "Rictus Robber": true,
	"Unscrupulous Contractor": true, "Pitiless Carnage": true,
	"Sheriff of Safe Passage": true, "Stagecoach Security": true,
}

// plotTally counts the arms. The event cursor mirrors warp_test's: the log
// only grows, but resets when a side is rebuilt (a reset rewinds the cursor
// and forgets the per-object maps, whose ObjIDs are then stale).
type plotTally struct {
	evAt     int
	plotTurn map[state.ObjID]int32
	nonPlot  map[state.ObjID]bool // plot cards exiled from a graveyard
	gyPlot   int                  // frames with a plot card in a graveyard
	actOffer int                  // Tormod's Crypt activation options seen
	plots    int                  // "plotted" moves
	lowered  int                  // plot actions taken when the pool could not pay any plot before the pick
	lastLow  bool                 // the previous priority frame offered plots only as lowered plays
	casts    int                  // later-turn PutOnStack of a plotted card
	sameTurn int                  // sorcery frames with an earlier-turn plotted card offered (must be 0)
	sameWith int                  // sorcery frames withholding a same-turn plotted cast
	instantP int                  // instant frames offering a plot action (must be 0)
	instantW int                  // instant frames withholding a hand plot card
	instantC int                  // instant frames offering a plotted cast (must be 0)
	instCW   int                  // instant frames withholding an earlier-turn plotted cast
	nonEx    int                  // non-plot exiles of a plot card
	nonOff   int                  // plot_cast offers naming a non-plot exile (must be 0)
	maxMay   int
}

func (p *plotTally) check(ref *GorgeSide, ker Side) *Divergence {
	return p.checkInner(ref, ker)
}

func (p *plotTally) checkInner(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
		p.plotTurn = map[state.ObjID]int32{}
		p.nonPlot = map[state.ObjID]bool{}
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.MoveZone:
			if e.To != state.ZExile {
				continue
			}
			o := g.Obj(e.Obj)
			if o == nil || o.Face() == nil || !plotNames[o.Face().Name] {
				continue
			}
			switch {
			case e.From == state.ZHand && e.Text == "plotted":
				p.plots++
				if p.lastLow {
					p.lowered++ // taken from a frame whose every plot needed lowering
				}
				delete(p.nonPlot, e.Obj)
				p.plotTurn[e.Obj] = g.Turn
			case e.From == state.ZGraveyard:
				p.nonEx++
				p.nonPlot[e.Obj] = true
			}
		case events.PutOnStack:
			if t, ok := p.plotTurn[e.Obj]; ok && g.Turn > t {
				p.casts++
			}
			delete(p.plotTurn, e.Obj) // the push ends the designation (CR 702.170d)
		}
	}
	p.evAt = len(evs)
	p.lastLow = false
	fr := ref.fr
	if fr == nil || fr.d == nil || fr.d.Kind != "priority" {
		return nil
	}
	actor := state.PlayerID(fr.d.Player)
	sorc := g.Active == actor && g.Step.IsMain() && len(g.Stack) == 0
	// gorge's plot plays: a hand-walk cast option (Mode "plot") and the
	// potential play the planner lowers to the same canonical option.
	plotPlay, realPlot, lowPlot := false, false, false
	for _, o := range fr.d.Options {
		if o.Kind == "cast" && o.Mode == "plot" {
			plotPlay, realPlot = true, true
		}
	}
	for _, pe := range fr.prio {
		a := pe.a
		if a.Kind() == ActCast && a.Mode()&0x0f == 7 {
			plotPlay = true
			if pe.kind == prLower || pe.kind == prScript {
				lowPlot = true
			}
		}
	}
	p.lastLow = lowPlot && !realPlot
	castOffer := func(id state.ObjID) bool {
		for _, o := range fr.d.Options {
			if o.Kind == "cast" && o.Mode == "plot_cast" && o.Obj == id {
				return true
			}
		}
		return false
	}
	if plotInGraveyard(g) {
		p.gyPlot++
	}
	for _, o := range fr.d.Options {
		if o.Kind == "ability" {
			p.actOffer++
		}
	}
	holdPlot := false
	for _, id := range g.Zone(state.ZHand, actor) {
		o := g.Obj(id)
		if o != nil && o.Face() != nil && plotNames[o.Face().Name] {
			holdPlot = true
		}
	}
	if !sorc {
		if holdPlot {
			if plotPlay {
				p.instantP++
			} else {
				p.instantW++
			}
		}
		for _, id := range g.Zone(state.ZExile, actor) {
			t, ok := p.plotTurn[id]
			if !ok || g.Turn <= t {
				continue
			}
			if castOffer(id) {
				p.instantC++
			} else {
				p.instCW++
			}
		}
	}
	if sorc {
		for _, id := range g.Zone(state.ZExile, actor) {
			t, ok := p.plotTurn[id]
			if !ok {
				continue
			}
			if t == g.Turn {
				if castOffer(id) {
					p.sameTurn++
				} else {
					p.sameWith++
				}
			}
		}
	}
	for _, o := range fr.d.Options {
		if o.Kind == "cast" && o.Mode == "plot_cast" && p.nonPlot[o.Obj] {
			p.nonOff++
		}
	}
	return nil
}

// plotPolicy prefers the plotted free cast, then the plot action, then a
// Tormod's Crypt activation while a plot card is in a graveyard (so the
// non-plot exile arm is reached), else plays or casts.
func plotPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		var plotAct, plotCast, activate Canon
		var haveAct, haveCast, haveActivate bool
		for _, a := range c.D.Options {
			switch {
			case a.Kind() == ActCast && a.Mode()&0x0f == 7:
				plotAct, haveAct = a, true
			case a.Kind() == ActCastFace && a.Mode()&0x0f == 11:
				plotCast, haveCast = a, true
			case a.Kind() == ActActivate:
				activate, haveActivate = a, true
			}
		}
		if haveCast {
			return plotCast
		}
		if haveAct {
			return plotAct
		}
		if haveActivate && c.G != nil && plotInGraveyard(c.G.E.G) {
			return activate
		}
		if c.G != nil && c.G.fr != nil {
			for i, o := range c.G.fr.d.Options {
				if o.Obj == 0 || o.Obj > state.ObjID(len(c.G.E.G.Objs)) {
					continue
				}
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && plotNames[ob.Face().Name] && ob.Zone == state.ZGraveyard {
					a := c.G.fr.acts[i]
					for _, x := range c.D.Options {
						if x == a {
							return a
						}
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
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

func plotInGraveyard(g *state.Game) bool {
	for _, p := range []state.PlayerID{0, 1} {
		for _, id := range g.Zone(state.ZGraveyard, p) {
			o := g.Obj(id)
			if o != nil && o.Face() != nil && plotNames[o.Face().Name] {
				return true
			}
		}
	}
	return false
}

// plotIgnored is mutation (a): the kernel behaves as if no card had Plot.
func plotIgnored(t *kop.Table) *kop.Table {
	m := *t
	m.Cards = append([]kop.CardDef(nil), t.Cards...)
	for i := range m.Cards {
		m.Cards[i].HasPlot = false
	}
	return &m
}

func plotRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, check func(*GorgeSide, Side) *Divergence) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"plot-a", "plot-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := plotPolicy(seed)
	if seed%4 == 0 {
		pol = RandomPolicy(seed)
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: check})
	CloseSide(k)
	return res
}

func plotSetup(t *testing.T) (*cards.Registry, cardset.Set, [][2][]*cards.Card, []*kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("plot")
	if err != nil {
		t.Fatal(err)
	}
	decks := make([][2][]*cards.Card, len(plotDecks))
	tbls := make([]*kop.Table, len(plotDecks))
	for mi, md := range plotDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[mi][s] = append(decks[mi][s], c)
				}
			}
			if len(decks[mi][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[mi][s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		tbls[mi] = tbl
	}
	return reg, set, decks, tbls
}

func TestPlotLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, set, decks, tbls := plotSetup(t)
	var p plotTally
	p.plotTurn = map[state.ObjID]int32{}
	p.nonPlot = map[state.ObjID]bool{}
	prevHook := OccupancyHook
	OccupancyHook = func(s *dirac.State) {
		if n := s.FeatMayCount(); n > p.maxMay {
			p.maxMay = n
		}
	}
	defer func() { OccupancyHook = prevHook }()
	games := 0
	for mi := range plotDecks {
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			res := plotRun(t, reg, set, decks[mi], tbls[mi], seed, cfg, p.check)
			games++
			if res.Stall != "" {
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games: %d plots (%d lowered), %d free casts, %d same-turn withheld (%d offered), %d instant plot withheld (%d offered), %d instant cast withheld (%d offered), %d non-plot exiles (%d offered), %d gy-plot frames, %d activation offers, max nmay %d",
		games, p.plots, p.lowered, p.casts, p.sameWith, p.sameTurn, p.instantW, p.instantP, p.instCW, p.instantC, p.nonEx, p.nonOff, p.gyPlot, p.actOffer, p.maxMay)
	if p.plots < 10 {
		t.Errorf("%d plot actions, want at least 10", p.plots)
	}
	if p.lowered < 10 {
		t.Errorf("%d lowered plot actions, want at least 10", p.lowered)
	}
	if p.casts < 10 {
		t.Errorf("%d plotted free casts, want at least 10", p.casts)
	}
	if p.sameWith < 10 {
		t.Errorf("%d same-turn withheld frames, want at least 10", p.sameWith)
	}
	if p.sameTurn != 0 {
		t.Errorf("%d same-turn plot_cast offers, want 0", p.sameTurn)
	}
	if p.instantW < 10 {
		t.Errorf("%d instant-speed plot withheld frames, want at least 10", p.instantW)
	}
	if p.instantP != 0 {
		t.Errorf("%d instant-speed plot offers, want 0", p.instantP)
	}
	if p.instCW < 10 {
		t.Errorf("%d instant-speed cast withheld frames, want at least 10", p.instCW)
	}
	if p.instantC != 0 {
		t.Errorf("%d instant-speed plot_cast offers, want 0", p.instantC)
	}
	if p.nonEx < 10 {
		t.Errorf("%d non-plot exiles, want at least 10", p.nonEx)
	}
	if p.nonOff != 0 {
		t.Errorf("%d plot_cast offers named a non-plot exile, want 0", p.nonOff)
	}
}

// TestPlotMutation: each mutation must diverge on some seed.
func TestPlotMutation(t *testing.T) {
	reg, set, decks, tbls := plotSetup(t)
	type mut struct {
		name string
		tbl  *kop.Table
		set  func()
	}
	muts := []mut{
		{name: "plot ignored", tbl: plotIgnored(tbls[0])},
		{name: "same-turn cast", tbl: tbls[0], set: func() { dirac.DebugPlotSameTurn = true }},
		{name: "instant plot", tbl: tbls[0], set: func() { dirac.DebugPlotInstant = true }},
	}
	for _, mu := range muts {
		if mu.set != nil {
			mu.set()
		}
		diverged := false
		for seed := uint64(1); seed <= 48 && !diverged; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			res := plotRun(t, reg, set, decks[0], mu.tbl, seed, cfg, nil)
			if diverged = res.Div != nil; diverged {
				t.Logf("mutation %q: seed %d diverged at decision %d: %s %s", mu.name, seed, res.Div.At, res.Div.What, res.Div.Field)
			}
		}
		dirac.DebugPlotSameTurn = false
		dirac.DebugPlotInstant = false
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}
