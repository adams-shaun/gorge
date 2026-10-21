package env_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/minihost"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
)

var update = flag.Bool("update", false, "regenerate goldens")

// record is one transcript line. A request that is not JSON at all
// (malformed_json) is kept as the string raw, since message must be an object.
type record struct {
	Dir     string          `json:"dir"`
	Message json.RawMessage `json:"message,omitempty"`
	Raw     string          `json:"raw,omitempty"`
}

// TestGoldensProtocolV2Unchanged is the ported TestGoldensReplayByteExact:
// every recorded Protocol v2 exchange must still replay byte for byte.
func TestGoldensProtocolV2Unchanged(t *testing.T) {
	t.Run("corpus_pinned", checkProtocolV2Corpus)
	dir := filepath.Join("..", "testdata", "goldens")
	if *update {
		if err := writeGoldens(t, dir); err != nil {
			t.Fatal(err)
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.transcript.jsonl.gz"))
	if len(files) != 27 {
		t.Fatalf("%d golden files, want exactly 27", len(files))
	}
	var digests map[string]string
	if b, err := os.ReadFile(filepath.Join(dir, "digests.json")); err != nil || json.Unmarshal(b, &digests) != nil || len(digests) != 5 {
		t.Fatalf("digests.json: %v, %d entries, want 5", err, len(digests))
	}
	for _, f := range files {
		s := env.NewForGoldens(testcorpus.Registry(t))
		fh, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		gz, err := gzip.NewReader(fh)
		if err != nil {
			fh.Close()
			t.Fatal(err)
		}
		sc := bufio.NewScanner(gz)
		sc.Buffer(make([]byte, 1<<20), 9<<20)
		var last []byte
		var dig *wire.GameDigest // Section 11.8 over the engine lines, from the reset on
		for sc.Scan() {
			var r record
			if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
				t.Fatalf("%s: %v", f, err)
			}
			switch {
			case r.Dir == "host_to_engine" && r.Raw != "":
				last = s.Handle([]byte(r.Raw))
			case r.Dir == "host_to_engine":
				last = s.Handle(r.Message)
			case r.Dir == "engine_to_host" && string(last) != string(r.Message):
				t.Fatalf("%s: engine response differs:\n got %s\nwant %s", filepath.Base(f), last, r.Message)
			}
			if (r.Dir == "host_to_engine" || r.Dir == "engine_to_host") && r.Raw == "" {
				// The mini-host now records its profile handshake before reset.
				// Section 11.8 starts the game chain at reset, so verify those
				// hello bytes above without including them in the game digest.
				if dig == nil {
					var head struct {
						RequestType string `json:"request_type"`
					}
					if json.Unmarshal(r.Message, &head) != nil || r.Dir != "host_to_engine" || head.RequestType != "reset" {
						continue
					}
				}
				msg, err := wire.WithoutRequestID(r.Message)
				switch {
				case err != nil:
					t.Fatalf("%s: %v", f, err)
				case dig == nil:
					dig, err = wire.NewGameDigest(msg)
				default:
					err = dig.Chain(msg)
				}
				if err != nil {
					t.Fatalf("%s: %v", f, err)
				}
			}
		}
		if err := sc.Err(); err != nil {
			gz.Close()
			fh.Close()
			t.Fatal(err)
		}
		gz.Close()
		fh.Close()
		name := strings.TrimSuffix(filepath.Base(f), ".transcript.jsonl.gz")
		if want, ok := digests[name]; ok && (dig == nil || dig.String() != want) {
			t.Fatalf("%s: replayed digest %v, digests.json has %s", name, dig, want)
		}
	}
}

// gzipDeterministic compresses with every header field that would otherwise
// vary (ModTime, Name, Comment, OS) held at its zero/default value, so
// regenerating byte-identical content produces a byte-identical .gz file.
func gzipDeterministic(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	w.Header.ModTime = time.Time{}
	w.Header.Name = ""
	w.Header.Comment = ""
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeTranscript(path string, recs []record) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b) // one compact line per record, no HTML escaping
	enc.SetEscapeHTML(false)
	for _, r := range recs {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	gz, err := gzipDeterministic(b.Bytes())
	if err != nil {
		return err
	}
	return os.WriteFile(path, gz, 0o644)
}

// recorder wraps a link and appends both directions of every exchange.
type recorder struct {
	inner    minihost.Link
	to, from string
	recs     *[]record
}

func (r *recorder) Round(req []byte) ([]byte, error) {
	out, err := r.inner.Round(req)
	*r.recs = append(*r.recs, record{Dir: r.to, Message: req}, record{Dir: r.from, Message: out})
	return out, err
}

func playRecorded(reg *cards.Registry, i uint64, deck catalog.Deck, maxSteps uint64) ([]record, minihost.Result, error) {
	var recs []record
	kinds := map[string]bool{}
	for _, k := range env.DecisionKinds {
		kinds[k] = true
	}
	h := &minihost.Host{RunSecret: make([]byte, 32),
		Engine:   &recorder{inner: &minihost.EngineLink{S: env.NewForGoldens(reg)}, to: "host_to_engine", from: "engine_to_host", recs: &recs},
		Profile:  validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{}},
		MaxSteps: maxSteps, MaxDecisions: maxSteps}
	agents := [2]minihost.Link{
		&recorder{inner: &minihost.Uniform{}, to: "host_to_agent", from: "agent_to_host", recs: &recs},
		&recorder{inner: &minihost.Uniform{}, to: "host_to_agent", from: "agent_to_host", recs: &recs}}
	res, err := h.Play(i, deck, "london", []string{}, agents)
	return recs, res, err
}

// firstGroup finds the first engine decision containing needle, and not
// exclude when it is set, whose group has at least minSize substeps, and
// returns the step that group started at and its size.
func firstGroup(recs []record, needle, exclude string, minSize uint64) (start, size uint64, ok bool) {
	for _, r := range recs {
		msg := string(r.Message)
		if r.Dir != "engine_to_host" || !strings.Contains(msg, needle) || (exclude != "" && strings.Contains(msg, exclude)) {
			continue
		}
		var d struct {
			Step         uint64 `json:"step"`
			SeatDecision struct {
				Group struct {
					SubstepIndex uint64 `json:"substep_index"`
					SubstepCount uint64 `json:"substep_count"`
				} `json:"group"`
			} `json:"seat_decision"`
		}
		if json.Unmarshal(r.Message, &d) == nil && d.SeatDecision.Group.SubstepCount >= minSize {
			return d.Step - d.SeatDecision.Group.SubstepIndex, d.SeatDecision.Group.SubstepCount, true
		}
	}
	return 0, 0, false
}

func writeGoldens(t *testing.T, dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Drop the pre-gzip fixtures so a regeneration cannot leave both forms.
	stale, _ := filepath.Glob(filepath.Join(dir, "*.transcript.jsonl"))
	for _, f := range stale {
		if err := os.Remove(f); err != nil {
			return err
		}
	}
	reg := testcorpus.Registry(t)
	hello := []byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"h-1","protocol_minor":0}`)
	step := func(game string, exp, cand int, echo string) []byte {
		return []byte(fmt.Sprintf(`{"request_type":"step","protocol":"spellbench/v2","request_id":"s-1","game_id":%q,"expected_step":%d,"selection":{"candidate_id":%d,"semantic_echo":%s}}`, game, exp, cand, echo))
	}
	// Goldens are generated without x_gorge_view_v1: its payload carries gorge
	// cost strings compiled from Forge scripts, which this module never ships.
	noExt := `"extensions":["x_gorge_view_v1"]=>"extensions":[]`
	noExtReset := func(deck, mutate string) []byte {
		return []byte(strings.Replace(string(reset("r-1", "g-1", deck, mutate)), `"extensions":["x_gorge_view_v1"]`, `"extensions":[]`, 1))
	}
	burn := reset("r-1", "g-1", "Burn", noExt)
	burnDeck, _ := catalog.ByID("Burn")
	// deck_id_mismatch: the wrong id must keep Section 4.3's form, since the
	// decoder refuses a malformed deck_id before the server runs.
	zeroID := "sha256:" + strings.Repeat("0", 64)
	pass := `{"kind":"pass"}`
	scenarios := []struct {
		name  string
		lines [][]byte
	}{
		{"hello", [][]byte{hello}},
		{"malformed_json", [][]byte{[]byte(`{"request_type":`)}},
		{"malformed_request", [][]byte{[]byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"h-1"}`)}},
		{"protocol_mismatch", [][]byte{[]byte(`{"request_type":"hello","protocol":"spellbench/v1","request_id":"h-1","protocol_minor":0}`)}},
		{"request_id_reuse_mismatch", [][]byte{hello, []byte(`{"request_type":"hello","protocol":"spellbench/v2","request_id":"h-1","protocol_minor":1}`)}},
		{"step_before_reset", [][]byte{step("g-1", 0, 0, pass)}},
		{"game_already_active", [][]byte{burn, reset("r-2", "g-2", "Burn", noExt)}},
		{"game_id_mismatch", [][]byte{burn, step("g-9", 0, 0, pass)}},
		{"expected_step_mismatch", [][]byte{burn, step("g-1", 5, 0, pass)}},
		{"candidate_id_out_of_range", [][]byte{burn, step("g-1", 0, 4095, pass)}},
		{"semantic_echo_mismatch", [][]byte{burn, step("g-1", 0, 0, `{"kind":"nonsense"}`)}},
		{"unsupported_format", [][]byte{noExtReset("Burn", `"format":"pauper-bo1"=>"format":"modern-bo1"`)}},
		{"unsupported_deck", [][]byte{noExtReset("Burn", `"catalog_id":"Burn"}}]=>"catalog_id":"Terror"}}]`)}},
		{"deck_id_mismatch", [][]byte{noExtReset("Burn", `"deck_id":"`+burnDeck.DeckID()+`"=>"deck_id":"`+zeroID+`"`)}},
		{"unsupported_rule", [][]byte{noExtReset("Burn", `"probe":false=>"probe":true`)}},
		{"unsupported_request", [][]byte{[]byte(`{"request_type":"probe_resample","protocol":"spellbench/v2","request_id":"p-1","game_id":"g-1","samples":4}`)}},
		{"game_already_terminal", [][]byte{noExtReset("Burn", `"max_steps":100000=>"max_steps":0`), step("g-1", 0, 0, pass)}},
	}
	for _, d := range catalog.Decks() {
		scenarios = append(scenarios, struct {
			name  string
			lines [][]byte
		}{"reset_first_decision_" + strings.ToLower(d.CatalogID), [][]byte{reset("r-1", "g-1", d.CatalogID, noExt)}})
	}
	for _, sc := range scenarios {
		s := env.NewForGoldens(reg)
		var recs []record
		for _, l := range sc.lines {
			in := record{Dir: "host_to_engine", Message: l}
			if !json.Valid(l) {
				in = record{Dir: "host_to_engine", Raw: string(l)}
			}
			recs = append(recs, in, record{Dir: "engine_to_host", Message: s.Handle(l)})
		}
		// Each error scenario's last engine line must carry its own code, so a
		// mutation regressing to another code (say malformed_request) fails here.
		if last := recs[len(recs)-1].Message; sc.name != "hello" && !strings.HasPrefix(sc.name, "reset_first_decision_") &&
			!bytes.Contains(last, []byte(`"code":"`+sc.name+`"`)) {
			return fmt.Errorf("%s: last engine line does not record its error code: %s", sc.name, last)
		}
		if err := writeTranscript(filepath.Join(dir, sc.name+".transcript.jsonl.gz"), recs); err != nil {
			return err
		}
	}
	// Game scenarios: whole host, engine and agent exchanges of seeded Uniform games.
	digests := map[string]string{}
	recs, res, err := playRecorded(reg, 0, burnDeck, 300)
	if err != nil {
		return err
	}
	if err := writeTranscript(filepath.Join(dir, "uniform_game_burn.transcript.jsonl.gz"), recs); err != nil {
		return err
	}
	digests["uniform_game_burn"] = res.Digest
	// Group shapes: the chosen seeded game that reaches each soonest, cut by
	// max_steps right after that group (a cap never splits a group).
	// Arrangements and order blocks need three substeps or more, so a scry 1
	// or a one-card bottom block never stands in for a full group; an order
	// block is one outside an arrangement, so the two goldens differ.
	//
	// The seeds and decks were measured as the earliest reach of each ask over
	// seeds 0..19 and every catalog deck (mana_payment has no earlier reach;
	// it first appears in seed 128's CawGates game at step 339, the earliest
	// over CawGates seeds 0..139 once a cycling cost stopped posing a choice
	// for the card cycled). scan bounds the
	// full-game pass that locates the group; it is larger than the known ask
	// but far below the 3000 a whole game can run.
	targets := []struct {
		name, needle, exclude string
		minSize, seed, scan   uint64
		deck                  string
	}{
		{"arrangement", `"kind":"arrange_card"`, "", 3, 8, 256, "CawGates"},
		{"attack_declaration", `"kind":"declare_attack"`, "", 1, 4, 256, "Rally"},
		{"order_pick", `"kind":"order_pick"`, `"purpose":"arrangement"`, 3, 1, 256, "Wildfire"},
		{"mana_payment", `"purpose":"mana_payment"`, "", 1, 128, 1200, "CawGates"},
	}
	for _, tg := range targets {
		d, ok := catalog.ByID(tg.deck)
		if !ok {
			return fmt.Errorf("no catalog deck %s for %s", tg.deck, tg.name)
		}
		full, _, err := playRecorded(reg, tg.seed, d, tg.scan)
		if err != nil {
			return err
		}
		start, size, ok := firstGroup(full, tg.needle, tg.exclude, tg.minSize)
		if !ok {
			return fmt.Errorf("seed %d deck %s did not reach %s", tg.seed, tg.deck, tg.name)
		}
		cut, res, err := playRecorded(reg, tg.seed, d, start+size)
		if err != nil {
			return err
		}
		if err := writeTranscript(filepath.Join(dir, tg.name+".transcript.jsonl.gz"), cut); err != nil {
			return err
		}
		digests[tg.name] = res.Digest
	}
	b, _ := json.MarshalIndent(digests, "", "  ")
	return os.WriteFile(filepath.Join(dir, "digests.json"), append(b, '\n'), 0o644)
}
