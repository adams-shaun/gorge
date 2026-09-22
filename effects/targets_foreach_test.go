package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// TestTargetsForEachPlayerSubAskGroupsPerController pins the effect-side
// pre-ask's TargetsForEachPlayer$ handling (pfpe1; the mvts1 round
// deliberately skipped the shape): a depth-2 SubAbility$ carrier (Kaya,
// Spirits' Justice's exile-each, measured one of exactly 4 sub-only corpus
// carriers) poses its ask with the OneEach bounds read against the
// distinct-controller count and each option bound to its controller's Group
// -- the same label rules' ask sites attach -- so Decision.Validate's
// mutual-exclusion rule enforces one pick per controller on the wire.
func TestTargetsForEachPlayerSubAskGroupsPerController(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(3))
	src := h.g.AddObject(mkCard(t, "Name:Asker\nTypes:Sorcery\nOracle:x\n"), 0)
	src.Zone = state.ZHand
	// One opponent carries TWO creatures: the OneEach bound and the group
	// exclusivity are both exercised.
	for _, p := range []state.PlayerID{1, 1, 2} {
		o := h.g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nOracle:x\n"), p)
		o.Zone = state.ZBattlefield
		h.g.SetZone(state.ZBattlefield, p, append(h.g.Zone(state.ZBattlefield, p), o.ID))
	}
	sa := &cards.SA{API: "Destroy", Params: map[string]string{
		"ValidTgts":            "Creature.OppCtrl",
		"TargetMin":            "0",
		"TargetMax":            "OneEach",
		"TargetsForEachPlayer": "True",
	}}
	c := &Ctx{Source: src.ID, Controller: 0}
	_, ok := chosenTargetsFor(h, c, sa, false)
	if !ok || h.asked == nil {
		t.Fatalf("ok=%v asked=%+v, want a posed ask", ok, h.asked)
	}
	d := h.asked
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("bounds = (%d, %d), want (0, 2): TargetMin$ 0 stays literal, OneEach Max is the distinct-controller count", d.Min, d.Max)
	}
	if len(d.Options) != 3 {
		t.Fatalf("options = %+v, want one per opposing creature", d.Options)
	}
	groups := map[state.ObjID]string{}
	for _, o := range d.Options {
		if o.Group == "" {
			t.Fatalf("option %d carries no controller Group", o.Index)
		}
		groups[o.Obj] = o.Group
	}
	if groups[d.Options[0].Obj] != groups[d.Options[1].Obj] || groups[d.Options[0].Obj] == groups[d.Options[2].Obj] {
		t.Fatalf("groups = %v, want one per controller", groups)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 1}}); err == nil {
		t.Fatal("two creatures controlled by one opponent were accepted")
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0, 2}}); err != nil {
		t.Fatalf("one target per controller rejected: %v", err)
	}

	// The TargetMin$ OneEach spelling (4 corpus carriers, e.g. Vaevictis
	// Asmadi the Dire's sacrifice-one-each): Min is the distinct-controller
	// count too.
	sa2 := &cards.SA{API: "Destroy", Params: map[string]string{
		"ValidTgts":            "Creature.OppCtrl",
		"TargetMin":            "OneEach",
		"TargetMax":            "OneEach",
		"TargetsForEachPlayer": "True",
	}}
	c2 := &Ctx{Source: src.ID, Controller: 0}
	h2 := &askHost{}
	h2.g = h.g
	if _, ok := chosenTargetsFor(h2, c2, sa2, false); !ok || h2.asked == nil {
		t.Fatalf("ok=%v asked=%+v, want a posed ask", ok, h2.asked)
	}
	if h2.asked.Min != 2 || h2.asked.Max != 2 {
		t.Fatalf("OneEach bounds = (%d, %d), want (2, 2)", h2.asked.Min, h2.asked.Max)
	}
}
