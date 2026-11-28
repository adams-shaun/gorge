package gorged

import (
	"net/http/httptest"
	"testing"

	"github.com/adams-shaun/gorge/server/host"
)

// TestReapedTableDropsDecksAndTokens: once the registry reaps a finished
// on-demand table, the hook serve binds releases that table's submitted decks
// and seat tokens, and leaves every other table's alone.
func TestReapedTableDropsDecksAndTokens(t *testing.T) {
	imp := newDeckImports(nil)
	for _, id := range []string{"import-g1-s0", "import-g1-s1", "import-g1-s3", "import-g12-s0", "import-g2-s0"} {
		imp.decks[id] = host.Deck{}
	}
	gate, err := newSeatGate("", "t1", nil)
	if err != nil {
		t.Fatal(err)
	}
	old, err := gate.mint("g1", 0)
	if err != nil {
		t.Fatal(err)
	}
	keep, err := gate.mint("g2", 0)
	if err != nil {
		t.Fatal(err)
	}
	resolves := func(tok string) bool {
		_, ok := gate.resolve(httptest.NewRequest("GET", "/?token="+tok, nil))
		return ok
	}
	if !resolves(old) || !resolves(keep) {
		t.Fatal("precondition: both minted tokens resolve")
	}

	tableRemover(imp, gate)("g1")

	for _, id := range []string{"import-g1-s0", "import-g1-s1", "import-g1-s3"} {
		if _, ok := imp.decks[id]; ok {
			t.Errorf("%s still stored after g1 was reaped", id)
		}
	}
	for _, id := range []string{"import-g12-s0", "import-g2-s0"} {
		if _, ok := imp.decks[id]; !ok {
			t.Errorf("%s dropped by g1's reap", id)
		}
	}
	if resolves(old) {
		t.Error("g1's seat token still resolves after the reap")
	}
	if gate.token("g1", 0) != "" {
		t.Error("g1's claim still names a token")
	}
	if !resolves(keep) {
		t.Error("g2's seat token revoked by g1's reap")
	}
	// A server with no gate releases the decks and revokes nothing.
	imp.decks["import-g2-s1"] = host.Deck{}
	tableRemover(imp, nil)("g2")
	if len(imp.decks) != 1 {
		t.Errorf("after g2's reap with no gate: %d decks, want only import-g12-s0", len(imp.decks))
	}
}
