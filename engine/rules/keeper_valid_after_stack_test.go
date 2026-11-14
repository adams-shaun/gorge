package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// ValidAfterStack$ on an activated Pump (Keeper of the Dead): the chosen
// opponent must have at most Y creature cards in their graveyard, Y being the
// controller's creature-card count minus two, judged as the ability is
// activated (CR 602.2b). K:Haste stands in for the card's summoning-sick
// {T} cost; the ability is the printed one, reworded.
const keeperFixture = "Name:Keeper Fixture\nManaCost:B B\nTypes:Creature Human Wizard\nPT:1/2\nK:Haste\n" +
	"A:AB$ Pump | ValidTgts$ Opponent | Cost$ B T | SubAbility$ KillIt | ValidAfterStack$ Ability.IsTargeting Player.HasCardsInGraveyard_Creature_LEY | SpellDescription$ Pick an opponent.\n" +
	"SVar:KillIt:DB$ Destroy | ValidTgts$ Creature.nonBlack+TargetedPlayerCtrl\n" +
	"SVar:Y:Count$ValidGraveyard Card.Creature+YouOwn/Minus.2\n" +
	"Oracle:x\n"

const keeperDead = "Name:Dead Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// keeperOffersOpponent seats Keeper with mine / theirs creature cards in the
// two graveyards and reports whether the ability is offered and, when it is,
// whether the opponent is the offered target.
func keeperOffersOpponent(t *testing.T, mine, theirs int) (offered, oppTarget bool) {
	t.Helper()
	return fixtureOffersOpponent(t, keeperFixture, mine, theirs)
}

func fixtureOffersOpponent(t *testing.T, fixture string, mine, theirs int) (offered, oppTarget bool) {
	t.Helper()
	e, _, id := newFixtureDeck(t, 41, fixture)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	for range mine {
		putGraveyard(t, e, 0, keeperDead)
	}
	for range theirs {
		putGraveyard(t, e, 1, keeperDead)
	}
	count := func(p state.PlayerID) int {
		n := 0
		for _, g := range e.G.Zone(state.ZGraveyard, p) {
			if o := e.G.Obj(g); o != nil && o.Face().Name == "Dead Bear" {
				n++
			}
		}
		return n
	}
	if count(0) != mine || count(1) != theirs {
		t.Fatalf("precondition: graveyard creature cards = %d/%d, want %d/%d", count(0), count(1), mine, theirs)
	}
	addMana(t, e, 0, "B")
	e.Advance()
	opt, ok := findAbilityOption(e, id, 0)
	if !ok {
		return false, false
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil {
		t.Fatal("no decision after activating the offered ability")
	}
	return true, indexOfPlayerOption(d, 1) != -1
}

func TestKeeperOfTheDeadValidAfterStack(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		mine, theirs int
		want         bool
	}{
		{"equal_counts", 2, 2, false},
		{"one_behind", 3, 2, false},
		{"empty_both", 0, 0, false},
		{"two_behind", 3, 1, true},
		{"two_ahead_of_empty", 2, 0, true},
		{"far_behind", 4, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			offered, opp := keeperOffersOpponent(t, tc.mine, tc.theirs)
			if offered != tc.want || opp != tc.want {
				t.Fatalf("mine=%d theirs=%d: offered=%v opponent-target=%v, want %v", tc.mine, tc.theirs, offered, opp, tc.want)
			}
		})
	}
}

// A ValidAfterStack$ spelling this build does not evaluate (the other Keepers'
// Player.hasMoreLife, withMoreCreaturesThanYou, HasCardsInHand_) must not
// empty the player-candidate census: the ability is offered against the
// opponent as it was before the key was read.
func TestUnevaluatedValidAfterStackAdmitsOpponent(t *testing.T) {
	t.Parallel()
	for _, spelling := range []string{
		"Player.hasMoreLife",
		"Player.withMoreCreaturesThanYou",
		"Player.HasCardsInHand_Card_GEY",
	} {
		t.Run(spelling, func(t *testing.T) {
			fixture := "Name:Keeper Spelling Fixture\nManaCost:B B\nTypes:Creature Human Wizard\nPT:1/2\nK:Haste\n" +
				"A:AB$ Pump | ValidTgts$ Opponent | Cost$ B T | ValidAfterStack$ Ability.IsTargeting " + spelling + " | SpellDescription$ Pick an opponent.\n" +
				"Oracle:x\n"
			offered, opp := fixtureOffersOpponent(t, fixture, 0, 0)
			if !offered || !opp {
				t.Fatalf("%s: offered=%v opponent-target=%v, want both true", spelling, offered, opp)
			}
		})
	}
}
