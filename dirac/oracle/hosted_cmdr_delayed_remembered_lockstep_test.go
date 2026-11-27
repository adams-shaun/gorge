package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander delayed-trigger shapes this ticket lowers, played
// decision by decision against gorge (gorge leads):
//
//   - Gift of Immortality: the dies trigger returns the enchanted creature and
//     remembers it, then registers a delayed trigger over Remembered whose body
//     returns the Aura attached to DelayTriggerRememberedLKI. A {0} removal
//     spell forces the dies trigger.
//   - Semester's End: a multi-object Exile RememberChanged$ whose delayed
//     RememberedLKI body returns the set with a +1/+1 counter per creature (the
//     Effect's Moved replacement).
//   - Unexplained Absence: a multi-object Exile RememberLKI$ whose Cloak reads
//     RememberedController; each exiled permanent's controller cloaks their
//     library top.
//
// Each case FAILS while kgen refuses its shape: printedDelveSetIn runs kgen over
// the deck and fails on a refused card. The reached predicate then requires the
// mechanism to have happened, so a game that never casts the card cannot pass.

type hostedCmdrDelayedCase struct {
	name    string
	carrier string
	kind    string // "gift", "semester", "unexplained"
	seats   [2]string
	decks   [2][]string
	reached func(g *GorgeSide) bool
}

// giftRemovalScript is an inline {0} removal spell, not a card script from the
// GPL corpus: it only forces Gift of Immortality's dies trigger.
const giftRemovalScript = "Name:Gift Removal\nManaCost:0\nTypes:Sorcery\n" +
	"A:SP$ Destroy | ValidTgts$ Creature | TargetMin$ 1 | TargetMax$ 1 | SpellDescription$ Destroy a creature.\n" +
	"Oracle:Destroy a creature.\n"

func hostedCmdrDelayedCases() []hostedCmdrDelayedCase {
	bears := append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)
	return []hostedCmdrDelayedCase{
		{
			name:    "gift_of_immortality_delayed_return_attached",
			carrier: "Gift of Immortality",
			kind:    "gift",
			seats:   [2]string{"hcd-gift-a", "hcd-gift-b"},
			decks: [2][]string{
				append(append(deckOf(4, "Gift of Immortality"), deckOf(4, "Gift Removal")...), append(deckOf(8, "Savannah Lions"), deckOf(24, "Plains")...)...),
				bears,
			},
			reached: giftReturnedFromGraveyard,
		},
		{
			name:    "semesters_end_delayed_return_with_counters",
			carrier: "Semester's End",
			kind:    "semester",
			seats:   [2]string{"hcd-semester-a", "hcd-semester-b"},
			decks: [2][]string{
				append(append(deckOf(4, "Semester's End"), deckOf(8, "Savannah Lions")...), deckOf(28, "Plains")...),
				bears,
			},
			reached: semesterReturnedWithP1P1,
		},
		{
			name:    "unexplained_absence_cloaks_each_controller",
			carrier: "Unexplained Absence",
			kind:    "unexplained",
			seats:   [2]string{"hcd-unexplained-a", "hcd-unexplained-b"},
			decks: [2][]string{
				append(append(deckOf(4, "Unexplained Absence"), deckOf(8, "Savannah Lions")...), deckOf(28, "Plains")...),
				bears,
			},
			reached: unexplainedCloakedForBothControllers,
		},
	}
}

// giftReturnedFromGraveyard: the Aura left the graveyard for the battlefield
// after its enchanted creature died (the delayed return), and is attached to a
// live permanent at the end.
func giftReturnedFromGraveyard(g *GorgeSide) bool {
	back := false
	for _, e := range g.E.L.Events {
		if e.Kind == events.MoveZone && e.From == state.ZGraveyard && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Gift of Immortality" {
				back = true
			}
		}
	}
	if !back {
		return false
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		o := g.E.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == "Gift of Immortality" {
			return o.AttachedTo != 0
		}
	}
	return false
}

// semesterReturnedWithP1P1: a Savannah Lions the spell exiled returned to the
// battlefield carrying a +1/+1 counter.
func semesterReturnedWithP1P1(g *GorgeSide) bool {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		o := g.E.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == "Savannah Lions" && o.Counter("P1P1") > 0 {
			return true
		}
	}
	return false
}

// unexplainedCloakedForBothControllers: one cloaked permanent under each seat
// (each exiled permanent's controller cloaked their library top).
func unexplainedCloakedForBothControllers(g *GorgeSide) bool {
	var seen [2]bool
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
			if o := g.E.G.Obj(id); o != nil && o.Cloaked {
				seen[p] = true
			}
		}
	}
	return seen[0] && seen[1]
}

// hostedCmdrDelayedPolicy casts the carrier once its precondition holds, then
// answers target prompts with the permanent the shape needs, plays lands and
// otherwise picks from the seed's own stream. It never attacks, so the returned
// permanents stay on the battlefield to be read.
func hostedCmdrDelayedPolicy(m *Matchup, c hostedCmdrDelayedCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	picked, carrierCasts, removalCasts := 0, 0, 0
	var chosenCtrl [2]bool
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G == nil {
			return rnd.Pick(p)
		}
		if p.D.Kind == DTarget {
			switch c.kind {
			case "gift":
				// the removal targets the creature the Aura is attached to;
				// the Aura itself targets an own creature before it is attached
				if bearer := giftBearer(p.G); bearer != 0 {
					if a, ok := targetWhere(p, func(o state.ObjID) bool { return o == bearer }); ok {
						return a
					}
				}
				return pickTarget(p, func(o state.ObjID) bool {
					ob := p.G.E.G.Obj(o)
					return ob != nil && ob.Face() != nil && ob.Face().Name == "Savannah Lions" && ob.Controller == 0
				}, rnd)
			case "semester":
				if picked < 2 {
					if a, ok := targetWhere(p, func(o state.ObjID) bool {
						ob := p.G.E.G.Obj(o)
						return ob != nil && ob.Face() != nil && ob.Face().Name == "Savannah Lions" && ob.Controller == 0
					}); ok {
						picked++
						return a
					}
				}
				return doneTarget(p, rnd)
			case "unexplained":
				for _, want := range []state.PlayerID{1, 0} {
					if chosenCtrl[want] {
						continue
					}
					if a, ok := targetWhere(p, func(o state.ObjID) bool {
						ob := p.G.E.G.Obj(o)
						return ob != nil && ob.Face() != nil && ob.Controller == want && ob.Face().Name != "Plains" && ob.Face().Name != "Forest"
					}); ok {
						chosenCtrl[want] = true
						return a
					}
				}
				return doneTarget(p, rnd)
			}
		}
		if p.D.Kind == DAttackers {
			for _, a := range p.D.Options {
				if a.Kind() != ActAttack {
					return a
				}
			}
		}
		if p.D.Kind == DPriority {
			if c.kind == "gift" && removalCasts == 0 && giftBearer(p.G) != 0 {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == "Gift Removal" {
						removalCasts++
						return o
					}
				}
			}
			if carrierCasts == 0 && carrierCastable(p, m, c) {
				carrierCasts++
				return castOf(p, m, c)
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActCast {
					n := m.InstanceCard(int(o.Src()))
					if n == "Savannah Lions" || n == "Grizzly Bears" {
						return o
					}
				}
			}
		}
		return rnd.Pick(p)
	})
}

// giftBearer is the live permanent Gift of Immortality is attached to, 0 when
// no Aura is on the battlefield.
func giftBearer(g *GorgeSide) state.ObjID {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Gift of Immortality" && o.AttachedTo != 0 {
			return o.AttachedTo
		}
	}
	return 0
}

// targetWhere returns the first target option naming an object where holds.
func targetWhere(p *PickCtx, where func(state.ObjID) bool) (Canon, bool) {
	for _, a := range p.D.Options {
		if a.Kind() != ActTarget {
			continue
		}
		o, ok := p.G.GorgeOption(a)
		if !ok || !where(o.Obj) {
			continue
		}
		return a, true
	}
	return 0, false
}

func pickTarget(p *PickCtx, where func(state.ObjID) bool, rnd Policy) Canon {
	if a, ok := targetWhere(p, where); ok {
		return a
	}
	return rnd.Pick(p)
}

// doneTarget answers a target prompt's non-target option.
func doneTarget(p *PickCtx, rnd Policy) Canon {
	for _, a := range p.D.Options {
		if a.Kind() != ActTarget {
			return a
		}
	}
	return rnd.Pick(p)
}

// carrierCastable reports whether the carrier is offered and its precondition
// holds: Gift after an own Savannah Lions is on the battlefield, Semester when
// the caster controls one, Unexplained when both seats control a nonland
// permanent.
func carrierCastable(p *PickCtx, m *Matchup, c hostedCmdrDelayedCase) bool {
	found := false
	for _, o := range p.D.Options {
		if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == c.carrier {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	own, opp := 0, 0
	for seat := state.PlayerID(0); seat < 2; seat++ {
		for _, id := range p.G.E.G.Zone(state.ZBattlefield, seat) {
			o := p.G.E.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			if o.Face().Name == "Savannah Lions" && o.Controller == 0 {
				own++
			}
			if o.Face().Name == "Grizzly Bears" && o.Controller == 1 {
				opp++
			}
		}
	}
	switch c.kind {
	case "gift", "semester":
		return own > 0
	case "unexplained":
		return own > 0 && opp > 0
	}
	return true
}

func castOf(p *PickCtx, m *Matchup, c hostedCmdrDelayedCase) Canon {
	for _, o := range p.D.Options {
		if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == c.carrier {
			return o
		}
	}
	return p.D.Options[0]
}

// TestHostedCmdrDelayedRememberedLockstep is the scripted lockstep per shape.
func TestHostedCmdrDelayedRememberedLockstep(t *testing.T) {
	for _, c := range hostedCmdrDelayedCases() {
		t.Run(c.name, func(t *testing.T) {
			dir := corpusDirOrSkip(t)
			dir = filters11Corpus(t, map[string]string{"Gift Removal": giftRemovalScript},
				"Gift of Immortality", "Semester's End", "Unexplained Absence",
				"Savannah Lions", "Grizzly Bears", "Plains", "Forest")
			reg, err := cards.SharedCorpus(dir)
			if err != nil {
				t.Fatal(err)
			}
			entries := func(names []string) []cardset.Entry {
				es := make([]cardset.Entry, len(names))
				for i, n := range names {
					es[i] = cardset.Entry{Name: n, Count: 1}
				}
				return es
			}
			var decks [2][]*cards.Card
			for s, names := range c.decks {
				for _, n := range names {
					card, ok := reg.Lookup(n)
					if !ok {
						t.Fatalf("%s not in the corpus", n)
					}
					decks[s] = append(decks[s], card)
				}
				if len(decks[s]) != 40 {
					t.Fatalf("seat %d deck has %d cards, want 40", s, len(decks[s]))
				}
			}
			// printedDelveSet runs kgen over the decks: a refused card fails here.
			set := printedDelveSetIn(t, dir, c.decks)
			tbl, _, _, err := set.Matchup(entries(c.decks[0]), entries(c.decks[1]))
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{}
			seen := false
			for seed := uint64(1); seed <= 30 && !seen; seed++ {
				m, err := NewMatchup(reg, c.seats, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				rc := RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 16}
				rc.Finish = func(g *GorgeSide) {
					if !seen && c.reached(g) {
						seen = true
					}
				}
				res := Lockstep(m, seed, k, hostedCmdrDelayedPolicy(m, c, seed), rc)
				if res.Div != nil {
					d := res.Div
					CloseSide(k)
					t.Fatalf("seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
				}
				CloseSide(k)
			}
			if !seen {
				t.Fatalf("no seed reached the mechanism of %s", c.name)
			}
		})
	}
}
