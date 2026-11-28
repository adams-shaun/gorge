package ogram

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Outcome is how the grammar read a face.
type Outcome uint8

const (
	OutUnique Outcome = iota
	OutAmbiguous
	OutRefused
)

// Outcomes lists the outcomes in census order.
var Outcomes = []Outcome{OutUnique, OutAmbiguous, OutRefused}

func (o Outcome) String() string {
	switch o {
	case OutUnique:
		return "unique"
	case OutAmbiguous:
		return "ambiguous"
	case OutRefused:
		return "refused"
	}
	return fmt.Sprintf("Outcome(%d)", uint8(o))
}

func parseOutcome(s string) (Outcome, error) {
	for _, o := range Outcomes {
		if o.String() == s {
			return o, nil
		}
	}
	return 0, fmt.Errorf("unknown status %q", s)
}

// Face is one record of faces.jsonl.gz (or outcomes.jsonl.gz).
type Face struct {
	Name string
	// Card is identity.record_name, the name of the whole card (Name is the
	// face's).
	Card    string
	Outcome Outcome
	// Record is identity.record_index: the line of oracle-source.jsonl.gz the
	// face was read from.
	Record int
	// Tree is readings[0].card, the typed reading; empty unless OutUnique.
	Tree json.RawMessage
}

// faceRecord is the part of a face record the reader decodes.
type faceRecord struct {
	FormatVersion int    `json:"format_version"`
	SchemaVersion int    `json:"schema_version"`
	Name          string `json:"name"`
	Status        string `json:"status"`
	Identity      struct {
		RecordIndex *int   `json:"record_index"`
		RecordName  string `json:"record_name"`
	} `json:"identity"`
	Readings []struct {
		Card json.RawMessage `json:"card"`
	} `json:"readings"`
}

func (r *faceRecord) face(withTree bool) (*Face, error) {
	if r.FormatVersion != 1 {
		return nil, fmt.Errorf("%q: format_version %d, want 1", r.Name, r.FormatVersion)
	}
	if r.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("%q: schema_version %d, want %d", r.Name, r.SchemaVersion, schemaVersion)
	}
	if r.Identity.RecordIndex == nil {
		return nil, fmt.Errorf("%q: no identity.record_index", r.Name)
	}
	o, err := parseOutcome(r.Status)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", r.Name, err)
	}
	f := &Face{Name: r.Name, Card: r.Identity.RecordName, Outcome: o, Record: *r.Identity.RecordIndex}
	if o == OutUnique && withTree {
		if len(r.Readings) == 0 {
			return nil, fmt.Errorf("%q: unique with no reading", r.Name)
		}
		f.Tree = r.Readings[0].Card
	}
	return f, nil
}

// readLines streams the JSON lines of the gzip file path to visit, one line
// at a time, with its 1-based number.
func readLines(path string, visit func(line []byte, n int) error) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	br := bufio.NewReaderSize(zr, 1<<20)
	for n := 1; ; n++ {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			if verr := visit(line, n); verr != nil {
				return fmt.Errorf("%s:%d: %w", filepath.Base(path), n, verr)
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
}

func readFaceFile(path string, withTree bool, visit func(*Face) error) error {
	return readLines(path, func(line []byte, _ int) error {
		var r faceRecord
		if !withTree {
			// Readings carry the trees; skip decoding them.
			var lite struct {
				faceRecord
				Readings json.RawMessage `json:"readings"`
			}
			if err := json.Unmarshal(line, &lite); err != nil {
				return err
			}
			r = lite.faceRecord
		} else if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		f, err := r.face(withTree)
		if err != nil {
			return err
		}
		return visit(f)
	})
}

// ReadFaces streams faces.jsonl.gz in dir to visit, one face at a time; it
// never holds the file in memory. The release writes only uniquely read faces
// there.
func ReadFaces(dir string, visit func(*Face) error) error {
	return readFaceFile(filepath.Join(dir, facesFile), true, visit)
}

// ReadOutcomes streams outcomes.jsonl.gz in dir, every input face, to visit.
// Face.Tree is left empty.
func ReadOutcomes(dir string, visit func(*Face) error) error {
	return readFaceFile(filepath.Join(dir, outcomesFile), false, visit)
}

// Legality is a source record's legality.
type Legality struct{ Commander, Paper bool }

// CommanderPaper is a Commander-legal paper card.
func (l Legality) CommanderPaper() bool { return l.Commander && l.Paper }

// ReadLegality reads oracle-source.jsonl.gz in dir: one Legality per source
// record, indexed by line (Face.Record). Commander is legalities.commander ==
// "legal"; Paper is "paper" in games. A record without either field is an
// error naming it.
//
// It is indexed by record, not keyed by face name: names repeat across
// records (a token or an art card shares its card's name), and a name key
// marks those faces Commander-legal paper too.
func ReadLegality(dir string) ([]Legality, error) {
	var out []Legality
	err := readLines(filepath.Join(dir, sourceFile), func(line []byte, n int) error {
		var r struct {
			Name       string             `json:"name"`
			Legalities *map[string]string `json:"legalities"`
			Games      *[]string          `json:"games"`
		}
		if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		if r.Legalities == nil {
			return fmt.Errorf("%q: no legalities field", r.Name)
		}
		c, ok := (*r.Legalities)["commander"]
		if !ok {
			return fmt.Errorf("%q: no legalities.commander field", r.Name)
		}
		if r.Games == nil {
			return fmt.Errorf("%q: no games field", r.Name)
		}
		l := Legality{Commander: c == "legal"}
		for _, g := range *r.Games {
			l.Paper = l.Paper || g == "paper"
		}
		out = append(out, l)
		return nil
	})
	return out, err
}
