package event_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/draft/event"
)

// updateGolden rewrites the golden files (go test ./draft/event -run
// TestScriptedEventStateGolden -update).
var updateGolden = flag.Bool("update", false, "rewrite the scripted-event golden files")

// TestScriptedEventStateGolden pins the full scripted 4-seat event's
// marshalled StateJSON and log bytes, so a refactor of the event shape must keep a
// default 4-seat draft round-robin event byte-for-byte identical. Regenerate
// only with -update on an unmodified tree, and never as part of a behaviour
// change.
func TestScriptedEventStateGolden(t *testing.T) {
	e := newScriptedEvent(t)
	state, err := e.StateJSON()
	if err != nil {
		t.Fatalf("StateJSON: %v", err)
	}
	wire, err := event.MarshalLog(e.Log())
	if err != nil {
		t.Fatalf("MarshalLog: %v", err)
	}
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("testdata", "scripted_state.json"), state, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join("testdata", "scripted_log.jsonl"), wire, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	wantState, err := os.ReadFile(filepath.Join("testdata", "scripted_state.json"))
	if err != nil {
		t.Fatalf("read state golden: %v", err)
	}
	wantWire, err := os.ReadFile(filepath.Join("testdata", "scripted_log.jsonl"))
	if err != nil {
		t.Fatalf("read log golden: %v", err)
	}
	if !bytes.Equal(state, wantState) {
		t.Errorf("StateJSON differs from the golden:\n got: %s\nwant: %s", state, wantState)
	}
	if !bytes.Equal(wire, wantWire) {
		t.Errorf("MarshalLog differs from the golden:\n got: %s\nwant: %s", wire, wantWire)
	}
}
