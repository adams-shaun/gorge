package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// differentPowerHost is a fakeHost with the ask seam: it records the search
// decision and serves a fixed choice list for it, the way a host that
// bypasses Validate would.
type differentPowerHost struct {
	*fakeHost
	choices []int
	asked   []*decision.Decision
}

func (h *differentPowerHost) AskCount() uint64 { return uint64(len(h.asked)) }

func (h *differentPowerHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.asked = append(h.asked, &cp)
	if h.choices == nil {
		return decision.Intent{}, false
	}
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: h.choices}, true
}

// differentPowerLine is Threats Undetected's search shape (Destination$ Hand
// so the move is observable by zone).
const differentPowerLine = "DB$ ChangeZone | Origin$ Library | Destination$ Hand | ChangeType$ Creature | ChangeNum$ 4 | DifferentPower$ True | Reveal$ True | Shuffle$ False"

// differentPowerBoard builds a library of 2/2, 2/2, 3/3, 4/4 creatures and
// runs the search; choices (nil = unserved) is the host's answer.
func differentPowerBoard(t *testing.T, choices []int) (*differentPowerHost, []state.ObjID) {
	t.Helper()
	h := &differentPowerHost{fakeHost: newHost(t, 2), choices: choices}
	var ids []state.ObjID
	for _, text := range []string{
		"Name:Bear A\nTypes:Creature\nPT:2/2\nOracle:x\n",
		"Name:Bear B\nTypes:Creature\nPT:2/2\nOracle:x\n",
		"Name:Three\nTypes:Creature\nPT:3/3\nOracle:x\n",
		"Name:Four\nTypes:Creature\nPT:4/4\nOracle:x\n",
	} {
		ids = append(ids, h.g.AddObject(mkCard(t, text), 0).ID)
	}
	h.g.SetZone(state.ZLibrary, 0, ids)
	for _, id := range ids {
		h.g.Obj(id).Zone = state.ZLibrary
	}
	src := h.g.AddObject(mkCard(t, "Name:Src\nTypes:Sorcery\nOracle:x\n"), 0)
	h.g.SetZone(state.ZHand, 0, []state.ObjID{src.ID})
	h.g.Obj(src.ID).Zone = state.ZHand
	Resolve(h, &Ctx{Source: src.ID, Controller: 0}, sa(t, differentPowerLine))
	return h, ids
}

func TestChangeZoneDifferentPowerGroupsOptionsByPower(t *testing.T) {
	h, ids := differentPowerBoard(t, nil)
	if len(h.asked) == 0 {
		t.Fatal("the search posed no decision")
	}
	d := h.asked[0]
	if len(d.Options) != 4 {
		t.Fatalf("options = %d, want the four creatures: %+v", len(d.Options), d.Options)
	}
	groups := map[string]int{}
	for _, o := range d.Options {
		if o.Group == "" {
			t.Fatalf("option carries no Group: %+v", o)
		}
		groups[o.Group]++
	}
	if len(groups) != 3 || groups[d.Options[0].Group] != 2 || d.Options[0].Group != d.Options[1].Group {
		t.Fatalf("groups = %v (options %+v), want the two 2/2s sharing one of three groups", groups, d.Options)
	}
	if d.Options[0].Obj != ids[0] || d.Options[1].Obj != ids[1] {
		t.Fatalf("option order does not follow the library: %+v", d.Options)
	}
	if d.Max != 3 {
		t.Fatalf("Max = %d, want 3 (the distinct powers, not the printed 4)", d.Max)
	}
}

func TestChangeZoneDifferentPowerRejectsSamePowerAnswer(t *testing.T) {
	h, _ := differentPowerBoard(t, nil)
	d := h.asked[0]
	same := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index, d.Options[1].Index}}
	if err := d.Validate(same); err == nil {
		t.Fatal("Validate accepted two 2-power picks")
	}
	distinct := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index, d.Options[2].Index, d.Options[3].Index}}
	if err := d.Validate(distinct); err != nil {
		t.Fatalf("Validate refused three distinct powers: %v", err)
	}
}

func TestChangeZoneDifferentPowerApplyDedupesBypass(t *testing.T) {
	h, ids := differentPowerBoard(t, []int{0, 1})
	if got := h.g.Obj(ids[0]).Zone; got != state.ZHand {
		t.Fatalf("first 2/2 in %s, want hand", got)
	}
	if got := h.g.Obj(ids[1]).Zone; got != state.ZLibrary {
		t.Fatalf("second 2/2 in %s, want it left in the library", got)
	}
}
