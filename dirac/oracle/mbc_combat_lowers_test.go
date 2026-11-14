package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC combat-5 spelling family: four Mystery Booster Commander combat
// spellings, each played against gorge decision by decision.
//
//	Curse of Predation    Attacks, Attacked$ Player.EnchantedBy
//	Gloom Sower           Defined$ TriggeredBlockerController on AttackerBlockedByCreature
//	Frenzied Saddlebrute  S:Mode$ CanAttackIfHaste with ValidTarget$ Opponent,Planeswalker.OppCtrl
//	Brave the Sands       S:Mode$ Continuous, Affected$ Creature.YouCtrl, CanBlockAmount$ 1
//
// Three of them lower with this family's change; the CanAttackIfHaste static
// (Frenzied Saddlebrute, Instill Energy) lowered earlier with Chaos Lord's,
// and its two cases stay here as lockstep cover of that lowering for the MBC
// card.
//
// Each case is built from the card names alone (printedDelveSet: the table
// kgen lowers for those cards, so a spelling kgen still refuses fails the
// case at Matchup) and must reach its observable effect in gorge's log in at
// least one of its seeds: a lockstep game that never exercises the lowering
// proves nothing.

type mbc5Case struct {
	name   string
	prefer []string
	decks  [2][]string
	seeds  uint64
	// attackAll declares every offered attacker (blocking stays random): the
	// random policy would rarely send the sick creature a static exempts.
	attackAll bool
	// blockAll declares the first offered block until none is offered, so a
	// blocker with room takes a second attacker.
	blockAll bool
	// reached reads the finished gorge log for the spelling's effect.
	reached func(g *GorgeSide) bool
}

func mbc5Entries(names []string) []cardset.Entry {
	var es []cardset.Entry
	for _, n := range names {
		found := false
		for i := range es {
			if es[i].Name == n {
				es[i].Count++
				found = true
				break
			}
		}
		if !found {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
	}
	return es
}

// mbc5Run plays the case's seeds in lockstep and returns the games, the games
// that reached the effect and the first divergence's description ("" for none).
func mbc5Run(t *testing.T, c mbc5Case) (games, hits int) {
	t.Helper()
	reg := synthReg(t)
	set := printedDelveSet(t, c.decks)
	var decks [2][]*cards.Card
	for s := range decks {
		for _, n := range c.decks[s] {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], card)
		}
	}
	tbl, _, _, err := set.Matchup(mbc5Entries(c.decks[0]), mbc5Entries(c.decks[1]))
	if err != nil {
		t.Fatalf("%s: the pool does not lower: %v", c.name, err)
	}
	for seed := uint64(1); seed <= c.seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: true}
		m, err := NewMatchup(reg, [2]string{c.name + "-a", c.name + "-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 30}
		reached := false
		rc.Finish = func(g *GorgeSide) { reached = c.reached(g) }
		pol := effectLongPolicy(m, effectLongCase{prefer: c.prefer}, seed)
		if c.attackAll || c.blockAll {
			base := pol
			pol = PolicyFunc(func(p *PickCtx) Canon {
				var want CanonKind
				switch {
				case p.D.Kind == DAttackers && c.attackAll:
					want = ActAttack
				case p.D.Kind == DBlockers && c.blockAll:
					want = ActBlock
				default:
					return base.Pick(p)
				}
				for _, o := range p.D.Options {
					if o.Kind() == want {
						return o
					}
				}
				return base.Pick(p)
			})
		}
		res := Lockstep(m, seed, k, pol, rc)
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			t.Fatalf("%s seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", c.name, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if reached {
			hits++
		}
	}
	return games, hits
}

// mbc5Name is the printed name of the object id in g's game.
func mbc5Name(g *GorgeSide, id state.ObjID) string {
	if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

// mbc5Count counts the events of g's log the pred admits.
func mbc5Count(g *GorgeSide, pred func(e *events.Event) bool) int {
	n := 0
	for i := range g.E.L.Events {
		if pred(&g.E.L.Events[i]) {
			n++
		}
	}
	return n
}

func curseOfPredationCase() mbc5Case {
	return mbc5Case{
		name:   "mbc5_curse_of_predation",
		prefer: []string{"Curse of Predation", "Grizzly Bears"},
		decks: [2][]string{
			append(deckOf(8, "Curse of Predation", deckOf(14, "Grizzly Bears")...), deckOf(18, "Forest")...),
			append(deckOf(20, "Grizzly Bears"), deckOf(20, "Forest")...),
		},
		seeds: 40,
		// no other card of the decks puts a counter: every +1/+1 counter on a
		// creature is a Curse of Predation trigger resolving on an attacker
		reached: func(g *GorgeSide) bool {
			return mbc5Count(g, func(e *events.Event) bool {
				return e.Kind == events.CounterChange && e.Counter == "P1P1" && e.Amount > 0 && mbc5Name(g, e.Obj) == "Grizzly Bears"
			}) > 0
		},
	}
}

func gloomSowerCase() mbc5Case {
	return mbc5Case{
		name:   "mbc5_gloom_sower",
		prefer: []string{"Gloom Sower", "Grizzly Bears"},
		decks: [2][]string{
			append(deckOf(8, "Gloom Sower", deckOf(8, "Grizzly Bears")...), deckOf(24, "Swamp")...),
			append(deckOf(24, "Grizzly Bears"), deckOf(16, "Forest")...),
		},
		seeds: 40,
		// no other card of the decks changes life: a two-life loss for the
		// blocker's controller and the Sower controller's two-life gain
		reached: func(g *GorgeSide) bool {
			lose := mbc5Count(g, func(e *events.Event) bool { return e.Kind == events.LifeChange && e.Player == 1 && e.Amount == -2 })
			gain := mbc5Count(g, func(e *events.Event) bool { return e.Kind == events.LifeChange && e.Player == 0 && e.Amount == 2 })
			return lose > 0 && gain > 0
		},
	}
}

func saddlebruteCase() mbc5Case {
	deck := func() []string {
		return append(deckOf(8, "Frenzied Saddlebrute", deckOf(14, "Hill Giant")...), deckOf(18, "Mountain")...)
	}
	return mbc5Case{
		name:      "mbc5_frenzied_saddlebrute",
		prefer:    []string{"Frenzied Saddlebrute", "Hill Giant"},
		decks:     [2][]string{deck(), deck()},
		seeds:     40,
		attackAll: true,
		// a creature declared as an attacker the turn it entered (no haste in
		// either deck: the Saddlebrute's exemption, against its controller's
		// opponent)
		reached: func(g *GorgeSide) bool {
			entered := map[state.ObjID]bool{}
			for i := range g.E.L.Events {
				e := &g.E.L.Events[i]
				switch {
				case e.Kind == events.TurnChange:
					clear(entered)
				case e.Kind == events.MoveZone && e.To == state.ZBattlefield:
					entered[e.Obj] = true
				case e.Kind == events.DeclareAttackers:
					for _, id := range e.IDs {
						if entered[id] && mbc5Name(g, id) == "Hill Giant" {
							return true
						}
					}
				}
			}
			return false
		},
	}
}

func braveTheSandsCase() mbc5Case {
	deck := func() []string {
		return append(deckOf(8, "Brave the Sands", deckOf(14, "Savannah Lions")...), deckOf(18, "Plains")...)
	}
	return mbc5Case{
		name:      "mbc5_brave_the_sands",
		prefer:    []string{"Brave the Sands", "Savannah Lions"},
		decks:     [2][]string{deck(), deck()},
		seeds:     40,
		attackAll: true,
		blockAll:  true,
		// one creature declared as the blocker of two attackers (no other card
		// of the decks lets a creature block twice)
		reached: func(g *GorgeSide) bool {
			for i := range g.E.L.Events {
				e := &g.E.L.Events[i]
				if e.Kind != events.DeclareBlockers {
					continue
				}
				seen := map[state.ObjID]int{}
				for _, pr := range e.Pairs {
					if seen[pr[1]]++; seen[pr[1]] == 2 {
						return true
					}
				}
			}
			return false
		},
	}
}

// The other cards the same lowerings reach, played in the same way:
// Instill Energy (CanAttackIfHaste over its ValidCard$ filter).

func instillEnergyCase() mbc5Case {
	deck := func() []string {
		return append(deckOf(8, "Instill Energy", deckOf(14, "Grizzly Bears")...), deckOf(18, "Forest")...)
	}
	return mbc5Case{
		name:      "mbc5_instill_energy",
		prefer:    []string{"Grizzly Bears", "Instill Energy"},
		decks:     [2][]string{deck(), deck()},
		seeds:     40,
		attackAll: true,
		// an enchanted creature declared as an attacker the turn it entered
		reached: func(g *GorgeSide) bool { return mbc5FreshAttacker(g, "Grizzly Bears") },
	}
}

func curseOfTheForsakenCase() mbc5Case {
	return mbc5Case{
		name:   "mbc5_curse_of_the_forsaken",
		prefer: []string{"Curse of the Forsaken", "Savannah Lions"},
		decks: [2][]string{
			append(deckOf(8, "Curse of the Forsaken", deckOf(14, "Savannah Lions")...), deckOf(18, "Plains")...),
			append(deckOf(20, "Savannah Lions"), deckOf(20, "Plains")...),
		},
		seeds:     40,
		attackAll: true,
		// no other card of the decks changes life: a one-life gain from the
		// Curse's trigger
		reached: func(g *GorgeSide) bool {
			return mbc5Count(g, func(e *events.Event) bool { return e.Kind == events.LifeChange && e.Amount == 1 }) > 0
		},
	}
}

// mbc5BlockedEffect is the shared read of the cards whose "becomes blocked
// by a creature" trigger names the blocker's controller (Nessian Boar draws,
// Vicious Battlerager loses 5): pred is true of an event of the effect, read
// between a declaration of blocks and the end of that step.
func mbc5BlockedEffect(g *GorgeSide, pred func(e *events.Event) bool) bool {
	blocked := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.DeclareBlockers && len(e.Pairs) > 0:
			blocked = true
		case e.Kind == events.StepChange || e.Kind == events.TurnChange:
			blocked = false
		case blocked && pred(e):
			return true
		}
	}
	return false
}

func nessianBoarCase() mbc5Case {
	return mbc5Case{
		name:   "mbc5_nessian_boar",
		prefer: []string{"Nessian Boar", "Grizzly Bears"},
		decks: [2][]string{
			append(deckOf(8, "Nessian Boar", deckOf(8, "Grizzly Bears")...), deckOf(24, "Forest")...),
			append(deckOf(20, "Grizzly Bears"), deckOf(20, "Forest")...),
		},
		seeds:     40,
		attackAll: true,
		// the blocker's controller (seat 1) draws a card between the blocks
		// and the end of the declare-blockers step
		reached: func(g *GorgeSide) bool {
			return mbc5BlockedEffect(g, func(e *events.Event) bool { return e.Kind == events.Draw && e.Player == 1 })
		},
	}
}

func viciousBattleragerCase() mbc5Case {
	return mbc5Case{
		name:   "mbc5_vicious_battlerager",
		prefer: []string{"Vicious Battlerager", "Grizzly Bears"},
		decks: [2][]string{
			append(deckOf(8, "Vicious Battlerager", deckOf(8, "Hill Giant")...), deckOf(24, "Swamp")...),
			append(deckOf(20, "Grizzly Bears"), deckOf(20, "Forest")...),
		},
		seeds:     40,
		attackAll: true,
		// the blocker's controller loses five life between the blocks and the
		// end of the declare-blockers step
		reached: func(g *GorgeSide) bool {
			return mbc5BlockedEffect(g, func(e *events.Event) bool { return e.Kind == events.LifeChange && e.Player == 1 && e.Amount == -5 })
		},
	}
}

// mbc5FreshAttacker: a creature named name was declared as an attacker in the
// turn it entered the battlefield.
func mbc5FreshAttacker(g *GorgeSide, name string) bool {
	entered := map[state.ObjID]bool{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.TurnChange:
			clear(entered)
		case e.Kind == events.MoveZone && e.To == state.ZBattlefield:
			entered[e.Obj] = true
		case e.Kind == events.DeclareAttackers:
			for _, id := range e.IDs {
				if entered[id] && mbc5Name(g, id) == name {
					return true
				}
			}
		}
	}
	return false
}

func TestMBCCombat5Lowers(t *testing.T) {
	t.Parallel()
	for _, c := range []mbc5Case{
		curseOfPredationCase(),
		gloomSowerCase(),
		saddlebruteCase(),
		braveTheSandsCase(),
		instillEnergyCase(),
		curseOfTheForsakenCase(),
		nessianBoarCase(),
		viciousBattleragerCase(),
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			games, hits := mbc5Run(t, c)
			t.Logf("%d games, %d reached the effect", games, hits)
			if hits == 0 {
				t.Fatalf("no game of %d reached the %s effect", games, c.name)
			}
		})
	}
}
