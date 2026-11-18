package gorged

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
)

// TestCreatePodRefusesASubmittedDeckForAReservedPoolPolicy: a bot policy with a
// reserved pool plays only the decks it was trained on, so a submitted deck at
// any seat is refused, as human_deck is for an off-pool id.
func TestCreatePodRefusesASubmittedDeckForAReservedPoolPolicy(t *testing.T) {
	enableClosed(t, testClosedA)
	r, gate := freshGameLock(t)
	c := config{autoMana: true, botAutoPayMana: true, botPolicy: host.BotPolicy,
		policyPools: map[string][]string{testClosedA: {"a", "b"}}}
	c.imports = newDeckImports(nil)
	create := c.createGame(r, gate, nil, []string{"c", "d"}, view.Omniscient)
	raw := json.RawMessage(`{}`)
	for _, seat := range []int{0, 1} {
		_, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: testClosedA,
			Decks: []httpapi.SeatDeck{{Seat: seat, Deck: raw}}})
		if err == nil || !strings.Contains(err.Error(), "plays only the decks it was trained on") {
			t.Errorf("seat %d: a submitted deck for a reserved-pool policy: error %v", seat, err)
		}
	}
	if n := len(r.Tables()); n != 0 {
		t.Errorf("%d tables left behind by refused requests", n)
	}
	// Without a submitted deck the same policy still seats a table.
	if _, err := create(httpapi.CreateGameOptions{Format: host.FormatConstructed, BotPolicy: testClosedA}); err != nil {
		t.Fatalf("the policy's own pool: %v", err)
	}
}

// TestCreatePodRefusesPastTheStoredDeckCap: the store never evicts, so a full
// one refuses a request that would add to it and stores nothing of it.
func TestCreatePodRefusesPastTheStoredDeckCap(t *testing.T) {
	imp := newDeckImports(nil)
	for i := 0; i < maxStoredImports; i++ {
		imp.decks[fmt.Sprintf("import-g%d-s0", i)] = host.Deck{}
	}
	if got := len(imp.decks); got != maxStoredImports {
		t.Fatalf("precondition: store holds %d decks, want %d", got, maxStoredImports)
	}
	err := imp.putAll([]string{"new-s0", "", "new-s2"}, map[int]host.Deck{0: {}, 2: {}})
	if err == nil || !strings.Contains(err.Error(), "already holds") {
		t.Fatalf("putAll past the cap: %v", err)
	}
	if len(imp.decks) != maxStoredImports {
		t.Errorf("a refused putAll stored %d decks", len(imp.decks)-maxStoredImports)
	}
	imp.drop("import-g0-s0")
	if err := imp.putAll([]string{"new-s0"}, map[int]host.Deck{0: {}}); err != nil {
		t.Errorf("a store below the cap refused: %v", err)
	}
}
