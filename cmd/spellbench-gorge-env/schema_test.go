package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/adams-shaun/gorge/bots/all"
	"github.com/adams-shaun/gorge/internal/schemacheck"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/minihost"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	engineRequest  = "requests.schema.json#/$defs/engine_request"
	engineResponse = "responses.schema.json#/$defs/engine_response"
	agentRequest   = "requests.schema.json#/$defs/agent_request"
	agentResponse  = "responses.schema.json#/$defs/agent_response"

	schemaDir = "../../docs/site/api/schemas/spellbench-v2"
)

// TestMain lets the test binary stand in for the command, so the tests below talk
// to the real main over its stdin and stdout.
func TestMain(m *testing.M) {
	if os.Getenv("SPELLBENCH_GORGE_ENV_CHILD") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}

// engineProcess is a running spellbench-gorge-env.
type engineProcess struct {
	cmd *exec.Cmd
	in  io.WriteCloser
	out *bufio.Reader
}

func startEngine(t *testing.T, args ...string) *engineProcess {
	t.Helper()
	cmd := exec.Command(os.Args[0], append([]string{"-corpus", testutil.CorpusDirOrSkip(t)}, args...)...)
	cmd.Env = append(os.Environ(), "SPELLBENCH_GORGE_ENV_CHILD=1")
	cmd.Stderr = os.Stderr
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { in.Close(); cmd.Wait() })
	return &engineProcess{cmd, in, bufio.NewReaderSize(out, 1<<20)}
}

func (e *engineProcess) Round(req []byte) ([]byte, error) {
	if _, err := e.in.Write(append(bytes.Clone(req), '\n')); err != nil {
		return nil, err
	}
	line, err := e.out.ReadBytes('\n')
	return bytes.TrimRight(line, "\r\n"), err
}

// checked is a link whose lines must match their schemas, and which keeps count
// of the kinds of answer it has seen. An empty ref leaves that direction out.
type checked struct {
	t             *testing.T
	set           *schemacheck.Set
	inner         minihost.Link
	request, resp string
	seen          map[string]int
	decision      []byte // the first decision line
}

func (c *checked) line(ref string, line []byte) {
	c.t.Helper()
	if ref == "" {
		return
	}
	doc, err := schemacheck.Decode(line)
	if err == nil {
		err = c.set.Validate(ref, doc)
	}
	if err != nil {
		c.t.Fatalf("%s: %v\nline: %.2000s", ref, err, line)
	}
}

func (c *checked) Round(req []byte) ([]byte, error) {
	c.t.Helper()
	c.line(c.request, req)
	out, err := c.inner.Round(req)
	if err != nil {
		return out, err
	}
	c.line(c.resp, out)
	var head struct {
		ResponseType string `json:"response_type"`
	}
	if json.Unmarshal(out, &head) == nil {
		c.seen[head.ResponseType]++
		if head.ResponseType == "decision" && c.decision == nil {
			c.decision = bytes.Clone(out)
		}
	}
	return out, nil
}

func loadSchemas(t *testing.T) *schemacheck.Set {
	t.Helper()
	set, err := schemacheck.Load(schemaDir)
	if err != nil {
		t.Fatal(err)
	}
	return set
}

// TestEngineLinesMatchTheSchemas plays small games through the command in each
// mana mode, with and without the extension, and holds every line of the engine
// channel, and the answers of gorge's own agent, to the published schemas.
func TestEngineLinesMatchTheSchemas(t *testing.T) {
	set := loadSchemas(t)
	burn, ok := catalog.ByID("Burn")
	if !ok {
		t.Fatal("catalog deck Burn is missing")
	}
	kinds := map[string]bool{}
	for _, k := range env.DecisionKinds {
		kinds[k] = true
	}
	seen := map[string]int{}
	var decision []byte
	play := func(t *testing.T, args, extensions []string, maxSteps uint64, agents func(*testing.T) [2]minihost.Link) {
		link := &checked{t: t, set: set, inner: startEngine(t, args...), request: engineRequest, resp: engineResponse, seen: seen}
		host := &minihost.Host{RunSecret: make([]byte, 32), Engine: link,
			Profile:  validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{}},
			MaxSteps: maxSteps, MaxDecisions: maxSteps}
		if _, err := host.Play(1, burn, "london", extensions, agents(t)); err != nil {
			t.Fatal(err)
		}
		if decision == nil {
			decision = link.decision
		}
	}
	uniform := func(*testing.T) [2]minihost.Link {
		return [2]minihost.Link{&minihost.Uniform{}, &minihost.Uniform{}}
	}
	t.Run("autopay", func(t *testing.T) { play(t, nil, nil, 100000, uniform) })
	t.Run("manual_mana", func(t *testing.T) { play(t, []string{"-manual-mana"}, nil, 300, uniform) })
	t.Run("extension_and_gorge_agent", func(t *testing.T) {
		play(t, nil, []string{"x_gorge_view_v1"}, 100000, func(t *testing.T) [2]minihost.Link {
			var links [2]minihost.Link
			for i := range links {
				a, err := agent.New("bot")
				if err != nil {
					t.Fatal(err)
				}
				link := &checked{t: t, set: set, inner: a, resp: agentResponse, seen: map[string]int{}}
				link.Round([]byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"h-1","protocol_minor":0}`))
				links[i] = link
			}
			return links
		})
	})
	t.Run("refused_requests", func(t *testing.T) {
		link := &checked{t: t, set: set, inner: startEngine(t), resp: engineResponse, seen: seen}
		for _, line := range []string{
			`{"request_type":"hello","protocol":"spellbench/v2","request_id":"1","protocol_minor":0}`,
			`{"request_type":"validate_deck","protocol":"spellbench/v2","request_id":"2","format":"pauper-bo1","deck":{"catalog_id":"Burn"}}`,
			`{"request_type":"validate_deck","protocol":"spellbench/v2","request_id":"3","format":"modern","deck":{"catalog_id":"Burn"}}`,
			`{"request_type":"probe_resample","protocol":"spellbench/v2","request_id":"4","game_id":"g-1","samples":4}`,
			`not json`,
		} {
			if _, err := link.Round([]byte(line)); err != nil {
				t.Fatal(err)
			}
		}
	})
	for _, kind := range []string{"hello_ok", "decision", "terminal", "deck_ok", "error"} {
		if seen[kind] == 0 {
			t.Errorf("no %s was checked: %v", kind, seen)
		}
	}
	t.Logf("checked %v", seen)

	// The schema is only worth holding the wire to if it refuses a wrong value.
	wrong := bytes.Replace(decision, []byte(`"day_night":"none"`), []byte(`"day_night":""`), 1)
	doc, err := schemacheck.Decode(wrong)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(wrong, decision) || set.Validate(engineResponse, doc) == nil {
		t.Fatal(`a decision with "day_night":"" passed the schema`)
	}
}

// refusedCodes are the errors that answer a request breaking the protocol, which
// the transcripts send on purpose, so those requests are left out.
var refusedCodes = map[string]bool{"malformed_json": true, "malformed_request": true, "protocol_mismatch": true}

// TestUpstreamCorpusMatchesTheSchemas runs the vendored conformance transcripts
// through the same schemas.
func TestUpstreamCorpusMatchesTheSchemas(t *testing.T) {
	set := loadSchemas(t)
	refs := map[string]string{"host_to_engine": engineRequest, "engine_to_host": engineResponse,
		"host_to_agent": agentRequest, "agent_to_host": agentResponse}
	files, err := filepath.Glob("../../server/spellbench/testdata/goldens/protocol_v2/*.transcript.jsonl")
	if err != nil || len(files) == 0 {
		t.Fatalf("%d transcripts: %v", len(files), err)
	}
	check := func(f string, line int, dir string, message []byte) {
		t.Helper()
		doc, err := schemacheck.Decode(message)
		if err == nil {
			err = set.Validate(refs[dir], doc)
		}
		if err != nil {
			t.Errorf("%s:%d %s: %v", filepath.Base(f), line, dir, err)
		}
	}
	checked := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var request []byte
		var requestDir string
		for i, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
			var rec struct {
				Dir     string          `json:"dir"`
				Message json.RawMessage `json:"message"`
			}
			if err := json.Unmarshal(line, &rec); err != nil {
				t.Fatalf("%s:%d: %v", f, i+1, err)
			}
			if rec.Dir == "host_to_engine" || rec.Dir == "host_to_agent" {
				request, requestDir = rec.Message, rec.Dir
				continue
			}
			check(f, i+1, rec.Dir, rec.Message)
			checked++
			var answer struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if request != nil && json.Unmarshal(rec.Message, &answer) == nil && !refusedCodes[answer.Error.Code] {
				check(f, i, requestDir, request)
				checked++
			}
			request = nil
		}
	}
	t.Logf("checked %d lines", checked)
}
