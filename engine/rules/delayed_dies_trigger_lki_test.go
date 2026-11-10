package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDelayedDiesTriggerReadsLastKnownPower pins CR 608.2h for End-Blaze
// Epiphany's delayed dies trigger: "its power" is the creature's power on the
// battlefield as it last existed, not the printed power of the card now in the
// graveyard. Each case makes those two values differ before the creature dies.
func TestDelayedDiesTriggerReadsLastKnownPower(t *testing.T) {
	t.Parallel()
	const auraSrc = "Name:Test Rage\nManaCost:R\nTypes:Enchantment Aura\nK:Enchant:Creature\n" +
		"S:Mode$ Continuous | Affected$ Creature.EnchantedBy | AddPower$ 1 | Description$ Enchanted creature gets +1/+0.\nOracle:x\n"
	for _, tc := range []struct {
		name     string
		creature string
		counter  bool
		aura     bool
		want     int32
	}{
		{
			name:     "minus one counter",
			creature: "Name:Test Brute\nManaCost:1 G\nTypes:Creature Bear\nPT:3/2\nOracle:x\n",
			counter:  true,
			want:     2,
		},
		{
			name:     "plus one aura",
			creature: "Name:Test Pup\nManaCost:1 G\nTypes:Creature Bear\nPT:2/1\nOracle:x\n",
			aura:     true,
			want:     3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hand := []*cards.Card{corpusCard(t, "End-Blaze Epiphany")}
			if tc.aura {
				hand = append(hand, card(t, auraSrc))
			}
			e := handEngine(t, hand...)
			target := onBoard(t, e, 1, tc.creature)
			if tc.counter {
				e.emit(events.Event{Kind: events.CounterChange, Obj: target, Counter: "M1M1", Amount: 1})
			}
			if tc.aura {
				lossAttach(t, e, "Test Rage", 0, target)
			}

			// Precondition: the battlefield power the trigger must read differs
			// from the printed power it will read once the card is in the
			// graveyard.
			if got := e.Power(target); got != tc.want {
				t.Fatalf("precondition: battlefield power = %d, want %d", got, tc.want)
			}
			if printed := int32(e.G.Obj(target).Face().Power()); printed == tc.want {
				t.Fatalf("precondition: printed power %d equals the last-known power", printed)
			}

			spell := e.G.Zone(state.ZHand, 0)[0]
			so := e.G.Obj(spell)
			before := len(e.G.Zone(state.ZLibrary, 0))
			effects.Resolve(e, &effects.Ctx{
				Source:     spell,
				Controller: 0,
				SVars:      so.Face().SVars,
				Targets:    []state.Target{{Obj: target}},
				X:          1,
			}, so.Face().SpellAbility())

			// The one damage is lethal for each fixture, so the next
			// state-based check moves the creature to the graveyard and queues
			// the delayed dies trigger.
			e.checkStateBased()
			if z := e.G.Obj(target).Zone; z != state.ZGraveyard {
				t.Fatalf("precondition: target zone = %s, want the graveyard", z)
			}

			for i := 0; i < 50; i++ {
				if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
					break
				}
				if d := e.Pending(); d != nil {
					switch d.Kind {
					case decision.KPriority:
						pass := -1
						for _, o := range d.Options {
							if o.Kind == "pass" {
								pass = o.Index
							}
						}
						if pass < 0 {
							t.Fatalf("priority ask with no pass option: %+v", d)
						}
						submitChoices(t, e, pass)
					case decision.KChoose:
						if len(d.Options) == 0 {
							t.Fatalf("the delayed trigger's choose ask offers nothing: %+v", d)
						}
						submitChoices(t, e, d.Options[0].Index)
					default:
						t.Fatalf("unexpected ask while resolving the delayed trigger: %+v", d)
					}
					continue
				}
				if e.putTriggersOnStack() {
					continue
				}
				if len(e.G.Stack) == 0 {
					break
				}
				e.resolveTop()
			}

			if got, want := len(e.G.Zone(state.ZLibrary, 0)), before-int(tc.want); got != want {
				t.Fatalf("library after the delayed trigger = %d, want %d (it exiled the creature's last-known power %d, not the printed %d)",
					got, want, tc.want, e.G.Obj(target).Face().Power())
			}
		})
	}
}
