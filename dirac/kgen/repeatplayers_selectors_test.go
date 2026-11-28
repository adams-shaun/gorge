package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRepeatPlayersSelectorsShapes: the RepeatEach RepeatPlayers$ selectors
// lower_repeatselectors.go reads. Keen Duelist (TargetedAndYou: a loop over
// the controller, then one over the targeted opponent), Skull Storm
// (OppNonRememberedController: one loop over RefOppNonRemCtrl) and
// Disorienting Choice (TargetedController: one loop over RefTgtControllers,
// with its per-player GenericChoice, remembered target and StoreSVar counter)
// lower with no refusal and no two-seat mark. Aetherspouts keeps its
// ImprintedOwner reason (read past its first blocker).
func TestRepeatPlayersSelectorsShapes(t *testing.T) {
	cs := lowerNamed(t, []string{"Keen Duelist", "Skull Storm", "Disorienting Choice"})
	by := map[string]*lCard{}
	for _, c := range cs {
		by[c.Name] = c
	}
	for name, want := range map[string][]string{
		"Keen Duelist":        {"You", "Target0"},
		"Skull Storm":         {"OppNonRemCtrl"},
		"Disorienting Choice": {"TgtControllers"},
	} {
		c := by[name]
		if c == nil {
			t.Fatalf("%s not lowered", name)
		}
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s: %s", name, r.Node, r.Reason)
		}
		if c.TwoSeat {
			t.Errorf("%s: two-seat mark set", name)
		}
		var got []string
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				if in.Op == "RepeatEach" {
					got = append(got, in.Obj.String())
				}
			}
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: RepeatEach loops over %v, want %v", name, got, want)
		}
	}
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Aetherspouts\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	p.allBlockers = true
	found := false
	for _, r := range lowerPool(p).cards[0].Refusals {
		found = found || strings.Contains(r.Reason, `RepeatEach RepeatPlayers "ImprintedOwner"`)
	}
	if !found {
		t.Error(`Aetherspouts: the RepeatEach RepeatPlayers "ImprintedOwner" reason is gone`)
	}
}

// TestRepeatPlayersTargetedControllerRefusesStackTargets: a
// RepeatPlayers$ TargetedController whose target slot can name a spell or
// ability refuses by name. The kernel's tgtControllers reads each target's
// live controller off the object record; a stack target is a stack code with
// no object slot, so the loop could not name its controller (lower_repeatselectors.go).
// Disorienting Choice and Guff Rewrites History carry no such slot today, so
// the probe is a synthetic Counterspell-shaped body over the selector.
func TestRepeatPlayersTargetedControllerRefusesStackTargets(t *testing.T) {
	c := lowerScriptCard(t, "Ktgtctrl Probe", "Name:Ktgtctrl Probe\nManaCost:1 U\nTypes:Instant\n"+
		"A:SP$ Counter | Cost$ 1 U | TargetType$ Spell | ValidTgts$ Card | SubAbility$ DBRepeat | SpellDescription$ Counter target spell.\n"+
		"SVar:DBRepeat:DB$ RepeatEach | RepeatPlayers$ TargetedController | RepeatSubAbility$ DBDraw\n"+
		"SVar:DBDraw:DB$ Draw | NumCards$ 1 | Defined$ You\n"+
		"Oracle:Counter target spell. Draw a card for each target's controller.\n")
	found := false
	for _, r := range c.Refusals {
		if strings.Contains(r.Reason, "RepeatEach TargetedController: target slot 0 can name a spell or ability") {
			found = true
		}
	}
	if !found {
		t.Errorf("a TargetedController loop over a spell target did not refuse by name: %v", c.Refusals)
	}
}
