package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestIrencragEverflameEquippedGrantsClassKeywords: The Irencrag transformed
// into Everflame, Heroes' Legacy is an Equipment only by its layer-4 derived
// type line (CR 613.1d); the creature it is attached to is equipped (CR
// 301.5a), so Blacksmith's Talent level 3 ("equipped creatures you control
// have double strike and haste") applies to it.
func TestIrencragEverflameEquippedGrantsClassKeywords(t *testing.T) {
	t.Parallel()
	crag := corpusAlternativeCard(t, "The Irencrag")
	talent := corpusAlternativeCard(t, "Blacksmith's Talent")
	legend := cardByName(t, "Name:Grant Test Legend\nTypes:Legendary Creature Human\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGame(t, 9, crag, legend, talent)
	cragID := moveSeededCard(t, e, 0, crag, state.ZBattlefield)
	legendID := moveSeededCard(t, e, 0, legend, state.ZBattlefield)
	addMana(t, e, 0, "")
	settle := func() {
		t.Helper()
		for i := 0; i < 40; i++ {
			d := e.Pending()
			if d == nil {
				e.Advance()
				continue
			}
			if d.Kind == decision.KPriority {
				if len(e.G.Stack) == 0 {
					break
				}
				passOnce(t, e)
				continue
			}
			// The Optional$ "become Everflame" question: answer yes.
			idx := d.Options[0].Index
			for _, o := range d.Options {
				if strings.EqualFold(o.Kind, "yes") || strings.EqualFold(o.Label, "yes") {
					idx = o.Index
				}
			}
			submitChoices(t, e, idx)
		}
	}
	settle()
	if got := e.Name(cragID); got != "Everflame, Heroes' Legacy" {
		t.Fatalf("precondition: The Irencrag did not become Everflame (name %q)", got)
	}
	// The Class enters after the Irencrag settled, so its own enters
	// trigger (the Sword token) resolves on an otherwise empty stack.
	talentID := moveSeededCard(t, e, 0, talent, state.ZBattlefield)
	settle()
	e.emit(events.Event{Kind: events.ClassLevelChange, Obj: talentID, Amount: 1})
	e.emit(events.Event{Kind: events.ClassLevelChange, Obj: talentID, Amount: 1})
	if got := e.G.Obj(talentID).ClassLevel(); got != 3 {
		t.Fatalf("precondition: Blacksmith's Talent level = %d, want 3", got)
	}
	if slices.Contains(e.G.Obj(cragID).Face().Types, "Equipment") {
		t.Fatalf("precondition: Everflame's printed types %v already name Equipment", e.G.Obj(cragID).Face().Types)
	}
	derived := false
	for _, ot := range e.EffectiveTypes() {
		if ot.ID == cragID && slices.Contains(ot.Types, "Equipment") {
			derived = true
		}
	}
	if !derived {
		t.Fatalf("precondition: Everflame's layer-4 types lack Equipment: %+v", e.EffectiveTypes())
	}
	if e.HasKeyword(legendID, "Double Strike") || e.HasKeyword(legendID, "Haste") {
		t.Fatalf("precondition: the unequipped legend already has double strike/haste")
	}
	e.emit(events.Event{Kind: events.Attach, Obj: cragID, IDs: []state.ObjID{legendID}})
	if e.G.Obj(cragID).AttachedTo != legendID {
		t.Fatalf("precondition: Everflame is not attached to the legend")
	}
	ds, haste := e.HasKeyword(legendID, "Double Strike"), e.HasKeyword(legendID, "Haste")
	if !ds || !haste {
		t.Fatalf("Everflame-equipped creature lacks double strike/haste (CR 301.5a, 613.1d): ds=%v haste=%v", ds, haste)
	}
	replayCheck(t, e, cfg)
}
