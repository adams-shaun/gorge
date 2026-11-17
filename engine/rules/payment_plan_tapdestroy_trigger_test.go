package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The tapdestroy planner fixtures: a four-source board for the {3}{B} cast
// (Cliffs with its non-mana fight ability, the two-ability choice land Isle,
// a Swamp, a Plains), the {3}{B} instant in hand, a target creature, and
// Brink of Disaster's "when enchanted permanent becomes tapped, destroy it"
// trigger attached to the choice land.
const tapDestroyTendrils = "Name:Tendr\nManaCost:3 B\nTypes:Instant\n" +
	"A:SP$ DealDamage | ValidTgts$ Creature | NumDmg$ X | SubAbility$ DBGainX | SpellDescription$ x\n" +
	"SVar:DBGainX:DB$ GainLife | LifeAmount$ X\n" +
	"SVar:X:Count$Valid SwampS.YouCtrl\n" +
	"Oracle:x\n"

const tapDestroyCliffs = "Name:Cliffs\nManaCost:no cost\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Bank {C}.\n" +
	"A:AB$ Pump | Cost$ R G T | ValidTgts$ Creature.Beast+YouCtrl | SubAbility$ ScratchFight | AILogic$ Fight | StackDescription$ None | SpellDescription$ x\n" +
	"SVar:ScratchFight:DB$ Fight | Defined$ ParentTarget | ValidTgts$ Creature.OppCtrl\n" +
	"Oracle:x\n"

const tapDestroyIsle = "Name:Isle\nManaCost:no cost\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Bank {C}.\n" +
	"A:AB$ Mana | Cost$ T | Produced$ Combo U B | IsPresent$ Swamp.YouCtrl | SpellDescription$ Add {U} or {B}.\n" +
	"Oracle:x\n"

const tapDestroySwamp = "Name:SwampS\nManaCost:no cost\nTypes:Basic Land Swamp\n" +
	"A:AB$ Mana | Cost$ T | Produced$ B | SpellDescription$ Bank {B}.\n" +
	"Oracle:x\n"

const tapDestroyPlains = "Name:PlainsS\nManaCost:no cost\nTypes:Basic Land Plains\n" +
	"A:AB$ Mana | Cost$ T | Produced$ W | SpellDescription$ Bank {W}.\n" +
	"Oracle:x\n"

const tapDestroyBeast = "Name:Beast\nManaCost:2 G\nTypes:Creature Beast\nPT:2/2\n" +
	"Oracle:x\n"

const tapDestroyBrink = "Name:BrinkD\nManaCost:2 B B\nTypes:Enchantment Aura\n" +
	"K:Enchant:Creature,Land:either one\n" +
	"T:Mode$ Taps | ValidCard$ Card.AttachedBy | Execute$ TrigBoom | TriggerDescription$ When enchanted permanent becomes tapped, destroy it.\n" +
	"SVar:TrigBoom:DB$ Destroy | Defined$ TriggeredCardLKICopy\n" +
	"Oracle:x\n"

// tapDestroyGame seeds seat 0's deck with the named extra fixtures and
// returns the engine plus the ids of each extra moved to the battlefield
// (the beast: minted as a token on seat 1's battlefield).
func tapDestroyGame(t *testing.T, seed uint64, hand, toBF []string) (*Engine, map[string]state.ObjID) {
	t.Helper()
	var extras []string
	extras = append(extras, hand...)
	extras = append(extras, toBF...)
	e, _, _ := newFixtureDeck(t, seed, tapDestroyTendrils, extras...)
	ids := map[string]state.ObjID{}
	for _, n := range hand {
		ids[card(t, n).Faces[0].Name] = moveSeeded(t, e, 0, n, state.ZHand)
	}
	for _, n := range toBF {
		ids[card(t, n).Faces[0].Name] = moveSeeded(t, e, 0, n, state.ZBattlefield)
	}
	beast := putToken(t, e, 1, tapDestroyBeast, state.ZBattlefield)
	ids["Beast"] = beast
	e.pending = nil
	e.priorityRound()
	return e, ids
}

// TestPaymentPlanTapDestroyTriggerSourcePlanned pins the destroy-on-tap
// vetting: a source whose tap fires a trigger whose only effect is to
// destroy the tapped permanent itself (Brink of Disaster on the enchanted
// land) stays an ordinary payment unit. The mana was produced at the tap
// (CR 605.4a) and the trigger resolves after the payment that caused it
// (CR 405.6c/603.2), so the {3}{B} cast beside the enchanted land is planned
// over all four untapped sources exactly as it is without the Aura.
func TestPaymentPlanTapDestroyTriggerSourcePlanned(t *testing.T) {
	t.Parallel()
	for _, brink := range []bool{false, true} {
		brink := brink
		t.Run(func() string {
			if brink {
				return "brink on isle"
			}
			return "no brink"
		}(), func(t *testing.T) {
			t.Parallel()
			e, ids := tapDestroyGame(t, 551, []string{tapDestroyTendrils}, []string{tapDestroyCliffs, tapDestroyIsle, tapDestroySwamp, tapDestroyPlains})
			if brink {
				brinkID := putToken(t, e, 0, tapDestroyBrink, state.ZBattlefield)
				e.emit(events.Event{Kind: events.Attach, Obj: brinkID, IDs: []state.ObjID{ids["Isle"]}})
				e.pending = nil
			}
			toMain1(t, e)
			e.pending = nil
			e.priorityRound()
			// Precondition: the four sources really can produce the cast's
			// four units, so the assertion below prices a payable cast.
			pm := e.PotentialMana(0)
			if pm.Total() < 4 {
				t.Fatalf("precondition: potential mana %v, want at least four units", pm)
			}
			got := e.PlanCastPayment(0, decision.PlannedCast{Object: ids["Tendr"], Origin: "hand"})
			if got.Plan == nil || got.Reason != "" {
				t.Fatalf("planner refused the cast: reason=%q detail=%q plan=%v", got.Reason, got.Detail, got.Plan != nil)
			}
			if n := len(got.Plan.Activations); n < 4 {
				t.Fatalf("plan has %d activations, want the four sources' taps", n)
			}
		})
	}
}
