package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// Gowron, Eager for Battle: "Sacrifice another creature: Gowron gains trample
// until end of turn. If you sacrificed a Klingon this way, you draw a card and
// lose 1 life." The gate is Sacrificed$Valid Card.Klingon over the
// sacrifice-time snapshot (gorge sacrificedValidCount): a Klingon, a
// non-Klingon, a changeling, and a Klingon De-evolved into a Salamander (which
// loses every other creature type) answer differently.
var gowronDecks = [2][]cardset.Entry{
	{
		{Name: "Gowron, Eager for Battle", Count: 4}, {Name: "Storied Veteran", Count: 4},
		{Name: "Targ Wrangler", Count: 2}, {Name: "Universal Automaton", Count: 4},
		{Name: "De-evolve", Count: 4}, {Name: "Starfleet Security", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Blood Crypt", Count: 4},
	},
	{
		{Name: "Gowron, Eager for Battle", Count: 4}, {Name: "Storied Veteran", Count: 4},
		{Name: "Targ Wrangler", Count: 2}, {Name: "Universal Automaton", Count: 4},
		{Name: "De-evolve", Count: 4}, {Name: "Starfleet Security", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Blood Crypt", Count: 4},
	},
}

// gowronTally is what the policy sacrificed, by the snapshot answer the gate
// needs. Every sacrifice pick in these decks is Gowron's cost.
type gowronTally struct {
	klingon, nonKlingon, changeling, deEvolved int
}

// Gowron's activated ability's index on its CardDef.
const gowronAbility = 0

// gowronPolicy steers the game to the seed's gate: the low two bits pick the
// sacrifice to look for (Klingon, non-Klingon, changeling, De-evolved
// Klingon); priority spends its setup and fires Gowron only once the wanted
// permanent exists.
type gowronPolicy struct {
	mode  int
	rnd   Policy
	tally *gowronTally
}

func (p *gowronPolicy) Pick(c *PickCtx) Canon {
	if c.G == nil {
		return p.rnd.Pick(c)
	}
	g := c.G
	st := g.E.G
	switch c.D.Kind {
	case DPriority:
		if a, ok := p.priority(g, st, c); ok {
			return a
		}
	case DTarget:
		if a, ok := p.target(g, st, c); ok {
			return a
		}
	case DChoose:
		if a, ok := p.choose(g, st, c); ok {
			return a
		}
	}
	return p.rnd.Pick(c)
}

// gowronName is the gorge card name of an option's object.
func gowronName(g *GorgeSide, a Canon) string {
	o, ok := g.GorgeOption(a)
	if !ok || o.Obj == 0 {
		return ""
	}
	return chName(g.E.G, o.Obj)
}

// isGowronActivation reports whether a is Gowron's activated ability.
func isGowronActivation(g *GorgeSide, a Canon) bool {
	return a.Kind() == ActActivate && a.Mode() == gowronAbility && gowronName(g, a) == "Gowron, Eager for Battle"
}

// gowronCreature is a battlefield creature we can sacrifice.
type gowronCreature struct {
	id   state.ObjID
	typ  []string
	dev  bool // a De-evolve Aura is attached
	chg  bool // a printed/copy changeling whose types still include every type
	king bool // the derived type line includes Klingon
}

// creatures lists p's battlefield creatures with the derived type facts the
// gate and the scenario need.
func creatures(g *GorgeSide, p uint8) []gowronCreature {
	st := g.E.G
	var out []gowronCreature
	for _, id := range st.Zone(state.ZBattlefield, state.PlayerID(p)) {
		o := st.Obj(id)
		if o == nil {
			continue
		}
		ch := g.E.Chars(id)
		creature, king := false, ch.AllCreatureTypes
		for _, w := range ch.Types {
			creature = creature || w == "Creature"
			king = king || w == "Klingon"
		}
		if !creature {
			continue
		}
		c := gowronCreature{
			id: id, typ: ch.Types, king: king,
			chg: ch.AllCreatureTypes && effects.IntrinsicAllCreatureTypes(o),
		}
		for seat := uint8(0); seat < 2; seat++ {
			for _, aid := range st.Zone(state.ZBattlefield, state.PlayerID(seat)) {
				if ao := st.Obj(aid); ao != nil && ao.AttachedTo == id && chName(st, aid) == "De-evolve" {
					c.dev = true
				}
			}
		}
		out = append(out, c)
	}
	return out
}

// wanted reports whether c is the sacrifice this seed steers toward.
func (p *gowronPolicy) wanted(c gowronCreature) bool {
	switch p.mode {
	case 0:
		return c.king && !c.chg // a printed Klingon, not the changeling case
	case 1:
		return !c.king
	case 2:
		return c.chg
	case 3:
		return c.dev
	}
	return false
}

// preferred reports whether p already controls a sacrifice of its mode.
func (p *gowronPolicy) preferred(g *GorgeSide, p0 uint8) bool {
	for _, c := range creatures(g, p0) {
		if p.wanted(c) {
			return true
		}
	}
	return false
}

// priority returns the next action that steers toward the mode.
func (p *gowronPolicy) priority(g *GorgeSide, st *state.Game, c *PickCtx) (Canon, bool) {
	p0 := c.D.Player
	var setup, gowron, cast, land Canon
	haveSetup, haveGowron, haveCast, haveLand := false, false, false, false
	for _, a := range c.D.Options {
		switch a.Kind() {
		case ActCast:
			name := gowronName(g, a)
			switch {
			case p.mode == 2 && name == "Universal Automaton" && !p.preferred(g, p0):
				setup, haveSetup = a, true
			case p.mode == 3 && name == "De-evolve" && !p.preferred(g, p0) && p.hasKlingon(g, p0):
				setup, haveSetup = a, true
			case name == "Gowron, Eager for Battle":
				cast, haveCast = a, true
			case name != "De-evolve" && name != "Universal Automaton":
				if !haveCast {
					cast, haveCast = a, true
				}
			}
		case ActActivate:
			if isGowronActivation(g, a) && p.preferred(g, p0) {
				gowron, haveGowron = a, true
			}
		case ActPlayLand:
			if !haveLand {
				land, haveLand = a, true
			}
		}
	}
	if haveSetup {
		return setup, true
	}
	if haveGowron {
		return gowron, true
	}
	if haveCast {
		return cast, true
	}
	if haveLand {
		return land, true
	}
	// Never fire Gowron's sacrifice for a mode that has no candidate yet.
	var opts []Canon
	for _, a := range c.D.Options {
		if !isGowronActivation(g, a) {
			opts = append(opts, a)
		}
	}
	if len(opts) == 0 {
		return c.D.Options[0], true
	}
	cc := *c
	cc.D = &CDecision{Kind: c.D.Kind, Player: c.D.Player, Options: opts}
	return p.rnd.Pick(&cc), true
}

// hasKlingon reports whether p controls a creature that is a Klingon now (for
// De-evolve's target).
func (p *gowronPolicy) hasKlingon(g *GorgeSide, p0 uint8) bool {
	for _, c := range creatures(g, p0) {
		if c.king && !c.dev {
			return true
		}
	}
	return false
}

// target answers a target pick for De-evolve: enchant our own Klingon.
func (p *gowronPolicy) target(g *GorgeSide, st *state.Game, c *PickCtx) (Canon, bool) {
	pd := g.Pending()
	if pd == nil {
		return 0, false
	}
	src := ""
	if pd.Source != 0 {
		src = chName(st, pd.Source)
	}
	if src != "De-evolve" && (src != "" || p.mode != 3) {
		return 0, false
	}
	for _, a := range c.D.Options {
		o, ok := g.GorgeOption(a)
		if !ok || o.Obj == 0 {
			continue
		}
		for _, cr := range creatures(g, c.D.Player) {
			if cr.id == o.Obj && cr.king && !cr.dev {
				return a, true
			}
		}
	}
	return 0, false
}

// choose answers Gowron's sacrifice pick with the mode's candidate.
func (p *gowronPolicy) choose(g *GorgeSide, st *state.Game, c *PickCtx) (Canon, bool) {
	sac, _ := chooseCode("sacrifice")
	for _, a := range c.D.Options {
		if a.Kind() != ActChoose+CanonKind(sac) {
			continue
		}
		o, ok := g.GorgeOption(a)
		if !ok || o.Obj == 0 {
			continue
		}
		for _, cr := range creatures(g, c.D.Player) {
			if cr.id != o.Obj || !p.wanted(cr) {
				continue
			}
			switch p.mode {
			case 0:
				p.tally.klingon++
			case 1:
				p.tally.nonKlingon++
			case 2:
				p.tally.changeling++
			case 3:
				p.tally.deEvolved++
			}
			return a, true
		}
	}
	return 0, false
}

func runGowron(t *testing.T, from, to uint64, mutate bool) (games, divs int, tally gowronTally) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("gowron")
	if err != nil {
		t.Fatal(err)
	}
	var cs [2][]*cards.Card
	for s, d := range gowronDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[s] = append(cs[s], c)
			}
		}
		if len(cs[s]) != 40 {
			t.Fatalf("deck %d has %d cards", s, len(cs[s]))
		}
	}
	tbl, _, _, err := set.Matchup(gowronDecks[0], gowronDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugGowronPostSac
	dirac.DebugGowronPostSac = mutate
	defer func() { dirac.DebugGowronPostSac = old }()
	for seed := from; seed <= to; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"gowron-a", "gowron-b"}, cs, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := &gowronPolicy{mode: int(seed % 4), rnd: RandomPolicy(seed), tally: &tally}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, tally
}

func TestGowronLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, tally := runGowron(t, 1, 40, false)
	t.Logf("%d games, %d divergent, tally %+v", games, divs, tally)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	for _, c := range []struct {
		name string
		n    int
	}{
		{"Gowron sacrificed a Klingon", tally.klingon},
		{"Gowron sacrificed a non-Klingon", tally.nonKlingon},
		{"Gowron sacrificed a changeling", tally.changeling},
		{"Gowron sacrificed a De-evolved Klingon", tally.deEvolved},
	} {
		if c.n == 0 {
			t.Errorf("%s: never reached", c.name)
		}
	}
}

func TestGowronMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, tally := runGowron(t, 1, 20, true)
	t.Logf("%d games, %d divergent with the post-sacrifice type read, tally %+v", games, divs, tally)
	if divs == 0 {
		t.Fatalf("the post-sacrifice type read diverged in none of %d games: the matchup does not reach it", games)
	}
}
