package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDamageTargetNamesDamageProofPermanents: a damage target ask lists the
// battlefield permanents that prevent all damage to themselves with no
// counter rider, so the bot can leave them last. A permanent whose prevention
// spends a counter, or only stops combat damage, still takes the hit's worth
// and is not listed.
func TestDamageTargetNamesDamageProofPermanents(t *testing.T) {
	t.Parallel()
	e := newSeats(t, 2)
	put := func(text string, p state.PlayerID) state.ObjID {
		o := e.G.AddObject(card(t, text), p)
		e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZBattlefield})
		return o.ID
	}
	proof := put("Name:Mirror Warden\nTypes:Creature Horror\nPT:4/4\n"+
		"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidTarget$ Card.Self | ReplaceWith$ WardenBounce | PreventionEffect$ True | Description$ Damage to this is stopped and sent elsewhere.\n"+
		"SVar:WardenBounce:DB$ ImmediateTrigger | Execute$ WardenHit | TriggerDescription$ It strikes back.\n"+
		"SVar:WardenHit:DB$ DealDamage | ValidTgts$ Any | NumDmg$ 1\nOracle:x\n", 1)
	counters := put("Name:Fading Ghost\nTypes:Creature Spirit\nPT:0/0\n"+
		"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidTarget$ Card.Self | ReplaceWith$ GhostFade | PreventionEffect$ True | Description$ Damage spends a counter instead.\n"+
		"SVar:GhostFade:DB$ RemoveCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1\nOracle:x\n", 1)
	combatOnly := put("Name:Parade Shield\nTypes:Creature Soldier\nPT:2/2\n"+
		"R:Event$ DamageDone | ActiveZones$ Battlefield | ValidTarget$ Card.Self | IsCombat$ True | PreventionEffect$ True | Description$ Combat damage to this is stopped.\nOracle:x\n", 1)
	bolt := e.G.AddObject(card(t, "Name:Test Spark\nManaCost:R\nTypes:Instant\nA:SP$ DealDamage | ValidTgts$ Any | NumDmg$ 2\nOracle:x\n"), 0)
	e.emit(events.Event{Kind: events.PutOnStack, Obj: bolt.ID, Player: 0, From: state.ZLibrary, To: state.ZStack, Text: "proof fixture"})
	e.askTarget(0, bolt.ID, e.G.Obj(bolt.ID).Face().SpellAbility())
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.TargetEffect == nil || d.TargetEffect.Damage == nil {
		t.Fatalf("fixture precondition: damage target ask = %+v", d)
	}
	got := d.TargetEffect.Damage.Prevented
	if !slices.Contains(got, proof) {
		t.Fatalf("Prevented = %v, want the all-damage preventer %d", got, proof)
	}
	if slices.Contains(got, counters) || slices.Contains(got, combatOnly) {
		t.Fatalf("Prevented = %v lists a counter-rider (%d) or combat-only (%d) preventer", got, counters, combatOnly)
	}
}
