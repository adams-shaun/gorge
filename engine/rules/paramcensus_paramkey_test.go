package rules

import (
	"strings"
	"testing"
)

// TestParamCensusAttributesParamKeyHelperReads: DigMultiple reads its keys
// through digMultipleParam(sa, cards.PK<Key>), whose `key` parameter is a
// cards.ParamKey; the census must attribute each call site's constant.
func TestParamCensusAttributesParamKeyHelperReads(t *testing.T) {
	t.Parallel()
	_, d := measureParamCensus(t, nil)
	set := d.api["DigMultiple"]
	for _, k := range []string{"DigNum", "ChangeValid", "Reveal", "Optional", "DestinationZone", "DestinationZone2",
		"RestRandomOrder", "RememberChanged", "ImprintRest", "ChangeLater", "SourceZone"} {
		if !set[k] {
			t.Errorf("api DigMultiple census lacks %s", k)
		}
	}
}

// TestParamCensusCatchesComputedParamKey: a compiled-param read whose key is
// neither a cards.PK constant nor a ParamKey parameter fails the rot guard.
func TestParamCensusCatchesComputedParamKey(t *testing.T) {
	t.Parallel()
	s := newScan()
	s.scanSource(t, "probe_pk.go", "effects", `package effects

import "github.com/adams-shaun/gorge/engine/cards"

func censusProbeComputedKey(sa *cards.SA, keys []cards.ParamKey) string {
	v, _ := sa.Param(keys[0])
	return v
}

func censusProbeKeyHelper(sa *cards.SA, key cards.ParamKey) string {
	v, _ := sa.Param(key)
	return v
}

func censusProbeKeyCaller(sa *cards.SA) string {
	return censusProbeKeyHelper(sa, cards.PKDigNum)
}
`)
	s.propagateKeyReads()
	if fi := s.fns["effects:censusProbeKeyCaller"]; fi == nil || !fi.reads[bSA]["DigNum"] {
		t.Fatalf("ParamKey helper read not attributed to the caller: %+v", fi)
	}
	flagged, helperFlagged := false, false
	for _, e := range s.unclassified {
		if strings.Contains(e, "censusProbeComputedKey") && strings.Contains(e, "computed ParamKey") {
			flagged = true
		}
		if strings.Contains(e, "censusProbeKeyHelper") {
			helperFlagged = true
		}
	}
	if !flagged || helperFlagged {
		t.Fatalf("computed key flagged=%v, tracked-parameter helper flagged=%v; findings: %v", flagged, helperFlagged, s.unclassified)
	}
}
