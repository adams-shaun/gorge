package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraceReportAggregatesAndSortsDiagnosticProxies(t *testing.T) {
	input := strings.Join([]string{
		`{"record_type":"run-v1","schema_version":1,"board_schema_version":1,"policy_a":"bot","policy_b":"legacy","base_seed":0,"games_per_pair":1,"seats":2,"format":"constructed","pairs":["a:b"]}`,
		`{"record_type":"decision-v1","schema_version":1,"pair_index":0,"pair":"a:b","game_index":0,"seed":0,"decision_sequence":1,"seat":0,"policy":"bot","kind":"priority","min":1,"max":1,"options":[{"index":0,"kind":"pass","player":0},{"index":1,"kind":"cast","player":0,"cast_mode":"kicked"}],"choices":[1],"board":{"schema_version":1,"is_main":true,"mana_wubrgc":[0,0,0,0,0,0],"cards":null,"life":null,"creatures":null,"commanders":null,"stack":null}}`,
		`{"record_type":"decision-v1","schema_version":1,"pair_index":0,"pair":"a:b","game_index":0,"seed":0,"decision_sequence":2,"seat":1,"policy":"legacy","kind":"target","min":1,"max":1,"options":[{"index":0,"kind":"target","player":0}],"choices":[0],"board":{"schema_version":1,"is_main":false,"mana_wubrgc":[0,0,0,0,0,0],"cards":null,"life":null,"creatures":null,"commanders":null,"stack":null}}`,
		`{"record_type":"game-v1","schema_version":1,"pair_index":0,"pair":"a:b","game_index":0,"seed":0,"winner_seat":0,"draw":false,"turns":7,"intents":2,"starting_seat":1}`,
	}, "\n") + "\n"
	var out bytes.Buffer
	if err := writeTraceDiagnostics(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	want := `{"label":"diagnostic proxies","families":[{"policy":"bot","kind":"priority","opportunities":1,"offered_options":2,"singleton":0,"widths":[{"value":"2","count":1}],"selected_kinds":[{"value":"cast","count":1}],"selected_cast_modes":[{"value":"kicked","count":1}],"selected_positions":[{"value":"1","count":1}],"wins":1,"losses":0,"draws":0,"stalls":0,"turns_total":7,"contexts":[{"pair":"a:b","starting_seat":1,"opportunities":1,"wins":1,"losses":0,"draws":0,"stalls":0}]},{"policy":"legacy","kind":"target","opportunities":1,"offered_options":1,"singleton":1,"widths":[{"value":"1","count":1}],"selected_kinds":[{"value":"target","count":1}],"selected_cast_modes":[],"selected_positions":[{"value":"0","count":1}],"wins":0,"losses":1,"draws":0,"stalls":0,"turns_total":7,"contexts":[{"pair":"a:b","starting_seat":1,"opportunities":1,"wins":0,"losses":1,"draws":0,"stalls":0}]}]}` + "\n"
	if out.String() != want {
		t.Fatalf("diagnostic report:\n%s\nwant:\n%s", out.String(), want)
	}
}

func TestTraceReportRejectsUnknownVersion(t *testing.T) {
	in := `{"record_type":"run-v1","schema_version":2,"board_schema_version":1}` + "\n"
	var out bytes.Buffer
	err := writeTraceDiagnostics(strings.NewReader(in), &out)
	if err == nil || !strings.Contains(err.Error(), "schema version") {
		t.Fatalf("unknown version error = %v", err)
	}
}

func TestAnalyzeTraceFileReadsJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	in := `{"record_type":"run-v1","schema_version":1,"board_schema_version":1}` + "\n"
	if err := os.WriteFile(path, []byte(in), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := analyzeTraceFile(path, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != `{"label":"diagnostic proxies","families":[]}`+"\n" {
		t.Fatalf("empty trace report = %s", out.String())
	}
}
