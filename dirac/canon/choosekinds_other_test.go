package canon

import (
	"strings"
	"testing"
)

// TestChooseOtherIsNoListedKind: the code of an option kind ChooseKinds lacks
// is past the list, so it never reads as a listed kind's choose action (127
// was "exert"), and its action kind stays below ActNone, so an unlisted ask
// never equals an untranslated action.
func TestChooseOtherIsNoListedKind(t *testing.T) {
	code, ok := ChooseCode("no_such_kind")
	if ok || code != ChooseOther {
		t.Fatalf("ChooseCode(unlisted) = %d,%v, want %d,false", code, ok, ChooseOther)
	}
	if ChooseOther < len(ChooseKinds) {
		t.Errorf("ChooseOther = %d is the code of listed kind %q (ChooseKinds has %d entries)", ChooseOther, ChooseKinds[ChooseOther], len(ChooseKinds))
	}
	if got := int(ActChoose) + ChooseOther; got >= int(ActNone) {
		t.Errorf("ActChoose+ChooseOther = %d is not below the ActNone sentinel %d", got, ActNone)
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
