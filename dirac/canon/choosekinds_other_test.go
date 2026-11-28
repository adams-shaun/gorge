package canon

import (
	"math"
	"strings"
	"testing"
)

// TestChooseOtherIsNoListedKind: the code of an option kind ChooseKinds lacks
// is past the list, so it never reads as a listed kind's choose action (127
// was "exert"), and its action kind still fits the wire form's kind byte.
func TestChooseOtherIsNoListedKind(t *testing.T) {
	code, ok := ChooseCode("no_such_kind")
	if ok || code != ChooseOther {
		t.Fatalf("ChooseCode(unlisted) = %d,%v, want %d,false", code, ok, ChooseOther)
	}
	if ChooseOther < len(ChooseKinds) {
		t.Errorf("ChooseOther = %d is the code of listed kind %q (ChooseKinds has %d entries)", ChooseOther, ChooseKinds[ChooseOther], len(ChooseKinds))
	}
	if got := int(ActChoose) + ChooseOther; got > math.MaxUint8 {
		t.Errorf("ActChoose+ChooseOther = %d does not fit the repro wire form's kind byte", got)
	}
	got := Pack(ActChoose+ChooseOther, RefNone, RefNone, 1).String()
	if !strings.HasPrefix(got, "choose:other(") {
		t.Errorf("ActChoose+ChooseOther renders as %q, want choose:other", got)
	}
	exert, ok := ChooseCode("exert")
	if !ok || exert == code {
		t.Errorf("ChooseCode(exert) = %d,%v, want a listed code apart from choose:other %d", exert, ok, code)
	}
}
