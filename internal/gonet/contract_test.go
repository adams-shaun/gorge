package gonet_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/gonet"
	"github.com/adams-shaun/gorge/internal/gonet/gonettest"
	"github.com/adams-shaun/gorge/internal/policynet"
)

var updateContracts = flag.Bool("update-contracts", false, "rewrite internal/gonet/testdata/contracts goldens")

// TestContractDZGWHeader pins contract dzgw's JSON header shape (the golden
// carries the optional encoder_hash with a stand-in value, and a one-tensor
// table). The producer is python -m dzg.export in DraftZero; this golden is
// what gonet.Load reads.
func TestContractDZGWHeader(t *testing.T) {
	got, err := gonettest.Header([]gonettest.Entry{{Name: "table.weight", Shape: []int{policynet.TableRows, 4}, Offset: 0, NBytes: 4 * 4 * policynet.TableRows}},
		map[string]any{"encoder_hash": "0123456789abcdef"})
	if err != nil {
		t.Fatal(err)
	}
	got = append(got, '\n')
	path := filepath.Join("testdata", "contracts", "dzgw_header.json")
	if *updateContracts {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("the dzgw header changed (docs/contracts.md):\n got: %s\nwant: %s", got, want)
	}
}

// TestLoadChecksEncoderHash: a matching encoder_hash loads checked, a
// different one is refused naming both hashes, and an absent one loads
// unchecked (every network exported before the field).
func TestLoadChecksEncoderHash(t *testing.T) {
	dir := t.TempDir()
	live := policynet.EncoderHashFor(policynet.FeaturesEntity)
	for _, tc := range []struct {
		name, hash  string
		ok, checked bool
	}{
		{"match.dzgw", fmt.Sprintf("%016x", live), true, true},
		{"match0x.dzgw", fmt.Sprintf("0x%016x", live), true, true},
		{"absent.dzgw", "", true, false},
		{"drift.dzgw", fmt.Sprintf("%016x", live^1), false, false},
		{"garbage.dzgw", "not-hex", false, false},
	} {
		var extra map[string]any
		if tc.hash != "" {
			extra = map[string]any{"encoder_hash": tc.hash}
		}
		path, err := gonettest.WriteWith(dir, tc.name, extra)
		if err != nil {
			t.Fatal(err)
		}
		g, err := gonet.Load(path)
		if (err == nil) != tc.ok {
			t.Fatalf("%s: err %v", tc.name, err)
		}
		if err != nil {
			if tc.name == "drift.dzgw" && !strings.Contains(err.Error(), "encoder hash") {
				t.Fatalf("%s: %v", tc.name, err)
			}
			continue
		}
		if g.EncoderHashChecked() != tc.checked {
			t.Fatalf("%s: EncoderHashChecked %v", tc.name, g.EncoderHashChecked())
		}
	}
}
