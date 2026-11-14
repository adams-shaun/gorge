package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/manifest"
)

// TestManifestGenIsCurrent: the files kmanifest generates (the manifest's
// constants and execExtGen, the dispatch of the //kmanifest:ext handlers) are
// what the manifest and the handlers in this tree generate.
func TestManifestGenIsCurrent(t *testing.T) {
	stale, err := manifest.WriteGen("..", true)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range stale {
		t.Errorf("%s is stale: run go run ./dirac/cmd/kmanifest gen", p)
	}
}
