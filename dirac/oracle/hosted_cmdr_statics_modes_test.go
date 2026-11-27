package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The hosted-Commander gap shapes of this ticket (The Motherlode, Excavator,
// ...), played decision by decision against gorge (gorge leads). Each case
// builds the kernel table with printedDelveSetIn, so a case is RED while kgen
// refuses its shape; the policy is hostedCmdrKwCostsPolicy (cast the carrier,
// target an opponent-owned object, seat 0 attacks with everything, the tail is
// the seed's own random stream). A case with a reached predicate FAILS unless
// some seed reaches the mechanism, so a game that never exercised it is not a
// pass.
type hostedCmdrShape struct {
	hostedCmdrCase
	real    []string // corpus cards the decks hold beside the fixtures
	policy  func(m *Matchup, seed uint64) Policy
	reached func(g *GorgeSide) bool
}

func runHostedCmdrShape(t *testing.T, c hostedCmdrShape) {
	dir := corpusDirOrSkip(t)
	if len(c.fixtures) > 0 {
		dir = filters11Corpus(t, c.fixtures, c.real...)
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
	set := printedDelveSetIn(t, dir, c.decks)
	tbl, _, _, err := set.Matchup(entries(c.decks[0]), entries(c.decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{GorgeAutopaySurface: c.surface}
	seen := false
	played := 0
	for seed := uint64(1); seed <= 80 && !(seen && played >= 3); seed++ {
		m, err := NewMatchup(reg, c.seats, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := hostedCmdrKwCostsPolicy(m, c.hostedCmdrCase, seed)
		if c.policy != nil {
			pol = c.policy(m, seed)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			if !seen && c.reached != nil && c.reached(g) {
				seen = true
			}
		}
		res := Lockstep(m, seed, k, pol, rc)
		if res.Div != nil {
			d := res.Div
			CloseSide(k)
			t.Fatalf("seed %d: divergence at %d: %s %s %v\ngorge %v\ndirac %v", seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions)
		}
		if res.Decisions > 100 {
			played++
		}
		CloseSide(k)
	}
	if played < 3 {
		t.Fatalf("only %d seeds played a real game for %s", played, c.name)
	}
	if c.reached != nil && !seen {
		t.Fatalf("%s: the mechanism was not reached on any seed played, so the lockstep is vacuous", c.name)
	}
}

// motherlodeProbeScript is The Motherlode, Excavator's attack trigger on a {1}
// body, the {E}{E}{E}{E} cost swapped for {1} life so a seed reaches it: the
// reflexive trigger destroys a nonbasic land the defending player controls and
// the creatures that player controls without flying can't block this turn.
const motherlodeProbeScript = "Name:Motherlode Probe\nManaCost:1\nTypes:Artifact Creature Robot\nPT:2/2\n" +
	"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigImmediateTrig | TriggerZones$ Battlefield | TriggerDescription$ Whenever CARDNAME attacks, you may pay 1 life. When you do, destroy target nonbasic land defending player controls, and creatures that player controls without flying can't block this turn.\n" +
	"SVar:TrigImmediateTrig:AB$ ImmediateTrigger | Cost$ PayLife<1> | Execute$ TrigDestroy | SpellDescription$ When you do, destroy target nonbasic land defending player controls, and creatures that player controls without flying can't block this turn.\n" +
	"SVar:TrigDestroy:DB$ Destroy | ValidTgts$ Land.nonBasic+ControlledBy Spawner>TriggeredDefendingPlayer | TgtPrompt$ Select target nonbasic land defending player controls | SubAbility$ DBEffect\n" +
	"SVar:DBEffect:DB$ Effect | StaticAbilities$ Static | RememberObjects$ TargetedController\n" +
	"SVar:Static:Mode$ CantBlockBy | ValidAttacker$ Creature | ValidBlocker$ Creature.withoutFlying+RememberedPlayerCtrl | Description$ Creatures that player controls without flying can't block this turn.\n" +
	"Oracle:Whenever this attacks, you may pay 1 life. When you do, destroy target nonbasic land defending player controls, and creatures that player controls without flying can't block this turn.\n"

const probeNonbasicLandScript = "Name:Probe Nonbasic Land\nManaCost:no cost\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ G | SpellDescription$ Add {G}.\n" +
	"Oracle:{T}: Add {G}.\n"

// destroyedProbeLandWithBlocker: over the whole game a seat-0 Motherlode Probe
// attacked, a seat-1 Probe Nonbasic Land left the battlefield for the
// graveyard on seat 0's turn (the reflexive Destroy; nothing else removes it)
// while seat 1 held a Grizzly Bears: the block gate the emblem closes was live,
// so a kernel that left the bears free to block would offer a block gorge
// refuses.
func destroyedProbeLandWithBlocker(g *GorgeSide) bool {
	bears := map[state.ObjID]bool{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.MoveZone {
			continue
		}
		o := g.E.G.Obj(e.Obj)
		if o == nil || o.Face() == nil || o.Owner != 1 {
			continue
		}
		switch o.Face().Name {
		case "Grizzly Bears":
			if e.To == state.ZBattlefield {
				bears[e.Obj] = true
			} else if e.From == state.ZBattlefield {
				delete(bears, e.Obj)
			}
		case "Probe Nonbasic Land":
			if e.From == state.ZBattlefield && e.To == state.ZGraveyard && len(bears) > 0 {
				return true
			}
		}
	}
	return false
}

func TestHostedCmdrCantBlockTargetedController(t *testing.T) {
	probeDeck := append(deckOf(12, "Motherlode Probe"), deckOf(28, "Swamp")...)
	oppDeck := append(append(deckOf(8, "Grizzly Bears"), deckOf(12, "Probe Nonbasic Land")...), deckOf(20, "Forest")...)
	runHostedCmdrShape(t, hostedCmdrShape{
		hostedCmdrCase: hostedCmdrCase{
			name:     "motherlode_probe",
			carrier:  "Motherlode Probe",
			seats:    [2]string{"hcs-motherlode-a", "hcs-motherlode-b"},
			decks:    [2][]string{probeDeck, oppDeck},
			fixtures: map[string]string{"Motherlode Probe": motherlodeProbeScript, "Probe Nonbasic Land": probeNonbasicLandScript},
		},
		real:    []string{"Swamp", "Forest", "Grizzly Bears"},
		reached: destroyedProbeLandWithBlocker,
	})
}
