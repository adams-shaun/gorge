package dirac

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

func landGrantEntries(names []string) []cardset.Entry {
	es := make([]cardset.Entry, len(names))
	for i, n := range names {
		es[i] = cardset.Entry{Name: n, Count: 1}
	}
	return es
}

// landGrantSet is the mbc named set: it holds Ashaya, Soul of the Wild and
// Thermopod, the granter and the non-plain mana source of the mbc-baaa class,
// and basics for the plain side. The raw synthetic pools are the oracle's to
// read (internal/archtest TestOnlyTheOracleReadsRawTables).
func landGrantSet(t *testing.T) cardset.Set {
	t.Helper()
	return cardsettest.Set(t, "mbc")
}

// TestLandGrantPairStallFlag: a table with Ashaya, Soul of the Wild (nontoken
// creatures are Forest lands) beside Thermopod ({T}, sacrifice a creature:
// Add {R}{R}) sets prep.landGrantPair, which initStall reads as
// StallLandGrant and the matchup is refused; Ashaya beside basics and plain
// duals does not, though the grant is on (blgOn). Yavimaya, Cradle of Growth
// beside City of Traitors is the same shape on the tail pools, which only the
// oracle reaches (dirac/oracle/land_grant_pair_test.go plays it).
func TestLandGrantPairStallFlag(t *testing.T) {
	set := landGrantSet(t)
	plain := []string{"Ashaya, Soul of the Wild", "Plains", "Forest"}
	tbl, _, _, err := set.Matchup(landGrantEntries(plain), landGrantEntries(plain))
	if err != nil {
		t.Fatal(err)
	}
	p := prepare(tbl)
	if !p.gen.blgOn {
		t.Fatal("precondition: Ashaya's grant is off (blgOn)")
	}
	if p.landGrantPair || p.initStall(2) != StallNone {
		t.Fatalf("Ashaya beside plain lands: flag %v, initStall %s", p.landGrantPair, StallName(p.initStall(2)))
	}
	// the matchup of the pair is refused as the same stall (pairRefusalN)
	pair := []string{"Ashaya, Soul of the Wild", "Thermopod"}
	_, _, _, err = set.Matchup(landGrantEntries(pair), landGrantEntries(pair))
	var ir *cardset.InteractionRefusal
	if !errors.As(err, &ir) || ir.Reason != StallName(StallLandGrant) {
		t.Fatalf("Ashaya beside Thermopod: err %v, want the landgrant refusal", err)
	}
	// with the refusal off the pair's table builds and its flag is set:
	// the guard is what keeps the pair out, not a missing view
	DebugLandGrantNoPairGuard = true
	defer func() { DebugLandGrantNoPairGuard = false }()
	tbl, _, _, err = set.Matchup(landGrantEntries(pair), landGrantEntries(pair))
	if err != nil {
		t.Fatalf("with the guard off the pair is still refused: %v", err)
	}
	if p := prepare(tbl); !p.gen.blgOn || !p.landGrantPair {
		t.Fatalf("the pair's table: blgOn %v, flag %v, want both", p.gen.blgOn, p.landGrantPair)
	}
}
