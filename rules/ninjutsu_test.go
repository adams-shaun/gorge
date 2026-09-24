package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// K:Ninjutsu (CR 702.49) is pinned end to end on a real corpus carrier,
// Walker of Secret Ways ("Ninjutsu {1}{U}"). cards/kw_ninjutsu.go expands the
// printed K: line into a hand-zone activated ability whose Cost$ carries the
// printed mana cost plus Return<1/Creature.YouCtrl+attacking+unblocked> (the
// unblocked-attacker half) and whose ChangeZone body puts the card onto the
// battlefield tapped and attacking the defender captured when the cost was
// paid (CR 702.49b).

// ninjutsuBearSrc is the inline 1/1 Haste Bear the ninjutsu tests attack
// with. Haste lets it attack the turn it is seeded onto the battlefield, so
// the test needs no direct SummonSick write that a log-only replay could not
// reproduce.
const ninjutsuBearSrc = "Name:Ally Bear\nManaCost:G\nTypes:Creature Bear\nPT:1/1\nK:Haste\nOracle:x\n"

// ninjutsuDeck builds a seat-zero-start two-seat game with the named real
// corpus ninjutsu card and an inline 1/1 Haste Bear attacker, padded to 40
// with Mountains; the opponent plays Mountains only.
func ninjutsuDeck(t *testing.T, seed uint64, ninja *cards.Card) (*Engine, Config) {
	t.Helper()
	s0 := []*cards.Card{ninja, card(t, ninjutsuBearSrc)}
	for len(s0) < 40 {
		s0 = append(s0, mountainDeck(t, 1)...)
	}
	cfg := seatZeroStart(Config{Seed: seed, Names: []string{"ninja", "defender"},
		Decks: [][]*cards.Card{s0, mountainDeck(t, 40)}, Tokens: map[string]*cards.Card{}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e, cfg
}

// attackWithBear drives the current turn to the seat-0 declare-blockers step
// with the inline Bear declared attacking seat 1 (and therefore unblocked,
// since seat 1 declares no blockers). It returns the Bear's id.
func attackWithBear(t *testing.T, e *Engine) state.ObjID {
	t.Helper()
	bear := putCreature(t, e, 0, ninjutsuBearSrc)
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Bear not on the battlefield: %+v", o)
	}
	e.priorityRound() // moveSeeded clears the pending ask; re-pose priority
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, bear)
	driveToBlockersPriority(t, e, 0)
	if e.G.Step != state.StepDeclareBlockers || e.G.Active != 0 {
		t.Fatalf("expected seat 0's declare-blockers step, got step %s active %d", e.G.Step, e.G.Active)
	}
	if o := e.G.Obj(bear); o == nil || !o.IsAttacking || o.Attacking != 1 || len(o.BlockedBy) != 0 {
		t.Fatalf("precondition: Bear should be an UNBLOCKED attacker of seat 1, got %+v", o)
	}
	return bear
}

// TestNinjutsuRealCardEntersTappedAndAttacking is the positive pin: the
// printed Walker of Secret Ways is offered from hand at the declare-blockers
// step, its Return cost is paid by an unblocked attacker, and the card lands
// on the battlefield tapped and attacking the SAME defender the returned
// creature attacked (CR 702.49b).
func TestNinjutsuRealCardEntersTappedAndAttacking(t *testing.T) {
	if !effects.Supported()["kw:Ninjutsu"] {
		t.Fatal("kw:Ninjutsu is not registered; the coverage ratchet would still report it")
	}
	reg := searchTestRegistry(t)
	ninja := searchCorpusCard(t, reg, "Walker of Secret Ways")
	if d := ninja.Link(); len(d) != 0 {
		t.Fatalf("link Walker of Secret Ways: %v", d)
	}
	if !ninja.Faces[0].HasKeyword("Ninjutsu") {
		t.Fatal("precondition: Walker of Secret Ways does not print Ninjutsu in the corpus")
	}
	e, cfg := ninjutsuDeck(t, 9401, ninja)
	ninjaID := searchMoveByName(t, e, "Walker of Secret Ways", state.ZHand)
	if o := e.G.Obj(ninjaID); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Walker of Secret Ways not in hand: %+v", o)
	}
	bear := attackWithBear(t, e)

	fundPool(t, e, "CU") // Walker's ninjutsu cost is {1}{U}
	opt := abilityFor(t, e, 0, ninjaID)
	if opt == nil {
		t.Fatalf("ninjutsu ability not offered at the declare-blockers step: %+v", e.Pending())
	}
	submitChoices(t, e, opt.Index)

	// The Return cost asks which unblocked attacker pays. Precondition: it is
	// the Bear, still a battlefield attacker at answer time.
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "returncost" {
		t.Fatalf("ninjutsu did not ask to return an unblocked attacker: %+v", d)
	}
	chosen := -1
	for _, o := range d.Options {
		if o.Obj == bear {
			chosen = o.Index
		}
	}
	if chosen < 0 {
		t.Fatalf("ninjutsu return ask did not offer the unblocked Bear: %+v", d.Options)
	}
	submitChoices(t, e, chosen)
	passUntilStackEmpty(t, e, 40)

	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZHand {
		t.Fatalf("the returned Bear is in %v, want its owner's hand", o)
	}
	o := e.G.Obj(ninjaID)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Walker of Secret Ways is in %v, want the battlefield", o)
	}
	if !o.Tapped {
		t.Error("Walker of Secret Ways entered untapped, want tapped (CR 702.49a)")
	}
	if !o.IsAttacking || o.Attacking != 1 {
		t.Errorf("Walker of Secret Ways attacking=%v defender=%d, want attacking seat 1", o.IsAttacking, o.Attacking)
	}
	replayCheck(t, e, cfg)
}

// TestNinjutsuCommanderVariantSplitsTheRiderField pins the one corpus line
// that carries a colon rider: Yuriko, the Tiger's Shadow prints
// `K:Ninjutsu:U B:Commander` (commander ninjutsu). Only the first colon
// field is the cost, so the rider must not leak into the mana cost string --
// otherwise ParseCost reports an unknown token and the whole ability is
// unpayable and silently withheld.
func TestNinjutsuCommanderVariantSplitsTheRiderField(t *testing.T) {
	reg := searchTestRegistry(t)
	yuriko := searchCorpusCard(t, reg, "Yuriko, the Tiger's Shadow")
	if d := yuriko.Link(); len(d) != 0 {
		t.Fatalf("link Yuriko, the Tiger's Shadow: %v", d)
	}
	if !yuriko.Faces[0].HasKeyword("Ninjutsu") {
		t.Fatal("precondition: Yuriko does not print Ninjutsu in the corpus")
	}
	var sa *cards.SA
	for _, ab := range yuriko.Faces[0].Abilities {
		if ab.Params["Keyword"] == "Ninjutsu" {
			sa = ab
			break
		}
	}
	if sa == nil {
		t.Fatal("Yuriko's K:Ninjutsu did not expand to an activated ability")
	}
	raw := sa.Params["Cost"]
	if strings.Contains(raw, ":Commander") {
		t.Fatalf("the commander rider leaked into the ninjutsu cost: %q", raw)
	}
	if c := ParseCost(raw); len(c.Unknown) != 0 {
		t.Fatalf("Yuriko's ninjutsu cost %q parsed with unknowns %v", raw, c.Unknown)
	}
}

// TestNinjutsuNotOfferedBeforeDeclareBlockers pins the CR 702.49a window: at
// the controller's own declare-attackers step the hand ability is withheld.
func TestNinjutsuNotOfferedBeforeDeclareBlockers(t *testing.T) {
	reg := searchTestRegistry(t)
	ninja := searchCorpusCard(t, reg, "Walker of Secret Ways")
	if d := ninja.Link(); len(d) != 0 {
		t.Fatalf("link Walker of Secret Ways: %v", d)
	}
	e, _ := ninjutsuDeck(t, 9402, ninja)
	ninjaID := searchMoveByName(t, e, "Walker of Secret Ways", state.ZHand)
	bear := putCreature(t, e, 0, ninjutsuBearSrc)
	e.priorityRound()
	driveToStep(t, e, e.G.Turn, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, bear)
	fundPool(t, e, "CU")
	if e.G.Step != state.StepDeclareAttackers {
		t.Fatalf("expected the declare-attackers window, got step %s", e.G.Step)
	}
	if abilityFor(t, e, 0, ninjaID) != nil {
		t.Fatal("ninjutsu was offered at the declare-attackers step; the Declare Blockers window did not bind")
	}
}

// TestNinjutsuWithholdsWhenTheOnlyAttackerIsBlocked pins the
// attacking+unblocked half of the Return cost spec: a blocked attacker is
// not a legal payment, so the ability is withheld entirely (never offered
// then aborted). The block is written onto the Bear directly rather than
// driven through a KBlockers ask -- this test measures the cost gate, and
// the log-only replay it would break is not asserted here.
func TestNinjutsuWithholdsWhenTheOnlyAttackerIsBlocked(t *testing.T) {
	reg := searchTestRegistry(t)
	ninja := searchCorpusCard(t, reg, "Walker of Secret Ways")
	if d := ninja.Link(); len(d) != 0 {
		t.Fatalf("link Walker of Secret Ways: %v", d)
	}
	e, _ := ninjutsuDeck(t, 9403, ninja)
	ninjaID := searchMoveByName(t, e, "Walker of Secret Ways", state.ZHand)
	bear := attackWithBear(t, e)
	// Precondition: unblocked -> offered.
	fundPool(t, e, "CU")
	if abilityFor(t, e, 0, ninjaID) == nil {
		t.Fatal("precondition: ninjutsu should be offered while the Bear is unblocked")
	}
	// Now record a blocker on the Bear and re-ask priority.
	e.G.Obj(bear).BlockedBy = []state.ObjID{bear}
	e.pending = nil
	e.priorityRound()
	if abilityFor(t, e, 0, ninjaID) != nil {
		t.Fatal("ninjutsu was offered with a BLOCKED attacker; the unblocked cost gate did not bind")
	}
}
