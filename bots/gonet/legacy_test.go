package gonet_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
)

func TestLegacyRawWidthNetIsExact(t *testing.T) {
	old := gonet.CacheSize
	gonet.CacheSize = 0
	t.Cleanup(func() { gonet.CacheSize = old })

	dir := t.TempDir()
	legacyPath, err := gonettest.WriteRaw(dir, "legacy.dzgw", policynet.EntityRawWidthV1, nil)
	if err != nil {
		t.Fatal(err)
	}
	widePath, err := gonettest.Write(dir, "wide.dzgw")
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := gonet.Load(legacyPath)
	if err != nil {
		t.Fatalf("legacy Load: %v", err)
	}
	wide, err := gonet.Load(widePath)
	if err != nil {
		t.Fatalf("wide Load: %v", err)
	}
	if legacy.RawWidth() != policynet.EntityRawWidthV1 || wide.RawWidth() != policynet.EntityRawWidth {
		t.Fatalf("raw widths %d/%d", legacy.RawWidth(), wide.RawWidth())
	}

	st, opts := sampleState()
	st0 := st
	st0.Cards = make([]policynet.EntityCard, len(st.Cards))
	for i := range st.Cards {
		st0.Cards[i] = st.Cards[i]
		st0.Cards[i].Raw = append([]float32(nil), st.Cards[i].Raw...)
		clear(st0.Cards[i].Raw[policynet.EntityRawWidthV1:])
	}
	hasRel := false
	for i := range st.Cards {
		for _, f := range st.Cards[i].Raw[policynet.EntityRawWidthV1:] {
			if f != 0 {
				hasRel = true
			}
		}
	}
	if !hasRel {
		t.Fatal("fixture has no non-zero relational slot; the comparison proves nothing")
	}

	run := func(g *gonet.Net, s *policynet.State) (float32, []float32) {
		scores := make([]float32, len(opts))
		v := g.Eval(s, opts, scores)
		return v, scores
	}
	wantV, wantS := run(legacy, &st)
	gotV, gotS := run(legacy, &st0)
	if math.Float32bits(gotV) != math.Float32bits(wantV) {
		t.Fatalf("legacy on zeroed relational slots: value %v vs %v", gotV, wantV)
	}
	for i := range wantS {
		if math.Float32bits(gotS[i]) != math.Float32bits(wantS[i]) {
			t.Fatalf("legacy on zeroed relational slots: score %d %v vs %v", i, gotS[i], wantS[i])
		}
	}
	wantWV, wantWS := run(wide, &st)
	gotWV, gotWS := run(wide, &st0)
	if math.Float32bits(gotWV) != math.Float32bits(wantWV) {
		t.Fatalf("wide on zeroed relational slots: value %v vs %v", gotWV, wantWV)
	}
	for i := range wantWS {
		if math.Float32bits(gotWS[i]) != math.Float32bits(wantWS[i]) {
			t.Fatalf("wide on zeroed relational slots: score %d %v vs %v", i, gotWS[i], wantWS[i])
		}
	}

	v1 := policynet.EncoderHashEntityV1()
	live := policynet.EncoderHashFor(policynet.FeaturesEntity)
	good, err := gonettest.WriteRaw(dir, "v1hash.dzgw", policynet.EntityRawWidthV1, map[string]any{"encoder_hash": fmt.Sprintf("%016x", v1)})
	if err != nil {
		t.Fatal(err)
	}
	g, err := gonet.Load(good)
	if err != nil {
		t.Fatalf("v1 hash with RAW_W 68: %v", err)
	}
	if !g.EncoderHashChecked() {
		t.Fatal("v1 hash pairing loaded unchecked")
	}
	for _, tc := range []struct {
		name string
		rawW int
		hash uint64
		want string
	}{
		{"narrow-live-hash", policynet.EntityRawWidthV1, live, "encoder hash"},
		{"wide-v1-hash", policynet.EntityRawWidth, v1, "encoder hash"},
		{"raw70", 70, 0, "RAW_W = 70"},
	} {
		extra := map[string]any(nil)
		if tc.hash != 0 {
			extra = map[string]any{"encoder_hash": fmt.Sprintf("%016x", tc.hash)}
		}
		p, err := gonettest.WriteRaw(dir, tc.name+".dzgw", tc.rawW, extra)
		if err != nil {
			t.Fatalf("%s: WriteRaw: %v", tc.name, err)
		}
		if _, err := gonet.Load(p); err == nil {
			t.Fatalf("%s: load succeeded, want %q", tc.name, tc.want)
		} else if !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error %q does not contain %q", tc.name, err, tc.want)
		}
	}
}
