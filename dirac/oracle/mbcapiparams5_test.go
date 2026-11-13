package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestMBCApiParams5Lowers plays the MBC api-params-5 spellings kgen now
// lowers decision by decision against gorge, on a fixture pool built from the
// decks (printedDelveSet), over seeds until each observable is met:
//
//   - param:DealDamage.NoPrevention: Arrow Storm's raid half (5 damage that
//     can't be prevented) dealt to a Swooping Protector, whose SHIELD counter
//     would otherwise prevent it (no Damage event of 5 then);
//   - param:Sacrifice.ConditionManaNotSpent: Azorius Herald sacrificed by its
//     own enters trigger when no {U} was spent to cast it, and kept when {U}
//     was (the opponent has nothing that can kill it); the same lowering's
//     ConditionManaSpent$ half on Steamcore Weird.
//
// Ardenn's Attach Object$ Valid ... Optional$ and Burrenton Forge-Tender's
// ChooseSource stay refused (see the report), so they have no game here.
func TestMBCApiParams5Lowers(t *testing.T) {
	reg := synthReg(t)
	protectors := append(deckOf(8, "Swooping Protector"), deckOf(16, "Plains")...)

	t.Run("arrow_storm_raid_damage_cant_be_prevented", func(t *testing.T) {
		c := effectLongCase{
			name:   "arrow_storm_raid",
			prefer: []string{"Arrow Storm", "Swooping Protector"},
			decks:  [2][]string{append(deckOf(6, "Arrow Storm"), append(deckOf(6, "Raging Goblin"), deckOf(12, "Mountain")...)...), protectors},
			reached: func(g *GorgeSide) bool {
				for _, e := range g.E.L.Events {
					if e.Kind == events.Damage && e.Amount == 5 && e.Obj != 0 {
						if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Swooping Protector" {
							return true // only Arrow Storm's raid half deals 5
						}
					}
				}
				return false
			},
		}
		mbcPlayUntil(t, reg, c, arrowStormPolicy, 200, func(reached int) bool { return reached >= 2 })
	})

	t.Run("azorius_herald_sacrificed_unless_blue_spent", func(t *testing.T) {
		var sacrificed, kept int
		c := effectLongCase{
			name:   "azorius_herald",
			prefer: []string{"Azorius Herald"},
			decks:  [2][]string{append(deckOf(8, "Azorius Herald"), append(deckOf(8, "Plains"), deckOf(8, "Island")...)...), protectors},
			reached: func(g *GorgeSide) bool {
				s := heraldSacrificed(g)
				k := heraldOnBattlefield(g)
				if s {
					sacrificed++
				}
				if k {
					kept++
				}
				return s || k
			},
		}
		mbcPlayUntil(t, reg, c, effectLongPolicy, 200, func(int) bool { return sacrificed >= 3 && kept >= 3 })
		t.Logf("games with a Herald sacrificed: %d, with one kept: %d", sacrificed, kept)
		if sacrificed < 3 || kept < 3 {
			t.Fatalf("Herald sacrificed in %d games and kept in %d: both reads must be met", sacrificed, kept)
		}
	})

	t.Run("steamcore_weird_damage_if_red_spent", func(t *testing.T) {
		// the same gate's ConditionManaSpent$ half: the enters trigger targets
		// as it triggers and deals 2 only if {R} was spent (the opponent has
		// only lands, so a Damage of 2 is the trigger's)
		var dealt, quiet int
		c := effectLongCase{
			name:   "steamcore_weird",
			prefer: []string{"Steamcore Weird"},
			decks:  [2][]string{append(deckOf(8, "Steamcore Weird"), append(deckOf(8, "Island"), deckOf(8, "Mountain")...)...), deckOf(24, "Plains")},
			reached: func(g *GorgeSide) bool {
				entered, hit := false, false
				for _, e := range g.E.L.Events {
					switch {
					case e.Kind == events.Damage && e.Amount == 2:
						hit = true
					case e.Kind == events.MoveZone && e.To == state.ZBattlefield:
						if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Steamcore Weird" {
							entered = true
						}
					}
				}
				if hit {
					dealt++
				} else if entered && len(g.E.G.Stack) == 0 {
					quiet++
				}
				return hit || entered
			},
		}
		mbcPlayUntil(t, reg, c, effectLongPolicy, 200, func(int) bool { return dealt >= 3 && quiet >= 1 })
		t.Logf("games with the trigger's 2 damage: %d, with a Weird entered and none dealt: %d", dealt, quiet)
	})
}

// mbcPlayUntil plays c over seeds 1..maxSeeds with pol until done holds (and
// at least mbcMinSeeds games were played), on the full option surface for
// even seeds and the autopay one for odd seeds,
// failing on any divergence or kernel stall, and on a run that never reached
// c's observable.
func mbcPlayUntil(t *testing.T, reg *cards.Registry, c effectLongCase, pol func(*Matchup, effectLongCase, uint64) Policy, maxSeeds uint64, done func(reached int) bool) {
	t.Helper()
	set := printedDelveSet(t, c.decks)
	reached := 0
	for seed := uint64(1); seed <= maxSeeds; seed++ {
		surface := c.surface
		if surface == "" {
			surface = [2]string{"full", "autopay"}[seed%2] // both option surfaces
		}
		r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: surface, Kernel: "custom"}
		m, err := r.Matchup(reg)
		if err != nil {
			t.Fatal(err)
		}
		res, ok := effectLongPlay(t, reg, c, r, set, pol(m, c, seed))
		if d := res.Div; d != nil {
			t.Logf("seed %d actions to the divergence: %v", seed, actionStrings(res.Actions[:d.At]))
			t.Fatalf("seed %d: divergence at %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stall %s", seed, res.Stall)
		}
		if ok {
			reached++
		}
		if seed >= mbcMinSeeds && done(reached) {
			t.Logf("%s: done after %d seeds, %d reaching the observable", c.name, seed, reached)
			return
		}
	}
	t.Fatalf("%s: not met in %d seeds (%d reached)", c.name, maxSeeds, reached)
}

// mbcMinSeeds is the fewest games mbcPlayUntil plays.
const mbcMinSeeds = 16

// arrowStormPolicy is effectLongPolicy, except that Arrow Storm is cast only
// in the second main phase with a Swooping Protector out (so the raid half,
// after an attack, meets the SHIELD counter).
func arrowStormPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || p.G != nil && p.G.E.G.Step == state.StepMain2 && protectorOut(p.G) {
			return inner.Pick(p)
		}
		d := *p.D
		d.Options = nil
		for _, o := range p.D.Options {
			if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Arrow Storm" {
				continue
			}
			d.Options = append(d.Options, o)
		}
		return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
	})
}

// heraldSacrificed: an Azorius Herald went from the battlefield to a
// graveyard (the opponent's Protectors cannot block it or deal it damage, so
// only its own enters trigger moves it).
func heraldSacrificed(g *GorgeSide) bool {
	for _, e := range g.E.L.Events {
		if e.Kind == events.MoveZone && e.From == state.ZBattlefield && e.To == state.ZGraveyard {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Azorius Herald" {
				return true
			}
		}
	}
	return false
}

// heraldOnBattlefield: an Azorius Herald is on the battlefield at the end
// with its sacrifice trigger no longer waiting (it resolved and kept it).
func heraldOnBattlefield(g *GorgeSide) bool {
	if len(g.E.G.Stack) != 0 {
		return false
	}
	for p := range g.E.G.Players {
		for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Azorius Herald" {
				return true
			}
		}
	}
	return false
}
