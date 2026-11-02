package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestDelayedDestroyAndSacrificeFindOnlyTheRememberedObject extends CR
// 400.7's delayed-trigger rule from ChangeZone to the Destroy and
// SacrificeAll movers: "destroy/sacrifice that creature at the beginning of
// the next end step" (Goblin Kites, Spinal Embrace, Berserk) remembers the
// creature, and if it left the battlefield and came back before the end step
// it is a new object the delayed trigger cannot find. The control arm (no
// round trip) is destroyed / sacrificed.
func TestDelayedDestroyAndSacrificeFindOnlyTheRememberedObject(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		"DB$ Destroy | Defined$ DelayTriggerRememberedLKI",
		"DB$ Destroy | Defined$ DelayTriggerRemembered",
		"DB$ SacrificeAll | Defined$ DelayTriggerRememberedLKI",
	} {
		for _, roundTrip := range []bool{false, true} {
			src := "Name:Doom Later\nManaCost:B\nTypes:Sorcery\n" +
				"A:SP$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | ValidTgts$ Creature | RememberObjects$ Targeted | Execute$ TrigDoom | SpellDescription$ x.\n" +
				"SVar:TrigDoom:" + body + "\nOracle:x\n"
			e := handEngine(t, card(t, src), corpusAlternativeCard(t, "Grizzly Bears"))
			hand := e.G.Zone(state.ZHand, 0)
			spell, bears := hand[0], hand[1]
			placeOnBattlefield(t, e, bears)
			so := e.G.Obj(spell)
			effects.Resolve(e, &effects.Ctx{Source: spell, Controller: 0, SVars: so.Face().SVars,
				Targets: []state.Target{{Obj: bears}}}, so.Face().SpellAbility())
			if len(e.G.Delayed) != 1 {
				t.Fatalf("%s: %d delayed triggers registered, want 1", body, len(e.G.Delayed))
			}
			if roundTrip {
				e.emit(events.Event{Kind: events.MoveZone, Obj: bears, From: state.ZBattlefield, To: state.ZExile})
				e.emit(events.Event{Kind: events.MoveZone, Obj: bears, From: state.ZExile, To: state.ZBattlefield})
			}
			e.pending = nil
			e.priorityRound()
			passUntil(t, e, func() bool {
				return len(e.G.Delayed) == 0 && len(e.G.Stack) == 0 && e.Pending() != nil
			})
			want := state.ZGraveyard
			if roundTrip {
				want = state.ZBattlefield
			}
			if z := e.G.Obj(bears).Zone; z != want {
				t.Fatalf("%s roundTrip=%v: Bears in %v, want %v", body, roundTrip, z, want)
			}
		}
	}
}
