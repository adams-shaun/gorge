package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// No-target Effect spells whose own Condition*$ keys gate the whole node,
// played decision by decision against gorge: Barrage of Boulders and Wild
// Slash's "Ferocious —" (a power-4 creature you control) and Demoralize's
// "Threshold —" (seven or more cards in your graveyard). TestStdSetReprosLockstep
// replays the repros; TestEffectConditionReprosReach requires each recorded
// game to reach the card with the case's condition truth, since a lockstep
// entry alone would pass a script that never cast it.
//
// The games are recorded by TestEffectConditionRecord (EFFECT_CONDITION_RECORD=1)
// on the kernel table built from the decks, with a policy that casts the case's
// condition cards first and answers the target asks the case's observable needs.
func effectConditionCases() []effectLongCase {
	bolts := append(deckOf(8, "Lightning Bolt"), deckOf(16, "Mountain")...)
	protectors := append(deckOf(8, "Swooping Protector"), deckOf(16, "Plains")...)
	// the Threshold cases need a long game: the opponent deck holds nothing
	// castable (no kill pressure), so the graveyard can fill up
	inert := deckOf(24, "Island")
	return []effectLongCase{
		{
			// The Wurm is cast first and stays: at the Barrage's resolution the
			// controller has a power-4 creature, so the rider's Effect lowers.
			name:    "barrage_of_boulders_ferocious_held_cant_block",
			prefer:  []string{"Craw Wurm", "Barrage of Boulders"},
			decks:   [2][]string{append(append(append(deckOf(4, "Barrage of Boulders"), deckOf(4, "Craw Wurm")...), deckOf(6, "Mountain")...), deckOf(6, "Forest")...), bolts},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				at := resolutionAt(g, "Barrage of Boulders")
				return at >= 0 && pow4HeldAt(g, at)
			},
		},
		{
			// No power-4 creature is ever cast: the Ferocious gate is false at
			// the resolution, so the can't-block Effect does not run in either
			// engine and the spell's own DamageAll still resolves.
			name:    "barrage_of_boulders_ferocious_not_held",
			prefer:  []string{"Barrage of Boulders"},
			decks:   [2][]string{append(deckOf(4, "Barrage of Boulders"), deckOf(20, "Mountain")...), bolts},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				at := resolutionAt(g, "Barrage of Boulders")
				return at >= 0 && !pow4HeldAt(g, at)
			},
		},
		{
			// Cathartic Reunion's additional cost discards two of the caster's
			// own cards, so three casts put six cards plus the resolved spells
			// in the graveyard before the Demoralize resolves; nothing in
			// either deck leaves a graveyard, so the finished count bounds the
			// resolution-time one from below.
			name:   "demoralize_threshold_held_cant_block",
			prefer: []string{"Cathartic Reunion", "Demoralize"},
			decks: [2][]string{
				append(append(deckOf(4, "Demoralize"), deckOf(4, "Cathartic Reunion")...), deckOf(16, "Mountain")...),
				inert,
			},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				at := resolutionAt(g, "Demoralize")
				// the check the sub-ability makes happens while the spell is
				// still on the stack, so the grave is everything that moved in
				// before its own resolution move; the finished state is
				// post-cleanup, so the count is read from the log
				return at >= 0 && graveCardsAt(g, at, "Demoralize", "Cathartic Reunion") >= 7
			},
		},
		{
			// No mill: fewer than seven cards are in the graveyard at the
			// resolution (the deck's only grave cards are its resolved spells),
			// so the Threshold rider does not run in either engine.
			name:    "demoralize_threshold_not_held",
			prefer:  []string{"Demoralize"},
			decks:   [2][]string{append(deckOf(4, "Demoralize"), deckOf(20, "Mountain")...), inert},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				at := resolutionAt(g, "Demoralize")
				return at >= 0 && graveCardsAt(g, at, "Demoralize", "Cathartic Reunion") < 7
			},
		},
		{
			// A Swooping Protector with the shield counter it entered with is
			// dealt Wild Slash's 2 damage: the Ferocious Effect's damage
			// can't-be-prevented held, so the SHIELD replacement did not
			// replace the damage (the spell's SubAbility$ DBDmg resolves).
			name:    "wild_slash_ferocious_held_damage_cant_be_prevented",
			prefer:  []string{"Craw Wurm", "Wild Slash", "Swooping Protector"},
			decks:   [2][]string{append(append(append(deckOf(4, "Wild Slash"), deckOf(4, "Craw Wurm")...), deckOf(8, "Mountain")...), deckOf(8, "Forest")...), protectors},
			surface: "autopay",
			reached: func(g *GorgeSide) bool {
				return resolutionAt(g, "Wild Slash") >= 0 && shieldIntactHit(g)
			},
		},
		{
			// No power-4 creature is ever cast, so the Ferocious Effect does
			// not run — but the spell's SubAbility$ DBDmg still resolves: its
			// 2 damage is dealt into a shielded Swooping Protector and the
			// SHIELD counter replaces it (no Damage event, the counter goes).
			name:    "wild_slash_ferocious_not_held_damage_prevented",
			prefer:  []string{"Wild Slash", "Swooping Protector"},
			decks:   [2][]string{append(deckOf(4, "Wild Slash"), deckOf(20, "Mountain")...), protectors},
			surface: "autopay",
			reached: slashIntoShield,
		},
	}
}

// graveCards counts the case deck's own cards that entered a graveyard from
// the stack (a resolved spell) or the hand (the discard cost). Nothing in
// these decks leaves a graveyard, so the count only grows.
func graveCardsAt(g *GorgeSide, at int, names ...string) int {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	n := 0
	for _, e := range g.E.L.Events[:at] {
		if e.Kind != events.MoveZone || e.To != state.ZGraveyard || (e.From != state.ZStack && e.From != state.ZHand) {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && want[o.Face().Name] {
			n++
		}
	}
	return n
}

// resolutionAt is the index of the named card's stack→graveyard move (its
// resolution), or -1 when it never resolved.
func resolutionAt(g *GorgeSide, name string) int {
	for i, e := range g.E.L.Events {
		if e.Kind != events.MoveZone || e.From != state.ZStack || e.To != state.ZGraveyard {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
			return i
		}
	}
	return -1
}

// pow4HeldAt: a creature with power 4 or greater was on the battlefield at
// event index at — it entered before at and had not left by then. Only the
// case's own decks cast such creatures, so the controller is the caster's.
func pow4HeldAt(g *GorgeSide, at int) bool {
	onField := map[state.ObjID]bool{}
	for _, e := range g.E.L.Events[:at] {
		if e.Kind != events.MoveZone {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil {
			continue
		}
		switch {
		case e.To == state.ZBattlefield && o.Face().IsCreature() && o.Face().Power() >= 4:
			onField[e.Obj] = true
		case e.From == state.ZBattlefield:
			delete(onField, e.Obj)
		}
	}
	return len(onField) > 0
}

// shieldIntactHit: a Swooping Protector was dealt 2 damage while it still had
// the shield counter it entered with (no Shield counter change and no earlier
// damage on it): only damage can't-be-prevented deals into an intact shield.
func shieldIntactHit(g *GorgeSide) bool {
	entered, intact := map[state.ObjID]bool{}, map[state.ObjID]bool{}
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Swooping Protector" {
				entered[e.Obj], intact[e.Obj] = true, true
			}
		case e.Kind == events.MoveZone && e.From == state.ZBattlefield:
			delete(entered, e.Obj)
			delete(intact, e.Obj)
		case e.Kind == events.CounterChange && e.Counter == "SHIELD" && e.Amount < 0:
			delete(intact, e.Obj)
		case e.Kind == events.Damage && e.Amount == 2:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Swooping Protector" &&
				entered[e.Obj] && intact[e.Obj] {
				return true
			}
		}
	}
	return false
}

// slashIntoShield: a Wild Slash resolved and a shielded Swooping Protector's
// shield counter was removed while no Swooping Protector ever took real
// damage: the not-held spell's SubAbility$ DBDmg dealt its 2 damage into the
// shield, which replaced it. The seat-0 deck has no creature, so the shield
// removal is the spell's own damage.
func slashIntoShield(g *GorgeSide) bool {
	resolved, shielded, gone, damaged := false, map[state.ObjID]bool{}, false, false
	for _, e := range g.E.L.Events {
		switch {
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Swooping Protector" {
				shielded[e.Obj] = true
			}
		case e.Kind == events.CounterChange && e.Counter == "SHIELD" && e.Amount < 0 && shielded[e.Obj]:
			gone = true
		case e.Kind == events.Damage && e.Amount > 0:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Swooping Protector" {
				damaged = true
			}
		case e.Kind == events.MoveZone && e.From == state.ZStack && e.To == state.ZGraveyard:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Wild Slash" {
				resolved = true
			}
		}
	}
	return resolved && gone && !damaged
}

// effectConditionPolicy is effectLongPolicy, with the case's target asks
// answered and its cast windows held:
//   - wild slash: the spell is cast only at a Swooping Protector on the
//     battlefield, and only while one is out.
func effectConditionPolicy(m *Matchup, c effectLongCase, seed uint64) Policy {
	inner := effectLongPolicy(m, c, seed)
	wild := strings.Contains(c.name, "wild_slash")
	return PolicyFunc(func(p *PickCtx) Canon {
		switch {
		case wild && p.D.Kind == DPriority && !protectorOut(p.G):
			d := *p.D
			d.Options = nil
			for _, o := range p.D.Options {
				if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Wild Slash" {
					continue
				}
				d.Options = append(d.Options, o)
			}
			return inner.Pick(&PickCtx{D: &d, G: p.G, Seq: p.Seq})
		case p.D.Kind == DTarget:
			for _, a := range p.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				if wild && int(a.Tgt()) < int(RefPlayer) {
					if o := p.G.E.G.Obj(p.G.id.obj[a.Tgt()]); o != nil && o.Face() != nil && o.Face().Name == "Swooping Protector" {
						return a
					}
				}
			}
		}
		return inner.Pick(p)
	})
}

// TestEffectConditionReprosReach replays the recorded games: no divergence,
// every scripted action offered, the card reached with the case's condition
// truth.
func TestEffectConditionReprosReach(t *testing.T) {
	reg := synthReg(t)
	for _, c := range effectConditionCases() {
		t.Run(c.name, func(t *testing.T) {
			replayEffectLongCase(t, reg, c)
		})
	}
}

// TestEffectConditionRecord plays the cases' policies over seeds until one
// reaches the card with the condition's truth, and writes the repro. It runs
// only with EFFECT_CONDITION_RECORD=1; the games it writes are the committed
// repros.
func TestEffectConditionRecord(t *testing.T) {
	if os.Getenv("EFFECT_CONDITION_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips, since a skip here must name a missing pool (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	for _, c := range effectConditionCases() {
		t.Run(c.name, func(t *testing.T) {
			set := printedDelveSet(t, c.decks)
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks, Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom", Pool: "custom", Why: "Effect-level Condition* gate defers to the node's condition read: " + c.name}
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c, r, set, effectConditionPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.name)
		})
	}
}
