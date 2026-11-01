package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/draftlab/extern/wire"
)

// runPipe drives run() over pipes and returns the agent's stdout bytes.
func runPipe(t *testing.T, args []string, in string) (string, int) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(in), &stdout, &stderr)
	return stdout.String() + stderr.String(), code
}

func readLine(t *testing.T, out string) wire.Message {
	t.Helper()
	line, rest, _ := strings.Cut(out, "\n")
	if rest != "" {
		t.Errorf("more than one output line:\n%s", out)
	}
	var m wire.Message
	if err := json.Unmarshal([]byte(line), &m); err != nil {
		t.Fatalf("decode %q: %v", line, err)
	}
	return m
}

// TestRunStdio serves a decide batch over pipes and exits 0; the default
// -view-omit is all, so the hello names every omission.
func TestRunStdio(t *testing.T) {
	hello := `{"v":1,"type":"hello","protocol":"gorge-agent/1","kinds":["pick","build","decide"]}` + "\n"
	batch := `{"v":1,"type":"batch","batch":1,"items":[{"id":1,"kind":"decide","seed":9,"stream":"g1.0","view":{"viewer":1,"decision":{"seq":1,"player":1,"kind":"priority","min":1,"max":1,"options":[{"index":0,"kind":"pass","label":"Pass"},{"index":1,"kind":"concede","label":"Concede"}]}}}]}` + "\n"
	bye := `{"v":1,"type":"bye"}` + "\n"
	out, code := runPipe(t, nil, hello+batch+bye)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("%d output lines, want 2 (hello, result):\n%s", len(lines), out)
	}
	var helloMsg wire.Message
	if err := json.Unmarshal([]byte(lines[0]), &helloMsg); err != nil {
		t.Fatal(err)
	}
	if err := helloMsg.Validate(wire.FromAgent); err != nil {
		t.Fatalf("agent hello invalid: %v", err)
	}
	if len(helloMsg.ViewOmit) != len(wire.ViewOmitNames) {
		t.Errorf("default -view-omit names %d omissions, want all %d", len(helloMsg.ViewOmit), len(wire.ViewOmitNames))
	}
	for _, e := range wire.ViewOmitNames {
		found := false
		for _, n := range helloMsg.ViewOmit {
			if n == e.Name {
				found = true
			}
		}
		if !found {
			t.Errorf("default -view-omit misses %q", e.Name)
		}
	}
	var result wire.Message
	if err := json.Unmarshal([]byte(lines[1]), &result); err != nil {
		t.Fatal(err)
	}
	if err := result.Validate(wire.FromAgent); err != nil {
		t.Fatalf("agent result invalid: %v", err)
	}
	if result.Type != wire.TResult || result.Batch != 1 || len(result.Answers) != 1 ||
		result.Answers[0].ID != 1 || result.Answers[0].Intent == nil {
		t.Fatalf("batch was not answered: %s", lines[1])
	}
	if result.Answers[0].Intent.Choices[0] == 1 {
		t.Errorf("the agent conceded")
	}
}

// TestRunViewOmitNone declares no omission: the hello carries no
// view_omit at all.
func TestRunViewOmitNone(t *testing.T) {
	hello := `{"v":1,"type":"hello","protocol":"gorge-agent/1","kinds":["pick"]}` + "\n"
	bye := `{"v":1,"type":"bye"}` + "\n"
	out, code := runPipe(t, []string{"-view-omit", "none", "-policy", "first"}, hello+bye)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, out)
	}
	m := readLine(t, out)
	if len(m.ViewOmit) != 0 {
		t.Errorf("view_omit = %v, want absent", m.ViewOmit)
	}
}

// TestRunRefusesExistingSocketPath refuses to listen on a path that
// already exists, without touching it.
func TestRunRefusesExistingSocketPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.sock")
	if err := os.WriteFile(path, []byte("occupied"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	code := run([]string{"-listen", "unix:" + path}, strings.NewReader(""), &bytes.Buffer{}, &stderr)
	if code == 0 {
		t.Fatalf("run accepted an existing socket path: %s", stderr.String())
	}
	b, err := os.ReadFile(path)
	if err != nil || string(b) != "occupied" {
		t.Errorf("the existing path was touched: %q, %v", b, err)
	}
}

// TestRunRejectsBadFlags rejects an unknown policy and an unknown
// view_omit name with a usage error, not a serve error.
func TestRunRejectsBadFlags(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"-policy", "greedy"}, "-policy"},
		{[]string{"-view-omit", "hands"}, "bad -view-omit"},
		{[]string{"-view-omit", "decision"}, "bad -view-omit"},
		{[]string{"-listen", "tcp:1.2.3.4"}, "-listen must be unix:"},
	} {
		var stderr bytes.Buffer
		code := run(tc.args, strings.NewReader(""), &bytes.Buffer{}, &stderr)
		if code != 2 {
			t.Errorf("%v: exit %d, want 2 (stderr %q)", tc.args, code, stderr.String())
		}
		if !strings.Contains(stderr.String(), tc.want) {
			t.Errorf("%v: stderr %q does not name %q", tc.args, stderr.String(), tc.want)
		}
	}
}
