package canon

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestChooseKindsOptInSeat: the named-seat yes / no kinds are appended at the
// tail of ChooseKinds, so no existing kind's code (ActChoose+i, a recorded
// action kind) moves, and the generic yes / no keep theirs.
func TestChooseKindsOptInSeat(t *testing.T) {
	// absolute codes: an append elsewhere in the list (a later kind landing
	// before or after these) moves none of them
	for name, want := range map[string]uint8{"x": 0, "yes": 9, "no": 10, "keep": 11, "asunblocked": 118} {
		if got, ok := ChooseCode(name); !ok || got != want {
			t.Errorf("ChooseCode(%q) = %d,%v, want %d (an append moved it)", name, got, ok, want)
		}
	}
	// the named-seat pair is appended after every kind that existed before it
	// (asunblocked), adjacent, and inside the option-kind range
	last, _ := ChooseCode("asunblocked")
	yes, okY := ChooseCode("yes_seat")
	no, okN := ChooseCode("no_seat")
	if !okY || !okN || yes <= last || no != yes+1 || int(no) >= ChooseOther {
		t.Fatalf("yes_seat %d,%v and no_seat %d,%v: want both listed, after asunblocked %d, adjacent, below choose:other %d", yes, okY, no, okN, last, ChooseOther)
	}
	seen := map[string]bool{}
	for _, k := range ChooseKinds {
		if seen[k] {
			t.Errorf("kind %q is listed twice", k)
		}
		seen[k] = true
	}
}

// TestChooseKindsOptInSeatRoundTrip: both kinds round trip through the kernel's
// translation and the gorge-option name the adapter reads them back as, and
// are never the generic yes / no.
func TestChooseKindsOptInSeatRoundTrip(t *testing.T) {
	yes, _ := ChooseCode("yes_seat")
	no, _ := ChooseCode("no_seat")
	gy, _ := ChooseCode("yes")
	gn, _ := ChooseCode("no")
	if yes == gy || no == gn || yes == no {
		t.Fatalf("codes yes_seat %d, no_seat %d collide with yes %d, no %d", yes, no, gy, gn)
	}
	s := new(dirac.State)
	d := &dirac.Decision{Kind: dirac.DkChoose, Sub: dirac.ChGenOptInSeat, Player: 2, N: 2}
	d.Options[0] = dirac.MakeAction(dirac.AkChoose, 0, 0, 1)
	d.Options[1] = dirac.MakeAction(dirac.AkChoose, 0, 0, 0)
	tr := &Translator{S: s, D: d, KP: new(dirac.Projection), Generated: true}
	for i, c := range []struct {
		kind  uint8
		name  string
		gorge string
	}{{yes, "yes_seat", "yes"}, {no, "no_seat", "no"}} {
		got := tr.Action(d.Options[i])
		want := Pack(ActChoose+CanonKind(c.kind), RefNone, RefNone, 0)
		if got != want || got != OptInSeatCanon(i == 0) {
			t.Fatalf("option %d = %#x, want %#x (%s)", i, got, want, c.name)
		}
		if name := ChooseKinds[got.Kind()-ActChoose]; name != c.name {
			t.Fatalf("option %d reads back as kind %q, want %q", i, name, c.name)
		}
		if g, ok := OptInSeatName(c.name); !ok || g != c.gorge {
			t.Fatalf("OptInSeatName(%q) = %q,%v, want %q", c.name, g, ok, c.gorge)
		}
	}
	if _, ok := OptInSeatName("yes"); ok {
		t.Fatal("the generic yes reads as a named-seat kind")
	}
}
