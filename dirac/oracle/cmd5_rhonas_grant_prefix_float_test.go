package oracle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Rhonas the Indomitable's "{2}{G}: another target creature gets +2/+0 and
// gains trample" activated under Rishkar, Peema Renegade ("each creature you
// control with a counter on it has '{T}: Add {G}'") beside Paradise Druid, which
// has its own "{T}: Add one mana of any color". gorge's payment census lists the
// Druid's printed ability, and the grant's {G} is a lone-tap ability that unit
// already makes (manaAbilityCovered, manaTapGapDominated): the Druid is no census
// gap. Its prefix search taps the creatures whose only mana ability is the
// granted one (Ghalta, Whiptongue Hydra) and prices the rest by the planner,
// which taps the Druid at the payment window after the target ask (CR 601.2c,
// 601.2f-g). The kernel's grant search also activated the Druid, so it was
// tapped and its mana floated before the target was chosen.
//
// firstAt is the decision the recording diverged at (kernel
// mana_grantcovered.go grantCovered).
var cmd5RhonasGrantPrefixFloat = []struct {
	name    string
	firstAt int
}{
	{"cmd5_rhonas_grant_prefix_float_s2p739", 1491}, // flags: gorge leaves the Druid untapped, the kernel taps it
}

func cmd5RhonasGrantPrefixFloatReplay(t *testing.T, name string, firstAt int) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if d := r.Divergence; d == nil || d.At != firstAt || d.Field != "Cards[372].Flags" || !strings.Contains(strings.Join(d.Diff, "\n"), "Pool[4]") {
		t.Fatalf("%s is not the Paradise Druid flags and pool divergence at %d: %+v", name, firstAt, d)
	}
	if len(r.Cards) != 2 || len(r.MoreCards) != 2 {
		t.Fatalf("%s: %d+%d decks, want a four-seat pod", name, len(r.Cards), len(r.MoreCards))
	}
	reg := synthReg(t)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("cmd5")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
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
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

func TestCmd5RhonasGrantPrefixFloatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, c := range cmd5RhonasGrantPrefixFloat {
		res := cmd5RhonasGrantPrefixFloatReplay(t, c.name, c.firstAt)
		if res.Div != nil {
			t.Errorf("%s: divergence at %d: %s %s %v", c.name, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.Decisions <= c.firstAt {
			t.Errorf("%s: %d decisions, the game must go past decision %d where Rhonas is activated", c.name, res.Decisions, c.firstAt)
		}
	}
}

// TestCmd5RhonasGrantPrefixFloatLockstepMutation: with every grant a census
// gap again the Druid is tapped before the target ask. Each recording must
// diverge at its recorded decision.
func TestCmd5RhonasGrantPrefixFloatLockstepMutation(t *testing.T) {
	dirac.DebugNoGrantCovered = true
	defer func() { dirac.DebugNoGrantCovered = false }()
	for _, c := range cmd5RhonasGrantPrefixFloat {
		res := cmd5RhonasGrantPrefixFloatReplay(t, c.name, c.firstAt)
		if res.Div == nil {
			t.Errorf("%s: no divergence with grantCovered off", c.name)
		} else if res.Div.At != c.firstAt {
			t.Errorf("%s: diverged at %d, want the recorded decision %d", c.name, res.Div.At, c.firstAt)
		}
	}
}
