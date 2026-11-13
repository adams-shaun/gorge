package cards

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"
	"time"
)

// loadBothRoutes loads the cache at path twice: the first open takes the gob
// route and writes the image file, the second must be served from it.
func loadBothRoutes(t *testing.T, path string) (gobRoute, imgRoute *Registry) {
	t.Helper()
	if _, err := os.Stat(ImagePath(path)); err == nil {
		t.Fatal("image file present before the first load")
	}
	a, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := readImageFile(ImagePath(path), path); err != nil {
		t.Fatalf("first load wrote no usable image file: %v", err)
	}
	b, err := LoadRegistry(path)
	if err != nil {
		t.Fatal(err)
	}
	return a, b
}

func sameTokens(t *testing.T, got, want map[string]*Card) {
	t.Helper()
	if len(got) != len(want) || (got == nil) != (want == nil) {
		t.Fatalf("tokens: %d (nil %v), want %d (nil %v)", len(got), got == nil, len(want), want == nil)
	}
	keys := make([]string, 0, len(want))
	for k := range want {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		sameCard(t, "token "+k, got[k], want[k])
	}
}

// TestImageFileRoundTrip: the image a read returns is the image the gob route
// built, row for row, so the registry served from it is the same registry.
func TestImageFileRoundTrip(t *testing.T) {
	r := fixtureRegistry(t)
	if err := r.CompileMetadata(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ir.gob.gz")
	if err := r.Save(path); err != nil {
		t.Fatal(err)
	}
	a, b := loadBothRoutes(t, path)
	if !reflect.DeepEqual(a.lazy.img, b.lazy.img) {
		t.Fatal("image read back differs from the image built")
	}
	if a.Len() != b.Len() {
		t.Fatalf("cards %d, want %d", b.Len(), a.Len())
	}
	for i := 0; i < a.Len(); i++ {
		sameCard(t, "card", b.Card(i), a.Card(i))
	}
	sameTokens(t, b.Tokens, a.Tokens)
	if a.Catalog().Identity != b.Catalog().Identity {
		t.Fatal("catalog identity differs")
	}
}

// TestImageFileRejected: an image file that no longer matches its cache, or
// whose bytes were damaged, is never served.
func TestImageFileRejected(t *testing.T) {
	r := fixtureRegistry(t)
	if err := r.CompileMetadata(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "ir.gob.gz")
	if err := r.Save(path); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadRegistry(path); err != nil {
		t.Fatal(err)
	}
	img := ImagePath(path)
	good, err := os.ReadFile(img)
	if err != nil {
		t.Fatal(err)
	}
	restore := func() {
		t.Helper()
		if err := os.WriteFile(img, good, 0o644); err != nil {
			t.Fatal(err)
		}
		now := time.Now().Add(time.Hour)
		if err := os.Chtimes(img, now, now); err != nil {
			t.Fatal(err)
		}
	}
	restore()
	if _, err := readImageFile(img, path); err != nil {
		t.Fatalf("intact image rejected: %v", err)
	}
	// A cache rewritten after the image.
	later := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if _, err := readImageFile(img, path); err == nil {
		t.Fatal("image older than its cache served")
	}
	restore()
	if err := os.Chtimes(path, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	// A flipped byte in the rows.
	bad := append([]byte(nil), good...)
	bad[len(bad)/2] ^= 0xff
	if err := os.WriteFile(img, bad, 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Add(time.Hour)
	_ = os.Chtimes(img, now, now)
	if _, err := readImageFile(img, path); err == nil {
		t.Fatal("damaged image served")
	}
	// A truncated file.
	if err := os.WriteFile(img, good[:len(good)/3], 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(img, now, now)
	if _, err := readImageFile(img, path); err == nil {
		t.Fatal("truncated image served")
	}
	// LoadRegistry still loads (gob route) and repairs the file.
	if _, err := LoadRegistry(path); err != nil {
		t.Fatal(err)
	}
	if _, err := readImageFile(img, path); err != nil {
		t.Fatalf("gob route did not rewrite the image: %v", err)
	}
}

// TestImageFileCorpus is TestImageFileRoundTrip over the whole corpus: the
// image rows, every token, and a sample of materialized cards.
func TestImageFileCorpus(t *testing.T) {
	full := compiledCorpus(t)
	path := CachePath(t.TempDir())
	if err := full.Save(path); err != nil {
		t.Fatal(err)
	}
	a, b := loadBothRoutes(t, path)
	if !reflect.DeepEqual(a.lazy.img, b.lazy.img) {
		t.Fatal("image read back differs from the image built")
	}
	sameTokens(t, b.Tokens, a.Tokens)
	for i := 0; i < a.Len(); i += 97 {
		sameCard(t, "card", b.Card(i), a.Card(i))
	}
}
