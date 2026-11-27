package mtgish

import (
	"os"
	"path/filepath"
)

// SkipMessage is the t.Skip text of a test that needs the data and finds
// none. It names .cards, so scripts/test_targets.py reports ENV, not PASS.
const SkipMessage = "cardc: no mtgish corpus under .cards/mtgish -- run make mtgish-fetch"

// ModuleRoot is the first ancestor of the working directory that holds go.mod.
func ModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// pinnedFile is <module root>/.cards/mtgish/<ref>/<name>, with <ref> read from
// engine/cardc/MTGISH.lock. It returns "" when the root or the lock is missing.
func pinnedFile(name string) string {
	root, err := ModuleRoot()
	if err != nil {
		return ""
	}
	lock, err := ReadLock(filepath.Join(root, "engine", "cardc", "MTGISH.lock"))
	if err != nil || lock.Ref == "" {
		return ""
	}
	return filepath.Join(root, ".cards", "mtgish", lock.Ref, name)
}

// DataPath is the path of mtgish.lines.json: $GORGE_MTGISH when set, else the
// file the pinned ref fetched under .cards. "" when neither can be resolved.
func DataPath() string {
	if p := os.Getenv("GORGE_MTGISH"); p != "" {
		return p
	}
	return pinnedFile("mtgish.lines.json")
}

// SchemaPath is the path of oracle_cards.d.ts, which CC-00's fetch lands
// beside the data. $GORGE_MTGISH_SCHEMA overrides it.
func SchemaPath() string {
	if p := os.Getenv("GORGE_MTGISH_SCHEMA"); p != "" {
		return p
	}
	return pinnedFile("oracle_cards.d.ts")
}
