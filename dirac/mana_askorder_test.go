package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestColourAskOrderFollowsPrintedCombo: the window's colour answer must read
// the colour list in the order gorge's askManaColor builds it. A printed
// Combo ("Combo B G U", Opulent Palace; the TDM Devotees) lists its colours in
// printed order, so the first owed colour is its printed first colour, not the
// lowest WUBRG index:
func TestColourAskOrderFollowsPrintedCombo(t *testing.T) {
	// "Combo B G U": three bits per printed position, B=2, G=4, U=1.
	ord := uint16(kop.B) | uint16(kop.G)<<3 | uint16(kop.U)<<6
	ab := &kop.Ability{
		ManaColors: (1 << kop.B) | (1 << kop.G) | (1 << kop.U),
		ManaColOrd: ord,
	}
	if ab.ManaColOrd == 0 {
		t.Fatal("precondition: ManaColOrd must record B G U's printed order")
	}
	var out [kop.NumManaKinds]uint8
	n, ok := colourAskOrder(ab, uint8(ab.ManaColors), &out)
	if !ok {
		t.Fatal("colourAskOrder(B G U) reported the WUBRG fallback")
	}
	want := []uint8{kop.B, kop.G, kop.U}
	if n != len(want) {
		t.Fatalf("colourAskOrder = %v (n=%d), want %v", out[:n], n, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("colourAskOrder = %v, want %v", out[:n], want)
		}
	}
	// The failure this guards: the WUBRG-lowest colour is U, but gorge's ask
	// starts with B.
	if out[0] == uint8(kop.U) {
		t.Fatal("precondition: the printed order must differ from WUBRG lowest")
	}

	// A Combo already in WUBRG order (ManaColOrd zero) and a mask with {C}
	// keep the caller's W U B R G C loop: ok is false, no order to carry.
	wab := &kop.Ability{ManaColors: (1 << kop.W) | (1 << kop.B) | (1 << kop.G)}
	if n, ok = colourAskOrder(wab, uint8(wab.ManaColors)|1<<kop.C, &out); ok || n != 0 {
		t.Fatalf("colourAskOrder(W B G C) = (n=%d, ok=%v), want the WUBRG fallback", n, ok)
	}

	// No ability shape read (a granted ability): the same fallback.
	if n, ok = colourAskOrder(nil, uint8(ab.ManaColors), &out); ok || n != 0 {
		t.Fatalf("colourAskOrder(nil) = (n=%d, ok=%v), want the WUBRG fallback", n, ok)
	}

	// Temur Devotee's "Combo G U R": printed G, U, R; WUBRG lowest is U.
	tord := uint16(kop.G) | uint16(kop.U)<<3 | uint16(kop.R)<<6
	tab := &kop.Ability{
		ManaColors: (1 << kop.G) | (1 << kop.U) | (1 << kop.R),
		ManaColOrd: tord,
	}
	n, ok = colourAskOrder(tab, uint8(tab.ManaColors), &out)
	if !ok {
		t.Fatal("colourAskOrder(Temur Devotee) reported the WUBRG fallback")
	}
	want = []uint8{kop.G, kop.U, kop.R}
	if n != len(want) {
		t.Fatalf("colourAskOrder(Temur Devotee) = %v (n=%d), want %v", out[:n], n, want)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Fatalf("colourAskOrder(Temur Devotee) = %v, want %v", out[:n], want)
		}
	}
}
