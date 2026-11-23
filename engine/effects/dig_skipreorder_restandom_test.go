package effects

// Hide in Plain Sight pairs SkipReorder$ True with RestRandomOrder$ True
// ("Look at the top five cards of your library, cloak two of them, and put
// the rest on the bottom of your library in a random order"). SkipReorder$
// suppresses the chooser's bottom-order ask; the remainder still moves, and
// RestRandomOrder$ sends it to the library bottom in a random order the
// chooser neither chooses nor sees (CR 401.4 -- the unordered pile; CR 701.58
// cloak). These tests pin that pairing: the untaken cards must leave the
// window, not stay on top in their existing order.

import (
	"slices"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// hipsTapeHost serves the take ask's answer: it takes the first two offered
// cards (the ask is the only one the walk poses -- RestRandomOrder$ poses no
// order ask).
type hipsTapeHost struct {
	*fakeHost
	askKinds []string
}

func (h *hipsTapeHost) AskCount() uint64 { return uint64(len(h.askKinds)) }

func (h *hipsTapeHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	h.askKinds = append(h.askKinds, d.ResumeKind)
	if d.ResumeKind != "dig" || len(d.Options) < 2 {
		return decision.Intent{}, false
	}
	return decision.Intent{Choices: []int{0, 1}}, true
}

const hipsDigSA = "SP$ Dig | Defined$ You | DigNum$ 5 | SkipReorder$ True | ChangeNum$ 2 | DestinationZone$ Library | LibraryPosition$ 0 | RestRandomOrder$ True"

// TestHideInPlainSightDigRestGoesToTheBottomRandomOrder drives the card's
// Dig line: after the answered two-card take, the three untaken cards go to
// the library bottom in a random order instead of staying on top. A sixth
// card sits UNDER the window, so "the rest stays where it is" and "the rest
// goes to the bottom" differ in position.
func TestHideInPlainSightDigRestGoesToTheBottomRandomOrder(t *testing.T) {
	h := &hipsTapeHost{fakeHost: newHost(t, 2)}
	ids := make([]state.ObjID, 0, 6)
	for i := 0; i < 6; i++ {
		ids = append(ids, h.g.AddObject(mkCard(t, "Name:Win"+string(rune('A'+i))+"\nTypes:Sorcery\nOracle:x\n"), 0).ID)
	}
	h.g.SetZone(state.ZLibrary, 0, ids)
	Resolve(h, &Ctx{Controller: 0}, sa(t, hipsDigSA))
	// Precondition: the take ask ran and was answered with two picks.
	if !slices.Equal(h.askKinds, []string{"dig"}) {
		t.Fatalf("asks = %v, want one take ask (\"dig\")", h.askKinds)
	}
	lib := h.g.Zone(state.ZLibrary, 0)
	if len(lib) != 6 {
		t.Fatalf("library = %v, want all six cards", lib)
	}
	// The two taken cards went back to the library top (LibraryPosition$ 0).
	head := append([]state.ObjID(nil), lib[:2]...)
	sort.Slice(head, func(i, j int) bool { return head[i] < head[j] })
	taken := []state.ObjID{ids[0], ids[1]}
	if !slices.Equal(head, taken) {
		t.Fatalf("library head = %v, want the two taken cards %v on top (LibraryPosition$ 0)", head, taken)
	}
	// Under the taken cards sits the card the window never held (it never
	// moved); below it, the three untaken cards in a random order (CR 401.4)
	// -- asserted as a set, since the seeded shuffle may keep any of the 3!
	// orders.
	if lib[2] != ids[5] {
		t.Fatalf("library[2] = %d, want %d (the card under the window; the untaken cards must have LEFT the top)", lib[2], ids[5])
	}
	want := append([]state.ObjID(nil), ids[2:5]...)
	sort.Slice(want, func(i, j int) bool { return want[i] < want[j] })
	got := append([]state.ObjID(nil), lib[3:]...)
	sort.Slice(got, func(i, j int) bool { return got[i] < got[j] })
	if !slices.Equal(got, want) {
		t.Fatalf("library tail = %v, want the three untaken cards %v on the BOTTOM (RestRandomOrder$ True; CR 401.4)", got, want)
	}
}
