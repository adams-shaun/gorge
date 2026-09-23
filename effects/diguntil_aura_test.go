package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestDigUntilAuraEntryAsksForBearer(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	bear1 := h.g.AddObject(mkCard(t, "Name:Bear One\nTypes:Creature\nPT:2/2\nOracle:x\n"), 0).ID
	bear2 := h.g.AddObject(mkCard(t, "Name:Bear Two\nTypes:Creature\nPT:3/3\nOracle:x\n"), 0).ID
	aura := h.g.AddObject(mkCard(t, "Name:Halo\nManaCost:W\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"), 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{bear1, bear2})
	h.g.SetZone(state.ZLibrary, 0, []state.ObjID{aura})

	// PRECONDITION: both distinct battlefield creatures satisfy Enchant:Creature.
	first, isAura := auraEntryBearer(h.g, aura, 0)
	if !isAura || first != bear1 || !MatchesSpecFrom(h.g, "Creature", bear1, 0, aura) ||
		!MatchesSpecFrom(h.g, "Creature", bear2, 0, aura) || bear1 == bear2 {
		t.Fatalf("setup: eligible Aura bearers = [%d %d], want two distinct creatures", first, bear2)
	}
	ability := sa(t, "SP$ DigUntil | Valid$ Aura | FoundDestination$ Battlefield")
	Resolve(h, &Ctx{Controller: 0}, ability)
	if h.asked == nil || h.asked.Kind != decision.KChoose || h.asked.ResumeKind != "diguntil_aura" {
		t.Fatalf("pending Aura bearer decision = %+v, want KChoose/diguntil_aura", h.asked)
	}
	if len(h.asked.Options) != 2 || h.asked.Options[0].Obj != bear1 || h.asked.Options[1].Obj != bear2 {
		t.Fatalf("Aura bearer options = %+v, want [%d %d]", h.asked.Options, bear1, bear2)
	}
	if h.g.Obj(aura).Zone != state.ZLibrary {
		t.Fatalf("Aura moved before its bearer was chosen: zone %s", h.g.Obj(aura).Zone)
	}

	// Simulate the engine's answered continuation, choosing the second
	// candidate. The effect must consume the answer and attach to that object,
	// not silently take option zero.
	ctx := &Ctx{Controller: 0, DigUntilAuraBearer: bear2, DigUntilAuraDone: true}
	Resolve(h, ctx, ability)
	if ctx.DigUntilAuraBearer != 0 || ctx.DigUntilAuraDone {
		t.Fatal("Aura bearer answer was not consumed on re-entry")
	}
	if o := h.g.Obj(aura); o.Zone != state.ZBattlefield || o.AttachedTo != bear2 {
		t.Fatalf("Aura zone/attachment = %s/%d, want battlefield/%d", o.Zone, o.AttachedTo, bear2)
	}
}

func TestDigUntilWithholdsUnsupportedParamsAndStillMoves(t *testing.T) {
	h, ids := digUntilFixture(t)
	ability := sa(t, "SP$ DigUntil | Valid$ Aura | Amount$ X | DigZone$ PlanarDeck | NoMoveFound$ True | FoundLibraryPosition$ 0 | Shuffle$ True | ShuffleCondition$ NoneFound | ImprintFound$ True | ImprintRevealed$ True | NoneFoundDestination$ Library | NoneFoundLibraryPosition$ 0 | FoundDestination$ Hand | RevealedDestination$ Graveyard")

	// PRECONDITION: the matching Aura is in the scanned library, and its
	// destination differs from the library so a no-op implementation fails.
	if h.g.Obj(ids[1]).Zone != state.ZLibrary {
		t.Fatalf("setup: matching Aura is not in the library: %s", h.g.Obj(ids[1]).Zone)
	}
	Resolve(h, &Ctx{Controller: 0}, ability)
	if h.g.Obj(ids[1]).Zone != state.ZHand {
		t.Fatalf("unsupported params prevented the core move: Aura zone = %s, want hand", h.g.Obj(ids[1]).Zone)
	}
	want := []string{"Amount$ X", "DigZone$ PlanarDeck", "NoMoveFound$ True", "FoundLibraryPosition$ 0", "Shuffle$ True", "ShuffleCondition$ NoneFound", "ImprintFound$ True", "ImprintRevealed$ True", "NoneFoundDestination$ Library", "NoneFoundLibraryPosition$ 0"}
	var notes []string
	for _, ev := range h.log {
		if strings.HasPrefix(ev.Text, "DigUntil withholds ") {
			notes = append(notes, ev.Text)
		}
	}
	if len(notes) != len(want) {
		t.Fatalf("withheld Notes = %d, want one per unsupported parameter %v: %v", len(notes), want, notes)
	}
	for _, param := range want {
		found := false
		for _, note := range notes {
			if strings.Contains(note, "withholds "+param+";") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing withheld-param Note for %s: %v", param, notes)
		}
	}
}
