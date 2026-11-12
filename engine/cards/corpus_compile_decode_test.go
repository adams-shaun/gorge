package cards

import (
	"bytes"
	"path/filepath"
	"testing"
)

// TestCorpusCardCompileAndDecodeAgree pins the two ways a card registry is
// built: a fresh CompileDir over the corpus, and the gob cache Save ->
// LoadRegistry route. The Fading expansion (kw_fading.go, CR 702.32) shaped
// real IR but was once outside compilerSources, so two builds that differed in
// it shared the same cache name and one decoded IR it could not produce. This
// asserts the file is covered by CompilerFingerprint, then that Woodripper's
// Fading IR survives the codec byte for byte.
//
// It compares GobEncode bytes rather than FaceEqual deliberately: FaceEqual
// compares printed.typesFirst/costData and compiledCatalog identity, which are
// offsets/pointers into one registry's own blobs and differ by layout alone.
func TestCorpusCardCompileAndDecodeAgree(t *testing.T) {
	// Fails when c5ce6fd161's embed line is reverted: kw_fading.go shaped IR
	// but was outside CompilerFingerprint, so two compilers shared ir-db4e56d.
	if _, err := compilerSources.ReadFile("kw_fading.go"); err != nil {
		t.Fatalf("kw_fading.go is outside CompilerFingerprint: %v", err)
	}

	fresh := compiledCorpus(t)             // a real CompileDir
	want, ok := fresh.Lookup("Woodripper") // K:Fading:3
	if !ok {
		t.Fatal("Woodripper missing from the fresh compile")
	}
	if len(want.Faces) != 1 {
		t.Fatalf("fresh Woodripper has %d faces, want 1", len(want.Faces))
	}
	front := want.Faces[0]
	if !front.HasKeyword("Fading") {
		t.Fatal("fresh Woodripper front face lost the Fading keyword")
	}
	foundRepl, foundTrigger := false, false
	for _, r := range front.Repls {
		if r.Params["Keyword"] == "Fading" {
			foundRepl = true
		}
	}
	for _, tr := range front.Triggers {
		if tr.Params["Keyword"] == "Fading" {
			foundTrigger = true
		}
	}
	if !foundRepl || !foundTrigger {
		t.Fatalf("fresh Woodripper Fading IR incomplete: repl=%v trigger=%v", foundRepl, foundTrigger)
	}

	cache := filepath.Join(t.TempDir(), "ir.gob.gz")
	if err := fresh.Save(cache); err != nil {
		t.Fatal(err)
	}
	decoded, err := LoadRegistry(cache) // the decode route
	if err != nil {
		t.Fatal(err)
	}
	got, ok := decoded.Lookup("Woodripper")
	if !ok {
		t.Fatal("Woodripper missing from the decoded cache")
	}
	if len(got.Faces) != len(want.Faces) {
		t.Fatalf("decoded Woodripper has %d faces, fresh has %d", len(got.Faces), len(want.Faces))
	}
	for i := range want.Faces { // bytes, not FaceEqual
		wb, err := want.Faces[i].GobEncode()
		if err != nil {
			t.Fatal(err)
		}
		gb, err := got.Faces[i].GobEncode()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(wb, gb) {
			t.Errorf("face %d: codec bytes differ", i)
		}
	}
}
