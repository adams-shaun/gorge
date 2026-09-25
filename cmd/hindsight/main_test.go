package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestOrderedRecordsIgnoresWorkerCompletionOrder(t *testing.T) {
	// Slots are indexed by selected game before workers start; completion may
	// fill them in any order, but the fold is always game then reverse rank.
	slots := make([]gameResult, 3)
	for _, game := range []int{2, 0, 1} {
		slots[game] = gameResult{records: []decisionRecord{
			{GameID: game, ReverseRank: 0},
			{GameID: game, ReverseRank: 1},
		}}
	}
	got := orderedRecords(slots)
	var keys [][2]int
	for _, rec := range got {
		keys = append(keys, [2]int{rec.GameID, rec.ReverseRank})
	}
	want := [][2]int{{0, 0}, {0, 1}, {1, 0}, {1, 1}, {2, 0}, {2, 1}}
	if !reflect.DeepEqual(gotKeys(keys), want) {
		t.Fatalf("order = %v, want %v", keys, want)
	}
}

func TestDecisionJSONExcludesWallClock(t *testing.T) {
	a, err := json.Marshal(decisionRecord{GameID: 3, WallSeconds: 1.25})
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(decisionRecord{GameID: 3, WallSeconds: 99})
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Fatalf("timing changed deterministic JSONL:\n%s\n%s", a, b)
	}
}

func gotKeys(in [][2]int) [][2]int { return in }
