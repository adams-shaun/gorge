package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestGenerateReproducesCommittedSets regenerates every set that has a
// trimmed MTGJSON input, in memory, and asserts the committed file under
// sets/ is byte-identical — so an MTGJSON pin move (a new printing, a
// collation correction) shows up as a diff instead of silently drifting
// from the committed spec.
func TestGenerateReproducesCommittedSets(t *testing.T) {
	// The generator runs from draft/limited/setdata; the test runs from this
	// package's directory two levels deeper.
	root := "../.."
	mtgjson := filepath.Join(root, "mtgjson")

	entries, err := os.ReadDir(mtgjson)
	if err != nil {
		t.Fatalf("read mtgjson inputs: %v", err)
	}
	var codes []string
	for _, e := range entries {
		if name := e.Name(); strings.HasSuffix(name, ".json") {
			codes = append(codes, strings.TrimSuffix(name, ".json"))
		}
	}
	if len(codes) != 2 {
		t.Fatalf("expected the two committed MTGJSON inputs (FDN, BLB), found %d: %v", len(codes), codes)
	}
	for _, code := range codes {
		t.Run(code, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(mtgjson, code+".json"))
			if err != nil {
				t.Fatalf("read MTGJSON input: %v", err)
			}
			got, err := Generate(raw)
			if err != nil {
				t.Fatalf("generate: %v", err)
			}
			if len(got) < 1000 {
				t.Fatalf("generated spec for %s is only %d bytes: the generator produced nothing", code, len(got))
			}
			want, err := os.ReadFile(filepath.Join(root, "sets", code+".json"))
			if err != nil {
				t.Fatalf("read committed set file: %v", err)
			}
			if !bytes.Equal(got, want) {
				// Show the first diverging line so the drift is readable.
				gotLines := strings.Split(string(got), "\n")
				wantLines := strings.Split(string(want), "\n")
				for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
					g, w := "", ""
					if i < len(gotLines) {
						g = gotLines[i]
					}
					if i < len(wantLines) {
						w = wantLines[i]
					}
					if g != w {
						t.Fatalf("sets/%s.json is stale: regenerate with `go generate ./` in limited/setdata.\nfirst differing line %d:\n  generated: %s\n  committed: %s", code, i+1, g, w)
					}
				}
				t.Fatalf("sets/%s.json differs from the regenerated spec", code)
			}
		})
	}
}

// TestGenerateIsDeterministic compiles one input twice and asserts the
// outputs are byte-identical, so a regenerate run can never churn the
// committed file.
func TestGenerateIsDeterministic(t *testing.T) {
	raw, err := os.ReadFile("../../mtgjson/FDN.json")
	if err != nil {
		t.Fatalf("read MTGJSON input: %v", err)
	}
	first, err := Generate(raw)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	second, err := Generate(raw)
	if err != nil {
		t.Fatalf("generate again: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("two Generate runs over the same input produced different bytes")
	}
}

// TestGenerateRejectsBrokenInputs proves the compile step fails loudly on
// the malformed shapes a trimmed MTGJSON file could carry: an unknown
// rarity word, a config naming a missing sheet, and a variant recipe that
// disagrees with its siblings about the pack size.
func TestGenerateRejectsBrokenInputs(t *testing.T) {
	raw, err := os.ReadFile("../../mtgjson/FDN.json")
	if err != nil {
		t.Fatalf("read MTGJSON input: %v", err)
	}
	cases := map[string]func(string) string{
		"unknown rarity word": func(s string) string {
			return strings.Replace(s, `"rarity": "common"`, `"rarity": "commonish"`, 1)
		},
		"variant names a missing sheet": func(s string) string {
			return strings.Replace(s, `"land": 1,`, `"unreal": 1,`, 1)
		},
		"zero card weight": func(s string) string {
			return strings.Replace(s, "\"weight\": 1\n", "\"weight\": 0\n", 1)
		},
	}
	for name, breakIt := range cases {
		if _, err := Generate([]byte(breakIt(string(raw)))); err == nil {
			t.Errorf("Generate accepted an input with a %s", name)
		}
	}
}
