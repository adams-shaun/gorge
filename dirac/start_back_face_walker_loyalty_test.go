package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// startWalkerTable is vanilla's table plus a transforming planeswalker whose
// front prints loyalty 3 and whose back prints Forge's "Loyalty: 0" carry-over
// marker (the Arlinn Kord shape), and the same walker's front as a standalone
// card.
func startWalkerTable() (tbl *kop.Table, front, back CardID) {
	t := *v.Table
	t.Name = "initnstartwalker"
	front = CardID(len(v.Table.Cards))
	back = front + 1
	t.Cards = append(slices.Clone(v.Table.Cards),
		kop.CardDef{Name: "Test Walker Front", Types: kop.TPlaneswalker, Loyalty: 3,
			FaceMode: kop.FaceTransform, Faces: []CardID{front, back}},
		kop.CardDef{Name: "Test Walker Back", Types: kop.TPlaneswalker, Loyalty: 0,
			FaceMode: kop.FaceTransform, Faces: []CardID{front, back}, Face: 1},
	)
	return &t, front, back
}

// TestInitNStartBackFaceWalkerLoyalty pins the kernel half of gorge's
// setupBackFaceLoyalty convention: a back-face planeswalker whose entering
// face prints "Loyalty: 0" is seated with the front face's printed starting
// loyalty, then the start's listed counters are added on top. The live start's
// engine/rules test pins the same 4.
func TestInitNStartBackFaceWalkerLoyalty(t *testing.T) {
	tbl, front, back := startWalkerTable()
	if f, b := tbl.Cards[front], tbl.Cards[back]; f.Loyalty != 3 || b.Loyalty != 0 {
		t.Fatalf("precondition: walker table is %d/%d, want a 3-loyalty front and a 0-loyalty back", f.Loyalty, b.Loyalty)
	}
	var decks [MaxSeats][]CardID
	decks[0] = startDeck(v.Plains, front)
	decks[1] = startDeck(v.Island, v.GrizzlyBears)
	st := &Start{Seats: []StartSeat{{
		Battlefield: []int{1},
		BackFace:    []int{1},
		Counters:    []StartCounter{{Card: 1, Kind: kop.CtrLoyalty, N: 1}},
	}}}
	s := new(State)
	s.InitNStart(tbl, decks, 2, 3, Config{NoElide: true}, st)
	if got := s.objs[1].card; got != back {
		t.Fatalf("precondition: card %d on the battlefield, want the back face %d", got, back)
	}
	if got := s.Counters(1, kop.CtrLoyalty); got != 4 {
		t.Fatalf("back-face walker loyalty %d, want 4 (front printed 3 + setup counter 1)", got)
	}
}
