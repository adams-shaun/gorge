package rules

// CR 702.66a: delve applies to the SPELL with delve on the stack. Teval,
// Arbiter of Virtue's "Spells you cast have delve" is a layer-6 grant scoped
// to AffectedZone$ Stack over Card.YouCtrl+wasCast, and hasCastDelve used to
// read that grant through any object not on the battlefield -- including a
// card in the GRAVEYARD whose activated ability is being activated (the card
// was cast once, so the wasCast half matches too). The card's ability then
// picked up the grant: with the floating pool one short of the activation
// cost, delveAsk posed a choose:exile Delve pick over the graveyard before
// the mana window. An ability never has Delve: these tests pin the
// graveyard-zone activation getting no Delve ask and no Delve credit.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const gyAbilitySrc = "Name:Goldkeeper\nManaCost:2 B\nTypes:Creature Zombie\nPT:2/2\n" +
	"A:AB$ ChangeZone | Cost$ 2 B | Origin$ Graveyard | Destination$ Hand | ActivationZone$ Graveyard | ValidTgts$ Card.Self | SpellDescription$ return\n" +
	"Oracle:x\n"

// tevalGraveyardEngine puts Teval on seat 0's battlefield (teval true), casts
// the {2}{B} graveyard-return ability card (so its provenance carries
// wasCast, the grant's Affected$ half), then moves it and a delve-fodder card
// to the graveyard -- Teval's stack-scoped grant is the shape the kfuzz pairs
// run. island puts one untapped Island on the battlefield (the CR 601.2g
// sources an offered-but-unfunded activation leans on); mana is the
// activation's floating pool, added AFTER the cast. Returns the engine, the
// ability card's and the fodder's ids.
func tevalGraveyardEngine(t *testing.T, teval, island bool, mana string) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	tevalCard := corpusAlternativeCard(t, "Teval, Arbiter of Virtue")
	e := handEngine(t, tevalCard, card(t, gyAbilitySrc),
		card(t, "Name:Husk\nManaCost:G\nTypes:Creature Husk\nPT:1/1\nOracle:x\n"))
	hand := append([]state.ObjID(nil), e.G.Zone(state.ZHand, 0)...)
	if teval {
		e.emit(events.Event{Kind: events.MoveZone, Obj: hand[0], From: state.ZHand, To: state.ZBattlefield})
	}
	if island {
		is := onBoard(t, e, 0, "Name:Island\nTypes:Basic Land Island\nOracle:x\n")
		if o := e.G.Obj(is); o.Tapped {
			t.Fatalf("precondition: Island tapped")
		}
	}
	addMana(t, e, 0, "BBB")
	if opt := offeredCast(e, hand[1]); opt < 0 {
		t.Fatalf("precondition: Goldkeeper cannot be cast from hand: %+v", e.Pending().Options)
	} else {
		submitChoices(t, e, opt)
		passUntilStackEmpty(t, e, 20)
	}
	if z := e.G.Obj(hand[1]).Zone; z != state.ZBattlefield {
		t.Fatalf("precondition: Goldkeeper zone=%s after its cast, want battlefield", z)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: hand[1], From: state.ZBattlefield, To: state.ZGraveyard})
	e.emit(events.Event{Kind: events.MoveZone, Obj: hand[2], From: state.ZHand, To: state.ZGraveyard})
	addMana(t, e, 0, mana)
	return e, hand[1], hand[2]
}

// offeredGraveyardAbility is the index of the "ability" option activating
// id's printed graveyard ability, -1 when absent.
func offeredGraveyardAbility(e *Engine, id state.ObjID) int {
	d := e.Pending()
	if d == nil {
		return -1
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == id {
			return o.Index
		}
	}
	return -1
}

// drainGraveyardAbility answers the pending decisions until the ability has
// resolved (the stack empties), answering every ask with its own offered
// picks. It FAILS the moment any ask offers the fodder object -- only a Delve
// pick would -- so the Delve ask the pre-fix engine posed between the
// activation and the target ask is pinned absent, not just skipped past.
func drainGraveyardAbility(t *testing.T, e *Engine, fodder state.ObjID, limit int) {
	t.Helper()
	for n := 0; n < limit; n++ {
		if len(e.G.Stack) == 0 {
			return
		}
		d := e.Pending()
		if d == nil {
			return
		}
		if d.Kind == decision.KChoose {
			for _, o := range d.Options {
				if o.Obj == fodder {
					t.Fatalf("CR 702.66a: an activated ability was offered a Delve pick over the graveyard: %+v", d)
				}
			}
		}
		switch d.Kind {
		case decision.KChoose:
			if len(d.Options) > 0 && d.Min > 0 {
				submitChoices(t, e, d.Options[0].Index)
				continue
			}
			if d.Min == 0 {
				submitChoices(t, e)
				continue
			}
		case "target":
			submitChoices(t, e, d.Options[0].Index)
			continue
		case decision.KPriority:
			submitPass(t, e)
			continue
		default:
			t.Fatalf("unexpected %s decision while draining: %+v", d.Kind, d)
		}
	}
	t.Fatalf("the graveyard ability did not resolve within %d decisions", limit)
}

// TestTevalGraveyardAbilityNoDelveAsk: the floating pool is one short of the
// {2}{B} cost and one untapped Island covers the gap, so the activation is
// offered and must resolve through the mana window. Pre-fix the activation
// picked up Teval's stack-scoped Delve grant (the card was cast, so wasCast
// matches) and posed a choose:exile Delve pick over the graveyard first.
func TestTevalGraveyardAbilityNoDelveAsk(t *testing.T) {
	t.Parallel()
	e, ab, fodder := tevalGraveyardEngine(t, true, true, "BB")
	if e.G.Obj(ab).Zone != state.ZGraveyard || e.G.Obj(fodder).Zone != state.ZGraveyard {
		t.Fatalf("precondition: ability card zone=%s fodder zone=%s, both want graveyard",
			e.G.Obj(ab).Zone, e.G.Obj(fodder).Zone)
	}
	if e.G.Players[0].Pool.Total() != 2 {
		t.Fatalf("precondition: pool %v, want exactly the short {B}{B}", e.G.Players[0].Pool)
	}
	opt := offeredGraveyardAbility(e, ab)
	if opt < 0 {
		t.Fatalf("the {2}{B} graveyard ability is not offered over the untapped Island: %+v", e.Pending().Options)
	}
	submitChoices(t, e, opt)
	drainGraveyardAbility(t, e, fodder, 20)
	if z := e.G.Obj(ab).Zone; z != state.ZHand {
		t.Fatalf("Goldkeeper zone=%s, want hand (ability resolved)", z)
	}
	if z := e.G.Obj(fodder).Zone; z != state.ZGraveyard {
		t.Fatalf("delve fodder zone=%s, want graveyard (an ability never delves)", z)
	}
}

// TestTevalGraveyardAbilityNotOfferedWithoutSourcesAndDelve: one short of the
// {2}{B} cost with NO untapped source to fund the gap, the activation is not
// offered -- an ability's cost never carries Delve credit to close it.
func TestTevalGraveyardAbilityNotOfferedWithoutSourcesAndDelve(t *testing.T) {
	t.Parallel()
	e, ab, _ := tevalGraveyardEngine(t, true, false, "BB")
	if e.G.Obj(ab).Zone != state.ZGraveyard {
		t.Fatalf("precondition: ability card zone=%s, want graveyard", e.G.Obj(ab).Zone)
	}
	if e.G.Players[0].Pool.Total() != 2 {
		t.Fatalf("precondition: pool %v, want exactly the short {B}{B}", e.G.Players[0].Pool)
	}
	if opt := offeredGraveyardAbility(e, ab); opt >= 0 {
		t.Fatalf("the {2}{B} graveyard ability is offered on {B}{B} with one card short in the graveyard")
	}
}
