package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestLeylineTyrantDeathXNamesRedMana pins XAnnounceTitle$ on a triggered
// "you may pay any amount of {R}" (Leyline Tyrant's dies trigger, an
// ImmediateTrigger whose Cost$ X has XColor$ R): the X announcement's prompt
// names what X counts, Forge's announce title, and the offered X values are
// bounded by the red sources.
func TestLeylineTyrantDeathXNamesRedMana(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, _ := searchEngine(t, reg, "Leyline Tyrant")
	id := searchMoveByName(t, e, "Leyline Tyrant", state.ZBattlefield)
	for i := 0; i < 3; i++ {
		xFoldLand(t, e, "Test Mountain "+string(rune('A'+i)), "R")
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	var d *decision.Decision
	for i := 0; i < 6; i++ {
		d = passUntilNonPriority(t, e, 40)
		if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind == "trigger_cost_x" {
			break
		}
		if d.Options[0].Kind != "trigger_cost_pay" {
			break
		}
		answerAsk(t, e, 0)
	}
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "trigger_cost_x" {
		t.Fatalf("expected the choose-X announcement ask, got %+v", d)
	}
	if !strings.Contains(d.Prompt, "any amount of red mana to pay") {
		t.Errorf("prompt %q does not name the XAnnounceTitle$", d.Prompt)
	}
	if last := d.Options[len(d.Options)-1].Amount; last != 3 {
		t.Errorf("largest X offered %d, want 3 (three red sources)", last)
	}
}
