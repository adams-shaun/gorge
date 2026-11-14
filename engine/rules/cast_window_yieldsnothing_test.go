package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A convoke creature whose announced taps leave a mana remainder the window
// cannot finish: cost {4}, one convoke body, and one untapped Plains-like
// land beside a commander-identity land that makes nothing (no commander,
// CR 903.4f).
const cynStranderSrc = "Name:Strander\nManaCost:4\nTypes:Creature Elf\nPT:2/2\nK:Convoke\nOracle:x\n"
const cynBodySrc = "Name:Strander Body\nManaCost:1\nTypes:Creature Elf\nPT:1/1\nOracle:x\n"
const cynSunFieldSrc = "Name:Fixture Sun Field\nManaCost:no cost\nTypes:Basic Land Plains\nOracle:x\n"

// TestStrandedConvokeAbortsBeforeAnyManaIsActivated (CR 601.2g-h, 733.1):
// announcing the convoke tap commits a payment the window cannot finish ({3}
// left, the only producing source makes one), so the cast is reversed BEFORE
// any mana is activated: the pool stays empty, the land stays untapped, the
// spell returns to hand. Judging the yieldless source unknown reach left the
// census incomplete, so the strand check deferred, the window opened, the
// land was activated, and the cast then aborted with that residue.
func TestStrandedConvokeAbortsBeforeAnyManaIsActivated(t *testing.T) {
	t.Parallel()
	e, cfg, spell := newFixtureDeck(t, 6309, cynStranderSrc, cynBodySrc, identityLandFixtureSrc, cynSunFieldSrc)
	body := moveSeeded(t, e, 0, cynBodySrc, state.ZBattlefield)
	identity := moveSeeded(t, e, 0, identityLandFixtureSrc, state.ZBattlefield)
	land := moveSeeded(t, e, 0, cynSunFieldSrc, state.ZBattlefield)
	if e.windowSourceYields(0, identity) {
		t.Fatal("precondition: the commander-identity land yields something; the board is wrong")
	}
	if o := e.G.Obj(land); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: the producing land is not an untapped battlefield permanent: %+v", o)
	}
	if o := e.G.Obj(body); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: the convoke body is not an untapped battlefield permanent: %+v", o)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("precondition: pool = %v, want empty", e.G.Players[0].Pool)
	}
	castMode(t, e, spell, "")
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("the convoke ask was not posed: %+v", d)
	}
	var picks []int
	for _, o := range d.Options {
		if o.Kind == "convoke_generic" {
			picks = append(picks, o.Index)
		}
	}
	if len(picks) == 0 {
		t.Fatalf("no convoke contribution offered: %+v", d.Options)
	}
	submitChoices(t, e, picks[0])
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		for _, o := range d.Options {
			if o.Kind == "activate" || o.Kind == "pass" || o.Kind == "done" {
				t.Fatalf("the mana window opened although the payment is stranded: %+v", d.Options)
			}
		}
	}
	if e.cast != nil {
		t.Fatalf("the cast survived a stranded payment: %+v", e.cast)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("reversal left %d mana floating: %v", got, e.G.Players[0].Pool)
	}
	for _, id := range []state.ObjID{body, land} {
		if o := e.G.Obj(id); o.Tapped {
			t.Errorf("reversal left object %d tapped", id)
		}
	}
	if o := e.G.Obj(spell); o.Zone != state.ZHand {
		t.Errorf("the spell ended in %s, want the hand", o.Zone)
	}
	if d := e.Pending(); d != nil && d.Kind != decision.KPriority {
		t.Errorf("pending %+v, want the seat back at priority", d)
	}
	replayCheck(t, e, cfg)
}
