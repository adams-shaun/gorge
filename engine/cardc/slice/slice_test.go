package slice

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// servedFloor is CC-24's count of proof-slice cards cardc serves.
const servedFloor = 222

func TestBuildProofSlice(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	reg, err := Build(root, forge, "fdn", "legacy")
	if errors.Is(err, ErrNoData) {
		t.Skip(mtgish.SkipMessage)
	}
	if err != nil {
		t.Fatal(err)
	}
	if reg.Served() < servedFloor {
		t.Errorf("served %d, want at least %d", reg.Served(), servedFloor)
	}
	t.Setenv("GORGE_MTGISH", filepath.Join(t.TempDir(), "absent.json"))
	if _, err := Build(root, forge, "fdn"); !errors.Is(err, ErrNoData) {
		t.Errorf("missing data: err %v", err)
	}
}
