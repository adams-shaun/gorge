package kgen

import (
	"reflect"
	"testing"
)

// TestSplitAfterlifeGrant: each literal Afterlife:N term leaves the list as its
// own instance in order (CR 702.135a, 113.2c); any other spelling of the head
// is refused, as gorge's synthesizer returns no trigger for it.
func TestSplitAfterlifeGrant(t *testing.T) {
	rest, ns, err := splitAfterlifeGrant("Flying & Afterlife:2 & Afterlife:1")
	if err != nil || rest != "Flying" || !reflect.DeepEqual(ns, []int{2, 1}) {
		t.Fatalf("split = %q %v %v, want Flying [2 1]", rest, ns, err)
	}
	if rest, ns, err = splitAfterlifeGrant("Flying"); err != nil || rest != "Flying" || len(ns) != 0 {
		t.Fatalf("no-afterlife list = %q %v %v", rest, ns, err)
	}
	for _, bad := range []string{"Afterlife", "Afterlife:", "Afterlife:0", "Afterlife:X", "Afterlife:01", "Afterlife:21"} {
		if _, _, err := splitAfterlifeGrant(bad); err == nil {
			t.Errorf("splitAfterlifeGrant(%q) did not refuse", bad)
		}
	}
}
