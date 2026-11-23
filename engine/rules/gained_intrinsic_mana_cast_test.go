package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// animateFixture is a freely authored sorcery that animates a target land
// into a 1/1 Elemental creature in addition to its other types, the shape of
// Rootwise Survivor's Survival rider ("That land becomes a 0/0 Elemental
// creature in addition to its other types", minus the counters that would
// make it 3/3), so the animated land keeps its printed basic land type and
// the CR 305.6 intrinsic mana ability its face carries.
const animateFixture = "Name:Test Awaken\nManaCost:1\nTypes:Sorcery\n" +
	"A:SP$ Animate | ValidTgts$ Land.YouCtrl | TgtPrompt$ Select target land | Defined$ Targeted | Power$ 1 | Toughness$ 1 | Types$ Creature,Elemental | Duration$ Permanent | SpellDescription$ Target land becomes a 1/1 Elemental creature in addition to its other types.\n" +
	"Oracle:Target land becomes a 1/1 Elemental creature in addition to its other types.\n"

// TestGainedIntrinsicManaPricesCast pins the gained mana ability a
// GainsAbilitiesOf$ recipient has for an activated CR 605.1a ability whose
// line is the parser's "intrinsic: basic land mana" (CR 613.1f, CR 305.6,
// CR 605.1a): Marvin, Murderous Mimic beside an animated Forest taps for
// {G} like the Forest itself, so its controller's potential actions price a
// {3}{U} spell from a {G}{G}{U}{U} board. The knife-edge is exact: the
// plain Forest alone leaves the cast one mana short, and the face-down
// shape is out of scope here (a manifested card has no printed abilities,
// CR 708.5, so it funds nothing and gains nothing).
func TestGainedIntrinsicManaPricesCast(t *testing.T) {
	t.Parallel()
	for _, animate := range []bool{false, true} {
		name := "forest_plain_no_plan"
		if animate {
			name = "forest_animated_marvin_gains_intrinsic"
		}
		t.Run(name, func(t *testing.T) {
			awaken := card(t, animateFixture)
			marvin := corpusAlternativeCard(t, "Marvin, Murderous Mimic")
			glimmer := corpusAlternativeCard(t, "Glimmerburst")
			forest := corpusAlternativeCard(t, "Forest")
			island := corpusAlternativeCard(t, "Island")
			e, cfg := tokenReplGameSeats(t, 9182,
				[]*cards.Card{awaken, glimmer, marvin, forest, island, island}, nil)
			toMain1(t, e)
			// The dealt cards sit in seat 0's opening hand (they head the
			// deck); move every one of them where the scenario needs it.
			// The deck is shuffled, so the dealt cards may sit in the library:
			// pull every named one into the hand through a logged MoveZone
			// (the same logged move the replay rebuilds) before anything else.
			var ids = map[string]state.ObjID{}
			var islands []state.ObjID
			for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
				for _, id := range e.G.Zone(z, 0) {
					o := e.G.Obj(id)
					if o == nil || o.Face() == nil {
						continue
					}
					switch o.Face().Name {
					case "Test Awaken", "Marvin, Murderous Mimic", "Forest", "Glimmerburst":
						if _, ok := ids[o.Face().Name]; !ok {
							ids[o.Face().Name] = id
							if z == state.ZLibrary {
								e.emit(events.Event{Kind: events.MoveZone, Obj: id, Player: 0,
									From: z, To: state.ZHand})
								e.pending = nil
							}
						}
					case "Island":
						ids["Island"+string(rune(len(islands))+'0')] = id
						islands = append(islands, id)
						if z == state.ZLibrary {
							e.emit(events.Event{Kind: events.MoveZone, Obj: id, Player: 0,
								From: z, To: state.ZHand})
							e.pending = nil
						}
					}
				}
			}
			for _, name := range []string{"Test Awaken", "Marvin, Murderous Mimic", "Forest", "Glimmerburst"} {
				if ids[name] == 0 {
					t.Fatalf("%q not in seat 0's opening hand", name)
				}
			}
			for _, name := range []string{"Island0", "Island1"} {
				if ids[name] == 0 {
					t.Fatalf("%q not in seat 0's opening hand", name)
				}
			}
			forestID, islandA, islandB := ids["Forest"], islands[0], islands[1]
			// The Islands onto the battlefield (CR 305.4 put, the land drop
			// unused), then the Forest beside them.
			for _, id := range []state.ObjID{islandA, islandB, forestID} {
				e.emit(events.Event{Kind: events.MoveZone, Obj: id, Player: 0,
					From: state.ZHand, To: state.ZBattlefield})
				e.pending = nil
			}
			// Marvin ({2}) and the Awaken ({1}, targeting the Forest) cast
			// from the funded pool.
			addMana(t, e, 0, "RR")
			castObj(t, e, ids["Marvin, Murderous Mimic"])
			if animate {
				addMana(t, e, 0, "R")
				// The Awaken targets the Forest (castObj answers the ask with
				// option 0, which here is an Island), so the cast is driven
				// by hand with the Forest's own target option.
				d := e.Pending()
				if d == nil {
					t.Fatal("no decision pending for the Awaken cast")
				}
				idx := -1
				for _, o := range d.Options {
					if o.Kind == "cast" && o.Obj == ids["Test Awaken"] {
						idx = o.Index
					}
				}
				if idx < 0 {
					t.Fatalf("no cast option for the Awaken: %+v", d.Options)
				}
				submitChoices(t, e, idx)
				if d = e.Pending(); d == nil || d.Kind != decision.KTarget {
					t.Fatalf("no target ask for the Awaken: %+v", d)
				}
				tidx := -1
				for _, o := range d.Options {
					if o.Obj == forestID {
						tidx = o.Index
					}
				}
				if tidx < 0 {
					t.Fatalf("the Forest is no Awaken target: %+v", d.Options)
				}
				submitChoices(t, e, tidx)
				passUntilStackEmpty(t, e, 20)
			}
			if o := e.G.Obj(forestID); o != nil && o.Zone != state.ZBattlefield {
				t.Fatalf("Forest zone = %v, want battlefield", o.Zone)
			}
			// Marvin and the Forest entered this turn, so both are
			// summoning sick (CR 302.6) until seat 0's next turn: the census
			// sees their {T} mana abilities from turn 3's main phase.
			driveToStep(t, e, 3, 0, state.StepMain1)
			pm := e.PotentialMana(0)
			var plan *decision.PaymentPlan
			priced := false
			for _, pp := range e.PotentialPaymentPlans(0) {
				if pp.Action.Kind == "cast" && pp.Action.Obj == ids["Glimmerburst"] {
					if pp.Reason == "" && pp.Plan != nil {
						priced = true
						plan = pp.Plan
					}
				}
			}
			assertEveryPriorityOptionActs(t, e)
			if animate {
				if !priced {
					t.Fatalf("Marvin's gained {G} does not price the cast: potential %v", pm)
				}
				tapsMarvin := false
				for _, a := range plan.Activations {
					if a.Source == ids["Marvin, Murderous Mimic"] {
						tapsMarvin = true
					}
				}
				if !tapsMarvin {
					t.Fatalf("the plan does not tap Marvin: %+v", plan.Activations)
				}
				if pm[state.MG] != 2 {
					t.Fatalf("potential {G} = %d, want 2 (the Forest and Marvin's gained one)", pm[state.MG])
				}
			} else {
				if priced {
					t.Fatalf("the plain Forest alone prices the cast: %+v", plan)
				}
				if pm[state.MG] != 1 {
					t.Fatalf("potential {G} = %d, want 1 (the Forest alone)", pm[state.MG])
				}
			}
			replayCheck(t, e, cfg)
		})
	}
}
