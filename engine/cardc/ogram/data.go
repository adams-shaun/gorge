package ogram

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
)

// SkipMessage is the t.Skip text of a test that needs the release and finds
// none. It names .cards, so scripts/test_targets.py reports ENV, not PASS.
const SkipMessage = "cardc: no oracle-grammar release under .cards/ogram -- run make ogram-fetch"

// lockTag is the release tag of engine/cardc/OGRAM.lock at path.
func lockTag(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if fs := strings.Fields(sc.Text()); len(fs) == 2 && fs[0] == "tag" {
			return fs[1], nil
		}
	}
	if err := sc.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s: no tag line", path)
}

func lockPath() (string, error) {
	root, err := mtgish.ModuleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "engine", "cardc", "OGRAM.lock"), nil
}

// DataDir is the release directory: $GORGE_OGRAM when set, else
// <module root>/.cards/ogram/<tag from the lock>. "" when neither resolves.
func DataDir() string {
	if p := os.Getenv("GORGE_OGRAM"); p != "" {
		return p
	}
	lp, err := lockPath()
	if err != nil {
		return ""
	}
	tag, err := lockTag(lp)
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(lp))), ".cards", "ogram", tag)
}

// Fetched reports whether dir holds the three record files and the schema.
func Fetched(dir string) bool {
	if dir == "" {
		return false
	}
	for _, n := range []string{facesFile, outcomesFile, sourceFile, schemaFile} {
		if _, err := os.Stat(filepath.Join(dir, n)); err != nil {
			return false
		}
	}
	return true
}

const (
	facesFile    = "faces.jsonl.gz"
	outcomesFile = "outcomes.jsonl.gz"
	sourceFile   = "oracle-source.jsonl.gz"
	coverageFile = "coverage.json"
	// schemaFile is the base name of the lock's schema line, the type
	// description of schemaVersion.
	schemaFile    = "193.json"
	schemaVersion = 193
)
