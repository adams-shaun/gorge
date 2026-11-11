package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestReproNoRegenParentTargetLockstep: CR 701.19c. Rage of Purphoros deals
// 4 damage to a creature and "it can't be regenerated this turn" (an Effect
// remembering its parent link's target). A Drudge Skeletons whose controller
// spent {B} on a regeneration shield and then targeted it with the Rage dies
// anyway: the shield is never spent. gorge and the kernel must leave the same
// state, with the Skeletons in the graveyard.
func TestReproNoRegenParentTargetLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	build := func(c map[string]int, filler string) []string {
		var out []string
		for _, n := range []string{"Drudge Skeletons", "Rage of Purphoros"} {
			for i := 0; i < c[n]; i++ {
				out = append(out, n)
			}
		}
		for len(out) < 40 {
			out = append(out, filler)
			if len(out) < 40 {
				out = append(out, "Mountain")
			}
		}
		return out
	}
	names := [2][]string{
		build(map[string]int{"Drudge Skeletons": 6, "Rage of Purphoros": 8}, "Swamp"),
		build(nil, "Forest"),
	}
	set := printedDelveSet(t, names)
	var decks [2][]*cards.Card
	var entries [2][]cardset.Entry
	for s := range names {
		for _, n := range names[s] {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
			entries[s] = append(entries[s], cardset.Entry{Name: n, Count: 1})
		}
	}
	tbl, _, _, err := set.Matchup(entries[0], entries[1])
	if err != nil {
		t.Fatal(err)
	}
	fired := 0
	for seed := uint64(1); seed <= 40; seed++ {
		m, err := NewMatchup(reg, [2]string{"noregen-a", "noregen-b"}, decks, 0)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		sc := &noRegenScenario{rnd: RandomPolicy(seed)}
		check := func(g *GorgeSide, _ Side) *Divergence {
			if sc.skelID != 0 {
				if o := g.E.G.Obj(sc.skelID); o != nil && o.Zone == state.ZGraveyard {
					sc.died = true
				}
			}
			return nil
		}
		res := Lockstep(m, seed, k, sc, RunConfig{Config: cfg, MaxDecisions: 20000, MaxTurns: 40, Check: check})
		CloseSide(k)
		if sc.rageCast == 0 {
			continue
		}
		fired++
		if res.Div != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail, res.Div.RefOptions, res.Div.KerOptions)
		}
		if sc.targeted == 0 {
			t.Fatalf("seed %d: the Rage was cast but never aimed at the Skeletons", seed)
		}
		if !sc.shieldUp {
			t.Fatalf("seed %d: the Rage was cast with no regeneration shield up", seed)
		}
		if !sc.died {
			t.Fatalf("seed %d: the shielded Skeletons survived the Rage", seed)
		}
		t.Logf("seed %d: Rage of Purphoros cast at a shielded Drudge Skeletons, no divergence", seed)
		break
	}
	if fired == 0 {
		t.Fatal("no seed cast the Rage at a shielded Skeletons")
	}
}

// noRegenScenario: player 0 plays lands and Drudge Skeletons, activates its
// regeneration shield and then casts Rage of Purphoros at it; every other
// decision is the seeded random policy.
type noRegenScenario struct {
	rnd        Policy
	skelID     state.ObjID
	wantTarget bool
	shieldUp   bool
	died       bool
	rageCast   int
	targeted   int
}

func (p *noRegenScenario) Pick(c *PickCtx) Canon {
	d := c.D
	g := c.G.E.G
	switch d.Kind {
	case DTarget:
		if p.wantTarget && p.skelID != 0 {
			ref := c.G.InstRef(p.skelID)
			for _, a := range d.Options {
				if a.Kind() == ActTarget && a.Tgt() == ref {
					p.wantTarget = false
					p.targeted++
					return a
				}
			}
		}
		return p.rnd.Pick(c)
	case DMulligan:
		return pickKind(c, ActKeep)
	case DAttackers, DBlockers:
		return pickKind(c, ActDone)
	case DPriority:
		return p.priority(c, g)
	}
	return p.rnd.Pick(c)
}

func (p *noRegenScenario) priority(c *PickCtx, g *state.Game) Canon {
	if c.D.Player == 0 {
		var skel *state.Object
		for i := range g.Objs {
			o := &g.Objs[i]
			if o.Zone == state.ZBattlefield && o.Controller == 0 && o.Card != nil {
				if f := o.Face(); f != nil && f.Name == "Drudge Skeletons" {
					skel = o
					break
				}
			}
		}
		if skel != nil {
			ref := c.G.InstRef(skel.ID)
			if skel.Counter("Shield") == 0 {
				if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActActivate && a.Src() == ref }); ok {
					return a
				}
			} else if a, ok := p.castRage(c, g); ok {
				p.skelID, p.wantTarget, p.shieldUp = skel.ID, true, true
				p.rageCast++
				return a
			}
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPlayLand }); ok {
			return a
		}
		if skel == nil {
			if a, ok := p.cast(c, g, "Drudge Skeletons"); ok {
				return a
			}
		}
	}
	return pickKind(c, ActPass)
}

func (p *noRegenScenario) castRage(c *PickCtx, g *state.Game) (Canon, bool) {
	return p.cast(c, g, "Rage of Purphoros")
}

func (p *noRegenScenario) cast(c *PickCtx, g *state.Game, name string) (Canon, bool) {
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZHand || o.Owner != 0 || o.Card == nil {
			continue
		}
		if f := o.Face(); f == nil || f.Name != name {
			continue
		}
		ref := c.G.InstRef(o.ID)
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActCast && a.Src() == ref }); ok {
			return a, true
		}
	}
	return 0, false
}
