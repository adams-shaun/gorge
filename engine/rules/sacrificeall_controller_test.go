package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSacrificeAllController: Controller$ You on a SacrificeAll keeps only the
// objects the ability's controller controls (Forge's SacrificeAllEffect
// filters by Controller$ after the LKI refresh). Crumbling Colossus, Fog
// Elemental, Mardu Blazebringer and Runaway Carriage ("DB$ SacrificeAll |
// Defined$ Self | Controller$ You") must not sacrifice the source when another
// player controls it; the sweep and Defined$ spellings both honour the filter.
func TestSacrificeAllController(t *testing.T) {
	t.Parallel()
	bodies := []string{
		"DB$ SacrificeAll | ValidCards$ Creature | Controller$ You",
		"DB$ SacrificeAll | Defined$ Targeted | Controller$ You",
	}
	for _, body := range bodies {
		for _, ctl := range []string{"", " | Controller$ You"} {
			b := body
			if ctl == "" {
				b = body[:len(body)-len(" | Controller$ You")] // control arm: no filter
			}
			src := "Name:Doom Now\nManaCost:B\nTypes:Sorcery\nA:SP$ Pump | Defined$ Self | SubAbility$ Doom | SpellDescription$ x.\n" +
				"SVar:Doom:" + b + "\nOracle:x\n"
			e := handEngine(t, card(t, src), corpusAlternativeCard(t, "Grizzly Bears"), corpusAlternativeCard(t, "Grizzly Bears"))
			hand := e.G.Zone(state.ZHand, 0)
			spell, mine, stolen := hand[0], hand[1], hand[2]
			placeOnBattlefield(t, e, mine)
			placeOnBattlefield(t, e, stolen)
			e.emit(events.Event{Kind: events.ControlChange, Obj: stolen, Player: 1})
			if got := e.G.Obj(stolen).Controller; got != 1 {
				t.Fatalf("%s: precondition: stolen Bears controlled by %d, want 1", b, got)
			}
			so := e.G.Obj(spell)
			effects.Resolve(e, &effects.Ctx{Source: spell, Controller: 0, SVars: so.Face().SVars,
				Targets: []state.Target{{Obj: mine}, {Obj: stolen}}}, so.Face().SpellAbility())
			if z := e.G.Obj(mine).Zone; z != state.ZGraveyard {
				t.Errorf("%s: own Bears in %v, want graveyard", b, z)
			}
			want := state.ZBattlefield
			if ctl == "" {
				want = state.ZGraveyard
			}
			if z := e.G.Obj(stolen).Zone; z != want {
				t.Errorf("%s: stolen Bears in %v, want %v", b, z, want)
			}
		}
	}
}
