package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFilterWordKickedAlt: Card.Self+kicked 1 / kicked 2 lower on the
// battlefield to kop.PredKickedAlt with PredN the alternative index; the bare
// kicked word keeps F2Kicked, kicked 3 and the negated spelling stay unknown
// properties, and the word off the battlefield refuses.
func TestFilterWordKickedAlt(t *testing.T) {
	lower := func(spec string, zone kop.Zone) (kop.Filter, error) {
		a := NewAtomCompiler()
		id, err := a.Filter(spec, zone)
		if err != nil {
			return kop.Filter{}, err
		}
		fs, _ := a.Tables()
		return fs[id], nil
	}
	for n := int16(1); n <= 2; n++ {
		spec := "Card.Self+kicked " + string(rune('0'+n))
		f, err := lower(spec, kop.ZoneBattlefield)
		if err != nil {
			t.Fatalf("%s: %v", spec, err)
		}
		if f.Pred != kop.PredKickedAlt || f.PredN != n {
			t.Errorf("%s: Pred %d PredN %d, want PredKickedAlt %d / %d", spec, f.Pred, f.PredN, kop.PredKickedAlt, n)
		}
	}
	// the two alternatives lower to different filters (the compared values differ)
	f1, _ := lower("Card.Self+kicked 1", kop.ZoneBattlefield)
	f2, _ := lower("Card.Self+kicked 2", kop.ZoneBattlefield)
	if f1 == f2 {
		t.Error("kicked 1 and kicked 2 lowered to the same filter")
	}

	bare, err := lower("Card.Self+kicked", kop.ZoneBattlefield)
	if err != nil {
		t.Fatalf("kicked: %v", err)
	}
	if bare.Pred == kop.PredKickedAlt || bare.Flags2&kop.F2Kicked == 0 {
		t.Errorf("bare kicked: Pred %d Flags2 %#x, want F2Kicked and no PredKickedAlt", bare.Pred, bare.Flags2)
	}

	for _, spec := range []string{"Card.Self+kicked 3", "Card.Self+!kicked 1"} {
		if _, err := lower(spec, kop.ZoneBattlefield); err == nil || !strings.Contains(err.Error(), "unknown property") {
			t.Errorf("%s: err %v, want an unknown property refusal", spec, err)
		}
	}
	if _, err := lower("Card.Self+kicked 1", kop.ZoneGraveyard); err == nil || !strings.Contains(err.Error(), "off the battlefield") {
		t.Errorf("kicked 1 in a graveyard: err %v, want an off-the-battlefield refusal", err)
	}
}
