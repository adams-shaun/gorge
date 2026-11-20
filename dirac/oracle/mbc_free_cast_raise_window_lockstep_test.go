package oracle

import "testing"

// precondition: the recording's pool is the one the evidence loop dealt, so
// the game above is the pair the ticket named.
func TestFreeCastRaiseWindowReproPool(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/free_cast_raise_window.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "dr-mbc-baaa" {
		t.Fatalf("precondition: repro pool %q, want dr-mbc-baaa", r.Pool)
	}
}
