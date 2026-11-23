package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

func coffersGrant(landType string) string {
	return "Name:Coffers Grant\nManaCost:U\nTypes:Creature Wizard\nPT:1/1\n" +
		"S:Mode$ Continuous | Affected$ Land.YouCtrl | AddType$ " + landType + " | Description$ x\nOracle:x\n"
}

const coffersFourSpell = "Name:Coffers Four\nManaCost:3 W\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"

const coffersLand = "Name:Coffers Land\nTypes:Land\n" +
	"A:AB$ Mana | Produced$ B | Amount$ SwampCount | Cost$ 2 T | SpellDescription$ x\n" +
	"SVar:SwampCount:Count$Valid Swamp.YouCtrl\nOracle:x\n"

const coffersPlains = "Name:Coffers Plains\nTypes:Basic Land Plains\nOracle:x\n"

// A Cabal Coffers-shaped land ("{2},{T}: add {B} for each Swamp you control")
// beside a static that makes every land a Swamp (Realmwright's shape): the
// Coffers gains the intrinsic Swamp ability, so three Plains and the Coffers'
// {B} would pay {3}{W}. gorge's cast planner still prices nothing: the paid
// filter is a census gap and takes the source's intrinsic unit with it, so the
// plain planner is insufficient and only the potential planner's prefix script
// prices the cast, which the autopay surface lists as no plain cast. The
// kernel's census counted that intrinsic {B} and offered it (dirac
// dynFiltIntrinsic; tail-41 pair 0, decision 661).
func TestPayCoffersWithSwampGrantIsScriptedNotPlanned(t *testing.T) {
	t.Parallel()
	e, spell := coffersGrantBoard(t, "Swamp", 9811)
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan != nil || got.Reason != "insufficient" {
		t.Fatalf("plain planner: reason=%q plan=%v, want insufficient and no plan", got.Reason, got.Plan != nil)
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if state.ObjID(pp.Action.Obj) != spell {
			continue
		}
		if pp.Reason != "" || len(pp.Script) == 0 {
			t.Fatalf("potential plan: reason=%q script=%d, want a priced prefix script", pp.Reason, len(pp.Script))
		}
		return
	}
	t.Fatal("the cast is not a potential play")
}

// The control of the test above: made a Forest, the Coffers' Swamp count is
// zero, so its filter yields nothing (manaAbilityYieldsNothing) and stays out
// of the census, and its intrinsic {G} is an ordinary planner unit: three
// Plains and that {G} pay {3}{W} as a plain plan. The kernel's
// dynFiltIntrinsic must hold the intrinsic unit back only for a filter that
// yields (dirac TestDynFiltIntrinsicBoundary).
func TestPayCoffersWithForestGrantIsPlanned(t *testing.T) {
	t.Parallel()
	e, spell := coffersGrantBoard(t, "Forest", 9812)
	if got := e.PlanCastPayment(0, paymentCast(spell)); got.Plan == nil || got.Reason != "" {
		t.Fatalf("plain planner: reason=%q plan=%v, want a plan", got.Reason, got.Plan != nil)
	}
}

// coffersGrantBoard is p0 holding the {3}{W} spell with the Coffers, three
// Plains and a grant making every land of theirs a landType, at main phase.
func coffersGrantBoard(t *testing.T, landType string, seed uint64) (*Engine, state.ObjID) {
	t.Helper()
	spellCard, coffers, plains, grant := card(t, coffersFourSpell), card(t, coffersLand), card(t, coffersPlains), card(t, coffersGrant(landType))
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{
			append([]*cards.Card{spellCard, coffers, plains, plains, plains, grant}, mountainDeck(t, 38)...),
			mountainDeck(t, 39),
		},
		Tokens:       map[string]*cards.Card{},
		NameUniverse: cards.UniverseOf([]*cards.Card{spellCard, coffers, plains, grant}),
	})
	e := New(cfg)
	e.Advance()
	spell := findAndMoveTo(t, e, 0, "Coffers Four", state.ZHand)
	findAndMoveToBattlefield(t, e, 0, "Coffers Land")
	for i := 0; i < 3; i++ {
		findAndMoveToBattlefield(t, e, 0, "Coffers Plains")
	}
	findAndMoveToBattlefield(t, e, 0, "Coffers Grant")
	driveToStep(t, e, 1, 0, state.StepMain1)
	return e, spell
}
