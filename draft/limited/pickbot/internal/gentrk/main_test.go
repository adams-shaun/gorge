package main

import (
	"bytes"
	"os"
	"testing"
)

// TestTRKRatingFileMatchesGenerator pins that the committed TRK rating file
// is exactly what the generator makes from the published card-strength
// CSV, so a re-published list cannot drift from the embedded ratings.
func TestTRKRatingFileMatchesGenerator(t *testing.T) {
	raw, err := os.ReadFile("../../../../../docs/site/research/card-strength/TRK.csv")
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("../../ratings/TRK.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Generate(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte(`"Sheoldred, the Apocalypse"`)) {
		t.Fatal("generator output lacks the list's first-ranked card")
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("ratings/TRK.json is stale (%d bytes committed, %d generated): run go generate in draft/limited/pickbot", len(want), len(got))
	}
}
