package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Well of Lost Dreams ("you may pay {X}") asks its controller to pay {1}
// while the controller's one untapped mana source is a Command Tower under a
// commander whose identity is nonempty. The Tower taps for one mana of the
// identity (CR 903.4, 605.3a), so the payment is reachable (CR 118.3) and
// the pay option is offered. gorge's unless-cost census read the Tower's
// "Combo ColorIdentity" production as no mana at all and offered only the
// decline, where the kernel offers both (four-seat edh pod 40 of the 100-card
// sweep, decision 2797). The census now prices it as one pinned alternative
// per identity colour.
const unlessTowerRepro = "unless_command_tower_identity_mana_offer"

// unlessTowerFirstAt is the decision the recording diverged at before the fix.
const unlessTowerFirstAt = 2797

// TestUnlessCommandTowerOfferLockstep replays the four-seat pod in lockstep
// past the recorded decision.
func TestUnlessCommandTowerOfferLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", unlessTowerRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	es := make([][]cardset.Entry, 0, 4)
	for _, names := range [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]} {
		es = append(es, ReproEntries(reg, set, names))
	}
	tbl, _, err := set.MatchupN(es)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	res := Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= unlessTowerFirstAt {
		t.Errorf("%d decisions, the game must go past decision %d", res.Decisions, unlessTowerFirstAt)
	}
}
