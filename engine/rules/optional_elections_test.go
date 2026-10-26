// cr-g128: the five "you may" elections that were never asked, or were asked
// of the wrong player. Each test drives the real corpus card end to end and
// answers the election both ways: Animate's Optional$ (Crawling Barrens),
// AlterAttribute's Optional$ (Rubblebelt Braggart), DealDamage's
// OptionalDecider$ (Requiem Monolith, asked of the target's controller),
// Dig's DestZone2Optional$ (Kellan, Daring Traveler) and Mana's Chooser$ You
// (Radiant Lotus, whose behaviour was already right but noted unread).
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// answerYesNoElection answers the pending yes/no election of the given ResumeKind:
// option 0 is yes (today's behaviour), option 1 no.
func answerYesNoElection(t *testing.T, e *Engine, resume string, yes bool) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != resume || d.Min != 1 || d.Max != 1 {
		t.Fatalf("pending = %+v, want the %s yes/no election", d, resume)
	}
	want := "no"
	if yes {
		want = "yes"
	}
	for _, o := range d.Options {
		if o.Kind == want {
			submitChoices(t, e, o.Index)
			return d
		}
	}
	t.Fatalf("no %q option in the %s election: %+v", want, resume, d.Options)
	return nil
}

// TestAnimateOptionalDecline is Crawling Barrens' "{4}: Put two +1/+1
// counters on CARDNAME. Then you may have it become a 0/0 Elemental creature
// until end of turn." Declining leaves the counters and the land; accepting
// makes it a 2/2 Elemental (0/0 plus the two counters).
func TestAnimateOptionalDecline(t *testing.T) {
	t.Parallel()
	for _, yes := range []bool{false, true} {
		name := "decline"
		if yes {
			name = "accept"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, cfg, id := gateFixture(t, 701, "Crawling Barrens")
			id = gateMoveFromLibrary(t, e, "Crawling Barrens", state.ZBattlefield)
			addMana(t, e, 0, "CCCC")
			submitChoices(t, e, abilityOption(t, e, id, 1).Index)
			if d := passUntilAsk(t, e); d.ResumeKind != "animate_optional" || d.Player != 0 {
				t.Fatalf("election = %+v, want seat 0's animate_optional", d)
			}
			answerYesNoElection(t, e, "animate_optional", yes)
			passUntilStackEmpty(t, e, 30)
			o := e.G.Obj(id)
			if got := counterCount(o, "P1P1"); got != 2 {
				t.Fatalf("counters = %d, want 2", got)
			}
			if got := e.IsCreature(id); got != yes {
				t.Fatalf("IsCreature = %v, want %v", got, yes)
			}
			if yes {
				if d := e.Derived(id); d.Power != 2 || d.Toughness != 2 {
					t.Fatalf("animated P/T = %d/%d, want 2/2", d.Power, d.Toughness)
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestAlterAttributeOptionalDecline is Rubblebelt Braggart's "Whenever
// CARDNAME attacks, if it's not suspected, you may suspect it."
func TestAlterAttributeOptionalDecline(t *testing.T) {
	t.Parallel()
	for _, yes := range []bool{false, true} {
		name := "decline"
		if yes {
			name = "accept"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, cfg, id := gateFixture(t, 702, "Rubblebelt Braggart")
			id = gateMoveFromLibrary(t, e, "Rubblebelt Braggart", state.ZBattlefield)
			attackWith(t, e, id)
			if d := passUntilAsk(t, e); d.ResumeKind != "alterattribute_optional" || d.Player != 0 {
				t.Fatalf("election = %+v, want seat 0's alterattribute_optional", d)
			}
			answerYesNoElection(t, e, "alterattribute_optional", yes)
			passUntilStackEmpty(t, e, 20)
			if got := e.G.Obj(id).Suspected; got != yes {
				t.Fatalf("Suspected = %v, want %v", got, yes)
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestRequiemMonolithDeciderDeclines is Requiem Monolith's "That creature's
// controller may have this artifact deal 1 damage to it": the election is the
// TARGET's controller's, and a decline deals nothing. Accepting deals 1 and
// the granted trigger ("you" is the creature's controller) draws and loses 1 for seat 1.
func TestRequiemMonolithDeciderDeclines(t *testing.T) {
	t.Parallel()
	bearSrc := "Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	for _, yes := range []bool{false, true} {
		name := "decline"
		if yes {
			name = "accept"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, cfg, mono := gateFixture(t, 703, "Requiem Monolith")
			mono = gateMoveFromLibrary(t, e, "Requiem Monolith", state.ZBattlefield)
			bear := putToken(t, e, 1, bearSrc, state.ZBattlefield)
			e.Advance()
			handBefore := len(e.G.Zone(state.ZHand, 1))
			submitChoices(t, e, abilityOption(t, e, mono, 0).Index)
			answerKTarget(t, e, bear)
			d := passUntilAsk(t, e)
			if d.ResumeKind != "dealdamage_optional" || d.Player != 1 {
				t.Fatalf("election = %+v, want seat 1's dealdamage_optional", d)
			}
			answerYesNoElection(t, e, "dealdamage_optional", yes)
			passUntilStackEmpty(t, e, 40)
			wantDamage := int32(0)
			if yes {
				wantDamage = 1
			}
			if got := e.G.Obj(bear).Damage; got != wantDamage {
				t.Fatalf("Bears damage = %d, want %d", got, wantDamage)
			}
			if yes {
				if got := e.G.Players[1].Life; got != 19 {
					t.Fatalf("seat 1 life = %d, want 19 (the granted trigger's draw/lose)", got)
				}
				if got := len(e.G.Zone(state.ZHand, 1)); got != handBefore+1 {
					t.Fatalf("seat 1 hand = %d, want %d", got, handBefore+1)
				}
			} else {
				if got := e.G.Players[1].Life; got != 20 {
					t.Fatalf("seat 1 life = %d, want 20 on the decline", got)
				}
				if got := len(e.G.Zone(state.ZHand, 1)); got != handBefore {
					t.Fatalf("seat 1 hand = %d, want %d on the decline", got, handBefore)
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestKellanDestZone2OptionalKeepsCard is Kellan, Daring Traveler's "If it's a
// creature card with mana value 3 or less, put it into your hand. Otherwise,
// you may put it into your graveyard." The top card is a Mountain, so the
// non-creature remainder is the election.
func TestKellanDestZone2OptionalKeepsCard(t *testing.T) {
	t.Parallel()
	for _, yes := range []bool{false, true} {
		name := "decline"
		if yes {
			name = "accept"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e, cfg, id := gateFixture(t, 704, "Kellan, Daring Traveler")
			id = gateMoveFromLibrary(t, e, "Kellan, Daring Traveler", state.ZBattlefield)
			attackWith(t, e, id)
			lib := e.G.Zone(state.ZLibrary, 0)
			if len(lib) == 0 {
				t.Fatal("precondition: empty library")
			}
			top := lib[0]
			if name := e.G.Obj(top).Face().Name; name != "Mountain" {
				t.Fatalf("precondition: top card = %q, want a non-creature Mountain", name)
			}
			if d := passUntilAsk(t, e); d.ResumeKind != "dig_destzone2_optional" || d.Player != 0 {
				t.Fatalf("election = %+v, want seat 0's dig_destzone2_optional", d)
			}
			answerYesNoElection(t, e, "dig_destzone2_optional", yes)
			passUntilStackEmpty(t, e, 20)
			inGrave := inZone(e, state.ZGraveyard, 0, top)
			if inGrave != yes {
				t.Fatalf("Mountain in graveyard = %v, want %v", inGrave, yes)
			}
			if !yes {
				if got := e.G.Zone(state.ZLibrary, 0); len(got) == 0 || got[0] != top {
					t.Fatalf("decline moved the top card: library head = %v, want %d", got, top)
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}

// TestRadiantLotusChooserIsActivator is Radiant Lotus's "Choose a color.
// Target player adds three mana of the chosen color": the colour ask goes to
// the activator (Chooser$ You) while the mana goes to the target, and the
// parameter is consumed for real (no unread-parameter Note names it).
func TestRadiantLotusChooserIsActivator(t *testing.T) {
	t.Parallel()
	e, cfg, ids := edrBoard(t, testutil.CorpusRegistry(t), 705, map[string]state.Zone{
		"Radiant Lotus": state.ZBattlefield, "Sol Ring": state.ZBattlefield})
	lotus, ring := ids["Radiant Lotus"], ids["Sol Ring"]
	submitChoices(t, e, abilityOption(t, e, lotus, 0).Index)
	submitChoices(t, e, gateOptionByLabel(t, e, "x", "X = 1").Index)
	submitChoices(t, e, gateOptionByLabel(t, e, "player", "b").Index)
	pick := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "sacrifice" && o.Obj == ring {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("Sol Ring is not a sacrifice option: %+v", e.Pending().Options)
	}
	submitChoices(t, e, pick)
	passToResolution(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Player != 0 {
		t.Fatalf("expected seat 0 (Chooser$ You) to choose the colour, got %+v", d)
	}
	answerManaChoose(t, e, "Add G")
	if got := e.G.Players[1].Pool[state.MG]; got != 3 || e.G.Players[1].Pool.Total() != 3 {
		t.Fatalf("target's pool %v, want exactly {G}{G}{G}", e.G.Players[1].Pool)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("activator's pool %v, want empty", e.G.Players[0].Pool)
	}
	if hasNote(e, "Chooser$") {
		t.Fatal("Mana still notes Chooser$ as unread")
	}
	replayCheck(t, e, cfg)
}

// attackWith drives the real turn structure to seat 0's turn-3 declare
// attackers step (the creature, put onto the battlefield on turn 1, has lost
// its summoning sickness by then) and declares the creature as the only
// attacker, leaving the attack trigger pending. Real turns keep the whole run
// replayable from the log.
func attackWith(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	driveToStep(t, e, 3, 0, state.StepMain1)
	d := passUntilAsk(t, e)
	if d.Kind != decision.KAttackers {
		t.Fatalf("pending = %+v, want the attackers declaration", d)
	}
	submitAttackersOnly(t, e, id)
}
