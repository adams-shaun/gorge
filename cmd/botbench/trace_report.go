package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"

	"github.com/adams-shaun/gorge/decision"
)

func analyzeTraceFile(path string, out io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening decision trace: %w", err)
	}
	defer f.Close()
	return writeTraceDiagnostics(f, out)
}

type traceDiagnosticReport struct {
	Label    string              `json:"label"`
	Families []traceFamilyReport `json:"families"`
}

type traceCount struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

type traceContextReport struct {
	Pair          string `json:"pair"`
	StartingSeat  int    `json:"starting_seat"`
	Opportunities int    `json:"opportunities"`
	Wins          int    `json:"wins"`
	Losses        int    `json:"losses"`
	Draws         int    `json:"draws"`
	Stalls        int    `json:"stalls"`
}

type traceFamilyReport struct {
	Policy            string               `json:"policy"`
	Kind              decision.Kind        `json:"kind"`
	Opportunities     int                  `json:"opportunities"`
	OfferedOptions    int                  `json:"offered_options"`
	Singleton         int                  `json:"singleton"`
	Widths            []traceCount         `json:"widths"`
	SelectedKinds     []traceCount         `json:"selected_kinds"`
	SelectedCastModes []traceCount         `json:"selected_cast_modes"`
	SelectedPositions []traceCount         `json:"selected_positions"`
	Wins              int                  `json:"wins"`
	Losses            int                  `json:"losses"`
	Draws             int                  `json:"draws"`
	Stalls            int                  `json:"stalls"`
	TurnsTotal        int64                `json:"turns_total"`
	Contexts          []traceContextReport `json:"contexts"`
}

type traceFamilyKey struct {
	policy string
	kind   decision.Kind
}

type traceContextKey struct {
	pair  string
	start int
}

type traceFamilyAccum struct {
	report    traceFamilyReport
	widths    map[string]int
	kinds     map[string]int
	modes     map[string]int
	positions map[string]int
	contexts  map[traceContextKey]*traceContextReport
}

func writeTraceDiagnostics(in io.Reader, out io.Writer) error {
	dec := json.NewDecoder(in)
	var run *traceRunV1
	var pending []traceDecisionV1
	families := make(map[traceFamilyKey]*traceFamilyAccum)
	for {
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("reading decision trace: %w", err)
		}
		var header struct {
			RecordType    string `json:"record_type"`
			SchemaVersion int    `json:"schema_version"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			return fmt.Errorf("reading decision trace header: %w", err)
		}
		if header.SchemaVersion != traceSchemaVersion {
			return fmt.Errorf("decision trace schema version %d is unsupported", header.SchemaVersion)
		}
		switch header.RecordType {
		case "run-v1":
			if run != nil || len(pending) != 0 {
				return fmt.Errorf("run-v1 must be the first and only run record")
			}
			var r traceRunV1
			if err := json.Unmarshal(raw, &r); err != nil {
				return err
			}
			if err := validateTraceRecord(r); err != nil {
				return err
			}
			run = &r
		case "decision-v1":
			if run == nil {
				return fmt.Errorf("decision-v1 precedes run-v1")
			}
			var r traceDecisionV1
			if err := json.Unmarshal(raw, &r); err != nil {
				return err
			}
			if err := validateTraceRecord(r); err != nil {
				return err
			}
			pending = append(pending, r)
		case "game-v1":
			if run == nil {
				return fmt.Errorf("game-v1 precedes run-v1")
			}
			var r traceGameV1
			if err := json.Unmarshal(raw, &r); err != nil {
				return err
			}
			if err := validateTraceRecord(r); err != nil {
				return err
			}
			for _, d := range pending {
				if d.PairIndex != r.PairIndex || d.GameIndex != r.GameIndex || d.Seed != r.Seed {
					return fmt.Errorf("decision does not belong to following game-v1")
				}
				accumulateTraceDecision(families, d, r)
			}
			pending = pending[:0]
		default:
			return fmt.Errorf("unknown decision trace record type %q", header.RecordType)
		}
	}
	if run == nil {
		return fmt.Errorf("decision trace has no run-v1 record")
	}
	if len(pending) != 0 {
		return fmt.Errorf("decision trace ends before game-v1")
	}
	report := traceDiagnosticReport{Label: "diagnostic proxies", Families: make([]traceFamilyReport, 0, len(families))}
	keys := make([]traceFamilyKey, 0, len(families))
	for key := range families {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].policy != keys[j].policy {
			return keys[i].policy < keys[j].policy
		}
		return keys[i].kind < keys[j].kind
	})
	for _, key := range keys {
		a := families[key]
		a.report.Widths = sortedTraceCounts(a.widths, true)
		a.report.SelectedKinds = sortedTraceCounts(a.kinds, false)
		a.report.SelectedCastModes = sortedTraceCounts(a.modes, false)
		a.report.SelectedPositions = sortedTraceCounts(a.positions, true)
		contextKeys := make([]traceContextKey, 0, len(a.contexts))
		for k := range a.contexts {
			contextKeys = append(contextKeys, k)
		}
		sort.Slice(contextKeys, func(i, j int) bool {
			if contextKeys[i].pair != contextKeys[j].pair {
				return contextKeys[i].pair < contextKeys[j].pair
			}
			return contextKeys[i].start < contextKeys[j].start
		})
		for _, k := range contextKeys {
			a.report.Contexts = append(a.report.Contexts, *a.contexts[k])
		}
		report.Families = append(report.Families, a.report)
	}
	return json.NewEncoder(out).Encode(report)
}

func accumulateTraceDecision(families map[traceFamilyKey]*traceFamilyAccum, d traceDecisionV1, game traceGameV1) {
	key := traceFamilyKey{policy: d.Policy, kind: d.Kind}
	a := families[key]
	if a == nil {
		a = &traceFamilyAccum{
			report: traceFamilyReport{Policy: d.Policy, Kind: d.Kind, Widths: []traceCount{}, SelectedKinds: []traceCount{}, SelectedCastModes: []traceCount{}, SelectedPositions: []traceCount{}, Contexts: []traceContextReport{}},
			widths: make(map[string]int), kinds: make(map[string]int), modes: make(map[string]int), positions: make(map[string]int), contexts: make(map[traceContextKey]*traceContextReport),
		}
		families[key] = a
	}
	a.report.Opportunities++
	a.report.OfferedOptions += len(d.Options)
	if len(d.Options) == 1 {
		a.report.Singleton++
	}
	a.widths[strconv.Itoa(len(d.Options))]++
	for _, pos := range d.Choices {
		a.positions[strconv.Itoa(pos)]++
		a.kinds[d.Options[pos].Kind]++
		if d.Options[pos].CastMode != "" {
			a.modes[d.Options[pos].CastMode]++
		}
	}
	start := -1
	if game.StartingSeat != nil {
		start = *game.StartingSeat
	}
	ck := traceContextKey{pair: game.Pair, start: start}
	c := a.contexts[ck]
	if c == nil {
		c = &traceContextReport{Pair: game.Pair, StartingSeat: start}
		a.contexts[ck] = c
	}
	a.report.TurnsTotal += int64(game.Turns)
	c.Opportunities++
	switch {
	case game.Stall != "":
		a.report.Stalls++
		c.Stalls++
	case game.Draw:
		a.report.Draws++
		c.Draws++
	case game.WinnerSeat != nil && *game.WinnerSeat == d.Seat:
		a.report.Wins++
		c.Wins++
	default:
		a.report.Losses++
		c.Losses++
	}
}

func sortedTraceCounts(m map[string]int, numeric bool) []traceCount {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	if numeric {
		sort.Slice(keys, func(i, j int) bool {
			a, _ := strconv.Atoi(keys[i])
			b, _ := strconv.Atoi(keys[j])
			return a < b
		})
	} else {
		sort.Strings(keys)
	}
	result := make([]traceCount, 0, len(keys))
	for _, key := range keys {
		result = append(result, traceCount{Value: key, Count: m[key]})
	}
	return result
}
