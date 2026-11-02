package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The until-leaves return after a departure batch (CR 603.6a, 610.3, 610.3d):
// Run Away Together bounces a Driftgloom Coyote and a Valley Mightcaller in one
// ChangeZone. The batch is one event, so the Coyote's exiled Stickytongue Sentinel
// comes back after it, with the Mightcaller already gone and so not triggering for
// it. The mirror below plays that shape.
var taildivDeck = []cardset.Entry{
	{Name: "Driftgloom Coyote", Count: 4}, {Name: "Run Away Together", Count: 4},
	{Name: "Stickytongue Sentinel", Count: 4}, {Name: "Valley Mightcaller", Count: 4},
	{Name: "Banishing Light", Count: 2}, {Name: "Brave-Kin Duo", Count: 2},
	{Name: "Bakersbane Duo", Count: 2}, {Name: "Plains", Count: 6},
	{Name: "Island", Count: 5}, {Name: "Forest", Count: 7},
}

// taildivPolicy: play a land; else cast Coyote; else Run Away Together when a
// Coyote is on the battlefield; else a random cast or attack; else pass.
// Targets: the exile prefers an opposing Sentinel then Mightcaller; Run Away
// Together prefers a Coyote, then (odd seeds) a Mightcaller.
func taildivPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	name := func(c *PickCtx, o state.ObjID) string {
		if ob := c.G.E.G.Obj(o); ob != nil && ob.Card != nil {
			return ob.Face().Name
		}
		return ""
	}
	// gorge does not set ExiledWith for this exile, so the policy links each
	// newly exiled card to an unlinked Coyote of the other player (the
	// newest first) and drops the link when the Coyote leaves.
	linked := map[state.ObjID]bool{}
	seen := map[state.ObjID]bool{}
	track := func(c *PickCtx) {
		g := c.G.E.G
		for p := state.PlayerID(0); p < 2; p++ {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				if linked[id] && name(c, id) != "Driftgloom Coyote" {
					delete(linked, id)
				}
			}
			for id := range linked {
				if ob := g.Obj(id); ob == nil || ob.Zone != state.ZBattlefield {
					delete(linked, id)
				}
			}
			for _, id := range g.Zone(state.ZExile, p) {
				if seen[id] {
					continue
				}
				seen[id] = true
				var pick state.ObjID
				for _, cy := range g.Zone(state.ZBattlefield, 1-p) {
					if name(c, cy) == "Driftgloom Coyote" && !linked[cy] && cy > pick {
						pick = cy
					}
				}
				if pick != 0 {
					linked[pick] = true
				}
			}
		}
	}
	// coyoteUp: the priority holder controls a Coyote and the opponent has a
	// card in exile (the Coyote's, or a Banishing Light's, exile)
	coyoteUp := func(c *PickCtx) bool {
		me := state.PlayerID(c.D.Player)
		if len(c.G.E.G.Zone(state.ZExile, 1-me)) == 0 {
			return false
		}
		for _, id := range c.G.E.G.Zone(state.ZBattlefield, 1-me) {
			if name(c, id) == "Valley Mightcaller" {
				for _, id := range c.G.E.G.Zone(state.ZBattlefield, me) {
					if linked[id] {
						return true
					}
				}
			}
		}
		return false
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		track(c)
		if c.D.Kind == DTarget {
			st := c.G.E.G.Stack
			ratCtx := len(st) > 0 && name(c, st[len(st)-1]) == "Run Away Together"
			best, bs := c.D.Options[0], -1
			for _, a := range c.D.Options {
				o, ok := c.G.GorgeOption(a)
				if !ok || a.Kind() != ActTarget {
					continue
				}
				mine := false
				if ob := c.G.E.G.Obj(o.Obj); ob != nil {
					mine = ob.Controller == state.PlayerID(c.D.Player)
				}
				sc := 0
				switch n := name(c, o.Obj); {
				case ratCtx && linked[o.Obj] && mine:
					sc = 5
				case ratCtx && n == "Valley Mightcaller" && !mine && seed%2 == 1:
					sc = 4
				case !ratCtx && n == "Stickytongue Sentinel":
					sc = 5
				case !ratCtx && n == "Valley Mightcaller":
					sc = 4
				}
				if sc > bs {
					best, bs = a, sc
				}
			}
			if bs > 0 {
				return best
			}
			return rnd.Pick(c)
		}
		var land, coy, rat, other, pass Canon
		var frog Canon
		var hFrog, hLand, hCoy, hRat, hOther, hPass bool
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActPass, ActDone:
				pass, hPass = a, true
			case ActPlayLand:
				land, hLand = a, true
			case ActCast, ActAttack:
				if a.Kind() == ActAttack {
					// departs from the measured policy: attacks only on
					// seed%3==0, and the "any other legal creature" Run Away
					// Together target falls back to RandomPolicy (branch counts
					// still clear the floor)
					if seed%3 == 0 { // the other games let the boards fill
						other, hOther = a, true
					}
					continue
				}
				o, ok := c.G.GorgeOption(a)
				if !ok {
					continue
				}
				switch name(c, o.Obj) {
				case "Driftgloom Coyote":
					coy, hCoy = a, true
				case "Run Away Together":
					rat, hRat = a, true
				case "Valley Mightcaller", "Stickytongue Sentinel":
					frog, hFrog = a, true
				default:
					other, hOther = a, true
				}
			}
		}
		switch {
		case hLand:
			return land
		case hCoy:
			return coy
		case hRat && coyoteUp(c):
			return rat
		case hFrog:
			return frog
		case hOther:
			return other
		case hPass:
			return pass
		}
		return rnd.Pick(c)
	})
}

// taildivBranches counts, from gorge's event log, the games' Run Away
// Together resolutions: a (an until-leaves return after the departure, in the
// same resolution, of a Valley Mightcaller of the returned card's owner) and b
// (a later Valley Mightcaller trigger for the returned card).
func taildivBranches(g *GorgeSide) (a, b int) {
	gm := g.E.G
	evs := g.E.L.Events
	nm := func(o state.ObjID) string {
		if ob := gm.Obj(o); ob != nil && ob.Card != nil {
			return ob.Face().Name
		}
		return ""
	}
	returned := map[state.ObjID]bool{}
	for i := range evs {
		if evs[i].Kind.String() != "stack_resolve" || nm(evs[i].Obj) != "Run Away Together" {
			continue
		}
		var left [2]bool // the owners of the Mightcallers gone so far
		for j := i + 1; j < len(evs); j++ {
			e := &evs[j]
			k := e.Kind.String()
			if k == "stack_resolve" || k == "priority" {
				break
			}
			if k != "move_zone" {
				continue
			}
			ob := gm.Obj(e.Obj)
			if ob == nil {
				continue
			}
			if e.From == state.ZBattlefield && nm(e.Obj) == "Valley Mightcaller" {
				left[ob.Owner] = true
			} else if e.From == state.ZExile && e.To == state.ZBattlefield {
				returned[e.Obj] = true
				if left[ob.Owner] {
					a++
				}
			}
		}
	}
	for i := range evs {
		if evs[i].Kind.String() == "trigger_push" && len(evs[i].IDs) > 0 && nm(evs[i].Obj) == "Valley Mightcaller" {
			if returned[evs[i].IDs[0]] {
				b++
			}
		}
	}
	return a, b
}

func TestTaildivLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("blb")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range taildivDeck {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck has %d cards", len(deck))
	}
	tbl, _, _, err := set.Matchup(taildivDeck, taildivDeck)
	if err != nil {
		t.Fatal(err)
	}
	games, brA, brB := 0, 0, 0
	for seed := uint64(1); seed <= 64; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"taildiv-a", "taildiv-b"}, [2][]*cards.Card{deck, deck}, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var last *GorgeSide
		pol := taildivPolicy(seed)
		res := Lockstep(m, seed, k, PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				last = c.G
			}
			return pol.Pick(c)
		}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		if last != nil {
			a, b := taildivBranches(last)
			if a > 0 {
				brA++
			}
			if b > 0 {
				brB++
			}
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d with a return+departure in one resolution (a), %d with a Mightcaller trigger for the returned card (b)", games, brA, brB)
	if brA < 10 || brB < 10 {
		t.Errorf("branches a=%d b=%d, want at least 10 each", brA, brB)
	}
}
