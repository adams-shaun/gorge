package neteval

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac/feat"
)

// TestNetevalLoadChecksEncoderHash: a matching encoder_hash loads checked, a
// different one is refused, an absent one loads (every net exported before
// the field); the ext_hash binds the extension layout the same way.
func TestNetevalLoadChecksEncoderHash(t *testing.T) {
	dir := t.TempDir()
	live := policynet.EncoderHashFor(policynet.FeaturesEntity)
	extLive := feat.ExtHash(feat.ExtAll)
	for _, tc := range []struct {
		name  string
		extra map[string]any
		ok    bool
	}{
		{"match.dzgw", map[string]any{"encoder_hash": fmt.Sprintf("%016x", live)}, true},
		{"match0x.dzgw", map[string]any{"encoder_hash": fmt.Sprintf("0x%016x", live)}, true},
		{"absent.dzgw", nil, true},
		{"drift.dzgw", map[string]any{"encoder_hash": fmt.Sprintf("%016x", live^1)}, false},
		{"garbage.dzgw", map[string]any{"encoder_hash": "not-hex"}, false},
		{"ext_on_plain.dzgw", map[string]any{"ext_hash": fmt.Sprintf("%016x", extLive)}, false},
		{"ext_drift.dzgw", map[string]any{"ext_hash": fmt.Sprintf("%016x", extLive^1)}, false},
	} {
		path, err := gonettest.WriteWith(dir, tc.name, tc.extra)
		if err != nil {
			t.Fatal(err)
		}
		w, err := LoadGonet(path)
		if (err == nil) != tc.ok {
			t.Fatalf("%s: LoadGonet err %v, want ok=%v", tc.name, err, tc.ok)
		}
		if err != nil {
			continue
		}
		if _, has := tc.extra["encoder_hash"]; has && !w.gn.EncoderHashChecked() {
			t.Fatalf("%s: header has a hash but the net loaded unchecked", tc.name)
		}
		if _, has := tc.extra["encoder_hash"]; !has && w.gn.EncoderHashChecked() {
			t.Fatalf("%s: no header hash but EncoderHashChecked", tc.name)
		}
	}

	// the ext net: a matching ext_hash loads, a drifted one is refused, one
	// naming the block on a plain net is refused (above)
	extIn := make([]float32, 64*feat.ExtWidth)
	for _, tc := range []struct {
		name string
		hash string
		ok   bool
	}{
		{"extmatch.dzgw", fmt.Sprintf("%016x", extLive), true},
		{"extabsent.dzgw", "", true},
		{"extdrift.dzgw", fmt.Sprintf("%016x", extLive^1), false},
	} {
		p, err := gonettest.WriteExtHash(dir, tc.name, gonettest.Wide, feat.ExtWidth, extIn, tc.hash)
		if err != nil {
			t.Fatal(err)
		}
		_, err = LoadWeights(p)
		if (err == nil) != tc.ok {
			t.Fatalf("%s: LoadWeights err %v, want ok=%v", tc.name, err, tc.ok)
		}
		if err != nil && !strings.Contains(err.Error(), "ext_hash") {
			t.Fatalf("%s: %v", tc.name, err)
		}
	}
}
