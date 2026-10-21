package events

import (
	"reflect"
	"testing"
)

// TestCopyExceptionsRoundTrip: Text and ParseCopyExceptions are inverses,
// and a malformed payload fails closed.
func TestCopyExceptionsRoundTrip(t *testing.T) {
	x := CopyExceptions{Types: []string{"Creature", "Dragon", "Zombie"}, TypesSet: true,
		Power: 4, Toughness: 4, PTSet: true, Keywords: []string{"Haste"}}
	got, ok := ParseCopyExceptions(x.Text())
	if !ok || !reflect.DeepEqual(got, x) {
		t.Fatalf("round trip %+v -> %q -> %+v (ok %v)", x, x.Text(), got, ok)
	}
	if (CopyExceptions{}).Empty() != true || x.Empty() {
		t.Fatal("Empty misreports")
	}
	if _, ok := ParseCopyExceptions("types Creature"); ok {
		t.Fatal("a malformed payload parsed")
	}
}
