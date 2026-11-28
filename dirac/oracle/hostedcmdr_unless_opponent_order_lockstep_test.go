package oracle

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// hostedCmdrUnlessOrderRepro is Vexing Devil's "any opponent may have it deal
// 4 damage to them" in a four-seat hosted-cmdr pod on its controller's (seat
// 2's) own turn. The asks go in APNAP order (CR 101.4): seat 3, then 0, then
// 1. The kernel used to ask only the controller's next opponent, seat 3.
const hostedCmdrUnlessOrderRepro = "hosted_cmdr_unless_opponent_turn_order"

// hostedCmdrUnlessOrderFirstAt is the first unless ask's decision.
const hostedCmdrUnlessOrderFirstAt = 383

// TestHostedCmdrUnlessOpponentTurnOrderLockstep replays the four-seat pod in
// lockstep and declines every opponent's ask, so the whole walk is played: the
// seats asked are the opponents in turn order from the active player.
func TestHostedCmdrUnlessOpponentTurnOrderLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", hostedCmdrUnlessOrderRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("hosted-cmdr")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no hosted-cmdr table (make dirac-tables): %v", err)
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
	script := &ScriptPolicy{Actions: r.Actions}
	var asked []uint8
	res := Lockstep(m, r.Seed, k, PolicyFunc(func(c *PickCtx) Canon {
		if c.Seq < hostedCmdrUnlessOrderFirstAt || c.Seq >= hostedCmdrUnlessOrderFirstAt+3 {
			return script.Pick(c)
		}
		if len(c.D.Options) != 2 {
			t.Errorf("decision %d: %d options, want the pay and decline pair", c.Seq, len(c.D.Options))
		}
		asked = append(asked, c.D.Player)
		return c.D.Options[len(c.D.Options)-1] // decline: the next opponent is asked
	}), rc)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= hostedCmdrUnlessOrderFirstAt+3 {
		t.Fatalf("%d decisions, the game must go past the walk at %d", res.Decisions, hostedCmdrUnlessOrderFirstAt)
	}
	if want := []uint8{3, 0, 1}; !slices.Equal(asked, want) {
		t.Fatalf("seats asked %v, want %v", asked, want)
	}
}
