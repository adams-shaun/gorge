package rules

// An Effect copies the host's chosen CARD at REGISTRATION (Forge's EffectEffect
// copies the chosen cards onto the effect card it creates), so a chain's
// `DB$ Cleanup | ClearChosenCard$ True` that follows it must not retract the
// chosen set an Effect-delivered trigger already reads. The measured carrier is
// Azra Oddsmaker: its BeginCombat trigger resolves ChooseCard, `DB$ Effect`
// registers a Mode$ DamageDone trigger whose ValidSource$ reads
// Card.ChosenCardStrict, and `DB$ Cleanup` clears the host's chosen list
// before the chosen creature ever deals combat damage.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const chosenBearSrc = "Name:Chosen Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

const otherBearSrc = "Name:Other Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// TestAzraOddsmakerEffectKeepsChosenCreature is the regression: the chosen
// creature's combat damage to a player draws two AFTER the chain's own
// Cleanup wiped the host's chosen list; another creature's combat damage
// (dealt in the same combat) draws nothing.
func TestAzraOddsmakerEffectKeepsChosenCreature(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	azraCard := lookup(t, reg, "Azra Oddsmaker")
	effectBody, ok := azraCard.Faces[0].SVars["DBEffect"]
	if !ok || !strings.Contains(effectBody, "Triggers$ TrigDamage") ||
		!strings.Contains(azraCard.Faces[0].SVars["TrigDamage"], "Card.ChosenCardStrict") ||
		!strings.Contains(azraCard.Faces[0].SVars["DBCleanup"], "ClearChosenCard$ True") {
		t.Fatalf("corpus precondition: Azra Oddsmaker chain = %q / %q / %q, want Effect TrigDamage then ClearChosenCard",
			effectBody, azraCard.Faces[0].SVars["TrigDamage"], azraCard.Faces[0].SVars["DBCleanup"])
	}
	e, cfg := combatTriggerBoard(t, reg, []string{"Azra Oddsmaker"},
		[]string{chosenBearSrc, otherBearSrc}, nil, nil)
	// A logged TurnChange arms seat 0 for combat, exactly as the
	// CombatDamage$ gate test parks the clock.
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	azra := findBattlefield(t, e, 0, "Azra Oddsmaker", 0)
	bear := findBattlefield(t, e, 0, "Chosen Bear", 0)
	other := findBattlefield(t, e, 0, "Other Bear", 0)

	// Seed the choice through the real event the ChooseCard primitive emits
	// (the Choose "chosen" fold), then resolve the corpus chain by hand: the
	// Effect registers TrigDamage, then SubAbility DBCleanup wipes the host.
	e.emit(events.Event{Kind: events.Choose, Obj: azra, Counter: "chosen", IDs: []state.ObjID{bear}})
	if got := e.G.Obj(azra).Chosen; len(got) != 1 || got[0].Obj != bear {
		t.Fatalf("precondition: chosen list = %+v, want [{%d}]", got, bear)
	}
	face := e.G.Obj(azra).Face()
	effectSA := cards.ResolveSVar(face.SVars, "DBEffect")
	if effectSA == nil {
		t.Fatal("corpus precondition: DBEffect SVar does not resolve")
	}
	effects.Resolve(e, effects.NewCtxPtr(azra, 0, effects.CtxInit{SVars: face.SVars}), effectSA)

	// The chain's own Cleanup must have wiped the host: that is the state the
	// registration has to survive.
	if got := e.G.Obj(azra).Chosen; len(got) != 0 {
		t.Fatalf("Cleanup left the chosen list = %+v, want empty (precondition for the regression)", got)
	}
	var dt *state.DelayedTrigger
	for i := range e.G.Delayed {
		if e.G.Delayed[i].Source == azra && e.G.Delayed[i].EventMode == "DamageDone" {
			dt = &e.G.Delayed[i]
			break
		}
	}
	if dt == nil {
		t.Fatal("Azra Oddsmaker Effect registered no DamageDone trigger")
	}
	if !dt.ChosenCardFrozen || len(dt.ChosenCard) != 1 || dt.ChosenCard[0] != bear {
		t.Fatalf("trigger chosen-card snapshot = (%v, frozen=%v), want ([%d], true)",
			dt.ChosenCard, dt.ChosenCardFrozen, bear)
	}

	// Both creatures attack a bare defender: the chosen bear and Other Bear
	// each deal 2 combat damage in their own batch. Only the chosen one's
	// damage fires the registration.
	handBefore := len(e.G.Zone(state.ZHand, 0))
	e.askAttackers()
	submitAttackers(t, e, bear, other)
	drainCombatDamagePriority(t, e)
	passUntilStackEmpty(t, e, 30)
	if got := e.G.Players[1].Life; got != 16 {
		t.Fatalf("defender life = %d, want 16 (both attackers dealt 2)", got)
	}
	if n := countDelayedPushes(e, azra); n != 1 {
		t.Fatalf("logged %d DelayedPush for the registration, want 1 (the chosen bear's damage only)", n)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != handBefore+2 {
		t.Fatalf("seat 0 hand = %d, want %d (two drawn off the chosen bear's hit)", got, handBefore+2)
	}
	replayCheck(t, e, cfg)
}
