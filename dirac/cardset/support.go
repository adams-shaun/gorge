package cardset

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// The support lists, under the module root and in the table sources, so that a change
// to either re-keys the tables:
//
//	dirac/support/refused/<slug>.json  a card that lowers and must not be played
//	dirac/support/evidence/<pool>.txt  cards <pool>'s clean lockstep games evidence
const (
	RefusedDir  = "dirac/support/refused"
	EvidenceDir = "dirac/support/evidence"
)

// RefusedRepro is a game that showed a refused card diverging.
type RefusedRepro struct {
	Pool    string `json:"pool"`    // a deck pool (blb), a synthetic pool (std-dsk) or custom
	Surface string `json:"surface"` // autopay or full
	Seed    uint64 `json:"seed"`
	Pair    int    `json:"pair"`
	Policy  string `json:"policy"` // random, heuristic, explore or first
	// Mulligans is the London mulligans each player may take.
	Mulligans int `json:"mulligans"`
	// File, with pool custom, is a recorded game (dirac/oracle/testdata/repro/<file>.repro.json)
	// replayed on a table of its own decks' cards: unlike a synthetic pool's pair, no
	// change to what a set supports re-deals it.
	File string `json:"file,omitempty"`
}

// RefusedEntry is one dirac/support/refused file.
type RefusedEntry struct {
	Card  string       `json:"card"`
	Class string       `json:"class"`
	Why   string       `json:"why"`
	Repro RefusedRepro `json:"repro"`
	// Evidence is the lists refusing the card took it out of, for undoing that.
	Evidence []string `json:"evidence,omitempty"`
}

// Reason is the entry as RefusedError and Set.Reason word it.
func (e *RefusedEntry) Reason() string {
	return fmt.Sprintf("refused: %s: %s", e.Class, e.Why)
}

// Marshal is the entry as a file holds it.
func (e *RefusedEntry) Marshal() ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(e)
	return b.Bytes(), err
}

// SupportSlug is the file stem of card's refused entry: lower case, each run of other
// characters one hyphen.
func SupportSlug(card string) string {
	var b strings.Builder
	gap := false
	for _, r := range strings.ToLower(card) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			if gap && b.Len() > 0 {
				b.WriteByte('-')
			}
			gap = false
			b.WriteRune(r)
		} else {
			gap = true
		}
	}
	return b.String()
}

// ReadRefused reads every refused entry in fsys, a tree rooted at the module, in file
// order. A field no entry may hold is an error: these files carry names and reasons
// and nothing else.
func ReadRefused(fsys fs.FS) ([]RefusedEntry, error) {
	files, err := fs.Glob(fsys, path.Join(RefusedDir, "*.json"))
	if err != nil {
		return nil, err
	}
	out := make([]RefusedEntry, 0, len(files))
	for _, f := range files {
		b, err := fs.ReadFile(fsys, f)
		if err != nil {
			return nil, err
		}
		var e RefusedEntry
		d := json.NewDecoder(strings.NewReader(string(b)))
		d.DisallowUnknownFields()
		if err := d.Decode(&e); err != nil {
			return nil, fmt.Errorf("%s: %w", f, err)
		}
		out = append(out, e)
	}
	return out, nil
}
