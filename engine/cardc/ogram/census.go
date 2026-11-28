package ogram

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CensusRow is one outcome's face counts: every face, and the faces whose
// source record is Commander-legal paper.
type CensusRow struct {
	Outcome        Outcome
	All, Commander int
}

// CensusResult is a census row per outcome, in Outcomes order.
type CensusResult struct {
	Rows []CensusRow
}

// Total is the number of faces counted.
func (r CensusResult) Total() int {
	n := 0
	for _, row := range r.Rows {
		n += row.All
	}
	return n
}

// Census counts the faces of the release in dir by outcome. The unique faces
// are faces.jsonl.gz's; the ambiguous and refused ones come from
// outcomes.jsonl.gz, whose unique count must agree. A face's legality is its
// source record's (ReadLegality, by Face.Record).
func Census(dir string) (CensusResult, error) {
	legal, err := ReadLegality(dir)
	if err != nil {
		return CensusResult{}, err
	}
	rows := make([]CensusRow, len(Outcomes))
	for i, o := range Outcomes {
		rows[i].Outcome = o
	}
	count := func(f *Face) error {
		if f.Record < 0 || f.Record >= len(legal) {
			return fmt.Errorf("%q: record_index %d outside the %d source records", f.Name, f.Record, len(legal))
		}
		rows[f.Outcome].All++
		if legal[f.Record].CommanderPaper() {
			rows[f.Outcome].Commander++
		}
		return nil
	}
	err = readFaceFile(filepath.Join(dir, facesFile), false, func(f *Face) error {
		if f.Outcome != OutUnique {
			return fmt.Errorf("%q: status %s in %s", f.Name, f.Outcome, facesFile)
		}
		return count(f)
	})
	if err != nil {
		return CensusResult{}, err
	}
	uniqueOutcomes := 0
	err = ReadOutcomes(dir, func(f *Face) error {
		if f.Outcome == OutUnique {
			uniqueOutcomes++
			return nil
		}
		return count(f)
	})
	if err != nil {
		return CensusResult{}, err
	}
	if uniqueOutcomes != rows[OutUnique].All {
		return CensusResult{}, fmt.Errorf("%s holds %d unique faces, %s %d", outcomesFile, uniqueOutcomes, facesFile, rows[OutUnique].All)
	}
	return CensusResult{Rows: rows}, nil
}

// Coverage is the release's own coverage.json.
type Coverage struct {
	InputFaces int `json:"input_faces"`
	Counts     struct {
		Unique    int `json:"unique"`
		Ambiguous int `json:"ambiguous"`
		Refused   int `json:"refused"`
		Limit     int `json:"limit"`
	} `json:"counts"`
}

// ReadCoverage reads coverage.json in dir.
func ReadCoverage(dir string) (Coverage, error) {
	var c Coverage
	b, err := os.ReadFile(filepath.Join(dir, coverageFile))
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}
