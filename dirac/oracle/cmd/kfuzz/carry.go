package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/dirac/oracle"
)

// carryCoverageEnv names the file a segment that hung leaves its coverage in for
// the continuation image (handCoverage writes it, mergeCarried reads it), so the
// run's -coverage, -branches and -evidence files (and -reach's plays column; its
// decks and games columns stay the segment's) count every segment's clean
// games, not the last segment's: card evidence needs
// oracle.EvidenceMinPlays plays over the run, which a union of per-segment files
// cannot tell.
const carryCoverageEnv = "GORGE_KFUZZ_CARRY_COVERAGE"

// segmentCoverage is the coverage this image last wrote its files from: its own
// games merged with every earlier segment's. handCoverage passes it on.
var segmentCoverage *oracle.Coverage

// mergeCarried merges the earlier segments' coverage into cov, when this image
// is a continuation that was handed some, and records cov as the coverage a
// further continuation inherits. The carried file is removed once merged.
func mergeCarried(cov *oracle.Coverage) error {
	segmentCoverage = cov
	path := os.Getenv(carryCoverageEnv)
	if cov == nil || path == "" {
		return nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("carried coverage: %w", err)
	}
	var prior oracle.Coverage
	if err := json.Unmarshal(b, &prior); err != nil {
		return fmt.Errorf("carried coverage %s: %w", path, err)
	}
	cov.Merge(&prior)
	os.Unsetenv(carryCoverageEnv) // merged: a further continuation is handed the sum
	return os.Remove(path)
}

// handCoverage writes segmentCoverage to a fresh temp file and names it in
// carryCoverageEnv, which the continuation's exec inherits. No coverage
// (no file flag asked for it) hands nothing.
func handCoverage() error {
	if segmentCoverage == nil {
		return nil
	}
	f, err := os.CreateTemp("", "kfuzz-coverage-*.json")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(f).Encode(segmentCoverage); err != nil {
		f.Close()
		os.Remove(f.Name())
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return err
	}
	return os.Setenv(carryCoverageEnv, f.Name())
}
