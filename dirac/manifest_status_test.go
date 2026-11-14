package dirac

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/manifest"
)

// TestManifestStatusMatchesCode holds the manifest's invariant 2: an entry is
// active exactly when this package's non-test, non-generated source
// references its code (kop.<Name>, a key or mech name, or a //kmanifest:ext
// handler). A change that implements a code flips its entry's status line to
// active; one that does not leaves it reserved.
func TestManifestStatusMatchesCode(t *testing.T) {
	fsys := os.DirFS("..")
	tr, err := manifest.Load(fsys)
	if err != nil {
		t.Fatal(err)
	}
	errs, err := manifest.StatusMismatches(fsys, tr)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range errs {
		t.Error(e)
	}
	if manifest.CounterKindCover != int(kop.NumCounterKinds3) {
		t.Errorf("manifest.CounterKindCover = %d, kop.NumCounterKinds3 = %d: the manifest defers the kinds the kernel does not serve", manifest.CounterKindCover, kop.NumCounterKinds3)
	}
}
