package oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// A data-driven pin is a pin whose conditions name only the standard
// cardOut/onBattlefield/cardSeen conditions, so they can live in the recording
// instead of in a Go test. testdata/pins then holds a document:
//
//	{"pool":"std-fin","config":{"gorge_autopay_surface":true},
//	 "mulligans":1,"conditions":[{"kind":"onBattlefield","card":"..."}],
//	 "games":[...repros...]}
//
// TestPinManifest plays one subtest per document. A recording that still holds
// its old bare array is a pin some Go test still drives and is skipped here.
// Adding a data-driven pin needs a document, no Go edit; scripts/dirac-gate.sh
// runs the manifest test for every pin under testdata/pins.
type pinDoc struct {
	Pool       string         `json:"pool"`
	Config     pinConfig      `json:"config"`
	Mulligans  int            `json:"mulligans"`
	Conditions []pinCondition `json:"conditions"`
	Games      []*Repro       `json:"games"`
}

// pinConfig is the lockstep Config a pin records. It mirrors Config so a pin
// document can be decoded without giving the shared run type JSON tags.
type pinConfig struct {
	PassElision         bool `json:"pass_elision,omitempty"`
	GorgeAutopaySurface bool `json:"gorge_autopay_surface,omitempty"`
	RecordChance        bool `json:"record_chance,omitempty"`
	ArrangeRestOrder    bool `json:"arrange_rest_order,omitempty"`
}

func (c pinConfig) config() Config {
	return Config{
		PassElision:         c.PassElision,
		GorgeAutopaySurface: c.GorgeAutopaySurface,
		RecordChance:        c.RecordChance,
		ArrangeRestOrder:    c.ArrangeRestOrder,
	}
}

// pinCondition is one reached condition as data. Kind is "cardOut",
// "onBattlefield", "cardSeen", "onStack", "activated", "counterAtLeast" or
// "anyOf"; Card names the card, counterAtLeast reads the Counter kind against
// N, and anyOf nests its alternatives in Conditions.
type pinCondition struct {
	Kind       string         `json:"kind"`
	Card       string         `json:"card,omitempty"`
	Counter    string         `json:"counter,omitempty"`
	N          int            `json:"n,omitempty"`
	Conditions []pinCondition `json:"conditions,omitempty"`
}

// validate reports a condition the manifest could not judge, so a bad document
// fails as it is read rather than after its games are played.
func (c pinCondition) validate() error {
	switch c.Kind {
	case "anyOf":
		if len(c.Conditions) == 0 {
			return fmt.Errorf("pin condition anyOf: no conditions")
		}
		for i := range c.Conditions {
			if err := c.Conditions[i].validate(); err != nil {
				return fmt.Errorf("anyOf condition %d: %w", i, err)
			}
		}
		return nil
	case "cardOut", "onBattlefield", "cardSeen", "onStack", "activated", "counterAtLeast":
	default:
		return fmt.Errorf("pin condition %q: unknown kind", c.Kind)
	}
	if c.Card == "" {
		return fmt.Errorf("pin condition %s: no card", c.Kind)
	}
	if c.Kind == "counterAtLeast" {
		if c.Counter == "" {
			return fmt.Errorf("pin condition counterAtLeast %q: no counter", c.Card)
		}
		if c.N < 1 {
			return fmt.Errorf("pin condition counterAtLeast %q: n %d, want at least 1", c.Card, c.N)
		}
	}
	return nil
}

func (c pinCondition) reached(t *testing.T) func(ref *GorgeSide) bool {
	t.Helper()
	switch c.Kind {
	case "cardOut":
		return cardOut(c.Card)
	case "onBattlefield":
		return onBattlefield(c.Card)
	case "cardSeen":
		return cardSeen(c.Card)
	case "onStack":
		return onStack(c.Card)
	case "activated":
		return activated(c.Card)
	case "counterAtLeast":
		return counterAtLeast(c.Card, c.Counter, c.N)
	case "anyOf":
		conds := make([]func(ref *GorgeSide) bool, len(c.Conditions))
		for i := range c.Conditions {
			conds[i] = c.Conditions[i].reached(t)
		}
		return anyOf(conds...)
	default:
		t.Fatalf("pin condition %q: unknown kind", c.Kind)
		return nil
	}
}

func (d *pinDoc) reached(t *testing.T) []func(ref *GorgeSide) bool {
	t.Helper()
	out := make([]func(ref *GorgeSide) bool, len(d.Conditions))
	for i := range d.Conditions {
		out[i] = d.Conditions[i].reached(t)
	}
	return out
}

// readPinDoc decodes path when it is a data-driven pin document. It reports
// false for a bare repro array, which a Go test still drives.
func readPinDoc(path string) (*pinDoc, bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, false, err
	}
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, false, nil
	}
	var doc pinDoc
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	if doc.Pool == "" || len(doc.Conditions) == 0 || len(doc.Games) == 0 {
		return nil, false, fmt.Errorf("%s: a pin document needs a pool, conditions and at least one game", path)
	}
	for i := range doc.Conditions {
		if err := doc.Conditions[i].validate(); err != nil {
			return nil, false, fmt.Errorf("%s: condition %d: %w", path, i, err)
		}
	}
	return &doc, true, nil
}

// readPinGames is a pin's recorded games, from a data-driven document or from
// the bare array a Go test's pin still uses.
func readPinGames(path string) ([]*Repro, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var doc pinDoc
		if err := json.Unmarshal(b, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		return doc.Games, nil
	}
	var games []*Repro
	if err := json.Unmarshal(b, &games); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return games, nil
}

func writePinGames(path string, games []*Repro) error {
	b, err := json.Marshal(games)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func writePinDoc(path string, doc *pinDoc) error {
	b, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// pinManifestFiles is every data-driven pin under testdata/pins, sorted by
// path so a run's subtests keep a fixed order.
func pinManifestFiles(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("testdata", "pins", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	var out []string
	for _, path := range paths {
		if _, ok, err := readPinDoc(path); err != nil {
			t.Fatal(err)
		} else if ok {
			out = append(out, path)
		}
	}
	return out
}

// pinDocName is a pin document's subtest name: the old test name and call
// index, without the extension.
func pinDocName(path string) string {
	return trimJSONExt(filepath.Base(path))
}

func trimJSONExt(name string) string {
	const ext = ".json"
	if len(name) > len(ext) && name[len(name)-len(ext):] == ext {
		return name[:len(name)-len(ext)]
	}
	return name
}

// TestPinManifest plays every data-driven pin, one subtest each. PINS_RECORD=1
// records the games again from the document's own pool, surface and conditions.
func TestPinManifest(t *testing.T) {
	paths := pinManifestFiles(t)
	if len(paths) == 0 {
		t.Fatal("testdata/pins has no data-driven pin documents")
	}
	reg := synthReg(t)
	record := os.Getenv("PINS_RECORD") == "1"
	for _, path := range paths {
		doc, ok, err := readPinDoc(path)
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			t.Fatalf("%s: not a data-driven pin", path)
		}
		rc := RunConfig{Config: doc.Config.config()}
		reached := doc.reached(t)
		t.Run(pinDocName(path), func(t *testing.T) {
			set := cardsettest.Synth(t, doc.Pool)
			if record {
				games, err := buildPins(t, path, doc.Pool, set, reg, rc, doc.Mulligans, reached)
				if err != nil {
					t.Fatalf("%s: %v", path, err)
				}
				doc.Games = games
				if err := writePinDoc(path, doc); err != nil {
					t.Fatal(err)
				}
			}
			for i, r := range doc.Games {
				res, hit, err := playPinned(set, reg, r, rc, reached)
				if err != nil {
					t.Fatalf("%s game %d: %v", path, i, err)
				}
				reportPinned(t, fmt.Sprintf("%s game %d", path, i), res, hit)
			}
		})
	}
}

// TestPinManifestCoversOldPins is the one-time equality check for the move
// from one-line pinnedGames tests to data-driven pins: every pin file the
// removed tests ran is still in testdata/pins and is a document the manifest
// plays. New data-driven pins may only add to that set.
func TestPinManifestCoversOldPins(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "pin_manifest_names.txt"))
	if err != nil {
		t.Fatal(err)
	}
	var old []string
	for _, line := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		old = append(old, string(bytes.TrimSpace(line)))
	}
	if len(old) < 66 {
		t.Fatalf("%d old pin files, want at least the pre-change 66", len(old))
	}
	manifest := make(map[string]bool)
	for _, path := range pinManifestFiles(t) {
		manifest[filepath.Base(path)] = true
	}
	for _, name := range old {
		if !manifest[name] {
			t.Errorf("old pin %s is not a data-driven pin the manifest plays", name)
		}
	}
}
