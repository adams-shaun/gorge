package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLifeLostExtremeValue: the life-lost extremes lower to OppExtLifeLost
// over the group the head names; any other property of those groups, and a
// different group, still refuses.
func TestLifeLostExtremeValue(t *testing.T) {
	lc := &lowerCtx{}
	hi, lo, all := kop.OppExtLifeLost, kop.OppExtLifeLost|kop.OppExtLowest, kop.OppExtAllPl
	for body, want := range map[string]int{
		"PlayerCountRegisteredOpponents$HighestLifeLostThisTurn":     hi,
		"PlayerCountRegisteredOpponents$LowestLifeLostThisTurn":      lo,
		"PlayerCountDefinedRegistered$HighestLifeLostThisTurn":       hi | all,
		"PlayerCountDefinedRegistered.Other$HighestLifeLostThisTurn": hi,
		"PlayerCountDefinedRegistered.Other$LowestLifeLostThisTurn":  lo,
	} {
		v, err := lc.svarValue(body, 0)
		if err != nil || v.Kind != vOppExtreme || v.N != want {
			t.Errorf("%s: got kind %v N %#x, %v; want OppExtreme N %#x", body, v.Kind, v.N, err, want)
		}
	}
	for _, body := range []string{
		"PlayerCountDefinedRegistered$LowestLifeLostThisTurn",
		"PlayerCountRegisteredOpponents$HighestLifeTotal",
		"PlayerCountDefinedRegistered$HasPropertyLostLifeThisTurn",
		"PlayerCountDefinedRegistered.Other$HighestLifeTotal",
		"PlayerCountOpponents$HighestLifeLostThisTurn",
	} {
		if _, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%s lowered, want a refusal", body)
		}
	}
}
