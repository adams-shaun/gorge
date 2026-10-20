package rules

// CR 601.2d for the two other divided primitives: a targeted PutCounter
// ("distribute N counters among targets") and a targeted PreventDamage
// ("prevent the next N damage ... divided as you choose") announce the
// division while casting, each target gets at least 1, and a target that
// becomes illegal loses its share (CR 608.2b). Real corpus cards.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBountyOfTheHuntDistributionIsAnnouncedAndKept: Bounty of the Hunt
// distributes three +1/+1 counters, 2 to Bears A and 1 to Bears B, announced
// while casting. Bears A dies in response; Bears B gets exactly its 1.
func TestBountyOfTheHuntDistributionIsAnnouncedAndKept(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{choiceCorpusCard(t, "Bounty of the Hunt"),
		choiceCorpusCard(t, "Grizzly Bears"), choiceCorpusCard(t, "Grizzly Bears")}, nil)
	bounty := moveByName(t, e, 0, "Bounty of the Hunt", state.ZHand)
	a := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	b := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	addMana(t, e, 0, "GGGGG")
	var base int = -1
	for _, o := range castOptions(t, e) {
		if o.Obj == bounty && o.AltCostIndex == 0 {
			base = o.Index
		}
	}
	if base < 0 {
		t.Fatal("Bounty of the Hunt's mana-cost cast not offered")
	}
	submitChoices(t, e, base)
	td := e.Pending()
	if td == nil || td.Kind != decision.KTarget {
		t.Fatalf("target ask missing: %+v", td)
	}
	submitChoices(t, e, optForObj(t, td, a), optForObj(t, td, b))
	dd := e.Pending()
	if !isCastDivision(dd) || dd.Min != 1 || dd.Max != 1 {
		t.Fatalf("CR 601.2d: no 1-pick distribution announced while casting; pending = %+v", dd)
	}
	submitChoices(t, e, optForObj(t, dd, a)) // A 2, B 1
	killBear(t, e, a)
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Obj(b).Counter("P1P1"); got != 1 {
		t.Fatalf("Bears B +1/+1 counters = %d, want 1: A's 2 must not be redistributed (CR 601.2d, 608.2b)", got)
	}
	replayCheck(t, e, cfg)
}

// TestRemedyPreventionDivisionIsAnnounced: Remedy divides "the next 5
// damage" between the opponent's Bears (1) and its caster (4), announced
// while casting. Each recipient's shield holds exactly its announced share.
func TestRemedyPreventionDivisionIsAnnounced(t *testing.T) {
	t.Parallel()
	reg := choiceCorpusRegistry(t)
	e, cfg := corpusEngineCfg(t, reg, []*cards.Card{choiceCorpusCard(t, "Remedy")},
		[]*cards.Card{choiceCorpusCard(t, "Grizzly Bears")})
	remedy := moveByName(t, e, 0, "Remedy", state.ZHand)
	bear := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	addMana(t, e, 0, "WW")
	submitChoices(t, e, castOptionFor(t, e, remedy).Index)
	td := e.Pending()
	if td == nil || td.Kind != decision.KTarget {
		t.Fatalf("target ask missing: %+v", td)
	}
	submitChoices(t, e, optForObj(t, td, bear), optForPlayer(t, td, 0))
	dd := e.Pending()
	if !isCastDivision(dd) || dd.Min != 3 || dd.Max != 3 {
		t.Fatalf("CR 601.2d: no 3-pick division announced while casting; pending = %+v", dd)
	}
	p := optForPlayer(t, dd, 0)
	submitChoices(t, e, p, p, p) // Bears 1, caster 4
	passUntilStackEmpty(t, e, 30)
	var bearShield, youShield int32 = -1, -1
	for _, ce := range e.continuous {
		if ce.ReplacementParams["PreventionShield"] != "True" {
			continue
		}
		if len(ce.Remembered) == 1 && ce.Remembered[0] == bear {
			bearShield = ce.ChosenNumber
		}
		if len(ce.RememberedPlayers) == 1 && ce.RememberedPlayers[0] == 0 {
			youShield = ce.ChosenNumber
		}
	}
	if bearShield != 1 || youShield != 4 {
		t.Fatalf("shields: Bears %d, caster %d; want the announced 1 and 4", bearShield, youShield)
	}
	replayCheck(t, e, cfg)
}
