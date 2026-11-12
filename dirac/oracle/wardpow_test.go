package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The pay-life-equal-to-power ward lockstep:
// Raubahn, Bull of Ala Mhigo's Ward—Pay life equal to Raubahn's power
// (gorge kw_ward.go beginWardPayment: the power when the ward is paid). B
// targets Raubahn with Shock or Unsummon whenever it can; A casts Giant
// Growth on Raubahn while its ward trigger is on the stack, so the life paid
// is the pumped power; B pays or declines by seed and decision parity. The
// expected answer at every decision is gorge's.
var kwardpowDecks = [2][]cardset.Entry{
	{
		{Name: "Raubahn, Bull of Ala Mhigo", Count: 4}, {Name: "Giant Growth", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Raging Goblin", Count: 4}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Shock", Count: 8}, {Name: "Unsummon", Count: 8}, {Name: "Storm Crow", Count: 6},
		{Name: "Island", Count: 9}, {Name: "Mountain", Count: 9},
	},
}

const kwardpowName = "Raubahn, Bull of Ala Mhigo"

// kwardpowTally counts, from gorge's state, the games with a paid ward
// (RW+), a paid ward after a Giant Growth (RW+G: 5 life) and a declined one
// (RW-: the warded spell countered).
type kwardpowTally struct {
	paid, paidGrowth, declined, games int
}

// kwardpowWard is Raubahn's lowest ward trigger on gorge's stack (0: none)
// and the stack object below it (the warded spell).
func kwardpowWard(g *state.Game) (ward, below state.ObjID) {
	for i, sid := range g.Stack {
		ob := g.Obj(sid)
		if ob == nil || ob.Ability == nil || ob.Ability.API != "Ward" || chName(g, ob.Source) != kwardpowName {
			continue
		}
		if i > 0 {
			below = g.Stack[i-1]
		}
		return sid, below
	}
	return 0, 0
}

// kwardpowReady: A holds a Giant Growth; A has an untapped Forest.
func kwardpowReady(g *state.Game) (gg, forest bool) {
	for _, id := range g.Zone(state.ZHand, 0) {
		gg = gg || chName(g, id) == "Giant Growth"
	}
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		if ob := g.Obj(id); ob != nil && !ob.Tapped && chName(g, id) == "Forest" {
			forest = true
		}
	}
	return gg, forest
}

// kwardpowGrown: Raubahn already has a Giant Growth this turn (one pump per
// ward, so the pumped payment is 5).
func kwardpowGrown(gs *GorgeSide) bool {
	g := gs.E.G
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		if chName(g, id) == kwardpowName {
			if gs.E.Power(id) != 2 {
				return true
			}
		}
	}
	return false
}

// kwardpowSpare: A may cast a creature and still keep a Forest up for a
// Giant Growth (it holds none, or it has two untapped Forests and four
// untapped lands).
func kwardpowSpare(g *state.Game) bool {
	gg := false
	for _, id := range g.Zone(state.ZHand, 0) {
		gg = gg || chName(g, id) == "Giant Growth"
	}
	if !gg {
		return true
	}
	forests, lands := 0, 0
	for _, id := range g.Zone(state.ZBattlefield, 0) {
		ob := g.Obj(id)
		if ob == nil || ob.Tapped {
			continue
		}
		switch chName(g, id) {
		case "Forest":
			forests++
			lands++
		case "Mountain":
			lands++
		}
	}
	return forests >= 2 && lands >= 4
}

func kwardpowPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	pick := func(c *PickCtx, opts []Canon) Canon {
		d := *c.D
		d.Options = opts
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	}
	// named: the options whose gorge object is named one of names
	named := func(c *PickCtx, kind CanonKind, names ...string) []Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if kind != 255 && a.Kind() != kind {
				continue
			}
			o, ok := c.G.GorgeOption(a)
			if !ok {
				continue
			}
			n := chName(c.G.E.G, o.Obj)
			for _, x := range names {
				if n == x {
					buf = append(buf, a)
					break
				}
			}
		}
		return buf
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		g := c.G.E.G
		ward, _ := kwardpowWard(g)
		raubahn := false
		for _, id := range g.Zone(state.ZBattlefield, 0) {
			raubahn = raubahn || chName(g, id) == kwardpowName
		}
		// B fires at Raubahn when A can answer with a Giant Growth (or
		// holds none), so the pumped payment (RW+G) is reached as well as
		// the plain ones
		gg, forest := kwardpowReady(g)
		bReady := forest || !gg
		switch {
		case c.D.Kind == DTarget:
			if o := named(c, 255, kwardpowName); len(o) > 0 {
				return o[0]
			}
		case c.D.Kind == DPriority && ward != 0 && c.D.Player == 0 && g.Stack[len(g.Stack)-1] == ward && !kwardpowGrown(c.G):
			if o := named(c, ActCast, "Giant Growth"); len(o) > 0 {
				return pick(c, o)
			}
		case c.D.Kind == DPriority && c.D.Player == 1 && ward == 0 && raubahn && bReady:
			if o := named(c, ActCast, "Shock", "Unsummon"); len(o) > 0 {
				return pick(c, o)
			}
		case c.D.Kind == DPriority && c.D.Player == 1 && ward != 0:
			for _, a := range c.D.Options {
				if a.Kind() == ActPass {
					return a // B lets the ward resolve
				}
			}
		case c.D.Kind != DPriority && c.D.Player == 1 && ward != 0 && len(c.D.Options) == 2:
			return c.D.Options[(seed+uint64(c.Seq))%2] // the ward's pay / decline
		}
		if c.D.Kind != DPriority {
			return rnd.Pick(c)
		}
		// held: A keeps Giant Growth (and a Forest) for the ward window; B
		// keeps its Shock and Unsummon while A could not answer them
		var keep, casts []Canon
		for _, a := range c.D.Options {
			k := a.Kind()
			if o, ok := c.G.GorgeOption(a); ok && k == ActCast {
				n := chName(g, o.Obj)
				if c.D.Player == 0 && (n == "Giant Growth" || !kwardpowSpare(g)) {
					continue
				}
				if c.D.Player == 1 && raubahn && !bReady && (n == "Shock" || n == "Unsummon") {
					continue
				}
			}
			keep = append(keep, a)
			if k == ActCast || k == ActPlayLand {
				casts = append(casts, a)
			}
		}
		if len(casts) > 0 && seed%4 != 0 {
			return pick(c, casts)
		}
		return pick(c, keep)
	})
}

// kwardpowGame tracks one game's ward resolutions on gorge's side.
type kwardpowGame struct {
	ward, below       state.ObjID
	life              int
	paid, grown, decl bool
}

func (w *kwardpowGame) observe(g *state.Game) {
	ward, below := kwardpowWard(g)
	if w.ward != 0 && ward != w.ward {
		lost := w.life - int(g.Players[1].Life)
		onStack := false
		for _, sid := range g.Stack {
			onStack = onStack || sid == w.below
		}
		switch {
		case lost > 0 && onStack:
			w.paid = true
			if lost == 5 {
				w.grown = true
			}
		case lost == 0 && !onStack:
			w.decl = true
		}
		w.ward = 0
	}
	if ward != 0 && w.ward == 0 {
		w.ward, w.below, w.life = ward, below, int(g.Players[1].Life)
	}
}

func TestWardpowLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("wardpow")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kwardpowDecks {
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
	tbl, _, _, err := set.Matchup(kwardpowDecks[0], kwardpowDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	var tal kwardpowTally
	for seed := uint64(1); seed <= 300; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"wp-a", "wp-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gm kwardpowGame
		check := func(ref *GorgeSide, _ Side) *Divergence {
			gm.observe(ref.E.G)
			return nil
		}
		res := Lockstep(m, seed, k, kwardpowPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		tal.games++
		if gm.paid {
			tal.paid++
		}
		if gm.grown {
			tal.paidGrowth++
		}
		if gm.decl {
			tal.declined++
		}
		if res.Stall != "" && res.Stall != "turns" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: ward paid (RW+) %d, paid after Giant Growth (RW+G) %d, declined (RW-) %d", tal.games, tal.paid, tal.paidGrowth, tal.declined)
	if tal.paid < 10 || tal.paidGrowth < 10 || tal.declined < 10 {
		t.Errorf("ward games: RW+ %d, RW+G %d, RW- %d; want at least 10 each", tal.paid, tal.paidGrowth, tal.declined)
	}
}
