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
//   - Gift of Immortality: a triggered ability returns the enchanted creature
//     and remembers it, then registers a delayed trigger over Remembered whose
//     body returns the Aura attached to DelayTriggerRememberedLKI. The probe is
//     the real script on a {W} Aura, with a {0} removal spell to force the dies
//     trigger.
//   - Semester's End: a multi-object Exile RememberChanged$ whose delayed
//     RememberedLKI body returns the set with a +1/+1 counter per creature (the
//     Effect's Moved replacement). The probe is the real script at {1}{W}.
//   - Unexplained Absence: a multi-object Exile RememberLKI$ whose Cloak reads
//     RememberedController; each exiled permanent's controller cloaks their
//     library top. The probe is the real script at {1}{W}.
//
// Each case FAILS while kgen refuses its shape: printedDelveSetIn runs kgen over
// the deck and fails on a refused card. The reached predicate then requires the
// mechanism to have happened, so a game that never casts the card cannot pass.

type hostedCmdrDelayedCase struct {
	name     string
	carrier  string
	kind     string // "gift", "semester", "unexplained"
	seats    [2]string
	decks    [2][]string
	fixtures map[string]string
	reached  func(g *GorgeSide) bool
}

const giftProbeScript = "Name:Gift Probe\nManaCost:W\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
	"SVar:AttachAITgts:Creature.!token\nSVar:AttachAILogic:Pump\n" +
	"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.EnchantedBy | Execute$ TrigChange | TriggerDescription$ When enchanted creature dies, return that card to the battlefield under its owner's control. Return CARDNAME to the battlefield attached to that creature at the beginning of the next end step.\n" +
	"SVar:TrigChange:DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | Defined$ TriggeredNewCardLKICopy | RememberChanged$ True | ForgetOtherRemembered$ True | SubAbility$ DBDelTrig\n" +
	"SVar:DBDelTrig:DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | Execute$ TrigReturn | TriggerDescription$ Return CARDNAME attached to that creature. | SubAbility$ DBCleanup | RememberObjects$ Remembered\n" +
	"SVar:TrigReturn:DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | Defined$ Self | AttachedTo$ DelayTriggerRememberedLKI\n" +
	"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\n" +
	"Oracle:Enchant creature. When enchanted creature dies, return it and this Aura at the next end step.\n"

const giftRemovalScript = "Name:Gift Removal\nManaCost:0\nTypes:Sorcery\n" +
	"A:SP$ Destroy | ValidTgts$ Creature | TargetMin$ 1 | TargetMax$ 1 | SpellDescription$ Destroy target creature.\n" +
	"Oracle:Destroy target creature.\n"

const semesterProbeScript = "Name:Semester Probe\nManaCost:1 W\nTypes:Instant\n" +
	"A:SP$ ChangeZone | ValidTgts$ Creature.YouCtrl | TargetMin$ 0 | TargetMax$ X | Origin$ Battlefield | Destination$ Exile | SubAbility$ DelTrig | RememberChanged$ True | SpellDescription$ Exile any number of target creatures you control. At the beginning of the next end step, return each of them with an additional +1/+1 counter on it.\n" +
	"SVar:DelTrig:DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | Execute$ TrigConditionEffect | RememberObjects$ RememberedLKI | SubAbility$ DBCleanup1 | TriggerDescription$ Return them with an additional +1/+1 counter.\n" +
	"SVar:DBCleanup1:DB$ Cleanup | ClearRemembered$ True\n" +
	"SVar:TrigConditionEffect:DB$ Effect | RememberObjects$ DelayTriggerRememberedLKI | ReplacementEffects$ ETBCreatPlans | ImprintOnHost$ True | ForgetOnMoved$ Exile | SubAbility$ DBReturn\n" +
	"SVar:ETBCreatPlans:Event$ Moved | ValidCard$ Creature.IsRemembered,Planeswalker.IsRemembered | Destination$ Battlefield | ReplaceWith$ DBPutP1P1 | ReplacementResult$ Updated | Description$ It enters with an additional +1/+1 counter, or an additional loyalty counter if it is a planeswalker.\n" +
	"SVar:DBPutP1P1:DB$ PutCounter | Defined$ ReplacedNewCard.Creature | CounterType$ P1P1 | ETB$ True | CounterNum$ 1 | SubAbility$ DBPutLOYALTY\n" +
	"SVar:DBPutLOYALTY:DB$ PutCounter | Defined$ ReplacedNewCard.Planeswalker | CounterType$ LOYALTY | ETB$ True | CounterNum$ 1\n" +
	"SVar:X:Count$Valid Permanent.YouCtrl\n" +
	"SVar:DBReturn:DB$ ChangeZone | Origin$ Exile | Destination$ Battlefield | Defined$ DelayTriggerRememberedLKI | SubAbility$ DBCleanEff\n" +
	"SVar:DBCleanEff:DB$ ChangeZone | Origin$ Command | Destination$ Exile | Defined$ Imprinted | SubAbility$ DBCleanup2\n" +
	"SVar:DBCleanup2:DB$ Cleanup | ClearImprinted$ True\n" +
	"Oracle:Exile any number of target creatures you control and return them with a +1/+1 counter at the next end step.\n"

const unexplainedProbeScript = "Name:Unexplained Probe\nManaCost:1 W\nTypes:Instant\n" +
	"A:SP$ ChangeZone | Origin$ Battlefield | Destination$ Exile | ValidTgts$ Permanent.nonLand | TgtPrompt$ Select up to one target nonland permanent each player controls | TargetMin$ 0 | TargetMax$ OneEach | TargetsForEachPlayer$ True | RememberLKI$ True | SubAbility$ CloakEach | SpellDescription$ For each player, exile up to one target nonland permanent that player controls. For each permanent exiled this way, its controller cloaks the top card of their library.\n" +
	"SVar:CloakEach:DB$ Cloak | Amount$ 1 | Defined$ TopOfLibrary | DefinedPlayer$ RememberedController | SubAbility$ DBCleanup\n" +
	"SVar:DBCleanup:DB$ Cleanup | ClearRemembered$ True\n" +
	"SVar:OneEach:PlayerCountPlayers$Amount\n" +
	"Oracle:For each player, exile up to one target nonland permanent that player controls. For each permanent exiled this way, its controller cloaks the top card of their library.\n"

func hostedCmdrDelayedCases() []hostedCmdrDelayedCase {
	bears := append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)
	return []hostedCmdrDelayedCase{
		{
			name:    "gift_of_immortality_delayed_return_attached",
			carrier: "Gift Probe",
			kind:    "gift",
			seats:   [2]string{"hcd-gift-a", "hcd-gift-b"},
			decks: [2][]string{
				append(append(deckOf(4, "Gift Probe"), deckOf(4, "Gift Removal")...), append(deckOf(8, "Savannah Lions"), deckOf(24, "Plains")...)...),
				bears,
			},
			fixtures: map[string]string{"Gift Probe": giftProbeScript, "Gift Removal": giftRemovalScript},
			reached:  giftReturnedFromGraveyard,
		},
		{
			name:    "semesters_end_delayed_return_with_counters",
			carrier: "Semester Probe",
			kind:    "semester",
			seats:   [2]string{"hcd-semester-a", "hcd-semester-b"},
			decks: [2][]string{
				append(append(deckOf(4, "Semester Probe"), deckOf(8, "Savannah Lions")...), deckOf(28, "Plains")...),
				bears,
			},
			fixtures: map[string]string{"Semester Probe": semesterProbeScript},
			reached:  semesterReturnedWithP1P1,
		},
		{
			name:    "unexplained_absence_cloaks_each_controller",
			carrier: "Unexplained Probe",
			kind:    "unexplained",
			seats:   [2]string{"hcd-unexplained-a", "hcd-unexplained-b"},
			decks: [2][]string{
				append(append(deckOf(4, "Unexplained Probe"), deckOf(8, "Savannah Lions")...), deckOf(28, "Plains")...),
				bears,
			},
			fixtures: map[string]string{"Unexplained Probe": unexplainedProbeScript},
			reached:  unexplainedCloakedForBothControllers,
		},
	}
}

// giftReturnedFromGraveyard: the Gift Probe Aura left the graveyard for the
// battlefield after its enchanted creature died (the delayed return), and is
// attached to a live permanent at the end.
func giftReturnedFromGraveyard(g *GorgeSide) bool {
	back := false
	for _, e := range g.E.L.Events {
		if e.Kind == events.MoveZone && e.From == state.ZGraveyard && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Gift Probe" {
				back = true
			}
		}
	}
	if !back {
		return false
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		o := g.E.G.Obj(id)
		if o != nil && o.Face() != nil && o.Face().Name == "Gift Probe" {
			return o.AttachedTo != 0
		}
	}
	return false
}

// semesterReturnedWithP1P1: a Savannah Lions the probe exiled returned to the
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

// giftBearer is the live permanent the "Gift Probe" Aura is attached to, 0
// when no Aura is on the battlefield.
func giftBearer(g *GorgeSide) state.ObjID {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Gift Probe" && o.AttachedTo != 0 {
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
			if len(c.fixtures) > 0 {
				dir = filters11Corpus(t, c.fixtures, "Savannah Lions", "Grizzly Bears", "Plains", "Forest")
			}
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
