package ogram

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
)

// writeGzipLines writes lines as a gzip JSON-lines file.
func writeGzipLines(t *testing.T, path string, lines ...string) {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	for _, l := range lines {
		zw.Write([]byte(l + "\n"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fixtureRecord is a face record with an invented name and no Oracle text.
func fixtureRecord(name, status string, index int, readings string) string {
	return `{"format_version":1,"schema_version":193,"name":"` + name + `","status":"` + status +
		`","identity":{"face_index":0,"layout":"normal","record_index":` + string(rune('0'+index)) +
		`,"record_name":"` + name + `"},"readings":` + readings + `}`
}

func TestReadFacesFixture(t *testing.T) {
	dir := t.TempDir()
	card := `{"schema_version":193,"name":"Probe Alpha","characteristics":{},"abilities":[]}`
	writeGzipLines(t, filepath.Join(dir, facesFile),
		fixtureRecord("Probe Alpha", "unique", 0, `[{"card":`+card+`,"notes":[]}]`),
		fixtureRecord("Probe Beta", "ambiguous", 1, `[{"card":{},"notes":[]},{"card":{},"notes":[]}]`),
		fixtureRecord("Probe Gamma", "refused", 2, `[]`),
	)
	var got []*Face
	if err := ReadFaces(dir, func(f *Face) error { got = append(got, f); return nil }); err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name string
		out  Outcome
	}{{"Probe Alpha", OutUnique}, {"Probe Beta", OutAmbiguous}, {"Probe Gamma", OutRefused}}
	if len(got) != len(want) {
		t.Fatalf("read %d faces, want %d", len(got), len(want))
	}
	for i, w := range want {
		f := got[i]
		if f.Name != w.name || f.Card != w.name || f.Outcome != w.out || f.Record != i {
			t.Errorf("face %d = %q/%q %s record %d, want %q %s record %d", i, f.Name, f.Card, f.Outcome, f.Record, w.name, w.out, i)
		}
		if (len(f.Tree) != 0) != (w.out == OutUnique) {
			t.Errorf("face %q: tree %q; want one only for a unique face", f.Name, f.Tree)
		}
	}
	if !bytes.Equal(got[0].Tree, []byte(card)) {
		t.Errorf("unique tree = %s, want %s", got[0].Tree, card)
	}

	// A record of another schema is an error, not a silent misread.
	writeGzipLines(t, filepath.Join(dir, facesFile), `{"format_version":1,"schema_version":192,"name":"X","status":"unique"}`)
	if err := ReadFaces(dir, func(*Face) error { return nil }); err == nil {
		t.Error("schema_version 192 read without error")
	}
}

func fetchedDir(t *testing.T) string {
	t.Helper()
	dir := DataDir()
	if !Fetched(dir) {
		t.Skip(SkipMessage)
	}
	return dir
}

func TestCensusMatchesCoverage(t *testing.T) {
	dir := fetchedDir(t)
	cov, err := ReadCoverage(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cov.InputFaces == 0 {
		t.Fatal("coverage.json holds no input_faces")
	}
	res, err := Census(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Total(); got != cov.InputFaces {
		t.Errorf("census counts %d faces, coverage.json %d", got, cov.InputFaces)
	}
	for _, c := range []struct {
		o    Outcome
		want int
	}{{OutUnique, cov.Counts.Unique}, {OutAmbiguous, cov.Counts.Ambiguous}, {OutRefused, cov.Counts.Refused}} {
		if got := res.Rows[c.o].All; got != c.want {
			t.Errorf("%s: census %d, coverage.json %d", c.o, got, c.want)
		}
	}
	for _, r := range res.Rows {
		if r.Commander == 0 || r.Commander > r.All {
			t.Errorf("%s: %d Commander-legal paper faces of %d", r.Outcome, r.Commander, r.All)
		}
		t.Logf("%s\t%d\t%d", r.Outcome, r.All, r.Commander)
	}
}

func TestLowerFilterFixture(t *testing.T) {
	perm := func(f *ir.Filter) *ir.Filter {
		f.Subject, f.Zones = ir.SubjPermanent, 1<<ir.ZoneBattlefield
		return f
	}
	// "creature that isn't black", as the release writes a target's filter.
	got, err := LowerFilter(json.RawMessage(`{"kind":"AllOf","args":[{"kind":"CardType","args":"Creature"},` +
		`{"kind":"Not","args":{"kind":"Color","args":"Black"}}]}`))
	if err != nil {
		t.Fatalf("AllOf: %v", err)
	}
	want := perm(&ir.Filter{Op: ir.FAll, Args: []*ir.Filter{
		perm(&ir.Filter{Op: ir.FPred, Pred: ir.PCardType, Word: "Creature"}),
		perm(&ir.Filter{Op: ir.FNot, Args: []*ir.Filter{perm(&ir.Filter{Op: ir.FPred, Pred: ir.PColor, Colors: ir.ColorB})}}),
	}})
	if !ir.Equal(got, want) {
		t.Errorf("AllOf:\n got %+v\nwant %+v", got, want)
	}

	// "spell with mana value 3 or less", in the Spells group.
	got, err = LowerFilterIn("Spells", json.RawMessage(`{"kind":"Property","args":{"property":"ManaValue",`+
		`"test":{"kind":"Compare","args":{"comparison":"LessOrEqual","value":{"kind":"Constant","args":3}}}}}`))
	if err != nil {
		t.Fatalf("Property: %v", err)
	}
	want = &ir.Filter{Subject: ir.SubjSpell, Zones: 1 << ir.ZoneStack, Op: ir.FPred, Pred: ir.PManaValue,
		Cmp: ir.CmpLE, Value: &ir.Value{Kind: ir.VConst, N: 3}}
	if !ir.Equal(got, want) {
		t.Errorf("Property:\n got %+v\nwant %+v", got, want)
	}

	// "this permanent" and "another permanent": SameObject(Source) is
	// mtgish's SinglePermanent row (PIs self), its Not the Other row.
	self := func() *ir.Filter { return perm(&ir.Filter{Op: ir.FPred, Pred: ir.PIs, Ref: &ir.Ref{Kind: ir.RefSelf}}) }
	got, err = LowerFilter(json.RawMessage(`{"kind":"SameObject","args":{"kind":"Source"}}`))
	if err != nil {
		t.Fatalf("SameObject: %v", err)
	}
	if want = self(); !ir.Equal(got, want) {
		t.Errorf("SameObject:\n got %+v\nwant %+v", got, want)
	}
	got, err = LowerFilter(json.RawMessage(`{"kind":"Not","args":{"kind":"SameObject","args":{"kind":"Source"}}}`))
	if err != nil {
		t.Fatalf("Not(SameObject): %v", err)
	}
	if want = perm(&ir.Filter{Op: ir.FNot, Args: []*ir.Filter{self()}}); !ir.Equal(got, want) {
		t.Errorf("Not(SameObject):\n got %+v\nwant %+v", got, want)
	}

	// A variant the adapter does not map is refused by its name; so is a
	// reference it has no mtgish node for.
	for src, key := range map[string]string{
		`{"kind":"InZone","args":"Graveyard"}`:                                       "ObjectFilter.InZone",
		`{"kind":"SameObject","args":{"kind":"Target","args":{"slot":0,"index":0}}}`: "ObjectRef.Target",
	} {
		_, err = LowerFilter(json.RawMessage(src))
		var re *RefusedError
		if !errors.As(err, &re) || re.Key != key {
			t.Errorf("%s: err %v, want a RefusedError naming %s", src, err, key)
		}
	}
}

func TestOgramFiltersLowerOnSlices(t *testing.T) {
	dir := fetchedDir(t)
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	for _, slice := range []string{"fdn", "blb"} {
		names, err := mtgish.SliceNames(root, slice)
		if err != nil {
			t.Fatal(err)
		}
		st, err := SliceFilters(dir, names, func(face string, op Operand, f *ir.Filter, lerr error) error {
			if f == nil {
				return nil
			}
			b, err := json.Marshal(f)
			if err != nil {
				return err
			}
			var back ir.Filter
			if err := json.Unmarshal(b, &back); err != nil {
				return err
			}
			if !ir.Equal(f, &back) {
				t.Errorf("%s %s: %s does not survive a JSON round-trip", slice, face, op.Node)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if st.Faces == 0 || st.Operands == 0 {
			t.Fatalf("%s: %d unique faces, %d operands; the check would pass vacuously", slice, st.Faces, st.Operands)
		}
		t.Logf("%s: %d unique faces, %d filter operands, %d lowered (%.1f%%), %d refused variants",
			slice, st.Faces, st.Operands, st.Lowered, 100*float64(st.Lowered)/float64(st.Operands), len(st.Refused))
	}
}
