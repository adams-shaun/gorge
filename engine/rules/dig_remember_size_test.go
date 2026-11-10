package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// expandTheSphere is the Forge script's rules lines, inline.
const expandTheSphere = "Name:Expand the Sphere\nManaCost:3 G\nTypes:Sorcery\n" +
	"A:SP$ Dig | DigNum$ 6 | ChangeNum$ 2 | Optional$ True | ForceRevealToController$ True | ChangeValid$ Land | DestinationZone$ Battlefield | Tapped$ True | RestRandomOrder$ True | RememberChanged$ True | SubAbility$ DBProl\n" +
	"SVar:DBProl:DB$ Proliferate | Amount$ Number$2/Minus.Y | StackDescription$ None | SubAbility$ DBClear\n" +
	"SVar:DBClear:DB$ Cleanup | ClearRemembered$ True\n" +
	"SVar:Y:Count$RememberedSize\nOracle:x\n"

// digRememberSizeRun casts Expand the Sphere, takes picks lands from the Dig,
// and returns how many proliferate asks followed (each answered empty).
func digRememberSizeRun(t *testing.T, picks int) int {
	t.Helper()
	e, cfg, id := newFixtureDeck(t, 4, expandTheSphere)
	e.emit(events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: 1})
	e.priorityRound()
	addMana(t, e, 0, "GGGG")
	d := castFixture(t, e, id, -1)
	if d == nil || d.ResumeKind != "dig" || len(d.Options) < 2 {
		t.Fatalf("first decision = %+v, want the Dig choose", d)
	}
	var idx []int
	for i := 0; i < picks; i++ {
		idx = append(idx, d.Options[i].Index)
	}
	submitChoices(t, e, idx...)
	asks := 0
	for asks < 5 {
		d = e.Pending()
		if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "proliferate" {
			break
		}
		asks++
		submitChoices(t, e)
	}
	lands := 0
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(oid); o != nil && o.Face() != nil && o.Face().Name == "Mountain" {
			lands++
		}
	}
	if lands != picks {
		t.Fatalf("%d Mountains on the battlefield, want %d", lands, picks)
	}
	replayCheck(t, e, cfg)
	return asks
}

func TestDigRememberChangedFeedsRememberedSize2(t *testing.T) {
	t.Parallel()
	if n := digRememberSizeRun(t, 2); n != 0 {
		t.Fatalf("%d proliferate asks after two lands, want 0", n)
	}
}

func TestDigRememberChangedFeedsRememberedSize1(t *testing.T) {
	t.Parallel()
	if n := digRememberSizeRun(t, 1); n != 1 {
		t.Fatalf("%d proliferate asks after one land, want 1", n)
	}
}

func TestDigRememberChangedFeedsRememberedSize0(t *testing.T) {
	t.Parallel()
	if n := digRememberSizeRun(t, 0); n != 2 {
		t.Fatalf("%d proliferate asks after no lands, want 2", n)
	}
}
